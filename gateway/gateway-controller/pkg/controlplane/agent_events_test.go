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
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/agent"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// Agent control-plane event handling, exercised against a real SQLite store and
// a real AgentService. The control plane is an httptest server playing both of
// its roles: the gateway-internal REST API the artifact is fetched from, and the
// WebSocket peer that receives deployment acks.

const (
	agentEvtGatewayID = "agent-events-gateway"
	agentEvtToken     = "gw-api-key"
	agentEvtID        = "3d6f1c2a-8b4e-4f7a-9c0d-1e2f3a4b5c6d"
	agentEvtOtherID   = "9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d"
	agentEvtHandle    = "weather-agent"
)

// agentEvtYAML is a deployable gateway Agent artifact, as the control plane's
// builder emits it: a managed public card whose advertised interface matches
// the JSONRPC transport's effective path.
func agentEvtYAML(displayName string) string {
	return fmt.Sprintf(`apiVersion: gateway.api-platform.wso2.com/v1
kind: Agent
metadata:
  name: %s
  labels:
    projectId: 0b7d3c1e-5f2a-4c8e-9d6b-1a2b3c4d5e6f
spec:
  displayName: %s
  version: v1.0
  context: /weather
  upstream:
    url: http://weather-agent:9000
  a2a:
    protocolVersion: "1.0"
    operationConfigs:
      transports:
        - protocolBinding: JSONRPC
          pathPrefix: /rpc
    agentCard:
      public:
        mode: managed
        content:
          name: Weather Agent
          version: 1.0.0
          supportedInterfaces:
            - protocolBinding: JSONRPC
              protocolVersion: "1.0"
              url: https://agents.example.com/weather/rpc
          capabilities: {}
          defaultInputModes: [text/plain]
          defaultOutputModes: [text/plain]
          skills: []
`, agentEvtHandle, displayName)
}

// agentEvtZip packages yamlContent exactly as the control plane's
// CreateAgentYamlZip does: one entry, agent-{id}.yaml.
func agentEvtZip(t *testing.T, entryID, yamlContent string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("agent-" + entryID + ".yaml")
	require.NoError(t, err)
	_, err = f.Write([]byte(yamlContent))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

type agentEvtArtifact struct {
	status      int
	contentType string
	body        []byte
}

type agentEvtSecretResolver struct{}

func (agentEvtSecretResolver) Resolve(string) (string, error) { return "resolved-secret", nil }

type agentEventsHarness struct {
	client    *Client
	db        storage.Storage
	hub       *mockControlPlaneEventHub
	acks      chan DeploymentAckPayload
	sentinels chan struct{}

	mu        sync.Mutex
	artifacts map[string]agentEvtArtifact
	fetches   []*http.Request
}

func newAgentEventsHarness(t *testing.T) *agentEventsHarness {
	t.Helper()
	metrics.Init()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := storage.NewStorage(storage.BackendConfig{
		Type:       "sqlite",
		SQLitePath: filepath.Join(t.TempDir(), "agent-events.db"),
		GatewayID:  agentEvtGatewayID,
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	h := &agentEventsHarness{
		db:        db,
		hub:       &mockControlPlaneEventHub{},
		acks:      make(chan DeploymentAckPayload, 16),
		sentinels: make(chan struct{}, 16),
		artifacts: map[string]agentEvtArtifact{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/internal/v1/agents/", func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.fetches = append(h.fetches, r.Clone(r.Context()))
		artifact, ok := h.artifacts[strings.TrimPrefix(r.URL.Path, "/api/internal/v1/agents/")]
		h.mu.Unlock()
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":404,"message":"Not Found"}`))
			return
		}
		if artifact.contentType != "" {
			w.Header().Set("Content-Type", artifact.contentType)
		}
		w.WriteHeader(artifact.status)
		_, _ = w.Write(artifact.body)
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var envelope struct {
				Type    string          `json:"type"`
				Payload json.RawMessage `json:"payload"`
			}
			if json.Unmarshal(msg, &envelope) != nil {
				continue
			}
			switch envelope.Type {
			case "deployment.ack":
				var ack DeploymentAckPayload
				if json.Unmarshal(envelope.Payload, &ack) == nil {
					h.acks <- ack
				}
			case "test.sentinel":
				h.sentinels <- struct{}{}
			}
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws", nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	store := storage.NewConfigStore()
	h.client = &Client{
		config:    config.ControlPlaneConfig{Host: "control-plane.test", Token: agentEvtToken},
		logger:    logger,
		store:     store,
		db:        db,
		eventHub:  h.hub,
		gatewayID: agentEvtGatewayID,
		state:     &ConnectionState{Current: Connected, Conn: conn},
		agentService: agent.NewAgentService(store, db, config.NewParser(), config.NewAgentValidator(),
			logger, h.hub, agentEvtSecretResolver{}, agentEvtGatewayID),
		apiUtilsService: utils.NewAPIUtilsService(utils.PlatformAPIConfig{
			BaseURL: server.URL + "/api/internal/v1",
			Token:   agentEvtToken,
		}, testHTTPClient(false), logger),
	}
	return h
}

// serve makes the control plane answer GET /agents/{id} with the CP-packaged
// artifact for yamlContent.
func (h *agentEventsHarness) serve(t *testing.T, id, yamlContent string) {
	h.serveRaw(id, agentEvtArtifact{status: http.StatusOK, contentType: "application/zip", body: agentEvtZip(t, id, yamlContent)})
}

func (h *agentEventsHarness) serveRaw(id string, artifact agentEvtArtifact) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.artifacts[id] = artifact
}

func (h *agentEventsHarness) fetchCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.fetches)
}

func (h *agentEventsHarness) lastFetch(t *testing.T) *http.Request {
	h.mu.Lock()
	defer h.mu.Unlock()
	require.NotEmpty(t, h.fetches)
	return h.fetches[len(h.fetches)-1]
}

// nextAck returns the next ack the control plane received.
func (h *agentEventsHarness) nextAck(t *testing.T) DeploymentAckPayload {
	t.Helper()
	select {
	case ack := <-h.acks:
		return ack
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a deployment ack")
		return DeploymentAckPayload{}
	}
}

// assertNoAck proves nothing was acked. A sentinel is written after the handler
// returned; the WebSocket is ordered, so an ack sent by the handler would reach
// the peer before the sentinel does.
func (h *agentEventsHarness) assertNoAck(t *testing.T) {
	t.Helper()
	require.NoError(t, h.client.sendMessage([]byte(`{"type":"test.sentinel"}`)))
	select {
	case ack := <-h.acks:
		t.Fatalf("unexpected deployment ack: %+v", ack)
	case <-h.sentinels:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the sentinel")
	}
}

// agentEvtEvent builds an event as handleMessage hands it to a handler: the wire
// JSON decoded into a generic map.
func agentEvtEvent(t *testing.T, eventType string, payload map[string]any, correlationID string) map[string]any {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"type":          eventType,
		"payload":       payload,
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": correlationID,
	})
	require.NoError(t, err)
	var event map[string]any
	require.NoError(t, json.Unmarshal(raw, &event))
	return event
}

