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
	"strings"
	"time"
)

func init() {
	register(&userIdpMigrator{baseMigrator{name: "user_idp_references"}})
	register(&organizationsMigrator{baseMigrator{name: "organizations", dependsOn: []string{"user_idp_references"}}})
	register(&projectsMigrator{baseMigrator{name: "projects", dependsOn: []string{"user_idp_references", "organizations"}}})
	register(&applicationsMigrator{baseMigrator{name: "applications", dependsOn: []string{"user_idp_references", "organizations", "projects"}}})
	register(&subscriptionPlansMigrator{baseMigrator{name: "subscription_plans", dependsOn: []string{"user_idp_references", "organizations"}}})
	register(&subscriptionsMigrator{baseMigrator{name: "subscriptions", dependsOn: []string{"organizations", "artifacts", "applications", "subscription_plans"}}})
}

// ---------------------------------------------------------------------------
// user_idp_references — seed the migration actor + every distinct v1 actor
// string up front (committed), so the FK target for created_by/updated_by exists
// before any dependent row and is robust against per-batch rollback (§B.4).
// ---------------------------------------------------------------------------

type userIdpMigrator struct{ baseMigrator }

// v1 tables carrying a created_by actor string (verified per resource group).
var v1ActorTables = []struct{ table, col string }{
	{"applications", "created_by"},
	{"rest_apis", "created_by"},
	{"llm_providers", "created_by"},
	{"llm_proxies", "created_by"},
	{"llm_provider_templates", "created_by"},
	{"mcp_proxies", "created_by"},
	{"websub_apis", "created_by"},
	{"webbroker_apis", "created_by"},
	{"api_keys", "created_by"},
}

