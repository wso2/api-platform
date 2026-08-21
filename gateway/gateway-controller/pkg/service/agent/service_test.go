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

package agent

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
)

const testGatewayID = "test-gateway"

// --- test doubles ---------------------------------------------------------

type recordingEventHub struct {
	events     []eventhub.Event
	publishErr error
}

func (h *recordingEventHub) Initialize() error            { return nil }
func (h *recordingEventHub) RegisterGateway(string) error { return nil }
func (h *recordingEventHub) UnsubscribeAll(string) error  { return nil }
func (h *recordingEventHub) CleanUpEvents() error         { return nil }
func (h *recordingEventHub) Close() error                 { return nil }
func (h *recordingEventHub) Subscribe(string) (<-chan eventhub.Event, error) {
	return nil, nil
}
func (h *recordingEventHub) Unsubscribe(string, <-chan eventhub.Event) error { return nil }

func (h *recordingEventHub) PublishEvent(gatewayID string, event eventhub.Event) error {
	if h.publishErr != nil {
		return h.publishErr
	}
	h.events = append(h.events, event)
	return nil
}

func (h *recordingEventHub) actions() []string {
	out := make([]string, 0, len(h.events))
	for _, e := range h.events {
		out = append(out, e.Action)
	}
	return out
}

// staticSecrets resolves a fixed set of handles and fails for anything else, so
// a test can assert both the resolve path and the missing-secret path.
type staticSecrets map[string]string

func (s staticSecrets) Resolve(handle string) (string, error) {
	if v, ok := s[handle]; ok {
		return v, nil
	}
	return "", fmt.Errorf("secret %q not found", handle)
}

// --- fixtures ------------------------------------------------------------

// agentYAML builds an Agent artifact. Fields are interpolated so each test can
// vary just the part it is about.
type agentYAMLOpts struct {
	handle       string
	displayName  string
	version      string
	context      string
	upstreamAuth string
	signing      string
	protected    string
	deployState  string
	annotations  string
}

func agentYAML(o agentYAMLOpts) []byte {
	if o.handle == "" {
		o.handle = "weather-agent-v1-0"
	}
	if o.displayName == "" {
		o.displayName = "Weather Agent"
	}
	if o.version == "" {
		o.version = "v1.0"
	}
	if o.context == "" {
		o.context = "/weather"
	}

	return []byte(fmt.Sprintf(`
apiVersion: gateway.api-platform.wso2.com/v1
kind: Agent
metadata:
  name: %s
%s
spec:
  displayName: %s
  version: %s
  context: %s
%s
%s
  a2a:
    protocolVersion: "1.0"
    operationConfigs:
      transports:
        - protocolBinding: JSONRPC
          pathPrefix: /rpc
    agentCard:
      public:
        mode: managed
%s
        content: {
          "name": "Weather Agent",
          "description": "Provides weather information",
          "version": "1.0.0",
          "supportedInterfaces": [
            {"protocolBinding": "JSONRPC", "protocolVersion": "1.0", "url": "https://agents.example.com/weather/rpc"}
          ],
          "capabilities": {"streaming": true},
          "defaultInputModes": ["text/plain"],
          "defaultOutputModes": ["text/plain"],
          "skills": []
        }
%s
`, o.handle, o.annotations, o.displayName, o.version, o.context, upstreamBlock(o.upstreamAuth), o.deployState, o.signing, o.protected))
}

func upstreamBlock(auth string) string {
	block := "  upstream:\n    url: https://weather.internal"
	if auth != "" {
		block += "\n" + auth
	}
	return block
}

// --- harness -------------------------------------------------------------

type harness struct {
	service  *AgentService
	db       storage.Storage
	store    *storage.ConfigStore
	eventHub *recordingEventHub
}