func deployedEvt(t *testing.T, id, deploymentID string, performedAt time.Time) map[string]any {
	return agentEvtEvent(t, "agent.deployed", map[string]any{
		"proxyId": id, "deploymentId": deploymentID, "performedAt": performedAt,
	}, "corr-deploy-"+deploymentID)
}

func undeployedEvt(t *testing.T, id, deploymentID string, performedAt time.Time) map[string]any {
	return agentEvtEvent(t, "agent.undeployed", map[string]any{
		"proxyId": id, "deploymentId": deploymentID, "performedAt": performedAt,
	}, "corr-undeploy-"+deploymentID)
}

func deletedEvt(t *testing.T, id string) map[string]any {
	return agentEvtEvent(t, "agent.deleted", map[string]any{"proxyId": id}, "corr-delete-"+id)
}

// deploy applies an agent.deployed event for the standard fixture and consumes
// its success ack.
func (h *agentEventsHarness) deploy(t *testing.T, id, deploymentID string, performedAt time.Time) {
	t.Helper()
	h.serve(t, id, agentEvtYAML("Weather Agent"))
	h.client.handleAgentDeployedEvent(deployedEvt(t, id, deploymentID, performedAt))
	ack := h.nextAck(t)
	require.Equal(t, "success", ack.Status, "precondition: deploy succeeded (error code %q)", ack.ErrorCode)
}

