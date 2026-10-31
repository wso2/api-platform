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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/authenticators"
	commonmodels "github.com/wso2/api-platform/common/models"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/certificate"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/restapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/subscription"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/testutil/pki"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

var toolsDiscard = slog.New(slog.NewTextHandler(io.Discard, nil))

// toolsCalls records every call a stub kind receives and carries the canned
// results and errors a test wants the stub to return.
type toolsCalls struct {
	create, update, delete, get, list int
	handle, manifest                  string
	rows                              []listedResource

	createErr, updateErr, deleteErr, getErr, listErr error

	keyCalls  map[string]int
	keyParent string
	keyName   string
	caller    *commonmodels.AuthContext
	issueReq  *api.APIKeyCreationRequest
	updateReq *api.APIKeyCreationRequest
	rotateReq *api.APIKeyRegenerationRequest
	keyErr    error
}

func (c *toolsCalls) recordKey(op, parent, keyName string, caller *commonmodels.AuthContext) error {
	c.keyCalls[op]++
	c.keyParent, c.keyName, c.caller = parent, keyName, caller
	return c.keyErr
}

func toolsStubKind(kind, collection string, routable, withKeys bool, c *toolsCalls) *kindOps {
	c.keyCalls = map[string]int{}
	ops := &kindOps{
		Kind:       kind,
		Routable:   routable,
		Collection: collection,
		Create: func(manifest []byte, _ string, _ *slog.Logger) (any, error) {
			c.create++
			c.manifest = string(manifest)
			if c.createErr != nil {
				return nil, c.createErr
			}
			return map[string]any{"created": kind}, nil
		},
		Update: func(handle string, manifest []byte, _ string, _ *slog.Logger) (any, error) {
			c.update++
			c.handle, c.manifest = handle, string(manifest)
			if c.updateErr != nil {
				return nil, c.updateErr
			}
			return map[string]any{"updated": handle}, nil
		},
		Delete: func(handle, _ string, _ *slog.Logger) error {
			c.delete++
			c.handle = handle
			return c.deleteErr
		},
		Get: func(handle string) (any, error) {
			c.get++
			c.handle = handle
			if c.getErr != nil {
				return nil, c.getErr
			}
			return map[string]any{"handle": handle}, nil
		},
		List: func() ([]listedResource, error) {
			c.list++
			if c.listErr != nil {
				return nil, c.listErr
			}
			return c.rows, nil
		},
	}
	if !withKeys {
		return ops
	}
	ops.Keys = &keyOps{
		Issue: func(parent string, req api.APIKeyCreationRequest, caller *commonmodels.AuthContext, _ string, _ *slog.Logger) (any, error) {
			c.issueReq = &req
			if err := c.recordKey("issue", parent, "", caller); err != nil {
				return nil, err
			}
			return map[string]any{"issued": true}, nil
		},
		List: func(parent string, caller *commonmodels.AuthContext, _ string, _ *slog.Logger) (any, error) {
			if err := c.recordKey("list", parent, "", caller); err != nil {
				return nil, err
			}
			return map[string]any{"listed": true}, nil
		},
		Rotate: func(parent, keyName string, req api.APIKeyRegenerationRequest, caller *commonmodels.AuthContext, _ string, _ *slog.Logger) (any, error) {
			c.rotateReq = &req
			if err := c.recordKey("rotate", parent, keyName, caller); err != nil {
				return nil, err
			}
			return map[string]any{"rotated": keyName}, nil
		},
		Update: func(parent, keyName string, req api.APIKeyCreationRequest, caller *commonmodels.AuthContext, _ string, _ *slog.Logger) (any, error) {
			c.updateReq = &req
			if err := c.recordKey("update", parent, keyName, caller); err != nil {
				return nil, err
			}
			return map[string]any{"updated": keyName}, nil
		},
		Revoke: func(parent, keyName string, caller *commonmodels.AuthContext, _ string, _ *slog.Logger) (any, error) {
			if err := c.recordKey("revoke", parent, keyName, caller); err != nil {
				return nil, err
			}
			return map[string]any{"revoked": keyName}, nil
		},
	}
	return ops
}

// toolsRoleMap grants developer the RestApi and Mcp routes, and admin the Secret
// and key-injection routes.
func toolsRoleMap() map[string][]string {
	both := []string{"developer", "admin"}
	admin := []string{"admin"}
	return map[string][]string{
		"POST /rest-apis":        both,
		"GET /rest-apis":         both,
		"GET /rest-apis/{id}":    both,
		"PUT /rest-apis/{id}":    both,
		"DELETE /rest-apis/{id}": both,

		"POST /mcp-proxies":        both,
		"GET /mcp-proxies":         both,
		"GET /mcp-proxies/{id}":    both,
		"PUT /mcp-proxies/{id}":    both,
		"DELETE /mcp-proxies/{id}": both,

		"POST /secrets":        admin,
		"GET /secrets":         admin,
		"GET /secrets/{id}":    admin,
		"PUT /secrets/{id}":    admin,
		"DELETE /secrets/{id}": admin,

		"POST /rest-apis/{id}/api-keys":                         both,
		"GET /rest-apis/{id}/api-keys":                          both,
		"POST /rest-apis/{id}/api-keys/{apiKeyName}/regenerate": both,
		"PUT /rest-apis/{id}/api-keys/{apiKeyName}":             admin,
		"DELETE /rest-apis/{id}/api-keys/{apiKeyName}":          both,
	}
}

type toolsFixture struct {
	h                  *McpHandler
	rest, mcp, secrets *toolsCalls
}

// newToolsHandler builds an McpHandler over three stub kinds: RestApi
// (routable, key-bearing), Mcp (routable, no keys) and Secret (config).
func newToolsHandler(immutable bool) *toolsFixture {
	f := &toolsFixture{rest: &toolsCalls{}, mcp: &toolsCalls{}, secrets: &toolsCalls{}}
	f.h = &McpHandler{
		immutable: immutable,
		logger:    toolsDiscard,
		authz:     newMcpAuthz(mcpAuthzParams{ResourceRoles: toolsRoleMap(), Logger: toolsDiscard}),
		kinds: map[string]*kindOps{
			models.KindRestApi: toolsStubKind(models.KindRestApi, "/rest-apis", true, true, f.rest),
			models.KindMcp:     toolsStubKind(models.KindMcp, "/mcp-proxies", true, false, f.mcp),
			models.KindSecret:  toolsStubKind(models.KindSecret, "/secrets", false, false, f.secrets),
		},
	}
	return f
}

// toolsCtx is a request that cleared the HTTP gate as user "alice" holding roles.
func toolsCtx(roles ...string) context.Context {
	return withMcpCaller(context.Background(), mcpCaller{
		Auth: commonmodels.AuthContext{UserID: "alice", Roles: roles},
	})
}

func resultMap(t *testing.T, out any) map[string]any {
	t.Helper()
	m, ok := out.(map[string]any)
	require.Truef(t, ok, "tool output is %T, not map[string]any", out)
	return m
}

// toolsAs asserts v's dynamic type and fails the test with a readable message,
// rather than panicking, when a tool returns a different shape.
func toolsAs[T any](t *testing.T, v any) T {
	t.Helper()
	got, ok := v.(T)
	require.Truef(t, ok, "got %T, want %T", v, *new(T))
	return got
}

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

func TestParseFutureTimestamp(t *testing.T) {
	future := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	got, err := parseFutureTimestamp("expiresAt", future)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, future, got.Format(time.RFC3339))

	got, err = parseFutureTimestamp("expiresAt", "  ")
	require.NoError(t, err)
	assert.Nil(t, got, "a blank value means no expiry, not an error")

	_, err = parseFutureTimestamp("expiresAt", "2020-01-01T00:00:00Z")
	require.Error(t, err)
	assert.Equal(t, `expiresAt "2020-01-01T00:00:00Z" is in the past; supply a timestamp later than the current time`, err.Error())

	_, err = parseFutureTimestamp("expiresAt", "next tuesday")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `expiresAt "next tuesday" is not a valid RFC 3339 timestamp`)
}

