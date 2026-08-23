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

// This file is the BATCH-ONLY orchestration: table iteration in FK order, FK-parent
// gating, handle generation (persist-and-replay), subscription dedup, and drop
// enumeration. The per-row transform + v2 write is delegated to the shared
// migrationcore.UpsertX — the ONE implementation the live dual-write path also uses.

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/wso2/api-platform/platform-api/migrationcore"
)

// blobSkipped reports whether err is the core's "config blob unparseable" sentinel
// (already quarantined by the core) — the batch just skips the row.
func blobSkipped(err error) bool { return errors.Is(err, migrationcore.ErrBlobUnparseable) }

// parentOK classifies a child's FK reference against an in-memory parent set and
// routes violations: cascade (parent seen in v1 but not migrated) → quarantine;
// missing (absent from v1) → fail-fast if v1 enforced the FK, else quarantine.
func (mc *migCtx) parentOK(ps *ParentSet, parentUUID, childTable, childKey, fk string, enforced bool, fullRow any) (bool, error) {
	// In a targeted reconcile the in-memory parent sets only hold the filtered rows, so
	// the usual FK gating would wrongly cascade-quarantine children whose parents already
	// exist in v2 from the backfill. Skip gating and let v2's FKs reject a genuinely
	// missing parent (surfaced as the upsert error). No-op for the batch (reconcile=false).
	if mc.reconcile {
		return true, nil
	}
	switch ps.status(parentUUID) {
	case ParentOK:
		return true, nil
	case ParentCascade:
		mc.run.quarantine(childTable, childKey, ReasonOrphanFK,
			fmt.Sprintf("%s parent %s exists in v1 but was not migrated", fk, parentUUID), fullRow)
		return false, nil
	default: // ParentMissing
		if enforced {
			return false, fmt.Errorf("fail-fast: %s.%s=%s is absent from v1 (v1-enforced FK ⇒ source corruption); key=%s",
				childTable, fk, parentUUID, childKey)
		}
		mc.run.quarantine(childTable, childKey, ReasonOrphanFK,
			fmt.Sprintf("%s references %s=%s not present in v1", childTable, fk, parentUUID), fullRow)
		return false, nil
	}
}

// ---- organizations ----

func migrateOrganizations(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT uuid, handle, name, region, created_at, updated_at FROM organizations`))
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var uuid, handle, name, region string
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&uuid, &handle, &name, &region, &createdAt, &updatedAt); err != nil {
			return err
		}
		mc.sets.orgs.seenV1(uuid)
		h, err := mc.carriedHandle("organizations", "", uuid, handle)
		if err != nil {
			mc.run.quarantine("organizations", uuid, ReasonHandleUnresolvable, err.Error(),
				map[string]any{"uuid": uuid, "handle": handle, "name": name})
			continue
		}
		if !mc.want("organizations", uuid) {
			continue
		}
		if err := migrationcore.UpsertOrganization(mc.v2, migrationcore.OrganizationRow{
			UUID: uuid, Handle: h, DisplayName: name, Region: region,
			CreatedAt: ntp(createdAt), UpdatedAt: ntp(updatedAt),
		}, mc.coreOpts(), mc.run); err != nil {
			return err
		}
		mc.sets.orgs.insertedV2(uuid)
		n++
	}
	mc.run.addProcessed("organizations", n)
	return rows.Err()
}

// ---- projects ----

func migrateProjects(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT uuid, name, organization_uuid, description, created_at, updated_at FROM projects`))
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var uuid, name, org string
		var description sql.NullString
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&uuid, &name, &org, &description, &createdAt, &updatedAt); err != nil {
			return err
		}
		mc.sets.projects.seenV1(uuid)
		full := map[string]any{"uuid": uuid, "name": name, "organization_uuid": org}
		if ok, err := mc.parentOK(mc.sets.orgs, org, "projects", uuid, "organization_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		h, err := mc.h.generate("projects", org, uuid, name)
		if err != nil {
			mc.run.quarantine("projects", uuid, ReasonHandleUnresolvable, err.Error(), full)
			continue
		}
		mc.run.flag("projects", uuid, FlagSynthesized, nil, map[string]any{"handle": h, "note": "generated from name"})
		if !mc.want("projects", uuid) {
			continue
		}
		if err := migrationcore.UpsertProject(mc.v2, migrationcore.ProjectRow{
			UUID: uuid, Handle: h, DisplayName: name, Org: org, Description: nsp(description),
			CreatedAt: ntp(createdAt), UpdatedAt: ntp(updatedAt),
		}, mc.coreOpts(), mc.run); err != nil {
			return err
		}
		mc.sets.projects.insertedV2(uuid)
		n++
	}
	mc.run.addProcessed("projects", n)
	return rows.Err()
}

// ---- applications ----