func newHarness(t *testing.T, secrets staticSecrets) *harness {
	t.Helper()
	metrics.Init()

	dbPath := filepath.Join(t.TempDir(), "agent-service.db")
	db, err := storage.NewStorage(storage.BackendConfig{
		Type:       "sqlite",
		SQLitePath: dbPath,
		GatewayID:  testGatewayID,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	hub := &recordingEventHub{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	store := storage.NewConfigStore()
	svc := NewAgentService(
		store,
		db,
		config.NewParser(),
		config.NewAgentValidator(),
		logger,
		hub,
		secrets,
		testGatewayID,
	)

	return &harness{service: svc, db: db, store: store, eventHub: hub}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func (h *harness) create(t *testing.T, body []byte) (*CreateResult, error) {
	t.Helper()
	return h.service.Create(CreateParams{
		Body:        body,
		ContentType: "application/yaml",
		Origin:      models.OriginGatewayAPI,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

// --- create --------------------------------------------------------------

func TestCreate_PersistsAndAnnounces(t *testing.T) {
	h := newHarness(t, nil)

	result, err := h.create(t, agentYAML(agentYAMLOpts{}))
	require.NoError(t, err)
	require.NotNil(t, result.StoredConfig)
	assert.False(t, result.IsUpdate)
	assert.False(t, result.IsStale)

	stored := result.StoredConfig
	assert.Equal(t, models.KindAgent, stored.Kind)
	assert.Equal(t, "weather-agent-v1-0", stored.Handle)
	assert.Equal(t, "Weather Agent", stored.DisplayName)
	assert.Equal(t, "v1.0", stored.Version)
	assert.Equal(t, models.StateDeployed, stored.DesiredState)
	assert.NotEmpty(t, stored.UUID)

	// Readable back with the a2a subtree intact.
	reread, err := h.db.GetConfigByKindAndHandle(models.KindAgent, "weather-agent-v1-0")
	require.NoError(t, err)
	cfg, ok := reread.Configuration.(api.AgentConfiguration)
	require.True(t, ok, "Configuration is %T", reread.Configuration)
	require.Len(t, cfg.Spec.A2a.OperationConfigs.Transports, 1)
	assert.Equal(t, api.JSONRPC, cfg.Spec.A2a.OperationConfigs.Transports[0].ProtocolBinding)
	require.NotNil(t, cfg.Spec.A2a.AgentCard.Public.Content)

	assert.Equal(t, []string{"CREATE"}, h.eventHub.actions(),
		"every replica converges through the event, so the create has to announce itself")
}

func TestCreate_RendersSecretBeforeValidating(t *testing.T) {
	h := newHarness(t, staticSecrets{"weather-key": "s3cret-value"})

	body := agentYAML(agentYAMLOpts{
		upstreamAuth: "    auth:\n      type: api-key\n      header: x-api-key\n      value: '{{ secret \"weather-key\" }}'",
	})

	result, err := h.create(t, body)
	require.NoError(t, err)

	// Configuration is rendered: this is the value the transformer and the
	// policy chain see.
	rendered, ok := result.StoredConfig.Configuration.(api.AgentConfiguration)
	require.True(t, ok)
	require.NotNil(t, rendered.Spec.Upstream.Auth)
	require.NotNil(t, rendered.Spec.Upstream.Auth.Value)
	assert.Equal(t, "s3cret-value", *rendered.Spec.Upstream.Auth.Value)

	// SourceConfiguration keeps the template, so a rotated secret is picked up
	// on the next consumption instead of being frozen into the artifact.
	source, ok := result.StoredConfig.SourceConfiguration.(api.AgentConfiguration)
	require.True(t, ok)
	require.NotNil(t, source.Spec.Upstream.Auth)
	require.NotNil(t, source.Spec.Upstream.Auth.Value)
	assert.Contains(t, *source.Spec.Upstream.Auth.Value, "{{ secret")

	// The resolved plaintext is tracked so the config dump can redact it.
	assert.Contains(t, result.StoredConfig.SensitiveValues, "s3cret-value")

	// What the database holds is the unrendered form.
	reread, err := h.db.GetConfigByKindAndHandle(models.KindAgent, "weather-agent-v1-0")
	require.NoError(t, err)
	stored, ok := reread.SourceConfiguration.(api.AgentConfiguration)
	require.True(t, ok)
	require.NotNil(t, stored.Spec.Upstream.Auth.Value)
	assert.Contains(t, *stored.Spec.Upstream.Auth.Value, "{{ secret",
		"the rendered secret must never be persisted")
}

func TestCreate_MissingSecretIsARenderError(t *testing.T) {
	h := newHarness(t, staticSecrets{})

	body := agentYAML(agentYAMLOpts{
		upstreamAuth: "    auth:\n      type: api-key\n      header: x-api-key\n      value: '{{ secret \"absent\" }}'",
	})

	_, err := h.create(t, body)
	require.Error(t, err)

	var renderErr *templateengine.RenderError
	assert.True(t, errors.As(err, &renderErr), "expected a render error, got %T: %v", err, err)
	assert.Empty(t, h.eventHub.actions(), "a failed create must not announce anything")
}

func TestCreate_RejectsUnparseableBody(t *testing.T) {
	h := newHarness(t, nil)

	_, err := h.create(t, []byte("spec: [this is not: valid yaml"))
	require.Error(t, err)

	var parseErr *ParseError
	assert.True(t, errors.As(err, &parseErr), "expected *ParseError, got %T", err)
}

func TestCreate_RejectsForeignKind(t *testing.T) {
	h := newHarness(t, nil)

	body := strings.Replace(string(agentYAML(agentYAMLOpts{})), "kind: Agent", "kind: Mcp", 1)

	_, err := h.create(t, []byte(body))
	require.Error(t, err)

	var kindErr *KindMismatchError
	require.True(t, errors.As(err, &kindErr), "expected *KindMismatchError, got %T: %v", err, err)
	assert.Equal(t, "Mcp", kindErr.Kind)

	// Nothing may be stored under the foreign kind either.
	_, err = h.db.GetConfigByKindAndHandle("Mcp", "weather-agent-v1-0")
	assert.Error(t, err)
}

func TestCreate_RejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		opts  agentYAMLOpts
		field string
	}{
		{
			name:  "signing enabled is not supported yet",
			opts:  agentYAMLOpts{signing: "        signing:\n          enabled: true\n          algorithm: ES256"},
			field: "spec.a2a.agentCard.public.signing.enabled",
		},
		{
			name:  "protected card is not supported yet",
			opts:  agentYAMLOpts{protected: "      protected:\n        mode: passthrough"},
			field: "spec.a2a.agentCard.protected",
		},
		{
			name:  "display name is required",
			opts:  agentYAMLOpts{displayName: `""`},
			field: "spec.displayName",
		},
		{
			name:  "version must be vMAJOR.MINOR",
			opts:  agentYAMLOpts{version: "1"},
			field: "spec.version",
		},
		{
			name:  "context must not shadow the gateway health namespace",
			opts:  agentYAMLOpts{context: "/_gateway-health/ready"},
			field: "spec.context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, nil)

			_, err := h.create(t, agentYAML(tt.opts))
			require.Error(t, err)

			var validationErr *ValidationError
			require.True(t, errors.As(err, &validationErr), "expected *ValidationError, got %T: %v", err, err)

			fields := make([]string, 0, len(validationErr.Errors))
			for _, e := range validationErr.Errors {
				fields = append(fields, e.Field)
			}
			assert.Contains(t, fields, tt.field)

			assert.Empty(t, h.eventHub.actions(), "a rejected agent must not be announced")
		})
	}
}

func TestCreate_RejectsConflicts(t *testing.T) {
	t.Run("same handle", func(t *testing.T) {
		h := newHarness(t, nil)
		_, err := h.create(t, agentYAML(agentYAMLOpts{}))
		require.NoError(t, err)

		// Same handle, different name/version so the handle is what collides.
		_, err = h.create(t, agentYAML(agentYAMLOpts{displayName: "Other Agent", version: "v2.0", context: "/other"}))
		require.Error(t, err)
		assert.True(t, storage.IsConflictError(err), "expected a conflict, got %v", err)
	})

	t.Run("same display name and version", func(t *testing.T) {
		h := newHarness(t, nil)
		_, err := h.create(t, agentYAML(agentYAMLOpts{}))
		require.NoError(t, err)

		_, err = h.create(t, agentYAML(agentYAMLOpts{handle: "other-agent-v1-0", context: "/other"}))
		require.Error(t, err)
		assert.True(t, storage.IsConflictError(err), "expected a conflict, got %v", err)
	})
}

func TestCreate_HonoursArtifactIDAnnotation(t *testing.T) {
	h := newHarness(t, nil)

	const pinned = "019d953f-d386-7a64-aa92-1869a28292e0"
	body := agentYAML(agentYAMLOpts{
		annotations: "  annotations:\n    gateway.api-platform.wso2.com/artifact-id: " + pinned,
	})

	result, err := h.create(t, body)
	require.NoError(t, err)
	assert.Equal(t, pinned, result.StoredConfig.UUID)

	// Re-applying the same artifact is an update of the same row, not a second
	// one. DeployedAt is explicit because the upsert is timestamp-guarded: two
	// applies landing in the same millisecond would make the second one look
	// stale, which is correct behaviour but not what this test is about.
	later := result.StoredConfig.DeployedAt.Add(time.Second)
	again, err := h.service.Create(CreateParams{
		Body:        body,
		ContentType: "application/yaml",
		Origin:      models.OriginGatewayAPI,
		DeployedAt:  &later,
		Logger:      discardLogger(),
	})
	require.NoError(t, err)
	assert.True(t, again.IsUpdate)
	assert.False(t, again.IsStale)
	assert.Equal(t, pinned, again.StoredConfig.UUID)
	assert.Equal(t, []string{"CREATE", "UPDATE"}, h.eventHub.actions())
}

func TestCreate_RejectsMalformedArtifactIDAnnotation(t *testing.T) {
	h := newHarness(t, nil)

	body := agentYAML(agentYAMLOpts{
		annotations: "  annotations:\n    gateway.api-platform.wso2.com/artifact-id: not-a-uuid",
	})

	_, err := h.create(t, body)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "artifact-id")
}

func TestCreate_UndeployedDeploymentState(t *testing.T) {
	h := newHarness(t, nil)

	result, err := h.create(t, agentYAML(agentYAMLOpts{deployState: "  deploymentState: undeployed"}))
	require.NoError(t, err)
	assert.Equal(t, models.StateUndeployed, result.StoredConfig.DesiredState)
}

// --- list / get ----------------------------------------------------------

func TestListAndGet(t *testing.T) {
	h := newHarness(t, nil)

	_, err := h.create(t, agentYAML(agentYAMLOpts{}))
	require.NoError(t, err)
	_, err = h.create(t, agentYAML(agentYAMLOpts{
		handle: "travel-agent-v2-0", displayName: "Travel Agent", version: "v2.0", context: "/travel",
	}))
	require.NoError(t, err)

	all, err := h.service.List(api.ListAgentsParams{})
	require.NoError(t, err)
	assert.Len(t, all.Items, 2)

	name := "Travel Agent"
	byName, err := h.service.List(api.ListAgentsParams{DisplayName: &name})
	require.NoError(t, err)
	require.Len(t, byName.Items, 1)
	assert.Equal(t, "travel-agent-v2-0", byName.Items[0].Handle)

	version := "v1.0"
	byVersion, err := h.service.List(api.ListAgentsParams{Version: &version})
	require.NoError(t, err)
	require.Len(t, byVersion.Items, 1)
	assert.Equal(t, "Weather Agent", byVersion.Items[0].DisplayName)

	ctx := "/travel"
	byContext, err := h.service.List(api.ListAgentsParams{Context: &ctx})
	require.NoError(t, err)
	require.Len(t, byContext.Items, 1)
	assert.Equal(t, "travel-agent-v2-0", byContext.Items[0].Handle)

	status := api.ListAgentsParamsStatusUndeployed
	byStatus, err := h.service.List(api.ListAgentsParams{Status: &status})
	require.NoError(t, err)
	assert.Empty(t, byStatus.Items)

	got, err := h.service.GetByHandle("weather-agent-v1-0")
	require.NoError(t, err)
	assert.Equal(t, "Weather Agent", got.Config.DisplayName)

	_, err = h.service.GetByHandle("absent")
	assert.ErrorIs(t, err, ErrNotFound)
}

// --- update --------------------------------------------------------------

func TestUpdate_AppliesChanges(t *testing.T) {
	h := newHarness(t, nil)

	created, err := h.create(t, agentYAML(agentYAMLOpts{}))
	require.NoError(t, err)

	result, err := h.service.Update(UpdateParams{
		Handle:      "weather-agent-v1-0",
		Body:        agentYAML(agentYAMLOpts{displayName: "Renamed Agent"}),
		ContentType: "application/yaml",
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	require.NoError(t, err)
	assert.Equal(t, "Renamed Agent", result.Config.DisplayName)
	assert.Equal(t, created.StoredConfig.UUID, result.Config.UUID, "update must not re-key the artifact")

	reread, err := h.db.GetConfigByKindAndHandle(models.KindAgent, "weather-agent-v1-0")
	require.NoError(t, err)
	assert.Equal(t, "Renamed Agent", reread.DisplayName)

	assert.Equal(t, []string{"CREATE", "UPDATE"}, h.eventHub.actions())
}

func TestUpdate_RejectsHandleMismatch(t *testing.T) {
	h := newHarness(t, nil)
	_, err := h.create(t, agentYAML(agentYAMLOpts{}))
	require.NoError(t, err)

	_, err = h.service.Update(UpdateParams{
		Handle:      "weather-agent-v1-0",
		Body:        agentYAML(agentYAMLOpts{handle: "different-handle"}),
		ContentType: "application/yaml",
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	require.Error(t, err)

	var handleErr *HandleMismatchError
	require.True(t, errors.As(err, &handleErr), "expected *HandleMismatchError, got %T", err)
	assert.Equal(t, "weather-agent-v1-0", handleErr.PathHandle)
	assert.Equal(t, "different-handle", handleErr.YAMLHandle)
}

func TestUpdate_NotFound(t *testing.T) {
	h := newHarness(t, nil)

	_, err := h.service.Update(UpdateParams{
		Handle:      "absent",
		Body:        agentYAML(agentYAMLOpts{handle: "absent"}),
		ContentType: "application/yaml",
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestUpdate_InheritsStoredCredentialWhenOmitted(t *testing.T) {
	h := newHarness(t, nil)

	withCredential := agentYAML(agentYAMLOpts{
		upstreamAuth: "    auth:\n      type: api-key\n      header: x-api-key\n      value: stored-secret",
	})
	_, err := h.create(t, withCredential)
	require.NoError(t, err)

	// The credential is write-only, so a body round-tripped from a GET carries
	// the auth block without a value. That must not wipe the stored credential.
	withoutValue := agentYAML(agentYAMLOpts{
		displayName:  "Weather Agent",
		upstreamAuth: "    auth:\n      type: api-key\n      header: x-api-key",
	})
	result, err := h.service.Update(UpdateParams{
		Handle:      "weather-agent-v1-0",
		Body:        withoutValue,
		ContentType: "application/yaml",
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	require.NoError(t, err)

	source, ok := result.Config.SourceConfiguration.(api.AgentConfiguration)
	require.True(t, ok)
	require.NotNil(t, source.Spec.Upstream.Auth)
	require.NotNil(t, source.Spec.Upstream.Auth.Value, "stored credential was dropped by an update that omitted it")
	assert.Equal(t, "stored-secret", *source.Spec.Upstream.Auth.Value)
}

func TestUpdate_TypeNoneRemovesCredential(t *testing.T) {
	h := newHarness(t, nil)

	_, err := h.create(t, agentYAML(agentYAMLOpts{
		upstreamAuth: "    auth:\n      type: api-key\n      header: x-api-key\n      value: stored-secret",
	}))
	require.NoError(t, err)

	result, err := h.service.Update(UpdateParams{
		Handle:      "weather-agent-v1-0",
		Body:        agentYAML(agentYAMLOpts{upstreamAuth: "    auth:\n      type: none"}),
		ContentType: "application/yaml",
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	require.NoError(t, err)

	source, ok := result.Config.SourceConfiguration.(api.AgentConfiguration)
	require.True(t, ok)
	require.NotNil(t, source.Spec.Upstream.Auth)
	assert.Equal(t, api.AgentConfigDataUpstreamAuthTypeNone, source.Spec.Upstream.Auth.Type)
	assert.Nil(t, source.Spec.Upstream.Auth.Value, "type: none is how a credential is removed")
}

// --- delete --------------------------------------------------------------

func TestDelete(t *testing.T) {
	h := newHarness(t, nil)

	created, err := h.create(t, agentYAML(agentYAMLOpts{}))
	require.NoError(t, err)

	result, err := h.service.Delete(DeleteParams{
		Handle: "weather-agent-v1-0",
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	require.NoError(t, err)
	assert.Equal(t, "weather-agent-v1-0", result.Handle)
	assert.Equal(t, created.StoredConfig.UUID, result.Config.UUID)

	_, err = h.db.GetConfigByKindAndHandle(models.KindAgent, "weather-agent-v1-0")
	assert.Error(t, err)

	assert.Equal(t, []string{"CREATE", "DELETE"}, h.eventHub.actions())
}

func TestDelete_NotFound(t *testing.T) {
	h := newHarness(t, nil)

	_, err := h.service.Delete(DeleteParams{
		Handle: "absent",
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	assert.ErrorIs(t, err, ErrNotFound)
	assert.Empty(t, h.eventHub.actions())
}

// --- control-plane push --------------------------------------------------

// recordingPusher stands in for the control plane client. Submitted tasks are
// recorded rather than run: the deploy-then-push task polls the config store on
// a ticker, so running it here would tie the test to that interval.
type recordingPusher struct {
	connected bool
	onPrem    bool
	submitted []func()
	pushed    []*models.StoredConfig
}

func (p *recordingPusher) IsConnected() bool { return p.connected }
func (p *recordingPusher) IsOnPrem() bool    { return p.onPrem }

func (p *recordingPusher) SubmitArtifactPush(task func()) {
	p.submitted = append(p.submitted, task)
}

// runSubmitted runs the recorded tasks. Only safe for tasks that do not wait for
// a deployment to land — the deploy-then-push task polls the config store until
// it times out, which is minutes.
func (p *recordingPusher) runSubmitted() {
	for _, task := range p.submitted {
		task()
	}
}

func (p *recordingPusher) PushArtifact(_ string, artifact *models.StoredConfig, _ string) error {
	p.pushed = append(p.pushed, artifact)
	return nil
}

// TestControlPlanePushIsOffByDefault is the state this gateway actually ships
// in: the push path exists but ControlPlanePushSupported is false, so nothing is
// pushed and no row claims a sync is pending.
func TestControlPlanePushIsOffByDefault(t *testing.T) {
	require.False(t, ControlPlanePushSupported,
		"flip this test's expectations together with the constant")

	h := newHarness(t, nil)
	pusher := &recordingPusher{connected: true}
	h.service.SetControlPlanePusher(pusher,
		ControlPlanePushSupported && true /* deployment sync enabled */)

	created, err := h.create(t, agentYAML(agentYAMLOpts{}))
	require.NoError(t, err)
	assert.Equal(t, models.CPSyncStatus(""), created.StoredConfig.CPSyncStatus,
		`an artifact that will never be pushed must not be recorded as "pending"`)
	assert.Empty(t, pusher.submitted, "nothing may be pushed while the control plane cannot model an Agent")

	_, err = h.service.Delete(DeleteParams{Handle: "weather-agent-v1-0", Logger: discardLogger()})
	require.NoError(t, err)
	assert.Empty(t, pusher.submitted)
	assert.Empty(t, pusher.pushed)
}

// TestControlPlanePushWhenEnabled exercises the wiring that
// ControlPlanePushSupported currently gates, so enabling it is a one-line change
// against a tested path rather than against never-run code.
func TestControlPlanePushWhenEnabled(t *testing.T) {
	h := newHarness(t, nil)
	pusher := &recordingPusher{connected: true}
	h.service.SetControlPlanePusher(pusher, true)

	created, err := h.create(t, agentYAML(agentYAMLOpts{}))
	require.NoError(t, err)
	assert.Equal(t, models.CPSyncStatusPending, created.StoredConfig.CPSyncStatus)
	require.Len(t, pusher.submitted, 1, "a gateway-originated create should schedule a push")

	// The scheduled task polls the store for a completed deployment, so it is
	// recorded rather than run and nothing has been pushed yet.
	assert.Empty(t, pusher.pushed)
	pusher.submitted = nil

	// A delete reports the artifact as undeployed without waiting for a
	// deployment, so running that task pushes immediately.
	_, err = h.service.Delete(DeleteParams{Handle: "weather-agent-v1-0", Logger: discardLogger()})
	require.NoError(t, err)
	require.Len(t, pusher.submitted, 1)
	pusher.runSubmitted()
	require.Len(t, pusher.pushed, 1)
	assert.Equal(t, models.StateUndeployed, pusher.pushed[0].DesiredState)
	assert.Equal(t, created.StoredConfig.UUID, pusher.pushed[0].UUID)
}

// TestControlPlanePushSkipsNonGatewayOrigin guards the loop that would otherwise
// exist: an artifact the control plane sent us must not be pushed back to it.
func TestControlPlanePushSkipsNonGatewayOrigin(t *testing.T) {
	h := newHarness(t, nil)
	pusher := &recordingPusher{connected: true}
	h.service.SetControlPlanePusher(pusher, true)

	created, err := h.service.Create(CreateParams{
		Body:        agentYAML(agentYAMLOpts{}),
		ContentType: "application/yaml",
		Origin:      models.OriginControlPlane,
		Logger:      discardLogger(),
	})
	require.NoError(t, err)
	assert.Equal(t, models.CPSyncStatus(""), created.StoredConfig.CPSyncStatus)
	assert.Empty(t, pusher.submitted)

	_, err = h.service.Delete(DeleteParams{Handle: "weather-agent-v1-0", Logger: discardLogger()})
	require.NoError(t, err)
	assert.Empty(t, pusher.submitted)
	assert.Empty(t, pusher.pushed)
}

func TestControlPlanePushSkipsDisconnectedAndOnPrem(t *testing.T) {
	for name, pusher := range map[string]*recordingPusher{
		"disconnected": {connected: false},
		"on-prem":      {connected: true, onPrem: true},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, nil)
			h.service.SetControlPlanePusher(pusher, true)

			_, err := h.create(t, agentYAML(agentYAMLOpts{}))
			require.NoError(t, err)
			assert.Empty(t, pusher.submitted)
		})
	}
}

// --- construction guards -------------------------------------------------

func TestNewAgentService_RequiresDependencies(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := &recordingEventHub{}
	validator := config.NewAgentValidator()

	// A service missing any of these would fail at the first request instead of
	// at startup, which is the wrong place to find out.
	assert.Panics(t, func() {
		NewAgentService(nil, nil, nil, validator, logger, hub, nil, testGatewayID)
	})
	assert.Panics(t, func() {
		h := newHarness(t, nil)
		NewAgentService(nil, h.db, nil, validator, logger, nil, nil, testGatewayID)
	})
	assert.Panics(t, func() {
		h := newHarness(t, nil)
		NewAgentService(nil, h.db, nil, nil, logger, hub, nil, testGatewayID)
	})
	assert.Panics(t, func() {
		h := newHarness(t, nil)
		NewAgentService(nil, h.db, nil, validator, logger, hub, nil, "   ")
	})
}