func TestParseExpiresIn(t *testing.T) {
	got, err := parseExpiresIn(nil)
	require.NoError(t, err)
	assert.Nil(t, got)

	got, err = parseExpiresIn(&expiresInInput{Duration: 90, Unit: " DAYS "})
	require.NoError(t, err)
	assert.Equal(t, &expiresInInput{Duration: 90, Unit: "days"}, got, "the unit is normalised to the service's spelling")

	got, err = parseExpiresIn(&expiresInInput{Duration: 0, Unit: "seconds"})
	require.NoError(t, err)
	assert.Equal(t, 0, got.Duration, "zero is allowed: the key expires immediately")

	_, err = parseExpiresIn(&expiresInInput{Duration: 1, Unit: "fortnights"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `expiresIn.unit "fortnights" is not supported`)

	_, err = parseExpiresIn(&expiresInInput{Duration: -1, Unit: "days"})
	require.Error(t, err)
	assert.Equal(t, "expiresIn.duration must not be negative", err.Error())
}

func TestExpiresInConversionsCarryBothFields(t *testing.T) {
	assert.Nil(t, creationExpiresIn(nil))
	assert.Nil(t, regenerationExpiresIn(nil))

	in := &expiresInInput{Duration: 7, Unit: "weeks"}
	c := creationExpiresIn(in)
	require.NotNil(t, c)
	assert.Equal(t, 7, c.Duration)
	assert.Equal(t, api.APIKeyCreationRequestExpiresInUnit("weeks"), c.Unit)

	r := regenerationExpiresIn(in)
	require.NotNil(t, r)
	assert.Equal(t, 7, r.Duration)
	assert.Equal(t, api.APIKeyRegenerationRequestExpiresInUnit("weeks"), r.Unit)
}

func TestIssueAPIKeyGenerateMode(t *testing.T) {
	f := newToolsHandler(false)

	_, out, err := f.h.issueAPIKey(toolsCtx("developer"), nil, issueKeyInput{
		Kind: "RestApi", ID: "orders", KeyName: "prod-key",
		ExpiresIn: &expiresInInput{Duration: 90, Unit: "Days"},
	})

	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, "generate", res["operation"])
	assert.Equal(t, "orders", res["id"])
	assert.Contains(t, res["notice"], "shown once", "a generated key must carry the one-time notice")

	require.Equal(t, 1, f.rest.keyCalls["issue"])
	assert.Equal(t, "orders", f.rest.keyParent)
	require.NotNil(t, f.rest.caller)
	assert.Equal(t, "alice", f.rest.caller.UserID, "the key is recorded against the verified caller")

	req := f.rest.issueReq
	require.NotNil(t, req.Name)
	assert.Equal(t, "prod-key", *req.Name)
	assert.Nil(t, req.ApiKey, "generate mode sends no key value")
	require.NotNil(t, req.ExpiresIn)
	assert.Equal(t, api.APIKeyCreationRequestExpiresInUnit("days"), req.ExpiresIn.Unit)
	assert.Nil(t, req.ExternalRefId)
	assert.Nil(t, req.Issuer)
}

func TestIssueAPIKeyRegisterMode(t *testing.T) {
	f := newToolsHandler(false)
	value := strings.Repeat("k", 40)
	expiresAt := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)

	_, out, err := f.h.issueAPIKey(toolsCtx("developer"), nil, issueKeyInput{
		Kind: "RestApi", ID: "orders",
		ApiKey:        "  " + value + "  ",
		ExpiresAt:     expiresAt.Format(time.RFC3339),
		ExternalRefId: " ext-42 ",
		Issuer:        "   ",
	})

	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, "register", res["operation"])
	assert.NotContains(t, res, "notice", "a caller-supplied key is never echoed, so there is nothing to warn about")

	req := f.rest.issueReq
	require.NotNil(t, req.ApiKey)
	assert.Equal(t, value, *req.ApiKey, "the supplied key is trimmed before it is sent")
	require.NotNil(t, req.ExpiresAt)
	assert.True(t, expiresAt.Equal(*req.ExpiresAt))
	require.NotNil(t, req.ExternalRefId)
	assert.Equal(t, "ext-42", *req.ExternalRefId)
	assert.Nil(t, req.Issuer, "a blank issuer is omitted, not sent as an empty restriction")
}

func TestIssueAPIKeyRefusesBeforeReachingTheService(t *testing.T) {
	tests := []struct {
		name      string
		immutable bool
		ctx       context.Context
		in        issueKeyInput
		wantErr   string
	}{
		{"immutable mode", true, toolsCtx("admin"),
			issueKeyInput{Kind: "RestApi", ID: "orders"}, "immutable mode"},
		{"invalid key name", false, toolsCtx("admin"),
			issueKeyInput{Kind: "RestApi", ID: "orders", KeyName: "Prod Key"}, "API key name must be lowercase"},
		{"expiry in the past", false, toolsCtx("admin"),
			issueKeyInput{Kind: "RestApi", ID: "orders", ExpiresAt: "2020-01-01T00:00:00Z"}, "is in the past"},
		{"unparseable expiry", false, toolsCtx("admin"),
			issueKeyInput{Kind: "RestApi", ID: "orders", ExpiresAt: "tomorrow"}, "not a valid RFC 3339 timestamp"},
		{"unknown expiresIn unit", false, toolsCtx("admin"),
			issueKeyInput{Kind: "RestApi", ID: "orders", ExpiresIn: &expiresInInput{Duration: 1, Unit: "years"}}, "is not supported"},
		{"negative expiresIn", false, toolsCtx("admin"),
			issueKeyInput{Kind: "RestApi", ID: "orders", ExpiresIn: &expiresInInput{Duration: -5, Unit: "days"}}, "must not be negative"},
		{"kind without API keys", false, toolsCtx("admin"),
			issueKeyInput{Kind: "Mcp", ID: "tools"}, `kind "Mcp" does not have API keys; the API key tools accept: RestApi`},
		{"unknown kind", false, toolsCtx("admin"),
			issueKeyInput{Kind: "Widget", ID: "w1"}, `unknown kind "Widget"`},
		{"caller lacks the role", false, toolsCtx("viewer"),
			issueKeyInput{Kind: "RestApi", ID: "orders"}, "insufficient scope"},
		// Role checks can be skipped, identity cannot: an empty UserID would act
		// as "no creator filter" in the key service.
		{"no caller identity", false,
			withMcpCaller(context.Background(), mcpCaller{Skipped: true}),
			issueKeyInput{Kind: "RestApi", ID: "orders"}, "requires an authenticated user identity"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newToolsHandler(tt.immutable)

			_, out, err := f.h.issueAPIKey(tt.ctx, nil, tt.in)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, out)
			assert.Zero(t, f.rest.keyCalls["issue"])
		})
	}
}

func TestIssueAPIKeyMapsServiceErrors(t *testing.T) {
	f := newToolsHandler(false)
	f.rest.keyErr = fmt.Errorf("%w: API configuration handle 'orders' not found", storage.ErrNotFound)

	_, _, err := f.h.issueAPIKey(toolsCtx("developer"), nil, issueKeyInput{Kind: "RestApi", ID: "orders"})

	require.Error(t, err)
	assert.Equal(t, `cannot issue an API key for RestApi "orders": no such resource, or no such key on it`, err.Error())
}

func TestListAPIKeys(t *testing.T) {
	f := newToolsHandler(false)

	_, out, err := f.h.listAPIKeys(toolsCtx("developer"), nil, listKeysInput{Kind: "RestApi", ID: "orders"})

	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, map[string]any{"listed": true}, res["result"])
	assert.Equal(t, "orders", res["id"])
	assert.Equal(t, 1, f.rest.keyCalls["list"])
	assert.Equal(t, "alice", f.rest.caller.UserID)
}

