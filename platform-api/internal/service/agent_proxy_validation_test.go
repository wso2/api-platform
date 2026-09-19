/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

// Agent proxy contract validation, rule by rule.
//
// Each case starts from a request that is valid in every other respect and
// breaks exactly one rule, so a failure names the rule that regressed rather
// than the fixture that drifted. Rules enforced by the request decoder —
// unknown fields, explicit nulls, wrong types, a missing or mismatched protocol
// block — are covered in internal/dto, where the raw bytes are still in hand.

package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
)

// boolPtr is local to this file; strPtr already exists in the package.
func boolPtr(b bool) *bool { return &b }

// completeAgentCardContent is a managed card carrying every key the contract
// requires, plus a vendor extension that must survive untouched.
func completeAgentCardContent() api.AgentCardDocument {
	return api.AgentCardDocument{
		"name":        "Weather Agent",
		"description": "Provides forecasts and severe-weather alerts",
		"version":     "1.0.0",
		"supportedInterfaces": []any{
			map[string]any{"protocolBinding": "JSONRPC", "url": "https://agents.gw.com/weather/rpc", "protocolVersion": "1.0"},
		},
		"capabilities":       map[string]any{"streaming": true},
		"defaultInputModes":  []any{"text/plain"},
		"defaultOutputModes": []any{"text/plain"},
		"skills": []any{
			map[string]any{"id": "forecast", "name": "Forecast", "description": "Multi-day forecast", "tags": []any{"weather"}},
		},
		"x-vendor-custom": map[string]any{"kept": true},
	}
}

// fullAgentProxyValidationRequest exercises every optional block at once, so a
// rule that is only reachable through one of them is still reachable here by
// mutating it.
func fullAgentProxyValidationRequest() *api.A2AAgentProxy {
	content := completeAgentCardContent()
	managed := api.ManagedPublicAgentCardModeManaged
	publicMode := api.PublicAgentCardMode(managed)
	protectedMode := api.ProtectedAgentCardMode(api.PassthroughProtectedAgentCardModePassthrough)
	authType := api.ApiKey

	return &api.A2AAgentProxy{
		DisplayName: "Weather Agent",
		Description: strPtr("Provides forecasts and severe-weather alerts"),
		Version:     "v1.0",
		ProjectId:   "default-project",
		Context:     strPtr("/weather"),
		Vhost:       strPtr("agents.gw.com"),
		Protocol:    api.A2AAgentProxyProtocolA2a,
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{
				Url: strPtr("http://weather-agent:9000"),
				Auth: &api.UpstreamAuth{
					Type:   &authType,
					Header: strPtr("X-API-Key"),
					Value:  strPtr(`{{ secret "weather-upstream" }}`),
				},
			},
		},
		Resilience: &api.Resilience{IdleTimeout: strPtr("5s")},
		A2a: api.A2AProtocolConfig{
			ProtocolVersion: "1.0",
			Transports: []api.A2ATransport{
				{ProtocolBinding: api.JSONRPC, PathPrefix: strPtr("/rpc")},
				{ProtocolBinding: api.HTTPJSON, PathPrefix: strPtr("/rest")},
			},
			OperationConfigs: &api.A2AOperationConfigs{
				Policies: &[]api.Policy{{Name: "jwt-auth", Version: "v1"}},
				Operations: &[]api.A2AOperation{
					{Name: api.SendMessage, Resilience: &api.Resilience{Timeout: strPtr("30s")}},
					{Name: api.GetTask},
				},
			},
			AgentCard: &api.AgentCardConfig{
				Public: &api.PublicAgentCard{
					Mode:     &publicMode,
					Path:     strPtr("/.well-known/agent-card.json"),
					Policies: &[]api.Policy{{Name: "cors", Version: "v1"}},
					Content:  &content,
				},
				Protected: &api.ProtectedAgentCard{
					Mode:        &protectedMode,
					RewriteUrls: boolPtr(false),
				},
			},
		},
	}
}

