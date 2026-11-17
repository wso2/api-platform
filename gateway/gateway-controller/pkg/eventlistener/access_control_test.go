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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/upstreamguard"
	"github.com/wso2/api-platform/httpkit/netguard"
)

// useAccessControl installs network access control with private networks
// denied, as main does, for the duration of the test.
func useAccessControl(t *testing.T) {
	t.Helper()
	policy, err := netguard.GuardConfig{Enabled: true, DenyPrivateNetworks: true}.Build()
	require.NoError(t, err)
	templateengine.SetPostRenderCheck(upstreamguard.New(policy, nil, newTestLogger()).CheckArtifact)
	t.Cleanup(func() { templateengine.SetPostRenderCheck(nil) })
}

func accessControlListener(t *testing.T, store *storage.ConfigStore, db storage.Storage) *EventListener {
	t.Helper()
	logger := newTestLogger()
	policyManager := policyxds.NewPolicyManager(policyxds.NewSnapshotManager(logger), logger)
	policyManager.SetRuntimeStore(storage.NewRuntimeConfigStore())
	return &EventListener{
		store:         store,
		db:            db,
		policyManager: policyManager,
		routerConfig: &config.RouterConfig{
			ListenerPort: 8080,
			GatewayHost:  "gateway.example.com",
			VHosts:       config.VHostsConfig{Main: config.VHostEntry{Default: "api.example.com"}},
		},
		systemConfig: &config.Config{},
		// The proxy transform attaches set-headers to every operation.
		policyDefinitions: map[string]models.PolicyDefinition{
			"set-headers-v1.0.0": {Name: "set-headers", Version: "v1.0.0"},
		},
		logger: logger,
	}
}

func desiredState(t *testing.T, store *storage.ConfigStore, id string) models.DesiredState {
	t.Helper()
	stored, err := store.Get(id)
	require.NoError(t, err)
	return stored.DesiredState
}

// An LLM provider whose upstream access control refuses is undeployed on the
// replica that processes its event; one with a permitted upstream deploys.
func TestHandleEvent_LLMProviderWithRefusedUpstream_IsUndeployed(t *testing.T) {
	useAccessControl(t)
	store := storage.NewConfigStore()
	template := testLLMProviderTemplate("tmpl-ac", "openai")
	require.NoError(t, store.AddTemplate(template))
	db := setupSQLiteDBForEventListenerTests(t)
	require.NoError(t, db.SaveLLMProviderTemplate(template))

	refused := testLLMProviderStoredConfig("provider-refused", "provider-refused", "openai", nil)
	src := refused.SourceConfiguration.(api.LLMProviderConfiguration)
	src.Spec.Upstream.Url = stringPtr("http://169.254.169.254/v1")
	refused.SourceConfiguration = src
	require.NoError(t, db.SaveConfig(refused))

	permitted := testLLMProviderStoredConfig("provider-permitted", "provider-permitted", "openai", nil)
	permitted.Version = "v2.0.0"
	permittedSrc := permitted.SourceConfiguration.(api.LLMProviderConfiguration)
	permittedSrc.Spec.Version = "v2.0.0"
	permittedSrc.Spec.Context = stringPtr("/llm-permitted")
	permitted.SourceConfiguration = permittedSrc
	require.NoError(t, db.SaveConfig(permitted))

	listener := accessControlListener(t, store, db)
	for _, id := range []string{refused.UUID, permitted.UUID} {
		listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeLLMProvider, Action: "CREATE", EntityID: id, EventID: "corr-" + id})
	}

	assert.Equal(t, models.StateUndeployed, desiredState(t, store, refused.UUID))
	assert.Equal(t, models.StateDeployed, desiredState(t, store, permitted.UUID), "positive control")
}

