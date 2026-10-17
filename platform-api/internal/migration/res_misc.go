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
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
)

func init() {
	register(&apiKeysMigrator{baseMigrator{name: "api_keys", dependsOn: []string{"user_idp_references", "artifacts"}}})
	register(&deploymentsMigrator{baseMigrator{name: "deployments", dependsOn: []string{"artifacts", "gateways"}}})
	register(&deploymentStatusMigrator{baseMigrator{name: "deployment_status", dependsOn: []string{"artifacts", "gateways"}}})
	register(&artifactGatewayMappingsMigrator{baseMigrator{name: "artifact_gateway_mappings", dependsOn: []string{"artifacts", "gateways"}}})
	register(&appArtifactMappingsMigrator{baseMigrator{name: "application_artifact_mappings", dependsOn: []string{"applications", "api_keys"}}})
	register(&appAPIKeyMappingsMigrator{baseMigrator{name: "application_api_key_mappings", dependsOn: []string{"applications", "api_keys"}}})
}

// ---------------------------------------------------------------------------
// api_keys — handle MINT from name; api_key_hashes TEXT->BYTEA opaque; issuer /
// allowed_targets carried verbatim, FAIL if >255 (§B.13 ②).
// ---------------------------------------------------------------------------

type apiKeysMigrator struct{ baseMigrator }

type apiKeyRow struct {
	uuid, artifactUUID, name string
	maskedAPIKey             sql.NullString
	apiKeyHashes             sql.NullString
	status                   sql.NullString
	createdAt                time.Time
	createdBy                sql.NullString
	updatedAt                time.Time
	expiresAt                sql.NullTime
	issuer                   sql.NullString
	allowedTargets           sql.NullString
}