func TestListAPIKeysFailures(t *testing.T) {
	f := newToolsHandler(false)

	_, _, err := f.h.listAPIKeys(toolsCtx("viewer"), nil, listKeysInput{Kind: "RestApi", ID: "orders"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient scope")
	assert.Zero(t, f.rest.keyCalls["list"])

	f.rest.keyErr = errors.New("connection reset by peer")
	_, _, err = f.h.listAPIKeys(toolsCtx("developer"), nil, listKeysInput{Kind: "RestApi", ID: "orders"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `failed to list API keys for RestApi "orders": the Gateway refused the operation`)
	assert.NotContains(t, err.Error(), "connection reset", "an unmapped error stays generic")
}

func TestRotateIsInjection(t *testing.T) {
	assert.False(t, rotateIsInjection(rotateKeyInput{}))
	assert.False(t, rotateIsInjection(rotateKeyInput{ApiKey: "   "}), "whitespace is not a key value")
	assert.True(t, rotateIsInjection(rotateKeyInput{ApiKey: "a-real-value"}))
}

func TestRotateAPIKeyRegeneratesWithoutAnAPIKey(t *testing.T) {
	f := newToolsHandler(false)

	_, out, err := f.h.rotateAPIKey(toolsCtx("developer"), nil, rotateKeyInput{
		Kind: "RestApi", ID: "orders", KeyName: "prod-key", ApiKey: "  ",
		ExpiresIn: &expiresInInput{Duration: 30, Unit: "days"},
	})

	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, "regenerate", res["operation"])
	assert.Equal(t, "prod-key", res["keyName"])
	assert.Contains(t, res["notice"], "previous key value is now invalid")
	assert.Equal(t, 1, f.rest.keyCalls["rotate"])
	assert.Zero(t, f.rest.keyCalls["update"])
	assert.Equal(t, "prod-key", f.rest.keyName)
	require.NotNil(t, f.rest.rotateReq.ExpiresIn)
	assert.Equal(t, 30, f.rest.rotateReq.ExpiresIn.Duration)
}

func TestRotateAPIKeyInstallsASuppliedValue(t *testing.T) {
	f := newToolsHandler(false)
	value := strings.Repeat("v", 40)

	_, out, err := f.h.rotateAPIKey(toolsCtx("admin"), nil, rotateKeyInput{
		Kind: "RestApi", ID: "orders", KeyName: "prod-key", ApiKey: value,
	})

	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, "update", res["operation"])
	assert.NotContains(t, res, "notice", "the caller supplied the value, so it is never echoed")
	assert.Equal(t, 1, f.rest.keyCalls["update"])
	assert.Zero(t, f.rest.keyCalls["rotate"])
	require.NotNil(t, f.rest.updateReq.ApiKey)
	assert.Equal(t, value, *f.rest.updateReq.ApiKey)
}

// Regeneration and value injection are two different REST routes with two
// different role requirements; the tool must authorize the one it performs.
func TestRotateAPIKeyAuthorizesTheRouteItPerforms(t *testing.T) {
	f := newToolsHandler(false)

	_, _, err := f.h.rotateAPIKey(toolsCtx("developer"), nil, rotateKeyInput{
		Kind: "RestApi", ID: "orders", KeyName: "prod-key", ApiKey: strings.Repeat("v", 40),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient scope", "injection needs PUT .../{apiKeyName}, admin only")
	assert.Zero(t, f.rest.keyCalls["update"])

	_, _, err = f.h.rotateAPIKey(toolsCtx("developer"), nil, rotateKeyInput{
		Kind: "RestApi", ID: "orders", KeyName: "prod-key",
	})
	require.NoError(t, err, "regeneration is POST .../regenerate, which developer holds")
}

func TestRotateAPIKeyRefusesBeforeReachingTheService(t *testing.T) {
	for _, tt := range []struct {
		name      string
		immutable bool
		in        rotateKeyInput
		wantErr   string
	}{
		{"immutable mode", true, rotateKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k1"}, "immutable mode"},
		{"expiry in the past", false,
			rotateKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k1", ExpiresAt: "2020-01-01T00:00:00Z"}, "is in the past"},
		{"unknown expiresIn unit", false,
			rotateKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k1", ExpiresIn: &expiresInInput{Duration: 1, Unit: "eons"}}, "is not supported"},
		{"kind without API keys", false,
			rotateKeyInput{Kind: "Mcp", ID: "tools", KeyName: "k1"}, "does not have API keys"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newToolsHandler(tt.immutable)

			_, _, err := f.h.rotateAPIKey(toolsCtx("admin"), nil, tt.in)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Zero(t, f.rest.keyCalls["rotate"]+f.rest.keyCalls["update"])
		})
	}
}

func TestRotateAPIKeyMapsServiceErrors(t *testing.T) {
	f := newToolsHandler(false)
	f.rest.keyErr = fmt.Errorf("%w: generated keys cannot be overwritten", storage.ErrOperationNotAllowed)

	_, _, err := f.h.rotateAPIKey(toolsCtx("admin"), nil, rotateKeyInput{
		Kind: "RestApi", ID: "orders", KeyName: "k1", ApiKey: strings.Repeat("v", 40),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `cannot update the API key on RestApi "orders"`)
	assert.Contains(t, err.Error(), `call the same tool again without "apiKey"`,
		"the refusal must name the retry that works")

	f.rest.keyErr = fmt.Errorf("%w: key k1", storage.ErrNotFound)
	_, _, err = f.h.rotateAPIKey(toolsCtx("developer"), nil, rotateKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k1"})
	require.Error(t, err)
	assert.Equal(t, `cannot rotate the API key on RestApi "orders": no such resource, or no such key on it`, err.Error())
}

func TestRevokeAPIKey(t *testing.T) {
	f := newToolsHandler(false)

	_, out, err := f.h.revokeAPIKey(toolsCtx("developer"), nil,
		revokeKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k1", Confirm: true})

	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, "success", res["status"])
	assert.Equal(t, "k1", res["keyName"])
	assert.Equal(t, map[string]any{"revoked": "k1"}, res["result"])
	assert.Equal(t, 1, f.rest.keyCalls["revoke"])
}

func TestRevokeAPIKeyRefusals(t *testing.T) {
	f := newToolsHandler(false)
	_, _, err := f.h.revokeAPIKey(toolsCtx("developer"), nil, revokeKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k1"})
	require.Error(t, err)
	assert.Equal(t, `refusing to revoke API key "k1" on RestApi "orders": confirm the details with the user, then retry with confirm=true`,
		err.Error())

	f = newToolsHandler(true)
	_, _, err = f.h.revokeAPIKey(toolsCtx("developer"), nil,
		revokeKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k1", Confirm: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "immutable mode")
	assert.Zero(t, f.rest.keyCalls["revoke"])

	f = newToolsHandler(false)
	f.rest.keyErr = fmt.Errorf("%w: duplicate", storage.ErrConflict)
	_, _, err = f.h.revokeAPIKey(toolsCtx("developer"), nil,
		revokeKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k1", Confirm: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a key with that name already exists")
}

func TestKeyOpErrorNeverEchoesTheCause(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want string
	}{
		{"not found", fmt.Errorf("%w: secret-detail", storage.ErrNotFound), "no such resource, or no such key on it"},
		{"conflict", fmt.Errorf("%w: secret-detail", storage.ErrConflict), "a key with that name already exists"},
		{"operation not allowed", fmt.Errorf("%w: secret-detail", storage.ErrOperationNotAllowed),
			"a key value can only be installed over a key whose value was supplied"},
		{"anything else", errors.New("secret-detail"), "the Gateway refused the operation"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := keyOpError("rotate the API key on", "RestApi", "orders", tt.err)
			assert.Contains(t, got.Error(), tt.want)
			assert.Contains(t, got.Error(), `RestApi "orders"`)
			assert.NotContains(t, got.Error(), "secret-detail")
		})
	}
}

// The API-key service's expiry sentinels are client-input errors. The REST
// api-key handlers return them as 400 with the service's message, so keyOpError
// echoes it too, unlike every other cause.
func TestKeyOpErrorEchoesExpiryErrors(t *testing.T) {
	for _, err := range []error{
		fmt.Errorf("%w, got: 2020-01-01T00:00:00Z (current time: 2026-10-16T00:00:00Z)", utils.ErrAPIKeyExpirationInPast),
		fmt.Errorf("%w: fortnights", utils.ErrUnsupportedAPIKeyExpirationUnit),
	} {
		got := keyOpError("issue an API key for", "RestApi", "orders", err)
		assert.Equal(t, `cannot issue an API key for RestApi "orders": `+err.Error(), got.Error())
	}
}

// stubCertStore and stubSnapshot stand in for the router-side collaborators the
// certificate service reaches through its XDSResolver, so the MCP certificate
// tool's success paths are reachable without a real *xds.SnapshotManager.
type stubCertStore struct {
	reloadErr error
	combined  []byte
}

func (c *stubCertStore) Reload() error                   { return c.reloadErr }
func (c *stubCertStore) GetCombinedCertificates() []byte { return c.combined }

type stubSnapshot struct {
	err error
}

func (s *stubSnapshot) UpdateSnapshot(_ context.Context, _ string) error { return s.err }

func toolsCertService(db storage.Storage, store *stubCertStore, snapshot *stubSnapshot) *certificate.CertificateService {
	return certificate.NewCertificateService(certificate.Dependencies{
		DB: db,
		ResolveXDS: func() *certificate.XDSTargets {
			return &certificate.XDSTargets{Store: store, Snapshot: snapshot}
		},
		Logger: toolsDiscard,
	})
}

func certToolsHandler(db storage.Storage, store *stubCertStore, snapshot *stubSnapshot) *McpHandler {
	h := newAuthzTestHandler(false)
	h.logger = toolsDiscard
	h.certificateService = toolsCertService(db, store, snapshot)
	return h
}

// certToolsHandlerWithIdentity is certToolsHandler with an encryption
// provider, so usage: identity certificates can be stored. log receives the
// handler's and the service's log lines.
func certToolsHandlerWithIdentity(t *testing.T, db storage.Storage, log *slog.Logger) *McpHandler {
	t.Helper()
	mgr := testEncryptionManager(t)
	h := newAuthzTestHandler(false)
	h.logger = log
	h.certificateService = certificate.NewCertificateService(certificate.Dependencies{
		DB: db,
		ResolveXDS: func() *certificate.XDSTargets {
			return &certificate.XDSTargets{Store: &stubCertStore{}, Snapshot: &stubSnapshot{}}
		},
		EncryptionManager: func() *encryption.ProviderManager { return mgr },
		Logger:            log,
	})
	return h
}

var toolsSkippedCtx = withMcpCaller(context.Background(), mcpCaller{Skipped: true})

func TestManageCertificatesLifecycle(t *testing.T) {
	db := NewMockStorage()
	h := certToolsHandler(db, &stubCertStore{combined: []byte("0123456789")}, &stubSnapshot{})

	_, out, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "apply", Name: "upstream-ca", Certificate: validTestCert,
	})
	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, "create", res["operation"])
	assert.Equal(t, "Certificate stored and pushed to the router.", res["message"])
	created := toolsAs[CertificateResponse](t, res["certificate"])
	assert.Equal(t, "upstream-ca", created.Name)
	require.Len(t, db.certs, 1)

	_, out, err = h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{Action: "list"})
	require.NoError(t, err)
	res = resultMap(t, out)
	assert.Equal(t, 1, res["count"])
	assert.Equal(t, len(validTestCert), res["totalBytes"])
	items := toolsAs[[]CertificateResponse](t, res["items"])
	require.Len(t, items, 1)
	assert.Equal(t, created.ID, items[0].ID)
	encoded, err := json.Marshal(res)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "BEGIN CERTIFICATE", "list must never put stored PEM into the transcript")

	_, out, err = h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{Action: "reload", Confirm: true})
	require.NoError(t, err)
	assert.Equal(t, 10, resultMap(t, out)["totalBytes"])

	_, out, err = h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "delete", ID: created.ID, Confirm: true,
	})
	require.NoError(t, err)
	assert.Equal(t, created.ID, resultMap(t, out)["id"])
	assert.Empty(t, db.certs)
}

