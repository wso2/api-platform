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
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
)

func init() {
	register(&artifactsMigrator{baseMigrator{name: "artifacts", dependsOn: []string{"organizations"}}})
	register(&restApisMigrator{baseMigrator{name: "rest_apis", dependsOn: []string{"artifacts", "projects", "user_idp_references"}}})
	register(&llmTemplatesMigrator{baseMigrator{name: "llm_provider_templates", dependsOn: []string{"organizations", "user_idp_references"}}})
	register(&llmProvidersMigrator{baseMigrator{name: "llm_providers", dependsOn: []string{"artifacts", "llm_provider_templates"}}})
	register(&llmProxiesMigrator{baseMigrator{name: "llm_proxies", dependsOn: []string{"artifacts", "llm_providers", "projects"}}})
	register(&mcpProxiesMigrator{baseMigrator{name: "mcp_proxies", dependsOn: []string{"artifacts", "projects"}}})
	register(&websubApisMigrator{baseMigrator{name: "websub_apis", dependsOn: []string{"artifacts", "projects"}}})
	register(&webbrokerApisMigrator{baseMigrator{name: "webbroker_apis", dependsOn: []string{"artifacts", "projects"}}})
}

// ---------------------------------------------------------------------------
// artifacts (base split) — stripped to (uuid, type, organization_uuid); the
// per-type rows are written by the type migrators that follow (§B.6). This
// migrator writes ALL base rows up front so every later artifact_uuid FK
// (api_keys, subscriptions, deployments, …) resolves.
// ---------------------------------------------------------------------------

type artifactsMigrator struct{ baseMigrator }

type artifactBaseRow struct{ uuid, kind, orgUUID string }

func (m *artifactsMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx, `SELECT uuid, kind, organization_uuid FROM artifacts`)
	if err != nil {
		return nil, err
	}
	var items []artifactBaseRow
	for rows.Next() {
		var r artifactBaseRow
		if err := rows.Scan(&r.uuid, &r.kind, &r.orgUUID); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, r)
	}
	rows.Close()
	rep.SrcCount = int64(len(items))

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r artifactBaseRow) error {
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "artifacts",
			[]string{"uuid", "type", "organization_uuid"},
			[]any{r.uuid, r.kind /* verbatim kind->type */, r.orgUUID}, conflictUUIDNothing)
	})
	return rep, err
}

func (m *artifactsMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM artifacts", "SELECT COUNT(*) FROM artifacts", false)
	return rep, nil
}

// artifactTypeRow is the joined artifacts + <type> shape shared by every
// artifact-type migrator. Type-specific columns are read separately.
type artifactTypeRow struct {
	uuid, orgUUID string
	handle        string
	name          string
	version       string
	createdAt     time.Time
	updatedAt     time.Time
	// type-specific
	projectUUID     sql.NullString
	description     sql.NullString
	createdBy       sql.NullString
	lifecycleStatus sql.NullString
	transport       sql.NullString
	templateUUID    sql.NullString
	providerUUID    sql.NullString
	openapiSpec     sql.NullString
	modelList       sql.NullString
	configuration   []byte
}

// artifactCreatedAt/updatedAt convert the v1 artifacts-row timestamps — v1 wrote
// artifacts (and the plugin type tables) with .UTC(), so the source zone is UTC
// regardless of --v1-timezone (§B.3).
func artifactCreatedAt(r artifactTypeRow) time.Time { return tsToTstz(r.createdAt, utcZone()) }
func artifactUpdatedAt(r artifactTypeRow) time.Time { return tsToTstz(r.updatedAt, utcZone()) }

// ---------------------------------------------------------------------------
// rest_apis
// ---------------------------------------------------------------------------

type restApisMigrator struct{ baseMigrator }