func (m *apiKeysMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT uuid, artifact_uuid, name, masked_api_key, api_key_hashes, status, created_at, created_by, updated_at, expires_at, issuer, allowed_targets
		 FROM api_keys`)
	if err != nil {
		return nil, err
	}
	var items []apiKeyRow
	for rows.Next() {
		var r apiKeyRow
		if err := rows.Scan(&r.uuid, &r.artifactUUID, &r.name, &r.maskedAPIKey, &r.apiKeyHashes, &r.status,
			&r.createdAt, &r.createdBy, &r.updatedAt, &r.expiresAt, &r.issuer, &r.allowedTargets); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	rep.SrcCount = int64(len(items))

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r apiKeyRow) error {
		if r.issuer.Valid {
			if err := assertVarchar255("issuer", r.uuid, r.issuer.String); err != nil {
				return err
			}
		}
		if r.allowedTargets.Valid {
			if err := assertVarchar255("allowed_targets", r.uuid, r.allowedTargets.String); err != nil {
				return err
			}
		}
		handle, err := rc.Kernels.mintScopedHandle("api_keys", r.artifactUUID, r.uuid, r.name,
			rc.handleExistsChecker(ctx, "api_keys", "artifact_uuid", r.artifactUUID))
		if err != nil {
			return err
		}
		if handle != r.name {
			// A gateway identifies a key by the name the control plane first sent it,
			// the v1 name; v2 addresses it by the handle. A key whose name does not
			// survive the mint (longer than 40 chars, or suffixed for uniqueness) can
			// then neither be re-synced nor revoked on the gateways that hold it.
			rep.warn("api_key %s on artifact %s: v1 name %q becomes handle %q; gateways keep it under the v1 name, so it cannot be revoked from v2 — recreate it after cut-over (or revoke it before)",
				r.uuid, r.artifactUUID, r.name, handle)
		}
		actor, err := rc.Kernels.resolveActor(ctx, q, nullToString(r.createdBy))
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "api_keys",
			[]string{"uuid", "artifact_uuid", "handle", "display_name", "masked_api_key", "api_key_hashes", "status",
				"created_at", "created_by", "updated_at", "updated_by", "expires_at", "issuer", "allowed_targets"},
			[]any{r.uuid, r.artifactUUID, handle, r.name, nullOrString(r.maskedAPIKey), textToBytea(r.apiKeyHashes), nullOrString(r.status),
				tsToTstz(r.createdAt, rc.Kernels.appZone()), actor, tsToTstz(r.updatedAt, rc.Kernels.appZone()), actor,
				tstzPtrArg(r.expiresAt, rc.Kernels.appZone()), nullOrString(r.issuer), nullOrString(r.allowedTargets)},
			conflictUUIDNothing)
	})
	return rep, err
}

func (m *apiKeysMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM api_keys", "SELECT COUNT(*) FROM api_keys", false)
	return rep, nil
}

// ---------------------------------------------------------------------------
// deployments — id->uuid rename; content BYTEA passthrough; metadata TEXT->BYTEA
// opaque; created_by v2-NEW. Inserted parent-first on base_deployment_uuid.
// ---------------------------------------------------------------------------

type deploymentsMigrator struct{ baseMigrator }

type deploymentRow struct {
	deploymentID, artifactUUID, orgUUID string
	name                                sql.NullString
	gatewayUUID                         sql.NullString
	baseDeploymentID                    sql.NullString
	content                             []byte
	metadata                            []byte
	createdAt                           time.Time
}

func (m *deploymentsMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT deployment_id, name, artifact_uuid, organization_uuid, gateway_uuid, base_deployment_id, content, metadata, created_at
		 FROM deployments ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	var items []deploymentRow
	for rows.Next() {
		var r deploymentRow
		if err := rows.Scan(&r.deploymentID, &r.name, &r.artifactUUID, &r.orgUUID, &r.gatewayUUID, &r.baseDeploymentID, &r.content, &r.metadata, &r.createdAt); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	rep.SrcCount = int64(len(items))
	if items, err = orderDeploymentsParentFirst(items); err != nil {
		return nil, err
	}

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r deploymentRow) error {
		actor, err := rc.Kernels.resolveActor(ctx, q, "") // created_by v2-NEW
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "deployments",
			[]string{"uuid", "display_name", "artifact_uuid", "organization_uuid", "gateway_uuid", "base_deployment_uuid", "content", "metadata", "created_by", "created_at"},
			[]any{r.deploymentID, nullOrString(r.name), r.artifactUUID, r.orgUUID, nullOrString(r.gatewayUUID), nullOrString(r.baseDeploymentID),
				nullOrBytes(r.content), nullOrBytes(r.metadata), actor, tsToTstz(r.createdAt, rc.Kernels.appZone())},
			conflictUUIDNothing)
	})
	return rep, err
}

// orderDeploymentsParentFirst reorders items so every base deployment precedes
// the deployments built on it, keeping the created_at order otherwise. A base
// outside the set (an orphan, or a row migrated earlier) imposes no order. A
// base_deployment_id cycle is an error: no insert order can satisfy it.
func orderDeploymentsParentFirst(items []deploymentRow) ([]deploymentRow, error) {
	const (
		unvisited = iota
		visiting
		done
	)
	idx := make(map[string]int, len(items))
	for i, r := range items {
		idx[r.deploymentID] = i
	}
	state := make([]int, len(items))
	out := make([]deploymentRow, 0, len(items))
	var visit func(i int) error
	visit = func(i int) error {
		switch state[i] {
		case done:
			return nil
		case visiting:
			return fmt.Errorf("base_deployment_id cycle through deployment %s", items[i].deploymentID)
		}
		state[i] = visiting
		if b := items[i].baseDeploymentID; b.Valid {
			if j, ok := idx[b.String]; ok {
				if err := visit(j); err != nil {
					return err
				}
			}
		}
		state[i] = done
		out = append(out, items[i])
		return nil
	}
	for i := range items {
		if err := visit(i); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (m *deploymentsMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM deployments", "SELECT COUNT(*) FROM deployments", false)
	return rep, nil
}

// ---------------------------------------------------------------------------
// deployment_status — upsert on natural triple; deployment_id->deployment_uuid;
// performed_by v2-NEW.
// ---------------------------------------------------------------------------

type deploymentStatusMigrator struct{ baseMigrator }

type deploymentStatusRow struct {
	artifactUUID, orgUUID, gatewayUUID string
	deploymentID                       sql.NullString
	status                             sql.NullString
	statusDesired                      sql.NullString
	performedAt                        sql.NullTime
	statusReason                       sql.NullString
	updatedAt                          sql.NullTime
}

func (m *deploymentStatusMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT artifact_uuid, organization_uuid, gateway_uuid, deployment_id, status, status_desired, performed_at, status_reason, updated_at
		 FROM deployment_status`)
	if err != nil {
		return nil, err
	}
	var items []deploymentStatusRow
	for rows.Next() {
		var r deploymentStatusRow
		if err := rows.Scan(&r.artifactUUID, &r.orgUUID, &r.gatewayUUID, &r.deploymentID, &r.status, &r.statusDesired, &r.performedAt, &r.statusReason, &r.updatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	rep.SrcCount = int64(len(items))

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r deploymentStatusRow) error {
		actor, err := rc.Kernels.resolveActor(ctx, q, "") // performed_by v2-NEW
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "deployment_status",
			[]string{"artifact_uuid", "organization_uuid", "gateway_uuid", "deployment_uuid", "status", "status_desired", "performed_at", "performed_by", "status_reason", "updated_at"},
			[]any{r.artifactUUID, r.orgUUID, r.gatewayUUID, nullOrString(r.deploymentID), nullOrString(r.status), nullOrString(r.statusDesired),
				tstzPtrArg(r.performedAt, rc.Kernels.appZone()), actor, nullOrString(r.statusReason), tstzPtrArg(r.updatedAt, rc.Kernels.appZone())},
			conflictCols("organization_uuid", "artifact_uuid", "gateway_uuid"))
	})
	return rep, err
}

