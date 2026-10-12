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
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// pgUndefinedTable is the Postgres SQLSTATE for a missing relation.
const pgUndefinedTable = "42P01"

func init() {
	register(&gatewaysMigrator{baseMigrator{name: "gateways", dependsOn: []string{"user_idp_references", "organizations"}}})
	register(&gatewayTokensMigrator{baseMigrator{name: "gateway_tokens", dependsOn: []string{"gateways"}}})
	register(&customPoliciesMigrator{baseMigrator{name: "gateway_custom_policies", dependsOn: []string{"organizations"}}})
}

// ---------------------------------------------------------------------------
// gateways (+ gateway_endpoints from vhost, + manifest side-channel)
// ---------------------------------------------------------------------------

type gatewaysMigrator struct{ baseMigrator }

type gatewayRow struct {
	uuid, orgUUID     string
	name              string // v1 machine name -> v2 handle (MINT)
	displayName       sql.NullString
	description       sql.NullString
	properties        []byte
	manifest          []byte
	vhost             sql.NullString
	isCritical        sql.NullBool
	functionalityType sql.NullString
	version           sql.NullString
	isActive          sql.NullBool
	createdAt         time.Time
	updatedAt         time.Time
}

func (m *gatewaysMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT uuid, organization_uuid, name, display_name, description, properties, manifest,
		        vhost, is_critical, gateway_functionality_type, version, is_active, created_at, updated_at
		 FROM gateways`)
	if err != nil {
		return nil, err
	}
	var items []gatewayRow
	for rows.Next() {
		var r gatewayRow
		if err := rows.Scan(&r.uuid, &r.orgUUID, &r.name, &r.displayName, &r.description, &r.properties, &r.manifest,
			&r.vhost, &r.isCritical, &r.functionalityType, &r.version, &r.isActive, &r.createdAt, &r.updatedAt); err != nil {
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

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r gatewayRow) error {
		// v1 name (machine name) -> v2 handle (VARCHAR(40)); mint + checkpoint.
		handle, err := rc.Kernels.mintScopedHandle("gateways", r.orgUUID, r.uuid, r.name,
			rc.handleExistsChecker(ctx, "gateways", "organization_uuid", r.orgUUID))
		if err != nil {
			return err
		}
		actor, err := rc.Kernels.resolveActor(ctx, q, "") // v1 gateways have no creator
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		if err := insertRow(ctx, q, "gateways",
			[]string{"uuid", "organization_uuid", "handle", "display_name", "description", "properties",
				"is_critical", "gateway_functionality_type", "version", "is_active", "created_by", "updated_by", "created_at", "updated_at"},
			[]any{r.uuid, r.orgUUID, handle, nullOrString(r.displayName), textOrEmpty(r.description), nullOrBytes(r.properties),
				boolToSmallint(r.isCritical), nullOrString(r.functionalityType), nullOrString(r.version), boolToSmallint(r.isActive),
				actor, actor, tsToTstz(r.createdAt, rc.Kernels.appZone()), tsToTstz(r.updatedAt, rc.Kernels.appZone())},
			conflictUUIDNothing); err != nil {
			return err
		}
		// manifest is written by a side-channel UPDATE in v2 (§ gateways SIDE_CHANNEL) — opaque byte-copy.
		if len(r.manifest) > 0 {
			if _, err := q.ExecContext(ctx, `UPDATE gateways SET manifest = $1 WHERE uuid = $2`, r.manifest, r.uuid); err != nil {
				return err
			}
		}
		// vhost -> gateway_endpoints (0..1). SERIAL PK => delete-by-parent then insert (§B.9).
		if err := deleteByParent(ctx, q, "gateway_endpoints", "gateway_uuid", r.uuid); err != nil {
			return err
		}
		if r.vhost.Valid && r.vhost.String != "" {
			if err := insertRow(ctx, q, "gateway_endpoints",
				[]string{"gateway_uuid", "url"}, []any{r.uuid, r.vhost.String}, ""); err != nil {
				return err
			}
		}
		return nil
	})
	return rep, err
}

func (m *gatewaysMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM gateways", "SELECT COUNT(*) FROM gateways", false)
	return rep, nil
}

// ---------------------------------------------------------------------------
// gateway_tokens — unsalted SHA-256 carried verbatim; salt vestigial '' (§B.8)
// ---------------------------------------------------------------------------

type gatewayTokensMigrator struct{ baseMigrator }

type gwTokenRow struct {
	uuid, gatewayUUID string
	tokenHash         string
	salt              sql.NullString
	status            sql.NullString
	createdAt         time.Time
	revokedAt         sql.NullTime
}

func (m *gatewayTokensMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT uuid, gateway_uuid, token_hash, salt, status, created_at, revoked_at FROM gateway_tokens`)
	if err != nil {
		return nil, err
	}
	var items []gwTokenRow
	for rows.Next() {
		var r gwTokenRow
		if err := rows.Scan(&r.uuid, &r.gatewayUUID, &r.tokenHash, &r.salt, &r.status, &r.createdAt, &r.revokedAt); err != nil {
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

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r gwTokenRow) error {
		actor, err := rc.Kernels.resolveActor(ctx, q, "") // created_by is v2-NEW -> migration actor
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		salt := "" // vestigial; carry the v1 value which is always ''
		if r.salt.Valid {
			salt = r.salt.String
		}
		return insertRow(ctx, q, "gateway_tokens",
			[]string{"uuid", "gateway_uuid", "token_hash", "salt", "status", "created_by", "created_at", "revoked_by", "revoked_at"},
			[]any{r.uuid, r.gatewayUUID, r.tokenHash, salt, nullOrString(r.status), actor,
				tsToTstz(r.createdAt, rc.Kernels.appZone()), nil /* revoked_by v2-NEW */, tstzPtrArg(r.revokedAt, rc.Kernels.appZone())},
			conflictUUIDNothing)
	})
	return rep, err
}

