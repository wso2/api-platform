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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/agent"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/graphqlapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/restapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// Fixtures shared by the tests below.

var errsTwo = []config.ValidationError{
	{Field: "spec.displayName", Message: "API display name must be URL-friendly"},
	{Field: "spec.upstream.main.url", Message: "Upstream URL must use http or https scheme"},
}

const errsTwoText = "Configuration validation failed (2 errors):\n" +
	"  1. spec.displayName: API display name must be URL-friendly\n" +
	"  2. spec.upstream.main.url: Upstream URL must use http or https scheme"

func errConflict() error {
	return fmt.Errorf("%w: configuration with handle 'orders' already exists", storage.ErrConflict)
}

func errInternal() error {
	return fmt.Errorf("failed to persist configuration update: %w", errors.New("database is locked"))
}

func errRenderSecret() error {
	return &templateengine.RenderError{Cause: &funcs.SecretError{Key: "db-pass", Cause: errors.New("not found")}}
}

func errRenderTemplate() error {
	return &templateengine.RenderError{Cause: &templateengine.TemplateParseError{
		Cause: errors.New(`template: artifact:3: function "secret2" not defined`)}}
}

func errRenderOther() error {
	return &templateengine.RenderError{Cause: errors.New("env var BACKEND_URL is not set")}
}

func errPolicyMissing() error {
	return fmt.Errorf("failed to resolve policies: %w", &utils.PolicyDefinitionMissingError{PolicyName: "token-based-ratelimit"})
}

