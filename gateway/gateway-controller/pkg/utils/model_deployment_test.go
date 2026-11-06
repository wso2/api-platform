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

package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
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
)

type modelServiceFixture struct {
	t       *testing.T
	svc     *LLMDeploymentService
	db      storage.Storage
	store   *storage.ConfigStore
	hub     *mockLLMEventHub
	logger  *slog.Logger
	enabled config.ModelsConfig
}

func newModelServiceFixture(t *testing.T, enabled bool) *modelServiceFixture {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	metrics.Init()
	db, err := storage.NewStorage(storage.BackendConfig{
		Type: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "gw.db"), GatewayID: testGatewayID,
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	templates, err := NewLLMTemplateLoader(logger).LoadTemplatesFromDirectory("../../default-llm-provider-templates")
	require.NoError(t, err)

	defs := map[string]models.PolicyDefinition{}
	for name, version := range map[string]string{
		"api-key-auth": "v1.2.1", "set-headers": "v1.0.0", "model-round-robin": "v1.1.1",
		"wso2_apip_sys_model": "v1.0.0", "jwt-auth": "v1.0.0",
	} {
		defs[name+"|"+version] = models.PolicyDefinition{Name: name, Version: version}
	}
	// A transformer declares the providerId it routes to; a policy that does not
	// cannot be one.
	transformerParams := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"providerId":       map[string]interface{}{"type": "string"},
			"anthropicVersion": map[string]interface{}{"type": "string"},
		},
	}
	defs["openai-to-anthropic-transformer|v0.9.2"] = models.PolicyDefinition{
		Name: "openai-to-anthropic-transformer", Version: "v0.9.2", Parameters: &transformerParams}

	store := storage.NewConfigStore()
	hub := &mockLLMEventHub{}
	routerCfg := &config.RouterConfig{ListenerPort: 8080, VHosts: config.VHostsConfig{Main: config.VHostEntry{Default: "*"}}}
	apiSvc := newTestAPIDeploymentServiceWithHub(store, db, nil, nil, routerCfg, hub, testGatewayID)
	svc := NewLLMDeploymentService(store, db, nil, nil, templates, apiSvc, routerCfg,
		NewLoadedPolicyVersionResolver(defs), config.NewPolicyValidator(defs, nil))
	mc := config.ModelsConfig{Enabled: enabled, Endpoint: config.ModelEndpointConfig{Provision: true, Context: "/wso2/v1"}}
	svc.SetModelsConfig(mc)
	return &modelServiceFixture{t: t, svc: svc, db: db, store: store, hub: hub, logger: logger, enabled: mc}
}

func (f *modelServiceFixture) params(body any) LLMDeploymentParams {
	data, err := json.Marshal(body)
	require.NoError(f.t, err)
	return LLMDeploymentParams{Data: data, ContentType: "application/json", Origin: models.OriginGatewayAPI, Logger: f.logger}
}

// provisionDefaultEndpoint provisions wso2, the endpoint modelBody names.
func (f *modelServiceFixture) provisionDefaultEndpoint() {
	f.t.Helper()
	require.NoError(f.t, f.svc.ProvisionModelEndpoint(f.logger))
}

func (f *modelServiceFixture) createDirectProvider(handle string) {
	f.t.Helper()
	_, err := f.svc.CreateLLMProvider(f.params(map[string]any{
		"apiVersion": "gateway.api-platform.wso2.com/v1",
		"kind":       "LlmProvider",
		"metadata":   map[string]any{"name": handle},
		"spec": map[string]any{
			"displayName": handle, "version": "v1.0", "template": "openai", "exposure": "none",
			"upstream": map[string]any{
				"url":  "https://api.openai.com/v1",
				"auth": map[string]any{"type": "api-key", "header": "Authorization", "value": "Bearer sk-direct"},
			},
		},
	}))
	require.NoError(f.t, err)
}

func (f *modelServiceFixture) createDirectAnthropicProvider(handle string) {
	f.t.Helper()
	_, err := f.svc.CreateLLMProvider(f.params(map[string]any{
		"apiVersion": "gateway.api-platform.wso2.com/v1",
		"kind":       "LlmProvider",
		"metadata":   map[string]any{"name": handle},
		"spec": map[string]any{
			"displayName": handle, "version": "v1.0", "template": "anthropic", "exposure": "none",
			"upstream": map[string]any{
				"url":  "https://api.anthropic.com",
				"auth": map[string]any{"type": "api-key", "header": "x-api-key", "value": "sk-ant"},
			},
		},
	}))
	require.NoError(f.t, err)
}

func (f *modelServiceFixture) createRoutedProvider(handle string) {
	f.t.Helper()
	_, err := f.svc.CreateLLMProvider(f.params(map[string]any{
		"apiVersion": "gateway.api-platform.wso2.com/v1",
		"kind":       "LlmProvider",
		"metadata":   map[string]any{"name": handle},
		"spec": map[string]any{
			"displayName": handle, "version": "v1.0", "template": "openai", "context": "/" + handle,
			"upstream":      map[string]any{"url": "https://api.openai.com/v1"},
			"accessControl": map[string]any{"mode": "allow_all"},
			"globalPolicies": []any{map[string]any{"name": "api-key-auth", "version": "v1",
				"params": map[string]any{"key": "x-api-key", "in": "header"}}},
		},
	}))
	require.NoError(f.t, err)
}

func modelBody(handle, modelName string, targets ...map[string]any) map[string]any {
	ts := make([]any, 0, len(targets))
	for _, t := range targets {
		ts = append(ts, t)
	}
	return map[string]any{
		"apiVersion": "gateway.api-platform.wso2.com/v1",
		"kind":       "Model",
		"metadata":   map[string]any{"name": handle},
		"spec": map[string]any{"displayName": handle, "version": "v1.0", "modelName": modelName,
			"endpoints": []any{"wso2"}, "targets": ts},
	}
}

