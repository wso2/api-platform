/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package migrationcore

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// Data-version / origin constants written on every migrated row.
const (
	DataVersion = "1.0"
	OriginCP    = "control_plane"
)

// MigrationActorIDPID is the synthetic IdP identity attributed to rows whose v1
// audit value is NULL/empty. Seeded into user_idp_references like any identity so
// it resolves to a real principal (not DeletedUser).
const MigrationActorIDPID = "migration"

// ErrBlobUnparseable signals that a config blob would not unmarshal into the v2
// model — the caller should quarantine (batch) or fail the mutation (live) rather
// than write a half-artifact.
var ErrBlobUnparseable = errors.New("config blob does not unmarshal into the v2 model")

// Options parameterizes the shared core. NO global env/flags are read inside the
// core — the batch main builds this from flags/env, the live path from v1 config.
// The EncryptionKey + Epoch used live MUST equal the batch backfill's, or
// synthesized UUIDs / token passthrough diverge.
type Options struct {
	EncryptionKey []byte         // subscription-token key (verbatim passthrough; used only by the decrypt guard)
	SourceTZ      *time.Location // timezone of naive v1 TIMESTAMP values (default UTC)
	Epoch         time.Time      // fixed epoch for deterministic synthesized UUIDs
	// InsertOnly makes upserts ON CONFLICT DO NOTHING (batch backfill semantics).
	// The live dual-write path sets this false so UPDATEs mirror via DO UPDATE.
	InsertOnly bool
	// SkipIdentityUpsert: when true (batch, which bulk-pre-seeds user_idp_references)
	// ResolveIdentity computes the deterministic UUID without a per-row write. The
	// live path leaves it false so each mutation upserts its actor on demand.
	SkipIdentityUpsert bool
	// DryRun runs every transform + Reporter event but performs NO database write
	// (the batch's --dry-run transform-and-validate pass). The live path never sets it.
	DryRun bool
}

func (o Options) loc() *time.Location {
	if o.SourceTZ == nil {
		return time.UTC
	}
	return o.SourceTZ
}

// Reporter receives per-row migrate-with-flag / quarantine / drop events. The batch
// implements it with the JSONL files (output unchanged); the live path implements
// it with structured logs + the v2_dual_write_failures table.
type Reporter interface {
	Flag(table, key, code string, oldV, newV any)
	Quarantine(table, key, code, detail string, row any)
	Dropped(scope, table, key, feature, label string)
	DroppedFields(structName string, fields []string)
}

// NopReporter discards all events (for callers that don't need reporting).
type NopReporter struct{}

func (NopReporter) Flag(string, string, string, any, any)   {}
func (NopReporter) Quarantine(string, string, string, string, any) {}
func (NopReporter) Dropped(string, string, string, string, string) {}
func (NopReporter) DroppedFields(string, []string)          {}

// Execer is satisfied by *sql.DB and *sql.Tx (and *database.DB, which embeds
// *sql.DB). The caller owns the transaction: pass a *sql.Tx to make one entity's
// full v2 footprint (artifact + type row + child rows) upsert atomically.
type Execer interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

// immutableOnUpdate lists creation-audit columns that a live ON CONFLICT DO UPDATE
// must never overwrite (§8.2): once a row exists, its created_at/created_by are fixed,
// so a later mirroring UPDATE keeps the original creation audit rather than stamping it
// with the updater's time/identity. The batch backfill is InsertOnly, so its INSERT …
// DO NOTHING path never consults this set — batch output is byte-identical.
var immutableOnUpdate = map[string]bool{
	"created_at": true,
	"created_by": true,
}

// upsert executes an idempotent INSERT … ON CONFLICT with $n placeholders (postgres).
// InsertOnly → DO NOTHING (batch); otherwise DO UPDATE SET every non-conflict,
// non-immutable column = excluded.<col> (live UPDATE mirroring; created_at/created_by
// are held immutable per §8.2). Empty conflictCols → plain INSERT.
func upsert(ex Execer, opts Options, table string, cols []string, args []any, conflictCols []string) error {
	if len(cols) != len(args) {
		return fmt.Errorf("upsert %s: %d cols but %d args", table, len(cols), len(args))
	}
	ph := make([]string, len(cols))
	for i := range cols {
		ph[i] = fmt.Sprintf("$%d", i+1)
	}
	q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ", "), strings.Join(ph, ", "))
	if len(conflictCols) > 0 {
		conflictSet := map[string]bool{}
		for _, c := range conflictCols {
			conflictSet[c] = true
		}
		set := make([]string, 0, len(cols))
		if !opts.InsertOnly {
			for _, c := range cols {
				if !conflictSet[c] && !immutableOnUpdate[c] {
					set = append(set, c+" = excluded."+c)
				}
			}
		}
		if len(set) == 0 {
			q += " ON CONFLICT (" + strings.Join(conflictCols, ", ") + ") DO NOTHING"
		} else {
			q += " ON CONFLICT (" + strings.Join(conflictCols, ", ") + ") DO UPDATE SET " + strings.Join(set, ", ")
		}
	}
	if opts.DryRun {
		return nil
	}
	if _, err := ex.Exec(q, args...); err != nil {
		return fmt.Errorf("upsert %s: %w", table, err)
	}
	return nil
}

