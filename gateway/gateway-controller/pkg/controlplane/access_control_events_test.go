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

package controlplane

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/upstreamguard"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// A deployment the gateway's network access control refuses is acked to the
// control plane as UPSTREAM_URL_NOT_ALLOWED, for every artifact kind the
// control plane deploys from YAML, so the publisher sees why it failed. Other
// failures keep GATEWAY_PROCESSING_ERROR.

const acEvtDeploymentID = "dep-ac-1"

var acEvtArtifacts = []struct {
	kind      string // path segment the artifact is fetched from
	eventType string
	idField   string
	yaml      string
	handle    func(*Client, map[string]any)
}{
	{
		kind: "graphql-apis", eventType: "graphqlapi.deployed", idField: "apiId",
		yaml: `apiVersion: gateway.api-platform.wso2.com/v1
kind: GraphQLApi
metadata:
  name: ac-graphql
spec:
  displayName: AC GraphQL
  version: v1.0
  context: /ac-graphql
  upstream:
    main:
      url: http://169.254.169.254/graphql
`,
		handle: (*Client).handleGraphQLAPIDeployedEvent,
	},
	{
		kind: "llm-providers", eventType: "llmprovider.deployed", idField: "providerId",
		yaml: `apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProvider
metadata:
  name: ac-provider
spec:
  displayName: AC Provider
  version: v1.0
  template: openai
  upstream:
    url: http://169.254.169.254/v1
`,
		handle: (*Client).handleLLMProviderDeployedEvent,
	},
	{
		kind: "llm-proxies", eventType: "llmproxy.deployed", idField: "proxyId",
		yaml: `apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProxy
metadata:
  name: ac-proxy
spec:
  displayName: AC Proxy
  version: v1.0
  provider:
    id: ac-provider
`,
		handle: (*Client).handleLLMProxyDeployedEvent,
	},
	{
		kind: "mcp-proxies", eventType: "mcpproxy.deployed", idField: "proxyId",
		yaml: `apiVersion: gateway.api-platform.wso2.com/v1
kind: Mcp
metadata:
  name: ac-mcp
spec:
  displayName: AC MCP
  version: v1.0
  context: /ac-mcp
  upstream:
    url: http://169.254.169.254/mcp
`,
		handle: (*Client).handleMCPProxyDeploymentEvent,
	},
}

// newAccessControlEventsHarness extends the agent events harness (a real SQLite
// store, a control plane serving artifacts and receiving acks) with the
// deployment services every artifact kind needs, and serves each kind's
// artifact under its own path.
func newAccessControlEventsHarness(t *testing.T) *agentEventsHarness {
	t.Helper()
	h := newAgentEventsHarness(t)

	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiService := utils.NewAPIDeploymentService(h.client.store, h.db, nil, nil, routerConfig, h.hub,
		agentEvtGatewayID, agentEvtSecretResolver{}, nil)
	h.client.deploymentService = apiService
	h.client.llmDeploymentService = utils.NewLLMDeploymentService(h.client.store, h.db, nil, nil, nil,
		apiService, routerConfig, nil, nil)
	h.client.mcpDeploymentService = utils.NewMCPDeploymentService(h.client.store, h.db, nil, nil, nil,
		h.hub, agentEvtGatewayID, agentEvtSecretResolver{}, nil)

	// Point the API utils service at a control plane serving every kind.
	mux := http.NewServeMux()
	for _, a := range acEvtArtifacts {
		body := agentEvtZip(t, a.kind, a.yaml)
		mux.HandleFunc("/api/internal/v1/"+a.kind+"/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(body)
		})
	}
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	h.client.apiUtilsService = utils.NewAPIUtilsService(utils.PlatformAPIConfig{
		BaseURL: server.URL + "/api/internal/v1",
		Token:   agentEvtToken,
	}, testHTTPClient(false), h.client.logger)
	return h
}

func acEvtEvent(t *testing.T, eventType, idField, id string) map[string]any {
	return agentEvtEvent(t, eventType, map[string]any{
		idField: id, "deploymentId": acEvtDeploymentID, "performedAt": time.Now(),
	}, "corr-ac-"+id)
}

func TestDeployedEvents_AckRefusalsAsUpstreamURLNotAllowed(t *testing.T) {
	h := newAccessControlEventsHarness(t)
	templateengine.SetPostRenderCheck(func(string, any) error {
		return &upstreamguard.RefusedError{FieldPath: "spec.upstream.url"}
	})
	t.Cleanup(func() { templateengine.SetPostRenderCheck(nil) })

	for _, a := range acEvtArtifacts {
		t.Run(a.kind, func(t *testing.T) {
			id := "ac-" + strings.TrimSuffix(a.kind, "s")
			a.handle(h.client, acEvtEvent(t, a.eventType, a.idField, id))
			ack := h.nextAck(t)
			assert.Equal(t, "failed", ack.Status)
			assert.Equal(t, ackCodeUpstreamURLNotAllowed, ack.ErrorCode)
			assert.Equal(t, acEvtDeploymentID, ack.DeploymentID)
		})
	}
}

// TestDeployedEvents_OtherFailuresKeepProcessingError is the control: the same
// deployments failing for another reason (an unparseable artifact) keep the
// generic code.
func TestDeployedEvents_OtherFailuresKeepProcessingError(t *testing.T) {
	h := newAccessControlEventsHarness(t)
	mux := http.NewServeMux()
	for _, a := range acEvtArtifacts {
		body := agentEvtZip(t, a.kind, "kind: [not valid yaml")
		mux.HandleFunc("/api/internal/v1/"+a.kind+"/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(body)
		})
	}
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	h.client.apiUtilsService = utils.NewAPIUtilsService(utils.PlatformAPIConfig{
		BaseURL: server.URL + "/api/internal/v1",
		Token:   agentEvtToken,
	}, testHTTPClient(false), h.client.logger)

	for _, a := range acEvtArtifacts {
		t.Run(a.kind, func(t *testing.T) {
			id := "bad-" + strings.TrimSuffix(a.kind, "s")
			a.handle(h.client, acEvtEvent(t, a.eventType, a.idField, id))
			ack := h.nextAck(t)
			require.Equal(t, "failed", ack.Status)
			assert.Equal(t, ackCodeGatewayProcessingError, ack.ErrorCode)
		})
	}
}
