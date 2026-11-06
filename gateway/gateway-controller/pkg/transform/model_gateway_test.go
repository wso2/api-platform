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

package transform

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/chainkey"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// modelGatewayFixture is a store holding the default model endpoint, two
// providers — one reached directly, one through its own route — and the Models on
// them.
type modelGatewayFixture struct {
	t        *testing.T
	store    *storage.ConfigStore
	db       storage.Storage
	compiler *ModelGatewayCompiler
	llm      *LLMTransformer
	endpoint *models.StoredConfig
	sysCfg   *config.Config
}

const testListenerPort = 8080

func modelGatewayPolicyDefinitions() map[string]models.PolicyDefinition {
	defs := map[string]models.PolicyDefinition{}
	for name, version := range map[string]string{
		"api-key-auth":                    "v1.2.1",
		"set-headers":                     "v1.0.0",
		"oauth2-generator":                "v1.0.0",
		"wso2_apip_sys_model":             "v1.0.0",
		"openai-to-anthropic-transformer": "v0.9.2",
		"anthropic-to-openai-transformer": "v1.0.0",
		"model-round-robin":               "v1.1.1",
		"semantic-model-routing":          "v1.0.0",
		"token-based-ratelimit":           "v1.0.0",
		"llm-cost":                        "v1.0.0",
	} {
		defs[name+"|"+version] = models.PolicyDefinition{Name: name, Version: version}
	}
	return defs
}

// anthropicEntry is the seeded transformer entry for Anthropic providers.
func anthropicEntry() api.ModelEndpointTransformer {
	return api.ModelEndpointTransformer{
		Template:      ptr("anthropic"),
		Type:          "openai-to-anthropic-transformer",
		Version:       ptr("v0"),
		Params:        &map[string]interface{}{"anthropicVersion": "{apiVersion:2023-06-01}"},
		UpstreamPaths: &[]string{"/v1/messages"},
	}
}

func newModelGatewayFixture(t *testing.T) *modelGatewayFixture {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := storage.NewStorage(storage.BackendConfig{
		Type: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "gw.db"), GatewayID: "gw-1",
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	routerCfg := testRouterCfg()
	routerCfg.ListenerPort = testListenerPort
	routerCfg.VHosts.Main.Default = "*"
	sysCfg := &config.Config{Router: *routerCfg}
	sysCfg.Controller.Models = config.ModelsConfig{
		Endpoint: config.ModelEndpointConfig{Provision: true, Context: "/openai/v1", MaxRequestBodyBytes: 1 << 20}}

	store := storage.NewConfigStore()
	defs := modelGatewayPolicyDefinitions()
	llm := NewLLMTransformer(store, db, routerCfg, sysCfg, defs, utils.NewLoadedPolicyVersionResolver(defs))
	compiler := NewModelGatewayCompiler(store, routerCfg, sysCfg, defs, llm.ProviderTransformer(), nil)

	f := &modelGatewayFixture{t: t, store: store, db: db, compiler: compiler, llm: llm, sysCfg: sysCfg}
	f.addTemplate("openai", "wso2-openai", payloadModel())
	f.addTemplate("anthropic", "wso2-anthropic", payloadModel())
	f.addTemplate("gemini", "wso2-gemini", &api.ExtractionIdentifier{Location: "pathParam", Identifier: `models/([a-zA-Z0-9.\-]+)`})
	f.addProvider("openai-direct", "openai", func(p *api.LLMProviderConfiguration) {
		none := api.LLMProviderConfigDataExposure(models.LLMProviderExposureNone)
		p.Spec.Exposure = &none
		p.Spec.Upstream.Url = ptr("https://api.openai.com/v1")
		p.Spec.Upstream.Auth = &struct {
			Header        *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
			PolicyName    *string                                   `json:"policyName,omitempty" yaml:"policyName,omitempty"`
			PolicyParams  *map[string]interface{}                   `json:"policyParams,omitempty" yaml:"policyParams,omitempty"`
			PolicyVersion *string                                   `json:"policyVersion,omitempty" yaml:"policyVersion,omitempty"`
			Type          api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
			Value         *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
		}{Type: api.LLMProviderConfigDataUpstreamAuthTypeApiKey, Header: ptr("Authorization"), Value: ptr("Bearer sk-direct")}
	})
	f.addProvider("anthropic-routed", "anthropic", func(p *api.LLMProviderConfiguration) {
		p.Spec.Context = ptr("/anthropic")
		p.Spec.Upstream.Url = ptr("https://api.anthropic.com")
		p.Spec.AccessControl = &api.LLMAccessControl{Mode: api.DenyAll, Exceptions: &[]api.RouteException{{
			Path: "/v1/messages", Methods: []api.RouteExceptionMethods{"POST"},
		}}}
		authParams := map[string]interface{}{"key": "x-api-key", "in": "header", "valuePrefix": "Key"}
		p.Spec.GlobalPolicies = &[]api.Policy{{Name: "api-key-auth", Version: "v1", Params: &authParams}}
	})
	f.endpoint = f.addEndpoint(models.DefaultModelEndpointHandle, "", "", nil, anthropicEntry())
	return f
}

func payloadModel() *api.ExtractionIdentifier {
	return &api.ExtractionIdentifier{Location: "payload", Identifier: "$.model"}
}

func ptr[T any](v T) *T { return &v }

func (f *modelGatewayFixture) addTemplate(handle, group string, requestModel *api.ExtractionIdentifier) {
	require.NoError(f.t, f.store.AddTemplate(&models.StoredLLMProviderTemplate{
		UUID: "tmpl-" + handle,
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: handle},
			Spec: api.LLMProviderTemplateData{
				DisplayName:  handle,
				GroupId:      ptr(group),
				RequestModel: requestModel,
			},
		},
	}))
}