func TestManageCertificatesFailures(t *testing.T) {
	t.Run("invalid PEM", func(t *testing.T) {
		h := certToolsHandler(NewMockStorage(), &stubCertStore{}, &stubSnapshot{})
		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
			Action: "apply", Name: "bad", Certificate: "not a certificate",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "the Gateway refused the certificate: certificate upload is invalid:\n  1. certificate:")
	})

	// As on REST, a private key pasted into "certificate" is refused rather
	// than silently skipped and the chain stored.
	t.Run("a pasted private key is refused", func(t *testing.T) {
		db := NewMockStorage()
		h := certToolsHandler(db, &stubCertStore{}, &stubSnapshot{})
		withKey := validTestCert + "\n-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"
		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
			Action: "apply", Name: "upstream-ca", Certificate: withKey,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "the private key belongs in privateKey")
		assert.Empty(t, db.certs, "nothing may be stored")
	})

	t.Run("name already taken", func(t *testing.T) {
		db := NewMockStorage()
		db.saveErr = fmt.Errorf("%w: certificate name", storage.ErrConflict)
		h := certToolsHandler(db, &stubCertStore{}, &stubSnapshot{})
		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
			Action: "apply", Name: "upstream-ca", Certificate: validTestCert,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `a certificate named "upstream-ca" already exists`)
	})

	// The row is committed before the push, so the model is told not to repeat
	// the write and to retry only the push.
	t.Run("stored but not pushed", func(t *testing.T) {
		db := NewMockStorage()
		h := certToolsHandler(db, &stubCertStore{reloadErr: errors.New("boom")}, &stubSnapshot{})
		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
			Action: "apply", Name: "upstream-ca", Certificate: validTestCert,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "the change was stored but could not be pushed to the router")
		assert.Contains(t, err.Error(), "action=reload")
		assert.Len(t, db.certs, 1)
	})

	t.Run("reload push fails with nothing stored", func(t *testing.T) {
		h := certToolsHandler(NewMockStorage(), &stubCertStore{}, &stubSnapshot{err: errors.New("xds down")})
		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{Action: "reload", Confirm: true})
		require.Error(t, err)
		assert.Equal(t, "nothing changed: the trust store could not be pushed to the router (it failed at the snapshot stage)",
			err.Error())
	})

	t.Run("delete of an unknown id", func(t *testing.T) {
		h := certToolsHandler(NewMockStorage(), &stubCertStore{}, &stubSnapshot{})
		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
			Action: "delete", ID: "no-such-cert", Confirm: true,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `cannot delete certificate "no-such-cert": no certificate with that id`)
	})

	// As on REST, a certificate a deployed API trusts cannot be deleted, and
	// the error names that API so the model can remove the reference first.
	t.Run("delete refused while a deployed API trusts it", func(t *testing.T) {
		db := NewMockStorage()
		db.certs = []*models.StoredCertificate{{
			UUID: "cert-1", Name: "upstream-ca", Usage: models.CertificateUsageUpstream,
			Certificate: []byte(validTestCert),
		}}
		tls := map[string]interface{}{"trustedCAs": []interface{}{"upstream-ca"}}
		defs := []api.UpstreamDefinition{{Tls: &tls}}
		db.configs["cfg-orders"] = &models.StoredConfig{
			UUID: "cfg-orders", Handle: "orders", Kind: models.KindRestApi, DesiredState: models.StateDeployed,
			Configuration: api.RestAPI{Spec: api.APIConfigData{UpstreamDefinitions: &defs}},
		}
		h := certToolsHandler(db, &stubCertStore{}, &stubSnapshot{})

		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
			Action: "delete", ID: "cert-1", Confirm: true,
		})

		require.Error(t, err)
		assert.Equal(t, `cannot delete certificate "cert-1": certificate 'upstream-ca' is named by 1 deployed API; remove those references first:`+"\n"+
			`  1. API "orders" (spec.upstreamDefinitions[0].tls.trustedCAs[0])`, err.Error())
		assert.Len(t, db.certs, 1, "nothing is deleted")
	})

	t.Run("list failure is generic", func(t *testing.T) {
		db := NewMockStorage()
		db.getErr = errors.New("database is locked")
		h := certToolsHandler(db, &stubCertStore{}, &stubSnapshot{})
		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{Action: "list"})
		require.Error(t, err)
		assert.Equal(t, "the Gateway could not list the certificate", err.Error())
	})
}

func TestManageCertificatesDownstream(t *testing.T) {
	db := NewMockStorage()
	h := certToolsHandler(db, &stubCertStore{}, &stubSnapshot{})

	t.Run("a client authority defaults to role client", func(t *testing.T) {
		_, out, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
			Action: "apply", Name: "client-ca", Usage: "downstream",
			Certificate: string(pki.NewRootCA(t, "MCP Client CA").PEM()),
		})
		require.NoError(t, err)
		created := toolsAs[CertificateResponse](t, resultMap(t, out)["certificate"])
		assert.Equal(t, models.CertificateUsageDownstream, created.Usage)
		assert.Equal(t, models.CertificateRoleClient, created.Role)
	})

	t.Run("a relay entry keeps its match", func(t *testing.T) {
		_, out, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
			Action: "apply", Name: "relay-ca", Usage: "downstream", Role: "relay",
			Certificate: string(pki.NewRootCA(t, "MCP Relay CA").PEM()),
			Match:       &certificateMatchInput{URISANs: []string{"spiffe://example.org/proxy"}},
		})
		require.NoError(t, err)
		created := toolsAs[CertificateResponse](t, resultMap(t, out)["certificate"])
		assert.Equal(t, models.CertificateRoleRelay, created.Role)
		require.NotNil(t, created.Match)
		assert.Equal(t, []string{"spiffe://example.org/proxy"}, created.Match.URISANs)
	})

	// REST refuses an empty match object; the tool must too, rather than store
	// a relay entry its caller believed was narrowed.
	t.Run("an empty match is refused", func(t *testing.T) {
		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
			Action: "apply", Name: "relay-empty", Usage: "downstream", Role: "relay",
			Certificate: string(pki.NewRootCA(t, "MCP Empty CA").PEM()),
			Match:       &certificateMatchInput{},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "match: match must list uriSANs or dnsSANs, or be omitted")
	})

	t.Run("list filters by usage and reports references", func(t *testing.T) {
		require.NoError(t, db.SaveCertificate(&models.StoredCertificate{
			UUID: "up-1", Name: "upstream-only", Usage: models.CertificateUsageUpstream, Certificate: []byte(validTestCert),
		}))
		_, out, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{Action: "list", Usage: "downstream"})
		require.NoError(t, err)
		items := toolsAs[[]CertificateResponse](t, resultMap(t, out)["items"])
		require.Len(t, items, 2)
		for _, item := range items {
			assert.Equal(t, models.CertificateUsageDownstream, item.Usage)
			assert.NotNil(t, item.ReferencedByApis)
		}
	})

	t.Run("an unknown usage filter is refused", func(t *testing.T) {
		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{Action: "list", Usage: "sideways"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "the Gateway refused the listing: invalid usage filter")
	})
}