func migrateApplications(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT uuid, handle, project_uuid, organization_uuid, created_by, name, description, type, created_at, updated_at FROM applications`))
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var uuid, handle, projectUUID, org, name, typ string
		var createdByRaw, description sql.NullString
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&uuid, &handle, &projectUUID, &org, &createdByRaw, &name, &description, &typ, &createdAt, &updatedAt); err != nil {
			return err
		}
		mc.sets.applications.seenV1(uuid)
		full := map[string]any{"uuid": uuid, "handle": handle, "organization_uuid": org, "project_uuid": projectUUID, "name": name}
		if ok, err := mc.parentOK(mc.sets.orgs, org, "applications", uuid, "organization_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if ok, err := mc.parentOK(mc.sets.projects, projectUUID, "applications", uuid, "project_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		h, err := mc.carriedHandle("applications", org, uuid, handle)
		if err != nil {
			mc.run.quarantine("applications", uuid, ReasonHandleUnresolvable, err.Error(), full)
			continue
		}
		if !mc.want("applications", uuid) {
			continue
		}
		if err := migrationcore.UpsertApplication(mc.v2, migrationcore.ApplicationRow{
			UUID: uuid, Handle: h, ProjectUUID: projectUUID, Org: org, DisplayName: name, Type: typ,
			Description: nsp(description), CreatedAt: ntp(createdAt), UpdatedAt: ntp(updatedAt), CreatedBy: strv(createdByRaw),
		}, mc.coreOpts(), mc.run); err != nil {
			return err
		}
		mc.sets.applications.insertedV2(uuid)
		n++
	}
	mc.run.addProcessed("applications", n)
	return rows.Err()
}

// ---- rest_apis (RestApi) ----

func migrateRestAPIs(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT t.uuid, t.description, t.created_by, t.project_uuid, t.lifecycle_status, t.transport, t.configuration,
		        a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at
		 FROM rest_apis t INNER JOIN artifacts a ON t.uuid = a.uuid`))
	if err != nil {
		return err
	}
	defer rows.Close()
	na, nt := 0, 0
	for rows.Next() {
		var uuid, projectUUID, handle, name, version, org string
		var description, createdByRaw, lifecycle, transport sql.NullString
		var config []byte
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&uuid, &description, &createdByRaw, &projectUUID, &lifecycle, &transport, &config,
			&handle, &name, &version, &org, &createdAt, &updatedAt); err != nil {
			return err
		}
		mc.sets.artifacts.seenV1(uuid)
		full := map[string]any{"uuid": uuid, "handle": handle, "name": name, "organization_uuid": org}
		if ok, err := mc.parentOK(mc.sets.orgs, org, "rest_apis", uuid, "organization_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if ok, err := mc.parentOK(mc.sets.projects, projectUUID, "rest_apis", uuid, "project_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		h, err := mc.carriedHandle("rest_apis", org, uuid, handle)
		if err != nil {
			mc.run.quarantine("rest_apis", uuid, ReasonHandleUnresolvable, err.Error(), full)
			continue
		}
		if !mc.want("rest_apis", uuid) {
			continue
		}
		if err := migrationcore.UpsertRestAPI(mc.v2, migrationcore.RestAPIRow{
			UUID: uuid, Handle: h, DisplayName: name, Version: version, Org: org, ProjectUUID: projectUUID,
			Description: nsp(description), Lifecycle: nsp(lifecycle), Transport: nsp(transport), Configuration: config,
			CreatedAt: ntp(createdAt), UpdatedAt: ntp(updatedAt), CreatedBy: strv(createdByRaw),
		}, mc.coreOpts(), mc.run); err != nil {
			if blobSkipped(err) {
				continue
			}
			return err
		}
		mc.sets.artifacts.insertedV2(uuid)
		mc.collectArtifactPlans(uuid, org, config)
		na++
		nt++
	}
	mc.run.addProcessed("artifacts", na)
	mc.run.addProcessed("rest_apis", nt)
	return rows.Err()
}

// ---- llm_provider_templates ----

func migrateLLMProviderTemplates(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT uuid, organization_uuid, handle, name, description, created_by, configuration, created_at, updated_at
		 FROM llm_provider_templates`))
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var uuid, org, handle, name, config string
		var description, createdByRaw sql.NullString
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&uuid, &org, &handle, &name, &description, &createdByRaw, &config, &createdAt, &updatedAt); err != nil {
			return err
		}
		full := map[string]any{"uuid": uuid, "handle": handle, "name": name, "organization_uuid": org}
		if ok, err := mc.parentOK(mc.sets.orgs, org, "llm_provider_templates", uuid, "organization_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		h, err := mc.carriedHandle("llm_provider_templates", org, uuid, handle)
		if err != nil {
			mc.run.quarantine("llm_provider_templates", uuid, ReasonHandleUnresolvable, err.Error(), full)
			continue
		}
		if !mc.want("llm_provider_templates", uuid) {
			continue
		}
		if err := migrationcore.UpsertLLMProviderTemplate(mc.v2, migrationcore.LLMTemplateRow{
			UUID: uuid, Org: org, Handle: h, DisplayName: name, Description: nsp(description),
			Configuration: []byte(config), CreatedAt: ntp(createdAt), UpdatedAt: ntp(updatedAt), CreatedBy: strv(createdByRaw),
		}, mc.coreOpts(), mc.run); err != nil {
			return err
		}
		mc.sets.templates.insertedV2(uuid)
		n++
	}
	mc.run.addProcessed("llm_provider_templates", n)
	return rows.Err()
}

// ---- llm_providers (LlmProvider) ----

func migrateLLMProviders(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT t.uuid, t.description, t.created_by, t.template_uuid, t.openapi_spec, t.model_list, t.status, t.configuration,
		        a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at
		 FROM llm_providers t INNER JOIN artifacts a ON t.uuid = a.uuid`))
	if err != nil {
		return err
	}
	defer rows.Close()
	na, nt := 0, 0
	for rows.Next() {
		var uuid, templateUUID, handle, name, version, org string
		var description, createdByRaw, openapiSpec, modelList, status sql.NullString
		var config []byte
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&uuid, &description, &createdByRaw, &templateUUID, &openapiSpec, &modelList, &status, &config,
			&handle, &name, &version, &org, &createdAt, &updatedAt); err != nil {
			return err
		}
		mc.sets.artifacts.seenV1(uuid)
		full := map[string]any{"uuid": uuid, "handle": handle, "organization_uuid": org, "template_uuid": templateUUID}
		if ok, err := mc.parentOK(mc.sets.orgs, org, "llm_providers", uuid, "organization_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if ok, err := mc.parentOK(mc.sets.templates, templateUUID, "llm_providers", uuid, "template_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		h, err := mc.carriedHandle("llm_providers", org, uuid, handle)
		if err != nil {
			mc.run.quarantine("llm_providers", uuid, ReasonHandleUnresolvable, err.Error(), full)
			continue
		}
		if !mc.want("llm_providers", uuid) {
			continue
		}
		if err := migrationcore.UpsertLLMProvider(mc.v2, migrationcore.LLMProviderRow{
			UUID: uuid, Handle: h, DisplayName: name, Version: version, Org: org, TemplateUUID: templateUUID,
			Description: nsp(description), OpenAPISpec: nsp(openapiSpec), ModelList: nsp(modelList), Status: nsp(status),
			Configuration: config, CreatedAt: ntp(createdAt), UpdatedAt: ntp(updatedAt), CreatedBy: strv(createdByRaw),
		}, mc.coreOpts(), mc.run); err != nil {
			if blobSkipped(err) {
				continue
			}
			return err
		}
		mc.sets.artifacts.insertedV2(uuid)
		mc.sets.providers.insertedV2(uuid)
		na++
		nt++
	}
	mc.run.addProcessed("artifacts", na)
	mc.run.addProcessed("llm_providers", nt)
	return rows.Err()
}