func (f *modelGatewayFixture) addProvider(handle, template string, mutate func(*api.LLMProviderConfiguration)) {
	p := api.LLMProviderConfiguration{
		ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProviderConfigurationKindLlmProvider,
		Metadata:   api.Metadata{Name: handle},
		Spec:       api.LLMProviderConfigData{DisplayName: handle, Version: "v1.0", Template: template},
	}
	mutate(&p)
	require.NoError(f.t, f.store.Add(&models.StoredConfig{
		UUID: "uuid-" + handle, Kind: models.KindLlmProvider, Handle: handle, DisplayName: handle, Version: "v1.0",
		SourceConfiguration: p, Configuration: p, DesiredState: models.StateDeployed,
	}))
}

// addEndpoint stores a model endpoint. An empty context is the provisioned
// endpoint's, from configuration; an empty inbound template is openai.
func (f *modelGatewayFixture) addEndpoint(handle, context, inboundTemplate string, policies *[]api.Policy,
	transformers ...api.ModelEndpointTransformer) *models.StoredConfig {
	if policies == nil {
		authParams := map[string]interface{}{"key": "Authorization", "in": "header", "valuePrefix": "Bearer"}
		policies = &[]api.Policy{{Name: "api-key-auth", Version: "v1", Params: &authParams}}
	}
	e := api.ModelEndpointConfiguration{
		ApiVersion: api.ModelEndpointConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.ModelEndpointConfigurationKindModelEndpoint,
		Metadata:   api.Metadata{Name: handle},
		Spec: api.ModelEndpointConfigData{
			DisplayName:  handle + " endpoint",
			Version:      "v1.0",
			Policies:     policies,
			Transformers: &transformers,
		},
	}
	if context != "" {
		e.Spec.Context = ptr(context)
	}
	if inboundTemplate != "" {
		e.Spec.InboundTemplate = ptr(inboundTemplate)
	}
	cfg := &models.StoredConfig{
		UUID: models.ModelEndpointUUID(handle), Kind: models.KindModelEndpoint, Handle: handle,
		DisplayName: e.Spec.DisplayName, Version: e.Spec.Version,
		SourceConfiguration: e, Configuration: e, DesiredState: models.StateDeployed,
	}
	require.NoError(f.t, f.store.Add(cfg))
	return cfg
}

func (f *modelGatewayFixture) addModel(handle, modelName string, targets []api.ModelTarget, routing *api.ModelRouting,
	endpoints ...string) {
	if len(endpoints) == 0 {
		endpoints = []string{models.DefaultModelEndpointHandle}
	}
	m := api.ModelConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1",
		Kind:       "Model",
		Metadata:   api.Metadata{Name: handle},
		Spec: api.ModelConfigData{
			DisplayName: handle, Version: "v1.0", ModelName: modelName, Endpoints: endpoints,
			Targets: targets, Routing: routing,
		},
	}
	require.NoError(f.t, f.store.Add(&models.StoredConfig{
		UUID: "uuid-" + handle, Kind: models.KindModel, Handle: handle, DisplayName: handle, Version: "v1.0",
		SourceConfiguration: m, Configuration: m, DesiredState: models.StateDeployed,
		CreatedAt: time.Unix(1_700_000_000, 0),
	}))
}

func directTarget(name, model string) api.ModelTarget {
	return api.ModelTarget{Name: model, Provider: "openai-direct", Alias: ptr(name)}
}

func routedTarget(name, model string) api.ModelTarget {
	return api.ModelTarget{
		Name: model, Provider: "anthropic-routed", Alias: ptr(name),
		Auth: &api.LLMUpstreamAuth{Type: api.LLMUpstreamAuthTypeApiKey, Header: ptr("x-api-key"), Value: ptr("route-key")},
	}
}

func modelJob(p models.Policy) string {
	for _, job := range []string{"route", "providerScope", "dispatchError", "catalog"} {
		if _, ok := p.Params[job]; ok {
			return job
		}
	}
	return ""
}

func statusesByHandle(statuses []ModelCompileStatus) map[string]ModelCompileStatus {
	out := map[string]ModelCompileStatus{}
	for _, s := range statuses {
		out[s.Handle] = s
	}
	return out
}

type resolverConfig struct {
	Models         map[string]string `json:"models"`
	ErrorOperation string            `json:"errorOperation"`
	ModelLocation  map[string]string `json:"modelLocation"`
	Stream         map[string]string `json:"stream"`
}

func resolverConfigOf(t *testing.T, route *models.Route) resolverConfig {
	t.Helper()
	require.NotNil(t, route)
	var cfg resolverConfig
	require.NoError(t, json.Unmarshal(route.ResolverConfig, &cfg))
	return cfg
}

