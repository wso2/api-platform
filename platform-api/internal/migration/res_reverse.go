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

// Reverse migration (v2 -> v1), §13. The framework (direction flag, role swap,
// reverse FK order, dry-run, verify) is in the runner; each cleanly-invertible
// migrator overrides migrateReverse below. Reverse is BEST-EFFORT and NOT the
// primary rollback (the primary rollback is keeping v1 untouched + a pre-cutover
// backup). Lossy / v2-NEW columns (updated_by, minted handles, data_version) are
// dropped; secret re-inline (externalizeSecret inverse) and the artifact-split
// recombine remain best-effort TODO and return ErrReverseUnsupported (skipped
// and logged by the runner) rather than fabricating data.

import (
	"context"
	"database/sql"
	"time"
)

// tsToNaiveLocal inverts tsToTstz: a v2 UTC instant becomes the naive
// wall-clock digits v1 expects in its TIMESTAMP column (pgx stores the digits of
// the returned time.Time, discarding the zone). Callers pass the app zone for the
// ~130 local tables and UTC for the .UTC() tables.
func tsToNaiveLocal(t time.Time, zone *time.Location) time.Time {
	if zone == nil {
		zone = time.UTC
	}
	return t.In(zone)
}

func tsToNaiveLocalPtr(t sql.NullTime, zone *time.Location) any {
	if !t.Valid {
		return nil
	}
	return tsToNaiveLocal(t.Time, zone)
}

// recoverActor maps a v2 audit UUID back to the raw v1 creator string via
// user_idp_references.idp_id (§13 "recoverable"). The migration actor and the
// seeded sentinel resolve to "" (v1 had no creator).
func (k *Kernels) recoverActor(ctx context.Context, srcV2 queryer, uuid string) (string, error) {
	if uuid == "" || uuid == k.cfg.MigrationActorUUID {
		return "", nil
	}
	var idpID string
	err := srcV2.QueryRowContext(ctx, `SELECT idp_id FROM user_idp_references WHERE uuid = $1`, uuid).Scan(&idpID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if idpID == migrationActorIdpID {
		return "", nil
	}
	return idpID, nil
}

// ---- organizations (reverse) ----

func (m *organizationsMigrator) migrateReverse(ctx context.Context, rc *RunContext, rep *ResourceReport) (*ResourceReport, error) {
	rows, err := rc.Src.QueryContext(ctx, `SELECT uuid, handle, display_name, region, created_at, updated_at FROM organizations`)
	if err != nil {
		return nil, err
	}
	type row struct {
		uuid, handle, name   string
		region               sql.NullString
		createdAt, updatedAt sql.NullTime
	}
	var items []row
	for rows.Next() {
		var r row
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
	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r row) error {
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "organizations",
			[]string{"uuid", "handle", "name", "region", "created_at", "updated_at"},
			[]any{r.uuid, r.handle, r.name, nullOrString(r.region),
				tsToNaiveLocalPtr(r.createdAt, rc.Kernels.appZone()), tsToNaiveLocalPtr(r.updatedAt, rc.Kernels.appZone())},
			conflictUUIDNothing)
	})
	return rep, err
}

// ---- projects (reverse; handle dropped — v1 has none) ----

func (m *projectsMigrator) migrateReverse(ctx context.Context, rc *RunContext, rep *ResourceReport) (*ResourceReport, error) {
	rows, err := rc.Src.QueryContext(ctx, `SELECT uuid, display_name, organization_uuid, description, created_at, updated_at FROM projects`)
	if err != nil {
		return nil, err
	}
	type row struct {
		uuid, name, orgUUID  string
		description          sql.NullString
		createdAt, updatedAt sql.NullTime
	}
	var items []row
	for rows.Next() {
		var r row
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
	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r row) error {
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "projects",
			[]string{"uuid", "name", "organization_uuid", "description", "created_at", "updated_at"},
			[]any{r.uuid, r.name, r.orgUUID, nullOrString(r.description),
				tsToNaiveLocalPtr(r.createdAt, rc.Kernels.appZone()), tsToNaiveLocalPtr(r.updatedAt, rc.Kernels.appZone())},
			conflictUUIDNothing)
	})
	return rep, err
}

// ---- applications (reverse; created_by recovered from idp_id) ----

func (m *applicationsMigrator) migrateReverse(ctx context.Context, rc *RunContext, rep *ResourceReport) (*ResourceReport, error) {
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT uuid, handle, project_uuid, organization_uuid, created_by, display_name, description, type, created_at, updated_at FROM applications`)
	if err != nil {
		return nil, err
	}
	type row struct {
		uuid, handle, orgUUID string
		projectUUID           sql.NullString
		createdBy             sql.NullString
		name                  string
		description           sql.NullString
		appType               sql.NullString
		createdAt, updatedAt  sql.NullTime
	}
	var items []row
	for rows.Next() {
		var r row
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
	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r row) error {
		creator, err := rc.Kernels.recoverActor(ctx, rc.Src, nullToString(r.createdBy))
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "applications",
			[]string{"uuid", "handle", "project_uuid", "organization_uuid", "created_by", "name", "description", "type", "created_at", "updated_at"},
			[]any{r.uuid, r.handle, nullOrString(r.projectUUID), r.orgUUID, nullOrEmpty(creator), r.name, nullOrString(r.description), nullOrString(r.appType),
				tsToNaiveLocalPtr(r.createdAt, rc.Kernels.appZone()), tsToNaiveLocalPtr(r.updatedAt, rc.Kernels.appZone())},
			conflictUUIDNothing)
	})
	return rep, err
}

// ---- subscriptions (reverse; artifact_uuid -> api_uuid; token/hash carried) ----

func (m *subscriptionsMigrator) migrateReverse(ctx context.Context, rc *RunContext, rep *ResourceReport) (*ResourceReport, error) {
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT uuid, artifact_uuid, subscriber_id, application_id, subscription_token, subscription_token_hash, subscription_plan_uuid, organization_uuid, status, created_at, updated_at FROM subscriptions`)
	if err != nil {
		return nil, err
	}
	type row struct {
		uuid, artifactUUID, orgUUID string
		subscriberID, applicationID sql.NullString
		token, tokenHash            sql.NullString
		planUUID, status            sql.NullString
		createdAt, updatedAt        sql.NullTime
	}
	var items []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.uuid, &r.artifactUUID, &r.subscriberID, &r.applicationID, &r.token, &r.tokenHash, &r.planUUID, &r.orgUUID, &r.status, &r.createdAt, &r.updatedAt); err != nil {
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
	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r row) error {
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "subscriptions",
			[]string{"uuid", "api_uuid", "subscriber_id", "application_id", "subscription_token", "subscription_token_hash", "subscription_plan_uuid", "organization_uuid", "status", "created_at", "updated_at"},
			[]any{r.uuid, r.artifactUUID /* artifact_uuid -> api_uuid */, nullOrString(r.subscriberID), nullOrString(r.applicationID),
				nullOrString(r.token), nullOrString(r.tokenHash), nullOrString(r.planUUID), r.orgUUID, nullOrString(r.status),
				tsToNaiveLocalPtr(r.createdAt, rc.Kernels.appZone()), tsToNaiveLocalPtr(r.updatedAt, rc.Kernels.appZone())},
			conflictUUIDNothing)
	})
	return rep, err
}
