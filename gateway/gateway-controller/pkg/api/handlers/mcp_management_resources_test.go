/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package handlers

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/restapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

func TestWriteCreatesThroughTheKindOps(t *testing.T) {
	f := newToolsHandler(false)
	manifest := "apiVersion: gateway.api-platform.wso2.com/v1alpha1\nkind: RestApi\n"

	_, out, err := f.h.deployAPI(toolsCtx("developer"), nil, deployInput{Kind: "RestApi", Yaml: manifest})

	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, "success", res["status"])
	assert.Equal(t, "create", res["operation"])
	assert.Equal(t, models.KindRestApi, res["kind"])
	assert.Equal(t, map[string]any{"created": models.KindRestApi}, res["resource"])
	assert.NotContains(t, res, "id", "a create has no id to echo")
	assert.Equal(t, 1, f.rest.create)
	assert.Equal(t, 0, f.rest.update)
	assert.Equal(t, manifest, f.rest.manifest, "the manifest must reach the service byte for byte")
}

func TestWriteUpdatesWhenAnIDIsGiven(t *testing.T) {
	f := newToolsHandler(false)

	_, out, err := f.h.deployAPI(toolsCtx("developer"), nil,
		deployInput{Kind: "RestApi", Yaml: "kind: RestApi\n", ID: "orders"})

	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, "update", res["operation"])
	assert.Equal(t, "orders", res["id"])
	assert.Equal(t, map[string]any{"updated": "orders"}, res["resource"])
	assert.Equal(t, 0, f.rest.create)
	assert.Equal(t, 1, f.rest.update)
	assert.Equal(t, "orders", f.rest.handle)
}

func TestApplyConfigWritesAConfigKind(t *testing.T) {
	f := newToolsHandler(false)

	_, out, err := f.h.applyConfig(toolsCtx("admin"), nil, deployInput{Kind: "secret", Yaml: "kind: Secret\n"})

	require.NoError(t, err)
	assert.Equal(t, models.KindSecret, resultMap(t, out)["kind"])
	assert.Equal(t, 1, f.secrets.create)
}

// The kind argument is normalised before lookup, so a model's spelling
// variations land on the same kind rather than failing.
func TestWriteAcceptsKindSpellingVariants(t *testing.T) {
	for _, kind := range []string{"rest-api", "REST_API", " RestApi "} {
		t.Run(kind, func(t *testing.T) {
			f := newToolsHandler(false)
			_, _, err := f.h.deployAPI(toolsCtx("developer"), nil, deployInput{Kind: kind, Yaml: "x"})
			require.NoError(t, err)
			assert.Equal(t, 1, f.rest.create)
		})
	}
}

// Every refusal happens before the kind's service is reached.
func TestWriteRefusesBeforeReachingTheService(t *testing.T) {
	tests := []struct {
		name      string
		immutable bool
		apply     bool
		roles     []string
		in        deployInput
		wantErr   string
	}{
		{"immutable mode", true, false, []string{"admin"},
			deployInput{Kind: "RestApi", Yaml: "x"}, "immutable mode"},
		{"deploy refuses a config kind", false, false, []string{"admin"},
			deployInput{Kind: "Secret", Yaml: "x"}, "use wso2_apip_gw_apply_config"},
		{"apply refuses a routable kind", false, true, []string{"admin"},
			deployInput{Kind: "RestApi", Yaml: "x"}, "use wso2_apip_gw_deploy_api"},
		{"unknown kind", false, false, []string{"admin"},
			deployInput{Kind: "Widget", Yaml: "x"}, `unknown kind "Widget"`},
		{"known kind this gateway does not register", false, false, []string{"admin"},
			deployInput{Kind: "LlmProxy", Yaml: "x"}, `kind "LlmProxy" is not supported by this gateway`},
		{"caller lacks the create role", false, false, []string{"viewer"},
			deployInput{Kind: "RestApi", Yaml: "x"}, "insufficient scope"},
		{"caller lacks the update role", false, true, []string{"developer"},
			deployInput{Kind: "Secret", Yaml: "x", ID: "db-pass"}, "insufficient scope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newToolsHandler(tt.immutable)
			tool := f.h.deployAPI
			if tt.apply {
				tool = f.h.applyConfig
			}

			_, out, err := tool(toolsCtx(tt.roles...), nil, tt.in)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, out)
			for _, c := range []*toolsCalls{f.rest, f.mcp, f.secrets} {
				assert.Zero(t, c.create+c.update, "no service may be reached on a refusal")
			}
		})
	}
}