func (m *userIdpMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		// Reverse: the raw v1 actor string is recovered from idp_id when writing
		// each dependent table's created_by, so nothing to do here.
		rep.Status = StatusSkip
		rep.addf("reverse: actor strings recovered from idp_id inline")
		return rep, nil
	}

	// Harvest distinct actor strings (tolerating tables/columns that don't exist).
	seen := map[string]bool{}
	var actors []string
	for _, t := range v1ActorTables {
		rows, err := rc.Src.QueryContext(ctx,
			"SELECT DISTINCT "+t.col+" FROM "+t.table+" WHERE "+t.col+" IS NOT NULL AND "+t.col+" <> ''")
		if err != nil {
			rc.Log.Warn("actor harvest skipped (table/column absent?)", "table", t.table, "error", err.Error())
			continue
		}
		for rows.Next() {
			var a string
			if err := rows.Scan(&a); err != nil {
				rows.Close()
				return nil, err
			}
			if a = strings.TrimSpace(a); a != "" && !seen[a] {
				seen[a] = true
				actors = append(actors, a)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	rep.SrcCount = int64(len(actors))
	rc.Log.Info("actor harvest complete", "distinctActors", len(actors))

	if rc.writes() && rc.Tgt != nil {
		if err := withTx(ctx, rc.Tgt, func(tx *sql.Tx) error {
			return rc.Kernels.seedMigrationActor(ctx, tx)
		}); err != nil {
			return nil, err
		}
	}
	// Seed actors in committed batches; also populates the in-memory cache.
	if err := runResource(ctx, rc, actors, func(ctx context.Context, q queryer, actor string) error {
		return rc.Kernels.seedActor(ctx, q, actor)
	}); err != nil {
		return nil, err
	}
	return rep, nil
}

func (m *userIdpMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Tgt == nil || rc.Direction == DirectionReverse {
		// v2-only table: in reverse the target is v1, which has no such relation.
		rep.Status = StatusSkip
		if rc.Direction == DirectionReverse {
			rep.addf("reverse: v2-only table, nothing to verify in v1")
		}
		return rep, nil
	}
	if rc.DryRun {
		rep.addf("dry-run: identity seed validated, no writes — target count not applicable")
		return rep, nil
	}
	n, err := scanCount(ctx, rc.Tgt, "SELECT COUNT(*) FROM user_idp_references")
	if err != nil {
		rep.fail("count: %v", err)
		return rep, nil
	}
	rep.TgtCount = n
	// +1 for the migration actor row.
	if n < 1 {
		rep.warn("expected at least the migration actor row")
	}
	return rep, nil
}

// ---------------------------------------------------------------------------
// organizations
// ---------------------------------------------------------------------------

type organizationsMigrator struct{ baseMigrator }

type orgRow struct {
	uuid, name string
	handle     string
	region     sql.NullString
	createdAt  time.Time
	updatedAt  time.Time
}

func (m *organizationsMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT uuid, handle, name, region, created_at, updated_at FROM organizations`)
	if err != nil {
		return nil, err
	}
	var items []orgRow
	for rows.Next() {
		var r orgRow
		if err := rows.Scan(&r.uuid, &r.handle, &r.name, &r.region, &r.createdAt, &r.updatedAt); err != nil {
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

	var fromChoreo, fallback int
	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r orgRow) error {
		// §B.14: the v2 handle is Choreo's (the AI Workspace URL / REST id), not
		// v1's random 7-letter string. An org the export does not know keeps
		// v1's handle rather than get an invented one (Gate 5 already WARNed).
		handle, ok, err := rc.Kernels.externalHandle(rc.Cfg.OrgHandles, r.uuid)
		if err != nil {
			return err
		}
		if ok {
			fromChoreo++
		} else {
			fallback++
			if handle, err = rc.Kernels.carryHandle("organizations", r.uuid, r.handle); err != nil {
				return err
			}
		}
		// display_name (§B.14): the AI Workspace v2 header shows v2's org
		// display_name (Choreo APIM forwards GET /organizations/{handle} to v2), so
		// take Choreo's org name from the export (org_name) — the value a freshly
		// provisioned org carries — and fall back to the handle. v1 `name` is a
		// random string and is dropped. Renames in Choreo never reach v2 (true for
		// every v2 org, not just migrated ones).
		displayName := handle
		if n := rc.Cfg.OrgHandles.NameOf(r.uuid); ok && n != "" && len(n) <= 255 {
			displayName = n
		}
		src := sourceChoreoExport
		if !ok {
			src = sourceV1HandleKept
		}
		rc.Handles.add(handleReportRow{kind: "organization", uuid: r.uuid, v1Value: r.handle, v2Handle: handle, v2DisplayName: displayName, source: src})
		actor, err := rc.Kernels.resolveActor(ctx, q, "") // v1 orgs have no creator -> migration actor
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "organizations",
			[]string{"uuid", "handle", "display_name", "region", "idp_organization_ref_uuid", "created_by", "updated_by", "created_at", "updated_at"},
			[]any{r.uuid, handle, displayName, nullOrString(r.region), r.uuid /* own uuid placeholder */, actor, actor,
				tsToTstz(r.createdAt, rc.Kernels.appZone()), tsToTstz(r.updatedAt, rc.Kernels.appZone())},
			conflictUUIDNothing)
	})
	rep.addf("handles: %d from the Choreo export, %d fallback (v1 handle kept — not in export); display_name = export org_name (fallback: handle)", fromChoreo, fallback)
	if fallback > 0 {
		rep.warn("%d organization(s) are not in the Choreo export — their v1 handle was kept and will NOT match any Choreo URL", fallback)
	}
	return rep, err
}

func (m *organizationsMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM organizations", "SELECT COUNT(*) FROM organizations", false)
	verifyHandleParity(ctx, rc, rep, "organizations", rc.Cfg.OrgHandles)
	return rep, nil
}

// ---------------------------------------------------------------------------
// projects
// ---------------------------------------------------------------------------

type projectsMigrator struct{ baseMigrator }

type projectRow struct {
	uuid, name, orgUUID  string
	description          sql.NullString
	createdAt, updatedAt time.Time
}

func (m *projectsMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT uuid, name, organization_uuid, description, created_at, updated_at FROM projects`)
	if err != nil {
		return nil, err
	}
	var items []projectRow
	for rows.Next() {
		var r projectRow
		if err := rows.Scan(&r.uuid, &r.name, &r.orgUUID, &r.description, &r.createdAt, &r.updatedAt); err != nil {
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

	// §B.14: decide every handle up front so a Choreo handle always wins inside
	// its org; v1's auto-created "default" projects (unknown to Choreo) are
	// minted from the v1 name and suffixed when they collide.
	var dbExists func(orgUUID, h string) bool
	if rc.Tgt != nil && rc.writes() {
		dbExists = func(orgUUID, h string) bool {
			return rc.handleExistsChecker(ctx, "projects", "organization_uuid", orgUUID)(h)
		}
	}
	plan, err := rc.Kernels.planProjectHandles(items, rc.Cfg.ProjectHandles, dbExists)
	if err != nil {
		return nil, err
	}
	rep.addf("handles: %d from the Choreo export, %d fallback (minted from v1 name; %d suffixed on collision with a Choreo handle)",
		plan.mapped, len(plan.fallback), len(plan.suffixed))
	for _, u := range plan.fallback {
		rc.Log.Debug("project not in Choreo export — handle minted from v1 name", "project", u, "handle", plan.handles[u])
	}
	minted := make(map[string]handleSource, len(plan.fallback))
	for _, u := range plan.fallback {
		minted[u] = sourceMinted
	}
	for _, u := range plan.suffixed {
		minted[u] = sourceMintedSuffixed
	}
	for _, r := range items {
		src, isMinted := minted[r.uuid]
		if !isMinted {
			src = sourceChoreoExport
		}
		rc.Handles.add(handleReportRow{kind: "project", uuid: r.uuid, orgUUID: r.orgUUID, v1Value: r.name,
			v2Handle: plan.handles[r.uuid], v2DisplayName: plan.handles[r.uuid], source: src})
	}

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r projectRow) error {
		handle := plan.handles[r.uuid]
		// display_name = handle (§B.14): v1 `name` is "default" or a random
		// string, and Choreo renames never reach v2 — see organizations.
		displayName := handle
		actor, err := rc.Kernels.resolveActor(ctx, q, "") // v1 projects have no creator
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "projects",
			[]string{"uuid", "handle", "display_name", "organization_uuid", "description", "created_by", "created_at", "updated_by", "updated_at"},
			[]any{r.uuid, handle, displayName, r.orgUUID, textOrEmpty(r.description), actor,
				tsToTstz(r.createdAt, rc.Kernels.appZone()), actor, tsToTstz(r.updatedAt, rc.Kernels.appZone())},
			conflictUUIDNothing)
	})
	return rep, err
}