func agentEvtRestConfig(uuid, handle string) *models.StoredConfig {
	url := "https://example.com"
	restAPI := api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Metadata:   api.Metadata{Name: handle},
		Spec: api.APIConfigData{
			DisplayName: "Rest API",
			Version:     "v1.0",
			Context:     "/rest",
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{Main: api.Upstream{Url: &url}},
			Operations: []api.Operation{{Method: api.Ptr(api.OperationMethodGET), Path: api.Ptr("/")}},
		},
	}
	now := time.Now()
	return &models.StoredConfig{
		UUID:                uuid,
		Kind:                models.KindRestApi,
		Handle:              handle,
		DisplayName:         "Rest API",
		Version:             "v1.0",
		Configuration:       restAPI,
		SourceConfiguration: restAPI,
		DesiredState:        models.StateDeployed,
		Origin:              models.OriginGatewayAPI,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

func agentEvtAPIKey(artifactUUID string) *models.APIKey {
	now := time.Now()
	return &models.APIKey{
		UUID:         "agent-key-uuid",
		Name:         "agent-key",
		APIKey:       "$sha256$seed$hash-agent-key",
		MaskedAPIKey: "masked-agent-key",
		ArtifactUUID: artifactUUID,
		Status:       models.APIKeyStatusActive,
		CreatedAt:    now,
		CreatedBy:    "user-a",
		UpdatedAt:    now,
		Source:       "external",
	}
}

// --- agent.deployed ---------------------------------------------------------

// A deploy event fetches the artifact from the gateway-internal API by artifact
// UUID, stores it under that UUID, announces it so every replica rebuilds its
// xDS snapshots, and acks success with the gateway's Agent kind.
func TestHandleAgentDeployedEvent_FetchesStoresAnnouncesAndAcks(t *testing.T) {
	h := newAgentEventsHarness(t)
	performedAt := time.Now().UTC().Truncate(time.Millisecond)
	h.serve(t, agentEvtID, agentEvtYAML("Weather Agent"))

	h.client.handleAgentDeployedEvent(deployedEvt(t, agentEvtID, "dep-1", performedAt))

	// Fetch: the internal route keyed by UUID, gateway API-key auth, ZIP only.
	require.Equal(t, 1, h.fetchCount())
	fetch := h.lastFetch(t)
	assert.Equal(t, http.MethodGet, fetch.Method)
	assert.Equal(t, "/api/internal/v1/agents/"+agentEvtID, fetch.URL.Path)
	assert.Equal(t, agentEvtToken, fetch.Header.Get("api-key"))
	assert.Equal(t, "application/zip", fetch.Header.Get("Accept"))

	// Store write: under the control plane's UUID, pinned to this deployment.
	stored, err := h.db.GetConfig(agentEvtID)
	require.NoError(t, err)
	assert.Equal(t, models.KindAgent, stored.Kind)
	assert.Equal(t, agentEvtHandle, stored.Handle)
	assert.Equal(t, "Weather Agent", stored.DisplayName)
	assert.Equal(t, "dep-1", stored.DeploymentID)
	require.NotNil(t, stored.DeployedAt)
	assert.True(t, performedAt.Equal(*stored.DeployedAt))
	assert.Equal(t, models.StateDeployed, stored.DesiredState)
	assert.Equal(t, models.OriginControlPlane, stored.Origin)

	// Announcement: the event every replica's listener rebuilds its snapshots on.
	require.Len(t, h.hub.publishedEvents, 1)
	published := h.hub.publishedEvents[0]
	assert.Equal(t, agentEvtGatewayID, published.gatewayID)
	assert.Equal(t, eventhub.EventTypeAgent, published.event.EventType)
	assert.Equal(t, "CREATE", published.event.Action)
	assert.Equal(t, agentEvtID, published.event.EntityID)
	assert.Equal(t, "corr-deploy-dep-1", published.event.EventID)

	// Ack.
	ack := h.nextAck(t)
	assert.Equal(t, DeploymentAckPayload{
		DeploymentID: "dep-1",
		ArtifactID:   agentEvtID,
		ResourceType: models.KindAgent,
		Action:       "deploy",
		Status:       "success",
		PerformedAt:  ack.PerformedAt,
	}, ack)
	assert.True(t, performedAt.Equal(ack.PerformedAt), "the ack echoes the concurrency token")
}

// A deploy for an Agent already on the gateway is an in-place update.
func TestHandleAgentDeployedEvent_RedeployUpdatesInPlace(t *testing.T) {
	h := newAgentEventsHarness(t)
	first := time.Now().UTC().Truncate(time.Millisecond)
	h.deploy(t, agentEvtID, "dep-1", first)

	h.serve(t, agentEvtID, agentEvtYAML("Weather Agent Renamed"))
	h.client.handleAgentDeployedEvent(deployedEvt(t, agentEvtID, "dep-2", first.Add(time.Second)))

	ack := h.nextAck(t)
	assert.Equal(t, "success", ack.Status)
	assert.Equal(t, "dep-2", ack.DeploymentID)

	stored, err := h.db.GetConfig(agentEvtID)
	require.NoError(t, err)
	assert.Equal(t, "dep-2", stored.DeploymentID)
	assert.Equal(t, "Weather Agent Renamed", stored.DisplayName)

	require.Len(t, h.hub.publishedEvents, 2)
	assert.Equal(t, "UPDATE", h.hub.publishedEvents[1].event.Action)
}

// A deploy event older than what is stored lost a race. Nothing is written, and
// nothing is acked — the replica that applied the newer write acks that one.
func TestHandleAgentDeployedEvent_StaleEventIsNotApplied(t *testing.T) {
	h := newAgentEventsHarness(t)
	newer := time.Now().UTC().Truncate(time.Millisecond)
	h.deploy(t, agentEvtID, "dep-2", newer)

	h.serve(t, agentEvtID, agentEvtYAML("Weather Agent Stale"))
	h.client.handleAgentDeployedEvent(deployedEvt(t, agentEvtID, "dep-1", newer.Add(-time.Minute)))

	h.assertNoAck(t)
	stored, err := h.db.GetConfig(agentEvtID)
	require.NoError(t, err)
	assert.Equal(t, "dep-2", stored.DeploymentID)
	assert.Equal(t, "Weather Agent", stored.DisplayName)
	assert.Len(t, h.hub.publishedEvents, 1, "a stale write must not be announced")
}

// Every way the fetch can fail is reported through the ack path, and none of
// them leaves configuration behind.
func TestHandleAgentDeployedEvent_FailedFetchWritesNothing(t *testing.T) {
	cases := map[string]func(t *testing.T, h *agentEventsHarness){
		"not deployed on this gateway": func(*testing.T, *agentEventsHarness) {},
		"control plane error": func(_ *testing.T, h *agentEventsHarness) {
			h.serveRaw(agentEvtID, agentEvtArtifact{status: http.StatusInternalServerError, body: []byte("boom")})
		},
		"not a zip media type": func(_ *testing.T, h *agentEventsHarness) {
			h.serveRaw(agentEvtID, agentEvtArtifact{status: http.StatusOK, contentType: "application/json",
				body: []byte(`{"kind":"Agent"}`)})
		},
		"zip media type, not a zip": func(_ *testing.T, h *agentEventsHarness) {
			h.serveRaw(agentEvtID, agentEvtArtifact{status: http.StatusOK, contentType: "application/zip",
				body: []byte(agentEvtYAML("Weather Agent"))})
		},
		"archive for another agent": func(t *testing.T, h *agentEventsHarness) {
			h.serveRaw(agentEvtID, agentEvtArtifact{status: http.StatusOK, contentType: "application/zip",
				body: agentEvtZip(t, agentEvtOtherID, agentEvtYAML("Weather Agent"))})
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			h := newAgentEventsHarness(t)
			setup(t, h)

			h.client.handleAgentDeployedEvent(deployedEvt(t, agentEvtID, "dep-1", time.Now().UTC()))

			ack := h.nextAck(t)
			assert.Equal(t, "failed", ack.Status)
			assert.Equal(t, ackCodeAgentArtifactFetchFailed, ack.ErrorCode,
				"a fetch failure is reported as such, not as a gateway fault")
			assert.Equal(t, models.KindAgent, ack.ResourceType)
			assert.Equal(t, "deploy", ack.Action)
			assert.Equal(t, agentEvtID, ack.ArtifactID)
			assert.Equal(t, "dep-1", ack.DeploymentID)

			_, err := h.db.GetConfig(agentEvtID)
			assert.True(t, storage.IsNotFoundError(err), "no configuration may be written, got %v", err)
			assert.Empty(t, h.hub.publishedEvents)
		})
	}
}

