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

package repository

import (
	"slices"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// seedGateway creates a gateway row through the real GatewayRepo, then stamps
// its reported semver directly (GatewayRepo.Create always registers a gateway
// with no reported version; the version is filled in later when the gateway
// controller reports its manifest — see service.GatewayService.ReceiveGatewayManifest).
func seedGateway(t *testing.T, tdb *schemaDB, orgID, version string) string {
	t.Helper()
	gwRepo := NewGatewayRepo(tdb.db)
	gw := &model.Gateway{
		ID:             id(),
		OrganizationID: orgID,
		Name:           "gw-" + id()[:8],
		Handle:         "gw-" + id()[:8],
	}
	if err := gwRepo.Create(gw); err != nil {
		t.Fatalf("create gateway failed: %v", err)
	}
	tdb.exec(t, `UPDATE gateways SET version = ? WHERE uuid = ? AND organization_uuid = ?`, version, gw.ID, orgID)
	return gw.ID
}

// TestIT_RestAPI_DataVersionStamped_AndTranslate drives the real APIRepo
// through Create, confirms data_version is stamped "1.0" (REST never diverges
// from the flat-policies shape), then verifies Translate against both gateway
// 1.2.0 (v1, passthrough) and 1.1.0 (v1alpha1, apiVersion swap only).
func TestIT_RestAPI_DataVersionStamped_AndTranslate(t *testing.T) {
	tdb := openSchemaDB(t)
	orgID, projID := seedOrgProject(t, tdb, "gwt-rest")
	apiRepo := NewAPIRepo(tdb.db)

	restAPI := &model.API{
		Handle:          "rest-" + id()[:8],
		Name:            "IT REST API",
		Kind:            constants.RestApi,
		Version:         "v1.0",
		ProjectID:       projID,
		OrganizationID:  orgID,
		LifeCycleStatus: "CREATED",
		Configuration:   model.RestAPIConfig{Name: "IT REST API", Version: "v1.0"},
	}
	if err := apiRepo.CreateAPI(restAPI); err != nil {
		t.Fatalf("CreateAPI failed: %v", err)
	}

	stored, err := apiRepo.GetAPIByUUID(restAPI.ID, orgID)
	if err != nil {
		t.Fatalf("GetAPIByUUID failed: %v", err)
	}
	if stored.DataVersion != "1.0" {
		t.Fatalf("want data_version 1.0 for a fresh REST API, got %q", stored.DataVersion)
	}

	sourceDataVersion := gatewaytranslator.PlatformDataVersion(stored.DataVersion)

	newGw120 := seedGateway(t, tdb, orgID, "1.2.0")
	artifactForNew := &dto.APIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.RestApi}
	if _, err := gatewaytranslator.Translate(constants.RestApi, sourceDataVersion, newGw120AsVersion(t, tdb, newGw120), artifactForNew); err != nil {
		t.Fatalf("Translate to 1.2.0 gateway failed: %v", err)
	}
	if artifactForNew.ApiVersion != constants.GatewayApiVersion {
		t.Fatalf("gateway 1.2.0: want apiVersion %q, got %q", constants.GatewayApiVersion, artifactForNew.ApiVersion)
	}

	oldGw110 := seedGateway(t, tdb, orgID, "1.1.0")
	artifactForOld := &dto.APIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.RestApi}
	if _, err := gatewaytranslator.Translate(constants.RestApi, sourceDataVersion, newGw120AsVersion(t, tdb, oldGw110), artifactForOld); err != nil {
		t.Fatalf("Translate to 1.1.0 gateway failed: %v", err)
	}
	if artifactForOld.ApiVersion != constants.GatewayApiVersionV1Alpha1 {
		t.Fatalf("gateway 1.1.0: want apiVersion %q, got %q", constants.GatewayApiVersionV1Alpha1, artifactForOld.ApiVersion)
	}
}

// newGw120AsVersion reads back the gateway's stored semver so the test exercises
// the same ParseVersion(gateway.Version) path the deploy services use. Fails the
// test immediately on a lookup error, rather than silently falling through to an
// empty string — an empty version resolves to "latest" (see
// GatewayDataVersionForGateway), which would make a lookup bug masquerade as a
// passing "new gateway" assertion instead of a test failure.
func newGw120AsVersion(t *testing.T, tdb *schemaDB, gatewayUUID string) string {
	t.Helper()
	var v string
	q := tdb.db.Rebind(`SELECT version FROM gateways WHERE uuid = ?`)
	if err := tdb.db.QueryRow(q, gatewayUUID).Scan(&v); err != nil {
		t.Fatalf("failed to look up gateway version for %s: %v", gatewayUUID, err)
	}
	return v
}