func TestModelGateway_CompilesRoutesChainsAndClusters(t *testing.T) {
	f := newModelGatewayFixture(t)
	f.addModel("gpt", "gpt-4o", []api.ModelTarget{directTarget("openai", "gpt-4o")}, nil)
	f.addModel("claude", "claude", []api.ModelTarget{routedTarget("anthropic", "claude-sonnet")}, nil)

	rdc, statuses, err := f.compiler.Compile(f.endpoint)
	require.NoError(t, err)
	for _, s := range statuses {
		assert.True(t, s.Served, "model %s not served: %s", s.Handle, s.Reason)
	}

	uuid := models.ModelEndpointUUID(models.DefaultModelEndpointHandle)
	assert.Equal(t, models.KindLlmProxy, rdc.Metadata.Kind, "the runtime kind stays LlmProxy")
	assert.Equal(t, uuid, rdc.Metadata.UUID)
	require.NotNil(t, rdc.Metadata.LLM)
	assert.Equal(t, "openai", rdc.Metadata.LLM.TemplateHandle)
	assert.Equal(t, models.ModelEndpointProviderName("openai"), rdc.Metadata.LLM.ProviderName)
	assert.Equal(t, "/openai/v1", rdc.Context, "the provisioned endpoint's context is configuration")

	chat := rdc.Routes["POST|/openai/v1/chat/completions|*"]
	require.NotNil(t, chat, "chat completions route; routes: %v", keys(rdc.Routes))
	assert.Equal(t, constants.LLM_MODEL_DISPATCH_RESOLVER_NAME, chat.ResolverName)
	assert.Equal(t, "openai", chat.ErrorFormat)
	assert.Equal(t, int64(1<<20), chat.MaxRequestBodyBytes)
	assert.True(t, chat.Upstream.UseClusterHeader)
	// The engine sets the path of every request the endpoint forwards; a route
	// rewrite would run again on it.
	assert.True(t, chat.NoPathRewrite)
	rc := resolverConfigOf(t, chat)
	assert.Equal(t, map[string]string{"gpt-4o": "gpt", "claude": "claude"}, rc.Models)
	assert.Equal(t, constants.LLM_MODEL_DISPATCH_ERROR_OPERATION, rc.ErrorOperation)
	assert.Equal(t, map[string]string{"location": "payload", "identifier": "$.model"}, rc.ModelLocation)
	assert.Equal(t, map[string]string{"location": "payload", "identifier": "$.stream"}, rc.Stream)

	catalogRoute := rdc.Routes["GET|/openai/v1/models|*"]
	require.NotNil(t, catalogRoute)
	assert.Equal(t, "openai", catalogRoute.ErrorFormat)
	assert.True(t, catalogRoute.NoPathRewrite)

	// One chain per Model, the dispatch chain, and the catalog chain.
	gpt := rdc.PolicyChains[chainkey.For(uuid, "*", "gpt")]
	require.NotNil(t, gpt)
	assert.Equal(t, []string{"api-key-auth", "wso2_apip_sys_model", "set-headers", "wso2_apip_sys_model"}, policyNames(gpt))
	assert.Equal(t, "route", modelJob(gpt.Policies[1]))
	assert.Equal(t, "providerScope", modelJob(gpt.Policies[3]))
	// The direct credential is the provider's own, gated on its selection and
	// applied once the body-phase routing decision has been made.
	cred := gpt.Policies[2]
	require.NotNil(t, cred.ExecutionCondition)
	assert.Contains(t, *cred.ExecutionCondition, "'openai-direct'")
	request := cred.Params["request"].(map[string]interface{})
	assert.Equal(t, "body", request["phase"])
	assert.Equal(t, "Bearer sk-direct", request["headers"].([]interface{})[0].(map[string]interface{})["value"])

	claude := rdc.PolicyChains[chainkey.For(uuid, "*", "claude")]
	require.NotNil(t, claude)
	assert.Equal(t, []string{"api-key-auth", "wso2_apip_sys_model", "openai-to-anthropic-transformer", "set-headers",
		"set-headers", "wso2_apip_sys_model"}, policyNames(claude))
	transformer := claude.Policies[2]
	assert.Equal(t, "anthropic-routed", transformer.Params["providerId"])
	assert.Equal(t, "2023-06-01", transformer.Params["anthropicVersion"], "the placeholder's default fills it")
	// The route credential carries the provider route's api-key-auth valuePrefix.
	routeCred := claude.Policies[3].Params["request"].(map[string]interface{})["headers"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "x-api-key", routeCred["name"])
	assert.Equal(t, "Key route-key", routeCred["value"])
	// The loopback marker is set only on the routed group.
	marker := claude.Policies[4].Params["request"].(map[string]interface{})["headers"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, constants.InternalLoopbackHeader, marker["name"])

	routeJob := claude.Policies[1].Params["route"].(map[string]any)
	targets := routeJob["targets"].([]any)
	require.Len(t, targets, 1)
	assert.Equal(t, true, targets[0].(map[string]any)["routed"])
	assert.Equal(t, "anthropic", targets[0].(map[string]any)["template"])
	assert.Equal(t, "openai", routeJob["format"])
	assert.Equal(t, map[string]any{"location": "payload", "identifier": "$.model"}, routeJob["modelLocation"])
	scope := routeJob["scope"].(map[string]any)
	assert.Equal(t, "openai", scope["templateHandle"])
	assert.Equal(t, models.ModelEndpointProviderName("openai"), scope["providerName"])

	dispatch := rdc.PolicyChains[chainkey.For(uuid, "*", constants.LLM_MODEL_DISPATCH_ERROR_OPERATION)]
	require.NotNil(t, dispatch)
	assert.Equal(t, []string{"api-key-auth", "wso2_apip_sys_model"}, policyNames(dispatch))
	assert.Equal(t, "dispatchError", modelJob(dispatch.Policies[1]))
	assert.Equal(t, map[string]any{"format": "openai"}, dispatch.Policies[1].Params["dispatchError"])

	catalog := rdc.PolicyChains["GET|/openai/v1/models|*"]
	require.NotNil(t, catalog)
	assert.Equal(t, []string{"api-key-auth", "wso2_apip_sys_model"}, policyNames(catalog))
	catalogJob := catalog.Policies[1].Params["catalog"].(map[string]any)
	assert.Equal(t, "openai", catalogJob["format"])
	entries, err := json.Marshal(catalogJob["models"])
	require.NoError(t, err)
	assert.JSONEq(t, `[
		{"handle":"claude","name":"claude","displayName":"claude","created":1700000000},
		{"handle":"gpt","name":"gpt-4o","displayName":"gpt","created":1700000000}]`, string(entries))

	// One cluster per provider, named so the engine resolves the provider handle
	// to it, plus the fail-closed default.
	direct := rdc.UpstreamClusters["upstream_LlmProxy_"+uuid+"_openai-direct"]
	require.NotNil(t, direct)
	assert.Equal(t, "openai-direct", direct.Name)
	assert.Equal(t, "/v1", direct.BasePath)
	assert.Equal(t, "api.openai.com", direct.Endpoints[0].Host)
	assert.Equal(t, 443, direct.Endpoints[0].Port)
	assert.True(t, direct.TLS.Enabled)
	// The upstream a request credential signs for once the route job picks this provider.
	assert.Equal(t, "https://api.openai.com", direct.URL)

	routed := rdc.UpstreamClusters["upstream_LlmProxy_"+uuid+"_anthropic-routed"]
	require.NotNil(t, routed)
	assert.Equal(t, "/anthropic", routed.BasePath)
	assert.Equal(t, constants.LocalhostIP, routed.Endpoints[0].Host)
	assert.Equal(t, testListenerPort, routed.Endpoints[0].Port)
	assert.Empty(t, routed.Endpoints[0].Hostname, "a provider on the '*' vhost needs no host name")
	assert.Equal(t, fmt.Sprintf("http://%s:%d", constants.LocalhostIP, testListenerPort), routed.URL)

	// The default reaches nothing: no endpoints, fail closed.
	unrouted := rdc.UpstreamClusters[chat.Upstream.DefaultCluster]
	require.NotNil(t, unrouted)
	assert.True(t, unrouted.FailClosed)
	assert.Empty(t, unrouted.Endpoints)
	assert.Empty(t, unrouted.URL, "the default upstream is not a definition a policy redirects to")

	// The direct credential is redacted from policy-chain logs.
	assert.NotContains(t, rdc.SensitiveValues, "")
}