// An artifact the gateway's own validation rejects is a deploy failure, not a
// partial write.
func TestHandleAgentDeployedEvent_InvalidArtifactWritesNothing(t *testing.T) {
	cases := map[string]string{
		"unregistered protocol version": strings.Replace(agentEvtYAML("Weather Agent"),
			`protocolVersion: "1.0"`+"\n    operationConfigs", `protocolVersion: "9.9"`+"\n    operationConfigs", 1),
		"another kind":  strings.Replace(agentEvtYAML("Weather Agent"), "kind: Agent", "kind: Mcp", 1),
		"not yaml":      "::: not yaml :::",
		"no transports": strings.Replace(agentEvtYAML("Weather Agent"), "        - protocolBinding: JSONRPC\n          pathPrefix: /rpc\n", "", 1),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			h := newAgentEventsHarness(t)
			require.NotEqual(t, agentEvtYAML("Weather Agent"), body, "precondition: the fixture was actually spoiled")
			h.serve(t, agentEvtID, body)

			h.client.handleAgentDeployedEvent(deployedEvt(t, agentEvtID, "dep-1", time.Now().UTC()))

			ack := h.nextAck(t)
			assert.Equal(t, "failed", ack.Status)
			assert.Equal(t, ackCodeAgentValidationFailed, ack.ErrorCode,
				"a definition the gateway rejects must be distinguishable from a gateway fault")

			_, err := h.db.GetConfig(agentEvtID)
			assert.True(t, storage.IsNotFoundError(err), "no configuration may be written, got %v", err)
			assert.Empty(t, h.hub.publishedEvents)
		})
	}
}

// A template the gateway cannot render is reported as a render failure, and
// nothing is written.
func TestHandleAgentDeployedEvent_UnrenderableTemplateIsReportedAsRenderFailure(t *testing.T) {
	h := newAgentEventsHarness(t)
	body := strings.Replace(agentEvtYAML("Weather Agent"),
		"url: http://weather-agent:9000", `url: '{{ nosuchfunc "x" }}'`, 1)
	require.NotEqual(t, agentEvtYAML("Weather Agent"), body, "precondition: the fixture was actually spoiled")
	h.serve(t, agentEvtID, body)

	h.client.handleAgentDeployedEvent(deployedEvt(t, agentEvtID, "dep-1", time.Now().UTC()))

	ack := h.nextAck(t)
	assert.Equal(t, "failed", ack.Status)
	assert.Equal(t, ackCodeAgentRenderFailed, ack.ErrorCode)
	_, err := h.db.GetConfig(agentEvtID)
	assert.True(t, storage.IsNotFoundError(err), "no configuration may be written, got %v", err)
	assert.Empty(t, h.hub.publishedEvents)
}

// A second Agent claiming a handle the gateway already holds under another UUID
// is reported as a conflict, and the existing Agent is left alone.
func TestHandleAgentDeployedEvent_HandleHeldByAnotherAgentIsReportedAsConflict(t *testing.T) {
	h := newAgentEventsHarness(t)
	h.deploy(t, agentEvtID, "dep-1", time.Now().UTC().Truncate(time.Millisecond))

	h.serve(t, agentEvtOtherID, agentEvtYAML("Another Weather Agent"))
	h.client.handleAgentDeployedEvent(deployedEvt(t, agentEvtOtherID, "dep-2", time.Now().UTC()))

	ack := h.nextAck(t)
	assert.Equal(t, "failed", ack.Status)
	assert.Equal(t, ackCodeAgentConflict, ack.ErrorCode)
	assert.Equal(t, agentEvtOtherID, ack.ArtifactID)

	_, err := h.db.GetConfig(agentEvtOtherID)
	assert.True(t, storage.IsNotFoundError(err), "the conflicting Agent must not be written, got %v", err)
	stored, err := h.db.GetConfig(agentEvtID)
	require.NoError(t, err)
	assert.Equal(t, "Weather Agent", stored.DisplayName)
}