// TestMcpWriteError walks every branch of every kind's copied REST mapping,
// through the dispatcher so the create/update prefix is covered too.
func TestMcpWriteError(t *testing.T) {
	tests := []struct {
		name   string
		kind   string
		op     mcpWriteOp
		handle string
		err    error
		want   string
	}{
		// RestApi create — mirrors RestAPIHandler.mapCreateError.
		{"RestApi create: render secret error", models.KindRestApi, mcpWriteOpCreate, "", errRenderSecret(),
			`failed to create RestApi: failed to resolve secret "db-pass": not found`},
		{"RestApi create: render template parse error", models.KindRestApi, mcpWriteOpCreate, "", errRenderTemplate(),
			`failed to create RestApi: function "secret2" not defined`},
		{"RestApi create: other render error", models.KindRestApi, mcpWriteOpCreate, "", errRenderOther(),
			"failed to create RestApi: Failed to render configuration: env var BACKEND_URL is not set"},
		{"RestApi create: conflict", models.KindRestApi, mcpWriteOpCreate, "", errConflict(),
			"failed to create RestApi: configuration already exists: configuration with handle 'orders' already exists"},
		{"RestApi create: validation list", models.KindRestApi, mcpWriteOpCreate, "",
			&utils.ValidationErrorListError{Errors: errsTwo},
			"failed to create RestApi: " + errsTwoText},
		{"RestApi create: parse failure is a bad request", models.KindRestApi, mcpWriteOpCreate, "",
			fmt.Errorf("failed to parse configuration: %w", errors.New("yaml: line 3: mapping values are not allowed")),
			"failed to create RestApi: failed to parse configuration: yaml: line 3: mapping values are not allowed"},
		{"RestApi create: internal error is hidden", models.KindRestApi, mcpWriteOpCreate, "", errInternal(),
			"failed to create RestApi: Failed to create configuration"},

		// RestApi update — mirrors RestAPIHandler.mapUpdateError.
		{"RestApi update: render error", models.KindRestApi, mcpWriteOpUpdate, "orders", errRenderOther(),
			`failed to update RestApi "orders": Failed to render configuration: env var BACKEND_URL is not set`},
		{"RestApi update: parse error", models.KindRestApi, mcpWriteOpUpdate, "orders",
			&restapi.ParseError{Cause: errors.New("unexpected end of input")},
			`failed to update RestApi "orders": Failed to parse configuration: unexpected end of input`},
		{"RestApi update: handle mismatch", models.KindRestApi, mcpWriteOpUpdate, "orders",
			&restapi.HandleMismatchError{PathHandle: "orders", YAMLHandle: "billing"},
			`failed to update RestApi "orders": handle mismatch: path has 'orders' but YAML metadata.name has 'billing'`},
		{"RestApi update: validation list", models.KindRestApi, mcpWriteOpUpdate, "orders",
			&restapi.ValidationError{Errors: errsTwo},
			`failed to update RestApi "orders": ` + errsTwoText},
		{"RestApi update: not found", models.KindRestApi, mcpWriteOpUpdate, "orders", restapi.ErrNotFound,
			`failed to update RestApi "orders": RestAPI with handle 'orders' not found`},
		{"RestApi update: conflict", models.KindRestApi, mcpWriteOpUpdate, "orders", errConflict(),
			`failed to update RestApi "orders": configuration already exists: configuration with handle 'orders' already exists`},
		{"RestApi update: internal error is hidden", models.KindRestApi, mcpWriteOpUpdate, "orders", errInternal(),
			`failed to update RestApi "orders": Failed to update configuration`},

		// GraphQLApi create — mirrors APIServer.CreateGraphQLAPI.
		{"GraphQLApi create: render error", models.KindGraphQLApi, mcpWriteOpCreate, "", errRenderTemplate(),
			`failed to create GraphQLApi: function "secret2" not defined`},
		{"GraphQLApi create: validation list", models.KindGraphQLApi, mcpWriteOpCreate, "",
			&utils.ValidationErrorListError{Errors: errsTwo},
			"failed to create GraphQLApi: " + errsTwoText},
		{"GraphQLApi create: conflict", models.KindGraphQLApi, mcpWriteOpCreate, "", errConflict(),
			"failed to create GraphQLApi: configuration already exists: configuration with handle 'orders' already exists"},
		{"GraphQLApi create: wrong kind in the body is a bad request", models.KindGraphQLApi, mcpWriteOpCreate, "",
			errors.New("unexpected configuration type management.GraphQLAPI after rendering"),
			"failed to create GraphQLApi: unexpected configuration type management.GraphQLAPI after rendering"},
		{"GraphQLApi create: internal error is hidden", models.KindGraphQLApi, mcpWriteOpCreate, "", errInternal(),
			"failed to create GraphQLApi: Failed to create configuration"},

		// GraphQLApi update — mirrors APIServer.UpdateGraphQLAPI.
		{"GraphQLApi update: not found", models.KindGraphQLApi, mcpWriteOpUpdate, "orders", graphqlapi.ErrNotFound,
			`failed to update GraphQLApi "orders": GraphQLApi with handle 'orders' not found`},
		{"GraphQLApi update: lookup failure is hidden", models.KindGraphQLApi, mcpWriteOpUpdate, "orders",
			&graphqlapi.LoadError{Cause: errors.New("database is locked")},
			`failed to update GraphQLApi "orders": Failed to retrieve configuration`},
		{"GraphQLApi update: parse error", models.KindGraphQLApi, mcpWriteOpUpdate, "orders",
			&graphqlapi.ParseError{Cause: errors.New("unexpected end of input")},
			`failed to update GraphQLApi "orders": failed to parse configuration: unexpected end of input`},
		{"GraphQLApi update: handle mismatch", models.KindGraphQLApi, mcpWriteOpUpdate, "orders",
			&graphqlapi.HandleMismatchError{PathHandle: "orders", YAMLHandle: "billing"},
			`failed to update GraphQLApi "orders": metadata.name 'billing' does not match path id 'orders'`},
		{"GraphQLApi update: validation list", models.KindGraphQLApi, mcpWriteOpUpdate, "orders",
			&utils.ValidationErrorListError{Errors: errsTwo},
			`failed to update GraphQLApi "orders": ` + errsTwoText},
		{"GraphQLApi update: conflict", models.KindGraphQLApi, mcpWriteOpUpdate, "orders", errConflict(),
			`failed to update GraphQLApi "orders": configuration already exists: configuration with handle 'orders' already exists`},
		{"GraphQLApi update: wrong kind is generic, as on REST", models.KindGraphQLApi, mcpWriteOpUpdate, "orders",
			errors.New("unexpected configuration type management.GraphQLAPI after rendering"),
			`failed to update GraphQLApi "orders": Failed to update configuration`},
		{"GraphQLApi update: internal error is hidden", models.KindGraphQLApi, mcpWriteOpUpdate, "orders", errInternal(),
			`failed to update GraphQLApi "orders": Failed to update configuration`},

		// Agent — mirrors AgentHandler.mapWriteError for both operations.
		{"Agent create: render error", models.KindAgent, mcpWriteOpCreate, "", errRenderSecret(),
			`failed to create Agent: failed to resolve secret "db-pass": not found`},
		{"Agent create: parse error", models.KindAgent, mcpWriteOpCreate, "",
			&agent.ParseError{Cause: errors.New("unexpected end of input")},
			"failed to create Agent: Failed to parse configuration: unexpected end of input"},
		{"Agent create: kind mismatch", models.KindAgent, mcpWriteOpCreate, "", &agent.KindMismatchError{Kind: "RestApi"},
			"failed to create Agent: kind mismatch: expected 'Agent' but payload declares 'RestApi'"},
		{"Agent create: validation list", models.KindAgent, mcpWriteOpCreate, "", &agent.ValidationError{Errors: errsTwo},
			"failed to create Agent: " + errsTwoText},
		{"Agent create: conflict", models.KindAgent, mcpWriteOpCreate, "", errConflict(),
			"failed to create Agent: configuration already exists: configuration with handle 'orders' already exists"},
		{"Agent create: database unavailable", models.KindAgent, mcpWriteOpCreate, "",
			fmt.Errorf("failed to persist agent configuration: %w", storage.ErrDatabaseUnavailable),
			"failed to create Agent: Database storage not available"},
		{"Agent create: internal error is hidden", models.KindAgent, mcpWriteOpCreate, "", errInternal(),
			"failed to create Agent: Failed to persist agent configuration"},
		{"Agent update: handle mismatch", models.KindAgent, mcpWriteOpUpdate, "orders",
			&agent.HandleMismatchError{PathHandle: "orders", YAMLHandle: "billing"},
			`failed to update Agent "orders": handle mismatch: path has 'orders' but YAML metadata.name has 'billing'`},
		{"Agent update: validation list", models.KindAgent, mcpWriteOpUpdate, "orders", &agent.ValidationError{Errors: errsTwo},
			`failed to update Agent "orders": ` + errsTwoText},
		{"Agent update: not found", models.KindAgent, mcpWriteOpUpdate, "orders", agent.ErrNotFound,
			`failed to update Agent "orders": Agent with handle 'orders' not found`},

		// LlmProxy create — mirrors APIServer.CreateLLMProxy.
		{"LlmProxy create: render error", models.KindLlmProxy, mcpWriteOpCreate, "", errRenderOther(),
			"failed to create LlmProxy: Failed to render configuration: env var BACKEND_URL is not set"},
		{"LlmProxy create: policy definition missing", models.KindLlmProxy, mcpWriteOpCreate, "", errPolicyMissing(),
			"failed to create LlmProxy: " + utils.PolicyDefinitionMissingUserMessage},
		{"LlmProxy create: validation", models.KindLlmProxy, mcpWriteOpCreate, "",
			fmt.Errorf("%w: 1 error(s): 1. spec.provider.id: is required", utils.ErrLLMProxyValidation),
			"failed to create LlmProxy: proxy configuration invalid: 1 error(s): 1. spec.provider.id: is required"},
		{"LlmProxy create: not found", models.KindLlmProxy, mcpWriteOpCreate, "", storage.ErrNotFound,
			"failed to create LlmProxy: LLM proxy configuration not found"},
		{"LlmProxy create: conflict", models.KindLlmProxy, mcpWriteOpCreate, "", errConflict(),
			"failed to create LlmProxy: configuration already exists: configuration with handle 'orders' already exists"},
		{"LlmProxy create: internal error is hidden", models.KindLlmProxy, mcpWriteOpCreate, "", errInternal(),
			"failed to create LlmProxy: Failed to create LLM proxy"},

		// LlmProxy update — mirrors APIServer.UpdateLLMProxy, which has no
		// conflict branch: a conflict falls to the generic message, as on REST.
		{"LlmProxy update: render error", models.KindLlmProxy, mcpWriteOpUpdate, "chat", errRenderOther(),
			`failed to update LlmProxy "chat": Failed to render configuration: env var BACKEND_URL is not set`},
		{"LlmProxy update: not found", models.KindLlmProxy, mcpWriteOpUpdate, "chat", storage.ErrNotFound,
			`failed to update LlmProxy "chat": LLM proxy configuration with handle 'chat' not found`},
		{"LlmProxy update: policy definition missing", models.KindLlmProxy, mcpWriteOpUpdate, "chat", errPolicyMissing(),
			`failed to update LlmProxy "chat": ` + utils.PolicyDefinitionMissingUserMessage},
		{"LlmProxy update: validation", models.KindLlmProxy, mcpWriteOpUpdate, "chat",
			fmt.Errorf("%w: 1 error(s): 1. spec.provider.id: is required", utils.ErrLLMProxyValidation),
			`failed to update LlmProxy "chat": proxy configuration invalid: 1 error(s): 1. spec.provider.id: is required`},
		{"LlmProxy update: conflict is generic, as on REST", models.KindLlmProxy, mcpWriteOpUpdate, "chat", errConflict(),
			`failed to update LlmProxy "chat": Failed to update LLM proxy configuration`},
		{"LlmProxy update: internal error is hidden", models.KindLlmProxy, mcpWriteOpUpdate, "chat", errInternal(),
			`failed to update LlmProxy "chat": Failed to update LLM proxy configuration`},

		// LlmProviderTemplate — mirrors the template handlers.
		{"LlmProviderTemplate create: validation", models.KindLlmProviderTemplate, mcpWriteOpCreate, "",
			fmt.Errorf("%w: 1 error(s): 1. spec.displayName: is required", utils.ErrLLMTemplateValidation),
			"failed to create LlmProviderTemplate: template configuration invalid: 1 error(s): 1. spec.displayName: is required"},
		{"LlmProviderTemplate create: conflict", models.KindLlmProviderTemplate, mcpWriteOpCreate, "", errConflict(),
			"failed to create LlmProviderTemplate: configuration already exists: configuration with handle 'orders' already exists"},
		{"LlmProviderTemplate create: internal error is hidden", models.KindLlmProviderTemplate, mcpWriteOpCreate, "", errInternal(),
			"failed to create LlmProviderTemplate: Failed to create LLM provider template"},
		{"LlmProviderTemplate update: not found", models.KindLlmProviderTemplate, mcpWriteOpUpdate, "openai",
			utils.ErrLLMTemplateNotFound,
			`failed to update LlmProviderTemplate "openai": Template with id 'openai' not found`},
		{"LlmProviderTemplate update: validation", models.KindLlmProviderTemplate, mcpWriteOpUpdate, "openai",
			fmt.Errorf("%w: 1 error(s): 1. spec.displayName: is required", utils.ErrLLMTemplateValidation),
			`failed to update LlmProviderTemplate "openai": template configuration invalid: 1 error(s): 1. spec.displayName: is required`},
		{"LlmProviderTemplate update: internal error is hidden", models.KindLlmProviderTemplate, mcpWriteOpUpdate, "openai", errInternal(),
			`failed to update LlmProviderTemplate "openai": Failed to update LLM provider template`},

		// LlmProvider — mirrors APIServer.Create/UpdateLLMProvider, which pass
		// the error through except for a missing policy definition.
		{"LlmProvider create: render error", models.KindLlmProvider, mcpWriteOpCreate, "", errRenderTemplate(),
			`failed to create LlmProvider: function "secret2" not defined`},
		{"LlmProvider create: policy definition missing", models.KindLlmProvider, mcpWriteOpCreate, "", errPolicyMissing(),
			"failed to create LlmProvider: " + utils.PolicyDefinitionMissingUserMessage},
		{"LlmProvider create: validation passes through", models.KindLlmProvider, mcpWriteOpCreate, "",
			errors.New("provider validation failed with 1 error(s): 1. spec.template: is required"),
			"failed to create LlmProvider: provider validation failed with 1 error(s): 1. spec.template: is required"},
		{"LlmProvider update: validation passes through", models.KindLlmProvider, mcpWriteOpUpdate, "openai",
			errors.New("provider validation failed with 1 error(s): 1. spec.template: is required"),
			`failed to update LlmProvider "openai": provider validation failed with 1 error(s): 1. spec.template: is required`},

		// Mcp — mirrors APIServer.CreateMCPProxy for both operations.
		{"Mcp create: render error", models.KindMcp, mcpWriteOpCreate, "", errRenderOther(),
			"failed to create Mcp: Failed to render configuration: env var BACKEND_URL is not set"},
		{"Mcp create: validation passes through", models.KindMcp, mcpWriteOpCreate, "",
			errors.New("configuration validation failed with 1 error(s): 1. spec.tools: is required"),
			"failed to create Mcp: configuration validation failed with 1 error(s): 1. spec.tools: is required"},
		{"Mcp create: conflict passes through", models.KindMcp, mcpWriteOpCreate, "", errConflict(),
			"failed to create Mcp: configuration already exists: configuration with handle 'orders' already exists"},
		// Deliberately NOT REST's behaviour: UpdateMCPProxy reports this as 404.
		{"Mcp update: validation passes through, not a not-found", models.KindMcp, mcpWriteOpUpdate, "weather",
			errors.New("configuration validation failed with 1 error(s): 1. spec.tools: is required"),
			`failed to update Mcp "weather": configuration validation failed with 1 error(s): 1. spec.tools: is required`},

		// Secret — the REST handlers return err.Error() on every branch.
		{"Secret create: passes through", models.KindSecret, mcpWriteOpCreate, "",
			errors.New("secret value exceeds the maximum size"),
			"failed to create Secret: secret value exceeds the maximum size"},
		{"Secret update: passes through", models.KindSecret, mcpWriteOpUpdate, "db-pass", errConflict(),
			`failed to update Secret "db-pass": configuration already exists: configuration with handle 'orders' already exists`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mcpWriteError(tt.kind, tt.op, tt.handle, tt.err, toolsDiscard)
			require.Error(t, err)
			assert.Equal(t, tt.want, err.Error())
		})
	}
}