// ---- llm_proxies (LlmProxy) ----

func migrateLLMProxies(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT t.uuid, t.project_uuid, t.description, t.created_by, t.provider_uuid, t.openapi_spec, t.status, t.configuration,
		        a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at
		 FROM llm_proxies t INNER JOIN artifacts a ON t.uuid = a.uuid`))
	if err != nil {
		return err
	}
	defer rows.Close()
	na, nt := 0, 0
	for rows.Next() {
		var uuid, projectUUID, providerUUID, handle, name, version, org string
		var description, createdByRaw, openapiSpec, status sql.NullString
		var config []byte
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&uuid, &projectUUID, &description, &createdByRaw, &providerUUID, &openapiSpec, &status, &config,
			&handle, &name, &version, &org, &createdAt, &updatedAt); err != nil {
			return err
		}
		mc.sets.artifacts.seenV1(uuid)
		full := map[string]any{"uuid": uuid, "handle": handle, "organization_uuid": org, "provider_uuid": providerUUID}
		if ok, err := mc.parentOK(mc.sets.orgs, org, "llm_proxies", uuid, "organization_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if ok, err := mc.parentOK(mc.sets.projects, projectUUID, "llm_proxies", uuid, "project_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if ok, err := mc.parentOK(mc.sets.providers, providerUUID, "llm_proxies", uuid, "provider_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		h, err := mc.carriedHandle("llm_proxies", org, uuid, handle)
		if err != nil {
			mc.run.quarantine("llm_proxies", uuid, ReasonHandleUnresolvable, err.Error(), full)
			continue
		}
		if !mc.want("llm_proxies", uuid) {
			continue
		}
		if err := migrationcore.UpsertLLMProxy(mc.v2, migrationcore.LLMProxyRow{
			UUID: uuid, Handle: h, DisplayName: name, Version: version, ProjectUUID: projectUUID, Org: org, ProviderUUID: providerUUID,
			Description: nsp(description), OpenAPISpec: nsp(openapiSpec), Status: nsp(status),
			Configuration: config, CreatedAt: ntp(createdAt), UpdatedAt: ntp(updatedAt), CreatedBy: strv(createdByRaw),
		}, mc.coreOpts(), mc.run); err != nil {
			if blobSkipped(err) {
				continue
			}
			return err
		}
		mc.sets.artifacts.insertedV2(uuid)
		na++
		nt++
	}
	mc.run.addProcessed("artifacts", na)
	mc.run.addProcessed("llm_proxies", nt)
	return rows.Err()
}

// ---- mcp_proxies (Mcp) ----

func migrateMCPProxies(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT t.uuid, t.project_uuid, t.description, t.created_by, t.status, t.configuration,
		        a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at
		 FROM mcp_proxies t INNER JOIN artifacts a ON t.uuid = a.uuid`))
	if err != nil {
		return err
	}
	defer rows.Close()
	na, nt := 0, 0
	for rows.Next() {
		var uuid, handle, name, version, org string
		var projectUUID, description, createdByRaw, status sql.NullString
		var config []byte
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&uuid, &projectUUID, &description, &createdByRaw, &status, &config,
			&handle, &name, &version, &org, &createdAt, &updatedAt); err != nil {
			return err
		}
		mc.sets.artifacts.seenV1(uuid)
		full := map[string]any{"uuid": uuid, "handle": handle, "organization_uuid": org}
		if ok, err := mc.parentOK(mc.sets.orgs, org, "mcp_proxies", uuid, "organization_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if projectUUID.Valid && projectUUID.String != "" {
			if ok, err := mc.parentOK(mc.sets.projects, projectUUID.String, "mcp_proxies", uuid, "project_uuid", true, full); err != nil {
				return err
			} else if !ok {
				continue
			}
		}
		h, err := mc.carriedHandle("mcp_proxies", org, uuid, handle)
		if err != nil {
			mc.run.quarantine("mcp_proxies", uuid, ReasonHandleUnresolvable, err.Error(), full)
			continue
		}
		if !mc.want("mcp_proxies", uuid) {
			continue
		}
		if err := migrationcore.UpsertMCPProxy(mc.v2, migrationcore.MCPProxyRow{
			UUID: uuid, Handle: h, DisplayName: name, Version: version, Org: org,
			ProjectUUID: nsp(projectUUID), Description: nsp(description), Status: nsp(status),
			Configuration: config, CreatedAt: ntp(createdAt), UpdatedAt: ntp(updatedAt), CreatedBy: strv(createdByRaw),
		}, mc.coreOpts(), mc.run); err != nil {
			if blobSkipped(err) {
				continue
			}
			return err
		}
		mc.sets.artifacts.insertedV2(uuid)
		na++
		nt++
	}
	mc.run.addProcessed("artifacts", na)
	mc.run.addProcessed("mcp_proxies", nt)
	return rows.Err()
}

// ---- websub_apis (WebSubApi, plugin table) ----

