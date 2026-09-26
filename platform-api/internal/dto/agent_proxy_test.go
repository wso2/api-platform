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
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

func ptr[T any](v T) *T { return &v }

func upstreamWithAuth(url, authType, header, value string) api.Upstream {
	t := api.UpstreamAuthType(authType)
	return api.Upstream{
		Main: api.UpstreamDefinition{
			Url:  ptr(url),
			Auth: &api.UpstreamAuth{Type: &t, Header: ptr(header), Value: ptr(value)},
		},
	}
}

func TestAgentProxyConfigurationFromRequestMapsTypedBlocks(t *testing.T) {
	req := &api.A2AAgentProxy{
		DisplayName: "Weather Agent",
		Version:     "v1.0",
		ProjectId:   "default-project",
		Context:     ptr("/weather"),
		Vhost:       ptr("agents.gw.com"),
		Upstream:    upstreamWithAuth("http://weather-agent:9000", "api-key", "X-API-Key", `{{ secret "weather-upstream" }}`),
		Resilience:  &api.Resilience{IdleTimeout: ptr("5s")},
		Protocol:    api.A2AAgentProxyProtocol(model.AgentProxyProtocolA2A),
		A2a: api.A2AProtocolConfig{
			ProtocolVersion: "1.0",
			OperationConfigs: api.A2AOperationConfigs{
				Transports: []api.A2ATransport{
					{ProtocolBinding: "JSONRPC", PathPrefix: ptr("/rpc")},
					{ProtocolBinding: "HTTP+JSON"},
				},
				Policies: &[]api.Policy{{Name: "jwt-auth", Version: "v1"}},
				Operations: &[]api.A2AOperation{{
					Name:       "SendMessage",
					Policies:   &[]api.Policy{{Name: "advanced-ratelimit", Version: "v1"}},
					Resilience: &api.Resilience{Timeout: ptr("30s")},
				}},
			},
			AgentCard: &api.AgentCardConfig{
				Public: &api.PublicAgentCard{
					Mode:    ptr(api.PublicAgentCardMode(model.AgentCardModeManaged)),
					Path:    ptr(model.DefaultAgentCardPath),
					Content: ptr(api.AgentCardDocument{"name": "Weather Agent", "x-vendor": "kept"}),
				},
				Protected: &api.ProtectedAgentCard{
					Mode:        ptr(api.ProtectedAgentCardMode(model.AgentCardModePassthrough)),
					RewriteUrls: ptr(false),
				},
			},
		},
	}

	cfg := AgentProxyConfigurationFromRequest(req)

	if cfg.Context == nil || *cfg.Context != "/weather" {
		t.Fatalf("context = %v", cfg.Context)
	}
	if cfg.Upstream.Main == nil || cfg.Upstream.Main.Auth == nil {
		t.Fatal("upstream auth was not mapped")
	}
	if cfg.Upstream.Main.Auth.Value != `{{ secret "weather-upstream" }}` {
		t.Fatalf("secret placeholder = %q", cfg.Upstream.Main.Auth.Value)
	}
	if cfg.A2A == nil || cfg.A2A.ProtocolVersion != "1.0" {
		t.Fatal("a2a block was not mapped")
	}
	transports := cfg.A2A.OperationConfigs.Transports
	if len(transports) != 2 {
		t.Fatalf("transports = %d, want 2", len(transports))
	}
	// An omitted pathPrefix stays omitted rather than becoming an explicit "/".
	if transports[1].PathPrefix != nil {
		t.Fatalf("omitted pathPrefix became %q", *transports[1].PathPrefix)
	}
	if len(cfg.A2A.OperationConfigs.Policies) != 1 || len(cfg.A2A.OperationConfigs.Operations) != 1 {
		t.Fatal("operation configuration was not mapped")
	}
	if cfg.A2A.AgentCard.Public.Content["x-vendor"] != "kept" {
		t.Fatal("free-form card extension was dropped")
	}
	// rewriteUrls is passthrough-only; a managed card must carry no pointer at all.
	if cfg.A2A.AgentCard.Public.RewriteUrls != nil {
		t.Fatal("managed public card gained a rewriteUrls value")
	}
	if cfg.A2A.AgentCard.Protected.RewriteUrls == nil || *cfg.A2A.AgentCard.Protected.RewriteUrls {
		t.Fatal("explicit protected rewriteUrls:false was not preserved")
	}

	// The serialized document is what reaches the column: it must carry the a2a
	// block and no discriminator or column-backed metadata.
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal configuration: %v", err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatalf("unmarshal configuration: %v", err)
	}
	if _, ok := document["a2a"]; !ok {
		t.Fatal("configuration document is missing the a2a block")
	}
	// The stored a2a block has the gateway's layout: transports sit under
	// operationConfigs.
	var a2aDocument map[string]json.RawMessage
	if err := json.Unmarshal(document["a2a"], &a2aDocument); err != nil {
		t.Fatalf("unmarshal a2a block: %v", err)
	}
	var operationConfigsDocument map[string]json.RawMessage
	if err := json.Unmarshal(a2aDocument["operationConfigs"], &operationConfigsDocument); err != nil {
		t.Fatalf("unmarshal a2a.operationConfigs: %v", err)
	}
	if _, ok := operationConfigsDocument["transports"]; !ok {
		t.Fatal("configuration document is missing a2a.operationConfigs.transports")
	}
	for _, forbidden := range []string{"protocol", "displayName", "version", "projectId", "specVersion", "kind", "id"} {
		if _, ok := document[forbidden]; ok {
			t.Errorf("configuration document carries %q", forbidden)
		}
	}
}