func TestModelGateway_FailedModelIsLeftOutOthersServe(t *testing.T) {
	f := newModelGatewayFixture(t)
	f.addModel("gpt", "gpt-4o", []api.ModelTarget{directTarget("openai", "gpt-4o")}, nil)
	f.addModel("broken", "broken", []api.ModelTarget{{
		Name: "m", Provider: "no-such-provider", Alias: ptr("x"),
	}}, nil)

	rdc, statuses, err := f.compiler.Compile(f.endpoint)
	require.NoError(t, err)

	byHandle := statusesByHandle(statuses)
	assert.True(t, byHandle["gpt"].Served)
	assert.False(t, byHandle["broken"].Served)
	assert.Contains(t, byHandle["broken"].Reason, "no-such-provider")

	uuid := models.ModelEndpointUUID(models.DefaultModelEndpointHandle)
	assert.NotNil(t, rdc.PolicyChains[chainkey.For(uuid, "*", "gpt")])
	assert.Nil(t, rdc.PolicyChains[chainkey.For(uuid, "*", "broken")])
	assert.Equal(t, map[string]string{"gpt-4o": "gpt"}, resolverConfigOf(t, rdc.Routes["POST|/openai/v1/chat/completions|*"]).Models)
}

// Each endpoint compiles only the Models that name it, with its own UUID, context,
// policies and transformers; a Model on two endpoints is compiled into both.
func TestModelGateway_SeveralEndpoints(t *testing.T) {
	f := newModelGatewayFixture(t)
	partners := f.addEndpoint("partners", "/partners/v1", "", nil) // no transformer entries
	f.addModel("gpt", "gpt-4o", []api.ModelTarget{directTarget("openai", "gpt-4o")}, nil, "openai", "partners")
	f.addModel("claude", "claude", []api.ModelTarget{routedTarget("anthropic", "claude-sonnet")}, nil, "openai", "partners")
	f.addModel("partner-only", "partner-chat", []api.ModelTarget{directTarget("openai", "gpt-4o-mini")}, nil, "partners")
	f.addModel("elsewhere", "elsewhere", []api.ModelTarget{directTarget("openai", "gpt-4o-mini")}, nil, "not-on-this-gateway")

	primary, statuses, err := f.compiler.Compile(f.endpoint)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"gpt-4o": "gpt", "claude": "claude"},
		resolverConfigOf(t, primary.Routes["POST|/openai/v1/chat/completions|*"]).Models)
	assert.Len(t, statuses, 2, "a Model naming another endpoint is not this endpoint's to report")

	rdc, statuses, err := f.compiler.Compile(partners)
	require.NoError(t, err)
	partnersUUID := models.ModelEndpointUUID("partners")
	assert.Equal(t, partnersUUID, rdc.Metadata.UUID)
	assert.NotEqual(t, primary.Metadata.UUID, rdc.Metadata.UUID)
	assert.Equal(t, "/partners/v1", rdc.Context)
	chat := rdc.Routes["POST|/partners/v1/chat/completions|*"]
	require.NotNil(t, chat, "routes: %v", keys(rdc.Routes))
	// partners has no transformer for Anthropic providers, so claude is left out of
	// partners — and only partners.
	assert.Equal(t, map[string]string{"gpt-4o": "gpt", "partner-chat": "partner-only"}, resolverConfigOf(t, chat).Models)
	byHandle := statusesByHandle(statuses)
	assert.False(t, byHandle["claude"].Served)
	assert.Contains(t, byHandle["claude"].Reason, "no transformer for provider 'anthropic-routed'")
	assert.NotNil(t, rdc.PolicyChains[chainkey.For(partnersUUID, "*", "gpt")])
	assert.Nil(t, rdc.PolicyChains[chainkey.For(partnersUUID, "*", "claude")])
	assert.NotNil(t, rdc.UpstreamClusters["upstream_LlmProxy_"+partnersUUID+"_openai-direct"])
	assert.Nil(t, rdc.UpstreamClusters["upstream_LlmProxy_"+partnersUUID+"_anthropic-routed"])
}

