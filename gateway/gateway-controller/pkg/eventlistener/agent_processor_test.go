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

package eventlistener

import (
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/agentproto"
	"github.com/wso2/api-platform/common/chainkey"
	commonconstants "github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/transform"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// agentTestVHost is the routing partition every Agent fixture below lands on.
const agentTestVHost = "agents.example.com"

// stubSecretResolver resolves every handle to one value, so a test can tell a
// rendered configuration apart from the template it was rendered from.
type stubSecretResolver struct {
	value string
}

func (r stubSecretResolver) Resolve(string) (string, error) { return r.value, nil }

// agentTestPolicyDefinitions is the policy catalogue the replica-side transform
// runs against: the A2A system policy (without it no managed card can be served,
// so every fixture would fail to transform) plus a rate limit whose `limit` is a
// declared integer, which is what makes the coercion step observable.
//
// Keys are `name|version`, the form the coercion lookup uses. The transformer
// only reads the values, so one map serves both.
func agentTestPolicyDefinitions() map[string]models.PolicyDefinition {
	limitParams := map[string]any{
		"properties": map[string]any{
			"limit": map[string]any{"type": "integer"},
		},
	}
	return map[string]models.PolicyDefinition{
		constants.A2A_SYSTEM_POLICY_NAME + "|v1.0.0": {
			Name:    constants.A2A_SYSTEM_POLICY_NAME,
			Version: "v1.0.0",
		},
		"rate-limit|v1.0.0": {
			Name:       "rate-limit",
			Version:    "v1.0.0",
			Parameters: &limitParams,
		},
	}
}

// agentTestRouterConfig is complete enough for the Envoy translator to build a
// real listener from the transformed Agent, not just for the transform to run —
// the snapshot version only moves when translation actually succeeds, and that
// is what these tests assert on.
func agentTestRouterConfig() *config.RouterConfig {
	return &config.RouterConfig{
		ListenerPort: 8080,
		GatewayHost:  "gw.local",
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: agentTestVHost},
			Sandbox: config.VHostEntry{Default: "sandbox.example.com"},
		},
		Upstream: config.RouterUpstream{
			TLS: config.UpstreamTLS{
				MinimumProtocolVersion: constants.TLSVersion12,
				MaximumProtocolVersion: constants.TLSVersion13,
				DisableSslVerification: true,
			},
			Timeouts: config.UpstreamTimeouts{
				RouteTimeoutMs:     60000,
				RouteIdleTimeoutMs: 300000,
				ConnectTimeoutMs:   5000,
			},
		},
		AccessLogs: config.AccessLogsConfig{Enabled: false},
		HTTPListener: config.HTTPListenerConfig{
			ServerHeaderTransformation:    commonconstants.OVERWRITE,
			PerConnectionBufferLimitBytes: 1048576,
			PathWithEscapedSlashesAction:  commonconstants.KEEP_UNCHANGED,
		},
		LuaScriptPath: "../../lua/request_transformation.lua",
	}
}

// agentStoredConfigOption spoils one field of the Agent fixture.
type agentStoredConfigOption func(*api.AgentConfiguration)

// withAgentOperationPolicy attaches one policy to every canonical operation.
func withAgentOperationPolicy(policy api.Policy) agentStoredConfigOption {
	return func(cfg *api.AgentConfiguration) {
		policies := []api.Policy{policy}
		cfg.Spec.A2a.OperationConfigs.Policies = &policies
	}
}

// withManagedProtectedCard attaches a managed protected Agent Card carrying a
// skill the public card does not, so the two documents are separable in an
// assertion.
func withManagedProtectedCard() agentStoredConfigOption {
	return func(cfg *api.AgentConfiguration) {
		content := api.A2AAgentCardDocument{
			"name":            cfg.Spec.DisplayName,
			"protocolVersion": "1.0",
			"skills":          []interface{}{map[string]interface{}{"id": "forecast_history"}},
		}
		cfg.Spec.A2a.AgentCard.Protected = &api.A2AProtectedAgentCard{
			Mode:    api.A2AProtectedAgentCardModeManaged,
			Content: &content,
		}
	}
}