func TestAgentProxyConfigurationFromRequestKeepsOmittedBlocksOmitted(t *testing.T) {
	req := &api.A2AAgentProxy{
		DisplayName: "Minimal Agent",
		Version:     "v1.0",
		ProjectId:   "default-project",
		Upstream:    api.Upstream{Main: api.UpstreamDefinition{Url: ptr("http://agent:9000")}},
		Protocol:    api.A2AAgentProxyProtocol(model.AgentProxyProtocolA2A),
		A2a: api.A2AProtocolConfig{
			ProtocolVersion: "1.0",
			OperationConfigs: api.A2AOperationConfigs{
				Transports: []api.A2ATransport{{ProtocolBinding: "JSONRPC"}},
			},
		},
	}

	cfg := AgentProxyConfigurationFromRequest(req)
	if cfg.Context != nil || cfg.Vhost != nil || cfg.Resilience != nil {
		t.Fatal("omitted top-level fields were materialized")
	}
	if cfg.A2A.AgentCard != nil {
		t.Fatal("omitted agentCard block was materialized")
	}
	if cfg.A2A.OperationConfigs.Policies != nil || cfg.A2A.OperationConfigs.Operations != nil {
		t.Fatal("omitted operationConfigs policies/operations were materialized")
	}
	if cfg.Upstream.Main == nil || cfg.Upstream.Main.Auth != nil {
		t.Fatal("upstream auth was invented for a request that carried none")
	}
}

