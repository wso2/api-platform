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
	"database/sql"
	"fmt"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// runPreflight enforces the §B.0 go/no-go gates against the live v1 DB before a
// single v2 row is written. Each gate silently corrupts data (not a loud error)
// if skipped, so "the load succeeded" is NOT evidence they passed. Any hard-gate
// failure aborts the run.
//
// v1DB is the source (v1) connection; v2DB the target (nil in a source-only
// dry-run — target-collision checks are then skipped). Called only for a
// forward run.
func runPreflight(ctx context.Context, cfg *Config, k *Kernels, v1DB, v2DB *sql.DB) error {
	k.log.Info("preflight: running §B.0 go/no-go gates against v1")

	if err := gateLengthLimits(ctx, k, v1DB); err != nil {
		return err
	}
	if err := gateDuplicateSubscriptions(ctx, k, v1DB); err != nil {
		return err
	}
	if err := gateHandleMaps(ctx, cfg, k, v1DB, v2DB); err != nil {
		return err
	}
	switch {
	case cfg.ValidateKeys && k.vault != nil:
		if err := gateVaultKeyRoundTrip(ctx, k); err != nil {
			return err
		}
		if err := gateSubscriptionTokenParity(ctx, cfg, k, v1DB); err != nil {
			return err
		}
	case cfg.ValidateKeys && k.vault == nil:
		// Only reachable in a source-only dry-run without a key configured.
		k.log.Warn("preflight: no encryption key configured — SKIPPING key gates (dry-run only); " +
			"a real run requires " + EnvV2EncryptionKey + " (§B.0 Gates 2 & 4)")
	default:
		k.log.Warn("preflight: --validate-keys=false — SKIPPING token-parity + vault round-trip gates (§B.0 2 & 4)")
	}

	k.log.Info("preflight: all gates passed")
	return nil
}

// gateLengthLimits is Gate 1 (§B.0.1 / §B.13): every value must fit v2's narrower
// columns. One scan over the 4 handle tables (≤40) + api_keys issuer/allowed_targets
// (≤255). Expect zero rows; any offender aborts (a handle cannot be re-slugged, an
// issuer/target must never be truncated).
func gateLengthLimits(ctx context.Context, k *Kernels, v1 *sql.DB) error {
	const q = `
SELECT 'handle' AS col, 'organizations' AS tbl, length(handle) AS len, 40 AS max FROM organizations WHERE length(handle) > 40
UNION ALL SELECT 'handle','applications',           length(handle),          40  FROM applications           WHERE length(handle) > 40
UNION ALL SELECT 'handle','artifacts',              length(handle),          40  FROM artifacts              WHERE length(handle) > 40
UNION ALL SELECT 'handle','llm_provider_templates', length(handle),          40  FROM llm_provider_templates WHERE length(handle) > 40
UNION ALL SELECT 'issuer','api_keys',               length(issuer),          255 FROM api_keys               WHERE length(issuer) > 255
UNION ALL SELECT 'allowed_targets','api_keys',      length(allowed_targets), 255 FROM api_keys               WHERE length(allowed_targets) > 255`
	rows, err := v1.QueryContext(ctx, q)
	if err != nil {
		return fmt.Errorf("gate 1 (length limits) scan: %w", err)
	}
	defer rows.Close()
	var offenders int
	for rows.Next() {
		var col, tbl string
		var length, max int
		if err := rows.Scan(&col, &tbl, &length, &max); err != nil {
			return fmt.Errorf("gate 1 scan row: %w", err)
		}
		offenders++
		k.log.Error("gate 1 offender", "table", tbl, "column", col, "length", length, "limit", max)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("gate 1 rows: %w", err)
	}
	if offenders > 0 {
		return fmt.Errorf("gate 1 FAILED: %d value(s) exceed v2 column widths — resolve in v1 "+
			"(handles cannot be re-slugged; issuer/allowed_targets must never be truncated) before migrating (§B.0 Gate 1)", offenders)
	}
	k.log.Info("gate 1 (length limits) passed — zero offenders")
	return nil
}