// endpointBody is a ModelEndpoint body.
func endpointBody(handle, context string, mutate ...func(spec map[string]any)) map[string]any {
	spec := map[string]any{"displayName": "Endpoint " + handle, "version": "v1.0"}
	if context != "" {
		spec["context"] = context
	}
	for _, m := range mutate {
		m(spec)
	}
	return map[string]any{
		"apiVersion": "gateway.api-platform.wso2.com/v1",
		"kind":       "ModelEndpoint",
		"metadata":   map[string]any{"name": handle},
		"spec":       spec,
	}
}

func modelTarget(alias, provider, name string) map[string]any {
	return map[string]any{"name": name, "provider": provider, "alias": alias}
}

func (f *modelServiceFixture) eventsOf(t eventhub.EventType) []eventhub.Event {
	var out []eventhub.Event
	for _, e := range f.hub.publishedEvents {
		if e.event.EventType == t {
			out = append(out, e.event)
		}
	}
	return out
}

func TestModelService_RefusedWhileDisabled(t *testing.T) {
	f := newModelServiceFixture(t, false)
	f.createDirectProvider("openai")
	_, err := f.svc.CreateModel(f.params(modelBody("gpt", "gpt-4o", modelTarget("a", "openai", "gpt-4o"))))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrModelValidation))
	assert.True(t, errors.Is(err, ErrModelsDisabled))
	assert.Empty(t, f.eventsOf(eventhub.EventTypeModel))
}

func TestModelService_CreateUpdateDelete(t *testing.T) {
	f := newModelServiceFixture(t, true)
	f.provisionDefaultEndpoint()
	f.createDirectProvider("openai")

	result, err := f.svc.CreateModel(f.params(modelBody("gpt", "gpt-4o", modelTarget("a", "openai", "gpt-4o"))))
	require.NoError(t, err)
	assert.Equal(t, models.KindModel, result.StoredConfig.Kind)
	events := f.eventsOf(eventhub.EventTypeModel)
	require.Len(t, events, 1)
	assert.Equal(t, "CREATE", events[0].Action)

	// The model name is unique across Models.
	_, err = f.svc.CreateModel(f.params(modelBody("gpt-copy", "gpt-4o", modelTarget("a", "openai", "gpt-4o"))))
	require.Error(t, err)
	assert.True(t, storage.IsConflictError(err), "got %v", err)

	updated := modelBody("gpt", "gpt-4o-latest", modelTarget("a", "openai", "gpt-4o"))
	stepPastLastWrite()
	_, err = f.svc.UpdateModel("gpt", f.params(updated))
	require.NoError(t, err)
	stored, err := f.svc.GetModelByHandle("gpt")
	require.NoError(t, err)
	assert.Equal(t, "gpt-4o-latest", stored.SourceConfiguration.(api.ModelConfiguration).Spec.ModelName)

	list := f.svc.ListModels(api.ListModelsParams{ModelName: ptrTo("gpt-4o-latest")})
	assert.Len(t, list, 1)

	_, err = f.svc.DeleteModel("gpt", "", f.logger)
	require.NoError(t, err)
	_, err = f.svc.GetModelByHandle("gpt")
	assert.True(t, storage.IsNotFoundError(err))
}

// A missing endpoint and a missing provider are both reported, in one error.
func TestModelService_ResolutionErrorsAreValidationErrors(t *testing.T) {
	f := newModelServiceFixture(t, true)
	_, err := f.svc.CreateModel(f.params(modelBody("gpt", "gpt-4o", modelTarget("a", "missing", "gpt-4o"))))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrModelValidation))
	assert.Contains(t, err.Error(), "2 error(s)")
	assert.Contains(t, err.Error(), "spec.endpoints[0]: model endpoint 'wso2' not found")
	assert.Contains(t, err.Error(), "provider 'missing' not found")
}

