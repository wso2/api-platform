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

package dto

import (
	"reflect"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// fullAgentProxyRequest exercises every optional block the mapper handles: a
// sandbox upstream addressed by ref, policies carrying a condition and params,
// per-operation resilience, and both card blocks with content.
func fullAgentProxyRequest() *api.A2AAgentProxy {
	authType := api.UpstreamAuthType("api-key")
	publicMode := api.PublicAgentCardModeManaged
	protectedMode := api.ProtectedAgentCardModePassthrough
	publicContent := api.AgentCardDocument{"name": "Weather Agent", "x-vendor": map[string]interface{}{"kept": true}}
	protectedContent := api.AgentCardDocument{"name": "Weather Agent (protected)"}
	params := map[string]interface{}{"requestsPerMinute": float64(10)}

	return &api.A2AAgentProxy{
		DisplayName: "Weather Agent",
		Version:     "v1.0",
		ProjectId:   "default-project",
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{
				Url:  ptr("http://weather-agent:9000"),
				Auth: &api.UpstreamAuth{Type: &authType, Header: ptr("X-API-Key"), Value: ptr(`{{ secret "main" }}`)},
			},
			Sandbox: &api.UpstreamDefinition{Ref: ptr("sandbox-backend")},
		},
		Resilience: &api.Resilience{Timeout: ptr("10s"), IdleTimeout: ptr("5s")},
		Protocol:   api.A2AAgentProxyProtocol(model.AgentProxyProtocolA2A),
		A2a: api.A2AProtocolConfig{
			ProtocolVersion: "1.0",
			OperationConfigs: api.A2AOperationConfigs{
				Transports: []api.A2ATransport{{ProtocolBinding: "JSONRPC", PathPrefix: ptr("/rpc")}},
				Policies: &[]api.Policy{{
					Name:               "advanced-ratelimit",
					Version:            "v1",
					ExecutionCondition: ptr("request.method == 'POST'"),
					Params:             &params,
				}},
				Operations: &[]api.A2AOperation{{
					Name:       "SendMessage",
					Policies:   &[]api.Policy{{Name: "jwt-auth", Version: "v1"}},
					Resilience: &api.Resilience{Timeout: ptr("30s")},
				}},
			},
			AgentCard: &api.AgentCardConfig{
				Public: &api.PublicAgentCard{
					Mode:     &publicMode,
					Path:     ptr(model.DefaultAgentCardPath),
					Policies: &[]api.Policy{{Name: "cors", Version: "v1"}},
					Content:  &publicContent,
				},
				Protected: &api.ProtectedAgentCard{
					Mode:        &protectedMode,
					RewriteUrls: ptr(false),
					Content:     &protectedContent,
				},
			},
		},
	}
}

func TestAgentProxyConfigurationFromRequestNilRequest(t *testing.T) {
	got := AgentProxyConfigurationFromRequest(nil)
	if !reflect.DeepEqual(got, model.AgentProxyConfiguration{}) {
		t.Fatalf("nil request mapped to %+v, want the zero configuration", got)
	}
}