func TestAgentAckFailureCode(t *testing.T) {
	wrap := func(err error) error { return fmt.Errorf("failed to deploy agent configuration from YAML: %w", err) }
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"validation", wrap(&agent.ValidationError{}), ackCodeAgentValidationFailed},
		{"parse", wrap(&agent.ParseError{Cause: errors.New("bad yaml")}), ackCodeAgentValidationFailed},
		{"kind mismatch", wrap(&agent.KindMismatchError{Kind: "Mcp"}), ackCodeAgentValidationFailed},
		{"handle mismatch", wrap(&agent.HandleMismatchError{PathHandle: "a", YAMLHandle: "b"}), ackCodeAgentValidationFailed},
		{"render", wrap(&templateengine.RenderError{Cause: errors.New("secret not found")}), ackCodeAgentRenderFailed},
		{"conflict", wrap(fmt.Errorf("%w: handle taken", storage.ErrConflict)), ackCodeAgentConflict},
		{"anything else", wrap(errors.New("disk full")), ackCodeGatewayProcessingError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, agentAckFailureCode(tc.err))
		})
	}
}

// Every code an Agent ack can carry fits the control plane's status_reason
// column and has the code shape it keeps; anything else would be replaced by
// GATEWAY_PROCESSING_ERROR there.
func TestAgentAckCodesFitTheControlPlaneReasonColumn(t *testing.T) {
	shape := regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	for _, code := range []string{
		ackCodeGatewayProcessingError, ackCodeDeploymentIDMismatch, ackCodeAgentArtifactFetchFailed,
		ackCodeAgentValidationFailed, ackCodeAgentRenderFailed, ackCodeAgentConflict,
	} {
		assert.LessOrEqual(t, len(code), 50, code)
		assert.Regexp(t, shape, code)
	}
}

// An event naming a UUID the gateway holds under another kind is refused before
// any fetch; applying it would overwrite that artifact through the Agent lane.
func TestHandleAgentDeployedEvent_RefusesArtifactOfAnotherKind(t *testing.T) {
	h := newAgentEventsHarness(t)
	require.NoError(t, h.db.SaveConfig(agentEvtRestConfig(agentEvtID, "rest-api")))
	h.serve(t, agentEvtID, agentEvtYAML("Weather Agent"))

	h.client.handleAgentDeployedEvent(deployedEvt(t, agentEvtID, "dep-1", time.Now().UTC()))

	ack := h.nextAck(t)
	assert.Equal(t, "failed", ack.Status)
	assert.Equal(t, "GATEWAY_PROCESSING_ERROR", ack.ErrorCode)
	assert.Zero(t, h.fetchCount(), "a kind mismatch is refused without fetching")

	stored, err := h.db.GetConfig(agentEvtID)
	require.NoError(t, err)
	assert.Equal(t, models.KindRestApi, stored.Kind)
	assert.Empty(t, h.hub.publishedEvents)
}

// A malformed event is dropped: there is no artifact to fetch and nothing to
// ack against.
func TestHandleAgentDeployedEvent_MissingIDIsIgnored(t *testing.T) {
	h := newAgentEventsHarness(t)

	h.client.handleAgentDeployedEvent(agentEvtEvent(t, "agent.deployed",
		map[string]any{"deploymentId": "dep-1", "performedAt": time.Now().UTC()}, "corr"))
	h.client.handleAgentDeployedEvent(agentEvtEvent(t, "agent.deployed",
		map[string]any{"proxyId": "", "deploymentId": "dep-1"}, "corr"))
	h.client.handleAgentDeployedEvent(map[string]any{"type": "agent.deployed", "payload": "not-an-object"})

	h.assertNoAck(t)
	assert.Zero(t, h.fetchCount())
	assert.Empty(t, h.hub.publishedEvents)
}

// The id is interpolated into the fetch path, so one that is not a UUID is
// refused before any request — and the refusal is acked, not swallowed.
func TestHandleAgentDeployedEvent_NonUUIDIDIsRefusedWithoutFetching(t *testing.T) {
	h := newAgentEventsHarness(t)

	h.client.handleAgentDeployedEvent(deployedEvt(t, "../apis/"+agentEvtID, "dep-1", time.Now().UTC()))

	ack := h.nextAck(t)
	assert.Equal(t, "failed", ack.Status)
	assert.Zero(t, h.fetchCount())
	assert.Empty(t, h.hub.publishedEvents)
}

// --- agent.undeployed -------------------------------------------------------