func TestModelService_InheritsTargetCredentialOnUpdate(t *testing.T) {
	f := newModelServiceFixture(t, true)
	f.provisionDefaultEndpoint()
	f.createRoutedProvider("routed")

	target := modelTarget("a", "routed", "gpt-4o")
	target["auth"] = map[string]any{"type": "api-key", "header": "x-api-key", "value": "issued-key"}
	_, err := f.svc.CreateModel(f.params(modelBody("gpt", "gpt-4o", target)))
	require.NoError(t, err)

	// A read-modify-write sees no credential (it is write-only) and must keep it.
	withoutValue := modelTarget("a", "routed", "gpt-4o")
	withoutValue["auth"] = map[string]any{"type": "api-key", "header": "x-api-key"}
	stepPastLastWrite()
	_, err = f.svc.UpdateModel("gpt", f.params(modelBody("gpt", "gpt-4o", withoutValue)))
	require.NoError(t, err)

	stored, err := f.svc.GetModelByHandle("gpt")
	require.NoError(t, err)
	auth := stored.SourceConfiguration.(api.ModelConfiguration).Spec.Targets[0].Auth
	require.NotNil(t, auth)
	require.NotNil(t, auth.Value)
	assert.Equal(t, "issued-key", *auth.Value)

	// The alias, not the model, identifies the target: moving it to a new model
	// under the same alias keeps its credential.
	newModel := modelTarget("a", "routed", "gpt-4o-mini")
	newModel["auth"] = map[string]any{"type": "api-key", "header": "x-api-key"}
	stepPastLastWrite()
	_, err = f.svc.UpdateModel("gpt", f.params(modelBody("gpt", "gpt-4o", newModel)))
	require.NoError(t, err)
	stored, err = f.svc.GetModelByHandle("gpt")
	require.NoError(t, err)
	auth = stored.SourceConfiguration.(api.ModelConfiguration).Spec.Targets[0].Auth
	require.NotNil(t, auth)
	require.NotNil(t, auth.Value)
	assert.Equal(t, "issued-key", *auth.Value)

	// Pointing the target at another provider inherits nothing.
	f.createRoutedProvider("other")
	moved := modelTarget("a", "other", "gpt-4o")
	moved["auth"] = map[string]any{"type": "api-key", "header": "x-api-key"}
	_, err = f.svc.UpdateModel("gpt", f.params(modelBody("gpt", "gpt-4o", moved)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "value")
}

func TestModelService_ReferencedProviderCannotBeDeletedOrUndeployed(t *testing.T) {
	f := newModelServiceFixture(t, true)
	f.provisionDefaultEndpoint()
	f.createDirectProvider("openai")
	_, err := f.svc.CreateModel(f.params(modelBody("gpt", "gpt-4o", modelTarget("a", "openai", "gpt-4o"))))
	require.NoError(t, err)

	_, err = f.svc.DeleteLLMProvider("openai", "", f.logger)
	require.Error(t, err)
	assert.True(t, storage.IsConflictError(err))
	assert.Contains(t, err.Error(), "gpt")
	_, err = f.svc.GetLLMProviderByHandle("openai")
	require.NoError(t, err, "a refused delete leaves the provider in place")

	undeploy := map[string]any{
		"apiVersion": "gateway.api-platform.wso2.com/v1",
		"kind":       "LlmProvider",
		"metadata":   map[string]any{"name": "openai"},
		"spec": map[string]any{
			"displayName": "openai", "version": "v1.0", "template": "openai", "exposure": "none",
			"deploymentState": "undeployed",
			"upstream":        map[string]any{"url": "https://api.openai.com/v1"},
		},
	}
	_, err = f.svc.UpdateLLMProvider("openai", f.params(undeploy))
	require.Error(t, err)
	assert.True(t, storage.IsConflictError(err))

	// With the Model gone, the provider can be deleted.
	_, err = f.svc.DeleteModel("gpt", "", f.logger)
	require.NoError(t, err)
	_, err = f.svc.DeleteLLMProvider("openai", "", f.logger)
	require.NoError(t, err)
}

func TestModelEndpointProvisioning(t *testing.T) {
	f := newModelServiceFixture(t, true)

	require.NoError(t, f.svc.ProvisionModelEndpoint(f.logger))
	cfg, err := f.db.GetConfigByKindAndHandle(models.KindModelEndpoint, models.DefaultModelEndpointHandle)
	require.NoError(t, err)
	assert.Equal(t, models.ModelEndpointUUID(models.DefaultModelEndpointHandle), cfg.UUID)
	assert.Empty(t, cfg.CPSyncStatus, "a model endpoint is not pushed to the control plane yet")
	endpoint := cfg.SourceConfiguration.(api.ModelEndpointConfiguration)
	assert.Equal(t, models.DefaultModelEndpointDisplayName, endpoint.Spec.DisplayName)
	assert.Equal(t, "/wso2/v1", *endpoint.Spec.Context)
	require.NotNil(t, endpoint.Spec.Policies)
	auth := (*endpoint.Spec.Policies)[0]
	assert.Equal(t, "api-key-auth", auth.Name)
	assert.Equal(t, "v1", auth.Version)
	assert.Equal(t, "Authorization", (*auth.Params)["key"])
	assert.Equal(t, "Bearer", (*auth.Params)["valuePrefix"])
	// The seed entries whose transformer this build has loaded, at its major version.
	require.NotNil(t, endpoint.Spec.Transformers)
	require.Len(t, *endpoint.Spec.Transformers, 1, "only the Anthropic transformer is loaded in this fixture")
	entry := (*endpoint.Spec.Transformers)[0]
	assert.Equal(t, "anthropic", *entry.Template)
	assert.Equal(t, "v0", *entry.Version)
	created := len(f.eventsOf(eventhub.EventTypeModelEndpoint))
	assert.Equal(t, 1, created)

	// Idempotent: a second start changes nothing.
	require.NoError(t, f.svc.ProvisionModelEndpoint(f.logger))
	assert.Len(t, f.eventsOf(eventhub.EventTypeModelEndpoint), created)

	// The context follows configuration; everything an operator owns is kept.
	moved := config.ModelsConfig{Enabled: true, Endpoint: config.ModelEndpointConfig{Provision: true, Context: "/ai/v1"}}
	f.svc.SetModelsConfig(moved)
	stepPastLastWrite()
	require.NoError(t, f.svc.ProvisionModelEndpoint(f.logger))
	cfg, err = f.db.GetConfigByKindAndHandle(models.KindModelEndpoint, models.DefaultModelEndpointHandle)
	require.NoError(t, err)
	assert.Equal(t, "/ai/v1", *cfg.SourceConfiguration.(api.ModelEndpointConfiguration).Spec.Context)

	// With provisioning off nothing is provisioned.
	f2 := newModelServiceFixture(t, true)
	f2.svc.SetModelsConfig(config.ModelsConfig{Enabled: true, Endpoint: config.ModelEndpointConfig{Context: "/wso2/v1"}})
	require.NoError(t, f2.svc.ProvisionModelEndpoint(f2.logger))
	_, err = f2.db.GetConfigByKindAndHandle(models.KindModelEndpoint, models.DefaultModelEndpointHandle)
	assert.True(t, storage.IsNotFoundError(err))
}

func TestModelEndpointProvisioning_RefusesWithoutAuthentication(t *testing.T) {
	f := newModelServiceFixture(t, true)
	f.svc.policyValidator = config.NewPolicyValidator(map[string]models.PolicyDefinition{}, nil)
	err := f.svc.ProvisionModelEndpoint(f.logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec.policies cannot default to it")
}

// Only an omitted spec.policies defaults to api-key-auth. An explicit list, even an
// empty one, is the author's choice and is stored as given.
func TestModelEndpointService_ExplicitEmptyPoliciesAreKept(t *testing.T) {
	f := newModelServiceFixture(t, true)
	_, err := f.svc.CreateModelEndpoint(f.params(endpointBody("open", "/open/v1", func(spec map[string]any) {
		spec["policies"] = []any{}
	})))
	require.NoError(t, err)

	stored, err := f.svc.GetModelEndpointByHandle("open")
	require.NoError(t, err)
	policies := stored.SourceConfiguration.(api.ModelEndpointConfiguration).Spec.Policies
	require.NotNil(t, policies)
	assert.Empty(t, *policies)
}

func TestModelEndpointService_CreateUpdateListDelete(t *testing.T) {
	f := newModelServiceFixture(t, true)

	// A context is required for every endpoint but the provisioned one.
	_, err := f.svc.CreateModelEndpoint(f.params(endpointBody("partners", "")))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrModelEndpointValidation))
	assert.Contains(t, err.Error(), "spec.context")

	result, err := f.svc.CreateModelEndpoint(f.params(endpointBody("partners", "/partners/v1", func(spec map[string]any) {
		spec["vhost"] = "partners.example.com"
	})))
	require.NoError(t, err)
	assert.Equal(t, models.KindModelEndpoint, result.StoredConfig.Kind)
	assert.Equal(t, models.ModelEndpointUUID("partners"), result.StoredConfig.UUID)
	events := f.eventsOf(eventhub.EventTypeModelEndpoint)
	require.Len(t, events, 1)
	assert.Equal(t, "CREATE", events[0].Action)

	// With no policies of its own an endpoint authenticates with api-key-auth.
	stored, err := f.svc.GetModelEndpointByHandle("partners")
	require.NoError(t, err)
	policies := stored.SourceConfiguration.(api.ModelEndpointConfiguration).Spec.Policies
	require.NotNil(t, policies)
	assert.Equal(t, "api-key-auth", (*policies)[0].Name)

	// Creating it again is a conflict.
	_, err = f.svc.CreateModelEndpoint(f.params(endpointBody("partners", "/partners/v1")))
	require.Error(t, err)
	assert.True(t, storage.IsConflictError(err))

	// Another endpoint may not take the same display name: API-level counters key on it.
	_, err = f.svc.CreateModelEndpoint(f.params(endpointBody("other", "/other/v1", func(spec map[string]any) {
		spec["displayName"] = "Endpoint partners"
	})))
	require.Error(t, err)
	assert.True(t, storage.IsConflictError(err))
	assert.Contains(t, err.Error(), "display name")

	// Nor its context on an overlapping vhost.
	_, err = f.svc.CreateModelEndpoint(f.params(endpointBody("same-context", "/partners/v1")))
	require.Error(t, err)
	assert.True(t, storage.IsConflictError(err))
	assert.Contains(t, err.Error(), "ModelEndpoint 'partners'")
	// A different explicit vhost does not overlap.
	_, err = f.svc.CreateModelEndpoint(f.params(endpointBody("other-host", "/partners/v1", func(spec map[string]any) {
		spec["vhost"] = "other.example.com"
	})))
	require.NoError(t, err)

	list := f.svc.ListModelEndpoints(api.ListModelEndpointsParams{Vhost: ptrTo("partners.example.com")})
	require.Len(t, list, 1)
	assert.Equal(t, "partners", list[0].Handle)
	assert.Len(t, f.svc.ListModelEndpoints(api.ListModelEndpointsParams{Context: ptrTo("/partners/v1")}), 2)

	stepPastLastWrite()
	_, err = f.svc.UpdateModelEndpoint("partners", f.params(endpointBody("partners", "/partners/v2")))
	require.NoError(t, err)
	stored, err = f.svc.GetModelEndpointByHandle("partners")
	require.NoError(t, err)
	assert.Equal(t, "/partners/v2", *stored.SourceConfiguration.(api.ModelEndpointConfiguration).Spec.Context)

	_, err = f.svc.UpdateModelEndpoint("partners", f.params(endpointBody("renamed", "/partners/v2")))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrModelEndpointValidation))

	_, err = f.svc.DeleteModelEndpoint("partners", "", f.logger)
	require.NoError(t, err)
	_, err = f.svc.GetModelEndpointByHandle("partners")
	assert.True(t, storage.IsNotFoundError(err))
}