func (m *projectsMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM projects", "SELECT COUNT(*) FROM projects", false)
	verifyHandleParity(ctx, rc, rep, "projects", rc.Cfg.ProjectHandles)
	return rep, nil
}

// ---------------------------------------------------------------------------
// applications
// ---------------------------------------------------------------------------

type applicationsMigrator struct{ baseMigrator }

type appRow struct {
	uuid, handle, orgUUID string
	projectUUID           sql.NullString
	createdBy             sql.NullString
	name                  string
	description           sql.NullString
	appType               sql.NullString
	createdAt, updatedAt  time.Time
}

func (m *applicationsMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT uuid, handle, project_uuid, organization_uuid, created_by, name, description, type, created_at, updated_at FROM applications`)
	if err != nil {
		return nil, err
	}
	var items []appRow
	for rows.Next() {
		var r appRow
		if err := rows.Scan(&r.uuid, &r.handle, &r.projectUUID, &r.orgUUID, &r.createdBy, &r.name, &r.description, &r.appType, &r.createdAt, &r.updatedAt); err != nil {
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

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r appRow) error {
		handle, err := rc.Kernels.carryHandle("applications", r.uuid, r.handle)
		if err != nil {
			return err
		}
		actor, err := rc.Kernels.resolveActor(ctx, q, nullToString(r.createdBy))
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "applications",
			[]string{"uuid", "handle", "project_uuid", "organization_uuid", "created_by", "updated_by", "display_name", "description", "type", "created_at", "updated_at"},
			[]any{r.uuid, handle, nullOrString(r.projectUUID), r.orgUUID, actor, actor, r.name, textOrEmpty(r.description), nullOrString(r.appType),
				tsToTstz(r.createdAt, rc.Kernels.appZone()), tsToTstz(r.updatedAt, rc.Kernels.appZone())},
			conflictUUIDNothing)
	})
	return rep, err
}

func (m *applicationsMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM applications", "SELECT COUNT(*) FROM applications", false)
	return rep, nil
}

// ---------------------------------------------------------------------------
// subscription_plans (+ subscription_plan_limits 0..1 child)
// ---------------------------------------------------------------------------

type subscriptionPlansMigrator struct{ baseMigrator }

type planRow struct {
	uuid, planName, orgUUID string
	status                  sql.NullString
	stopOnQuota             sql.NullBool
	throttleCount           sql.NullInt64
	throttleUnit            sql.NullString
	expiryTime              sql.NullTime
	createdAt, updatedAt    time.Time
}

func (m *subscriptionPlansMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT uuid, plan_name, stop_on_quota_reach, throttle_limit_count, throttle_limit_unit, expiry_time, organization_uuid, status, created_at, updated_at FROM subscription_plans`)
	if err != nil {
		return nil, err
	}
	var items []planRow
	for rows.Next() {
		var r planRow
		if err := rows.Scan(&r.uuid, &r.planName, &r.stopOnQuota, &r.throttleCount, &r.throttleUnit, &r.expiryTime, &r.orgUUID, &r.status, &r.createdAt, &r.updatedAt); err != nil {
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

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r planRow) error {
		handle, err := rc.Kernels.mintScopedHandle("subscription_plans", r.orgUUID, r.uuid, r.planName,
			rc.handleExistsChecker(ctx, "subscription_plans", "organization_uuid", r.orgUUID))
		if err != nil {
			return err
		}
		actor, err := rc.Kernels.resolveActor(ctx, q, "") // v1 plans have no creator
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		createdAt := tsToTstz(r.createdAt, rc.Kernels.appZone())
		if err := insertRow(ctx, q, "subscription_plans",
			[]string{"uuid", "handle", "display_name", "expiry_time", "organization_uuid", "status", "created_by", "updated_by", "created_at", "updated_at"},
			[]any{r.uuid, handle, r.planName, tsToTstzPtr(nullTimePtr(r.expiryTime), rc.Kernels.appZone()), r.orgUUID, nullOrString(r.status), actor, actor,
				createdAt, tsToTstz(r.updatedAt, rc.Kernels.appZone())},
			conflictUUIDNothing); err != nil {
			return err
		}
		// Child limit row — emit ONLY when v1 has a throttle count (§ plans §6).
		if r.throttleCount.Valid {
			limitUUID := detUUID("splimit|"+r.uuid+"|REQUEST_COUNT", createdAt)
			return insertRow(ctx, q, "subscription_plan_limits",
				[]string{"uuid", "subscription_plan_uuid", "limit_type", "time_unit", "time_amount", "limit_count", "stop_on_quota_reach"},
				[]any{limitUUID, r.uuid, "REQUEST_COUNT", mapThrottleUnit(r.throttleUnit), 1, r.throttleCount.Int64, boolToSmallint(r.stopOnQuota)},
				conflictCols("subscription_plan_uuid", "limit_type", "time_amount", "time_unit"))
		}
		return nil
	})
	return rep, err
}

