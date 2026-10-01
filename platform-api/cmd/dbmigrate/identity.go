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

package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// migrationActorIDPID is the synthetic IdP identity used to attribute rows whose
// v1 audit value is NULL/empty. It is seeded into user_idp_references like any
// other identity so it resolves to a real principal (not DeletedUser).
const migrationActorIDPID = "migration"

// v1 tables that carry a raw created_by identity string (verified against the v1
// schema). v1 has no updated_by columns.
var v1CreatedByTables = []string{
	"applications", "rest_apis", "llm_provider_templates", "llm_providers",
	"llm_proxies", "mcp_proxies", "websub_apis", "webbroker_apis", "api_keys",
}

// IdentityMap maps raw v1 audit identity strings to internal v2 user UUIDs
// (§C.1). UUIDs are deterministic (GenerateDeterministicUUIDv7), so seeding and
// rewriting are idempotent/resumable with no persisted state.
type IdentityMap struct {
	epoch     time.Time
	actorUUID string
	m         map[string]string // idp_id -> uuid
}

// resolve maps a raw v1 identity to its internal UUID, registering it on first
// sight. An empty/blank identity maps to the migration-actor UUID and reports
// defaulted=true (the caller records a DEFAULTED_NULL flag).
func (im *IdentityMap) resolve(raw string) (uuid string, defaulted bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return im.actorUUID, true
	}
	if u, ok := im.m[raw]; ok {
		return u, false
	}
	u := utils.GenerateDeterministicUUIDv7(raw, im.epoch)
	im.m[raw] = u
	return u, false
}

// collectAndSeedIdentities scans every v1 audit column for distinct identities,
// builds the identity→uuid map (including the migration actor), and — unless this
// is a dry-run — seeds one user_idp_references row per identity FIRST in FK order.
// Each seeded identity is recorded as SYNTHESIZED (fabricated UUID).
func collectAndSeedIdentities(v1, v2 *database.DB, run *Run, epoch time.Time) (*IdentityMap, error) {
	im := &IdentityMap{epoch: epoch, m: map[string]string{}}
	im.actorUUID = utils.GenerateDeterministicUUIDv7(migrationActorIDPID, epoch)
	im.m[migrationActorIDPID] = im.actorUUID

	distinct := map[string]bool{}
	for _, t := range v1CreatedByTables {
		set, err := loadStringSet(v1, fmt.Sprintf(
			"SELECT DISTINCT created_by FROM %s WHERE created_by IS NOT NULL AND created_by <> ''", t))
		if err != nil {
			return nil, fmt.Errorf("collect identities from %s: %w", t, err)
		}
		for id := range set {
			distinct[id] = true
		}
	}

	// Register (compute deterministic UUIDs). Sort for stable logs/flag ordering.
	ids := make([]string, 0, len(distinct))
	for id := range distinct {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		im.resolve(id)
	}

	run.logger.Info("collected audit identities", "distinct", len(ids), "actor", migrationActorIDPID)

	// Seed user_idp_references FIRST (dependency-free) so the map exists before any
	// audited insert. created_at is pinned to the migration epoch for determinism.
	seedList := append([]string{migrationActorIDPID}, ids...)
	seeded := 0
	for _, idpID := range seedList {
		uuid := im.m[idpID]
		if len(idpID) > 200 {
			run.flag("user_idp_references", uuid, FlagTruncated,
				map[string]any{"len": len(idpID)}, map[string]any{"note": "raw identity > 200 chars; idp_id VARCHAR(255) holds it, but the source string is unusually long"})
		}
		if !run.opts.DryRun {
			if err := insertRow(v2, "user_idp_references",
				[]string{"uuid", "idp_id", "created_at"},
				[]any{uuid, idpID, epoch}, "(idp_id)"); err != nil {
				return nil, fmt.Errorf("seed user_idp_references %q: %w", idpID, err)
			}
		}
		run.flag("user_idp_references", uuid, FlagSynthesized,
			nil, map[string]any{"idp_id": idpID, "note": "fabricated internal user UUID (deterministic v7)"})
		seeded++
	}
	run.addProcessed("user_idp_references", seeded)
	run.markTableComplete("user_idp_references")
	return im, nil
}