// Undeploy keeps the configuration and the Agent's keys for a later redeploy,
// marks it undeployed, announces it and acks.
func TestHandleAgentUndeployedEvent_UndeploysKeepsKeysAndAcks(t *testing.T) {
	h := newAgentEventsHarness(t)
	deployedAt := time.Now().UTC().Truncate(time.Millisecond)
	h.deploy(t, agentEvtID, "dep-1", deployedAt)
	require.NoError(t, h.db.SaveAPIKey(agentEvtAPIKey(agentEvtID)))

	undeployedAt := deployedAt.Add(time.Second)
	h.client.handleAgentUndeployedEvent(undeployedEvt(t, agentEvtID, "dep-1", undeployedAt))

	ack := h.nextAck(t)
	assert.Equal(t, DeploymentAckPayload{
		DeploymentID: "dep-1",
		ArtifactID:   agentEvtID,
		ResourceType: models.KindAgent,
		Action:       "undeploy",
		Status:       "success",
		PerformedAt:  ack.PerformedAt,
	}, ack)
	assert.True(t, undeployedAt.Equal(ack.PerformedAt))

	stored, err := h.db.GetConfig(agentEvtID)
	require.NoError(t, err, "undeploy keeps the configuration")
	assert.Equal(t, models.StateUndeployed, stored.DesiredState)
	require.NotNil(t, stored.DeployedAt)
	assert.True(t, undeployedAt.Equal(*stored.DeployedAt))

	keys, err := h.db.GetAPIKeysByAPI(agentEvtID)
	require.NoError(t, err)
	assert.Len(t, keys, 1, "undeploy is reversible, so the Agent's keys survive it")

	require.Len(t, h.hub.publishedEvents, 2)
	published := h.hub.publishedEvents[1].event
	assert.Equal(t, eventhub.EventTypeAgent, published.EventType)
	assert.Equal(t, "UPDATE", published.Action)
	assert.Equal(t, agentEvtID, published.EntityID)
}

// An undeploy naming a deployment this gateway never applied is refused.
func TestHandleAgentUndeployedEvent_DeploymentIDMismatch(t *testing.T) {
	h := newAgentEventsHarness(t)
	deployedAt := time.Now().UTC().Truncate(time.Millisecond)
	h.deploy(t, agentEvtID, "dep-2", deployedAt)

	h.client.handleAgentUndeployedEvent(undeployedEvt(t, agentEvtID, "dep-1", deployedAt.Add(time.Second)))

	ack := h.nextAck(t)
	assert.Equal(t, "failed", ack.Status)
	assert.Equal(t, "DEPLOYMENT_ID_MISMATCH", ack.ErrorCode)

	stored, err := h.db.GetConfig(agentEvtID)
	require.NoError(t, err)
	assert.Equal(t, models.StateDeployed, stored.DesiredState)
	assert.Equal(t, "dep-2", stored.DeploymentID)
	assert.Len(t, h.hub.publishedEvents, 1)
}

// Undeploying an Agent the gateway does not hold has nothing left to do.
func TestHandleAgentUndeployedEvent_UnknownAgentAcksSuccess(t *testing.T) {
	h := newAgentEventsHarness(t)

	h.client.handleAgentUndeployedEvent(undeployedEvt(t, agentEvtID, "dep-1", time.Now().UTC()))

	ack := h.nextAck(t)
	assert.Equal(t, "success", ack.Status)
	assert.Equal(t, "undeploy", ack.Action)
	assert.Empty(t, h.hub.publishedEvents)
}

// An undeploy older than the stored deployment lost a race and is not acked.
func TestHandleAgentUndeployedEvent_StaleEventIsNotApplied(t *testing.T) {
	h := newAgentEventsHarness(t)
	deployedAt := time.Now().UTC().Truncate(time.Millisecond)
	h.deploy(t, agentEvtID, "dep-1", deployedAt)

	h.client.handleAgentUndeployedEvent(undeployedEvt(t, agentEvtID, "dep-1", deployedAt.Add(-time.Minute)))

	h.assertNoAck(t)
	stored, err := h.db.GetConfig(agentEvtID)
	require.NoError(t, err)
	assert.Equal(t, models.StateDeployed, stored.DesiredState)
	assert.Len(t, h.hub.publishedEvents, 1)
}

func TestHandleAgentUndeployedEvent_RefusesArtifactOfAnotherKind(t *testing.T) {
	h := newAgentEventsHarness(t)
	require.NoError(t, h.db.SaveConfig(agentEvtRestConfig(agentEvtID, "rest-api")))

	h.client.handleAgentUndeployedEvent(undeployedEvt(t, agentEvtID, "dep-1", time.Now().UTC()))

	ack := h.nextAck(t)
	assert.Equal(t, "failed", ack.Status)
	stored, err := h.db.GetConfig(agentEvtID)
	require.NoError(t, err)
	assert.Equal(t, models.StateDeployed, stored.DesiredState)
	assert.Empty(t, h.hub.publishedEvents)
}

// --- agent.deleted ----------------------------------------------------------

// Deletion removes the configuration and the Agent's keys and announces it, so
// every replica drops its routes and chains. Deletion is not acked.
func TestHandleAgentDeletedEvent_DeletesConfigAndKeys(t *testing.T) {
	h := newAgentEventsHarness(t)
	h.deploy(t, agentEvtID, "dep-1", time.Now().UTC().Truncate(time.Millisecond))
	require.NoError(t, h.db.SaveAPIKey(agentEvtAPIKey(agentEvtID)))

	h.client.handleAgentDeletedEvent(deletedEvt(t, agentEvtID))

	_, err := h.db.GetConfig(agentEvtID)
	assert.True(t, storage.IsNotFoundError(err), "the configuration should be gone, got %v", err)
	keys, err := h.db.GetAPIKeysByAPI(agentEvtID)
	require.NoError(t, err)
	assert.Empty(t, keys)

	require.Len(t, h.hub.publishedEvents, 2)
	published := h.hub.publishedEvents[1].event
	assert.Equal(t, eventhub.EventTypeAgent, published.EventType)
	assert.Equal(t, "DELETE", published.Action)
	assert.Equal(t, agentEvtID, published.EntityID)

	h.assertNoAck(t)
}