// Internal detail must not reach the model for any kind whose REST handler
// hides it.
func TestMcpWriteErrorNeverEchoesInternalDetail(t *testing.T) {
	for _, kind := range []string{
		models.KindRestApi, models.KindGraphQLApi, models.KindAgent, models.KindLlmProxy, models.KindLlmProviderTemplate,
	} {
		for _, op := range []mcpWriteOp{mcpWriteOpCreate, mcpWriteOpUpdate} {
			err := mcpWriteError(kind, op, "x", errInternal(), toolsDiscard)
			assert.NotContainsf(t, err.Error(), "database is locked", "%s %s", kind, op)
			assert.NotContainsf(t, err.Error(), "failed to persist", "%s %s", kind, op)
		}
	}
}

// A read-back failure means the write was saved; it must not be reported as a
// failed save, which would send the model into a retry and a conflict.
func TestMcpWriteErrorReturnsReadBackErrorsUnchanged(t *testing.T) {
	readBack := &mcpReadBackError{msg: "failed to read stored agent configuration"}

	for _, op := range []mcpWriteOp{mcpWriteOpCreate, mcpWriteOpUpdate} {
		err := mcpWriteError(models.KindAgent, op, "orders", readBack, toolsDiscard)
		assert.Equal(t, "failed to read stored agent configuration", err.Error())
		assert.NotContains(t, err.Error(), "Failed to persist")
	}
}