// The service's validation detail is surfaced the way the REST handlers surface
// it: every field and message, which is how the model learns what to correct in
// the manifest. The stubs return the typed errors the real RestAPIService
// returns - create and update use different types - because a plain string
// error would pass whether or not write() unpacks the list.
func TestWriteSurfacesTheServiceValidationError(t *testing.T) {
	f := newToolsHandler(false)
	f.rest.createErr = &utils.ValidationErrorListError{Errors: []config.ValidationError{
		{Field: "spec.context", Message: "must start with '/'"},
		{Field: "spec.upstream.main.url", Message: "Upstream URL must use http or https scheme"},
	}}
	f.rest.updateErr = &restapi.ValidationError{Errors: []config.ValidationError{
		{Field: "spec.version", Message: "API version is required"},
	}}

	_, _, err := f.h.deployAPI(toolsCtx("developer"), nil, deployInput{Kind: "RestApi", Yaml: "x"})
	require.Error(t, err)
	assert.Equal(t, "failed to create RestApi: Configuration validation failed (2 errors):\n"+
		"  1. spec.context: must start with '/'\n"+
		"  2. spec.upstream.main.url: Upstream URL must use http or https scheme", err.Error())

	_, _, err = f.h.deployAPI(toolsCtx("developer"), nil, deployInput{Kind: "RestApi", Yaml: "x", ID: "orders"})
	require.Error(t, err)
	assert.Equal(t, `failed to update RestApi "orders": Configuration validation failed (1 error):`+"\n"+
		"  1. spec.version: API version is required", err.Error())
}

// An internal failure is not echoed, matching the REST handlers' 500 bodies;
// the full error goes to the log only.
func TestWriteHidesInternalErrorsLikeREST(t *testing.T) {
	f := newToolsHandler(false)
	f.rest.updateErr = fmt.Errorf("failed to persist configuration update: %w", errors.New("database is locked"))

	_, _, err := f.h.deployAPI(toolsCtx("developer"), nil, deployInput{Kind: "RestApi", Yaml: "x", ID: "orders"})

	require.Error(t, err)
	assert.Equal(t, `failed to update RestApi "orders": Failed to update configuration`, err.Error())
	assert.NotContains(t, err.Error(), "database is locked")
}

// Kinds whose REST handler passes the service error through keep doing so over
// MCP — their services put the validation detail in the error string itself.
func TestWritePassesThroughForPassThroughKinds(t *testing.T) {
	f := newToolsHandler(false)
	f.mcp.createErr = errors.New("configuration validation failed with 1 error(s): 1. spec.tools: is required")

	_, _, err := f.h.deployAPI(toolsCtx("developer"), nil, deployInput{Kind: "Mcp", Yaml: "x"})

	require.Error(t, err)
	assert.Equal(t, "failed to create Mcp: configuration validation failed with 1 error(s): "+
		"1. spec.tools: is required", err.Error())
}

func TestWriteDeniesWithoutTheGate(t *testing.T) {
	f := newToolsHandler(false)

	_, _, err := f.h.deployAPI(context.Background(), nil, deployInput{Kind: "RestApi", Yaml: "x"})

	require.Error(t, err)
	assert.Equal(t, "this operation is not available", err.Error())
	assert.Zero(t, f.rest.create)
}

func TestRemoveDeletesThroughTheKindOps(t *testing.T) {
	f := newToolsHandler(false)

	_, out, err := f.h.undeployAPI(toolsCtx("developer"), nil,
		deleteInput{Kind: "RestApi", ID: "orders", Confirm: true})

	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, "success", res["status"])
	assert.Equal(t, "orders", res["id"])
	assert.Equal(t, `RestApi "orders" deleted successfully`, res["message"])
	assert.Equal(t, 1, f.rest.delete)
	assert.Equal(t, "orders", f.rest.handle)
}

func TestDeleteConfigDeletesAConfigKind(t *testing.T) {
	f := newToolsHandler(false)

	_, _, err := f.h.deleteConfig(toolsCtx("admin"), nil, deleteInput{Kind: "Secret", ID: "db-pass", Confirm: true})

	require.NoError(t, err)
	assert.Equal(t, 1, f.secrets.delete)
}