func migrateWebSubAPIs(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT t.uuid, t.project_uuid, t.description, t.created_by, t.lifecycle_status, t.transport, t.configuration,
		        a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at
		 FROM websub_apis t INNER JOIN artifacts a ON t.uuid = a.uuid`))
	if err != nil {
		return err
	}
	defer rows.Close()
	na, nt := 0, 0
	for rows.Next() {
		var uuid, projectUUID, handle, name, version, org string
		var description, createdByRaw, lifecycle, transport sql.NullString
		var config []byte
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&uuid, &projectUUID, &description, &createdByRaw, &lifecycle, &transport, &config,
			&handle, &name, &version, &org, &createdAt, &updatedAt); err != nil {
			return err
		}
		mc.sets.artifacts.seenV1(uuid)
		full := map[string]any{"uuid": uuid, "handle": handle, "organization_uuid": org}
		if ok, err := mc.parentOK(mc.sets.orgs, org, "websub_apis", uuid, "organization_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if ok, err := mc.parentOK(mc.sets.projects, projectUUID, "websub_apis", uuid, "project_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		h, err := mc.carriedHandle("websub_apis", org, uuid, handle)
		if err != nil {
			mc.run.quarantine("websub_apis", uuid, ReasonHandleUnresolvable, err.Error(), full)
			continue
		}
		if !mc.want("websub_apis", uuid) {
			continue
		}
		if err := migrationcore.UpsertWebSubAPI(mc.v2, migrationcore.WebSubRow{
			UUID: uuid, Handle: h, DisplayName: name, Version: version, Org: org, ProjectUUID: projectUUID,
			Description: nsp(description), Lifecycle: nsp(lifecycle), Transport: nsp(transport), Configuration: config,
			CreatedAt: ntp(createdAt), UpdatedAt: ntp(updatedAt), CreatedBy: strv(createdByRaw),
		}, mc.coreOpts(), mc.run); err != nil {
			if blobSkipped(err) {
				continue
			}
			return err
		}
		mc.sets.artifacts.insertedV2(uuid)
		mc.collectArtifactPlans(uuid, org, config)
		na++
		nt++
	}
	mc.run.addProcessed("artifacts", na)
	mc.run.addProcessed("websub_apis", nt)
	return rows.Err()
}

// ---- webbroker_apis (WebBrokerApi, plugin table) ----

func migrateWebBrokerAPIs(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT t.uuid, t.project_uuid, t.description, t.created_by, t.lifecycle_status, t.transport, t.configuration,
		        a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at
		 FROM webbroker_apis t INNER JOIN artifacts a ON t.uuid = a.uuid`))
	if err != nil {
		return err
	}
	defer rows.Close()
	na, nt := 0, 0
	for rows.Next() {
		var uuid, projectUUID, handle, name, version, org string
		var description, createdByRaw, lifecycle, transport sql.NullString
		var config []byte
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&uuid, &projectUUID, &description, &createdByRaw, &lifecycle, &transport, &config,
			&handle, &name, &version, &org, &createdAt, &updatedAt); err != nil {
			return err
		}
		mc.sets.artifacts.seenV1(uuid)
		full := map[string]any{"uuid": uuid, "handle": handle, "organization_uuid": org}
		if ok, err := mc.parentOK(mc.sets.orgs, org, "webbroker_apis", uuid, "organization_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if ok, err := mc.parentOK(mc.sets.projects, projectUUID, "webbroker_apis", uuid, "project_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		h, err := mc.carriedHandle("webbroker_apis", org, uuid, handle)
		if err != nil {
			mc.run.quarantine("webbroker_apis", uuid, ReasonHandleUnresolvable, err.Error(), full)
			continue
		}
		if !mc.want("webbroker_apis", uuid) {
			continue
		}
		if err := migrationcore.UpsertWebBrokerAPI(mc.v2, migrationcore.WebBrokerRow{
			UUID: uuid, Handle: h, DisplayName: name, Version: version, Org: org, ProjectUUID: projectUUID,
			Description: nsp(description), Lifecycle: nsp(lifecycle), Transport: nsp(transport), Configuration: config,
			CreatedAt: ntp(createdAt), UpdatedAt: ntp(updatedAt), CreatedBy: strv(createdByRaw),
		}, mc.coreOpts(), mc.run); err != nil {
			if blobSkipped(err) {
				continue
			}
			return err
		}
		mc.sets.artifacts.insertedV2(uuid)
		mc.collectArtifactPlans(uuid, org, config)
		na++
		nt++
	}
	mc.run.addProcessed("artifacts", na)
	mc.run.addProcessed("webbroker_apis", nt)
	return rows.Err()
}

// ---- subscription_plans (+ subscription_plan_limits) ----