func TestManageCertificatesIdentity(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	db := NewMockStorage()
	h := certToolsHandlerWithIdentity(t, db, log)

	first := gatewayIdentityLeaf(t, "mcp-identity-v1")
	second := gatewayIdentityLeaf(t, "mcp-identity-v2")
	var transcript []string // every output and error the model would see
	record := func(out any, err error) {
		if out != nil {
			encoded, mErr := json.Marshal(out)
			require.NoError(t, mErr)
			transcript = append(transcript, string(encoded))
		}
		if err != nil {
			transcript = append(transcript, err.Error())
		}
	}

	_, out, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "apply", Name: "gw-identity", Usage: "identity", Certificate: string(first.PEM()),
	})
	record(out, err)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "privateKey: both certificate and privateKey are required for usage: identity")

	_, out, err = h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "apply", Name: "gw-identity", Usage: "identity", Role: "default",
		Certificate: string(first.PEM()), PrivateKey: string(first.KeyPEM()),
	})
	record(out, err)
	require.NoError(t, err)
	created := toolsAs[CertificateResponse](t, resultMap(t, out)["certificate"])
	assert.Equal(t, models.CertificateUsageIdentity, created.Usage)
	assert.Equal(t, models.CertificateRoleDefault, created.Role)
	assert.NotEmpty(t, created.KeyAlgorithm)
	assert.Equal(t, 1, created.ChainLength)

	_, out, err = h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "update", ID: created.ID, Confirm: true,
		Certificate: string(second.PEM()), PrivateKey: string(second.KeyPEM()),
	})
	record(out, err)
	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, "update", res["operation"])
	rotated := toolsAs[CertificateResponse](t, res["certificate"])
	assert.Equal(t, created.ID, rotated.ID, "rotation keeps the id")
	assert.Equal(t, "gw-identity", rotated.Name, "rotation keeps the name")
	assert.Equal(t, models.CertificateRoleDefault, rotated.Role, "rotation keeps the role")
	assert.Contains(t, rotated.Subject, "mcp-identity-v2")

	_, out, err = h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{Action: "list", Usage: "identity"})
	record(out, err)
	require.NoError(t, err)

	// The key must never reach the model or the log, on success or failure.
	keyLine := strings.Split(string(second.KeyPEM()), "\n")[1]
	for _, text := range append(transcript, logs.String()) {
		assert.NotContains(t, text, "PRIVATE KEY")
		assert.NotContains(t, text, keyLine)
	}
}

func TestManageCertificatesUpdateFailures(t *testing.T) {
	db := NewMockStorage()
	db.certs = []*models.StoredCertificate{{
		UUID: "up-1", Name: "upstream-ca", Usage: models.CertificateUsageUpstream, Certificate: []byte(validTestCert),
	}}
	h := certToolsHandlerWithIdentity(t, db, toolsDiscard)
	leaf := gatewayIdentityLeaf(t, "mcp-update-failures")

	_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "update", ID: "up-1", Confirm: true, Certificate: string(leaf.PEM()), PrivateKey: string(leaf.KeyPEM()),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage: only usage: identity certificates can be updated")

	_, _, err = h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "update", ID: "no-such-cert", Confirm: true, Certificate: string(leaf.PEM()), PrivateKey: string(leaf.KeyPEM()),
	})
	require.Error(t, err)
	assert.Equal(t, `no certificate with id "no-such-cert"; call this tool with action=list to find it`, err.Error())
}

// Without an encryption provider a gateway identity is never stored.
func TestManageCertificatesIdentityNeedsEncryption(t *testing.T) {
	db := NewMockStorage()
	h := certToolsHandler(db, &stubCertStore{}, &stubSnapshot{})
	leaf := gatewayIdentityLeaf(t, "mcp-no-encryption")

	_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "apply", Name: "gw-identity", Usage: "identity",
		Certificate: string(leaf.PEM()), PrivateKey: string(leaf.KeyPEM()),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no encryption provider configured")
	assert.Empty(t, db.certs)
}

// The certificate tool enforces the limit POST /certificates does, without
// stating it.
func TestManageCertificatesRequestSizeLimit(t *testing.T) {
	db := NewMockStorage()
	h := certToolsHandler(db, &stubCertStore{}, &stubSnapshot{})
	h.maxCertificateUploadBytes = 256

	_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "apply", Name: "upstream-ca", Certificate: validTestCert,
	})
	require.Error(t, err)
	assert.Equal(t, "the certificate request is too large for this Gateway; nothing was stored", err.Error())
	assert.Empty(t, db.certs)

	// As PUT /certificates/{id}, update checks the stored certificate before
	// the size, so an unknown id is reported as unknown, not as too large.
	_, _, err = h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "update", ID: "no-such-cert", Confirm: true, Certificate: validTestCert, PrivateKey: validTestCert,
	})
	require.Error(t, err)
	assert.Equal(t, `no certificate with id "no-such-cert"; call this tool with action=list to find it`, err.Error())
}

func TestMcpCertErrorMapsTheServiceErrors(t *testing.T) {
	err := mcpCertError(certActionApply, "ca", &certificate.ValidationError{
		Message: certificate.MsgUploadInvalid,
		Fields:  []certificate.FieldError{{Field: "name", Message: "name may contain only letters, digits, ., _ and -"}},
	})
	assert.Equal(t, "the Gateway refused the certificate: certificate upload is invalid:\n"+
		"  1. name: name may contain only letters, digits, ., _ and -", err.Error())

	err = mcpCertError(certActionDelete, "ca-id", &certificate.InUseError{
		Message:    "certificate 'ca' is named by 1 deployed API; remove those references first",
		References: []certificate.Reference{{Field: "spec.upstreamDefinitions[0].tls.trustedCAs[0]", API: "orders"}},
	})
	assert.Equal(t, `cannot delete certificate "ca-id": certificate 'ca' is named by 1 deployed API; remove those references first:`+"\n"+
		`  1. API "orders" (spec.upstreamDefinitions[0].tls.trustedCAs[0])`, err.Error())

	// The read failure behind a refused delete stays in the log.
	err = mcpCertError(certActionDelete, "ca-id", &certificate.ReferenceCheckError{
		Message: "Failed to verify certificate references", Cause: errors.New("database is locked"),
	})
	assert.Contains(t, err.Error(), "nothing was deleted")
	assert.NotContains(t, err.Error(), "database is locked")

	err = mcpCertError(certActionApply, "gw", &certificate.DefaultIdentityExistsError{Name: "first"})
	assert.Equal(t, "gateway identity first already has role: default; delete it before uploading another default identity", err.Error())

	err = mcpCertError(certActionApply, "gw", certificate.ErrIdentityStorageUnavailable)
	assert.Equal(t, "this Gateway has no encryption provider configured, so gateway identities cannot be stored", err.Error())

	err = mcpCertError(certActionApply, "gw", &certificate.EncryptionError{Cause: errors.New("kms down")})
	assert.NotContains(t, err.Error(), "kms down")
}

func toolsSubscriptionService(db storage.Storage) *subscription.SubscriptionService {
	return subscription.NewSubscriptionService(db,
		utils.NewSubscriptionResourceService(db, nil, &mockEventHub{}, "test-gateway"))
}

func subscriptionToolsHandler(db storage.Storage) *McpHandler {
	h := newAuthzTestHandler(false)
	h.logger = toolsDiscard
	h.subscriptionService = toolsSubscriptionService(db)
	return h
}

func TestManageSubscriptionPlanLifecycle(t *testing.T) {
	h := subscriptionToolsHandler(NewMockStorage())
	call := func(in manageSubscriptionsInput) (map[string]any, error) {
		in.Type = "SubscriptionPlan"
		_, out, err := h.manageSubscriptions(toolsSkippedCtx, nil, in)
		if err != nil {
			return nil, err
		}
		return resultMap(t, out), nil
	}

	res, err := call(manageSubscriptionsInput{Action: "apply", Spec: &subscriptionSpec{
		PlanName: ptr("gold"), ThrottleLimitCount: ptr(100), ThrottleLimitUnit: ptr("minute"),
	}})
	require.NoError(t, err)
	assert.Equal(t, "create", res["operation"])
	plan := toolsAs[api.SubscriptionPlanResponse](t, res["resource"])
	require.NotNil(t, plan.Id)
	planID := *plan.Id
	assert.Equal(t, "gold", *plan.PlanName)

	res, err = call(manageSubscriptionsInput{Action: "list"})
	require.NoError(t, err)
	assert.Equal(t, 1, res["count"])

	res, err = call(manageSubscriptionsInput{Action: "get", ID: planID})
	require.NoError(t, err)
	assert.Equal(t, planID, res["id"])

	res, err = call(manageSubscriptionsInput{Action: "apply", ID: planID, Spec: &subscriptionSpec{PlanName: ptr("platinum")}})
	require.NoError(t, err)
	assert.Equal(t, "update", res["operation"])
	assert.Equal(t, "platinum", *toolsAs[api.SubscriptionPlanResponse](t, res["resource"]).PlanName)

	res, err = call(manageSubscriptionsInput{Action: "delete", ID: planID, Confirm: true})
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("Subscription plan %q deleted successfully", planID), res["message"])

	_, err = call(manageSubscriptionsInput{Action: "get", ID: planID})
	require.Error(t, err)
	assert.Equal(t, fmt.Sprintf("no SubscriptionPlan with id %q; call this tool with action=list to see what exists", planID),
		err.Error())
}