// A kind with no mapping fails closed rather than echoing an error it cannot
// classify.
func TestMcpWriteErrorUnmappedKindIsGeneric(t *testing.T) {
	err := mcpWriteError("WebSubApi", mcpWriteOpCreate, "", errInternal(), toolsDiscard)

	assert.Equal(t, "failed to create WebSubApi: the Gateway could not complete the operation", err.Error())
	assert.NotContains(t, err.Error(), "database is locked")
}

func TestMcpFormatValidationErrors(t *testing.T) {
	assert.Equal(t, "Configuration validation failed (1 error):\n  1. spec.version: API version is required",
		mcpFormatValidationErrors([]config.ValidationError{{Field: "spec.version", Message: "API version is required"}}))

	assert.Equal(t, "Configuration validation failed (1 error):\n  1. at least one operation is required",
		mcpFormatValidationErrors([]config.ValidationError{{Message: "at least one operation is required"}}),
		"an entry with no field renders the message alone")
}

// Every kind an MCP tool can reach must have a write-error mapping. A new kind
// becomes reachable only by being added to kindAliases, so this fails the
// moment one is added without a mapping.
func TestEveryKindHasAWriteErrorMapping(t *testing.T) {
	kinds := canonicalKinds()
	for _, kind := range kinds {
		mapper, ok := mcpWriteErrorMappers[kind]
		require.Truef(t, ok, "kind %q has no MCP write-error mapping: copy its REST handler's "+
			"create/update error mapping into mcpWriteErrorMappers (mcp_management_errors.go)", kind)
		assert.NotNilf(t, mapper.Create, "kind %q: Create mapper is nil", kind)
		assert.NotNilf(t, mapper.Update, "kind %q: Update mapper is nil", kind)
	}
	for kind := range mcpWriteErrorMappers {
		assert.Containsf(t, kinds, kind, "mcpWriteErrorMappers has an entry for unknown kind %q", kind)
	}
}