// gateDuplicateSubscriptions is Gate 3 (§B.0.3): v2 adds a partial-unique index on
// (organization_uuid, artifact_uuid, application_id) WHERE application_id IS NOT
// NULL that v1 never enforced. Pre-scan v1; any collision would fail the second
// insert of the pair.
func gateDuplicateSubscriptions(ctx context.Context, k *Kernels, v1 *sql.DB) error {
	const q = `
SELECT organization_uuid, api_uuid, application_id, COUNT(*)
FROM subscriptions
WHERE application_id IS NOT NULL
GROUP BY organization_uuid, api_uuid, application_id
HAVING COUNT(*) > 1`
	rows, err := v1.QueryContext(ctx, q)
	if err != nil {
		return fmt.Errorf("gate 3 (duplicate subscriptions) scan: %w", err)
	}
	defer rows.Close()
	var dupes int
	for rows.Next() {
		var org, api, app string
		var n int
		if err := rows.Scan(&org, &api, &app, &n); err != nil {
			return fmt.Errorf("gate 3 scan row: %w", err)
		}
		dupes++
		k.log.Error("gate 3 duplicate subscription", "org", org, "api", api, "application", app, "count", n)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("gate 3 rows: %w", err)
	}
	if dupes > 0 {
		return fmt.Errorf("gate 3 FAILED: %d duplicate subscription tuple(s) violate v2's partial-unique key — "+
			"resolve collisions in v1 before migrating (§B.0 Gate 3)", dupes)
	}
	k.log.Info("gate 3 (duplicate subscriptions) passed — zero collisions")
	return nil
}

// gateVaultKeyRoundTrip is Gate 4 (§B.0.4): the v2 vault key must be configured and
// functional (encrypt→decrypt + HMAC), or every externalized secret is written
// unusable.
func gateVaultKeyRoundTrip(ctx context.Context, k *Kernels) error {
	if k.vault == nil {
		return fmt.Errorf("gate 4 FAILED: %s is not configured — inline secrets would be written unusable (§B.0 Gate 4)", EnvV2EncryptionKey)
	}
	const probe = "migration-preflight-probe"
	ct, err := k.vault.Encrypt(ctx, probe)
	if err != nil {
		return fmt.Errorf("gate 4 FAILED: vault encrypt: %w", err)
	}
	pt, err := k.vault.Decrypt(ctx, ct)
	if err != nil {
		return fmt.Errorf("gate 4 FAILED: vault decrypt: %w", err)
	}
	if pt != probe {
		return fmt.Errorf("gate 4 FAILED: vault round-trip mismatch")
	}
	if h := secretHash(k.vault.HashKey(), probe); h == "" {
		return fmt.Errorf("gate 4 FAILED: empty secret hash")
	}
	k.log.Info("gate 4 (vault key round-trip) passed")
	return nil
}