func TestAgentProxyToResponseRedactsCredentialsAndReadsProtocolFromColumn(t *testing.T) {
	created := time.Date(2026, 1, 31, 10, 30, 0, 0, time.UTC)
	m := &model.AgentProxy{
		UUID:             "00000000-0000-0000-0000-000000000001",
		Handle:           "weather-agent",
		OrganizationUUID: "org-1",
		ProjectUUID:      "project-uuid-not-a-handle",
		Name:             "Weather Agent",
		Description:      "Provides forecasts",
		Protocol:         model.AgentProxyProtocolA2A,
		Version:          "v1.0",
		CreatedBy:        "john.doe",
		UpdatedBy:        "john.doe",
		CreatedAt:        created,
		UpdatedAt:        created,
		Origin:           constants.OriginCP,
		Configuration: model.AgentProxyConfiguration{
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{
					URL:  "http://weather-agent:9000",
					Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-API-Key", Value: `{{ secret "weather-upstream" }}`},
				},
				Sandbox: &model.UpstreamEndpoint{
					URL:  "http://weather-agent-sandbox:9000",
					Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-API-Key", Value: "plain-text-key"},
				},
			},
			A2A: &model.A2AProtocolConfig{
				ProtocolVersion: "1.0",
				OperationConfigs: model.A2AOperationConfigs{
					Transports: []model.A2ATransport{{ProtocolBinding: "JSONRPC"}},
				},
			},
		},
	}

	resp := AgentProxyToResponse(m, "default-project", nil)
	if resp.Id == nil || *resp.Id != "weather-agent" {
		t.Fatalf("id = %v, want the handle", resp.Id)
	}
	if resp.ProjectId != "default-project" {
		t.Fatalf("projectId = %q, want the resolved handle (never the UUID)", resp.ProjectId)
	}
	if string(resp.Protocol) != string(model.AgentProxyProtocolA2A) {
		t.Fatalf("protocol = %q", resp.Protocol)
	}
	if resp.Kind == nil || string(*resp.Kind) != AgentProxyKind {
		t.Fatalf("kind = %v, want %q", resp.Kind, AgentProxyKind)
	}
	if resp.ReadOnly == nil || *resp.ReadOnly {
		t.Fatalf("readOnly = %v, want explicit false for a control-plane artifact", resp.ReadOnly)
	}
	if resp.Upstream.Main.Auth == nil || resp.Upstream.Main.Auth.Value != nil {
		t.Fatalf("main upstream credential leaked: %+v", resp.Upstream.Main.Auth)
	}
	if resp.Upstream.Sandbox == nil || resp.Upstream.Sandbox.Auth == nil || resp.Upstream.Sandbox.Auth.Value != nil {
		t.Fatalf("sandbox upstream credential leaked: %+v", resp.Upstream.Sandbox)
	}
	if resp.Upstream.Main.Auth.Header == nil || *resp.Upstream.Main.Auth.Header != "X-API-Key" {
		t.Fatal("non-secret auth metadata was redacted along with the value")
	}

	// A response must never carry the internal UUID under any field.
	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	for _, leaked := range []string{m.UUID, m.ProjectUUID, "weather-upstream", "plain-text-key"} {
		if strings.Contains(string(encoded), leaked) {
			t.Errorf("response leaked %q: %s", leaked, encoded)
		}
	}
}

func TestAgentProxyToResponseMarksImportedArtifactsReadOnly(t *testing.T) {
	m := &model.AgentProxy{
		Handle:   "imported-agent",
		Name:     "Imported Agent",
		Version:  "v1.0",
		Protocol: model.AgentProxyProtocolA2A,
		Origin:   constants.OriginDP,
		Configuration: model.AgentProxyConfiguration{
			A2A: &model.A2AProtocolConfig{ProtocolVersion: "1.0"},
		},
	}
	resp := AgentProxyToResponse(m, "default-project", nil)
	if resp.ReadOnly == nil || !*resp.ReadOnly {
		t.Fatalf("readOnly = %v, want true for a gateway-origin artifact", resp.ReadOnly)
	}
	item := AgentProxyToListItem(m, "default-project")
	if item.ReadOnly == nil || !*item.ReadOnly {
		t.Fatalf("list item readOnly = %v, want true", item.ReadOnly)
	}
}

