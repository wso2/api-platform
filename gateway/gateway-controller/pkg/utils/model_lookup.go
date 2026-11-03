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
	"fmt"
	"sync"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/modelgateway"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
)

// ModelLookup implements modelgateway.Lookup over the gateway's artifacts.
//
// Two sources back it. A save reads the database, which is the source of truth the
// moment a write returns; the compiler reads the in-memory store, which is what the
// snapshot being built must agree with. Everything else — rendering a provider's
// credential, resolving a policy definition — is the same for both, which is why
// this is one type with two constructors rather than two lookups.
//
// A lookup is made for one save or one compile, and renders each provider once
// however many targets name it: rendering reads the provider's secrets, and a
// compile runs for every endpoint on every Envoy snapshot. Nothing is kept beyond
// the lookup, so a provider or secret change is read by the next save or compile.
type ModelLookup struct {
	providerByHandle  func(handle string) (*models.StoredConfig, error)
	templateByHandle  func(handle string) (*models.StoredLLMProviderTemplate, error)
	policyDefinitions map[string]models.PolicyDefinition
	secretResolver    funcs.SecretResolver

	mu        sync.Mutex
	providers map[string]providerLookup
}

// providerLookup is one provider as this lookup first read it, or why it could not.
type providerLookup struct {
	provider *modelgateway.Provider
	err      error
}

// NewDBModelLookup returns a lookup that reads providers and templates from the
// database.
func NewDBModelLookup(db storage.Storage, policyDefinitions map[string]models.PolicyDefinition,
	secretResolver funcs.SecretResolver) *ModelLookup {
	return &ModelLookup{
		providerByHandle: func(handle string) (*models.StoredConfig, error) {
			return db.GetConfigByKindAndHandle(models.KindLlmProvider, handle)
		},
		templateByHandle:  db.GetLLMProviderTemplateByHandle,
		policyDefinitions: policyDefinitions,
		secretResolver:    secretResolver,
	}
}

// NewStoreModelLookup returns a lookup that reads providers and templates from the
// in-memory store.
func NewStoreModelLookup(store *storage.ConfigStore, policyDefinitions map[string]models.PolicyDefinition,
	secretResolver funcs.SecretResolver) *ModelLookup {
	return &ModelLookup{
		providerByHandle: func(handle string) (*models.StoredConfig, error) {
			return store.GetByKindAndHandle(models.KindLlmProvider, handle)
		},
		templateByHandle:  store.GetTemplateByHandle,
		policyDefinitions: policyDefinitions,
		secretResolver:    secretResolver,
	}
}

// Provider implements modelgateway.Lookup. The provider's source is rendered here,
// so a direct target presents the resolved credential, and the secrets that
// rendering produced travel with it for redaction. The result, or the error, is
// reused for every later target on the same provider.
func (l *ModelLookup) Provider(handle string) (*modelgateway.Provider, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if got, ok := l.providers[handle]; ok {
		return got.provider, got.err
	}
	provider, err := l.renderProvider(handle)
	if l.providers == nil {
		l.providers = make(map[string]providerLookup)
	}
	l.providers[handle] = providerLookup{provider: provider, err: err}
	return provider, err
}

// renderProvider reads one provider and renders its template expressions.
func (l *ModelLookup) renderProvider(handle string) (*modelgateway.Provider, error) {
	cfg, err := l.providerByHandle(handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			return nil, fmt.Errorf("%w: %s", modelgateway.ErrProviderNotFound, handle)
		}
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("%w: %s", modelgateway.ErrProviderNotFound, handle)
	}
	source, ok := cfg.SourceConfiguration.(api.LLMProviderConfiguration)
	if !ok {
		return nil, fmt.Errorf("provider '%s' has unexpected source configuration %T", handle, cfg.SourceConfiguration)
	}

	holder := &models.StoredConfig{Handle: cfg.Handle, Kind: cfg.Kind, Configuration: source}
	if err := templateengine.RenderSpec(holder, l.secretResolver, nil); err != nil {
		return nil, fmt.Errorf("provider '%s' could not be rendered: %w", handle, err)
	}
	rendered, ok := holder.Configuration.(api.LLMProviderConfiguration)
	if !ok {
		return nil, fmt.Errorf("provider '%s' rendered to unexpected type %T", handle, holder.Configuration)
	}
	return &modelgateway.Provider{
		UUID:            cfg.UUID,
		DesiredState:    cfg.DesiredState,
		Config:          rendered,
		SensitiveValues: holder.SensitiveValues,
	}, nil
}

// Template implements modelgateway.Lookup.
func (l *ModelLookup) Template(handle string) (*models.StoredLLMProviderTemplate, error) {
	return l.templateByHandle(handle)
}

// PolicyDefinition implements modelgateway.Lookup.
func (l *ModelLookup) PolicyDefinition(name, majorVersion string) (*models.PolicyDefinition, error) {
	return LookupPolicyDefinition(l.policyDefinitions, name, majorVersion)
}

// LookupPolicyDefinition resolves a policy at a major version to its loaded
// definition. An empty version resolves to the latest loaded one.
func LookupPolicyDefinition(definitions map[string]models.PolicyDefinition, name, majorVersion string) (*models.PolicyDefinition, error) {
	full, err := config.ResolvePolicyVersion(definitions, nil, name, majorVersion)
	if err != nil {
		return nil, err
	}
	def, ok := definitions[name+"|"+full]
	if !ok {
		return nil, fmt.Errorf("policy '%s' version '%s' not found in loaded policy definitions", name, full)
	}
	return &def, nil
}