// testAgentStoredConfig builds a deployable Agent: both transports, a managed
// public card, on the default vhost.
func testAgentStoredConfig(
	uuid, handle, displayName, version string,
	desiredState models.DesiredState,
	options ...agentStoredConfigOption,
) *models.StoredConfig {
	upstreamURL := "https://weather.internal/a2a"
	cardContent := api.A2AAgentCardDocument{
		"name":            displayName,
		"protocolVersion": "1.0",
	}
	agentConfig := api.AgentConfiguration{
		ApiVersion: api.AgentConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.AgentConfigurationKindAgent,
		Metadata:   api.Metadata{Name: handle},
		Spec: api.AgentConfigData{
			DisplayName: displayName,
			Version:     version,
			Context:     stringPtr("/" + handle),
			Upstream:    api.AgentConfigData_Upstream{Url: &upstreamURL},
			A2a: api.A2AConfig{
				ProtocolVersion: "1.0",
				OperationConfigs: api.A2AOperationConfigs{
					Transports: []api.A2ATransport{
						{ProtocolBinding: api.JSONRPC, PathPrefix: stringPtr("/rpc")},
						{ProtocolBinding: api.HTTPJSON, PathPrefix: stringPtr("/")},
					},
				},
				AgentCard: api.A2AAgentCard{
					Public: api.A2APublicAgentCard{
						Mode:    api.A2APublicAgentCardModeManaged,
						Content: &cardContent,
					},
				},
			},
		},
	}
	for _, option := range options {
		option(&agentConfig)
	}

	now := time.Now()
	return &models.StoredConfig{
		UUID:                uuid,
		Kind:                models.KindAgent,
		Handle:              handle,
		DisplayName:         displayName,
		Version:             version,
		Configuration:       agentConfig,
		SourceConfiguration: agentConfig,
		DesiredState:        desiredState,
		Origin:              models.OriginGatewayAPI,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

// agentReplica is the second replica under test: its own in-memory stores and
// snapshot managers, fed only by events.
type agentReplica struct {
	listener     *EventListener
	store        *storage.ConfigStore
	runtimeStore *storage.RuntimeConfigStore
}

// newAgentReplica wires a listener the way cmd/controller does for the Agent
// kind: both xDS lanes present (Envoy snapshot and policy snapshot) and the
// transformer registry dispatching Agent to the Agent transformer.
func newAgentReplica(t *testing.T, db storage.Storage) *agentReplica {
	t.Helper()

	logger := newTestLogger()
	routerConfig := agentTestRouterConfig()
	systemConfig := &config.Config{Router: *routerConfig}
	policyDefinitions := agentTestPolicyDefinitions()

	store := storage.NewConfigStore()
	runtimeStore := storage.NewRuntimeConfigStore()

	registry := transform.NewRegistry(
		nil, nil,
		transform.NewAgentTransformer(routerConfig, systemConfig, policyDefinitions),
	)

	policySnapshotManager := policyxds.NewSnapshotManager(logger)
	policySnapshotManager.SetRuntimeStore(runtimeStore)
	policyManager := policyxds.NewPolicyManager(policySnapshotManager, logger)
	policyManager.SetRuntimeStore(runtimeStore)
	policyManager.SetTransformers(registry)

	snapshotManager := xds.NewSnapshotManager(store, logger, routerConfig, db, systemConfig)
	snapshotManager.GetTranslator().SetTransformers(map[string]models.ConfigTransformer{
		models.KindAgent: registry,
	})

	return &agentReplica{
		listener: &EventListener{
			store:             store,
			db:                db,
			snapshotManager:   snapshotManager,
			policyManager:     policyManager,
			routerConfig:      routerConfig,
			systemConfig:      systemConfig,
			policyDefinitions: policyDefinitions,
			policyValidator:   config.NewPolicyValidator(policyDefinitions),
			secretResolver:    stubSecretResolver{value: "42"},
			logger:            logger,
		},
		store:        store,
		runtimeStore: runtimeStore,
	}
}

func agentEvent(action, entityID, eventID string) eventhub.Event {
	return eventhub.Event{
		EventType: eventhub.EventTypeAgent,
		Action:    action,
		EntityID:  entityID,
		EventID:   eventID,
		EventData: eventhub.EmptyEventData,
	}
}

// operationChainCount counts the composed (per-operation) chains, so the count
// is about canonical operations and not about the card or preflight routes.
func operationChainCount(rdc *models.RuntimeDeployConfig) int {
	count := 0
	for key := range rdc.PolicyChains {
		if chainkey.IsComposed(key) {
			count++
		}
	}
	return count
}

// An Agent deployed via replica 1 becomes visible to replica 2: the artifact
// reaches the ConfigStore, its chains reach the RuntimeConfigStore, and both xDS
// snapshot versions move.
func TestHandleEvent_AgentCreate_ConvergesSecondReplica(t *testing.T) {
	db := setupSQLiteDBForEventListenerTests(t)
	cfg := testAgentStoredConfig("agent-create-id", "weather-agent", "Weather Agent", "v1.0", models.StateDeployed)
	require.NoError(t, db.SaveConfig(cfg))

	replica := newAgentReplica(t, db)
	snapshotVersionBefore := replica.store.GetSnapshotVersion()
	policyVersionBefore := replica.runtimeStore.GetResourceVersion()

	replica.listener.handleEvent(agentEvent("CREATE", cfg.UUID, "corr-agent-create"))

	stored, err := replica.store.Get(cfg.UUID)
	require.NoError(t, err)
	assert.Equal(t, models.KindAgent, stored.Kind)
	assert.Equal(t, models.StateDeployed, stored.DesiredState)

	// R6: Configuration must arrive non-nil on every read path — the Agent lane
	// has no hydrate step to rebuild it later.
	require.NotNil(t, stored.Configuration)
	_, ok := stored.Configuration.(api.AgentConfiguration)
	assert.True(t, ok, "Configuration should be an api.AgentConfiguration, got %T", stored.Configuration)

	rdc, exists := replica.runtimeStore.Get(storage.Key(models.KindAgent, cfg.Handle))
	require.True(t, exists, "the Agent's runtime deploy config should reach the policy store")
	assert.Equal(t, models.KindAgent, rdc.Metadata.Kind)
	assert.Equal(t, 11, operationChainCount(rdc),
		"one canonical chain per 1.0 operation on the single routing partition")
	assert.NotEmpty(t, rdc.Routes)

	assert.Greater(t, replica.store.GetSnapshotVersion(), snapshotVersionBefore,
		"the Envoy snapshot version should increment")
	assert.Greater(t, replica.runtimeStore.GetResourceVersion(), policyVersionBefore,
		"the policy xDS resource version should increment")
}

// A managed protected Agent Card survives the event path. It is carried inside a
// canonical operation chain rather than on a route of its own, so a replica that
// rebuilt every route and every route-keyed chain would still be missing it —
// and the failure is silent: the operation comes back proxying to the agent,
// serving the upstream's own extended card unguarded in place of the gateway's.
func TestHandleEvent_AgentCreate_ConvergesManagedProtectedCard(t *testing.T) {
	db := setupSQLiteDBForEventListenerTests(t)
	cfg := testAgentStoredConfig("agent-protected-id", "protected-agent", "Protected Agent", "v1.0",
		models.StateDeployed, withManagedProtectedCard())
	require.NoError(t, db.SaveConfig(cfg))

	replica := newAgentReplica(t, db)
	replica.listener.handleEvent(agentEvent("CREATE", cfg.UUID, "corr-agent-protected"))

	rdc, exists := replica.runtimeStore.Get(storage.Key(models.KindAgent, cfg.Handle))
	require.True(t, exists)
	assert.Equal(t, 11, operationChainCount(rdc),
		"a protected card adds policies to an existing chain, never a chain of its own")

	chain, ok := rdc.PolicyChains[rdc.ChainKeyFor(agentTestVHost, string(agentproto.GetExtendedAgentCard))]
	require.True(t, ok, "the extended-card chain should exist; have %v", chainKeysOf(rdc))

	block := protectedCardBlockOf(t, chain)
	content, ok := block[constants.A2A_POLICY_PARAM_CONTENT].(string)
	require.True(t, ok, "the protected card content should reach the replica as serialized bytes")
	assert.Contains(t, content, "forecast_history")

	// The bytes are the supplied document, unchanged by the round trip through
	// storage and the event path. Card signing will sign exactly these, so a
	// re-serialization anywhere between here and the runtime is a signature that
	// does not verify.
	expected, err := json.Marshal(map[string]interface{}(
		*cfg.Configuration.(api.AgentConfiguration).Spec.A2a.AgentCard.Protected.Content))
	require.NoError(t, err)
	assert.Equal(t, string(expected), content)

	// Only that one chain. An instance anywhere else would guard, or leak, an
	// operation the author did not configure.
	for key, other := range rdc.PolicyChains {
		if key == rdc.ChainKeyFor(agentTestVHost, string(agentproto.GetExtendedAgentCard)) {
			continue
		}
		for _, policy := range other.Policies {
			if policy.Name != constants.A2A_SYSTEM_POLICY_NAME {
				continue
			}
			assert.NotContains(t, policy.Params, constants.A2A_POLICY_PARAM_PROTECTED_AGENT_CARD,
				"chain %q carries a protected-card instance", key)
		}
	}
}

// chainKeysOf renders a config's chain keys for a failure message.
func chainKeysOf(rdc *models.RuntimeDeployConfig) []string {
	keys := make([]string, 0, len(rdc.PolicyChains))
	for key := range rdc.PolicyChains {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// protectedCardBlockOf returns the protectedAgentCard parameter block of the A2A
// system policy instance in a chain, failing the test if there is none.
func protectedCardBlockOf(t *testing.T, chain *models.PolicyChain) map[string]interface{} {
	t.Helper()
	require.NotNil(t, chain)
	for _, policy := range chain.Policies {
		if policy.Name != constants.A2A_SYSTEM_POLICY_NAME {
			continue
		}
		if block, ok := policy.Params[constants.A2A_POLICY_PARAM_PROTECTED_AGENT_CARD].(map[string]interface{}); ok {
			return block
		}
	}
	t.Fatalf("the chain carries no protected-card instance")
	return nil
}

// An update is read back from the database rather than taken from the event, so
// a replica holding a stale copy converges on the stored one.
func TestHandleEvent_AgentUpdate_RefreshesStaleConfigFromDB(t *testing.T) {
	db := setupSQLiteDBForEventListenerTests(t)

	latest := testAgentStoredConfig("agent-update-id", "weather-agent", "Weather Agent", "v1.0", models.StateUndeployed)
	require.NoError(t, db.SaveConfig(latest))

	replica := newAgentReplica(t, db)
	stale := testAgentStoredConfig("agent-update-id", "weather-agent", "Weather Agent", "v1.0", models.StateDeployed)
	require.NoError(t, replica.store.Add(stale))

	replica.listener.handleEvent(agentEvent("UPDATE", latest.UUID, "corr-agent-update"))

	stored, err := replica.store.Get(latest.UUID)
	require.NoError(t, err)
	assert.Equal(t, models.StateUndeployed, stored.DesiredState)

	_, exists := replica.runtimeStore.Get(storage.Key(models.KindAgent, latest.Handle))
	assert.True(t, exists, "an undeployed Agent still holds its chains for redeployment")
}

// Delete removes both halves of the Agent's local state. Leaving the chains
// behind would strand them: nothing names their kind/handle any more, so nothing
// would ever remove them.
func TestHandleEvent_AgentDelete_RemovesConfigAndPolicyChains(t *testing.T) {
	db := setupSQLiteDBForEventListenerTests(t)
	cfg := testAgentStoredConfig("agent-delete-id", "weather-agent", "Weather Agent", "v1.0", models.StateDeployed)
	require.NoError(t, db.SaveConfig(cfg))

	replica := newAgentReplica(t, db)
	replica.listener.handleEvent(agentEvent("CREATE", cfg.UUID, "corr-agent-create"))

	runtimeKey := storage.Key(models.KindAgent, cfg.Handle)
	_, exists := replica.runtimeStore.Get(runtimeKey)
	require.True(t, exists, "precondition: the Agent converged before it is deleted")

	// The row goes first: the deleting replica removes it, then announces.
	require.NoError(t, db.DeleteConfig(cfg.UUID))
	snapshotVersionBefore := replica.store.GetSnapshotVersion()

	replica.listener.handleEvent(agentEvent("DELETE", cfg.UUID, "corr-agent-delete"))

	_, err := replica.store.Get(cfg.UUID)
	require.ErrorIs(t, err, storage.ErrNotFound)

	_, exists = replica.runtimeStore.Get(runtimeKey)
	assert.False(t, exists, "the Agent's runtime deploy config should be gone")

	assert.Greater(t, replica.store.GetSnapshotVersion(), snapshotVersionBefore,
		"the Envoy snapshot version should increment on deletion")
}

// Deleting an Agent this replica never saw is a no-op, not a failure: an event
// can arrive after a restart that rebuilt the stores without it.
func TestHandleEvent_AgentDelete_UnknownAgentIsHarmless(t *testing.T) {
	db := setupSQLiteDBForEventListenerTests(t)
	replica := newAgentReplica(t, db)

	replica.listener.handleEvent(agentEvent("DELETE", "agent-never-seen", "corr-agent-delete-missing"))

	assert.Empty(t, replica.store.GetAll())
	assert.Empty(t, replica.runtimeStore.GetAll())
}

// Render-on-consumption: the database holds the artifact as its author wrote it,
// so each replica resolves template expressions against its own environment.
// The rendered value must reach the store, and the stored source must not.
func TestHandleEvent_AgentCreate_RendersAndCoercesOnConsumption(t *testing.T) {
	db := setupSQLiteDBForEventListenerTests(t)
	cfg := testAgentStoredConfig(
		"agent-render-id", "weather-agent", "Weather Agent", "v1.0", models.StateDeployed,
		withAgentOperationPolicy(api.Policy{
			Name:    "rate-limit",
			Version: "v1",
			Params:  &map[string]any{"limit": `{{ secret "agent-limit" }}`},
		}),
	)
	require.NoError(t, db.SaveConfig(cfg))

	replica := newAgentReplica(t, db)
	replica.listener.handleEvent(agentEvent("CREATE", cfg.UUID, "corr-agent-render"))

	stored, err := replica.store.Get(cfg.UUID)
	require.NoError(t, err)
	rendered, ok := stored.Configuration.(api.AgentConfiguration)
	require.True(t, ok)

	require.NotNil(t, rendered.Spec.A2a.OperationConfigs.Policies)
	renderedPolicies := *rendered.Spec.A2a.OperationConfigs.Policies
	require.Len(t, renderedPolicies, 1)
	require.NotNil(t, renderedPolicies[0].Params)

	// Rendered, then coerced back to the schema-declared type — a rendered
	// string would fail the policy's own schema at the runtime.
	assert.Equal(t, float64(42), (*renderedPolicies[0].Params)["limit"])

	// The stored source still carries the template, so a rotated secret is
	// picked up on the next event without rewriting the artifact.
	fromDB, err := db.GetConfig(cfg.UUID)
	require.NoError(t, err)
	source, ok := fromDB.SourceConfiguration.(api.AgentConfiguration)
	require.True(t, ok)
	require.NotNil(t, source.Spec.A2a.OperationConfigs.Policies)
	storedPolicies := *source.Spec.A2a.OperationConfigs.Policies
	require.Len(t, storedPolicies, 1)
	require.NotNil(t, storedPolicies[0].Params)
	assert.Equal(t, `{{ secret "agent-limit" }}`, (*storedPolicies[0].Params)["limit"])
}

// An entity id is unique across kinds, so an AGENT event naming a row of another
// kind means the two disagree. The row must not be pushed through the Agent lane.
func TestHandleEvent_AgentEvent_SkipsRowOfAnotherKind(t *testing.T) {
	db := setupSQLiteDBForEventListenerTests(t)
	restCfg := testRestStoredConfig("not-an-agent-id", "test-api", "Test API", "v1.0.0", models.StateDeployed)
	require.NoError(t, db.SaveConfig(restCfg))

	replica := newAgentReplica(t, db)

	replica.listener.handleEvent(agentEvent("CREATE", restCfg.UUID, "corr-agent-wrong-kind"))

	_, err := replica.store.Get(restCfg.UUID)
	require.ErrorIs(t, err, storage.ErrNotFound)
	assert.Empty(t, replica.runtimeStore.GetAll())
}

// An unrecognised action changes nothing rather than being read as a create.
func TestHandleEvent_AgentEvent_UnknownActionIsIgnored(t *testing.T) {
	db := setupSQLiteDBForEventListenerTests(t)
	cfg := testAgentStoredConfig("agent-unknown-action-id", "weather-agent", "Weather Agent", "v1.0", models.StateDeployed)
	require.NoError(t, db.SaveConfig(cfg))

	replica := newAgentReplica(t, db)

	replica.listener.handleEvent(agentEvent("REDEPLOY", cfg.UUID, "corr-agent-unknown-action"))

	_, err := replica.store.Get(cfg.UUID)
	require.ErrorIs(t, err, storage.ErrNotFound)
	assert.Empty(t, replica.runtimeStore.GetAll())
}