func TestModelEndpointService_InboundTemplate(t *testing.T) {
	f := newModelServiceFixture(t, true)

	_, err := f.svc.CreateModelEndpoint(f.params(endpointBody("bedrock", "/bedrock-native", func(spec map[string]any) {
		spec["inboundTemplate"] = "awsbedrock"
	})))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrModelEndpointValidation))
	assert.Contains(t, err.Error(), "spec.inboundTemplate")

	for _, tmpl := range []string{"anthropic", "gemini"} {
		_, err = f.svc.CreateModelEndpoint(f.params(endpointBody(tmpl+"-native", "/"+tmpl+"-native", func(spec map[string]any) {
			spec["inboundTemplate"] = tmpl
		})))
		require.NoError(t, err, tmpl)
	}
}

func TestModelEndpointService_TransformerContract(t *testing.T) {
	f := newModelServiceFixture(t, true)

	// A policy that is not loaded is refused.
	_, err := f.svc.CreateModelEndpoint(f.params(endpointBody("p", "/p", func(spec map[string]any) {
		spec["transformers"] = []any{map[string]any{"template": "anthropic", "type": "no-such-transformer", "version": "v1"}}
	})))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrModelEndpointValidation))

	// A loaded policy that declares no providerId cannot be a transformer.
	_, err = f.svc.CreateModelEndpoint(f.params(endpointBody("p", "/p", func(spec map[string]any) {
		spec["transformers"] = []any{map[string]any{"template": "anthropic", "type": "set-headers", "version": "v1"}}
	})))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "providerId")

	// Its params are checked against its schema, placeholders filled.
	_, err = f.svc.CreateModelEndpoint(f.params(endpointBody("p", "/p", func(spec map[string]any) {
		spec["transformers"] = []any{map[string]any{"template": "anthropic", "type": "openai-to-anthropic-transformer",
			"version": "v0", "params": map[string]any{"anthropicVersion": "{apiVersion:2023-06-01}"}}}
	})))
	require.NoError(t, err)
}

