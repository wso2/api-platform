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

package migration

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/utils"
	"github.com/wso2/api-platform/platform-api/internal/vault"
)

// Handle length limits, mirrored from v2 internal/utils/handle.go. The migration
// asserts against maxHandleLen defensively (§B.0 Gate 1 / §4): Prod is assumed
// clean, but a carried handle over 40 is quarantined, never truncated.
const (
	maxHandleLen = 40
	// maxIssuerLen / maxAllowedTargetsLen mirror v2 api_keys VARCHAR(255) (§B.13 ②).
	maxVarchar255 = 255
)

// Kernels holds the shared, source-verified transform primitives. It wraps v2's
// OWN helpers (imported, not re-implemented) so migrated bytes match v2's create
// path exactly: deterministic uuidv7, handle slugging, vault AES-GCM + HMAC.
type Kernels struct {
	cfg   *Config
	log   *slog.Logger
	cp    *Checkpoint
	vault *vault.InHouseVault // nil in a source-only dry-run without a key

	actorMu    sync.Mutex
	actorCache map[string]string // v1 actor string -> resolved v2 user uuid
	actorSeed  bool              // migration-actor idp ref seeded this run

	claimMu sync.Mutex
	claims  map[string]map[string]string // resource+scope -> handle -> owning sourceID, this run
}

// NewKernels builds the kernel set. vault may be nil when no encryption key is
// configured (only valid for a source-only dry-run / verify).
func NewKernels(cfg *Config, log *slog.Logger, cp *Checkpoint) (*Kernels, error) {
	k := &Kernels{
		cfg:        cfg,
		log:        log,
		cp:         cp,
		actorCache: map[string]string{},
		claims:     map[string]map[string]string{},
	}
	if len(cfg.EncryptionKey) == 32 {
		v, err := vault.NewInHouseVault(cfg.EncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("init vault: %w", err)
		}
		k.vault = v
	}
	return k, nil
}

// ---------------------------------------------------------------------------
// Kernel 1: resolveActor — v1 creator string -> v2 internal user UUID (§B.4)
// ---------------------------------------------------------------------------

// migrationActorIdpID is the readable idp_id seeded for the synthetic migration
// actor, so v2 audit resolution (GetSubByUUID) renders a stable name for NEW
// audit columns that had no v1 source.
const migrationActorIdpID = "platform-api-migration"

// actorSeedEpoch is the FIXED timestamp used for every deterministic actor UUID.
// Using a constant (not the row's created_at) makes the "one uuid per distinct
// actor" mapping order-independent and stable across runs — the invariant the
// Explorer shows (§B.4/§B.9). The idp-ref row's created_at uses it too.
var actorSeedEpoch = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

// actorUUID computes the deterministic v2 user UUID for a v1 actor string.
func actorUUID(actor string) string {
	return utils.GenerateDeterministicUUIDv7(actor, actorSeedEpoch)
}

// seedMigrationActor inserts the synthetic migration-actor idp reference. Called
// once up front (committed) by the user_idp_references migrator so the FK target
// for every NEW *_by column exists before any dependent row.
func (k *Kernels) seedMigrationActor(ctx context.Context, q queryer) error {
	if q == nil {
		return nil
	}
	if _, err := q.ExecContext(ctx,
		`INSERT INTO user_idp_references (uuid, idp_id, created_at)
		 VALUES ($1, $2, $3) ON CONFLICT (uuid) DO NOTHING`,
		k.cfg.MigrationActorUUID, migrationActorIdpID, actorSeedEpoch); err != nil {
		return fmt.Errorf("seed migration actor idp ref: %w", err)
	}
	k.actorMu.Lock()
	k.actorSeed = true
	k.actorMu.Unlock()
	return nil
}