// Parity with REST
//
// The copies of the three REST mapper functions are checked against the
// functions themselves: the same error goes through both, and the MCP text must
// carry the REST body's message and every entry of its errors array. If a REST
// mapper changes and its copy does not, this fails. (The other kinds' REST
// mapping is written inline in their handlers, so it has no function to compare
// against; TestMcpWriteError pins those.)

// parityInputs covers every branch of all three REST mappers, and is run
// through each of them: parity must hold for every error, not just the ones a
// mapper is written for.
func parityInputs() map[string]error {
	return map[string]error{
		"render secret":            errRenderSecret(),
		"render template":          errRenderTemplate(),
		"render other":             errRenderOther(),
		"conflict":                 errConflict(),
		"validation list (create)": &utils.ValidationErrorListError{Errors: errsTwo},
		"validation (rest update)": &restapi.ValidationError{Errors: errsTwo},
		"validation (agent)":       &agent.ValidationError{Errors: errsTwo},
		"bad request string":       fmt.Errorf("failed to parse configuration: %w", errors.New("bad yaml")),
		"rest parse error":         &restapi.ParseError{Cause: errors.New("bad yaml")},
		"agent parse error":        &agent.ParseError{Cause: errors.New("bad yaml")},
		"rest handle mismatch":     &restapi.HandleMismatchError{PathHandle: "orders", YAMLHandle: "billing"},
		"agent handle mismatch":    &agent.HandleMismatchError{PathHandle: "orders", YAMLHandle: "billing"},
		"agent kind mismatch":      &agent.KindMismatchError{Kind: "RestApi"},
		"rest not found":           restapi.ErrNotFound,
		"agent not found":          agent.ErrNotFound,
		"database unavailable":     fmt.Errorf("lookup failed: %w", storage.ErrDatabaseUnavailable),
		"internal":                 errInternal(),
	}
}