// What the API may and may not do to the provisioned endpoint.
func TestModelEndpointService_DefaultEndpointRules(t *testing.T) {
	f := newModelServiceFixture(t, true)

	// It cannot be created through the API while the gateway provisions it.
	_, err := f.svc.CreateModelEndpoint(f.params(endpointBody("wso2", "")))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrModelEndpointReserved), "%v", err)

	require.NoError(t, f.svc.ProvisionModelEndpoint(f.logger))

	// Its context is configuration.
	_, err = f.svc.UpdateModelEndpoint("wso2", f.params(endpointBody("wso2", "/elsewhere")))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrModelEndpointReserved))

	// An update that leaves the context out keeps the configured one.
	stepPastLastWrite()
	_, err = f.svc.UpdateModelEndpoint("wso2", f.params(endpointBody("wso2", "", func(spec map[string]any) {
		spec["displayName"] = "Renamed Endpoint"
	})))
	require.NoError(t, err)
	cfg, err := f.db.GetConfigByKindAndHandle(models.KindModelEndpoint, "wso2")
	require.NoError(t, err)
	assert.Equal(t, "/wso2/v1", *cfg.SourceConfiguration.(api.ModelEndpointConfiguration).Spec.Context)
	assert.Equal(t, "Renamed Endpoint", cfg.DisplayName)

	// It cannot be deleted while provisioning is on.
	_, err = f.svc.DeleteModelEndpoint("wso2", "", f.logger)
	require.Error(t, err)
	assert.True(t, storage.IsConflictError(err))
	assert.Contains(t, err.Error(), "provisioned by the gateway")

	// With provisioning off it is an ordinary endpoint.
	f.svc.SetModelsConfig(config.ModelsConfig{Enabled: true, Endpoint: config.ModelEndpointConfig{Context: "/wso2/v1"}})
	_, err = f.svc.DeleteModelEndpoint("wso2", "", f.logger)
	require.NoError(t, err)
	_, err = f.svc.CreateModelEndpoint(f.params(endpointBody("wso2", "")))
	require.NoError(t, err)
}

func TestModelEndpointService_DeleteRefusedWhileAModelNamesIt(t *testing.T) {
	f := newModelServiceFixture(t, true)
	f.createDirectProvider("openai")
	_, err := f.svc.CreateModelEndpoint(f.params(endpointBody("partners", "/partners/v1")))
	require.NoError(t, err)
	body := modelBody("gpt", "gpt-4o", modelTarget("a", "openai", "gpt-4o"))
	body["spec"].(map[string]any)["endpoints"] = []any{"partners"}
	_, err = f.svc.CreateModel(f.params(body))
	require.NoError(t, err)

	_, err = f.svc.DeleteModelEndpoint("partners", "", f.logger)
	require.Error(t, err)
	assert.True(t, storage.IsConflictError(err))
	assert.Contains(t, err.Error(), "gpt")

	_, err = f.svc.DeleteModel("gpt", "", f.logger)
	require.NoError(t, err)
	_, err = f.svc.DeleteModelEndpoint("partners", "", f.logger)
	require.NoError(t, err)
}

// An endpoint change that would stop it serving a Model it serves today is
// refused, naming the Model; one that breaks nothing is not.
func TestModelEndpointService_ChangeThatBreaksAModelIsRefused(t *testing.T) {
	f := newModelServiceFixture(t, true)
	require.NoError(t, f.svc.ProvisionModelEndpoint(f.logger))
	f.createDirectAnthropicProvider("claude-prod")
	_, err := f.svc.CreateModel(f.params(modelBody("claude", "claude", modelTarget("a", "claude-prod", "claude-haiku"))))
	require.NoError(t, err)

	authParams := map[string]any{"key": "Authorization", "in": "header", "valuePrefix": "Bearer"}
	withoutTransformers := endpointBody("wso2", "", func(spec map[string]any) {
		spec["policies"] = []any{map[string]any{"name": "api-key-auth", "version": "v1", "params": authParams}}
		spec["transformers"] = []any{}
	})
	stepPastLastWrite()
	_, err = f.svc.UpdateModelEndpoint("wso2", f.params(withoutTransformers))
	require.Error(t, err)
	assert.True(t, storage.IsConflictError(err))
	assert.Contains(t, err.Error(), "claude")
	assert.Contains(t, err.Error(), "no transformer for provider 'claude-prod'")

	// Switching the inbound format would also take the Model off.
	stepPastLastWrite()
	_, err = f.svc.UpdateModelEndpoint("wso2", f.params(endpointBody("wso2", "", func(spec map[string]any) {
		spec["inboundTemplate"] = "gemini"
	})))
	require.Error(t, err)
	assert.True(t, storage.IsConflictError(err))

	// A change the Model survives is accepted.
	stored, err := f.svc.GetModelEndpointByHandle("wso2")
	require.NoError(t, err)
	current := stored.SourceConfiguration.(api.ModelEndpointConfiguration)
	current.Spec.DisplayName = "Renamed"
	stepPastLastWrite()
	_, err = f.svc.UpdateModelEndpoint("wso2", f.params(current))
	require.NoError(t, err)

	// A Model the endpoint could not serve before does not block a change. A save
	// refuses such a Model, but a provider change can still make one: here the
	// provider moves to Mistral, and no Mistral transformer is loaded here.
	mistral := func(template string) map[string]any {
		return map[string]any{
			"apiVersion": "gateway.api-platform.wso2.com/v1",
			"kind":       "LlmProvider",
			"metadata":   map[string]any{"name": "mistral-prod"},
			"spec": map[string]any{
				"displayName": "mistral-prod", "version": "v1.0", "template": template, "exposure": "none",
				"upstream": map[string]any{"url": "https://api.mistral.ai"},
			},
		}
	}
	_, err = f.svc.CreateLLMProvider(f.params(mistral("openai")))
	require.NoError(t, err)
	_, err = f.svc.CreateModel(f.params(modelBody("mistral", "mistral", modelTarget("a", "mistral-prod", "mistral-large"))))
	require.NoError(t, err)
	stepPastLastWrite()
	_, err = f.svc.UpdateLLMProvider("mistral-prod", f.params(mistral("mistralai")))
	require.NoError(t, err, "a provider change is not checked against the endpoints its Models name")
	current.Spec.DisplayName = "Renamed Again"
	stepPastLastWrite()
	_, err = f.svc.UpdateModelEndpoint("wso2", f.params(current))
	require.NoError(t, err)
}