func (m *restApisMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT a.uuid, a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at,
		        p.description, p.created_by, p.project_uuid, p.lifecycle_status, p.transport, p.configuration
		 FROM artifacts a JOIN rest_apis p ON a.uuid = p.uuid`)
	if err != nil {
		return nil, err
	}
	var items []artifactTypeRow
	for rows.Next() {
		var r artifactTypeRow
		if err := rows.Scan(&r.uuid, &r.handle, &r.name, &r.version, &r.orgUUID, &r.createdAt, &r.updatedAt,
			&r.description, &r.createdBy, &r.projectUUID, &r.lifecycleStatus, &r.transport, &r.configuration); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, r)
	}
	rows.Close()
	rep.SrcCount = int64(len(items))

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r artifactTypeRow) error {
		handle, err := rc.Kernels.carryHandle("rest_apis", r.uuid, r.handle)
		if err != nil {
			return err
		}
		actor, err := rc.Kernels.resolveActor(ctx, q, nullToString(r.createdBy))
		if err != nil {
			return err
		}
		createdAt := artifactCreatedAt(r)
		blob, err := rc.Kernels.transformArtifactConfig(ctx, q, constants.RestApi, r.uuid, r.orgUUID, actor, createdAt, r.configuration, nullToString(r.transport))
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "rest_apis",
			[]string{"uuid", "organization_uuid", "handle", "display_name", "version", "description", "created_by", "updated_by",
				"project_uuid", "lifecycle_status", "configuration", "origin", "data_version", "created_at", "updated_at"},
			[]any{r.uuid, r.orgUUID, handle, r.name, r.version, nullOrString(r.description), actor, actor,
				nullOrString(r.projectUUID), nullOrString(r.lifecycleStatus), blob, constants.OriginCP, "1.0",
				createdAt, artifactUpdatedAt(r)},
			conflictUUIDNothing)
	})
	return rep, err
}

func (m *restApisMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM rest_apis", "SELECT COUNT(*) FROM rest_apis", false)
	return rep, nil
}

// ---------------------------------------------------------------------------
// llm_provider_templates (standalone, NOT an artifact; app-zone timestamps)
// ---------------------------------------------------------------------------

type llmTemplatesMigrator struct{ baseMigrator }

type templateRow struct {
	uuid, orgUUID, handle, name string
	description                 sql.NullString
	createdBy                   sql.NullString
	configuration               []byte
	createdAt, updatedAt        time.Time
}

func (m *llmTemplatesMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT uuid, organization_uuid, handle, name, description, created_by, configuration, created_at, updated_at
		 FROM llm_provider_templates`)
	if err != nil {
		return nil, err
	}
	var items []templateRow
	for rows.Next() {
		var r templateRow
		if err := rows.Scan(&r.uuid, &r.orgUUID, &r.handle, &r.name, &r.description, &r.createdBy, &r.configuration, &r.createdAt, &r.updatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, r)
	}
	rows.Close()
	rep.SrcCount = int64(len(items))

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r templateRow) error {
		handle, err := rc.Kernels.carryHandle("llm_provider_templates", r.uuid, r.handle)
		if err != nil {
			return err
		}
		actor, err := rc.Kernels.resolveActor(ctx, q, nullToString(r.createdBy))
		if err != nil {
			return err
		}
		blob, err := addManagedBy(r.configuration, "organization")
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "llm_provider_templates",
			[]string{"uuid", "organization_uuid", "handle", "group_id", "display_name", "managed_by", "description",
				"created_by", "updated_by", "origin", "configuration", "openapi_spec", "version", "is_latest", "enabled", "created_at", "updated_at"},
			[]any{r.uuid, r.orgUUID, handle, handle /* group_id = handle */, r.name, "organization", nullOrString(r.description),
				actor, actor, constants.OriginCP, blob, []byte{} /* openapi_spec empty */, "v1.0", 1, 1,
				tsToTstz(r.createdAt, rc.Kernels.appZone()), tsToTstz(r.updatedAt, rc.Kernels.appZone())},
			conflictUUIDNothing)
	})
	return rep, err
}

func (m *llmTemplatesMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM llm_provider_templates", "SELECT COUNT(*) FROM llm_provider_templates", false)
	return rep, nil
}

// ---------------------------------------------------------------------------
// llm_providers (status dropped; openapi_spec + model_list opaque; policy split
// + secret externalization on configuration; data_version 1.1)
// ---------------------------------------------------------------------------

type llmProvidersMigrator struct{ baseMigrator }