func TestRemoveRefusesBeforeReachingTheService(t *testing.T) {
	tests := []struct {
		name      string
		immutable bool
		config    bool
		roles     []string
		in        deleteInput
		wantErr   string
	}{
		// confirm is checked first, so even an unresolvable kind is refused for
		// the missing confirmation rather than leaking which kinds exist.
		{"confirm missing", false, false, []string{"admin"},
			deleteInput{Kind: "Widget", ID: "w1"}, `refusing to delete Widget "w1"`},
		{"immutable mode", true, false, []string{"admin"},
			deleteInput{Kind: "RestApi", ID: "orders", Confirm: true}, "immutable mode"},
		{"undeploy refuses a config kind", false, false, []string{"admin"},
			deleteInput{Kind: "Secret", ID: "db-pass", Confirm: true}, "use wso2_apip_gw_apply_config or wso2_apip_gw_delete_config"},
		{"delete_config refuses a routable kind", false, true, []string{"admin"},
			deleteInput{Kind: "Mcp", ID: "m1", Confirm: true}, "use wso2_apip_gw_deploy_api or wso2_apip_gw_undeploy_api"},
		{"caller lacks the delete role", false, true, []string{"developer"},
			deleteInput{Kind: "Secret", ID: "db-pass", Confirm: true}, "insufficient scope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newToolsHandler(tt.immutable)
			tool := f.h.undeployAPI
			if tt.config {
				tool = f.h.deleteConfig
			}

			_, out, err := tool(toolsCtx(tt.roles...), nil, tt.in)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, out)
			assert.Zero(t, f.rest.delete+f.mcp.delete+f.secrets.delete)
		})
	}
}

// A delete failure is reported generically: echoing the storage error would
// confirm what does and does not exist.
func TestRemoveFailureDoesNotEchoTheStorageError(t *testing.T) {
	f := newToolsHandler(false)
	f.rest.deleteErr = errors.New("sql: no rows in result set for handle 'orders'")

	_, _, err := f.h.undeployAPI(toolsCtx("developer"), nil, deleteInput{Kind: "RestApi", ID: "orders", Confirm: true})

	require.Error(t, err)
	assert.Equal(t, `failed to delete RestApi "orders"`, err.Error())
}

func TestGetResourceReturnsTheManifest(t *testing.T) {
	f := newToolsHandler(false)

	_, out, err := f.h.getResource(toolsCtx("developer"), nil, getInput{Kind: "restapi", ID: "orders"})

	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, models.KindRestApi, res["kind"])
	assert.Equal(t, "orders", res["id"])
	assert.Equal(t, map[string]any{"handle": "orders"}, res["resource"])
	assert.Equal(t, "orders", f.rest.handle)
}

func TestGetResourceNotFoundIsGeneric(t *testing.T) {
	f := newToolsHandler(false)
	f.rest.getErr = errors.New("storage: row for uuid 8c2f… missing")

	_, _, err := f.h.getResource(toolsCtx("developer"), nil, getInput{Kind: "RestApi", ID: "orders"})

	require.Error(t, err)
	assert.Equal(t, `RestApi with handle "orders" not found`, err.Error())
}

