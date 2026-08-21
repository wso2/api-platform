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

package immutable

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	agentsvc "github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/agent"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/restapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

const agentArtifact = `apiVersion: gateway.api-platform.wso2.com/v1
kind: Agent
metadata:
  name: weather-agent-v1-0
spec:
  displayName: Weather Agent
  version: v1.0
  context: /weather
  upstream:
    url: https://weather.internal
  a2a:
    protocolVersion: "1.0"
    operationConfigs:
      transports:
        - protocolBinding: JSONRPC
          pathPrefix: /rpc
    agentCard:
      public:
        mode: managed
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
`

// discardEventHub satisfies the EventHub dependency without recording anything;
// file-mode loading is asserted through storage, not through events.
type discardEventHub struct{}

func (discardEventHub) Initialize() error                               { return nil }
func (discardEventHub) RegisterGateway(string) error                    { return nil }
func (discardEventHub) PublishEvent(string, eventhub.Event) error       { return nil }
func (discardEventHub) Subscribe(string) (<-chan eventhub.Event, error) { return nil, nil }
func (discardEventHub) Unsubscribe(string, <-chan eventhub.Event) error { return nil }
func (discardEventHub) UnsubscribeAll(string) error                     { return nil }
func (discardEventHub) CleanUpEvents() error                            { return nil }
func (discardEventHub) Close() error                                    { return nil }

// newAgentOnlyGW builds an ImmutableGW wired with just the Agent service, which
// is all the Agent path needs. The other services stay nil so a test that
// accidentally routed an Agent elsewhere would panic rather than pass.
func newAgentOnlyGW(t *testing.T, artifactsDir string) (*ImmutableGW, storage.Storage) {
	t.Helper()
	metrics.Init()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := storage.NewStorage(storage.BackendConfig{
		Type:       "sqlite",
		SQLitePath: filepath.Join(t.TempDir(), "immutable-agent.db"),
		GatewayID:  "test-gateway",
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	service := agentsvc.NewAgentService(
		storage.NewConfigStore(), db, config.NewParser(), config.NewAgentValidator(),
		logger, discardEventHub{}, nil, "test-gateway",
	)

	// NewImmutableGW refuses to build without a RestAPIService, so supply a real
	// one; only the Agent path is exercised below.
	routerCfg := &config.RouterConfig{GatewayHost: "localhost"}
	systemCfg := &config.Config{
		Controller: config.Controller{Server: config.ServerConfig{GatewayID: "test-gateway"}},
	}
	deploymentService := utils.NewAPIDeploymentService(
		nil, db, nil, config.NewAPIValidator(), routerCfg, discardEventHub{}, "test-gateway", nil)
	restService := restapi.NewRestAPIService(
		nil, db, nil, nil, deploymentService, nil, nil,
		routerCfg, systemCfg, nil, config.NewParser(), config.NewAPIValidator(),
		logger, discardEventHub{}, nil,
	)

	gw := NewImmutableGW(
		config.ImmutableGatewayConfig{Enabled: true, ArtifactsDir: artifactsDir},
		restService, nil, nil, service,
	)
	return gw, db
}

func writeArtifact(t *testing.T, dir, name, contents string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600))
}

func TestLoadArtifacts_AppliesAgent(t *testing.T) {
	dir := t.TempDir()
	writeArtifact(t, dir, "weather-agent.yaml", agentArtifact)

	gw, db := newAgentOnlyGW(t, dir)

	require.NoError(t, gw.LoadArtifacts(slog.New(slog.NewTextHandler(io.Discard, nil))))

	stored, err := db.GetConfigByKindAndHandle(models.KindAgent, "weather-agent-v1-0")
	require.NoError(t, err)
	assert.Equal(t, "Weather Agent", stored.DisplayName)
	assert.Equal(t, "v1.0", stored.Version)

	cfg, ok := stored.Configuration.(api.AgentConfiguration)
	require.True(t, ok, "Configuration is %T", stored.Configuration)
	require.Len(t, cfg.Spec.A2a.OperationConfigs.Transports, 1)
}

// TestLoadArtifacts_UnknownKindStillHardErrors guards the reason the loader had
// to change at all: it rejects any kind it does not bucket, so adding the Agent
// kind without teaching this loader about it would have broken every file-mode
// gateway, whether or not it deployed an Agent.
func TestLoadArtifacts_UnknownKindStillHardErrors(t *testing.T) {
	dir := t.TempDir()
	writeArtifact(t, dir, "unknown.yaml", `apiVersion: gateway.api-platform.wso2.com/v1
kind: NotARealKind
metadata:
  name: whatever
spec: {}
`)

	gw, _ := newAgentOnlyGW(t, dir)

	err := gw.LoadArtifacts(slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported kind")
}

// TestLoadArtifacts_MalformedAgentFailsStartup asserts a bad Agent artifact
// surfaces as an error from LoadArtifacts, which main.go treats as fatal. The
// loader reports every failure rather than the first, so one bad file does not
// hide another.
func TestLoadArtifacts_MalformedAgentFailsStartup(t *testing.T) {
	dir := t.TempDir()
	// Structurally an Agent, but with a version the validator rejects.
	writeArtifact(t, dir, "bad-agent.yaml",
		replaceFirst(agentArtifact, "version: v1.0", "version: nonsense"))
	// A second, independent failure: an Agent Card feature that is not
	// supported yet.
	writeArtifact(t, dir, "signed-agent.yaml", replaceFirst(
		replaceFirst(agentArtifact, "name: weather-agent-v1-0", "name: signed-agent-v1-0"),
		"        content: {",
		"        signing:\n          enabled: true\n        content: {"))

	gw, db := newAgentOnlyGW(t, dir)

	err := gw.LoadArtifacts(slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad-agent.yaml")
	assert.Contains(t, err.Error(), "signed-agent.yaml")

	// Neither artifact may be left half-applied.
	_, err = db.GetConfigByKindAndHandle(models.KindAgent, "weather-agent-v1-0")
	assert.Error(t, err)
	_, err = db.GetConfigByKindAndHandle(models.KindAgent, "signed-agent-v1-0")
	assert.Error(t, err)
}

// replaceFirst is strings.Replace with a count of one, named for what the
// fixtures use it for: varying exactly one line of a known-good artifact.
func replaceFirst(s, old, new string) string {
	return strings.Replace(s, old, new, 1)
}
