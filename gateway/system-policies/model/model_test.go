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

package model

import (
	"os"
	"strings"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// routeBlock is the shape the controller writes for a Model with the given
// targets, each {name, provider, upstreamModel, template, routed}.
func routeBlock(targets ...map[string]any) map[string]any {
	list := make([]any, 0, len(targets))
	for _, target := range targets {
		list = append(list, target)
	}
	return map[string]any{
		ParamRoute: map[string]any{
			ParamModelHandle: "chat-default",
			ParamModelName:   "chat",
			ParamScope: map[string]any{
				ParamTemplateHandle: "openai",
				ParamProviderName:   "__wso2_models__",
			},
			ParamTargets: list,
		},
	}
}

func target(name, provider, upstreamModel, template string, routed bool) map[string]any {
	return map[string]any{
		ParamTargetName:          name,
		ParamTargetProvider:      provider,
		ParamTargetUpstreamModel: upstreamModel,
		ParamTargetTemplate:      template,
		ParamTargetDisplayName:   strings.ToUpper(provider),
		ParamTargetRouted:        routed,
	}
}

func mustPolicy(t *testing.T, params map[string]any) policy.Policy {
	t.Helper()
	instance, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("GetPolicy: %v", err)
	}
	return instance
}

// ─── the contract with the controller and the policy engine ─────────────────

// The parameter names are the contract with the controller's model gateway
// compiler, which writes them from a separate Go module and so cannot share these
// constants. A rename on one side alone fails every Model chain's build.
func TestParameterNamesMatchTheControllerContract(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{PolicyName, "wso2_apip_sys_model"},
		{ParamRoute, "route"},
		{ParamProviderScope, "providerScope"},
		{ParamDispatchError, "dispatchError"},
		{ParamCatalog, "catalog"},
		{ParamModelHandle, "modelHandle"},
		{ParamModelName, "modelName"},
		{ParamScope, "scope"},
		{ParamTemplateHandle, "templateHandle"},
		{ParamProviderName, "providerName"},
		{ParamTargets, "targets"},
		{ParamTargetName, "name"},
		{ParamTargetProvider, "provider"},
		{ParamTargetUpstreamModel, "upstreamModel"},
		{ParamTargetTemplate, "template"},
		{ParamTargetDisplayName, "providerDisplayName"},
		{ParamTargetRouted, "routed"},
		{ParamModelLocation, "modelLocation"},
		{ParamFormat, "format"},
		{ParamCatalogModels, "models"},
		{ParamCatalogHandle, "handle"},
		{ParamCatalogName, "name"},
		{ParamCatalogDisplayName, "displayName"},
		{ParamCatalogCreated, "created"},
		{FormatOpenAI, "openai"},
		{FormatAnthropic, "anthropic"},
		{FormatGemini, "gemini"},
		// The auth context api-key-auth records, which the catalog filters on.
		{APIKeyAuthType, "apikey"},
		{AuthPropertyAPIKeyOperations, "APIKeyOperations"},
	} {
		if tc.got != tc.want {
			t.Errorf("parameter name = %q, want %q", tc.got, tc.want)
		}
	}
}