// seedActor inserts one distinct v1-actor idp reference (deterministic uuid,
// UNIQUE idp_id) and caches it. Called by the up-front user_idp_references pass.
func (k *Kernels) seedActor(ctx context.Context, q queryer, actor string) error {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return nil
	}
	u := actorUUID(actor)
	if q != nil {
		if _, err := q.ExecContext(ctx,
			`INSERT INTO user_idp_references (uuid, idp_id, created_at)
			 VALUES ($1, $2, $3) ON CONFLICT (idp_id) DO NOTHING`,
			u, actor, actorSeedEpoch); err != nil {
			return fmt.Errorf("seed user_idp_references for actor: %w", err)
		}
	}
	k.actorMu.Lock()
	k.actorCache[actor] = u
	k.actorMu.Unlock()
	return nil
}

// resolveActor maps a raw v1 actor string to a v2 user_idp_references.uuid. An
// empty actor resolves to the migration actor. Real actors are expected to have
// been seeded up front (committed) by the user_idp_references migrator, so this
// is primarily a cache/lookup; as a belt-and-suspenders it will seed in-tx if an
// actor was somehow missed (§B.4).
func (k *Kernels) resolveActor(ctx context.Context, q queryer, actor string) (string, error) {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return k.cfg.MigrationActorUUID, nil
	}
	k.actorMu.Lock()
	if u, ok := k.actorCache[actor]; ok {
		k.actorMu.Unlock()
		return u, nil
	}
	k.actorMu.Unlock()

	// Fallback: not pre-seeded — seed now (deterministic, idempotent).
	if err := k.seedActor(ctx, q, actor); err != nil {
		return "", err
	}
	return actorUUID(actor), nil
}

// ---------------------------------------------------------------------------
// Kernel 2: handle mint / carry (§B.2, §B.12)
// ---------------------------------------------------------------------------

// mintHandle generates a v2 handle for a table with no v1 handle column
// (projects, gateways, subscription_plans, api_keys). It is checkpointed by
// (resource, sourceID) because GenerateHandle's collision path appends a random
// suffix (non-deterministic) — a resumed run must reuse the first choice or it
// orphans child rows / secret refs (§B.2).
//
// existsCheck queries the target for an already-taken handle in this table; pass
// nil to skip (e.g. dry-run) — the base slug is then returned without a suffix.
func (k *Kernels) mintHandle(resource, sourceID, source string, existsCheck func(string) bool) (string, error) {
	if h, ok := k.cp.Handle(resource, sourceID); ok {
		return h, nil
	}
	h, err := utils.GenerateHandle(source, existsCheck)
	if err != nil {
		return "", fmt.Errorf("mint handle for %s/%s: %w", resource, sourceID, err)
	}
	if err := k.cp.PutHandle(resource, sourceID, h); err != nil {
		return "", err
	}
	return h, nil
}

// mintScopedHandle mints (or reuses the checkpointed) handle for a row whose
// handle must be unique within scope (an org, or an artifact for api_keys). The
// target probe cannot see rows still uncommitted in the open batch, and is nil
// in a dry-run, so handles claimed earlier in this run are tracked in memory and
// treated as taken. A checkpointed handle already claimed by another row is a
// hard error, never re-minted: rows may already be inserted under it (§B.2).
func (k *Kernels) mintScopedHandle(resource, scope, sourceID, source string, dbExists func(string) bool) (string, error) {
	k.claimMu.Lock()
	defer k.claimMu.Unlock()
	key := resource + "\x00" + scope
	owners := k.claims[key]
	if owners == nil {
		owners = map[string]string{}
		k.claims[key] = owners
	}
	taken := func(h string) bool {
		if owner, ok := owners[h]; ok && owner != sourceID {
			return true
		}
		return dbExists != nil && dbExists(h)
	}
	h, err := k.mintHandle(resource, sourceID, source, taken)
	if err != nil {
		return "", err
	}
	if owner, ok := owners[h]; ok && owner != sourceID {
		return "", fmt.Errorf("handle %q for %s %s is already assigned to %s in scope %s — "+
			"it came from the checkpoint (fresh mints never collide), so the checkpoint belongs to another run: "+
			"use a NEW --checkpoint-file for a new migration (§B.2)",
			h, resource, sourceID, owner, scope)
	}
	owners[h] = sourceID
	return h, nil
}