func (m *llmProvidersMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT a.uuid, a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at,
		        p.description, p.created_by, p.template_uuid, p.openapi_spec, p.model_list, p.configuration
		 FROM artifacts a JOIN llm_providers p ON a.uuid = p.uuid`)
	if err != nil {
		return nil, err
	}
	var items []artifactTypeRow
	for rows.Next() {
		var r artifactTypeRow
		if err := rows.Scan(&r.uuid, &r.handle, &r.name, &r.version, &r.orgUUID, &r.createdAt, &r.updatedAt,
			&r.description, &r.createdBy, &r.templateUUID, &r.openapiSpec, &r.modelList, &r.configuration); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, r)
	}
	rows.Close()
	rep.SrcCount = int64(len(items))

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r artifactTypeRow) error {
		handle, err := rc.Kernels.carryHandle("llm_providers", r.uuid, r.handle)
		if err != nil {
			return err
		}
		actor, err := rc.Kernels.resolveActor(ctx, q, nullToString(r.createdBy))
		if err != nil {
			return err
		}
		createdAt := artifactCreatedAt(r)
		blob, err := rc.Kernels.transformArtifactConfig(ctx, q, constants.LLMProvider, r.uuid, r.orgUUID, actor, createdAt, r.configuration, "")
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "llm_providers",
			[]string{"uuid", "handle", "display_name", "version", "description", "created_by", "updated_by", "template_uuid",
				"openapi_spec", "model_list", "configuration", "origin", "data_version", "created_at", "updated_at", "organization_uuid"},
			[]any{r.uuid, handle, r.name, r.version, nullOrString(r.description), actor, actor, nullOrString(r.templateUUID),
				textToBytea(r.openapiSpec), textToBytea(r.modelList), blob, constants.OriginCP, "1.1",
				createdAt, artifactUpdatedAt(r), r.orgUUID},
			conflictUUIDNothing)
	})
	return rep, err
}

func (m *llmProvidersMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM llm_providers", "SELECT COUNT(*) FROM llm_providers", false)
	return rep, nil
}

// ---------------------------------------------------------------------------
// llm_proxies (status dropped; openapi_spec opaque; top-level upstreamAuth
// secret + policy split; data_version 1.1)
// ---------------------------------------------------------------------------

type llmProxiesMigrator struct{ baseMigrator }

func (m *llmProxiesMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT a.uuid, a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at,
		        p.project_uuid, p.description, p.created_by, p.provider_uuid, p.openapi_spec, p.configuration
		 FROM artifacts a JOIN llm_proxies p ON a.uuid = p.uuid`)
	if err != nil {
		return nil, err
	}
	var items []artifactTypeRow
	for rows.Next() {
		var r artifactTypeRow
		if err := rows.Scan(&r.uuid, &r.handle, &r.name, &r.version, &r.orgUUID, &r.createdAt, &r.updatedAt,
			&r.projectUUID, &r.description, &r.createdBy, &r.providerUUID, &r.openapiSpec, &r.configuration); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, r)
	}
	rows.Close()
	rep.SrcCount = int64(len(items))

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r artifactTypeRow) error {
		handle, err := rc.Kernels.carryHandle("llm_proxies", r.uuid, r.handle)
		if err != nil {
			return err
		}
		actor, err := rc.Kernels.resolveActor(ctx, q, nullToString(r.createdBy))
		if err != nil {
			return err
		}
		createdAt := artifactCreatedAt(r)
		blob, err := rc.Kernels.transformArtifactConfig(ctx, q, constants.LLMProxy, r.uuid, r.orgUUID, actor, createdAt, r.configuration, "")
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "llm_proxies",
			[]string{"uuid", "handle", "display_name", "version", "project_uuid", "description", "created_by", "updated_by", "provider_uuid",
				"openapi_spec", "configuration", "origin", "data_version", "created_at", "updated_at", "organization_uuid"},
			[]any{r.uuid, handle, r.name, r.version, nullOrString(r.projectUUID), nullOrString(r.description), actor, actor, nullOrString(r.providerUUID),
				textToBytea(r.openapiSpec), blob, constants.OriginCP, "1.1", createdAt, artifactUpdatedAt(r), r.orgUUID},
			conflictUUIDNothing)
	})
	return rep, err
}

func (m *llmProxiesMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM llm_proxies", "SELECT COUNT(*) FROM llm_proxies", false)
	return rep, nil
}

// ---------------------------------------------------------------------------
// mcp_proxies (status dropped; secret externalization upstream.main; dv 1.0)
// ---------------------------------------------------------------------------

type mcpProxiesMigrator struct{ baseMigrator }