// TestIT_MCPProxy_DataVersionStamped_AndTranslate mirrors the REST case for MCP.
func TestIT_MCPProxy_DataVersionStamped_AndTranslate(t *testing.T) {
	tdb := openSchemaDB(t)
	orgID, projID := seedOrgProject(t, tdb, "gwt-mcp")
	mcpRepo := NewMCPProxyRepo(tdb.db)

	proxy := &model.MCPProxy{
		Handle:           "mcp-" + id()[:8],
		Name:             "IT MCP Proxy",
		OrganizationUUID: orgID,
		ProjectUUID:      &projID,
		Version:          "v1.0",
		Configuration:    model.MCPProxyConfiguration{Name: "IT MCP Proxy", Version: "v1.0"},
	}
	if err := mcpRepo.Create(proxy); err != nil {
		t.Fatalf("Create MCP proxy failed: %v", err)
	}

	stored, err := mcpRepo.GetByUUID(proxy.UUID, orgID)
	if err != nil {
		t.Fatalf("GetByUUID failed: %v", err)
	}
	// 1.1 since MCPProxyConfiguration gained specVersions; the gateway CRD apiVersion stays v1.
	if stored.DataVersion != "1.1" {
		t.Fatalf("want data_version 1.1 for a fresh MCP proxy, got %q", stored.DataVersion)
	}

	gwOld := seedGateway(t, tdb, orgID, "1.1.0")
	artifact := &model.MCPProxyDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.MCPProxy}
	if _, err := gatewaytranslator.Translate(constants.MCPProxy, gatewaytranslator.PlatformDataVersion(stored.DataVersion), newGw120AsVersion(t, tdb, gwOld), artifact); err != nil {
		t.Fatalf("Translate to 1.1.0 gateway failed: %v", err)
	}
	if artifact.ApiVersion != constants.GatewayApiVersionV1Alpha1 {
		t.Fatalf("gateway 1.1.0: want apiVersion %q, got %q", constants.GatewayApiVersionV1Alpha1, artifact.ApiVersion)
	}
}

// seedLLMProviderTemplate inserts the minimal template row llm_providers.template_uuid
// requires (FK). Raw SQL, matching the seeding style already used by cascade_test.go
// for graphs that only need a parent row to exist to satisfy a foreign key.
func seedLLMProviderTemplate(t *testing.T, tdb *schemaDB, orgID string) string {
	t.Helper()
	templateID := id()
	tdb.exec(t, `INSERT INTO llm_provider_templates (uuid, organization_uuid, handle, group_id, display_name, configuration) VALUES (?, ?, ?, ?, ?, ?)`,
		templateID, orgID, "tpl-"+templateID[:8], "grp-"+templateID[:8], "IT Template", []byte("{}"))
	return templateID
}