// TestValidateAgentProxyRequestAcceptsValidBodies is the other half of the
// rejection table: a rule that rejects everything passes every negative case,
// so the shapes the contract actually permits are pinned too.
func TestValidateAgentProxyRequestAcceptsValidBodies(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*api.A2AAgentProxy)
	}{
		{name: "every optional block populated", mutate: func(*api.A2AAgentProxy) {}},
		{
			name: "no optional blocks at all",
			mutate: func(r *api.A2AAgentProxy) {
				r.Description, r.Context, r.Vhost, r.Resilience = nil, nil, nil, nil
				r.A2a.OperationConfigs, r.A2a.AgentCard = nil, nil
				r.Upstream.Main.Auth = nil
			},
		},
		{
			// An omitted public mode means passthrough, which is also what an
			// omitted card block means — so the two agree by construction.
			name: "passthrough public card with an omitted mode",
			mutate: func(r *api.A2AAgentProxy) {
				r.A2a.AgentCard.Public = &api.PublicAgentCard{RewriteUrls: boolPtr(true)}
			},
		},
		{
			name: "managed protected card",
			mutate: func(r *api.A2AAgentProxy) {
				content := completeAgentCardContent()
				mode := api.ProtectedAgentCardMode(api.ManagedProtectedAgentCardModeManaged)
				r.A2a.AgentCard.Protected = &api.ProtectedAgentCard{Mode: &mode, Content: &content}
			},
		},
		{
			// "/" is the documented default and inserts no extra segment, so it
			// must pass the same check a real prefix does.
			name: "root path prefix",
			mutate: func(r *api.A2AAgentProxy) {
				r.A2a.Transports = []api.A2ATransport{{ProtocolBinding: api.JSONRPC, PathPrefix: strPtr("/")}}
			},
		},
		{
			name: "upstream by ref instead of url",
			mutate: func(r *api.A2AAgentProxy) {
				r.Upstream.Main = api.UpstreamDefinition{Ref: strPtr("weather-backend")}
			},
		},
		{
			// An absent id is the request to derive one from displayName, and
			// is the only form that asks for that.
			name:   "omitted id",
			mutate: func(r *api.A2AAgentProxy) { r.Id = nil },
		},
		{
			name:   "supplied id",
			mutate: func(r *api.A2AAgentProxy) { r.Id = strPtr("weather-agent") },
		},
		{
			name: "sandbox upstream alongside main",
			mutate: func(r *api.A2AAgentProxy) {
				r.Upstream.Sandbox = &api.UpstreamDefinition{Url: strPtr("https://sandbox.weather-agent.example.com")}
			},
		},
		{
			// A blank policy version means "latest" to the gateway's resolver,
			// so the control plane must not reject it either.
			name: "policy with an unspecified version",
			mutate: func(r *api.A2AAgentProxy) {
				r.A2a.OperationConfigs.Policies = &[]api.Policy{{Name: "jwt-auth"}}
			},
		},
		{
			name:   "wildcard vhost in the left-most label",
			mutate: func(r *api.A2AAgentProxy) { r.Vhost = strPtr("*.gw.com") },
		},
		{
			name: "custom public card path",
			mutate: func(r *api.A2AAgentProxy) {
				r.A2a.AgentCard.Public.Path = strPtr("/agent/card.json")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := fullAgentProxyValidationRequest()
			tc.mutate(req)
			if err := validateAgentProxyRequest(req); err != nil {
				t.Fatalf("expected the body to be accepted, got: %v", err)
			}
		})
	}
}