// The RestAPI hydrated from an LLM proxy routes to its provider through the
// gateway's own loopback listener. With private networks denied, that generated
// routing must not get the proxy refused.
func TestHandleEvent_LLMProxyLoopbackRoutingIsNotRefused(t *testing.T) {
	useAccessControl(t)
	store := storage.NewConfigStore()
	template := testLLMProviderTemplate("tmpl-ac", "openai")
	require.NoError(t, store.AddTemplate(template))
	provider := testLLMProviderStoredConfig("provider-1", "provider-a", "openai", nil)
	require.NoError(t, store.Add(provider))

	db := setupSQLiteDBForEventListenerTests(t)
	require.NoError(t, db.SaveLLMProviderTemplate(template))
	require.NoError(t, db.SaveConfig(provider))
	proxy := testLLMProxyStoredConfig("llm-proxy-ac", "proxy-ac", "provider-a", nil)
	require.NoError(t, db.SaveConfig(proxy))

	listener := accessControlListener(t, store, db)
	listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeLLMProxy, Action: "CREATE", EntityID: proxy.UUID, EventID: "corr-proxy-ac"})

	stored, err := store.Get(proxy.UUID)
	require.NoError(t, err)
	assert.Equal(t, models.StateDeployed, stored.DesiredState)
	restAPI, ok := stored.Configuration.(api.RestAPI)
	require.True(t, ok, "the proxy is stored in its derived RestAPI form")
	require.NotNil(t, restAPI.Spec.Upstream.Main.Url)
	assert.Contains(t, *restAPI.Spec.Upstream.Main.Url, "127.0.0.1", "precondition: the derived upstream is the loopback listener")
}

// An LLM proxy the gateway configuration refuses is undeployed on the replica.
func TestHandleEvent_LLMProxyRefused_IsUndeployed(t *testing.T) {
	templateengine.SetPostRenderCheck(func(kind string, _ any) error {
		if kind == string(models.KindLlmProxy) {
			return &upstreamguard.RefusedError{FieldPath: "spec.policies[0].params.endpoint"}
		}
		return nil
	})
	t.Cleanup(func() { templateengine.SetPostRenderCheck(nil) })

	store := storage.NewConfigStore()
	template := testLLMProviderTemplate("tmpl-ac", "openai")
	require.NoError(t, store.AddTemplate(template))
	provider := testLLMProviderStoredConfig("provider-1", "provider-a", "openai", nil)
	require.NoError(t, store.Add(provider))
	db := setupSQLiteDBForEventListenerTests(t)
	require.NoError(t, db.SaveLLMProviderTemplate(template))
	require.NoError(t, db.SaveConfig(provider))
	proxy := testLLMProxyStoredConfig("llm-proxy-refused", "proxy-refused", "provider-a", nil)
	require.NoError(t, db.SaveConfig(proxy))

	listener := accessControlListener(t, store, db)
	listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeLLMProxy, Action: "CREATE", EntityID: proxy.UUID, EventID: "corr-proxy-refused"})

	assert.Equal(t, models.StateUndeployed, desiredState(t, store, proxy.UUID))
}

// An MCP proxy whose upstream access control refuses is undeployed; one with a
// permitted upstream deploys.
func TestHandleEvent_MCPWithRefusedUpstream_IsUndeployed(t *testing.T) {
	useAccessControl(t)
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)

	refused := testMCPStoredConfig("mcp-refused", "mcp-refused", "Refused MCP", "v1.0", models.StateDeployed, nil)
	src := refused.SourceConfiguration.(api.MCPProxyConfiguration)
	metadataURL := "http://169.254.169.254/mcp"
	src.Spec.Upstream.Url = &metadataURL
	refused.SourceConfiguration = src
	require.NoError(t, db.SaveConfig(refused))

	permitted := testMCPStoredConfig("mcp-permitted", "mcp-permitted", "Permitted MCP", "v1.0", models.StateDeployed, nil)
	require.NoError(t, db.SaveConfig(permitted))

	listener := accessControlListener(t, store, db)
	for _, id := range []string{refused.UUID, permitted.UUID} {
		listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeMCPProxy, Action: "CREATE", EntityID: id, EventID: "corr-" + id})
	}

	assert.Equal(t, models.StateUndeployed, desiredState(t, store, refused.UUID))
	assert.Equal(t, models.StateDeployed, desiredState(t, store, permitted.UUID), "positive control")
}