// TestIT_LLMProvider_CurrentDataVersion_SplitPoliciesPreservedOnNewGateway_FlattenedOnOld
// verifies the "any db entry now present is 1.1" behavior: a freshly created LLM
// provider stores data_version "1.1" and its split globalPolicies/operationPolicies
// pass through unchanged to a 1.2.0 gateway (v1) but are flattened to the legacy
// policies list for a 1.1.0 gateway (v1alpha1).
func TestIT_LLMProvider_CurrentDataVersion_SplitPoliciesPreservedOnNewGateway_FlattenedOnOld(t *testing.T) {
	tdb := openSchemaDB(t)
	orgID, _ := seedOrgProject(t, tdb, "gwt-llm")
	templateID := seedLLMProviderTemplate(t, tdb, orgID)
	providerRepo := NewLLMProviderRepo(tdb.db)

	provider := &model.LLMProvider{
		OrganizationUUID: orgID,
		ID:               "llm-" + id()[:8],
		Name:             "IT LLM Provider",
		Version:          "v1.0",
		TemplateUUID:     templateID,
		Configuration: model.LLMProviderConfig{
			GlobalPolicies: []model.GlobalPolicy{{Name: "llm-cost-based-ratelimit", Version: "v1"}},
		},
	}
	if err := providerRepo.Create(provider); err != nil {
		t.Fatalf("Create LLM provider failed: %v", err)
	}

	stored, err := providerRepo.GetByID(provider.ID, orgID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if stored.DataVersion != "1.1" {
		t.Fatalf("want data_version 1.1 for a fresh LLM provider, got %q", stored.DataVersion)
	}

	sourceDataVersion := gatewaytranslator.PlatformDataVersion(stored.DataVersion)

	gwNew := seedGateway(t, tdb, orgID, "1.2.0")
	artifactNew := &dto.LLMProviderDeploymentYAML{ApiVersion: constants.GatewayApiVersion}
	artifactNew.Spec.GlobalPolicies = []api.Policy{{Name: "llm-cost-based-ratelimit", Version: "v1"}}
	if _, err := gatewaytranslator.Translate(constants.LLMProvider, sourceDataVersion, newGw120AsVersion(t, tdb, gwNew), artifactNew); err != nil {
		t.Fatalf("Translate to 1.2.0 gateway failed: %v", err)
	}
	if artifactNew.ApiVersion != constants.GatewayApiVersion {
		t.Fatalf("gateway 1.2.0: want apiVersion %q, got %q", constants.GatewayApiVersion, artifactNew.ApiVersion)
	}
	if len(artifactNew.Spec.GlobalPolicies) != 1 || len(artifactNew.Spec.Policies) != 0 {
		t.Fatalf("gateway 1.2.0: want split policies preserved, got globalPolicies=%d legacyPolicies=%d",
			len(artifactNew.Spec.GlobalPolicies), len(artifactNew.Spec.Policies))
	}

	gwOld := seedGateway(t, tdb, orgID, "1.1.0")
	artifactOld := &dto.LLMProviderDeploymentYAML{ApiVersion: constants.GatewayApiVersion}
	artifactOld.Spec.GlobalPolicies = []api.Policy{{Name: "llm-cost-based-ratelimit", Version: "v1"}}
	if _, err := gatewaytranslator.Translate(constants.LLMProvider, sourceDataVersion, newGw120AsVersion(t, tdb, gwOld), artifactOld); err != nil {
		t.Fatalf("Translate to 1.1.0 gateway failed: %v", err)
	}
	if artifactOld.ApiVersion != constants.GatewayApiVersionV1Alpha1 {
		t.Fatalf("gateway 1.1.0: want apiVersion %q, got %q", constants.GatewayApiVersionV1Alpha1, artifactOld.ApiVersion)
	}
	if len(artifactOld.Spec.GlobalPolicies) != 0 || len(artifactOld.Spec.Policies) != 1 {
		t.Fatalf("gateway 1.1.0: want flattened to legacy policies, got globalPolicies=%d legacyPolicies=%d",
			len(artifactOld.Spec.GlobalPolicies), len(artifactOld.Spec.Policies))
	}
	if artifactOld.Spec.Policies[0].Name != "llm-cost-based-ratelimit" {
		t.Fatalf("gateway 1.1.0: want flattened policy name llm-cost-based-ratelimit, got %q", artifactOld.Spec.Policies[0].Name)
	}
}

// TestIT_LLMProvider_LegacyDataVersion_FlatPoliciesNormalizedOnNewGateway verifies
// the "legacy data which has not been updated by the platform-api remains 1.0"
// case: an LLM provider whose data_version is explicitly stamped "1.0" (as any
// pre-existing row would be) with legacy flat policies still deploys correctly to
// a 1.2.0 gateway (normalized up to the split shape by gatewaytranslator's
// normalizer) and to a 1.1.0 gateway (stays flat) — same stored legacy provider
// data used for both target gateways.
func TestIT_LLMProvider_LegacyDataVersion_FlatPoliciesNormalizedOnNewGateway(t *testing.T) {
	tdb := openSchemaDB(t)
	orgID, _ := seedOrgProject(t, tdb, "gwt-llm-legacy")
	templateID := seedLLMProviderTemplate(t, tdb, orgID)
	providerRepo := NewLLMProviderRepo(tdb.db)

	provider := &model.LLMProvider{
		OrganizationUUID: orgID,
		ID:               "llm-legacy-" + id()[:8],
		Name:             "IT Legacy LLM Provider",
		Version:          "v1.0",
		TemplateUUID:     templateID,
		DataVersion:      "1.0", // explicitly pre-set, simulating a legacy stored row
		Configuration: model.LLMProviderConfig{
			Policies: []model.LLMPolicy{{
				Name:  "llm-cost-based-ratelimit",
				Paths: []model.LLMPolicyPath{{Path: "/*", Methods: []string{"*"}}},
			}},
		},
	}
	if err := providerRepo.Create(provider); err != nil {
		t.Fatalf("Create legacy LLM provider failed: %v", err)
	}

	stored, err := providerRepo.GetByID(provider.ID, orgID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if stored.DataVersion != "1.0" {
		t.Fatalf("want the pre-set data_version 1.0 to be preserved (not overwritten), got %q", stored.DataVersion)
	}

	gwNew := seedGateway(t, tdb, orgID, "1.2.0")
	artifactNew := &dto.LLMProviderDeploymentYAML{ApiVersion: constants.GatewayApiVersion}
	artifactNew.Spec.Policies = []api.LLMPolicy{{
		Name:  "llm-cost-based-ratelimit",
		Paths: []api.LLMPolicyPath{{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{}}},
	}}
	if _, err := gatewaytranslator.Translate(constants.LLMProvider, gatewaytranslator.PlatformDataVersion(stored.DataVersion), newGw120AsVersion(t, tdb, gwNew), artifactNew); err != nil {
		t.Fatalf("Translate legacy source to 1.2.0 gateway failed: %v", err)
	}
	if artifactNew.ApiVersion != constants.GatewayApiVersion {
		t.Fatalf("gateway 1.2.0: want apiVersion %q, got %q", constants.GatewayApiVersion, artifactNew.ApiVersion)
	}
	if len(artifactNew.Spec.GlobalPolicies) != 1 || len(artifactNew.Spec.Policies) != 0 {
		t.Fatalf("gateway 1.2.0: want legacy source normalized up to split shape, got globalPolicies=%d legacyPolicies=%d",
			len(artifactNew.Spec.GlobalPolicies), len(artifactNew.Spec.Policies))
	}

	// Same stored legacy provider data (data_version "1.0", flat policies),
	// deployed to a 1.1.0 gateway instead: it must stay flat, not be normalized
	// up then immediately re-flattened into a different-shaped list.
	gwOld := seedGateway(t, tdb, orgID, "1.1.0")
	artifactOld := &dto.LLMProviderDeploymentYAML{ApiVersion: constants.GatewayApiVersion}
	artifactOld.Spec.Policies = []api.LLMPolicy{{
		Name:  "llm-cost-based-ratelimit",
		Paths: []api.LLMPolicyPath{{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{}}},
	}}
	if _, err := gatewaytranslator.Translate(constants.LLMProvider, gatewaytranslator.PlatformDataVersion(stored.DataVersion), newGw120AsVersion(t, tdb, gwOld), artifactOld); err != nil {
		t.Fatalf("Translate legacy source to 1.1.0 gateway failed: %v", err)
	}
	if artifactOld.ApiVersion != constants.GatewayApiVersionV1Alpha1 {
		t.Fatalf("gateway 1.1.0: want apiVersion %q, got %q", constants.GatewayApiVersionV1Alpha1, artifactOld.ApiVersion)
	}
	if len(artifactOld.Spec.GlobalPolicies) != 0 || len(artifactOld.Spec.Policies) != 1 {
		t.Fatalf("gateway 1.1.0: want legacy source to stay flat, got globalPolicies=%d legacyPolicies=%d",
			len(artifactOld.Spec.GlobalPolicies), len(artifactOld.Spec.Policies))
	}
}

// TestIT_MCPProxy_LegacyScalarRowStillReadsAsAList covers a row written before specVersions
// existed: its configuration blob carries only the deprecated scalar. Nothing rewrites such a
// row, so every read has to fold it, and a deploy has to emit the list form.
func TestIT_MCPProxy_LegacyScalarRowStillReadsAsAList(t *testing.T) {
	tdb := openSchemaDB(t)
	orgID, projID := seedOrgProject(t, tdb, "gwt-mcp-legacy")
	mcpRepo := NewMCPProxyRepo(tdb.db)

	proxy := &model.MCPProxy{
		Handle:           "mcp-" + id()[:8],
		Name:             "IT Legacy MCP Proxy",
		OrganizationUUID: orgID,
		ProjectUUID:      &projID,
		Version:          "v1.0",
		Configuration: model.MCPProxyConfiguration{
			Name:        "IT Legacy MCP Proxy",
			Version:     "v1.0",
			SpecVersion: "2025-11-25",
		},
	}
	if err := mcpRepo.Create(proxy); err != nil {
		t.Fatalf("Create MCP proxy failed: %v", err)
	}

	stored, err := mcpRepo.GetByUUID(proxy.UUID, orgID)
	if err != nil {
		t.Fatalf("GetByUUID failed: %v", err)
	}
	if got := stored.Configuration.EffectiveSpecVersions(); !slices.Equal(got, []string{"2025-11-25"}) {
		t.Errorf("a legacy scalar must read back as the canonical list, got %v", got)
	}
	if stored.Configuration.SpecVersions != nil {
		t.Errorf("reading must not rewrite the stored row, got %v",
			stored.Configuration.SpecVersions)
	}

	yamlString, err := (&utils.MCPUtils{}).GenerateMCPDeploymentYAML(stored)
	if err != nil {
		t.Fatalf("GenerateMCPDeploymentYAML failed: %v", err)
	}
	if !strings.Contains(yamlString, "specVersions:") || !strings.Contains(yamlString, "2025-11-25") {
		t.Errorf("a legacy row must deploy as the list form:\n%s", yamlString)
	}
}