// The plan update keeps what it always did: a field the spec sets is applied,
// a field it leaves out is kept.
func TestManageSubscriptionPlanUpdateKeepsWhatItDoesNotSet(t *testing.T) {
	h := subscriptionToolsHandler(NewMockStorage())
	call := func(in manageSubscriptionsInput) map[string]any {
		in.Type = "SubscriptionPlan"
		_, out, err := h.manageSubscriptions(toolsSkippedCtx, nil, in)
		require.NoError(t, err)
		return resultMap(t, out)
	}

	plan := toolsAs[api.SubscriptionPlanResponse](t, call(manageSubscriptionsInput{Action: "apply", Spec: &subscriptionSpec{
		PlanName: ptr("gold"), BillingPlan: ptr("stripe-gold"), ThrottleLimitCount: ptr(100), ThrottleLimitUnit: ptr("Min"),
	}})["resource"])

	updated := toolsAs[api.SubscriptionPlanResponse](t, call(manageSubscriptionsInput{Action: "apply", ID: *plan.Id, Spec: &subscriptionSpec{
		ThrottleLimitCount: ptr(500), ThrottleLimitUnit: ptr("hour"),
	}})["resource"])

	assert.Equal(t, 500, *updated.ThrottleLimitCount)
	assert.Equal(t, "Hour", *updated.ThrottleLimitUnit)
	assert.Equal(t, "stripe-gold", *updated.BillingPlan, "a field the spec leaves out is kept")
	assert.Equal(t, "gold", *updated.PlanName)
}

// A whitespace-only id carries no id, so apply creates: the same decision the
// authorization gate made when it authorized POST on the collection.
func TestManageSubscriptionPlanBlankIDCreates(t *testing.T) {
	h := subscriptionToolsHandler(NewMockStorage())

	_, out, err := h.manageSubscriptions(toolsSkippedCtx, nil, manageSubscriptionsInput{
		Type: "SubscriptionPlan", Action: "apply", ID: "   ", Spec: &subscriptionSpec{PlanName: ptr("silver")},
	})

	require.NoError(t, err)
	assert.Equal(t, "create", resultMap(t, out)["operation"])
}