func (m *subscriptionPlansMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM subscription_plans", "SELECT COUNT(*) FROM subscription_plans", false)
	return rep, nil
}

// ---------------------------------------------------------------------------
// subscriptions
// ---------------------------------------------------------------------------

type subscriptionsMigrator struct{ baseMigrator }

type subRow struct {
	uuid, apiUUID, orgUUID string
	subscriberID           sql.NullString
	applicationID          sql.NullString
	token                  sql.NullString
	tokenHash              sql.NullString
	planUUID               sql.NullString
	status                 sql.NullString
	createdAt, updatedAt   time.Time
}

func (m *subscriptionsMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT uuid, api_uuid, subscriber_id, application_id, subscription_token, subscription_token_hash, subscription_plan_uuid, organization_uuid, status, created_at, updated_at FROM subscriptions`)
	if err != nil {
		return nil, err
	}
	var items []subRow
	for rows.Next() {
		var r subRow
		if err := rows.Scan(&r.uuid, &r.apiUUID, &r.subscriberID, &r.applicationID, &r.token, &r.tokenHash, &r.planUUID, &r.orgUUID, &r.status, &r.createdAt, &r.updatedAt); err != nil {
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

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r subRow) error {
		actor, err := rc.Kernels.resolveActor(ctx, q, "") // v1 subscriptions have no creator
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "subscriptions",
			[]string{"uuid", "artifact_uuid", "subscriber_id", "application_id", "subscription_token", "subscription_token_hash",
				"subscription_plan_uuid", "organization_uuid", "status", "created_by", "updated_by", "created_at", "updated_at"},
			[]any{r.uuid, r.apiUUID /* rename api_uuid->artifact_uuid */, nullOrString(r.subscriberID), nullOrString(r.applicationID),
				nullOrString(r.token) /* ciphertext carried verbatim */, nullOrString(r.tokenHash), nullOrString(r.planUUID),
				r.orgUUID, nullOrString(r.status), actor, actor,
				tsToTstz(r.createdAt, rc.Kernels.appZone()), tsToTstz(r.updatedAt, rc.Kernels.appZone())},
			conflictUUIDNothing)
	})
	return rep, err
}

func (m *subscriptionsMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM subscriptions", "SELECT COUNT(*) FROM subscriptions", false)
	return rep, nil
}