// failingModelListDB fails listing Models and passes every other call through.
type failingModelListDB struct {
	storage.Storage
}

func (d failingModelListDB) GetAllConfigsByKind(kind string) ([]*models.StoredConfig, error) {
	if kind == models.KindModel {
		return nil, errors.New("database unavailable")
	}
	return d.Storage.GetAllConfigsByKind(kind)
}

// An endpoint change whose Models cannot be listed is refused, not saved without
// the check that it breaks none of them.
func TestModelEndpointService_ChangeGuardFailsClosed(t *testing.T) {
	f := newModelServiceFixture(t, true)
	require.NoError(t, f.svc.ProvisionModelEndpoint(f.logger))

	stored, err := f.svc.GetModelEndpointByHandle("wso2")
	require.NoError(t, err)
	current := stored.SourceConfiguration.(api.ModelEndpointConfiguration)
	before := current.Spec.DisplayName
	current.Spec.DisplayName = "Renamed"

	f.svc.db = failingModelListDB{Storage: f.db}
	stepPastLastWrite()
	_, err = f.svc.UpdateModelEndpoint("wso2", f.params(current))
	require.Error(t, err)
	assert.False(t, storage.IsConflictError(err), "a failed check is not a conflict: %v", err)
	assert.Contains(t, err.Error(), "database unavailable")

	f.svc.db = f.db
	unchanged, err := f.svc.GetModelEndpointByHandle("wso2")
	require.NoError(t, err)
	assert.Equal(t, before, unchanged.SourceConfiguration.(api.ModelEndpointConfiguration).Spec.DisplayName)
}

func TestModelService_ModelNeedsEndpoints(t *testing.T) {
	f := newModelServiceFixture(t, true)
	f.createDirectProvider("openai")
	body := modelBody("gpt", "gpt-4o", modelTarget("a", "openai", "gpt-4o"))
	delete(body["spec"].(map[string]any), "endpoints")
	_, err := f.svc.CreateModel(f.params(body))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrModelValidation))
	assert.Contains(t, err.Error(), "spec.endpoints")

	// Every endpoint named must exist; the error names each one that does not.
	f.provisionDefaultEndpoint()
	body["spec"].(map[string]any)["endpoints"] = []any{"wso2", "not-here"}
	_, err = f.svc.CreateModel(f.params(body))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrModelValidation))
	assert.Contains(t, err.Error(), "1 error(s)")
	assert.Contains(t, err.Error(), "spec.endpoints[1]: model endpoint 'not-here' not found")
	_, err = f.svc.GetModelByHandle("gpt")
	assert.True(t, storage.IsNotFoundError(err), "a refused Model is not saved")

	body["spec"].(map[string]any)["endpoints"] = []any{"wso2"}
	_, err = f.svc.CreateModel(f.params(body))
	require.NoError(t, err)
	stored, err := f.svc.GetModelByHandle("gpt")
	require.NoError(t, err)
	assert.Empty(t, stored.CPSyncStatus, "a Model is not pushed to the control plane yet")

	// Adding one on an update is refused the same way, and the Model is unchanged.
	body["spec"].(map[string]any)["endpoints"] = []any{"wso2", "partners"}
	stepPastLastWrite()
	_, err = f.svc.UpdateModel("gpt", f.params(body))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrModelValidation))
	assert.Contains(t, err.Error(), "model endpoint 'partners' not found")
	stored, err = f.svc.GetModelByHandle("gpt")
	require.NoError(t, err)
	assert.Equal(t, []string{"wso2"}, stored.SourceConfiguration.(api.ModelConfiguration).Spec.Endpoints)

	// Once the endpoint exists, even undeployed, the update is accepted.
	_, err = f.svc.CreateModelEndpoint(f.params(endpointBody("partners", "/partners/v1")))
	require.NoError(t, err)
	stepPastLastWrite()
	_, err = f.svc.UpdateModelEndpoint("partners", f.params(endpointBody("partners", "/partners/v1", func(spec map[string]any) {
		spec["deploymentState"] = "undeployed"
	})))
	require.NoError(t, err)
	stepPastLastWrite()
	_, err = f.svc.UpdateModel("gpt", f.params(body))
	require.NoError(t, err)
}

// An endpoint deleted while the only Model naming it is undeployed leaves that
// handle behind; the Model cannot be saved again until the handle is removed.
func TestModelService_HandleOfADeletedEndpointMustBeRemoved(t *testing.T) {
	f := newModelServiceFixture(t, true)
	f.provisionDefaultEndpoint()
	f.createDirectProvider("openai")
	_, err := f.svc.CreateModelEndpoint(f.params(endpointBody("partners", "/partners/v1")))
	require.NoError(t, err)
	body := modelBody("gpt", "gpt-4o", modelTarget("a", "openai", "gpt-4o"))
	spec := body["spec"].(map[string]any)
	spec["endpoints"] = []any{"wso2", "partners"}
	_, err = f.svc.CreateModel(f.params(body))
	require.NoError(t, err)
	spec["deploymentState"] = "undeployed"
	stepPastLastWrite()
	_, err = f.svc.UpdateModel("gpt", f.params(body))
	require.NoError(t, err)

	_, err = f.svc.DeleteModelEndpoint("partners", "", f.logger)
	require.NoError(t, err, "an undeployed Model does not hold its endpoints")

	delete(spec, "deploymentState")
	stepPastLastWrite()
	_, err = f.svc.UpdateModel("gpt", f.params(body))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec.endpoints[1]: model endpoint 'partners' not found")

	spec["endpoints"] = []any{"wso2"}
	_, err = f.svc.UpdateModel("gpt", f.params(body))
	require.NoError(t, err)
}