// gateSubscriptionTokenParity is Gate 2 (§B.0.2 / §B.8): the copied
// subscription_token ciphertext only decrypts under the same 32 key bytes v1
// used. Per the decision v2 is configured with v1's key, so this is a WIRING
// smoke-test — sample-decrypt a handful of real v1 tokens with the configured key
// and abort if any fail (catches a mis-plumbed key/typo). Zero tokens → skip.
func gateSubscriptionTokenParity(ctx context.Context, cfg *Config, k *Kernels, v1 *sql.DB) error {
	rows, err := v1.QueryContext(ctx,
		`SELECT subscription_token FROM subscriptions
		 WHERE subscription_token IS NOT NULL AND subscription_token <> '' LIMIT 5`)
	if err != nil {
		return fmt.Errorf("gate 2 (token parity) sample: %w", err)
	}
	defer rows.Close()
	var sampled, ok int
	for rows.Next() {
		var ct string
		if err := rows.Scan(&ct); err != nil {
			return fmt.Errorf("gate 2 scan: %w", err)
		}
		sampled++
		if _, derr := utils.DecryptSubscriptionToken(cfg.EncryptionKey, ct); derr == nil {
			ok++
		} else {
			k.log.Error("gate 2 token failed to decrypt under configured key (do NOT log the token/plaintext)", "error", derr.Error())
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("gate 2 rows: %w", err)
	}
	if sampled == 0 {
		k.log.Warn("gate 2 (token parity): no subscription tokens present to sample — skipping (nothing to validate)")
		return nil
	}
	if ok != sampled {
		return fmt.Errorf("gate 2 FAILED: %d/%d sampled subscription tokens did not decrypt under %s — "+
			"the key is mis-plumbed or is not v1's key (§B.0 Gate 2). ⚠ If v1 used an auto-generated ephemeral "+
			"JWT secret, there is no key to carry and those tokens are unmigratable", sampled-ok, sampled, EnvV2EncryptionKey)
	}
	k.log.Info("gate 2 (subscription-token key parity) passed", "sampled", sampled)
	return nil
}

// ---------------------------------------------------------------------------
// Gate 5 — Choreo handle maps (§B.0.5 / §B.14)
// ---------------------------------------------------------------------------

// gateHandleMaps is Gate 5: the Choreo handle exports must cover the v1 rows and
// yield a collision-free, width-safe handle set BEFORE any write. Hard failures:
// a handle claimed twice (orgs globally, projects per org), or a target row that
// already holds a planned handle under a DIFFERENT uuid. Soft (WARN): rows the
// export does not know — orgs then keep v1's handle, projects mint from the v1
// name (v1's auto-created "default" projects are expected here: they are not
// Choreo projects) — and Choreo handles v2's own create-validation would reject
// (uppercase, '.'): written verbatim, since v2 serves reads/path params as stored.
func gateHandleMaps(ctx context.Context, cfg *Config, k *Kernels, v1, v2 *sql.DB) error {
	if cfg.OrgHandles == nil || cfg.ProjectHandles == nil {
		return fmt.Errorf("gate 5 FAILED: both --org-handles-csv and --project-handles-csv are required (§B.14)")
	}
	if err := gateOrgHandles(ctx, cfg.OrgHandles, k, v1, v2); err != nil {
		return err
	}
	if err := gateProjectHandles(ctx, cfg.ProjectHandles, k, v1, v2); err != nil {
		return err
	}
	k.log.Info("gate 5 (Choreo handle maps) passed")
	return nil
}

func gateOrgHandles(ctx context.Context, m *HandleMap, k *Kernels, v1, v2 *sql.DB) error {
	rows, err := v1.QueryContext(ctx, `SELECT uuid, handle FROM organizations`)
	if err != nil {
		return fmt.Errorf("gate 5 (org handles) scan: %w", err)
	}
	defer rows.Close()
	byHandle := map[string][]string{}     // final handle -> uuids
	planned := map[string]plannedHandle{} // uuid -> final handle
	var total, mapped, unmapped, badFmt, inactive int
	for rows.Next() {
		var uuid, v1Handle string
		if err := rows.Scan(&uuid, &v1Handle); err != nil {
			return fmt.Errorf("gate 5 scan row: %w", err)
		}
		total++
		h, ok := m.Lookup(uuid)
		if !ok {
			unmapped++
			h = v1Handle
			k.log.Warn("gate 5: organization not in the Choreo export — v1 handle kept, will NOT match any Choreo URL",
				"org", uuid, "v1Handle", v1Handle)
		} else {
			mapped++
			if issue := handleFormatIssue(h); issue != "" {
				badFmt++
				k.log.Warn("gate 5: Choreo org handle fails v2's create-validation (written verbatim; reads are served as stored)",
					"org", uuid, "handle", h, "issue", issue)
			}
			if s := m.StatusOf(uuid); s != "" && !strings.EqualFold(s, "ACTIVE") {
				inactive++
				k.log.Warn("gate 5: organization maps to a non-ACTIVE Choreo org", "org", uuid, "handle", h, "status", s)
			}
		}
		byHandle[h] = append(byHandle[h], uuid)
		planned[uuid] = plannedHandle{handle: h}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("gate 5 rows: %w", err)
	}
	var dups int
	for h, uuids := range byHandle {
		if len(uuids) > 1 {
			dups++
			k.log.Error("gate 5: organization handle claimed by several orgs", "handle", h, "orgs", uuids)
		}
	}
	if dups > 0 {
		return fmt.Errorf("gate 5 FAILED: %d organization handle(s) are claimed by more than one org (organizations.handle is UNIQUE) — fix the export (§B.0 Gate 5)", dups)
	}
	if v2 != nil {
		conflicts, stale, err := targetHandleConflicts(ctx, v2, "organizations", "", planned)
		if err != nil {
			return fmt.Errorf("gate 5 (org handles) target check: %w", err)
		}
		for _, c := range conflicts {
			k.log.Error("gate 5: target already holds a planned org handle under another uuid", "handle", c.handle, "targetUUID", c.tgtUUID, "plannedFor", c.uuid)
		}
		for _, s := range stale {
			k.log.Warn("gate 5: target org already migrated with a different handle — insert-only, NOT updated", "org", s.uuid, "targetHandle", s.tgtHandle, "exportHandle", s.handle)
		}
		if len(conflicts) > 0 {
			return fmt.Errorf("gate 5 FAILED: %d planned organization handle(s) already exist in the target under a different uuid", len(conflicts))
		}
	}
	k.log.Info("gate 5 organizations", "v1Orgs", total, "fromChoreoExport", mapped, "unmappedFallback", unmapped,
		"formatWarnings", badFmt, "nonActiveChoreoOrgs", inactive, "exportRows", m.Len())
	return nil
}

func gateProjectHandles(ctx context.Context, m *HandleMap, k *Kernels, v1, v2 *sql.DB) error {
	rows, err := v1.QueryContext(ctx, `SELECT uuid, name, organization_uuid FROM projects`)
	if err != nil {
		return fmt.Errorf("gate 5 (project handles) scan: %w", err)
	}
	defer rows.Close()
	type pr struct{ uuid, name, org string }
	var unmappedRows []pr
	claimed := map[string]map[string][]string{} // org -> Choreo handle -> uuids
	planned := map[string]plannedHandle{}       // uuid -> (org, handle), Choreo-mapped only
	var total, mapped, badFmt int
	for rows.Next() {
		var r pr
		if err := rows.Scan(&r.uuid, &r.name, &r.org); err != nil {
			return fmt.Errorf("gate 5 scan row: %w", err)
		}
		total++
		h, ok := m.Lookup(r.uuid)
		if !ok {
			unmappedRows = append(unmappedRows, r)
			k.log.Debug("gate 5: project not in the Choreo export — handle will be minted from the v1 name (suffixed if it collides with a Choreo handle in the org)",
				"project", r.uuid, "v1Name", r.name, "org", r.org)
			continue
		}
		mapped++
		if issue := handleFormatIssue(h); issue != "" {
			badFmt++
			k.log.Warn("gate 5: Choreo project handle fails v2's create-validation (written verbatim; reads are served as stored)",
				"project", r.uuid, "handle", h, "issue", issue)
		}
		if claimed[r.org] == nil {
			claimed[r.org] = map[string][]string{}
		}
		claimed[r.org][h] = append(claimed[r.org][h], r.uuid)
		planned[r.uuid] = plannedHandle{org: r.org, handle: h}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("gate 5 rows: %w", err)
	}
	var dups int
	for org, hs := range claimed {
		for h, uuids := range hs {
			if len(uuids) > 1 {
				dups++
				k.log.Error("gate 5: Choreo project handle claimed by several projects in one org", "org", org, "handle", h, "projects", uuids)
			}
		}
	}
	if dups > 0 {
		return fmt.Errorf("gate 5 FAILED: %d Choreo project handle(s) are claimed twice within an org (projects UNIQUE(organization_uuid, handle)) — fix the export (§B.0 Gate 5)", dups)
	}
	var willSuffix int
	for _, r := range unmappedRows {
		if base, err := utils.GenerateHandle(r.name, nil); err == nil && claimed[r.org][base] != nil {
			willSuffix++
		}
	}
	if v2 != nil {
		conflicts, stale, err := targetHandleConflicts(ctx, v2, "projects", "organization_uuid", planned)
		if err != nil {
			return fmt.Errorf("gate 5 (project handles) target check: %w", err)
		}
		for _, c := range conflicts {
			k.log.Error("gate 5: target already holds a planned project handle under another uuid", "handle", c.handle, "targetUUID", c.tgtUUID, "plannedFor", c.uuid)
		}
		for _, s := range stale {
			k.log.Warn("gate 5: target project already migrated with a different handle — insert-only, NOT updated", "project", s.uuid, "targetHandle", s.tgtHandle, "exportHandle", s.handle)
		}
		if len(conflicts) > 0 {
			return fmt.Errorf("gate 5 FAILED: %d planned project handle(s) already exist in the target under a different uuid", len(conflicts))
		}
	}
	k.log.Info("gate 5 projects", "v1Projects", total, "fromChoreoExport", mapped, "unmappedFallback", len(unmappedRows),
		"willBeSuffixed", willSuffix, "formatWarnings", badFmt, "exportRows", m.Len(), "exportEmptyHandlers", m.EmptyCount())
	return nil
}

type plannedHandle struct{ org, handle string }
type handleConflict struct{ uuid, tgtUUID, handle string }
type handleStale struct{ uuid, tgtHandle, handle string }

// targetHandleConflicts compares planned handles against rows already in the
// target: a planned (org, handle) held by a DIFFERENT uuid is a conflict (the
// insert would violate the UNIQUE key); a planned uuid already present with a
// different handle is stale (insert-only: it will not be updated). The whole
// target table is read — it is at most the size of the v1 table.
func targetHandleConflicts(ctx context.Context, db *sql.DB, table, orgCol string, planned map[string]plannedHandle) ([]handleConflict, []handleStale, error) {
	q := "SELECT uuid, handle FROM " + table
	if orgCol != "" {
		q = "SELECT uuid, handle, " + orgCol + " FROM " + table
	}
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	byKey := make(map[string]string, len(planned)) // org\x1fhandle -> planned uuid
	for u, p := range planned {
		byKey[p.org+"\x1f"+p.handle] = u
	}
	var conflicts []handleConflict
	var stale []handleStale
	for rows.Next() {
		var uuid, handle, org string
		if orgCol != "" {
			err = rows.Scan(&uuid, &handle, &org)
		} else {
			err = rows.Scan(&uuid, &handle)
		}
		if err != nil {
			return nil, nil, err
		}
		if pu, ok := byKey[org+"\x1f"+handle]; ok && pu != uuid {
			conflicts = append(conflicts, handleConflict{uuid: pu, tgtUUID: uuid, handle: handle})
		}
		if p, ok := planned[uuid]; ok && p.handle != handle {
			stale = append(stale, handleStale{uuid: uuid, tgtHandle: handle, handle: p.handle})
		}
	}
	return conflicts, stale, rows.Err()
}