func (m *gatewayTokensMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM gateway_tokens", "SELECT COUNT(*) FROM gateway_tokens", false)
	return rep, nil
}

// ---------------------------------------------------------------------------
// gateway_custom_policies (+ gateway_custom_policy_usages) — upsert on
// (organization_uuid, name, version); description clip-safe to 1023 (§B.13 ③);
// usages FK rename api_uuid -> artifact_uuid.
// ---------------------------------------------------------------------------

type customPoliciesMigrator struct{ baseMigrator }

type customPolicyRow struct {
	uuid, orgUUID, name  string
	displayName          sql.NullString
	version              sql.NullString
	description          sql.NullString
	policyDefinition     []byte
	createdAt, updatedAt time.Time
}

func (m *customPoliciesMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT uuid, organization_uuid, name, display_name, version, description, policy_definition, created_at, updated_at
		 FROM gateway_custom_policies`)
	if err != nil {
		return nil, err
	}
	var items []customPolicyRow
	for rows.Next() {
		var r customPolicyRow
		if err := rows.Scan(&r.uuid, &r.orgUUID, &r.name, &r.displayName, &r.version, &r.description, &r.policyDefinition, &r.createdAt, &r.updatedAt); err != nil {
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

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r customPolicyRow) error {
		actor, err := rc.Kernels.resolveActor(ctx, q, "") // created_by/updated_by v2-NEW
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		desc := nullOrString(r.description)
		// VARCHAR(1023) counts characters, so measure and clip by rune to keep
		// the value valid UTF-8.
		if runes := []rune(r.description.String); r.description.Valid && len(runes) > 1023 {
			desc = string(runes[:1023]) // tier ③ clip-safe (purely descriptive)
			rc.Log.Warn("clipped gateway_custom_policies.description to 1023", "uuid", r.uuid)
			rep.warn("gateway_custom_policies %s: description clipped to 1023 characters", r.uuid)
		}
		return insertRow(ctx, q, "gateway_custom_policies",
			[]string{"uuid", "organization_uuid", "name", "display_name", "version", "description", "policy_definition", "created_by", "updated_by", "created_at", "updated_at"},
			[]any{r.uuid, r.orgUUID, r.name, nullOrString(r.displayName), nullOrString(r.version), desc, nullOrBytes(r.policyDefinition), actor, actor,
				tsToTstz(r.createdAt, rc.Kernels.appZone()), tsToTstz(r.updatedAt, rc.Kernels.appZone())},
			conflictCols("organization_uuid", "name", "version"))
	})
	if err != nil {
		return rep, err
	}

	// Child: gateway_custom_policy_usages (policy_uuid, api_uuid) -> (policy_uuid, artifact_uuid).
	if err := m.migrateUsages(ctx, rc, rep); err != nil {
		return rep, err
	}
	return rep, nil
}

type policyUsageRow struct{ policyUUID, apiUUID string }

func (m *customPoliciesMigrator) migrateUsages(ctx context.Context, rc *RunContext, rep *ResourceReport) error {
	rows, err := rc.Src.QueryContext(ctx, `SELECT policy_uuid, api_uuid FROM gateway_custom_policy_usages`)
	if err != nil {
		// The table is absent on older v1 schemas; any other failure must not
		// silently drop every policy usage.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUndefinedTable {
			rc.Log.Warn("gateway_custom_policy_usages read skipped: table absent")
			rep.warn("gateway_custom_policy_usages absent in v1; no policy usages migrated")
			return nil
		}
		return err
	}
	var items []policyUsageRow
	for rows.Next() {
		var r policyUsageRow
		if err := rows.Scan(&r.policyUUID, &r.apiUUID); err != nil {
			rows.Close()
			return err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	rep.addf("gateway_custom_policy_usages: %d rows", len(items))

	return runResource(ctx, rc, items, func(ctx context.Context, q queryer, r policyUsageRow) error {
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "gateway_custom_policy_usages",
			[]string{"policy_uuid", "artifact_uuid"}, []any{r.policyUUID, r.apiUUID},
			conflictCols("policy_uuid", "artifact_uuid"))
	})
}

func (m *customPoliciesMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM gateway_custom_policies", "SELECT COUNT(*) FROM gateway_custom_policies", false)
	return rep, nil
}