func TestCheckModelEndpointStartup(t *testing.T) {
	enabled := config.ModelsConfig{Enabled: true, Endpoint: config.ModelEndpointConfig{Provision: true, Context: "/wso2/v1"}}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	restAt := func(context string) *models.StoredConfig {
		return &models.StoredConfig{
			UUID: "rest", Kind: models.KindRestApi, Handle: "orders", DisplayName: "Orders", Version: "v1.0",
			SourceConfiguration: api.RestAPI{Spec: api.APIConfigData{Context: context}},
		}
	}
	endpointAt := func(handle, context string) *models.StoredConfig {
		return &models.StoredConfig{
			UUID: models.ModelEndpointUUID(handle), Kind: models.KindModelEndpoint, Handle: handle,
			DisplayName: "Endpoint " + handle, Version: "v1.0",
			SourceConfiguration: api.ModelEndpointConfiguration{Metadata: api.Metadata{Name: handle},
				Spec: api.ModelEndpointConfigData{Context: &context}},
		}
	}

	store := storage.NewConfigStore()
	require.NoError(t, CheckModelEndpointStartup(store, enabled, logger))

	// First provisioning does not take over an artifact already at the context.
	require.NoError(t, store.Add(restAt("/wso2/v1/")))
	err := CheckModelEndpointStartup(store, enabled, logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RestApi 'orders'")
	assert.NoError(t, CheckModelEndpointStartup(store, config.ModelsConfig{}, logger), "nothing to check with the feature off")
	assert.NoError(t, CheckModelEndpointStartup(store, config.ModelsConfig{Enabled: true, Endpoint: config.ModelEndpointConfig{Context: "/wso2/v1"}}, logger),
		"nothing is provisioned at the context with provisioning off")

	// Nor does moving the endpoint to a context in use.
	store = storage.NewConfigStore()
	require.NoError(t, store.Add(endpointAt(models.DefaultModelEndpointHandle, "/old/v1")))
	require.NoError(t, store.Add(restAt("/wso2/v1")))
	err = CheckModelEndpointStartup(store, enabled, logger)
	require.Error(t, err, "controller.models.endpoint.context moved onto an artifact")

	// An artifact written later at the context an endpoint already serves is a
	// conflict to report, not a gateway that can no longer restart.
	store = storage.NewConfigStore()
	require.NoError(t, store.Add(endpointAt(models.DefaultModelEndpointHandle, "/wso2/v1")))
	require.NoError(t, store.Add(endpointAt("partners", "/orders")))
	require.NoError(t, CheckModelEndpointStartup(store, enabled, logger))
	assert.Empty(t, logs.String())
	require.NoError(t, store.Add(restAt("/orders")))
	require.NoError(t, store.Add(&models.StoredConfig{
		UUID: "rest-2", Kind: models.KindRestApi, Handle: "shadow", DisplayName: "Shadow", Version: "v1.0",
		SourceConfiguration: api.RestAPI{Spec: api.APIConfigData{Context: "/wso2/v1"}},
	}))
	require.NoError(t, CheckModelEndpointStartup(store, enabled, logger))
	assert.Contains(t, logs.String(), "model_endpoint=partners")
	assert.Contains(t, logs.String(), "other=orders")
	assert.Contains(t, logs.String(), "model_endpoint=wso2")
	assert.Contains(t, logs.String(), "other=shadow")
}

func ptrTo[T any](v T) *T { return &v }

// stepPastLastWrite moves past the millisecond of the previous write. The store's
// timestamp guard ignores an update no newer than the row it would replace, so two
// writes in one millisecond would make the second look stale.
func stepPastLastWrite() { time.Sleep(5 * time.Millisecond) }

// A deployed Model is refused when an endpoint it names cannot serve it, and the
// error says which endpoint and why. One that can serve it everywhere is saved.
func TestModelService_RefusedWhereAnEndpointCannotServeIt(t *testing.T) {
	f := newModelServiceFixture(t, true)
	f.provisionDefaultEndpoint()
	f.createDirectProvider("openai")
	f.createDirectAnthropicProvider("claude-prod")
	for handle, context := range map[string]string{"claude-native": "/anthropic", "claude-two": "/anthropic-two"} {
		_, err := f.svc.CreateModelEndpoint(f.params(endpointBody(handle, context, func(spec map[string]any) {
			spec["inboundTemplate"] = "anthropic"
		})))
		require.NoError(t, err)
	}

	// An OpenAI provider on an Anthropic endpoint with no transformer entries.
	body := modelBody("gpt", "gpt-4o", modelTarget("a", "openai", "gpt-4o"))
	body["spec"].(map[string]any)["endpoints"] = []any{"wso2", "claude-native"}
	_, err := f.svc.CreateModel(f.params(body))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrModelValidation))
	assert.Contains(t, err.Error(), "1 error(s)")
	assert.Contains(t, err.Error(), "spec.endpoints[1]: endpoint 'claude-native' (inbound format 'anthropic') "+
		"has no transformer for provider 'openai' (template 'openai')")
	assert.Contains(t, err.Error(), "with a transformer that reads 'anthropic' requests")
	_, err = f.svc.GetModelByHandle("gpt")
	assert.True(t, storage.IsNotFoundError(err), "a refused Model is not saved")

	// Every endpoint that cannot serve it is reported at once.
	body["spec"].(map[string]any)["endpoints"] = []any{"claude-native", "wso2", "claude-two"}
	_, err = f.svc.CreateModel(f.params(body))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2 error(s)")
	assert.Contains(t, err.Error(), "spec.endpoints[0]: endpoint 'claude-native'")
	assert.Contains(t, err.Error(), "spec.endpoints[2]: endpoint 'claude-two'")

	// An Anthropic provider passes through on the Anthropic endpoints, and wso2
	// has a transformer for it.
	claude := modelBody("claude", "claude", modelTarget("a", "claude-prod", "claude-haiku"))
	claude["spec"].(map[string]any)["endpoints"] = []any{"wso2", "claude-native", "claude-two"}
	_, err = f.svc.CreateModel(f.params(claude))
	require.NoError(t, err)
}