func TestAgentProxyToListItemOmitsProtocolConfiguration(t *testing.T) {
	m := &model.AgentProxy{
		Handle:   "weather-agent",
		Name:     "Weather Agent",
		Version:  "v1.0",
		Protocol: model.AgentProxyProtocolA2A,
		Origin:   constants.OriginCP,
		Configuration: model.AgentProxyConfiguration{
			Context: ptr("/weather"),
			A2A: &model.A2AProtocolConfig{
				ProtocolVersion: "1.0",
				AgentCard: &model.AgentCardConfig{Public: &model.PublicAgentCard{
					Mode:    ptr(model.AgentCardModeManaged),
					Content: model.AgentCardDocument{"name": "Weather Agent"},
				}},
			},
		},
	}

	encoded, err := json.Marshal(AgentProxyToListItem(m, "default-project"))
	if err != nil {
		t.Fatalf("marshal list item: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("unmarshal list item: %v", err)
	}
	// The card alone may be up to 1 MiB, so it must not ride along in a page of 20.
	for _, heavy := range []string{"a2a", "agentCard", "upstream", "resilience", "associatedGateways"} {
		if _, ok := fields[heavy]; ok {
			t.Errorf("list item carries %q", heavy)
		}
	}
	if string(fields["protocol"]) != `"a2a"` {
		t.Errorf("list item protocol = %s", fields["protocol"])
	}
}

func TestPreserveAgentProxyUpstreamAuth(t *testing.T) {
	existing := func() *model.UpstreamConfig {
		return &model.UpstreamConfig{
			Main:    &model.UpstreamEndpoint{URL: "http://agent:9000", Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-API-Key", Value: "stored-secret"}},
			Sandbox: &model.UpstreamEndpoint{URL: "http://sandbox:9000", Auth: &model.UpstreamAuth{Type: "api-key", Value: "stored-sandbox-secret"}},
		}
	}

	t.Run("omitted value inherits the stored credential", func(t *testing.T) {
		updated := &model.UpstreamConfig{
			Main:    &model.UpstreamEndpoint{URL: "http://agent:9000", Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-API-Key"}},
			Sandbox: &model.UpstreamEndpoint{URL: "http://sandbox:9000", Auth: &model.UpstreamAuth{Type: "api-key"}},
		}
		got := PreserveAgentProxyUpstreamAuth(existing(), updated)
		if got.Main.Auth.Value != "stored-secret" {
			t.Fatalf("main value = %q, want the stored credential", got.Main.Auth.Value)
		}
		if got.Sandbox.Auth.Value != "stored-sandbox-secret" {
			t.Fatalf("sandbox value = %q, want the stored credential", got.Sandbox.Auth.Value)
		}
	})

	t.Run("supplied value wins", func(t *testing.T) {
		updated := &model.UpstreamConfig{
			Main: &model.UpstreamEndpoint{Auth: &model.UpstreamAuth{Type: "api-key", Value: "new-secret"}},
		}
		got := PreserveAgentProxyUpstreamAuth(existing(), updated)
		if got.Main.Auth.Value != "new-secret" {
			t.Fatalf("main value = %q, want the supplied credential", got.Main.Auth.Value)
		}
	})

	t.Run("a changed auth type never inherits", func(t *testing.T) {
		updated := &model.UpstreamConfig{
			Main: &model.UpstreamEndpoint{Auth: &model.UpstreamAuth{Type: "oauth2"}},
		}
		got := PreserveAgentProxyUpstreamAuth(existing(), updated)
		if got.Main.Auth.Value != "" {
			t.Fatalf("main value = %q, want no inheritance across an auth type change", got.Main.Auth.Value)
		}
	})

	t.Run("auth type none removes the credential", func(t *testing.T) {
		updated := &model.UpstreamConfig{
			Main: &model.UpstreamEndpoint{Auth: &model.UpstreamAuth{Type: "none"}},
		}
		got := PreserveAgentProxyUpstreamAuth(existing(), updated)
		if got.Main.Auth.Value != "" {
			t.Fatalf("main value = %q, want the credential removed", got.Main.Auth.Value)
		}
	})

	t.Run("dropping the auth block entirely keeps it dropped", func(t *testing.T) {
		updated := &model.UpstreamConfig{Main: &model.UpstreamEndpoint{URL: "http://agent:9000"}}
		got := PreserveAgentProxyUpstreamAuth(existing(), updated)
		if got.Main.Auth != nil {
			t.Fatalf("auth = %+v, want nil", got.Main.Auth)
		}
	})
}

// TestAgentProxyRequestRoundTripsThroughTheModel asserts the request shape, the
// stored model and the response agree — a create payload rendered back must be
// recognisably the same resource, minus the redacted credential.
func TestAgentProxyRequestRoundTripsThroughTheModel(t *testing.T) {
	const payload = `{
	  "displayName": "Weather Agent",
	  "version": "v1.0",
	  "projectId": "default-project",
	  "context": "/weather",
	  "upstream": {"main": {"url": "http://weather-agent:9000"}},
	  "protocol": "a2a",
	  "a2a": {
	    "protocolVersion": "1.0",
	    "operationConfigs": {
	      "transports": [{"protocolBinding": "JSONRPC", "pathPrefix": "/rpc"}]
	    }
	  }
	}`

	var req api.A2AAgentProxy
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if req.Protocol != "a2a" || req.A2a.ProtocolVersion == "" {
		t.Fatalf("request did not decode into the A2A variant: %+v", req)
	}

	m := &model.AgentProxy{
		Handle:        "weather-agent",
		Name:          req.DisplayName,
		Version:       req.Version,
		Protocol:      model.AgentProxyProtocol(req.Protocol),
		Origin:        constants.OriginCP,
		Configuration: AgentProxyConfigurationFromRequest(&req),
	}

	resp := AgentProxyToResponse(m, req.ProjectId, nil)
	if resp.DisplayName != "Weather Agent" || resp.Version != "v1.0" {
		t.Fatalf("response metadata = displayName %q version %q", resp.DisplayName, resp.Version)
	}
	if resp.Context == nil || *resp.Context != "/weather" {
		t.Fatalf("response context = %v", resp.Context)
	}
	if len(resp.A2a.OperationConfigs.Transports) != 1 {
		t.Fatalf("response a2a = %+v", resp.A2a)
	}
	if prefix := resp.A2a.OperationConfigs.Transports[0].PathPrefix; prefix == nil || *prefix != "/rpc" {
		t.Fatalf("response pathPrefix = %v", prefix)
	}
	if resp.A2a.OperationConfigs.Policies != nil || resp.A2a.OperationConfigs.Operations != nil {
		t.Fatal("response materialized operationConfigs policies/operations the request omitted")
	}
	if resp.A2a.AgentCard != nil {
		t.Fatal("response materialized an agentCard block the request omitted")
	}
}

func TestPreserveAgentProxyUpstreamAuthRefusesInheritanceAcrossAChangedHeader(t *testing.T) {
	existing := &model.UpstreamConfig{
		Main: &model.UpstreamEndpoint{
			URL:  "http://agent:9000",
			Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-Old", Value: "stored-secret"},
		},
	}
	// Same type, different header, no value: this is a new auth configuration, so
	// the stored credential must not follow the header it was never issued for.
	updated := &model.UpstreamConfig{
		Main: &model.UpstreamEndpoint{
			URL:  "http://agent:9000",
			Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-New"},
		},
	}

	got := PreserveAgentProxyUpstreamAuth(existing, updated)
	if got.Main.Auth.Value != "" {
		t.Fatalf("value = %q, want no inheritance across a changed header", got.Main.Auth.Value)
	}
}

func TestFetchAgentCardRequestDistinguishesOmittedFromExplicitNull(t *testing.T) {
	tests := []struct {
		name                           string
		body                           string
		wantURL, wantAuth, wantProxyID bool
		wantUnknown                    []string
	}{
		{
			name:        "stored form",
			body:        `{"agentProxyId":"weather-agent"}`,
			wantProxyID: true,
		},
		{
			// The forbidden shape: it must not decode into the same state as the
			// valid stored form above, or validation cannot tell them apart.
			name:        "stored form with explicit nulls",
			body:        `{"agentProxyId":"weather-agent","url":null,"auth":null}`,
			wantURL:     true,
			wantAuth:    true,
			wantProxyID: true,
		},
		{
			name:    "direct form",
			body:    `{"url":"http://weather-agent:9000"}`,
			wantURL: true,
		},
		{
			name:     "direct form with auth",
			body:     `{"url":"http://weather-agent:9000","auth":{"type":"api-key","header":"X-API-Key","value":"preview"}}`,
			wantURL:  true,
			wantAuth: true,
		},
		{
			name: "empty object",
			body: `{}`,
		},
		{
			name:        "unknown property",
			body:        `{"agentProxyId":"weather-agent","agentId":"legacy"}`,
			wantProxyID: true,
			wantUnknown: []string{"agentId"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var req FetchAgentCardRequest
			if err := json.Unmarshal([]byte(tc.body), &req); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if req.HasURL() != tc.wantURL {
				t.Errorf("HasURL() = %v, want %v", req.HasURL(), tc.wantURL)
			}
			if req.HasAuth() != tc.wantAuth {
				t.Errorf("HasAuth() = %v, want %v", req.HasAuth(), tc.wantAuth)
			}
			if req.HasAgentProxyID() != tc.wantProxyID {
				t.Errorf("HasAgentProxyID() = %v, want %v", req.HasAgentProxyID(), tc.wantProxyID)
			}
			if got := req.UnknownFields(); !equalStrings(got, tc.wantUnknown) {
				t.Errorf("UnknownFields() = %v, want %v", got, tc.wantUnknown)
			}
		})
	}
}

func TestFetchAgentCardRequestStillDecodesValues(t *testing.T) {
	var req FetchAgentCardRequest
	body := `{"url":"http://weather-agent:9000","auth":{"type":"api-key","header":"X-API-Key","value":"preview-key"}}`
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.Url == nil || *req.Url != "http://weather-agent:9000" {
		t.Fatalf("url = %v", req.Url)
	}
	if req.Auth == nil || req.Auth.Value == nil || *req.Auth.Value != "preview-key" {
		t.Fatalf("auth = %+v", req.Auth)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