func TestGetResourceRefusals(t *testing.T) {
	f := newToolsHandler(false)

	_, _, err := f.h.getResource(toolsCtx("developer"), nil, getInput{Kind: "Secret", ID: "db-pass"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient scope")

	_, _, err = f.h.getResource(toolsCtx("admin"), nil, getInput{Kind: "Widget", ID: "w1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown kind "Widget"`)

	assert.Zero(t, f.rest.get+f.secrets.get)
}

// A Secret read must name a choice, the way confirm guards delete; nothing is
// read until it does, and each choice reads through the matching function.
func TestGetResourceSecretNeedsAValueChoice(t *testing.T) {
	f := newToolsHandler(false)
	redacted := 0
	f.h.kinds[models.KindSecret].GetRedacted = func(handle string) (any, error) {
		redacted++
		return map[string]any{"handle": handle}, nil
	}

	_, _, err := f.h.getResource(toolsCtx("admin"), nil, getInput{Kind: "Secret", ID: "db-pass"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `secretValue="reveal"`)
	assert.Zero(t, f.secrets.get+redacted, "nothing is read before the user chooses")

	_, _, err = f.h.getResource(toolsCtx("admin"), nil, getInput{Kind: "Secret", ID: "db-pass", SecretValue: "show"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `must be "reveal" or "redact"`)

	_, _, err = f.h.getResource(toolsCtx("admin"), nil, getInput{Kind: "Secret", ID: "db-pass", SecretValue: "reveal"})
	require.NoError(t, err)
	assert.Equal(t, 1, f.secrets.get)

	_, out, err := f.h.getResource(toolsCtx("admin"), nil, getInput{Kind: "Secret", ID: "db-pass", SecretValue: "redact"})
	require.NoError(t, err)
	assert.Equal(t, 1, redacted)
	assert.Contains(t, resultMap(t, out), "notice")
}

func TestListResourcesForOneKindReturnsFullManifests(t *testing.T) {
	f := newToolsHandler(false)
	f.rest.rows = []listedResource{
		{ID: "orders", Resource: map[string]any{"metadata": "orders"}},
		{ID: "billing", Resource: map[string]any{"metadata": "billing"}},
	}

	_, out, err := f.h.listResources(toolsCtx("developer"), nil, listInput{Kind: "RestApi"})

	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, models.KindRestApi, res["kind"])
	assert.Equal(t, 2, res["count"])
	assert.Equal(t, []any{map[string]any{"metadata": "orders"}, map[string]any{"metadata": "billing"}}, res["items"],
		"the one-kind form returns each row's full Resource, not the summary")
}

func TestListResourcesForOneKindFailures(t *testing.T) {
	f := newToolsHandler(false)
	f.rest.listErr = errors.New("database is locked")

	_, _, err := f.h.listResources(toolsCtx("developer"), nil, listInput{Kind: "RestApi"})
	require.Error(t, err)
	assert.Equal(t, "failed to list resources of kind RestApi", err.Error())

	_, _, err = f.h.listResources(toolsCtx("developer"), nil, listInput{Kind: "Secret"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient scope")

	_, _, err = f.h.listResources(toolsCtx("developer"), nil, listInput{Kind: "Widget"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown kind "Widget"`)
}

// The cross-kind inventory lists only the kinds the caller may read, and omits
// the rest rather than failing the whole call.
func TestListResourcesInventoryOmitsUnreadableKinds(t *testing.T) {
	f := newToolsHandler(false)
	f.rest.rows = []listedResource{{ID: "orders"}, {ID: "billing"}}
	f.mcp.rows = []listedResource{{ID: "tools"}}
	f.secrets.rows = []listedResource{{ID: "db-pass"}}

	_, out, err := f.h.listResources(toolsCtx("developer"), nil, listInput{})

	require.NoError(t, err)
	res := resultMap(t, out)
	kinds := toolsAs[map[string]any](t, res["kinds"])
	assert.Contains(t, kinds, models.KindRestApi)
	assert.Contains(t, kinds, models.KindMcp)
	assert.NotContains(t, kinds, models.KindSecret, "developer cannot read Secret, so it is omitted")
	assert.Equal(t, 3, res["total"])
	assert.Equal(t, 2, toolsAs[map[string]any](t, kinds[models.KindRestApi])["count"])
	assert.Equal(t, []listedResource{{ID: "tools"}}, toolsAs[map[string]any](t, kinds[models.KindMcp])["resources"])
	assert.Contains(t, res["hint"], "wso2_apip_gw_manage_certificates")
	assert.Zero(t, f.secrets.list, "an unreadable kind is never listed")

	_, out, err = f.h.listResources(toolsCtx("admin"), nil, listInput{})
	require.NoError(t, err)
	res = resultMap(t, out)
	assert.Contains(t, toolsAs[map[string]any](t, res["kinds"]), models.KindSecret)
	assert.Equal(t, 4, res["total"])
}

func TestListResourcesInventoryReportsAFailingKindInPlace(t *testing.T) {
	f := newToolsHandler(false)
	f.rest.rows = []listedResource{{ID: "orders"}}
	f.mcp.listErr = errors.New("disk I/O error")

	_, out, err := f.h.listResources(toolsCtx("developer"), nil, listInput{})

	require.NoError(t, err, "one failing kind must not fail the inventory")
	res := resultMap(t, out)
	kinds := toolsAs[map[string]any](t, res["kinds"])
	assert.Equal(t, map[string]any{"error": "failed to list this kind"}, kinds[models.KindMcp])
	assert.Equal(t, 1, res["total"])
}

func TestListResourcesInventoryWithNoReadableKind(t *testing.T) {
	f := newToolsHandler(false)

	_, _, err := f.h.listResources(toolsCtx("viewer"), nil, listInput{})

	require.Error(t, err)
	assert.Equal(t, "your credentials do not permit reading any resource kind on this Gateway", err.Error())
}