func TestValidateAgentProxyRequestRejections(t *testing.T) {
	tests := []struct {
		// rule names the Section 5 rule the case covers, so a failure points at
		// the contract clause rather than only at the field.
		rule     string
		name     string
		mutate   func(*api.A2AAgentProxy)
		contains string
	}{
		// Rule 1 — required fields, types, enum membership.
		{rule: "1", name: "missing displayName", contains: "displayName field is required",
			mutate: func(r *api.A2AAgentProxy) { r.DisplayName = "   " }},
		{rule: "1", name: "displayName too long", contains: "at most 128 characters",
			mutate: func(r *api.A2AAgentProxy) { r.DisplayName = strings.Repeat("a", 129) }},
		{rule: "1", name: "displayName with an illegal character", contains: "displayName may contain only",
			mutate: func(r *api.A2AAgentProxy) { r.DisplayName = "Weather/Agent" }},
		{rule: "1", name: "description too long", contains: "at most 1023 characters",
			mutate: func(r *api.A2AAgentProxy) { r.Description = strPtr(strings.Repeat("a", 1024)) }},
		{rule: "1", name: "missing version", contains: "version field is required",
			mutate: func(r *api.A2AAgentProxy) { r.Version = "" }},
		{rule: "1", name: "version not vMAJOR.MINOR", contains: "version must look like v1.0",
			mutate: func(r *api.A2AAgentProxy) { r.Version = "1.0.0" }},
		{rule: "1", name: "wrong kind", contains: `kind field must be "AgentProxy"`,
			mutate: func(r *api.A2AAgentProxy) {
				kind := api.A2AAgentProxyKind("RestApi")
				r.Kind = &kind
			}},
		// Rule 2 — routing. Handle syntax, reservation, uniqueness and
		// body/path agreement are asserted on the service and handler paths,
		// where the organization and the route are in hand.
		{rule: "2", name: "missing projectId", contains: "projectId field is required",
			mutate: func(r *api.A2AAgentProxy) { r.ProjectId = "" }},
		{rule: "2", name: "projectId not a handle", contains: "projectId must be lowercase",
			mutate: func(r *api.A2AAgentProxy) { r.ProjectId = "Default_Project" }},
		{rule: "2", name: "context not a path", contains: "context must be a valid path",
			mutate: func(r *api.A2AAgentProxy) { r.Context = strPtr("weather") }},
		{rule: "2", name: "context with a trailing slash", contains: "context must be a valid path",
			mutate: func(r *api.A2AAgentProxy) { r.Context = strPtr("/weather/") }},
		{rule: "2", name: "empty context", contains: "context must not be empty",
			mutate: func(r *api.A2AAgentProxy) { r.Context = strPtr("") }},
		{rule: "2", name: "context too long", contains: "context must be at most 200 characters",
			mutate: func(r *api.A2AAgentProxy) { r.Context = strPtr("/" + strings.Repeat("a", 200)) }},
		{rule: "2", name: "vhost is not a hostname", contains: "vhost must be a valid hostname",
			mutate: func(r *api.A2AAgentProxy) { r.Vhost = strPtr("not a host") }},
		{rule: "2", name: "wildcard outside the left-most vhost label", contains: "wildcard only in its left-most label",
			mutate: func(r *api.A2AAgentProxy) { r.Vhost = strPtr("agents.*.com") }},
		{rule: "1", name: "explicitly empty id", contains: "id cannot be empty",
			mutate: func(r *api.A2AAgentProxy) { r.Id = strPtr("") }},
		{rule: "1", name: "id shorter than the handle minimum", contains: "id must be at least 3 characters",
			mutate: func(r *api.A2AAgentProxy) { r.Id = strPtr("ab") }},
		{rule: "1", name: "id longer than the handle maximum", contains: "id must be at most 40 characters",
			mutate: func(r *api.A2AAgentProxy) { r.Id = strPtr(strings.Repeat("a", 41)) }},
		{rule: "1", name: "id with an illegal character", contains: "id must be lowercase alphanumeric",
			mutate: func(r *api.A2AAgentProxy) { r.Id = strPtr("weather.agent") }},
		{rule: "1", name: "id with surrounding whitespace", contains: "id must be lowercase alphanumeric",
			mutate: func(r *api.A2AAgentProxy) { r.Id = strPtr(" weather-agent ") }},
		// Rule 5 — upstream shape. Credential completeness is checked against
		// the effective configuration after PUT retention, not here.
		{rule: "5", name: "upstream with neither url nor ref", contains: "upstream main must specify either a url or a ref",
			mutate: func(r *api.A2AAgentProxy) { r.Upstream.Main = api.UpstreamDefinition{} }},
		{rule: "5", name: "upstream with both url and ref", contains: "not both",
			mutate: func(r *api.A2AAgentProxy) { r.Upstream.Main.Ref = strPtr("weather-backend") }},
		{
			// Both keys are present, which matches both oneOf branches and so
			// satisfies neither — however empty one of the two values is.
			rule: "5", name: "upstream with a url and an empty ref", contains: "not both",
			mutate: func(r *api.A2AAgentProxy) { r.Upstream.Main.Ref = strPtr("") },
		},
		{rule: "5", name: "upstream with an empty url and a ref", contains: "not both",
			mutate: func(r *api.A2AAgentProxy) {
				r.Upstream.Main = api.UpstreamDefinition{Url: strPtr(""), Ref: strPtr("weather-backend")}
			}},
		{rule: "5", name: "upstream with an empty ref alone", contains: "upstream main ref must not be empty",
			mutate: func(r *api.A2AAgentProxy) {
				r.Upstream.Main = api.UpstreamDefinition{Ref: strPtr("   ")}
			}},
		{rule: "5", name: "upstream url with surrounding whitespace", contains: "upstream main url is not a valid URL",
			mutate: func(r *api.A2AAgentProxy) { r.Upstream.Main.Url = strPtr("  http://weather-agent:9000  ") }},
		{rule: "5", name: "upstream url with an unsupported scheme", contains: "upstream main url is not a valid URL",
			mutate: func(r *api.A2AAgentProxy) { r.Upstream.Main.Url = strPtr("file:///etc/passwd") }},
		{rule: "5", name: "upstream url carrying credentials", contains: "upstream main url is not a valid URL",
			mutate: func(r *api.A2AAgentProxy) { r.Upstream.Main.Url = strPtr("http://user:pass@weather-agent:9000") }},
		{rule: "5", name: "unsupported upstream auth type", contains: "auth type \"hmac\" is not supported",
			mutate: func(r *api.A2AAgentProxy) {
				authType := api.UpstreamAuthType("hmac")
				r.Upstream.Main.Auth.Type = &authType
			}},
		{rule: "5", name: "sandbox upstream with neither url nor ref", contains: "upstream sandbox must specify either a url or a ref",
			mutate: func(r *api.A2AAgentProxy) { r.Upstream.Sandbox = &api.UpstreamDefinition{} }},
		// Rule 6 — protocol and its matching configuration block.
		{rule: "6", name: "unsupported protocol", contains: `protocol "grpc" is not supported`,
			mutate: func(r *api.A2AAgentProxy) { r.Protocol = "grpc" }},
		{rule: "6", name: "missing protocolVersion", contains: "a2a.protocolVersion field is required",
			mutate: func(r *api.A2AAgentProxy) { r.A2a.ProtocolVersion = "" }},
		{rule: "6", name: "unregistered protocolVersion", contains: `a2a.protocolVersion "2.0" is not supported`,
			mutate: func(r *api.A2AAgentProxy) { r.A2a.ProtocolVersion = "2.0" }},
		{
			// The enum is an exact match, and this exact string is what gets
			// stored — so a padded version is a rejection, not a value to trim
			// on the way past.
			rule: "6", name: "protocolVersion with surrounding whitespace", contains: "is not supported",
			mutate: func(r *api.A2AAgentProxy) { r.A2a.ProtocolVersion = " 1.0 " },
		},
		{rule: "6", name: "whitespace-only protocolVersion", contains: "is not supported",
			mutate: func(r *api.A2AAgentProxy) { r.A2a.ProtocolVersion = "   " }},
		// Rule 7 — canonical operation names.
		{rule: "7", name: "unknown operation name", contains: `"Teleport" is not an A2A 1.0 operation`,
			mutate: func(r *api.A2AAgentProxy) {
				(*r.A2a.OperationConfigs.Operations)[0].Name = "Teleport"
			}},
		{rule: "7", name: "operation name with the wrong case", contains: "is not an A2A 1.0 operation",
			mutate: func(r *api.A2AAgentProxy) {
				(*r.A2a.OperationConfigs.Operations)[0].Name = "sendmessage"
			}},
		{rule: "7", name: "duplicate operation configuration", contains: `"SendMessage" appears more than once`,
			mutate: func(r *api.A2AAgentProxy) {
				(*r.A2a.OperationConfigs.Operations)[1].Name = api.SendMessage
			}},
		{rule: "7", name: "policy version that is not major-only", contains: "invalid version",
			mutate: func(r *api.A2AAgentProxy) {
				r.A2a.OperationConfigs.Policies = &[]api.Policy{{Name: "jwt-auth", Version: "v1.0.0"}}
			}},
		{rule: "7", name: "policy with no name", contains: "policies[0].name field is required",
			mutate: func(r *api.A2AAgentProxy) {
				r.A2a.OperationConfigs.Policies = &[]api.Policy{{Version: "v1"}}
			}},
		{rule: "7", name: "per-operation resilience with a bad duration", contains: "must be a duration",
			mutate: func(r *api.A2AAgentProxy) {
				(*r.A2a.OperationConfigs.Operations)[0].Resilience = &api.Resilience{Timeout: strPtr("30 seconds")}
			}},
		{rule: "7", name: "agent-wide resilience with a bad duration", contains: "resilience.idleTimeout must be a duration",
			mutate: func(r *api.A2AAgentProxy) { r.Resilience = &api.Resilience{IdleTimeout: strPtr("5")} }},
		// Rule 8 — transports.
		{rule: "8", name: "no transports", contains: "At least one a2a.transports entry is required",
			mutate: func(r *api.A2AAgentProxy) { r.A2a.Transports = nil }},
		{rule: "8", name: "more than two transports", contains: "At most 2 a2a.transports entries",
			mutate: func(r *api.A2AAgentProxy) {
				r.A2a.Transports = append(r.A2a.Transports, api.A2ATransport{ProtocolBinding: api.JSONRPC})
			}},
		{rule: "8", name: "unsupported protocol binding", contains: `protocolBinding "GRPC" is not supported`,
			mutate: func(r *api.A2AAgentProxy) { r.A2a.Transports[0].ProtocolBinding = "GRPC" }},
		{rule: "8", name: "duplicate protocol binding", contains: `"JSONRPC" appears more than once`,
			mutate: func(r *api.A2AAgentProxy) { r.A2a.Transports[1].ProtocolBinding = api.JSONRPC }},
		{rule: "8", name: "relative path prefix", contains: "pathPrefix must be an absolute path",
			mutate: func(r *api.A2AAgentProxy) { r.A2a.Transports[0].PathPrefix = strPtr("rpc") }},
		{rule: "8", name: "path prefix with a query string", contains: "pathPrefix must be an absolute path",
			mutate: func(r *api.A2AAgentProxy) { r.A2a.Transports[0].PathPrefix = strPtr("/rpc?v=1") }},
		{rule: "8", name: "empty path prefix", contains: "pathPrefix must not be empty",
			mutate: func(r *api.A2AAgentProxy) { r.A2a.Transports[0].PathPrefix = strPtr("") }},
		// Rule 9 — public card path.
		{rule: "9", name: "card path with a query string", contains: "must not contain a query string or a fragment",
			mutate: func(r *api.A2AAgentProxy) { r.A2a.AgentCard.Public.Path = strPtr("/card.json?v=1") }},
		{rule: "9", name: "card path with a fragment", contains: "must not contain a query string or a fragment",
			mutate: func(r *api.A2AAgentProxy) { r.A2a.AgentCard.Public.Path = strPtr("/card.json#top") }},
		{rule: "9", name: "templated card path", contains: "must be a fixed path, not a template",
			mutate: func(r *api.A2AAgentProxy) { r.A2a.AgentCard.Public.Path = strPtr("/{tenant}/card.json") }},
		{rule: "9", name: "card path with a trailing slash", contains: "must not end with a trailing slash",
			mutate: func(r *api.A2AAgentProxy) { r.A2a.AgentCard.Public.Path = strPtr("/cards/") }},
		{rule: "9", name: "relative card path", contains: "must be an absolute path starting with /",
			mutate: func(r *api.A2AAgentProxy) { r.A2a.AgentCard.Public.Path = strPtr("card.json") }},
		{rule: "9", name: "card path too long", contains: "must be between 2 and 200 characters",
			mutate: func(r *api.A2AAgentProxy) { r.A2a.AgentCard.Public.Path = strPtr("/" + strings.Repeat("a", 200)) }},
		// Rule 10 — card modes and the fields each one permits.
		{rule: "10", name: "unsupported public card mode", contains: `public.mode "cached" is not supported`,
			mutate: func(r *api.A2AAgentProxy) {
				mode := api.PublicAgentCardMode("cached")
				r.A2a.AgentCard.Public.Mode = &mode
			}},
		{rule: "10", name: "managed public card with no content", contains: "public.content field is required in managed mode",
			mutate: func(r *api.A2AAgentProxy) { r.A2a.AgentCard.Public.Content = nil }},
		{rule: "10", name: "passthrough public card carrying content", contains: "public.content field is not allowed in passthrough mode",
			mutate: func(r *api.A2AAgentProxy) {
				mode := api.PublicAgentCardMode(api.PassthroughPublicAgentCardModePassthrough)
				r.A2a.AgentCard.Public.Mode = &mode
			}},
		{rule: "10", name: "managed public card with rewriteUrls", contains: "public.rewriteUrls field applies to passthrough cards only",
			mutate: func(r *api.A2AAgentProxy) { r.A2a.AgentCard.Public.RewriteUrls = boolPtr(true) }},
		{rule: "10", name: "managed protected card with rewriteUrls", contains: "protected.rewriteUrls field applies to passthrough cards only",
			mutate: func(r *api.A2AAgentProxy) {
				content := completeAgentCardContent()
				mode := api.ProtectedAgentCardMode(api.ManagedProtectedAgentCardModeManaged)
				r.A2a.AgentCard.Protected = &api.ProtectedAgentCard{Mode: &mode, Content: &content, RewriteUrls: boolPtr(true)}
			}},
		{rule: "10", name: "passthrough protected card carrying content", contains: "protected.content field is not allowed in passthrough mode",
			mutate: func(r *api.A2AAgentProxy) {
				content := completeAgentCardContent()
				r.A2a.AgentCard.Protected.Content = &content
			}},
		// Rule 11 — encoded card size.
		{rule: "11", name: "card content over 1 MiB", contains: "exceeds the maximum Agent Card size",
			mutate: func(r *api.A2AAgentProxy) {
				content := completeAgentCardContent()
				content["x-oversized"] = strings.Repeat("a", 1<<20)
				r.A2a.AgentCard.Public.Content = &content
			}},
		// Rule 12 — the keys a managed card cannot be read without.
		{rule: "12", name: "managed card missing required keys", contains: `missing required Agent Card fields: "skills"`,
			mutate: func(r *api.A2AAgentProxy) {
				content := completeAgentCardContent()
				delete(content, "skills")
				r.A2a.AgentCard.Public.Content = &content
			}},
		{rule: "12", name: "managed card that is an empty document", contains: "missing required Agent Card fields",
			mutate: func(r *api.A2AAgentProxy) {
				content := api.AgentCardDocument{}
				r.A2a.AgentCard.Public.Content = &content
			}},
		// Rule 14 — the protected card's explicit mode.
		{rule: "14", name: "protected card with no mode", contains: "protected.mode field is required",
			mutate: func(r *api.A2AAgentProxy) { r.A2a.AgentCard.Protected.Mode = nil }},
		{rule: "14", name: "unsupported protected card mode", contains: `protected.mode "cached" is not supported`,
			mutate: func(r *api.A2AAgentProxy) {
				mode := api.ProtectedAgentCardMode("cached")
				r.A2a.AgentCard.Protected.Mode = &mode
			}},
	}

	for _, tc := range tests {
		t.Run("rule"+tc.rule+"/"+tc.name, func(t *testing.T) {
			req := fullAgentProxyValidationRequest()
			tc.mutate(req)

			err := validateAgentProxyRequest(req)
			if err == nil {
				t.Fatalf("expected a rejection, got none")
			}

			// Every contract failure is the same catalog entry and the same
			// status: a client can tell one rule from another by reading the
			// message, never by branching on the code.
			var appErr *apperror.Error
			if !apperror.ValidationFailed.Is(err) || !errorsAs(err, &appErr) {
				t.Fatalf("expected apperror.ValidationFailed, got: %v", err)
			}
			if appErr.HTTPStatus != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", appErr.HTTPStatus, http.StatusBadRequest)
			}
			if !strings.Contains(appErr.Message, tc.contains) {
				t.Fatalf("message %q does not contain %q", appErr.Message, tc.contains)
			}

			// error-handling.md: the client-facing message carries no internal
			// detail, and no stored or supplied credential.
			for _, leak := range []string{"Go struct", "json:", "api.", "model.", "internal/", ".go:", "weather-upstream"} {
				if strings.Contains(appErr.Message, leak) {
					t.Fatalf("message %q leaks an internal detail (%q)", appErr.Message, leak)
				}
			}
		})
	}
}