// deleteWhere executes DELETE FROM table WHERE <col=$n AND …>. Used only by the
// live path's DeleteX (batch never deletes). Honors opts.DryRun (no write).
func deleteWhere(ex Execer, opts Options, table string, whereCols []string, whereArgs []any) error {
	conds := make([]string, len(whereCols))
	for i, c := range whereCols {
		conds[i] = fmt.Sprintf("%s = $%d", c, i+1)
	}
	q := fmt.Sprintf("DELETE FROM %s WHERE %s", table, strings.Join(conds, " AND "))
	if opts.DryRun {
		return nil
	}
	if _, err := ex.Exec(q, whereArgs...); err != nil {
		return fmt.Errorf("delete %s: %w", table, err)
	}
	return nil
}

// DeterministicUUID is a thin alias over the product's deterministic UUIDv7 helper
// so synthesized PKs are idempotent/resumable without persisted state.
func DeterministicUUID(entity string, ts time.Time) string {
	return utils.GenerateDeterministicUUIDv7(entity, ts)
}

// ResolveIdentity maps a raw v1 actor to its internal v2 user UUID (§C.1). Empty →
// the migration actor (defaulted=true). The UUID is deterministic; when
// opts.SkipIdentityUpsert is false and ex != nil, it also upserts the
// user_idp_references row on demand (ON CONFLICT (idp_id) DO NOTHING) — the live
// path's incremental seeding. The batch pre-seeds and sets SkipIdentityUpsert.
func ResolveIdentity(ex Execer, rawActor string, opts Options) (uuid string, defaulted bool, err error) {
	idpID := strings.TrimSpace(rawActor)
	if idpID == "" {
		idpID = MigrationActorIDPID
		defaulted = true
	}
	uuid = utils.GenerateDeterministicUUIDv7(idpID, opts.Epoch)
	if !opts.SkipIdentityUpsert && !opts.DryRun && ex != nil {
		if _, err = ex.Exec(
			`INSERT INTO user_idp_references (uuid, idp_id, created_at) VALUES ($1, $2, $3) ON CONFLICT (idp_id) DO NOTHING`,
			uuid, idpID, opts.Epoch); err != nil {
			return "", defaulted, fmt.Errorf("resolve identity %q: %w", idpID, err)
		}
	}
	return uuid, defaulted, nil
}

// audit resolves the created_by actor into the v2 audit UUID and, when the source
// was NULL/empty, emits a DEFAULTED_NULL flag (§C.1). updated_by == created_by.
func audit(ex Execer, opts Options, rep Reporter, table, key, rawActor string) (string, error) {
	uuid, defaulted, err := ResolveIdentity(ex, rawActor, opts)
	if err != nil {
		return "", err
	}
	if defaulted {
		rep.Flag(table, key, FlagDefaultedNull,
			map[string]any{"created_by": nil},
			map[string]any{"created_by": uuid, "actor": MigrationActorIDPID})
	}
	return uuid, nil
}

// tsArg converts a nullable v1 TIMESTAMP into a TIMESTAMPTZ insert arg (UTC
// instant) or nil. Never substitutes now().
func tsArg(t *time.Time, opts Options) any {
	if t == nil {
		return nil
	}
	return ReinterpretTZ(*t, opts.loc())
}

// Flag / quarantine / drop codes (shared vocabulary; the batch's JSONL and the
// live path's failure table use the same strings).
const (
	FlagDefaultedNull       = "DEFAULTED_NULL"
	FlagTruncated           = "TRUNCATED"
	FlagPlaceholderIDP      = "PLACEHOLDER_IDP"
	FlagSynthesized         = "SYNTHESIZED"
	FlagPlaintextCredential = "PLAINTEXT_CREDENTIAL"

	ReasonBlobUnparseable = "BLOB_UNPARSEABLE"

	DropBillingPlan       = "billing_plan"
	DropLLMProviderStatus = "llm_provider_status"
	DropLLMProxyStatus    = "llm_proxy_status"
	DropMCPStatus         = "mcp_status"
)