func (m *deploymentStatusMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM deployment_status", "SELECT COUNT(*) FROM deployment_status", false)
	return rep, nil
}

// ---------------------------------------------------------------------------
// artifact_gateway_mappings — from v1 association_mappings WHERE
// association_type='gateway'; resource_uuid->gateway_uuid; metadata v2-NEW/NULL.
// ---------------------------------------------------------------------------

type artifactGatewayMappingsMigrator struct{ baseMigrator }

type assocRow struct {
	artifactUUID, orgUUID, resourceUUID string
	createdAt, updatedAt                sql.NullTime
}

func (m *artifactGatewayMappingsMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT artifact_uuid, organization_uuid, resource_uuid, created_at, updated_at
		 FROM association_mappings WHERE association_type = 'gateway'`)
	if err != nil {
		return nil, err
	}
	var items []assocRow
	for rows.Next() {
		var r assocRow
		if err := rows.Scan(&r.artifactUUID, &r.orgUUID, &r.resourceUUID, &r.createdAt, &r.updatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	rep.SrcCount = int64(len(items))

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r assocRow) error {
		actor, err := rc.Kernels.resolveActor(ctx, q, "") // created_by/updated_by v2-NEW
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "artifact_gateway_mappings",
			[]string{"artifact_uuid", "organization_uuid", "gateway_uuid", "created_by", "updated_by", "created_at", "updated_at"},
			[]any{r.artifactUUID, r.orgUUID, r.resourceUUID, actor, actor,
				tstzPtrArg(r.createdAt, rc.Kernels.appZone()), tstzPtrArg(r.updatedAt, rc.Kernels.appZone())},
			conflictCols("organization_uuid", "artifact_uuid", "gateway_uuid"))
	})
	return rep, err
}

func (m *artifactGatewayMappingsMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep,
		"SELECT COUNT(*) FROM association_mappings WHERE association_type = 'gateway'",
		"SELECT COUNT(*) FROM artifact_gateway_mappings", false)
	return rep, nil
}

// ---------------------------------------------------------------------------
// application_artifact_mappings — DERIVED from application_api_keys ⋈ api_keys,
// not copied from v1 application_artifacts. v1 fills that table only through
// POST /applications/{id}/associations, which AI Workspace v1 never calls, so it
// is empty or stale (the dev dump: 2 rows against 10 derivable pairs) and older
// v1 schemas do not have it at all. v2's Associations tab lists an application's
// artifacts from this table and resolves its keys per association, so every
// (application, artifact) pair a genai application holds a key for gets one
// row; created_at is the earliest such key mapping. The derivation follows
// v2's own association rules (service/application.go): only genai applications
// have the tab, and only LLM providers and LLM proxies can be associated — key
// mappings outside those rules are reported and skipped.
// ---------------------------------------------------------------------------

type appArtifactMappingsMigrator struct{ baseMigrator }

type appArtifactRow struct {
	applicationUUID, artifactUUID string
	createdAt                     sql.NullTime
}

// genaiApplicationType is the applications.type whose associations are derived
// (api.Genai; the Gen AI apps of AI Workspace).
const genaiApplicationType = "genai"

// appArtifactRuleSQL is v2's association rule (service/application.go
// validateAssociationTargetForApplication) as a predicate over v1 rows: a genai
// application; a key of an LLM provider or LLM proxy (v1 artifacts.kind) in the
// application's organization; and an LLM proxy only from the application's project.
const appArtifactRuleSQL = `a.type = '` + genaiApplicationType + `'
	AND art.organization_uuid = a.organization_uuid
	AND art.kind IN ('` + constants.LLMProvider + `', '` + constants.LLMProxy + `')
	AND (art.kind <> '` + constants.LLMProxy + `' OR EXISTS (
		SELECT 1 FROM llm_proxies p WHERE p.uuid = art.uuid AND p.project_uuid = a.project_uuid))`

// appArtifactSkipReasonSQL names the first rule a skipped key mapping breaks.
const appArtifactSkipReasonSQL = `CASE
	WHEN a.type <> '` + genaiApplicationType + `' THEN 'not a genai application'
	WHEN art.organization_uuid <> a.organization_uuid THEN 'the artifact belongs to another organization'
	WHEN art.kind NOT IN ('` + constants.LLMProvider + `', '` + constants.LLMProxy + `') THEN 'v2 associates only LLM providers and LLM proxies'
	ELSE 'the LLM proxy is in a different project than the application'
	END`

// appArtifactPairsQuery lists every key mapping that satisfies the rule with the
// artifact its key belongs to, oldest first, so the first row seen per pair
// carries the earliest created_at.
const appArtifactPairsQuery = `SELECT m.application_uuid, k.artifact_uuid, m.created_at
	FROM application_api_keys m
	INNER JOIN applications a ON a.uuid = m.application_uuid
	INNER JOIN api_keys k ON k.uuid = m.api_key_id
	INNER JOIN artifacts art ON art.uuid = k.artifact_uuid
	WHERE ` + appArtifactRuleSQL + `
	ORDER BY m.created_at, m.application_uuid, k.artifact_uuid`

// appArtifactPairsCountQuery counts the distinct pairs the query above yields.
const appArtifactPairsCountQuery = `SELECT COUNT(*) FROM (
	SELECT DISTINCT m.application_uuid, k.artifact_uuid
	FROM application_api_keys m
	INNER JOIN applications a ON a.uuid = m.application_uuid
	INNER JOIN api_keys k ON k.uuid = m.api_key_id
	INNER JOIN artifacts art ON art.uuid = k.artifact_uuid
	WHERE ` + appArtifactRuleSQL + `) p`

// appArtifactSkippedQuery lists the key mappings the rule excludes, for the report.
const appArtifactSkippedQuery = `SELECT a.handle, a.type, k.name, art.handle, art.kind, ` + appArtifactSkipReasonSQL + `
	FROM application_api_keys m
	INNER JOIN applications a ON a.uuid = m.application_uuid
	INNER JOIN api_keys k ON k.uuid = m.api_key_id
	INNER JOIN artifacts art ON art.uuid = k.artifact_uuid
	WHERE NOT (` + appArtifactRuleSQL + `)
	ORDER BY a.handle, k.name`

// warnSkippedAppArtifactMappings reports every key mapping that yields no
// association because v2 would refuse one, naming the rule it breaks.
func warnSkippedAppArtifactMappings(ctx context.Context, src queryer, rep *ResourceReport) error {
	rows, err := src.QueryContext(ctx, appArtifactSkippedQuery)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var appHandle, appType, keyName, artHandle, artKind, reason string
		if err := rows.Scan(&appHandle, &appType, &keyName, &artHandle, &artKind, &reason); err != nil {
			return err
		}
		rep.warn("application %s (type %s) maps key %q of %s %s: no association derived — %s; the key mapping itself is migrated",
			appHandle, appType, keyName, artKind, artHandle, reason)
	}
	return rows.Err()
}

// deriveAppArtifactPairs keeps one row per (application, artifact) in first-seen
// order; rows must arrive oldest first for created_at to be the earliest mapping.
func deriveAppArtifactPairs(rows []appArtifactRow) []appArtifactRow {
	seen := make(map[[2]string]struct{}, len(rows))
	out := make([]appArtifactRow, 0, len(rows))
	for _, r := range rows {
		key := [2]string{r.applicationUUID, r.artifactUUID}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, r)
	}
	return out
}

func (m *appArtifactMappingsMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx, appArtifactPairsQuery)
	if err != nil {
		return nil, err
	}
	var mappings []appArtifactRow
	for rows.Next() {
		var r appArtifactRow
		if err := rows.Scan(&r.applicationUUID, &r.artifactUUID, &r.createdAt); err != nil {
			rows.Close()
			return nil, err
		}
		mappings = append(mappings, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	items := deriveAppArtifactPairs(mappings)
	rep.SrcCount = int64(len(items))
	rc.Log.Info("application artifact associations derived from API key mappings",
		"keyMappings", len(mappings), "associations", len(items))
	if err := warnSkippedAppArtifactMappings(ctx, rc.Src, rep); err != nil {
		return nil, err
	}

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r appArtifactRow) error {
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "application_artifact_mappings",
			[]string{"application_uuid", "artifact_uuid", "created_at"},
			[]any{r.applicationUUID, r.artifactUUID, tstzPtrArg(r.createdAt, rc.Kernels.appZone())},
			conflictCols("application_uuid", "artifact_uuid"))
	})
	return rep, err
}

func (m *appArtifactMappingsMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, appArtifactPairsCountQuery, "SELECT COUNT(*) FROM application_artifact_mappings", false)
	return rep, nil
}

// ---------------------------------------------------------------------------
// application_api_key_mappings (from v1 application_api_keys)
// ---------------------------------------------------------------------------

type appAPIKeyMappingsMigrator struct{ baseMigrator }

type appAPIKeyRow struct {
	applicationUUID string
	apiKeyID        string
	createdAt       sql.NullTime
}

func (m *appAPIKeyMappingsMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx, `SELECT application_uuid, api_key_id, created_at FROM application_api_keys`)
	if err != nil {
		return nil, err
	}
	var items []appAPIKeyRow
	for rows.Next() {
		var r appAPIKeyRow
		if err := rows.Scan(&r.applicationUUID, &r.apiKeyID, &r.createdAt); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	rep.SrcCount = int64(len(items))

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r appAPIKeyRow) error {
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "application_api_key_mappings",
			[]string{"application_uuid", "api_key_id", "created_at"},
			[]any{r.applicationUUID, r.apiKeyID, tstzPtrArg(r.createdAt, rc.Kernels.appZone())},
			conflictCols("application_uuid", "api_key_id"))
	})
	return rep, err
}

func (m *appAPIKeyMappingsMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM application_api_keys", "SELECT COUNT(*) FROM application_api_key_mappings", false)
	return rep, nil
}