func TestAgentProxyConfigurationFromRequestMapsEveryOptionalBlock(t *testing.T) {
	req := fullAgentProxyRequest()
	cfg := AgentProxyConfigurationFromRequest(req)

	if cfg.Resilience == nil || cfg.Resilience.Timeout != "10s" || cfg.Resilience.IdleTimeout != "5s" {
		t.Fatalf("resilience = %+v", cfg.Resilience)
	}
	if cfg.Upstream.Sandbox == nil || cfg.Upstream.Sandbox.Ref != "sandbox-backend" || cfg.Upstream.Sandbox.URL != "" {
		t.Fatalf("sandbox upstream = %+v", cfg.Upstream.Sandbox)
	}
	if cfg.Upstream.Sandbox.Auth != nil {
		t.Fatal("sandbox upstream materialized an auth block the request omitted")
	}

	ops := cfg.A2A.OperationConfigs
	if len(ops.Policies) != 1 {
		t.Fatalf("common policies = %+v", ops.Policies)
	}
	common := ops.Policies[0]
	if common.ExecutionCondition == nil || *common.ExecutionCondition != "request.method == 'POST'" {
		t.Fatalf("executionCondition = %v", common.ExecutionCondition)
	}
	if common.Params == nil || (*common.Params)["requestsPerMinute"] != float64(10) {
		t.Fatalf("params = %v", common.Params)
	}
	// The mapped condition must be a copy, never an alias of request memory.
	*(*req.A2a.OperationConfigs.Policies)[0].ExecutionCondition = "mutated"
	if *common.ExecutionCondition == "mutated" {
		t.Fatal("executionCondition aliases the request")
	}

	if len(ops.Operations) != 1 || ops.Operations[0].Name != "SendMessage" {
		t.Fatalf("operations = %+v", ops.Operations)
	}
	if r := ops.Operations[0].Resilience; r == nil || r.Timeout != "30s" || r.IdleTimeout != "" {
		t.Fatalf("operation resilience = %+v", r)
	}

	card := cfg.A2A.AgentCard
	if card == nil || card.Public == nil || card.Protected == nil {
		t.Fatalf("agent card = %+v", card)
	}
	if card.Public.Mode == nil || *card.Public.Mode != model.AgentCardModeManaged {
		t.Fatalf("public mode = %v", card.Public.Mode)
	}
	if card.Public.RewriteUrls != nil {
		t.Fatal("managed public card gained a rewriteUrls the request omitted")
	}
	if card.Public.Content["x-vendor"] == nil || len(card.Public.Policies) != 1 {
		t.Fatalf("public card = %+v", card.Public)
	}
	if card.Protected.Mode != model.AgentCardModePassthrough {
		t.Fatalf("protected mode = %q", card.Protected.Mode)
	}
	if card.Protected.RewriteUrls == nil || *card.Protected.RewriteUrls {
		t.Fatalf("protected rewriteUrls = %v, want explicit false", card.Protected.RewriteUrls)
	}
	if card.Protected.Content["name"] != "Weather Agent (protected)" {
		t.Fatalf("protected content = %+v", card.Protected.Content)
	}
}

func TestAgentProxyConfigurationFromRequestEmptyCardBlocks(t *testing.T) {
	var nilDoc api.AgentCardDocument
	req := fullAgentProxyRequest()
	req.A2a.AgentCard = &api.AgentCardConfig{
		// A present-but-null content document stays absent rather than becoming {}.
		Public: &api.PublicAgentCard{Content: &nilDoc},
		// A protected block without a mode stores the empty string, not a panic.
		Protected: &api.ProtectedAgentCard{},
	}
	req.A2a.OperationConfigs.Operations = &[]api.A2AOperation{}

	cfg := AgentProxyConfigurationFromRequest(req)
	card := cfg.A2A.AgentCard
	if card.Public == nil || card.Public.Mode != nil || card.Public.Path != nil || card.Public.Policies != nil || card.Public.Content != nil {
		t.Fatalf("empty public card = %+v", card.Public)
	}
	if card.Protected == nil || card.Protected.Mode != "" || card.Protected.Content != nil || card.Protected.RewriteUrls != nil {
		t.Fatalf("empty protected card = %+v", card.Protected)
	}
	if ops := cfg.A2A.OperationConfigs.Operations; ops == nil || len(ops) != 0 {
		t.Fatalf("explicit empty operations = %#v, want a non-nil empty slice", ops)
	}
}