// TestValidateAgentProxyRequestDefersToTheGateway pins the boundary in the
// other direction: these bodies are wrong, but wrong in ways only the gateway
// can know about, so the control plane stores them and the failure surfaces at
// deployment rather than at create.
//
// Duplicating the gateway's rules here would make an Agent proxy unauthorable
// against a gateway that does accept it, and would drift the moment the
// gateway's own validation changes.
func TestValidateAgentProxyRequestDefersToTheGateway(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*api.A2AAgentProxy)
	}{
		{
			// Whether a policy of that name and version exists is resolved
			// against the target gateway's own policy catalog.
			name: "policy that may not exist on the gateway",
			mutate: func(r *api.A2AAgentProxy) {
				r.A2a.OperationConfigs.Policies = &[]api.Policy{{Name: "no-such-policy", Version: "v9"}}
			},
		},
		{
			// Policy parameters are free-form by contract; their schema belongs
			// to the policy definition, which lives on the gateway.
			name: "policy parameters the policy definition would reject",
			mutate: func(r *api.A2AAgentProxy) {
				params := map[string]any{"quota": "not-a-number"}
				r.A2a.OperationConfigs.Policies = &[]api.Policy{{Name: "advanced-ratelimit", Version: "v1", Params: &params}}
			},
		},
		{
			// A card advertising the wrong host is gateway L9: the control
			// plane has no gateway external URL to compare it against.
			name: "card advertising an interface on another host",
			mutate: func(r *api.A2AAgentProxy) {
				content := completeAgentCardContent()
				content["supportedInterfaces"] = []any{
					map[string]any{"protocolBinding": "JSONRPC", "url": "https://somewhere-else.example.com/rpc", "protocolVersion": "1.0"},
				}
				r.A2a.AgentCard.Public.Content = &content
			},
		},
		{
			// Card-versus-policy security agreement is the gateway's Section 5,
			// and is absent there too (CP-L1) — so nothing objects today. The
			// control plane storing it is deliberate, not an oversight.
			name: "card security requirements contradicting the policy chain",
			mutate: func(r *api.A2AAgentProxy) {
				content := completeAgentCardContent()
				content["securityRequirements"] = []any{
					map[string]any{"schemes": map[string]any{"never-attached": map[string]any{"list": []any{"a2a.invoke"}}}},
				}
				r.A2a.AgentCard.Public.Content = &content
				r.A2a.OperationConfigs.Policies = nil
			},
		},
		{
			// A card path that collides with a transport route is a route
			// collision, which only the assembled route table can detect.
			name: "card path colliding with a transport prefix",
			mutate: func(r *api.A2AAgentProxy) {
				r.A2a.Transports = []api.A2ATransport{{ProtocolBinding: api.JSONRPC, PathPrefix: strPtr("/rpc")}}
				r.A2a.AgentCard.Public.Path = strPtr("/rpc")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := fullAgentProxyValidationRequest()
			tc.mutate(req)
			if err := validateAgentProxyRequest(req); err != nil {
				t.Fatalf("the control plane must accept this and let deployment reject it, got: %v", err)
			}
		})
	}
}