func TestModelGateway_AnthropicEndpoint(t *testing.T) {
	f := newModelGatewayFixture(t)
	native := f.addEndpoint("anthropic-native", "/anthropic-native", "anthropic", nil,
		api.ModelEndpointTransformer{Template: ptr("openai"), Type: "anthropic-to-openai-transformer", Version: ptr("v1")})
	f.addModel("claude", "claude-haiku", []api.ModelTarget{routedTarget("anthropic", "claude-haiku-4-5")}, nil,
		"anthropic-native")
	f.addModel("gpt", "gpt-4o", []api.ModelTarget{directTarget("openai", "gpt-4o")}, nil, "anthropic-native")

	rdc, statuses, err := f.compiler.Compile(native)
	require.NoError(t, err)
	for _, s := range statuses {
		require.True(t, s.Served, "%s: %s", s.Handle, s.Reason)
	}
	uuid := models.ModelEndpointUUID("anthropic-native")
	assert.Equal(t, "anthropic", rdc.Metadata.LLM.TemplateHandle)

	messages := rdc.Routes["POST|/anthropic-native/v1/messages|*"]
	require.NotNil(t, messages, "routes: %v", keys(rdc.Routes))
	assert.Equal(t, "anthropic", messages.ErrorFormat)
	assert.Equal(t, map[string]string{"location": "payload", "identifier": "$.model"}, resolverConfigOf(t, messages).ModelLocation)
	require.NotNil(t, rdc.Routes["GET|/anthropic-native/v1/models|*"])
	assert.Nil(t, rdc.Routes["POST|/anthropic-native/chat/completions|*"])

	// An Anthropic provider speaks the endpoint's format and passes through; an
	// OpenAI one runs the customer's anthropic-to-openai transformer.
	claude := rdc.PolicyChains[chainkey.For(uuid, "*", "claude")]
	require.NotNil(t, claude)
	assert.NotContains(t, policyNames(claude), "openai-to-anthropic-transformer")
	gpt := rdc.PolicyChains[chainkey.For(uuid, "*", "gpt")]
	require.NotNil(t, gpt)
	assert.Contains(t, policyNames(gpt), "anthropic-to-openai-transformer")

	route := claude.Policies[1].Params["route"].(map[string]any)
	assert.Equal(t, "anthropic", route["format"])
	assert.Equal(t, "anthropic", route["scope"].(map[string]any)["templateHandle"])
}