func TestAgentProxyFullRequestRoundTripsToResponse(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	m := &model.AgentProxy{
		Handle:        "weather-agent",
		Name:          "Weather Agent",
		Version:       "v1.0",
		Protocol:      model.AgentProxyProtocolA2A,
		CreatedAt:     created,
		UpdatedAt:     created.Add(time.Hour),
		Configuration: AgentProxyConfigurationFromRequest(fullAgentProxyRequest()),
	}
	resp := AgentProxyToResponse(m, "default-project", nil)

	if resp.Resilience == nil || resp.Resilience.Timeout == nil || *resp.Resilience.Timeout != "10s" {
		t.Fatalf("resilience = %+v", resp.Resilience)
	}
	if resp.Upstream.Sandbox == nil || resp.Upstream.Sandbox.Ref == nil || *resp.Upstream.Sandbox.Ref != "sandbox-backend" {
		t.Fatalf("sandbox = %+v", resp.Upstream.Sandbox)
	}
	if resp.Upstream.Sandbox.Url != nil {
		t.Fatal("sandbox rendered an empty url instead of omitting it")
	}
	if a := resp.Upstream.Main.Auth; a == nil || a.Value != nil || a.Type == nil || *a.Type != "api-key" {
		t.Fatalf("main auth = %+v, want type kept and value redacted", a)
	}

	ops := resp.A2a.OperationConfigs
	if ops.Policies == nil || len(*ops.Policies) != 1 {
		t.Fatalf("common policies = %v", ops.Policies)
	}
	p := (*ops.Policies)[0]
	if p.ExecutionCondition == nil || p.Params == nil || (*p.Params)["requestsPerMinute"] != float64(10) {
		t.Fatalf("common policy = %+v", p)
	}
	if ops.Operations == nil || len(*ops.Operations) != 1 {
		t.Fatalf("operations = %v", ops.Operations)
	}
	op := (*ops.Operations)[0]
	if op.Name != "SendMessage" || op.Policies == nil || op.Resilience == nil || *op.Resilience.Timeout != "30s" {
		t.Fatalf("operation = %+v", op)
	}
	if op.Resilience.IdleTimeout != nil {
		t.Fatal("operation resilience rendered an empty idleTimeout")
	}

	card := resp.A2a.AgentCard
	if card == nil || card.Public == nil || card.Protected == nil {
		t.Fatalf("agent card = %+v", card)
	}
	if card.Public.Mode == nil || *card.Public.Mode != api.PublicAgentCardModeManaged {
		t.Fatalf("public mode = %v", card.Public.Mode)
	}
	if card.Public.Path == nil || *card.Public.Path != model.DefaultAgentCardPath {
		t.Fatalf("public path = %v", card.Public.Path)
	}
	if card.Public.Content == nil || (*card.Public.Content)["x-vendor"] == nil {
		t.Fatalf("public content = %v", card.Public.Content)
	}
	if card.Public.Policies == nil || len(*card.Public.Policies) != 1 {
		t.Fatalf("public policies = %v", card.Public.Policies)
	}
	if card.Protected.Mode == nil || *card.Protected.Mode != api.ProtectedAgentCardModePassthrough {
		t.Fatalf("protected mode = %v", card.Protected.Mode)
	}
	if card.Protected.RewriteUrls == nil || *card.Protected.RewriteUrls {
		t.Fatalf("protected rewriteUrls = %v", card.Protected.RewriteUrls)
	}
	if card.Protected.Content == nil || (*card.Protected.Content)["name"] != "Weather Agent (protected)" {
		t.Fatalf("protected content = %v", card.Protected.Content)
	}
	if resp.CreatedAt == nil || !resp.CreatedAt.Equal(created) || resp.UpdatedAt == nil {
		t.Fatalf("timestamps = %v / %v", resp.CreatedAt, resp.UpdatedAt)
	}
}

func TestAgentProxyToResponseNilAndSparseModel(t *testing.T) {
	if AgentProxyToResponse(nil, "p", nil) != nil {
		t.Fatal("nil model rendered a response")
	}

	// A model with no A2A block and no timestamps must render without panicking
	// and leave every optional block absent.
	resp := AgentProxyToResponse(&model.AgentProxy{Handle: "h", Protocol: model.AgentProxyProtocolA2A}, "p", nil)
	if resp.Resilience != nil || resp.CreatedAt != nil || resp.UpdatedAt != nil || resp.Description != nil {
		t.Fatalf("sparse response = %+v", resp)
	}
	if resp.A2a.OperationConfigs.Transports != nil || resp.A2a.AgentCard != nil {
		t.Fatalf("absent a2a block rendered as %+v", resp.A2a)
	}

	// A stored A2A block without transports renders an empty array, never a missing key.
	resp = AgentProxyToResponse(&model.AgentProxy{
		Handle:        "h",
		Protocol:      model.AgentProxyProtocolA2A,
		Configuration: model.AgentProxyConfiguration{A2A: &model.A2AProtocolConfig{ProtocolVersion: "1.0"}},
	}, "p", nil)
	if tr := resp.A2a.OperationConfigs.Transports; tr == nil || len(tr) != 0 {
		t.Fatalf("transports = %#v, want a non-nil empty slice", tr)
	}
	if resp.A2a.OperationConfigs.Operations != nil || resp.A2a.OperationConfigs.Policies != nil {
		t.Fatal("absent operations/policies were materialized")
	}
}