func TestHandleAgentDeletedEvent_UnknownAgentIsNoop(t *testing.T) {
	h := newAgentEventsHarness(t)

	h.client.handleAgentDeletedEvent(deletedEvt(t, agentEvtID))

	assert.Empty(t, h.hub.publishedEvents)
	h.assertNoAck(t)
}

// Delete resolves the event's UUID to a handle and deletes the Agent with that
// handle. A row of another kind sharing the UUID must stop it there — otherwise
// an Agent that merely shares that row's handle is deleted in its place.
func TestHandleAgentDeletedEvent_IgnoresArtifactOfAnotherKind(t *testing.T) {
	h := newAgentEventsHarness(t)
	h.deploy(t, agentEvtID, "dep-1", time.Now().UTC().Truncate(time.Millisecond))
	require.NoError(t, h.db.SaveConfig(agentEvtRestConfig(agentEvtOtherID, agentEvtHandle)))

	h.client.handleAgentDeletedEvent(deletedEvt(t, agentEvtOtherID))

	agentCfg, err := h.db.GetConfig(agentEvtID)
	require.NoError(t, err, "the unrelated Agent sharing the handle must survive")
	assert.Equal(t, models.KindAgent, agentCfg.Kind)
	restCfg, err := h.db.GetConfig(agentEvtOtherID)
	require.NoError(t, err)
	assert.Equal(t, models.KindRestApi, restCfg.Kind)
	assert.Len(t, h.hub.publishedEvents, 1, "nothing is announced")
}

// --- dispatch ---------------------------------------------------------------

// The three agent.* event types reach their handlers from the raw WebSocket
// message, with the control plane's wire payload.
func TestHandleMessage_DispatchesAgentEvents(t *testing.T) {
	h := newAgentEventsHarness(t)
	h.serve(t, agentEvtID, agentEvtYAML("Weather Agent"))
	performedAt := time.Now().UTC().Truncate(time.Millisecond)

	send := func(eventType string, payload map[string]any) {
		raw, err := json.Marshal(map[string]any{
			"type": eventType, "payload": payload,
			"timestamp": time.Now().Format(time.RFC3339), "correlationId": "corr-" + eventType,
		})
		require.NoError(t, err)
		h.client.handleMessage(websocket.TextMessage, raw)
	}

	send("agent.deployed", map[string]any{"proxyId": agentEvtID, "deploymentId": "dep-1", "performedAt": performedAt})
	assert.Equal(t, "deploy", h.nextAck(t).Action)
	stored, err := h.db.GetConfig(agentEvtID)
	require.NoError(t, err)
	assert.Equal(t, models.StateDeployed, stored.DesiredState)

	send("agent.undeployed", map[string]any{"proxyId": agentEvtID, "deploymentId": "dep-1", "performedAt": performedAt.Add(time.Second)})
	assert.Equal(t, "undeploy", h.nextAck(t).Action)
	stored, err = h.db.GetConfig(agentEvtID)
	require.NoError(t, err)
	assert.Equal(t, models.StateUndeployed, stored.DesiredState)

	send("agent.deleted", map[string]any{"proxyId": agentEvtID})
	_, err = h.db.GetConfig(agentEvtID)
	assert.True(t, storage.IsNotFoundError(err))

	actions := make([]string, 0, len(h.hub.publishedEvents))
	for _, e := range h.hub.publishedEvents {
		assert.Equal(t, eventhub.EventTypeAgent, e.event.EventType)
		actions = append(actions, e.event.Action)
	}
	assert.Equal(t, []string{"CREATE", "UPDATE", "DELETE"}, actions)
}

// --- control-plane artifact round trip ---------------------------------------

// controlPlaneAgentFixtures is the control plane's golden Agent deployment
// YAML, read as files: the control plane asserts its builder reproduces them
// byte for byte, so they are exactly what its gateway-internal API serves. The
// gateway does not (and must not) depend on the platform-api module.
var controlPlaneAgentFixtures = filepath.Join("..", "..", "..", "..",
	"platform-api", "internal", "utils", "testdata", "agent_proxy")