// restBody runs one REST mapper and decodes the body it wrote.
func restBody(t *testing.T, write func(w http.ResponseWriter)) api.ErrorResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	write(rec)
	var body api.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "REST mapper wrote: %s", rec.Body.String())
	return body
}

// assertMatchesREST checks the MCP message carries everything the REST body does.
func assertMatchesREST(t *testing.T, body api.ErrorResponse, got string) {
	t.Helper()
	if body.Errors == nil {
		assert.Equal(t, body.Message, got)
		return
	}
	assert.Truef(t, strings.HasPrefix(got, body.Message),
		"MCP text %q does not start with the REST message %q", got, body.Message)
	for i, e := range *body.Errors {
		want := fmt.Sprintf("%d. %s: %s", i+1, derefString(e.Field), derefString(e.Message))
		assert.Containsf(t, got, want, "REST errors[%d] is missing from the MCP text", i)
	}
}

func TestMcpRestAPICreateErrorMatchesREST(t *testing.T) {
	for name, err := range parityInputs() {
		t.Run(name, func(t *testing.T) {
			body := restBody(t, func(w http.ResponseWriter) { (&RestAPIHandler{}).mapCreateError(w, err) })
			assertMatchesREST(t, body, mcpRestAPICreateError(err))
		})
	}
}