// TestManagedAgentCardContentIsNotRewritten guards the one thing validation
// must not do to a managed card: change it. The document is stored and served
// byte-for-byte because re-serializing it would change the bytes a future
// signing implementation signs, so the size check must read it without
// normalizing, reordering or dropping anything.
func TestManagedAgentCardContentIsNotRewritten(t *testing.T) {
	req := fullAgentProxyValidationRequest()
	before := *req.A2a.AgentCard.Public.Content

	if err := validateAgentProxyRequest(req); err != nil {
		t.Fatalf("validate: %v", err)
	}

	after := *req.A2a.AgentCard.Public.Content
	if len(after) != len(before) {
		t.Fatalf("card content gained or lost keys: %d -> %d", len(before), len(after))
	}
	ext, ok := after["x-vendor-custom"].(map[string]any)
	if !ok || ext["kept"] != true {
		t.Fatalf("free-form card extension not preserved: %#v", after)
	}
}

// errorsAs is a local alias kept to one line so the assertion above reads as
// one thought rather than two.
func errorsAs(err error, target **apperror.Error) bool {
	e, ok := err.(*apperror.Error)
	if ok {
		*target = e
	}
	return ok
}

// TestSanitizeAgentProxySecretRefErrorHidesHandles covers the two shapes the
// shared secret validator returns, and the one thing neither may carry into a
// response or a log line: the handle.
//
// A handle names a tenant resource, and the validator reports exactly which
// ones did not resolve — so forwarding it unchanged turns Agent proxy creation
// into a secret-existence oracle for a caller with no right to read secrets.
func TestSanitizeAgentProxySecretRefErrorHidesHandles(t *testing.T) {
	const handle = "weather-upstream"

	t.Run("missing secrets", func(t *testing.T) {
		// The shape SecretService.ValidateSecretRefs returns for a handle that
		// does not resolve.
		source := apperror.ValidationFailed.New(
			"The following referenced secrets do not exist: " + handle + ".")

		err := sanitizeAgentProxySecretRefError(source)

		var appErr *apperror.Error
		if !errorsAs(err, &appErr) {
			t.Fatalf("expected a catalog error, got: %v", err)
		}
		if appErr.HTTPStatus != http.StatusBadRequest || appErr.Code != apperror.ValidationFailed.Code {
			t.Fatalf("expected a 400 VALIDATION_FAILED, got %d %s", appErr.HTTPStatus, appErr.Code)
		}
		assertNoHandleAnywhere(t, appErr, handle)
	})

	t.Run("lookup failure", func(t *testing.T) {
		// The validator wraps its repository failure with the handle; the
		// repository itself writes the cause without one, which is why only the
		// unwrapped cause is carried through.
		source := fmt.Errorf("failed to check existence of secret %q: %w",
			handle, errors.New("failed to check secret existence: connection refused"))

		err := sanitizeAgentProxySecretRefError(source)

		var appErr *apperror.Error
		if !errorsAs(err, &appErr) {
			t.Fatalf("expected a catalog error, got: %v", err)
		}
		if appErr.HTTPStatus != http.StatusInternalServerError {
			t.Fatalf("a lookup failure is not the caller's fault; status = %d", appErr.HTTPStatus)
		}
		if appErr.Cause == nil || !strings.Contains(appErr.Cause.Error(), "connection refused") {
			t.Fatalf("the handle-free cause must survive for the log line, got: %v", appErr.Cause)
		}
		assertNoHandleAnywhere(t, appErr, handle)
	})

	t.Run("unwrappable failure drops the cause", func(t *testing.T) {
		// Nothing returns this today. If something starts to, the handle must
		// not ride along on the assumption that it was wrapped the usual way.
		err := sanitizeAgentProxySecretRefError(errors.New("secret " + handle + " exploded"))

		var appErr *apperror.Error
		if !errorsAs(err, &appErr) {
			t.Fatalf("expected a catalog error, got: %v", err)
		}
		assertNoHandleAnywhere(t, appErr, handle)
	})
}

// assertNoHandleAnywhere checks every field that reaches either the client or
// the log: the response message, the internal log message and the wrapped cause.
func assertNoHandleAnywhere(t *testing.T, err *apperror.Error, handle string) {
	t.Helper()

	if strings.Contains(err.Message, handle) {
		t.Fatalf("the client message names the secret handle: %q", err.Message)
	}
	if strings.Contains(err.LogMessage, handle) {
		t.Fatalf("the log message names the secret handle: %q", err.LogMessage)
	}
	if err.Cause != nil && strings.Contains(err.Cause.Error(), handle) {
		t.Fatalf("the logged cause names the secret handle: %q", err.Cause.Error())
	}
}