func TestModelGateway_GeminiEndpointReadsTheModelFromThePath(t *testing.T) {
	f := newModelGatewayFixture(t)
	f.addProvider("gemini-direct", "gemini", func(p *api.LLMProviderConfiguration) {
		none := api.LLMProviderConfigDataExposure(models.LLMProviderExposureNone)
		p.Spec.Exposure = &none
		p.Spec.Upstream.Url = ptr("https://generativelanguage.googleapis.com")
	})
	gem := f.addEndpoint("gem", "/gemini", "gemini", nil)
	f.addModel("flash", "gemini-flash", []api.ModelTarget{{
		Name: "gemini-2.0-flash", Provider: "gemini-direct", Alias: ptr("g"),
	}}, nil, "gem")

	rdc, statuses, err := f.compiler.Compile(gem)
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	require.True(t, statuses[0].Served, statuses[0].Reason)

	for _, version := range []string{"v1", "v1beta"} {
		for _, method := range []string{"generateContent", "streamGenerateContent"} {
			key := "POST|/gemini/" + version + "/models/{model}:" + method + "|*"
			route := rdc.Routes[key]
			require.NotNil(t, route, "route %s; routes: %v", key, keys(rdc.Routes))
			assert.Equal(t, "gemini", route.ErrorFormat)
			rc := resolverConfigOf(t, route)
			assert.Equal(t, "pathParam", rc.ModelLocation["location"])
			assert.Equal(t, `models/([a-zA-Z0-9.\-]+)`, rc.ModelLocation["identifier"])
			assert.Equal(t, map[string]string{"location": "pathParam", "identifier": ":streamGenerateContent$"}, rc.Stream)
			assert.Equal(t, map[string]string{"gemini-flash": "flash"}, rc.Models)
		}
		require.NotNil(t, rdc.Routes["GET|/gemini/"+version+"/models|*"])
	}
}

func TestModelGateway_MixedFormatsWarnAboutModelPolicies(t *testing.T) {
	f := newModelGatewayFixture(t)
	f.addEndpoint("anthropic-native", "/anthropic-native", "anthropic", nil,
		api.ModelEndpointTransformer{Template: ptr("openai"), Type: "anthropic-to-openai-transformer", Version: ptr("v1")})
	m := api.ModelConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1", Kind: "Model", Metadata: api.Metadata{Name: "gpt"},
		Spec: api.ModelConfigData{
			DisplayName: "gpt", Version: "v1.0", ModelName: "gpt-4o", Endpoints: []string{"openai", "anthropic-native"},
			Targets:  []api.ModelTarget{directTarget("openai", "gpt-4o")},
			Policies: &[]api.Policy{{Name: "token-based-ratelimit", Version: "v1"}},
		},
	}
	require.NoError(t, f.store.Add(&models.StoredConfig{UUID: "uuid-gpt", Kind: models.KindModel, Handle: "gpt",
		DisplayName: "gpt", Version: "v1.0", SourceConfiguration: m, Configuration: m, DesiredState: models.StateDeployed}))

	_, statuses, err := f.compiler.Compile(f.endpoint)
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.True(t, statuses[0].Served)
	assert.Contains(t, statuses[0].Warning, "more than one inbound format")
}

func TestModelGateway_RouterParamsReachTheRouterAsWritten(t *testing.T) {
	f := newModelGatewayFixture(t)
	params := map[string]interface{}{
		"models": []interface{}{
			map[string]interface{}{"model": "primary"},
			map[string]interface{}{"model": "claude-sonnet", "provider": "anthropic-routed"},
		},
	}
	f.addModel("mixed", "mixed", []api.ModelTarget{
		directTarget("primary", "gpt-4o"),
		routedTarget("secondary", "claude-sonnet"),
	}, &api.ModelRouting{Policy: api.ModelRoutingPolicy{Name: "model-round-robin", Version: "v1"}, Params: &params})

	rdc, statuses, err := f.compiler.Compile(f.endpoint)
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	require.True(t, statuses[0].Served, statuses[0].Reason)

	chain := rdc.PolicyChains[chainkey.For(models.ModelEndpointUUID("openai"), "*", "mixed")]
	require.NotNil(t, chain)
	assert.Equal(t, []string{
		"api-key-auth",
		"model-round-robin",
		"wso2_apip_sys_model",
		"set-headers",                     // openai-direct credential
		"openai-to-anthropic-transformer", // anthropic-routed group
		"set-headers",
		"set-headers",
		"wso2_apip_sys_model",
	}, policyNames(chain))

	// Nothing is rewritten: the route job matches each choice to a target at
	// request time.
	router := chain.Policies[1]
	assert.Equal(t, []interface{}{
		map[string]interface{}{"model": "primary"},
		map[string]interface{}{"model": "claude-sonnet", "provider": "anthropic-routed"},
	}, router.Params["models"])
	// The inbound template's extraction fields reach the router, as they reach an
	// LLM proxy's policies.
	requestModel, err := json.Marshal(router.Params["requestModel"])
	require.NoError(t, err)
	assert.JSONEq(t, `{"location":"payload","identifier":"$.model"}`, string(requestModel))
}

func TestModelGateway_DeterministicOutput(t *testing.T) {
	f := newModelGatewayFixture(t)
	f.addModel("gpt", "gpt-4o", []api.ModelTarget{directTarget("openai", "gpt-4o")}, nil)
	f.addModel("claude", "claude", []api.ModelTarget{routedTarget("anthropic", "claude-sonnet")}, nil)

	first, _, err := f.compiler.Compile(f.endpoint)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		again, _, err := f.compiler.Compile(f.endpoint)
		require.NoError(t, err)
		a, _ := json.Marshal(first)
		b, _ := json.Marshal(again)
		require.Equal(t, string(a), string(b))
	}
}