func (m *mcpProxiesMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Direction == DirectionReverse {
		return m.migrateReverse(ctx, rc, rep)
	}
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT a.uuid, a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at,
		        p.project_uuid, p.description, p.created_by, p.configuration
		 FROM artifacts a JOIN mcp_proxies p ON a.uuid = p.uuid`)
	if err != nil {
		return nil, err
	}
	var items []artifactTypeRow
	for rows.Next() {
		var r artifactTypeRow
		if err := rows.Scan(&r.uuid, &r.handle, &r.name, &r.version, &r.orgUUID, &r.createdAt, &r.updatedAt,
			&r.projectUUID, &r.description, &r.createdBy, &r.configuration); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, r)
	}
	rows.Close()
	rep.SrcCount = int64(len(items))

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r artifactTypeRow) error {
		handle, err := rc.Kernels.carryHandle("mcp_proxies", r.uuid, r.handle)
		if err != nil {
			return err
		}
		actor, err := rc.Kernels.resolveActor(ctx, q, nullToString(r.createdBy))
		if err != nil {
			return err
		}
		createdAt := artifactCreatedAt(r)
		blob, err := rc.Kernels.transformArtifactConfig(ctx, q, constants.MCPProxy, r.uuid, r.orgUUID, actor, createdAt, r.configuration, "")
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, "mcp_proxies",
			[]string{"uuid", "handle", "display_name", "version", "project_uuid", "description", "created_by", "updated_by",
				"configuration", "origin", "data_version", "created_at", "updated_at", "organization_uuid"},
			[]any{r.uuid, handle, r.name, r.version, nullOrString(r.projectUUID), nullOrString(r.description), actor, actor,
				blob, constants.OriginCP, "1.0", createdAt, artifactUpdatedAt(r), r.orgUUID},
			conflictUUIDNothing)
	})
	return rep, err
}

func (m *mcpProxiesMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM mcp_proxies", "SELECT COUNT(*) FROM mcp_proxies", false)
	return rep, nil
}

// ---------------------------------------------------------------------------
// websub_apis / webbroker_apis (eventgateway plugin) — NO secret handling; fold
// the transport column into the config blob; carry lifecycle_status; dv 1.0.
// ---------------------------------------------------------------------------

type websubApisMigrator struct{ baseMigrator }

func (m *websubApisMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	return migratePluginArtifact(ctx, rc, newReport(m.name), m.name, "websub_apis", constants.WebSubApi)
}
func (m *websubApisMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM websub_apis", "SELECT COUNT(*) FROM websub_apis", false)
	return rep, nil
}

type webbrokerApisMigrator struct{ baseMigrator }

func (m *webbrokerApisMigrator) Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	return migratePluginArtifact(ctx, rc, newReport(m.name), m.name, "webbroker_apis", constants.WebBrokerApi)
}
func (m *webbrokerApisMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	verifyRowCounts(ctx, rc, rep, "SELECT COUNT(*) FROM webbroker_apis", "SELECT COUNT(*) FROM webbroker_apis", false)
	return rep, nil
}

// migratePluginArtifact handles the shared websub/webbroker shape (identical
// column set + bind order, transport fold, zero secrets).
func migratePluginArtifact(ctx context.Context, rc *RunContext, rep *ResourceReport, name, table, kind string) (*ResourceReport, error) {
	if rc.Direction == DirectionReverse {
		rep.Status = StatusSkip
		return rep, ErrReverseUnsupported
	}
	rows, err := rc.Src.QueryContext(ctx,
		`SELECT a.uuid, a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at,
		        p.project_uuid, p.description, p.created_by, p.lifecycle_status, p.transport, p.configuration
		 FROM artifacts a JOIN `+table+` p ON a.uuid = p.uuid`)
	if err != nil {
		return nil, err
	}
	var items []artifactTypeRow
	for rows.Next() {
		var r artifactTypeRow
		if err := rows.Scan(&r.uuid, &r.handle, &r.name, &r.version, &r.orgUUID, &r.createdAt, &r.updatedAt,
			&r.projectUUID, &r.description, &r.createdBy, &r.lifecycleStatus, &r.transport, &r.configuration); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, r)
	}
	rows.Close()
	rep.SrcCount = int64(len(items))

	err = runResource(ctx, rc, items, func(ctx context.Context, q queryer, r artifactTypeRow) error {
		handle, err := rc.Kernels.carryHandle(table, r.uuid, r.handle)
		if err != nil {
			return err
		}
		actor, err := rc.Kernels.resolveActor(ctx, q, nullToString(r.createdBy))
		if err != nil {
			return err
		}
		createdAt := artifactCreatedAt(r)
		blob, err := rc.Kernels.transformArtifactConfig(ctx, q, kind, r.uuid, r.orgUUID, actor, createdAt, r.configuration, nullToString(r.transport))
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}
		return insertRow(ctx, q, table,
			[]string{"uuid", "organization_uuid", "handle", "display_name", "version", "project_uuid", "description", "created_by", "updated_by",
				"lifecycle_status", "configuration", "origin", "data_version", "created_at", "updated_at"},
			[]any{r.uuid, r.orgUUID, handle, r.name, r.version, nullOrString(r.projectUUID), nullOrString(r.description), actor, actor,
				nullOrString(r.lifecycleStatus), blob, constants.OriginCP, "1.0", createdAt, artifactUpdatedAt(r)},
			conflictUUIDNothing)
	})
	return rep, err
}