// An undeployed Model is a draft: it is saved even where an endpoint cannot serve
// it, and checked when it is saved as deployed again.
func TestModelService_UndeployedModelIsCheckedWhenDeployed(t *testing.T) {
	f := newModelServiceFixture(t, true)
	f.provisionDefaultEndpoint()
	f.createDirectProvider("openai")
	_, err := f.svc.CreateModelEndpoint(f.params(endpointBody("claude-native", "/anthropic", func(spec map[string]any) {
		spec["inboundTemplate"] = "anthropic"
	})))
	require.NoError(t, err)
	body := modelBody("gpt", "gpt-4o", modelTarget("a", "openai", "gpt-4o"))
	_, err = f.svc.CreateModel(f.params(body))
	require.NoError(t, err)

	spec := body["spec"].(map[string]any)
	spec["endpoints"] = []any{"wso2", "claude-native"}
	spec["deploymentState"] = "undeployed"
	stepPastLastWrite()
	_, err = f.svc.UpdateModel("gpt", f.params(body))
	require.NoError(t, err, "an undeployed Model is not checked against its endpoints")

	delete(spec, "deploymentState")
	stepPastLastWrite()
	_, err = f.svc.UpdateModel("gpt", f.params(body))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec.endpoints[1]: endpoint 'claude-native'")
	stored, err := f.svc.GetModelByHandle("gpt")
	require.NoError(t, err)
	assert.Equal(t, models.StateUndeployed, stored.DesiredState, "the refused deploy leaves the draft as it was")
}

// A transformer entry that uses {apiVersion} needs the provider to set it.
// Setting it on the provider is what makes the Model servable there.
func TestModelService_RefusedWhenAProviderLeavesAPlaceholderEmpty(t *testing.T) {
	f := newModelServiceFixture(t, true)
	f.createDirectAnthropicProvider("claude-prod")
	_, err := f.svc.CreateModelEndpoint(f.params(endpointBody("versioned", "/versioned", func(spec map[string]any) {
		spec["transformers"] = []any{map[string]any{"template": "anthropic", "type": "openai-to-anthropic-transformer",
			"version": "v0", "params": map[string]any{"anthropicVersion": "{apiVersion}"}}}
	})))
	require.NoError(t, err)

	body := modelBody("claude", "claude", modelTarget("a", "claude-prod", "claude-haiku"))
	body["spec"].(map[string]any)["endpoints"] = []any{"versioned"}
	_, err = f.svc.CreateModel(f.params(body))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrModelValidation))
	assert.Contains(t, err.Error(), "spec.endpoints[0]: endpoint 'versioned', provider 'claude-prod'")
	assert.Contains(t, err.Error(), "must set spec.upstream.apiVersion")

	stepPastLastWrite()
	_, err = f.svc.UpdateLLMProvider("claude-prod", f.params(map[string]any{
		"apiVersion": "gateway.api-platform.wso2.com/v1",
		"kind":       "LlmProvider",
		"metadata":   map[string]any{"name": "claude-prod"},
		"spec": map[string]any{
			"displayName": "claude-prod", "version": "v1.0", "template": "anthropic", "exposure": "none",
			"upstream": map[string]any{"url": "https://api.anthropic.com", "apiVersion": "2023-06-01",
				"auth": map[string]any{"type": "api-key", "header": "x-api-key", "value": "sk-ant"}},
		},
	}))
	require.NoError(t, err)
	_, err = f.svc.CreateModel(f.params(body))
	require.NoError(t, err)
}

// A routed target is refused where its provider's accessControl does not allow
// the path the endpoint would call it on.
func TestModelService_RefusedWhereARoutedProviderRefusesThePath(t *testing.T) {
	f := newModelServiceFixture(t, true)
	f.provisionDefaultEndpoint()
	_, err := f.svc.CreateLLMProvider(f.params(map[string]any{
		"apiVersion": "gateway.api-platform.wso2.com/v1",
		"kind":       "LlmProvider",
		"metadata":   map[string]any{"name": "locked"},
		"spec": map[string]any{
			"displayName": "locked", "version": "v1.0", "template": "openai", "context": "/locked",
			"upstream":      map[string]any{"url": "https://api.openai.com/v1"},
			"accessControl": map[string]any{"mode": "deny_all"},
			"globalPolicies": []any{map[string]any{"name": "api-key-auth", "version": "v1",
				"params": map[string]any{"key": "x-api-key", "in": "header"}}},
		},
	}))
	require.NoError(t, err)

	target := modelTarget("a", "locked", "gpt-4o")
	target["auth"] = map[string]any{"type": "api-key", "header": "x-api-key", "value": "issued-key"}
	_, err = f.svc.CreateModel(f.params(modelBody("gpt", "gpt-4o", target)))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrModelValidation))
	assert.Contains(t, err.Error(), "spec.endpoints[0]: provider 'locked' does not allow POST /chat/completions")
	assert.Contains(t, err.Error(), "on endpoint 'wso2'")
}