func migrateSubscriptionPlans(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT uuid, plan_name, billing_plan, stop_on_quota_reach, throttle_limit_count, throttle_limit_unit,
		        expiry_time, organization_uuid, status, created_at, updated_at
		 FROM subscription_plans`))
	if err != nil {
		return err
	}
	defer rows.Close()
	np, nl := 0, 0
	for rows.Next() {
		var uuid, planName, org, status string
		var billingPlan, throttleUnit sql.NullString
		var stopOnQuota sql.NullBool
		var throttleCount sql.NullInt64
		var expiry, createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&uuid, &planName, &billingPlan, &stopOnQuota, &throttleCount, &throttleUnit,
			&expiry, &org, &status, &createdAt, &updatedAt); err != nil {
			return err
		}
		mc.sets.plans.seenV1(uuid)
		full := map[string]any{"uuid": uuid, "plan_name": planName, "organization_uuid": org}
		if ok, err := mc.parentOK(mc.sets.orgs, org, "subscription_plans", uuid, "organization_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		h, err := mc.h.generate("subscription_plans", org, uuid, planName)
		if err != nil {
			mc.run.quarantine("subscription_plans", uuid, ReasonHandleUnresolvable, err.Error(), full)
			continue
		}
		mc.run.flag("subscription_plans", uuid, FlagSynthesized, nil, map[string]any{"handle": h, "note": "generated from plan_name"})
		row := migrationcore.SubscriptionPlanRow{
			UUID: uuid, Handle: h, DisplayName: planName, Org: org, Status: status,
			BillingPlan: nsp(billingPlan), ThrottleUnit: nsp(throttleUnit),
			ExpiryTime: ntp(expiry), CreatedAt: ntp(createdAt), UpdatedAt: ntp(updatedAt),
		}
		if stopOnQuota.Valid {
			b := stopOnQuota.Bool
			row.StopOnQuota = &b
		}
		if throttleCount.Valid {
			c := throttleCount.Int64
			row.ThrottleCount = &c
		}
		if !mc.want("subscription_plans", uuid) {
			continue
		}
		if err := migrationcore.UpsertSubscriptionPlan(mc.v2, row, mc.coreOpts(), mc.run); err != nil {
			var unmapped migrationcore.ErrUnmappedThrottleUnit
			if errors.As(err, &unmapped) {
				return fmt.Errorf("fail-fast: subscription_plan %s has %w", uuid, err)
			}
			return err
		}
		mc.sets.plans.insertedV2(uuid)
		np++
		if throttleCount.Valid && throttleUnit.Valid && throttleUnit.String != "" {
			nl++
		}
	}
	mc.run.addProcessed("subscription_plans", np)
	mc.run.addProcessed("subscription_plan_limits", nl)
	return rows.Err()
}

// ---- subscriptions ----

func migrateSubscriptions(mc *migCtx) error {
	if mc.subAppSeen == nil {
		mc.subAppSeen = map[string]bool{}
		mc.subHashSeen = map[string]bool{}
	}
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT uuid, api_uuid, subscriber_id, application_id, subscription_token, subscription_token_hash,
		        subscription_plan_uuid, organization_uuid, status, created_at, updated_at
		 FROM subscriptions`))
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var uuid, apiUUID, subscriberID, token, hash, org, status string
		var applicationID, planUUID sql.NullString
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&uuid, &apiUUID, &subscriberID, &applicationID, &token, &hash,
			&planUUID, &org, &status, &createdAt, &updatedAt); err != nil {
			return err
		}
		full := map[string]any{"uuid": uuid, "api_uuid": apiUUID, "organization_uuid": org, "subscriber_id": subscriberID}
		if ok, err := mc.parentOK(mc.sets.artifacts, apiUUID, "subscriptions", uuid, "api_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if planUUID.Valid && planUUID.String != "" {
			if ok, err := mc.parentOK(mc.sets.plans, planUUID.String, "subscriptions", uuid, "subscription_plan_uuid", true, full); err != nil {
				return err
			} else if !ok {
				continue
			}
		}
		hkey := apiUUID + "|" + hash
		if mc.subHashSeen[hkey] {
			mc.run.quarantine("subscriptions", uuid, ReasonDupKey, "duplicate (artifact_uuid, subscription_token_hash)", full)
			continue
		}
		if applicationID.Valid && applicationID.String != "" {
			akey := org + "|" + apiUUID + "|" + applicationID.String
			if mc.subAppSeen[akey] {
				mc.run.quarantine("subscriptions", uuid, ReasonDupKey,
					"duplicate (organization_uuid, artifact_uuid, application_id) — v1 enforced subscriber_id, not application_id", full)
				continue
			}
			mc.subAppSeen[akey] = true
		}
		mc.subHashSeen[hkey] = true
		if !mc.want("subscriptions", uuid) {
			continue
		}
		if err := migrationcore.UpsertSubscription(mc.v2, migrationcore.SubscriptionRow{
			UUID: uuid, ArtifactUUID: apiUUID, SubscriberID: subscriberID, Token: token, Hash: hash, Org: org, Status: status,
			ApplicationID: nsp(applicationID), PlanUUID: nsp(planUUID), CreatedAt: ntp(createdAt), UpdatedAt: ntp(updatedAt),
		}, mc.coreOpts(), mc.run); err != nil {
			return err
		}
		n++
	}
	mc.run.addProcessed("subscriptions", n)
	return rows.Err()
}

// ---- gateways (+ gateway_endpoints) ----

func migrateGateways(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT uuid, organization_uuid, name, version, display_name, description, properties, vhost,
		        is_critical, gateway_functionality_type, is_active, manifest, created_at, updated_at
		 FROM gateways`))
	if err != nil {
		return err
	}
	defer rows.Close()
	ng, ne := 0, 0
	for rows.Next() {
		var uuid, org, name, version, displayName, funcType, vhost string
		var description sql.NullString
		var properties, manifest []byte
		var isCritical, isActive sql.NullBool
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&uuid, &org, &name, &version, &displayName, &description, &properties, &vhost,
			&isCritical, &funcType, &isActive, &manifest, &createdAt, &updatedAt); err != nil {
			return err
		}
		mc.sets.gateways.seenV1(uuid)
		full := map[string]any{"uuid": uuid, "name": name, "organization_uuid": org, "vhost": vhost}
		if ok, err := mc.parentOK(mc.sets.orgs, org, "gateways", uuid, "organization_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		h, err := mc.h.generate("gateways", org, uuid, name)
		if err != nil {
			mc.run.quarantine("gateways", uuid, ReasonHandleUnresolvable, err.Error(), full)
			continue
		}
		mc.run.flag("gateways", uuid, FlagSynthesized, nil, map[string]any{"handle": h, "note": "generated from name"})
		if !mc.want("gateways", uuid) {
			continue
		}
		if err := migrationcore.UpsertGateway(mc.v2, migrationcore.GatewayRow{
			UUID: uuid, Org: org, Handle: h, DisplayName: displayName, Version: version, FuncType: funcType, Vhost: vhost,
			Description: nsp(description), Properties: properties, Manifest: manifest,
			IsCritical: isCritical.Bool, IsActive: isActive.Bool,
			CreatedAt: ntp(createdAt), UpdatedAt: ntp(updatedAt),
		}, mc.coreOpts(), mc.run); err != nil {
			return err
		}
		mc.sets.gateways.insertedV2(uuid)
		ng++
		if strings.TrimSpace(vhost) != "" {
			ne++
		}
	}
	mc.run.addProcessed("gateways", ng)
	mc.run.addProcessed("gateway_endpoints", ne)
	return rows.Err()
}

// ---- artifact_gateway_mappings (from association_mappings gateway rows) ----

func migrateArtifactGatewayMappings(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT id, artifact_uuid, organization_uuid, resource_uuid, association_type, created_at, updated_at
		 FROM association_mappings`))
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var id int64
		var artifactUUID, org, resourceUUID, assocType string
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&id, &artifactUUID, &org, &resourceUUID, &assocType, &createdAt, &updatedAt); err != nil {
			return err
		}
		key := fmt.Sprintf("assoc:%d", id)
		full := map[string]any{"id": id, "artifact_uuid": artifactUUID, "organization_uuid": org, "resource_uuid": resourceUUID, "association_type": assocType}
		switch assocType {
		case "gateway":
			// migrate below
		case "dev_portal":
			mc.run.dropRow("association_mappings", key, DropDevPortalAssoc, fmt.Sprintf("artifact %s → resource %s", artifactUUID, resourceUUID))
			continue
		default:
			return fmt.Errorf("fail-fast: association_mappings id=%d has unmapped association_type %q", id, assocType)
		}
		if ok, err := mc.parentOK(mc.sets.artifacts, artifactUUID, "artifact_gateway_mappings", key, "artifact_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if ok, err := mc.parentOK(mc.sets.gateways, resourceUUID, "artifact_gateway_mappings", key, "gateway_uuid", false, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if !mc.want("artifact_gateway_mappings", org+"|"+artifactUUID+"|"+resourceUUID) {
			continue
		}
		if err := migrationcore.UpsertArtifactGatewayMapping(mc.v2, migrationcore.ArtifactGatewayMappingRow{
			ArtifactUUID: artifactUUID, Org: org, GatewayUUID: resourceUUID, CreatedAt: ntp(createdAt), UpdatedAt: ntp(updatedAt),
		}, mc.coreOpts(), mc.run); err != nil {
			return err
		}
		n++
	}
	mc.run.addProcessed("artifact_gateway_mappings", n)
	return rows.Err()
}