// The metadata keys are read by policies this module does not own: the
// controller's conditional translator and credential policies match on
// selected_provider, every LLM-aware policy reads the two scope keys, and the
// analytics collector reads the model.* facts. A rename here silently routes a
// request with no credential, or reports the wrong provider.
func TestMetadataKeysMatchTheirConsumers(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{MetadataSelectedProvider, "selected_provider"},
		{MetadataTemplateHandle, "template_handle"},
		{MetadataProviderName, "provider_name"},
		{MetadataModelHandle, "model.handle"},
		{MetadataModelName, "model.name"},
		{MetadataModelTarget, "model.target"},
		{MetadataProviderHandle, "model.provider.handle"},
		{MetadataProviderTemplate, "model.provider.template"},
		{MetadataProviderDisplayName, "model.provider.displayName"},
		{MetadataUpstreamModel, "model.upstreamModel"},
		{MetadataResponseModel, "model.responseModel"},
		{InternalLoopbackHeader, "x-wso2-internal-loopback"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}

// The resolution attribute names and the dispatch error values are the contract
// with the llm-model-dispatch resolver in the policy engine, which spells them in
// an internal package of another module. A rename on one side alone answers every
// dispatch-chain request with a 500.
func TestResolutionAttributeSpellingsMatchThePolicyEngine(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{AttrModelName, "llm.model.name"},
		{AttrModelHandle, "llm.model.handle"},
		{AttrRequestStream, "llm.request.stream"},
		{AttrDispatchError, "llm.dispatch.error"},
		{DispatchErrorInvalidJSON, "invalid_json"},
		{DispatchErrorMissingModel, "missing_model"},
		{DispatchErrorInvalidModel, "invalid_model"},
		{DispatchErrorModelNotFound, "model_not_found"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}

// The definition the builder registers must name every block and field this
// package reads. Checked by name rather than by parsing the schema, so the module
// needs no YAML dependency for it; the controller's own contract test parses it.
func TestPolicyDefinitionDeclaresEveryParameter(t *testing.T) {
	raw, err := os.ReadFile("policy-definition.yaml")
	if err != nil {
		t.Fatalf("read policy definition: %v", err)
	}
	definition := string(raw)
	if !strings.Contains(definition, "name: "+PolicyName+"\n") {
		t.Errorf("policy definition does not declare name %q", PolicyName)
	}
	if !strings.Contains(definition, "version: v1.0.0\n") {
		t.Error("policy definition does not declare version v1.0.0")
	}
	for _, name := range []string{
		ParamRoute, ParamProviderScope, ParamDispatchError, ParamCatalog,
		ParamModelHandle, ParamModelName, ParamScope, ParamTemplateHandle, ParamProviderName,
		ParamTargets, ParamTargetName, ParamTargetProvider, ParamTargetUpstreamModel,
		ParamTargetTemplate, ParamTargetDisplayName, ParamTargetRouted, ParamModelLocation, ParamFormat,
		ParamCatalogModels, ParamCatalogHandle, ParamCatalogName, ParamCatalogDisplayName, ParamCatalogCreated,
	} {
		if !strings.Contains(definition, "\n    "+name+":") &&
			!strings.Contains(definition, "\n        "+name+":") &&
			!strings.Contains(definition, "\n            "+name+":") &&
			!strings.Contains(definition, "\n              "+name+":") {
			t.Errorf("policy definition does not declare parameter %q", name)
		}
	}
}

// The policy must be in the system build lock, or the gateway is built without it
// and every Model chain fails to build.
func TestPolicyIsInTheSystemBuildLock(t *testing.T) {
	raw, err := os.ReadFile("../system-build-lock.yaml")
	if err != nil {
		t.Fatalf("read build lock: %v", err)
	}
	if !strings.Contains(string(raw), "- name: "+PolicyName+"\n    filePath: ./model\n") {
		t.Errorf("%s is not listed in the system build lock with filePath ./model", PolicyName)
	}
}

// ─── instance construction ───────────────────────────────────────────────────

func TestGetPolicyV2IsAnAlias(t *testing.T) {
	instance, err := GetPolicyV2(policy.PolicyMetadata{}, map[string]any{ParamDispatchError: map[string]any{}})
	if err != nil {
		t.Fatalf("GetPolicyV2: %v", err)
	}
	if _, ok := instance.(*dispatchErrorPolicy); !ok {
		t.Fatalf("GetPolicyV2 returned %T, want the dispatch error job", instance)
	}
}

// An instance that does not carry exactly one usable block is refused at chain
// build, so its chain is never served and nothing reaches a provider through it.
func TestUnusableBlocksAreRefusedAtChainBuild(t *testing.T) {
	valid := target("primary", "openai-prod", "gpt-4o", "openai", false)
	// A nil value removes the field rather than setting it to null.
	withField := func(field string, value any) map[string]any {
		block := routeBlock(valid)
		if value == nil {
			delete(block[ParamRoute].(map[string]any), field)
		} else {
			block[ParamRoute].(map[string]any)[field] = value
		}
		return block
	}
	withTargetField := func(field string, value any) map[string]any {
		broken := target("primary", "openai-prod", "gpt-4o", "openai", false)
		if value == nil {
			delete(broken, field)
		} else {
			broken[field] = value
		}
		return routeBlock(broken)
	}

	for name, params := range map[string]map[string]any{
		"no params":             nil,
		"no job block":          {"attachedTo": "api"},
		"two job blocks":        {ParamDispatchError: map[string]any{}, ParamProviderScope: map[string]any{}},
		"route not an object":   {ParamRoute: "route"},
		"route missing handle":  withField(ParamModelHandle, ""),
		"route handle not text": withField(ParamModelHandle, 7),
		"route missing name":    withField(ParamModelName, nil),
		"route scope missing":   withField(ParamScope, nil),
		"route scope partial": withField(ParamScope, map[string]any{
			ParamTemplateHandle: "openai",
		}),
		"route no targets":             withField(ParamTargets, []any{}),
		"route targets not an array":   withField(ParamTargets, map[string]any{}),
		"route target not an object":   withField(ParamTargets, []any{"primary"}),
		"target missing provider":      withTargetField(ParamTargetProvider, nil),
		"target missing upstream":      withTargetField(ParamTargetUpstreamModel, nil),
		"target missing template":      withTargetField(ParamTargetTemplate, nil),
		"target missing name":          withTargetField(ParamTargetName, nil),
		"target routed missing":        withTargetField(ParamTargetRouted, nil),
		"target routed not a boolean":  withTargetField(ParamTargetRouted, "false"),
		"target display name not text": withTargetField(ParamTargetDisplayName, 3),
		"duplicate target names": routeBlock(valid,
			target("primary", "anthropic", "claude", "anthropic", true)),
		"duplicate provider and model": routeBlock(valid,
			target("secondary", "openai-prod", "gpt-4o", "openai", false)),
		"provider scope not an object":  {ParamProviderScope: true},
		"provider scope with a field":   {ParamProviderScope: map[string]any{"provider": "x"}},
		"dispatch error with a field":   {ParamDispatchError: map[string]any{"status": 404}},
		"dispatch error unknown format": {ParamDispatchError: map[string]any{ParamFormat: "cohere"}},
		"catalog models not an array":   {ParamCatalog: map[string]any{ParamCatalogModels: "gpt"}},
		"catalog entry not an object":   {ParamCatalog: map[string]any{ParamCatalogModels: []any{"gpt"}}},
		"catalog entry without handle":  {ParamCatalog: map[string]any{ParamCatalogModels: []any{map[string]any{ParamCatalogName: "g"}}}},
		"catalog entry without name":    {ParamCatalog: map[string]any{ParamCatalogModels: []any{map[string]any{ParamCatalogHandle: "g"}}}},
		"catalog unknown format":        {ParamCatalog: map[string]any{ParamFormat: 3}},
		"catalog block not an object":   {ParamCatalog: "{}"},
		"route nested model location":   withField(ParamModelLocation, map[string]any{"location": "payload", "identifier": "$.a.model"}),
		"route path location without a group": withField(ParamModelLocation,
			map[string]any{"location": "pathParam", "identifier": "models/[a-z]+"}),
		"route unknown location":       withField(ParamModelLocation, map[string]any{"location": "cookie", "identifier": "m"}),
		"route location not an object": withField(ParamModelLocation, "payload"),
		"route unknown format":         withField(ParamFormat, "cohere"),
		"dispatch error not an object": {ParamDispatchError: []any{}},
		"route block null":             {ParamRoute: nil},
	} {
		t.Run(name, func(t *testing.T) {
			instance, err := GetPolicy(policy.PolicyMetadata{}, params)
			if err == nil {
				t.Fatalf("GetPolicy accepted the block and returned %T", instance)
			}
			if instance != nil {
				t.Fatalf("GetPolicy returned an instance alongside its error")
			}
		})
	}
}

// A key beside the job block is not a reason to refuse the chain: the engine
// merges system parameters into what the controller wrote.
func TestKeysBesideTheJobBlockAreIgnored(t *testing.T) {
	instance := mustPolicy(t, map[string]any{
		ParamCatalog: map[string]any{ParamCatalogModels: []any{}},
		"attachedTo": "api",
	})
	if _, ok := instance.(*catalogPolicy); !ok {
		t.Fatalf("got %T, want the catalog job", instance)
	}
}

// Each job takes part in exactly the phases it needs. The route job commits a
// known provider in the request headers and decides in the body; the two scope
// jobs stream the response, so a Model chain that can stream still does; the two
// answering jobs touch nothing but the request headers.
func TestModeMatchesEachJob(t *testing.T) {
	for name, tc := range map[string]struct {
		params map[string]any
		want   policy.ProcessingMode
	}{
		"route": {
			params: routeBlock(target("primary", "openai-prod", "gpt-4o", "openai", false)),
			want: policy.ProcessingMode{
				RequestHeaderMode:  policy.HeaderModeProcess,
				RequestBodyMode:    policy.BodyModeBuffer,
				ResponseHeaderMode: policy.HeaderModeProcess,
				ResponseBodyMode:   policy.BodyModeStream,
			},
		},
		"providerScope": {
			params: map[string]any{ParamProviderScope: map[string]any{}},
			want: policy.ProcessingMode{
				RequestHeaderMode:  policy.HeaderModeSkip,
				RequestBodyMode:    policy.BodyModeSkip,
				ResponseHeaderMode: policy.HeaderModeProcess,
				ResponseBodyMode:   policy.BodyModeStream,
			},
		},
		"dispatchError": {
			params: map[string]any{ParamDispatchError: map[string]any{}},
			want: policy.ProcessingMode{
				RequestHeaderMode:  policy.HeaderModeProcess,
				RequestBodyMode:    policy.BodyModeSkip,
				ResponseHeaderMode: policy.HeaderModeSkip,
				ResponseBodyMode:   policy.BodyModeSkip,
			},
		},
		"catalog": {
			params: map[string]any{ParamCatalog: map[string]any{ParamCatalogModels: []any{}}},
			want: policy.ProcessingMode{
				RequestHeaderMode:  policy.HeaderModeProcess,
				RequestBodyMode:    policy.BodyModeSkip,
				ResponseHeaderMode: policy.HeaderModeSkip,
				ResponseBodyMode:   policy.BodyModeSkip,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := mustPolicy(t, tc.params).Mode(); got != tc.want {
				t.Fatalf("Mode() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// The kernel dispatches by interface as well as by Mode, so a job declaring a
// phase it does not implement would be skipped without a word. The streaming
// interface matters most: a policy declaring STREAM without it drops the whole
// chain to buffered responses.
func TestEachJobImplementsThePhasesItDeclares(t *testing.T) {
	route := mustPolicy(t, routeBlock(target("primary", "openai-prod", "gpt-4o", "openai", false)))
	if _, ok := route.(policy.RequestHeaderPolicy); !ok {
		t.Error("route job does not implement RequestHeaderPolicy")
	}
	if _, ok := route.(policy.RequestPolicy); !ok {
		t.Error("route job does not implement RequestPolicy")
	}
	if _, ok := route.(policy.ResponseHeaderPolicy); !ok {
		t.Error("route job does not implement ResponseHeaderPolicy")
	}
	if _, ok := route.(policy.StreamingResponsePolicy); !ok {
		t.Error("route job does not implement StreamingResponsePolicy")
	}

	scope := mustPolicy(t, map[string]any{ParamProviderScope: map[string]any{}})
	if _, ok := scope.(policy.ResponseHeaderPolicy); !ok {
		t.Error("providerScope job does not implement ResponseHeaderPolicy")
	}
	if _, ok := scope.(policy.StreamingResponsePolicy); !ok {
		t.Error("providerScope job does not implement StreamingResponsePolicy")
	}

	for _, params := range []map[string]any{
		{ParamDispatchError: map[string]any{}},
		{ParamCatalog: map[string]any{ParamCatalogModels: []any{}}},
	} {
		if _, ok := mustPolicy(t, params).(policy.RequestHeaderPolicy); !ok {
			t.Errorf("%v job does not implement RequestHeaderPolicy", params)
		}
	}
}