func TestMcpRestAPIUpdateErrorMatchesREST(t *testing.T) {
	for name, err := range parityInputs() {
		t.Run(name, func(t *testing.T) {
			body := restBody(t, func(w http.ResponseWriter) { (&RestAPIHandler{}).mapUpdateError(w, "orders", err) })
			assertMatchesREST(t, body, mcpRestAPIUpdateError("orders", err))
		})
	}
}

func TestMcpAgentWriteErrorMatchesREST(t *testing.T) {
	for name, err := range parityInputs() {
		for _, op := range []string{"create", "update"} {
			t.Run(name+"/"+op, func(t *testing.T) {
				body := restBody(t, func(w http.ResponseWriter) { (&AgentHandler{}).mapWriteError(w, op, "orders", err) })
				assertMatchesREST(t, body, mcpAgentWriteError("orders", err))
			})
		}
	}
}

// GraphQL's REST error mapping is written inline in its handlers rather than in a
// function the parity tests above could call, so the same failing request is sent
// through the REST handler and through the MCP tool, each against a fresh gateway
// in the same state, and the MCP message must carry what the REST body does.
func TestMcpGraphQLAPIWriteErrorsMatchREST(t *testing.T) {
	const handle = "orders"
	valid := graphQLManifest(handle, "Orders", "v1.0", "/orders")
	noUpstream := []byte(strings.Replace(string(valid), "url: https://backend.example.com/graphql", `url: ""`, 1))
	wrongKind := []byte(strings.Replace(string(valid), "kind: GraphQLApi", "kind: RestApi", 1))
	seedOrders := func(t *testing.T, s *APIServer) { seedGraphQLAPIForAPIKeyHandlerTests(t, s, handle) }
	seedUsers := func(t *testing.T, s *APIServer) {
		createGraphQLAPIViaHandler(t, s, graphQLManifest("users", "Users", "v1.0", "/users"))
	}

	tests := []struct {
		name   string
		update bool
		setup  func(t *testing.T, s *APIServer)
		body   []byte
	}{
		{name: "create: malformed body", body: []byte("kind: [")},
		{name: "create: validation", body: noUpstream},
		{name: "create: wrong kind in the body", body: wrongKind},
		{name: "create: render error", body: graphQLManifest(handle, "{{ .unclosed", "v1.0", "/orders")},
		{name: "create: conflict", setup: seedUsers, body: graphQLManifest(handle, "Users", "v1.0", "/orders")},
		{name: "create: save failure", setup: func(_ *testing.T, s *APIServer) {
			s.db.(*MockStorage).updateErr = errors.New("disk full")
		}, body: valid},

		{name: "update: unknown handle", update: true, body: valid},
		{name: "update: lookup failure", update: true, setup: func(_ *testing.T, s *APIServer) {
			s.db.(*MockStorage).getErr = errors.New("database is locked")
		}, body: valid},
		{name: "update: malformed body", update: true, setup: seedOrders, body: []byte("kind: [")},
		{name: "update: handle mismatch", update: true, setup: seedOrders,
			body: graphQLManifest("other", "Orders", "v2.0", "/orders")},
		{name: "update: validation", update: true, setup: seedOrders, body: noUpstream},
		{name: "update: wrong kind in the body", update: true, setup: seedOrders, body: wrongKind},
		{name: "update: render error", update: true, setup: seedOrders,
			body: graphQLManifest(handle, "{{ .unclosed", "v2.0", "/orders")},
		{name: "update: conflict", update: true, setup: func(t *testing.T, s *APIServer) {
			seedOrders(t, s)
			seedUsers(t, s)
		}, body: graphQLManifest(handle, "Users", "v1.0", "/orders")},
		{name: "update: save failure", update: true, setup: func(t *testing.T, s *APIServer) {
			seedOrders(t, s)
			s.db.(*MockStorage).updateErr = errors.New("disk full")
		}, body: graphQLManifest(handle, "Orders", "v2.0", "/orders")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newServer := func() *APIServer {
				s := createTestAPIServer()
				if tt.setup != nil {
					tt.setup(t, s)
				}
				return s
			}

			restServer := newServer()
			body := restBody(t, func(w http.ResponseWriter) {
				if tt.update {
					_, r := createTestContextWithHeader(http.MethodPut, "/graphql-apis/"+handle, tt.body, yamlHeaders())
					restServer.UpdateGraphQLAPI(w, r, handle)
					return
				}
				_, r := createTestContextWithHeader(http.MethodPost, "/graphql-apis", tt.body, yamlHeaders())
				restServer.CreateGraphQLAPI(w, r)
			})

			in := deployInput{Kind: models.KindGraphQLApi, Yaml: string(tt.body)}
			prefix := "failed to create GraphQLApi: "
			if tt.update {
				in.ID = handle
				prefix = `failed to update GraphQLApi "orders": `
			}
			_, _, err := newServer().EnableMCP(nil, nil, "").deployAPI(toolsSkippedCtx, nil, in)
			require.Error(t, err)
			require.True(t, strings.HasPrefix(err.Error(), prefix), err.Error())

			assertMatchesREST(t, body, strings.TrimPrefix(err.Error(), prefix))
		})
	}
}

// graphQLManifest is a minimal valid GraphQLApi manifest.
func graphQLManifest(handle, displayName, version, context string) []byte {
	return []byte(fmt.Sprintf(`apiVersion: gateway.api-platform.wso2.com/v1
kind: GraphQLApi
metadata:
  name: %s
spec:
  displayName: %q
  version: %q
  context: %q
  upstream:
    main:
      url: https://backend.example.com/graphql
`, handle, displayName, version, context))
}

func yamlHeaders() map[string]string {
	return map[string]string{"Content-Type": "application/yaml"}
}

// createGraphQLAPIViaHandler deploys a GraphQLApi through the REST handler and fails
// the test unless it is created.
func createGraphQLAPIViaHandler(t *testing.T, server *APIServer, manifest []byte) {
	t.Helper()
	w, r := createTestContextWithHeader(http.MethodPost, "/graphql-apis", manifest, yamlHeaders())
	server.CreateGraphQLAPI(w, r)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}