// ---- gateway_custom_policies ----

func migrateGatewayCustomPolicies(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT uuid, organization_uuid, name, display_name, version, description, policy_definition, created_at, updated_at
		 FROM gateway_custom_policies`))
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var uuid, org, name, version string
		var displayName, description sql.NullString
		var policyDef []byte
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&uuid, &org, &name, &displayName, &version, &description, &policyDef, &createdAt, &updatedAt); err != nil {
			return err
		}
		mc.sets.policies.seenV1(uuid)
		full := map[string]any{"uuid": uuid, "name": name, "organization_uuid": org}
		if ok, err := mc.parentOK(mc.sets.orgs, org, "gateway_custom_policies", uuid, "organization_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if !mc.want("gateway_custom_policies", uuid) {
			continue
		}
		if err := migrationcore.UpsertGatewayCustomPolicy(mc.v2, migrationcore.GatewayCustomPolicyRow{
			UUID: uuid, Org: org, Name: name, Version: version, DisplayName: nsp(displayName), Description: nsp(description),
			PolicyDefinition: policyDef, CreatedAt: ntp(createdAt), UpdatedAt: ntp(updatedAt),
		}, mc.coreOpts(), mc.run); err != nil {
			return err
		}
		mc.sets.policies.insertedV2(uuid)
		n++
	}
	mc.run.addProcessed("gateway_custom_policies", n)
	return rows.Err()
}

// ---- gateway_custom_policy_usages ----

func migrateGatewayCustomPolicyUsages(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(`SELECT policy_uuid, api_uuid FROM gateway_custom_policy_usages`))
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var policyUUID, apiUUID string
		if err := rows.Scan(&policyUUID, &apiUUID); err != nil {
			return err
		}
		key := policyUUID + "|" + apiUUID
		full := map[string]any{"policy_uuid": policyUUID, "api_uuid": apiUUID}
		if ok, err := mc.parentOK(mc.sets.policies, policyUUID, "gateway_custom_policy_usages", key, "policy_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if ok, err := mc.parentOK(mc.sets.artifacts, apiUUID, "gateway_custom_policy_usages", key, "artifact_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if !mc.want("gateway_custom_policy_usages", policyUUID+"|"+apiUUID) {
			continue
		}
		if err := migrationcore.UpsertPolicyUsage(mc.v2, migrationcore.PolicyUsageRow{PolicyUUID: policyUUID, ArtifactUUID: apiUUID}, mc.coreOpts(), mc.run); err != nil {
			return err
		}
		n++
	}
	mc.run.addProcessed("gateway_custom_policy_usages", n)
	return rows.Err()
}

// ---- gateway_tokens ----

func migrateGatewayTokens(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT uuid, gateway_uuid, token_hash, salt, status, created_at, revoked_at FROM gateway_tokens`))
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var uuid, gatewayUUID, tokenHash, salt, status string
		var createdAt, revokedAt sql.NullTime
		if err := rows.Scan(&uuid, &gatewayUUID, &tokenHash, &salt, &status, &createdAt, &revokedAt); err != nil {
			return err
		}
		full := map[string]any{"uuid": uuid, "gateway_uuid": gatewayUUID, "status": status}
		if ok, err := mc.parentOK(mc.sets.gateways, gatewayUUID, "gateway_tokens", uuid, "gateway_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if !mc.want("gateway_tokens", uuid) {
			continue
		}
		if err := migrationcore.UpsertGatewayToken(mc.v2, migrationcore.GatewayTokenRow{
			UUID: uuid, GatewayUUID: gatewayUUID, TokenHash: tokenHash, Salt: salt, Status: status,
			CreatedAt: ntp(createdAt), RevokedAt: ntp(revokedAt),
		}, mc.coreOpts(), mc.run); err != nil {
			return err
		}
		n++
	}
	mc.run.addProcessed("gateway_tokens", n)
	return rows.Err()
}

// ---- deployments (ordered by created_at so base_deployment predecessors exist first) ----