func TestManageSubscriptionPlanFailures(t *testing.T) {
	h := subscriptionToolsHandler(NewMockStorage())

	_, _, err := h.manageSubscriptions(toolsSkippedCtx, nil, manageSubscriptionsInput{
		Type: "SubscriptionPlan", Action: "apply", Spec: &subscriptionSpec{PlanName: ptr("p"), ExpiryTime: ptr("soon")},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `spec.expiryTime "soon" is not a valid RFC 3339 timestamp`)

	_, _, err = h.manageSubscriptions(toolsSkippedCtx, nil, manageSubscriptionsInput{
		Type: "SubscriptionPlan", Action: "apply", ID: "no-such-plan", Spec: &subscriptionSpec{PlanName: ptr("p")},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no SubscriptionPlan with id "no-such-plan"`)

	_, _, err = h.manageSubscriptions(toolsSkippedCtx, nil, manageSubscriptionsInput{
		Type: "SubscriptionPlan", Action: "delete", ID: "no-such-plan", Confirm: true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no SubscriptionPlan with id "no-such-plan"`)
}

func TestManageSubscriptionLifecycle(t *testing.T) {
	db := NewMockStorage()
	require.NoError(t, db.SaveConfig(&models.StoredConfig{UUID: "api-uuid-1", Kind: models.KindRestApi, Handle: "petstore"}))
	h := subscriptionToolsHandler(db)
	call := func(in manageSubscriptionsInput) (map[string]any, error) {
		in.Type = "Subscription"
		_, out, err := h.manageSubscriptions(toolsSkippedCtx, nil, in)
		if err != nil {
			return nil, err
		}
		return resultMap(t, out), nil
	}

	// The handle is accepted in place of the deployment id.
	res, err := call(manageSubscriptionsInput{Action: "apply", Spec: &subscriptionSpec{
		ApiId: ptr("petstore"), SubscriptionToken: ptr("tok-123"), ApplicationId: ptr("app-1"),
	}})
	require.NoError(t, err)
	assert.Equal(t, "create", res["operation"])
	assert.Contains(t, res["notice"], "stored only as a hash")
	sub := toolsAs[api.SubscriptionResponse](t, res["resource"])
	require.NotNil(t, sub.Id)
	subID := *sub.Id
	require.NotNil(t, sub.SubscriptionToken, "create echoes the token once, as POST /subscriptions does")
	assert.Equal(t, "tok-123", *sub.SubscriptionToken)

	res, err = call(manageSubscriptionsInput{Action: "list", Spec: &subscriptionSpec{
		ApiId: ptr("petstore"), ApplicationId: ptr("app-1"), Status: ptr("active"),
	}})
	require.NoError(t, err)
	assert.Equal(t, 1, res["count"])
	listed := toolsAs[[]api.SubscriptionResponse](t, res["items"])
	require.Len(t, listed, 1)
	assert.Nil(t, listed[0].SubscriptionToken, "list never returns the token")

	res, err = call(manageSubscriptionsInput{Action: "get", ID: subID})
	require.NoError(t, err)
	assert.Equal(t, subID, res["id"])

	res, err = call(manageSubscriptionsInput{Action: "apply", ID: subID, Spec: &subscriptionSpec{Status: ptr("inactive")}})
	require.NoError(t, err)
	assert.Equal(t, "update", res["operation"])
	assert.Equal(t, api.SubscriptionResponseStatus("INACTIVE"), *toolsAs[api.SubscriptionResponse](t, res["resource"]).Status)
	assert.Contains(t, res["notice"], "changes only a subscription's status, plan and token")

	res, err = call(manageSubscriptionsInput{Action: "delete", ID: subID, Confirm: true})
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("Subscription %q deleted successfully", subID), res["message"])

	_, err = call(manageSubscriptionsInput{Action: "get", ID: subID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("no Subscription with id %q", subID))
}

func TestManageSubscriptionFailures(t *testing.T) {
	db := NewMockStorage()
	require.NoError(t, db.SaveConfig(&models.StoredConfig{UUID: "mcp-uuid-1", Kind: models.KindMcp, Handle: "tools"}))
	h := subscriptionToolsHandler(db)
	call := func(in manageSubscriptionsInput) error {
		in.Type = "Subscription"
		_, _, err := h.manageSubscriptions(toolsSkippedCtx, nil, in)
		return err
	}

	err := call(manageSubscriptionsInput{Action: "apply", Spec: &subscriptionSpec{ApiId: ptr("nope"), SubscriptionToken: ptr("t")}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no RestApi with identifier "nope" on this Gateway`)

	err = call(manageSubscriptionsInput{Action: "apply", Spec: &subscriptionSpec{ApiId: ptr("mcp-uuid-1"), SubscriptionToken: ptr("t")}})
	require.Error(t, err)
	assert.Equal(t, `"mcp-uuid-1" is a Mcp, not a RestApi. Only REST APIs bear subscriptions`, err.Error())

	err = call(manageSubscriptionsInput{Action: "list", Spec: &subscriptionSpec{ApiId: ptr("nope")}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no RestApi with identifier "nope"`)

	// As PUT /subscriptions/{id}, the stored subscription is read first, so an
	// unknown id is reported whatever the spec contains.
	err = call(manageSubscriptionsInput{Action: "apply", ID: "sub-1", Spec: &subscriptionSpec{}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no Subscription with id "sub-1"`)

	err = call(manageSubscriptionsInput{Action: "delete", ID: "no-such-sub", Confirm: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no Subscription with id "no-such-sub"`)
}

// specJSON decodes a spec as the SDK does, so the keys it carried, nulls
// included, are recorded exactly as an agent's call would record them.
func specJSON(t *testing.T, raw string) *subscriptionSpec {
	t.Helper()
	var spec subscriptionSpec
	require.NoError(t, json.Unmarshal([]byte(raw), &spec))
	return &spec
}

func TestSubscriptionSpecRecordsSentKeys(t *testing.T) {
	spec := specJSON(t, `{"billingPlan":null,"planName":"gold"}`)
	assert.Nil(t, spec.BillingPlan)
	assert.Equal(t, "gold", *spec.PlanName)
	fields := spec.sentFields()
	assert.Contains(t, fields, "billingPlan", "an explicit null is a sent key")
	assert.Contains(t, fields, "planName")
	assert.NotContains(t, fields, "expiryTime", "a key left out is not")

	// A spec built in code has no record, so its non-nil fields count as sent.
	built := (&subscriptionSpec{BillingPlan: ptr(""), PlanName: ptr("gold")}).sentFields()
	assert.Contains(t, built, "billingPlan")
	assert.Contains(t, built, "planName")
	assert.NotContains(t, built, "expiryTime")
}

// The subscription update has REST parity: PUT /subscriptions/{id} changes the
// status, moves or removes the plan, and rotates the token, each only when sent.
func TestManageSubscriptionPartialUpdate(t *testing.T) {
	db := NewMockStorage()
	require.NoError(t, db.SaveConfig(&models.StoredConfig{UUID: "api-uuid-1", Kind: models.KindRestApi, Handle: "petstore"}))
	h := subscriptionToolsHandler(db)
	call := func(in manageSubscriptionsInput) (map[string]any, error) {
		_, out, err := h.manageSubscriptions(toolsSkippedCtx, nil, in)
		if err != nil {
			return nil, err
		}
		return resultMap(t, out), nil
	}
	createPlan := func(name string) string {
		res, err := call(manageSubscriptionsInput{Type: "SubscriptionPlan", Action: "apply", Spec: &subscriptionSpec{PlanName: ptr(name)}})
		require.NoError(t, err)
		return *toolsAs[api.SubscriptionPlanResponse](t, res["resource"]).Id
	}
	update := func(raw string) (api.SubscriptionResponse, error) {
		res, err := call(manageSubscriptionsInput{Type: "Subscription", Action: "apply", ID: "apim-sub-1", Spec: specJSON(t, raw)})
		if err != nil {
			return api.SubscriptionResponse{}, err
		}
		return toolsAs[api.SubscriptionResponse](t, res["resource"]), nil
	}
	gold, silver := createPlan("gold"), createPlan("silver")

	// externalRefId becomes the subscription's id, so get and apply take it.
	res, err := call(manageSubscriptionsInput{Type: "Subscription", Action: "apply", Spec: &subscriptionSpec{
		ApiId: ptr("petstore"), SubscriptionToken: ptr("tok-1"), ExternalRefId: ptr("apim-sub-1"), SubscriptionPlanId: ptr(gold),
	}})
	require.NoError(t, err)
	assert.Equal(t, "apim-sub-1", *toolsAs[api.SubscriptionResponse](t, res["resource"]).Id)

	_, err = call(manageSubscriptionsInput{Type: "Subscription", Action: "apply", Spec: &subscriptionSpec{
		ApiId: ptr("petstore"), SubscriptionToken: ptr("tok-2"), ExternalRefId: ptr("apim-sub-1"),
	}})
	require.Error(t, err)
	assert.Equal(t, "the subscription conflicts with an existing subscription", err.Error())

	_, err = call(manageSubscriptionsInput{Type: "Subscription", Action: "apply", Spec: &subscriptionSpec{
		ApiId: ptr("petstore"), SubscriptionToken: ptr("tok-2"), ExternalRefId: ptr("-bad"),
	}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "externalRefId must be 1-64 characters")

	sub, err := update(`{"subscriptionPlanId":"` + silver + `"}`)
	require.NoError(t, err)
	assert.Equal(t, silver, *sub.SubscriptionPlanId, "the subscription moves to the plan sent")
	assert.EqualValues(t, "ACTIVE", *sub.Status)

	_, err = call(manageSubscriptionsInput{Type: "SubscriptionPlan", Action: "delete", ID: silver, Confirm: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is in use by one or more subscriptions, so it was not deleted")

	sub, err = update(`{"status":"inactive","subscriptionToken":"tok-rotated"}`)
	require.NoError(t, err)
	assert.EqualValues(t, "INACTIVE", *sub.Status)
	assert.Equal(t, silver, *sub.SubscriptionPlanId, "a plan left out is kept")
	assert.Nil(t, sub.SubscriptionToken, "an update never returns the token, as PUT /subscriptions/{id}")
	assert.Equal(t, "tok-rotated", db.subscriptions["apim-sub-1"].SubscriptionToken)

	sub, err = update(`{"subscriptionPlanId":null}`)
	require.NoError(t, err)
	assert.Nil(t, sub.SubscriptionPlanId, "an explicit null removes the plan")

	_, err = update(`{"subscriptionToken":"  "}`)
	require.Error(t, err)
	assert.Equal(t, "subscriptionToken must not be empty", err.Error())

	// With no subscription on it any more, the plan can be deleted.
	_, err = call(manageSubscriptionsInput{Type: "SubscriptionPlan", Action: "delete", ID: silver, Confirm: true})
	require.NoError(t, err)
}

// The plan update has REST parity: PUT /subscription-plans/{id} keeps a field
// left out and removes one sent as null.
func TestManageSubscriptionPlanNullRemoves(t *testing.T) {
	h := subscriptionToolsHandler(NewMockStorage())
	call := func(in manageSubscriptionsInput) (api.SubscriptionPlanResponse, error) {
		in.Type = "SubscriptionPlan"
		_, out, err := h.manageSubscriptions(toolsSkippedCtx, nil, in)
		if err != nil {
			return api.SubscriptionPlanResponse{}, err
		}
		return toolsAs[api.SubscriptionPlanResponse](t, resultMap(t, out)["resource"]), nil
	}

	plan, err := call(manageSubscriptionsInput{Action: "apply", Spec: specJSON(t,
		`{"planName":"gold","billingPlan":"stripe-gold","throttleLimitCount":100,"throttleLimitUnit":"Min","expiryTime":"2027-01-31T23:59:59Z"}`)})
	require.NoError(t, err)
	_, err = call(manageSubscriptionsInput{Action: "apply", Spec: &subscriptionSpec{PlanName: ptr("silver")}})
	require.NoError(t, err)

	_, err = call(manageSubscriptionsInput{Action: "apply", ID: *plan.Id, Spec: specJSON(t, `{"throttleLimitCount":null}`)})
	require.Error(t, err)
	assert.Equal(t, "throttleLimitCount and throttleLimitUnit must be set or cleared together", err.Error())

	_, err = call(manageSubscriptionsInput{Action: "apply", ID: *plan.Id, Spec: specJSON(t, `{"planName":"silver"}`)})
	require.Error(t, err)
	assert.Equal(t, "a subscription plan with this name already exists; choose a different spec.planName", err.Error())

	updated, err := call(manageSubscriptionsInput{Action: "apply", ID: *plan.Id, Spec: specJSON(t,
		`{"billingPlan":null,"throttleLimitCount":null,"throttleLimitUnit":null,"expiryTime":null}`)})
	require.NoError(t, err)
	assert.Nil(t, updated.BillingPlan)
	assert.Nil(t, updated.ThrottleLimitCount)
	assert.Nil(t, updated.ThrottleLimitUnit)
	assert.Nil(t, updated.ExpiryTime)
	assert.Equal(t, "gold", *updated.PlanName, "a field left out is kept")
}

// The tool applies the body limit POST and PUT on both collections enforce,
// checked after the stored resource on update, as PUT checks it.
func TestManageSubscriptionsRequestSizeLimit(t *testing.T) {
	db := NewMockStorage()
	require.NoError(t, db.SaveConfig(&models.StoredConfig{UUID: "api-uuid-1", Kind: models.KindRestApi, Handle: "petstore"}))
	h := subscriptionToolsHandler(db)
	huge := strings.Repeat("x", maxSubscriptionRequestBodyBytes)

	_, _, err := h.manageSubscriptions(toolsSkippedCtx, nil, manageSubscriptionsInput{
		Type: "Subscription", Action: "apply", Spec: &subscriptionSpec{ApiId: ptr("petstore"), SubscriptionToken: ptr(huge)},
	})
	require.Error(t, err)
	assert.Equal(t, "the Subscription request is too large for this Gateway; nothing was stored", err.Error())
	assert.Empty(t, db.subscriptions)

	_, _, err = h.manageSubscriptions(toolsSkippedCtx, nil, manageSubscriptionsInput{
		Type: "SubscriptionPlan", Action: "apply", Spec: &subscriptionSpec{PlanName: ptr("gold"), BillingPlan: ptr(huge)},
	})
	require.Error(t, err)
	assert.Equal(t, "the SubscriptionPlan request is too large for this Gateway; nothing was stored", err.Error())
	assert.Empty(t, db.subscriptionPlans)

	_, _, err = h.manageSubscriptions(toolsSkippedCtx, nil, manageSubscriptionsInput{
		Type: "SubscriptionPlan", Action: "apply", ID: "no-such-plan", Spec: &subscriptionSpec{BillingPlan: ptr(huge)},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no SubscriptionPlan with id "no-such-plan"`)
}

// Through the real MCP endpoint, a null in spec reaches the tool as a sent key:
// the SDK's input schema accepts it, and the spec records it on decode.
func TestManageSubscriptionsNullThroughTheEndpoint(t *testing.T) {
	server := createTestAPIServerWithDB(NewMockStorage())
	h := server.EnableMCP(map[string][]string{
		"POST /subscription-plans":         {"admin"},
		"PUT /subscription-plans/{planId}": {"admin"},
	}, map[string][]string{"admin": {"gw-admin"}}, "")
	admin := &commonmodels.AuthContext{UserID: "alice", Roles: []string{"admin"}}
	callTool := func(args map[string]any) api.SubscriptionPlanResponse {
		rec := mcpPost(t, h, admin, "tools/call", map[string]any{
			"name": "wso2_apip_gw_manage_subscriptions", "arguments": args,
		})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		result := toolsAs[map[string]any](t, decodeMCPResponse(t, rec)["result"])
		require.NotEqual(t, true, result["isError"], rec.Body.String())
		raw, err := json.Marshal(toolsAs[map[string]any](t, result["structuredContent"])["resource"])
		require.NoError(t, err)
		var plan api.SubscriptionPlanResponse
		require.NoError(t, json.Unmarshal(raw, &plan))
		return plan
	}

	plan := callTool(map[string]any{"type": "SubscriptionPlan", "action": "apply",
		"spec": map[string]any{"planName": "gold", "billingPlan": "stripe-gold"}})
	require.NotNil(t, plan.BillingPlan)

	updated := callTool(map[string]any{"type": "SubscriptionPlan", "action": "apply", "id": *plan.Id,
		"spec": map[string]any{"billingPlan": nil}})
	assert.Nil(t, updated.BillingPlan)
	assert.Equal(t, "gold", *updated.PlanName)
}

func TestMcpSubscriptionError(t *testing.T) {
	planOp := subAction{Type: subTypePlan, Action: subActionApply, RouteKey: "POST /subscription-plans"}
	planUpdateOp := subAction{Type: subTypePlan, Action: subActionApply, RouteKey: "PUT /subscription-plans/{planId}"}
	planDeleteOp := subAction{Type: subTypePlan, Action: subActionDelete, RouteKey: "DELETE /subscription-plans/{planId}"}
	subOp := subAction{Type: subTypeSubscription, Action: subActionApply, RouteKey: "POST /subscriptions"}
	subUpdateOp := subAction{Type: subTypeSubscription, Action: subActionApply, RouteKey: "PUT /subscriptions/{subscriptionId}"}

	for _, tt := range []struct {
		name string
		op   subAction
		err  error
		want string
	}{
		{"validation detail is echoed", subOp, &subscription.ValidationError{Message: "Subscription plan is not active"},
			"Subscription plan is not active"},
		{"not a REST API", subOp, &subscription.NotRestAPIError{Identifier: "tools", Kind: "Mcp"},
			`"tools" is a Mcp, not a RestApi. Only REST APIs bear subscriptions`},
		{"API not found", subOp, fmt.Errorf("%w: petstore", subscription.ErrAPINotFound),
			`no RestApi with identifier "petstore" on this Gateway; call wso2_apip_gw_list_resources with kind=RestApi to see what is deployed`},
		{"subscription not found", subOp, subscription.ErrSubscriptionNotFound,
			`no Subscription with id "petstore"; call this tool with action=list to see what exists`},
		{"plan not found", planOp, subscription.ErrPlanNotFound,
			`no SubscriptionPlan with id "petstore"; call this tool with action=list to see what exists`},
		{"duplicate plan", planOp, fmt.Errorf("%w: plan name", storage.ErrConflict),
			`a subscription plan named "petstore" already exists`},
		{"plan renamed onto another plan's name", planUpdateOp, fmt.Errorf("%w: plan name", storage.ErrConflict),
			"a subscription plan with this name already exists; choose a different spec.planName"},
		{"plan still in use", planDeleteOp, fmt.Errorf("%w: in use", storage.ErrConflict),
			`subscription plan "petstore" is in use by one or more subscriptions, so it was not deleted; ` +
				"move those subscriptions to another plan, or delete them, first"},
		// One message for every subscription conflict, as the REST API's 409.
		{"subscription create conflict", subOp, fmt.Errorf("%w: token", storage.ErrConflict),
			"the subscription conflicts with an existing subscription"},
		{"subscription update conflict", subUpdateOp, &subscription.OpError{Op: subscription.OpUpdate,
			Cause: fmt.Errorf("%w: token", storage.ErrConflict)},
			"the subscription conflicts with an existing subscription"},
		{"plan validation could not run", subOp, &subscription.OpError{Op: subscription.OpValidate,
			Cause: errors.New("database is locked")},
			"the Gateway could not validate the subscription plan"},
		{"anything else stays generic", subOp, errors.New("database is locked"),
			"the Gateway could not apply the Subscription"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, mcpSubscriptionError(tt.op, "petstore", tt.err).Error())
		})
	}
}

// mcpPost sends one JSON-RPC message through the real handler, as an MCP
// client would, and returns the recorder.
func mcpPost(t *testing.T, h http.Handler, auth *commonmodels.AuthContext, method string, params map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	if params == nil {
		params = map[string]any{}
	}
	params["_meta"] = map[string]any{
		"io.modelcontextprotocol/protocolVersion":    "2026-07-28",
		"io.modelcontextprotocol/clientInfo":         map[string]any{"name": "tools-test", "version": "1"},
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
	}
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/management/v1/mcp", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2026-07-28")
	req.Header.Set("Mcp-Method", method)
	if name, ok := params["name"].(string); ok {
		req.Header.Set("Mcp-Name", name)
	}
	if auth != nil {
		req = req.WithContext(authenticators.WithAuthContext(req.Context(), *auth))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// decodeMCPResponse reads the JSON-RPC response from either a JSON body or an
// SSE stream, whichever the SDK chose to answer with.
func decodeMCPResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	raw := rec.Body.String()
	if strings.Contains(rec.Header().Get("Content-Type"), "text/event-stream") {
		scanner := bufio.NewScanner(strings.NewReader(raw))
		scanner.Buffer(make([]byte, 0, 1<<20), 1<<24)
		raw = ""
		for scanner.Scan() {
			if line := scanner.Text(); strings.HasPrefix(line, "data:") {
				raw = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			}
		}
	}
	var msg map[string]any
	require.NoErrorf(t, json.Unmarshal([]byte(raw), &msg), "response was not JSON-RPC: %q", rec.Body.String())
	return msg
}

func registeredToolNames(t *testing.T, h http.Handler) []string {
	t.Helper()
	rec := mcpPost(t, h, nil, "tools/list", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	result, ok := decodeMCPResponse(t, rec)["result"].(map[string]any)
	require.True(t, ok, "tools/list returned no result: %s", rec.Body.String())
	var names []string
	for _, tool := range toolsAs[[]any](t, result["tools"]) {
		names = append(names, toolsAs[string](t, toolsAs[map[string]any](t, tool)["name"]))
	}
	return names
}

var baseMCPTools = []string{
	"wso2_apip_gw_deploy_api", "wso2_apip_gw_undeploy_api",
	"wso2_apip_gw_apply_config", "wso2_apip_gw_delete_config",
	"wso2_apip_gw_list_resources", "wso2_apip_gw_get_resource",
	"wso2_apip_gw_issue_api_key", "wso2_apip_gw_list_api_keys",
	"wso2_apip_gw_rotate_api_key", "wso2_apip_gw_revoke_api_key",
}

// The certificate and subscription tools are only registered when their service is set.
func TestNewMcpHandlerRegistersToolsForTheWiredServices(t *testing.T) {
	bare := newMcpHandler(McpHandlerParams{Logger: toolsDiscard})
	assert.ElementsMatch(t, baseMCPTools, registeredToolNames(t, bare))

	db := NewMockStorage()
	full := newMcpHandler(McpHandlerParams{
		Logger:              toolsDiscard,
		CertificateService:  toolsCertService(db, &stubCertStore{}, &stubSnapshot{}),
		SubscriptionService: toolsSubscriptionService(db),
	})
	assert.ElementsMatch(t,
		append(append([]string{}, baseMCPTools...), "wso2_apip_gw_manage_certificates", "wso2_apip_gw_manage_subscriptions"),
		registeredToolNames(t, full))
}

// A call the caller has no role for is refused at the HTTP layer with a 403 challenge.
func TestNewMcpHandlerGateStepsUpBeforeTheToolRuns(t *testing.T) {
	h := newMcpHandler(McpHandlerParams{
		Logger: toolsDiscard,
		ResourceRoles: map[string][]string{
			"POST /rest-apis":     {"developer", "admin"},
			"POST /llm-providers": {"admin"},
		},
	})
	developer := &commonmodels.AuthContext{UserID: "alice", Roles: []string{"developer"}}

	rec := mcpPost(t, h, developer, "tools/call", map[string]any{
		"name":      "wso2_apip_gw_deploy_api",
		"arguments": map[string]any{"kind": "LlmProvider", "yaml": "kind: LlmProvider\n"},
	})

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), `error="insufficient_scope"`)
}

// A tool's own refusal is a tool execution error (isError) inside a 200, not a
// JSON-RPC protocol error: it is what the model is meant to read and correct.
func TestNewMcpHandlerReturnsToolRefusalsAsToolErrors(t *testing.T) {
	h := newMcpHandler(McpHandlerParams{
		Logger:        toolsDiscard,
		Immutable:     true,
		ResourceRoles: map[string][]string{"POST /rest-apis": {"developer"}},
	})
	// No RestAPIService is wired, so record calls to Create instead of reaching a nil service.
	var createCalled bool
	h.kinds[models.KindRestApi].Create = func([]byte, string, *slog.Logger) (any, error) {
		createCalled = true
		return nil, errors.New("create reached")
	}
	developer := &commonmodels.AuthContext{UserID: "alice", Roles: []string{"developer"}}

	rec := mcpPost(t, h, developer, "tools/call", map[string]any{
		"name":      "wso2_apip_gw_deploy_api",
		"arguments": map[string]any{"kind": "RestApi", "yaml": "kind: RestApi\n"},
	})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	msg := decodeMCPResponse(t, rec)
	assert.NotContains(t, msg, "error", "a tool refusal is not a protocol error")
	result := toolsAs[map[string]any](t, msg["result"])
	assert.Equal(t, true, result["isError"])
	encoded, err := json.Marshal(result["content"])
	require.NoError(t, err)
	assert.Contains(t, string(encoded), "immutable mode")
	assert.False(t, createCalled, "an immutable gateway must refuse before the service is reached")
}