// carryHandle returns a v1 handle verbatim — NEVER re-slugged (Choreo APIM
// addresses resources by it; the handle IS Choreo's UUID, §B.12). It defensively
// asserts the v2 VARCHAR(40) limit (§B.0 Gate 1 / §4): Prod is assumed clean, so
// an offender is a loud error to quarantine, not a value to truncate.
func (k *Kernels) carryHandle(resource, sourceID, v1Handle string) (string, error) {
	if l := len(v1Handle); l > maxHandleLen {
		return "", fmt.Errorf("QUARANTINE %s/%s: handle %q is %d chars > v2 limit %d — "+
			"cannot be truncated (external id); resolve in v1 before migrating (§B.0 Gate 1)",
			resource, sourceID, v1Handle, l, maxHandleLen)
	}
	return v1Handle, nil
}

// assertVarchar255 enforces the §B.13 ② functional-carry rule for
// api_keys.issuer / allowed_targets: carry verbatim, FAIL if >255, never truncate.
func assertVarchar255(field, sourceID, val string) error {
	if l := len(val); l > maxVarchar255 {
		return fmt.Errorf("QUARANTINE api_keys/%s: %s is %d chars > v2 limit %d — "+
			"functional value, never truncate; resolve in v1 first (§B.13 ②)", sourceID, field, l, maxVarchar255)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Kernel 3: TIMESTAMP -> TIMESTAMPTZ (§B.3)
// ---------------------------------------------------------------------------

// tsToTstz reinterprets a naive v1 TIMESTAMP (read through the pgx codec as
// wall-clock digits labelled UTC) in its true source zone, returning the real
// instant. Callers pass the v1 app zone for the ~130 bare time.Now() columns, and
// time.UTC for the 4 tables v1 wrote with .UTC() (artifacts/mcp_proxies/
// websub_apis/webbroker_apis) — the choice keys off the writer, not a global flag.
func tsToTstz(t time.Time, sourceZone *time.Location) time.Time {
	if sourceZone == nil {
		sourceZone = time.UTC
	}
	// Rebuild the same wall-clock digits in the source zone, then normalise to UTC.
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), sourceZone).UTC()
}

// tsToTstzPtr is the nullable variant.
func tsToTstzPtr(t *time.Time, sourceZone *time.Location) *time.Time {
	if t == nil {
		return nil
	}
	v := tsToTstz(*t, sourceZone)
	return &v
}

// appZone / utcZone name the two source zones for readability at call sites.
func (k *Kernels) appZone() *time.Location { return k.cfg.V1Location }
func utcZone() *time.Location              { return time.UTC }

// ---------------------------------------------------------------------------
// Kernel: deterministic secret uuid/handle + HMAC hash (§B.10)
// ---------------------------------------------------------------------------

// secretHash reproduces v2 secret_service.hashSecret exactly:
// "hmac-sha256:" + hex(HMAC-SHA256(vaultHashKey, plaintext)). The HMAC key is the
// vault key (vault.HashKey() returns the raw encryption key).
func secretHash(hashKey []byte, plaintext string) string {
	mac := hmac.New(sha256.New, hashKey)
	mac.Write([]byte(plaintext))
	return fmt.Sprintf("hmac-sha256:%x", mac.Sum(nil))
}

// secretHandle mints the deterministic UUID-form handle for an externalized
// inline secret, keyed on artifact_uuid|field so it is stable across resumes
// (§B.10 step 2). Fits VARCHAR(40) (36-char uuid).
func secretHandle(artifactUUID, field string, ts time.Time) string {
	return utils.GenerateDeterministicUUIDv7(artifactUUID+"|"+field, ts)
}

// secretUUID mints the deterministic PK for a secrets row (distinct namespace
// from the handle so the two never collide).
func secretUUID(artifactUUID, field string, ts time.Time) string {
	return utils.GenerateDeterministicUUIDv7("secret-pk|"+artifactUUID+"|"+field, ts)
}

// detUUID is a thin wrapper over v2's deterministic uuidv7 so resource files
// don't each import internal/utils.
func detUUID(key string, ts time.Time) string {
	return utils.GenerateDeterministicUUIDv7(key, ts)
}