func TestModelGateway_TransformerDispatch(t *testing.T) {
	f := newModelGatewayFixture(t)
	f.addModel("gpt", "gpt-4o", []api.ModelTarget{directTarget("openai", "gpt-4o")}, nil)

	registry := NewRegistry(nil, f.llm, nil, nil)
	// Without a compiler wired, an endpoint serves nothing rather than failing.
	rdc, err := registry.Transform(f.endpoint)
	require.NoError(t, err)
	assert.Empty(t, rdc.Routes)

	registry.SetModelEndpointTransformer(NewModelEndpointTransformer(f.compiler))
	rdc, err = registry.Transform(f.endpoint)
	require.NoError(t, err)
	assert.NotEmpty(t, rdc.Routes)

	// A Model has no runtime config of its own.
	model, err := f.store.GetByKindAndHandle(models.KindModel, "gpt")
	require.NoError(t, err)
	_, err = registry.Transform(model)
	require.ErrorIs(t, err, ErrUnsupportedKind)

	// A provider with exposure none compiles to nothing at all.
	provider, err := f.store.GetByKindAndHandle(models.KindLlmProvider, "openai-direct")
	require.NoError(t, err)
	rdc, err = f.llm.Transform(provider)
	require.NoError(t, err)
	assert.Empty(t, rdc.Routes)
	assert.Empty(t, rdc.PolicyChains)
	assert.Empty(t, rdc.UpstreamClusters)
	require.NotNil(t, rdc.Metadata.LLM)
	assert.Equal(t, "openai", rdc.Metadata.LLM.TemplateHandle)
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// A compiled model endpoint goes through both xDS paths: Envoy gets its routes on
// cluster-header routing and one cluster per provider, the policy engine gets every
// chain and a route config carrying the resolver and the error format — and neither
// a Model nor an unexposed provider contributes anything of its own.
func TestModelGateway_TranslatesOnBothXDSPaths(t *testing.T) {
	f := newModelGatewayFixture(t)
	f.addModel("gpt", "gpt-4o", []api.ModelTarget{directTarget("openai", "gpt-4o")}, nil)
	f.addModel("claude", "claude", []api.ModelTarget{routedTarget("anthropic", "claude-sonnet")}, nil)

	routerCfg := f.compiler.routerConfig
	routerCfg.LuaScriptPath = "../../lua/request_transformation.lua"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	translator, err := xds.NewTranslator(logger, routerCfg, f.db, f.sysCfg)
	require.NoError(t, err)

	restT := NewRestAPITransformer(routerCfg, f.sysCfg, modelGatewayPolicyDefinitions())
	registry := NewRegistry(restT, f.llm, agentTransformer(), NewGraphQLAPITransformer(routerCfg, f.sysCfg, nil))
	registry.SetModelEndpointTransformer(NewModelEndpointTransformer(f.compiler))
	transformers := map[string]models.ConfigTransformer{}
	for _, kind := range EnvoyTranslatorKinds() {
		transformers[kind] = registry
	}
	translator.SetTransformers(transformers)

	resources, err := translator.TranslateConfigs(f.store.GetAll(), "")
	require.NoError(t, err)

	uuid := models.ModelEndpointUUID(models.DefaultModelEndpointHandle)
	var routeNames []string
	for _, res := range resources[resource.RouteType] {
		rc := res.(*routev3.RouteConfiguration)
		for _, vh := range rc.GetVirtualHosts() {
			for _, r := range vh.GetRoutes() {
				routeNames = append(routeNames, r.GetName())
				if r.GetName() == "POST|/openai/v1/chat/completions|*" {
					assert.Equal(t, constants.TargetUpstreamHeader, r.GetRoute().GetClusterHeader())
				}
			}
		}
	}
	assert.Contains(t, routeNames, "POST|/openai/v1/chat/completions|*")
	assert.Contains(t, routeNames, "GET|/openai/v1/models|*")
	for _, name := range routeNames {
		// The unexposed provider has no route, and nothing was built at "/" by the
		// legacy path for it or for a Model.
		assert.NotContains(t, name, "|/|", "unexpected root route %q", name)
	}

	clusterNames := map[string]bool{}
	for _, res := range resources[resource.ClusterType] {
		clusterNames[res.(*cluster.Cluster).GetName()] = true
	}
	assert.True(t, clusterNames["upstream_LlmProxy_"+uuid+"_openai-direct"], "direct provider cluster")
	assert.True(t, clusterNames["upstream_LlmProxy_"+uuid+"_anthropic-routed"], "routed provider cluster")

	rdc, err := registry.Transform(f.endpoint)
	require.NoError(t, err)
	policyResources, err := policyxds.NewTranslator(logger).TranslateRuntimeConfigs([]*models.RuntimeDeployConfig{rdc})
	require.NoError(t, err)
	assert.Contains(t, policyResources[policyxds.PolicyChainTypeURL], chainkey.For(uuid, "*", "gpt"))
	assert.Contains(t, policyResources[policyxds.PolicyChainTypeURL], chainkey.For(uuid, "*", "claude"))
	assert.Contains(t, policyResources[policyxds.PolicyChainTypeURL],
		chainkey.For(uuid, "*", constants.LLM_MODEL_DISPATCH_ERROR_OPERATION))
	assert.Contains(t, policyResources[policyxds.RouteConfigTypeURL], "POST|/openai/v1/chat/completions|*")
}

// A Model whose provider's cluster cannot be built is left out of the endpoint, with
// the reason, like a Model that does not resolve: served, its requests would name a
// cluster Envoy does not have.
func TestModelGateway_ModelWithUnbuildableClusterIsLeftOut(t *testing.T) {
	f := newModelGatewayFixture(t)
	f.addProvider("bad-upstream", "openai", func(p *api.LLMProviderConfiguration) {
		none := api.LLMProviderConfigDataExposure(models.LLMProviderExposureNone)
		p.Spec.Exposure = &none
		p.Spec.Upstream.Url = ptr("ftp://files.example.com")
	})
	f.addModel("gpt", "gpt-4o", []api.ModelTarget{directTarget("openai", "gpt-4o")}, nil)
	f.addModel("broken", "broken", []api.ModelTarget{{
		Name: "m", Provider: "bad-upstream", Alias: ptr("x"),
	}}, nil)

	rdc, statuses, err := f.compiler.Compile(f.endpoint)
	require.NoError(t, err)

	byHandle := statusesByHandle(statuses)
	assert.True(t, byHandle["gpt"].Served)
	assert.False(t, byHandle["broken"].Served)
	assert.Contains(t, byHandle["broken"].Reason, "bad-upstream")

	uuid := models.ModelEndpointUUID(models.DefaultModelEndpointHandle)
	assert.Nil(t, rdc.PolicyChains[chainkey.For(uuid, "*", "broken")])
	assert.NotContains(t, rdc.UpstreamClusters, "upstream_LlmProxy_"+uuid+"_bad-upstream")
	assert.Equal(t, map[string]string{"gpt-4o": "gpt"},
		resolverConfigOf(t, rdc.Routes["POST|/openai/v1/chat/completions|*"]).Models)
	catalog := rdc.PolicyChains["GET|/openai/v1/models|*"].Policies[1].Params["catalog"].(map[string]any)
	assert.Len(t, catalog["models"], 1, "the catalog lists only the Model that is served")
}

// countingSecretResolver counts how often each secret is read.
type countingSecretResolver struct{ calls map[string]int }

func (r *countingSecretResolver) Resolve(handle string) (string, error) {
	r.calls[handle]++
	return "sk-" + handle, nil
}

// A compile renders each provider once, however many targets are on it: rendering
// reads the provider's secrets, and the compiler runs on every Envoy snapshot.
func TestModelGateway_ProviderRenderedOncePerCompile(t *testing.T) {
	f := newModelGatewayFixture(t)
	resolver := &countingSecretResolver{calls: map[string]int{}}
	f.compiler = NewModelGatewayCompiler(f.store, f.compiler.routerConfig, f.sysCfg, f.compiler.policyDefinitions,
		f.llm.ProviderTransformer(), resolver)

	f.addProvider("openai-secret", "openai", func(p *api.LLMProviderConfiguration) {
		none := api.LLMProviderConfigDataExposure(models.LLMProviderExposureNone)
		p.Spec.Exposure = &none
		p.Spec.Upstream.Url = ptr("https://api.openai.com/v1")
		p.Spec.Upstream.Auth = &struct {
			Header        *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
			PolicyName    *string                                   `json:"policyName,omitempty" yaml:"policyName,omitempty"`
			PolicyParams  *map[string]interface{}                   `json:"policyParams,omitempty" yaml:"policyParams,omitempty"`
			PolicyVersion *string                                   `json:"policyVersion,omitempty" yaml:"policyVersion,omitempty"`
			Type          api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
			Value         *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
		}{Type: api.LLMProviderConfigDataUpstreamAuthTypeApiKey, Header: ptr("Authorization"),
			Value: ptr(`Bearer {{ secret "openai-key" }}`)}
	})
	target := func(name, model string) api.ModelTarget {
		return api.ModelTarget{Name: model, Provider: "openai-secret", Alias: ptr(name)}
	}
	roundRobin := &api.ModelRouting{Policy: api.ModelRoutingPolicy{Name: "model-round-robin", Version: "v1"}}
	f.addModel("a", "a", []api.ModelTarget{target("t", "gpt-4o")}, nil)
	f.addModel("b", "b", []api.ModelTarget{target("t", "gpt-4o")}, nil)
	f.addModel("c", "c", []api.ModelTarget{target("t1", "gpt-4o"), target("t2", "gpt-4o-mini"),
		target("t3", "o3-mini")}, roundRobin)

	rdc, statuses, err := f.compiler.Compile(f.endpoint)
	require.NoError(t, err)
	for _, s := range statuses {
		require.True(t, s.Served, "model %s not served: %s", s.Handle, s.Reason)
	}
	assert.Equal(t, 1, resolver.calls["openai-key"], "five targets on one provider, one render per compile")
	assert.Contains(t, rdc.SensitiveValues, "sk-openai-key", "the rendered credential is still redacted")

	_, _, err = f.compiler.Compile(f.endpoint)
	require.NoError(t, err)
	assert.Equal(t, 2, resolver.calls["openai-key"], "nothing is kept between compiles")
}