func TestAgentProxyToListItemNilAndTimestamps(t *testing.T) {
	if got := AgentProxyToListItem(nil, "p"); !reflect.DeepEqual(got, api.AgentProxyListItem{}) {
		t.Fatalf("nil model rendered %+v", got)
	}

	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	item := AgentProxyToListItem(&model.AgentProxy{
		Handle:    "weather-agent",
		CreatedAt: created,
		UpdatedAt: created.Add(time.Minute),
	}, "p")
	if item.CreatedAt == nil || !item.CreatedAt.Equal(created) {
		t.Fatalf("createdAt = %v", item.CreatedAt)
	}
	if item.UpdatedAt == nil || !item.UpdatedAt.Equal(created.Add(time.Minute)) {
		t.Fatalf("updatedAt = %v", item.UpdatedAt)
	}
}

func TestPreserveAgentProxyUpstreamAuthNilAndSandboxCases(t *testing.T) {
	existing := &model.UpstreamConfig{Main: &model.UpstreamEndpoint{URL: "http://a"}}
	if got := PreserveAgentProxyUpstreamAuth(existing, nil); got != existing {
		t.Fatal("nil update did not return the existing upstream")
	}
	updated := &model.UpstreamConfig{Main: &model.UpstreamEndpoint{URL: "http://a"}}
	if got := PreserveAgentProxyUpstreamAuth(nil, updated); got != updated {
		t.Fatal("nil existing did not return the updated upstream")
	}

	tests := []struct {
		name      string
		existing  *model.UpstreamEndpoint
		updated   *model.UpstreamEndpoint
		wantValue string
	}{
		{
			name:      "sandbox credential carried forward on an unchanged config",
			existing:  &model.UpstreamEndpoint{Ref: "sb", Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-Key", Value: "stored"}},
			updated:   &model.UpstreamEndpoint{Ref: "sb", Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-Key"}},
			wantValue: "stored",
		},
		{
			name:      "sandbox ref change refuses inheritance",
			existing:  &model.UpstreamEndpoint{Ref: "sb", Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-Key", Value: "stored"}},
			updated:   &model.UpstreamEndpoint{Ref: "other", Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-Key"}},
			wantValue: "",
		},
		{
			name:      "explicit new value wins over the stored one",
			existing:  &model.UpstreamEndpoint{Ref: "sb", Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-Key", Value: "stored"}},
			updated:   &model.UpstreamEndpoint{Ref: "sb", Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-Key", Value: "fresh"}},
			wantValue: "fresh",
		},
		{
			name:      "type change refuses inheritance",
			existing:  &model.UpstreamEndpoint{Ref: "sb", Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-Key", Value: "stored"}},
			updated:   &model.UpstreamEndpoint{Ref: "sb", Auth: &model.UpstreamAuth{Type: "bearer", Header: "X-Key"}},
			wantValue: "",
		},
		{
			name:      "existing without auth leaves the update untouched",
			existing:  &model.UpstreamEndpoint{Ref: "sb"},
			updated:   &model.UpstreamEndpoint{Ref: "sb", Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-Key"}},
			wantValue: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PreserveAgentProxyUpstreamAuth(
				&model.UpstreamConfig{Sandbox: tc.existing},
				&model.UpstreamConfig{Sandbox: tc.updated},
			)
			if got.Sandbox.Auth.Value != tc.wantValue {
				t.Fatalf("sandbox auth value = %q, want %q", got.Sandbox.Auth.Value, tc.wantValue)
			}
		})
	}

	// An update that drops the sandbox endpoint entirely must not panic or revive it.
	got := PreserveAgentProxyUpstreamAuth(
		&model.UpstreamConfig{Sandbox: &model.UpstreamEndpoint{Ref: "sb", Auth: &model.UpstreamAuth{Value: "stored"}}},
		&model.UpstreamConfig{},
	)
	if got.Sandbox != nil {
		t.Fatal("dropped sandbox endpoint was revived")
	}
}

func TestAgentProxyMappingHelpers(t *testing.T) {
	t.Run("resilienceToAPI", func(t *testing.T) {
		if resilienceToAPI(nil) != nil {
			t.Fatal("nil resilience rendered a block")
		}
		got := resilienceToAPI(&model.Resilience{})
		if got == nil || got.Timeout != nil || got.IdleTimeout != nil {
			t.Fatalf("empty resilience = %+v, want a block with both fields omitted", got)
		}
	})

	t.Run("a2aConfigToModel and a2aConfigToAPI nil", func(t *testing.T) {
		if a2aConfigToModel(nil) != nil {
			t.Fatal("nil request a2a mapped to a block")
		}
		if a2aConfigToAPI(nil) != nil {
			t.Fatal("nil model a2a rendered a block")
		}
	})

	t.Run("agentCardToAPI", func(t *testing.T) {
		if agentCardToAPI(nil) != nil {
			t.Fatal("nil card rendered a block")
		}
		got := agentCardToAPI(&model.AgentCardConfig{})
		if got == nil || got.Public != nil || got.Protected != nil {
			t.Fatalf("empty card = %+v", got)
		}
	})

	t.Run("policiesToModel and policiesToAPI", func(t *testing.T) {
		if policiesToModel(nil) != nil {
			t.Fatal("nil request policies mapped to a slice")
		}
		if got := policiesToModel(&[]api.Policy{}); got == nil || len(got) != 0 {
			t.Fatalf("explicit empty policies = %#v, want a non-nil empty slice", got)
		}
		if policiesToAPI(nil) != nil {
			t.Fatal("nil model policies rendered a slice")
		}
		got := policiesToAPI([]model.Policy{{Name: "cors", Version: "v1"}})
		if got == nil || len(*got) != 1 || (*got)[0].ExecutionCondition != nil || (*got)[0].Params != nil {
			t.Fatalf("plain policy = %+v", got)
		}
	})

	t.Run("cardDocumentToAPI", func(t *testing.T) {
		if cardDocumentToAPI(nil) != nil {
			t.Fatal("nil document rendered a value")
		}
		got := cardDocumentToAPI(model.AgentCardDocument{"k": "v"})
		if got == nil || (*got)["k"] != "v" {
			t.Fatalf("document = %v", got)
		}
	})

	t.Run("card modes", func(t *testing.T) {
		if cardModeToModel[api.PublicAgentCardMode](nil) != nil {
			t.Fatal("nil mode mapped to a value")
		}
		mode := api.PublicAgentCardModeManaged
		if got := cardModeToModel(&mode); got == nil || *got != "managed" {
			t.Fatalf("mode = %v", got)
		}
		if publicCardModeToAPI(nil) != nil {
			t.Fatal("nil public mode rendered a value")
		}
		if got := publicCardModeToAPI(ptr("passthrough")); got == nil || *got != api.PublicAgentCardModePassthrough {
			t.Fatalf("public mode = %v", got)
		}
		if got := protectedCardModeToAPI(""); got == nil || *got != "" {
			t.Fatalf("protected mode = %v, want an explicit empty value", got)
		}
	})

	t.Run("derefString", func(t *testing.T) {
		if derefString(nil) != "" || derefString(ptr("x")) != "x" {
			t.Fatal("derefString mismatch")
		}
	})

	t.Run("upstreamToModel without sandbox", func(t *testing.T) {
		got := upstreamToModel(api.Upstream{Main: api.UpstreamDefinition{Url: ptr("http://a")}})
		if got.Sandbox != nil || got.Main == nil || got.Main.URL != "http://a" {
			t.Fatalf("upstream = %+v", got)
		}
	})
}