func migrateDeployments(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT deployment_id, name, artifact_uuid, organization_uuid, gateway_uuid, base_deployment_id,
		        content, metadata, created_at
		 FROM deployments ORDER BY created_at ASC NULLS FIRST`))
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var uuid, name, artifactUUID, org, gatewayUUID string
		var baseDeployment, metadata sql.NullString
		var content []byte
		var createdAt sql.NullTime
		if err := rows.Scan(&uuid, &name, &artifactUUID, &org, &gatewayUUID, &baseDeployment, &content, &metadata, &createdAt); err != nil {
			return err
		}
		mc.sets.deployments.seenV1(uuid)
		full := map[string]any{"deployment_id": uuid, "artifact_uuid": artifactUUID, "gateway_uuid": gatewayUUID, "organization_uuid": org}
		if ok, err := mc.parentOK(mc.sets.artifacts, artifactUUID, "deployments", uuid, "artifact_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if ok, err := mc.parentOK(mc.sets.gateways, gatewayUUID, "deployments", uuid, "gateway_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		var base *string
		if baseDeployment.Valid && baseDeployment.String != "" && mc.sets.deployments.status(baseDeployment.String) == ParentOK {
			s := baseDeployment.String
			base = &s
		}
		if !mc.want("deployments", uuid) {
			continue
		}
		if err := migrationcore.UpsertDeployment(mc.v2, migrationcore.DeploymentRow{
			UUID: uuid, DisplayName: name, ArtifactUUID: artifactUUID, Org: org, GatewayUUID: gatewayUUID,
			BaseDeploymentUUID: base, Metadata: nsp(metadata), Content: content, CreatedAt: ntp(createdAt),
		}, mc.coreOpts(), mc.run); err != nil {
			return err
		}
		mc.sets.deployments.insertedV2(uuid)
		n++
	}
	mc.run.addProcessed("deployments", n)
	return rows.Err()
}

// ---- deployment_status ----

func migrateDeploymentStatus(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT artifact_uuid, organization_uuid, gateway_uuid, deployment_id, status, status_desired,
		        performed_at, status_reason, updated_at
		 FROM deployment_status`))
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var artifactUUID, org, gatewayUUID, deploymentUUID, status string
		var statusDesired, statusReason sql.NullString
		var performedAt, updatedAt sql.NullTime
		if err := rows.Scan(&artifactUUID, &org, &gatewayUUID, &deploymentUUID, &status, &statusDesired, &performedAt, &statusReason, &updatedAt); err != nil {
			return err
		}
		key := artifactUUID + "|" + org + "|" + gatewayUUID
		full := map[string]any{"artifact_uuid": artifactUUID, "organization_uuid": org, "gateway_uuid": gatewayUUID, "deployment_id": deploymentUUID}
		if ok, err := mc.parentOK(mc.sets.artifacts, artifactUUID, "deployment_status", key, "artifact_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if ok, err := mc.parentOK(mc.sets.gateways, gatewayUUID, "deployment_status", key, "gateway_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if ok, err := mc.parentOK(mc.sets.deployments, deploymentUUID, "deployment_status", key, "deployment_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if !mc.want("deployment_status", org+"|"+artifactUUID+"|"+gatewayUUID) {
			continue
		}
		if err := migrationcore.UpsertDeploymentStatus(mc.v2, migrationcore.DeploymentStatusRow{
			ArtifactUUID: artifactUUID, Org: org, GatewayUUID: gatewayUUID, DeploymentUUID: deploymentUUID, Status: status,
			StatusDesired: nsp(statusDesired), StatusReason: nsp(statusReason), PerformedAt: ntp(performedAt), UpdatedAt: ntp(updatedAt),
		}, mc.coreOpts(), mc.run); err != nil {
			return err
		}
		n++
	}
	mc.run.addProcessed("deployment_status", n)
	return rows.Err()
}

// ---- api_keys ----

func migrateAPIKeys(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT uuid, artifact_uuid, name, masked_api_key, api_key_hashes, status, created_at, created_by,
		        updated_at, expires_at, issuer, allowed_targets
		 FROM api_keys`))
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var uuid, artifactUUID, name, maskedKey, apiKeyHashes, status, allowedTargets string
		var createdByRaw, issuer sql.NullString
		var createdAt, updatedAt, expiresAt sql.NullTime
		if err := rows.Scan(&uuid, &artifactUUID, &name, &maskedKey, &apiKeyHashes, &status, &createdAt, &createdByRaw,
			&updatedAt, &expiresAt, &issuer, &allowedTargets); err != nil {
			return err
		}
		mc.sets.apiKeys.seenV1(uuid)
		full := map[string]any{"uuid": uuid, "artifact_uuid": artifactUUID, "name": name}
		if ok, err := mc.parentOK(mc.sets.artifacts, artifactUUID, "api_keys", uuid, "artifact_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		h, err := mc.h.generate("api_keys", artifactUUID, uuid, name)
		if err != nil {
			mc.run.quarantine("api_keys", uuid, ReasonHandleUnresolvable, err.Error(), full)
			continue
		}
		mc.run.flag("api_keys", uuid, FlagSynthesized, nil, map[string]any{"handle": h, "note": "generated from name"})
		if !mc.want("api_keys", uuid) {
			continue
		}
		if err := migrationcore.UpsertAPIKey(mc.v2, migrationcore.APIKeyRow{
			UUID: uuid, ArtifactUUID: artifactUUID, Handle: h, DisplayName: name, MaskedKey: maskedKey,
			APIKeyHashes: apiKeyHashes, Status: status, AllowedTargets: allowedTargets, Issuer: nsp(issuer),
			CreatedAt: ntp(createdAt), UpdatedAt: ntp(updatedAt), ExpiresAt: ntp(expiresAt), CreatedBy: strv(createdByRaw),
		}, mc.coreOpts(), mc.run); err != nil {
			return err
		}
		mc.sets.apiKeys.insertedV2(uuid)
		n++
	}
	mc.run.addProcessed("api_keys", n)
	return rows.Err()
}

// ---- application_api_key_mappings ----

func migrateApplicationAPIKeyMappings(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT application_uuid, api_key_id, created_at FROM application_api_keys`))
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var appUUID, apiKeyID string
		var createdAt sql.NullTime
		if err := rows.Scan(&appUUID, &apiKeyID, &createdAt); err != nil {
			return err
		}
		key := appUUID + "|" + apiKeyID
		full := map[string]any{"application_uuid": appUUID, "api_key_id": apiKeyID}
		if ok, err := mc.parentOK(mc.sets.applications, appUUID, "application_api_key_mappings", key, "application_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if ok, err := mc.parentOK(mc.sets.apiKeys, apiKeyID, "application_api_key_mappings", key, "api_key_id", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if !mc.want("application_api_key_mappings", appUUID+"|"+apiKeyID) {
			continue
		}
		if err := migrationcore.UpsertApplicationAPIKeyMapping(mc.v2, migrationcore.ApplicationAPIKeyMappingRow{
			ApplicationUUID: appUUID, APIKeyID: apiKeyID, CreatedAt: ntp(createdAt),
		}, mc.coreOpts(), mc.run); err != nil {
			return err
		}
		n++
	}
	mc.run.addProcessed("application_api_key_mappings", n)
	return rows.Err()
}

// ---- application_artifact_mappings ----