// jsonValue normalises v through JSON, so YAML-decoded fixture values and
// gateway model values compare as the same document.
func jsonValue(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	var out any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

// lookup walks a normalised document along path, reporting whether it exists.
func lookup(doc any, path ...string) (any, bool) {
	for _, key := range path {
		m, ok := doc.(map[string]any)
		if !ok {
			return nil, false
		}
		if doc, ok = m[key]; !ok {
			return nil, false
		}
	}
	return doc, true
}

// The control plane moves authoring fields into the gateway shape — the Agent
// Card under spec.a2a.agentCard, the transports under
// spec.a2a.operationConfigs.transports. Both have to survive the whole path:
// CP build → gateway-internal ZIP → fetch → gateway parse and validation →
// storage, with omitted blocks still omitted and explicit false values intact.
func TestAgentDeployment_ControlPlaneArtifactSurvivesToGatewayConfig(t *testing.T) {
	if _, err := os.Stat(controlPlaneAgentFixtures); err != nil {
		t.Skipf("control-plane fixtures not available at %s: %v", controlPlaneAgentFixtures, err)
	}

	for _, fixture := range []string{"full", "minimal", "card_variants"} {
		t.Run(fixture, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(controlPlaneAgentFixtures, fixture+".yaml"))
			require.NoError(t, err)
			var fixtureDoc map[string]any
			require.NoError(t, yaml.Unmarshal(raw, &fixtureDoc))
			want := jsonValue(t, fixtureDoc)

			h := newAgentEventsHarness(t)
			h.serve(t, agentEvtID, string(raw))
			performedAt := time.Now().UTC().Truncate(time.Millisecond)

			h.client.handleAgentDeployedEvent(deployedEvt(t, agentEvtID, "dep-"+fixture, performedAt))

			ack := h.nextAck(t)
			require.Equal(t, "success", ack.Status, "the gateway rejected the control plane's %s artifact", fixture)

			stored, err := h.db.GetConfig(agentEvtID)
			require.NoError(t, err)
			agentCfg, ok := stored.SourceConfiguration.(api.AgentConfiguration)
			require.True(t, ok, "stored configuration is %T", stored.SourceConfiguration)
			got := jsonValue(t, agentCfg)

			wantTransports, ok := lookup(want, "spec", "a2a", "operationConfigs", "transports")
			require.True(t, ok, "every control-plane artifact carries transports")
			gotTransports, ok := lookup(got, "spec", "a2a", "operationConfigs", "transports")
			require.True(t, ok, "transports must land under spec.a2a.operationConfigs")
			assert.Equal(t, wantTransports, gotTransports)

			wantCard, wantHasCard := lookup(want, "spec", "a2a", "agentCard")
			gotCard, gotHasCard := lookup(got, "spec", "a2a", "agentCard")
			assert.Equal(t, wantHasCard, gotHasCard, "an omitted agentCard stays omitted")
			assert.Equal(t, wantCard, gotCard, "the Agent Card must survive intact under spec.a2a.agentCard")

			// Nothing lands where the control plane's authoring layout keeps it.
			_, misplaced := lookup(got, "spec", "a2a", "transports")
			assert.False(t, misplaced, "transports must not appear directly under spec.a2a")
			_, misplaced = lookup(got, "spec", "agentCard")
			assert.False(t, misplaced, "the card must not appear directly under spec")
			_, misplaced = lookup(got, "spec", "protocol")
			assert.False(t, misplaced, "the control plane's protocol discriminator is not a gateway field")

			// The rest of the a2a block and the shared fields round-trip too.
			for _, path := range [][]string{
				{"spec", "a2a", "protocolVersion"},
				{"spec", "a2a", "operationConfigs", "policies"},
				{"spec", "a2a", "operationConfigs", "operations"},
				{"spec", "upstream"},
				{"spec", "resilience"},
				{"spec", "context"},
				{"spec", "vhost"},
				{"spec", "displayName"},
				{"spec", "version"},
			} {
				wantValue, wantHas := lookup(want, path...)
				gotValue, gotHas := lookup(got, path...)
				assert.Equalf(t, wantHas, gotHas, "presence of %s", strings.Join(path, "."))
				assert.Equalf(t, wantValue, gotValue, "value of %s", strings.Join(path, "."))
			}

			assert.Equal(t, fixtureDoc["metadata"].(map[string]any)["name"], stored.Handle)
		})
	}
}

// --- manifest ---------------------------------------------------------------

// The A2A system policy is gateway-internal; the control plane must never see it
// in the gateway's policy manifest. The exclusion is a name-prefix filter, so
// this pins it to the policy's real name rather than to a lookalike.
func TestPushGatewayManifestOnConnect_ExcludesA2ASystemPolicy(t *testing.T) {
	require.True(t, strings.HasPrefix(constants.A2A_SYSTEM_POLICY_NAME, "wso2_apip_sys_"),
		"the A2A system policy relies on the system-policy prefix to stay out of the manifest")

	var capturedBody []byte
	client := newManifestTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	})
	client.policyDefinitions = map[string]models.PolicyDefinition{
		constants.A2A_SYSTEM_POLICY_NAME + "|v1.0.0": makePolicyDef(constants.A2A_SYSTEM_POLICY_NAME, "v1.0.0", "wso2"),
		"cors|v1.0.0":       makePolicyDef("cors", "v1.0.0", "wso2"),
		"jwt-auth|v1.0.0":   makePolicyDef("jwt-auth", "v1.0.0", "wso2"),
		"org-policy|v1.0.0": makePolicyDef("org-policy", "v1.0.0", "organization"),
	}

	client.pushGatewayManifestOnConnect(testGatewayID)

	var payload manifestPayload
	require.NoError(t, json.Unmarshal(capturedBody, &payload))
	names := make([]string, 0, len(payload.Policies))
	for _, p := range payload.Policies {
		names = append(names, p.Name)
	}
	assert.NotContains(t, names, constants.A2A_SYSTEM_POLICY_NAME)
	assert.ElementsMatch(t, []string{"cors", "jwt-auth", "org-policy"}, names)
}