func migrateApplicationArtifactMappings(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(
		`SELECT application_uuid, artifact_uuid, created_at FROM application_artifacts`))
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var appUUID, artifactUUID string
		var createdAt sql.NullTime
		if err := rows.Scan(&appUUID, &artifactUUID, &createdAt); err != nil {
			return err
		}
		key := appUUID + "|" + artifactUUID
		full := map[string]any{"application_uuid": appUUID, "artifact_uuid": artifactUUID}
		if ok, err := mc.parentOK(mc.sets.applications, appUUID, "application_artifact_mappings", key, "application_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if ok, err := mc.parentOK(mc.sets.artifacts, artifactUUID, "application_artifact_mappings", key, "artifact_uuid", true, full); err != nil {
			return err
		} else if !ok {
			continue
		}
		if !mc.want("application_artifact_mappings", appUUID+"|"+artifactUUID) {
			continue
		}
		if err := migrationcore.UpsertApplicationArtifactMapping(mc.v2, migrationcore.ApplicationArtifactMappingRow{
			ApplicationUUID: appUUID, ArtifactUUID: artifactUUID, CreatedAt: ntp(createdAt),
		}, mc.coreOpts(), mc.run); err != nil {
			return err
		}
		n++
	}
	mc.run.addProcessed("application_artifact_mappings", n)
	return rows.Err()
}

// ---- artifact_subscription_plans (optional; dead in v2 code) ----

func migrateArtifactSubscriptionPlans(mc *migCtx) error {
	if !mc.opts.PopulateArtifactSubPlans {
		mc.run.logger.Info("skipping artifact_subscription_plans (redundant; -populate-artifact-subscription-plans to enable)")
		return nil
	}
	planByHandle := map[string]string{}
	rows, err := mc.v2.Query(mc.v2.Rebind(`SELECT organization_uuid, handle, uuid FROM subscription_plans`))
	if err != nil {
		return err
	}
	for rows.Next() {
		var org, handle, uuid string
		if err := rows.Scan(&org, &handle, &uuid); err != nil {
			rows.Close()
			return err
		}
		planByHandle[org+"|"+handle] = uuid
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	n := 0
	for _, ap := range mc.artifactPlans {
		for _, ph := range ap.planHandles {
			planUUID, ok := planByHandle[ap.org+"|"+ph]
			if !ok {
				mc.run.flag("artifact_subscription_plans", ap.artifactUUID, FlagSynthesized,
					map[string]any{"plan_handle": ph}, map[string]any{"note": "config plan handle did not resolve to a migrated plan; skipped"})
				continue
			}
			if err := mc.insert("artifact_subscription_plans",
				[]string{"artifact_uuid", "subscription_plan_uuid", "created_by", "created_at"},
				[]any{ap.artifactUUID, planUUID, mc.im.actorUUID, mc.epoch},
				"(artifact_uuid, subscription_plan_uuid)"); err != nil {
				return err
			}
			n++
		}
	}
	mc.run.addProcessed("artifact_subscription_plans", n)
	return nil
}

// collectArtifactPlans records an artifact's config subscription-plan handles for
// the optional artifact_subscription_plans derivation (parsed from the v1 config).
func (mc *migCtx) collectArtifactPlans(artifactUUID, org string, rawConfig []byte) {
	if !mc.opts.PopulateArtifactSubPlans {
		return
	}
	var m struct {
		SubscriptionPlans []string `json:"subscriptionPlans"`
	}
	if json.Unmarshal(rawConfig, &m) == nil && len(m.SubscriptionPlans) > 0 {
		mc.artifactPlans = append(mc.artifactPlans, artifactPlanRef{artifactUUID: artifactUUID, org: org, planHandles: m.SubscriptionPlans})
	}
}

// ---- audit markers (optional) ----

func migrateAuditMarkers(mc *migCtx) error {
	if !mc.opts.AuditMarker {
		mc.run.logger.Info("skipping audit markers (-audit-marker to enable)")
		return nil
	}
	n := 0
	for org := range mc.sets.orgs.v2 {
		uuid := deterministicUUID("audit-migrated|"+org, mc.epoch)
		if err := mc.insert("audit",
			[]string{"uuid", "action", "resource_uuid", "resource_type", "organization_uuid", "performed_by", "performed_at"},
			[]any{uuid, "migrated", org, "organization", org, mc.im.actorUUID, mc.epoch},
			"(uuid)"); err != nil {
			return err
		}
		n++
	}
	mc.run.addProcessed("audit", n)
	return nil
}

// ---- intentional §H row drops (enumerate + report, never migrate) ----

func dropDevportals(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(`SELECT uuid, name FROM devportals`))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var uuid, name string
		if err := rows.Scan(&uuid, &name); err != nil {
			return err
		}
		mc.run.dropRow("devportals", uuid, DropDevportals, name)
	}
	return rows.Err()
}

func dropPublicationMappings(mc *migCtx) error {
	rows, err := mc.v1.Query(mc.v1.Rebind(`SELECT api_uuid, devportal_uuid FROM publication_mappings`))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var apiUUID, devportalUUID string
		if err := rows.Scan(&apiUUID, &devportalUUID); err != nil {
			return err
		}
		mc.run.dropRow("publication_mappings", apiUUID+"|"+devportalUUID, DropPublicationMappings,
			fmt.Sprintf("api %s → devportal %s", apiUUID, devportalUUID))
	}
	return rows.Err()
}

// strv returns a NullString's value or "" (raw actor for the core's audit resolution).
func strv(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// printReportSummary prints a compact human summary of a completed run.
func printReportSummary(r *Report) {
	fmt.Printf("\n=== migration summary (%s, run %s) ===\n", r.Mode, r.RunID)
	fmt.Printf("processed_per_table: %v\n", r.ProcessedPerTable)
	if len(r.QuarantineByCode) > 0 {
		fmt.Printf("quarantine_by_code:  %v\n", r.QuarantineByCode)
	}
	if len(r.DropsByFeature) > 0 {
		fmt.Printf("drops_by_feature:    %v\n", r.DropsByFeature)
	}
	fmt.Printf("flags_by_code:       %v\n", r.FlagsByCode)
	if len(r.DroppedConfigFields) > 0 {
		fmt.Printf("dropped_config_fields (REVIEW): %v\n", r.DroppedConfigFields)
	}
}
