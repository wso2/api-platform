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

// Upstream credential inheritance on update.
//
// An upstream `auth.value` is write-only: accepted on create/update, never
// returned on a read (see pkg/api/handlers/credential_redaction.go). An update
// that does not carry a credential therefore inherits the persisted one:
//
//	value supplied              -> use it
//	auth present, value omitted -> inherit the stored value
//	auth omitted entirely       -> inherit the stored auth block
//	type: none                  -> no inheritance; auth is removed
//
// Inheritance reads the stored SourceConfiguration, which is the unrendered
// artifact, so a credential held as a `secret` expression is carried forward
// unresolved and re-rendered downstream like any other value.
//
// This applies to control-plane deploys as well as management API updates, so a
// declarative apply that intends to drop upstream auth must say `type: none`
// rather than omit the block.
package utils

import (
	"encoding/json"
	"fmt"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// storedSourceForUpdate returns the persisted SourceConfiguration for the
// artifact being updated, or nil when there is nothing to inherit from: a create
// (no ID), or an ID with no stored artifact.
//
// A lookup failure is returned as an error rather than treated as "nothing to
// inherit". Both validators only descend into an auth block that is present, so
// silently skipping inheritance here would let an update that omits the block
// deploy with no upstream auth attached.
func storedSourceForUpdate(db storage.Storage, id string) (any, error) {
	if db == nil || id == "" {
		return nil, nil
	}
	existing, err := db.GetConfig(id)
	if storage.IsNotFoundError(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("look up stored configuration %q for credential inheritance: %w", id, err)
	}
	if existing == nil {
		return nil, nil
	}
	return existing.SourceConfiguration, nil
}

// inheritStoredProviderCredential inherits the persisted upstream credential for
// an LLM provider update.
func (s *LLMDeploymentService) inheritStoredProviderCredential(incoming *api.LLMProviderConfiguration, params LLMDeploymentParams) error {
	source, err := storedSourceForUpdate(s.db, params.ID)
	if err != nil {
		return err
	}
	inheritLLMProviderCredential(incoming, source)
	return nil
}

// inheritStoredProxyCredentials inherits persisted upstream credentials for an
// LLM proxy update.
func (s *LLMDeploymentService) inheritStoredProxyCredentials(incoming *api.LLMProxyConfiguration, params LLMDeploymentParams) error {
	source, err := storedSourceForUpdate(s.db, params.ID)
	if err != nil {
		return err
	}
	inheritLLMProxyCredentials(incoming, source)
	return nil
}

// inheritStoredMCPCredential inherits the persisted upstream credential for an
// MCP proxy update.
func (s *MCPDeploymentService) inheritStoredMCPCredential(incoming *api.MCPProxyConfiguration, params MCPDeploymentParams) error {
	source, err := storedSourceForUpdate(s.db, params.ID)
	if err != nil {
		return err
	}
	inheritMCPProxyCredential(incoming, source)
	return nil
}

// authTypeNone is the explicit "remove upstream authentication" signal, spelled
// the same way in every auth enum in the management spec.
const authTypeNone = "none"

// reinterpret re-encodes a stored SourceConfiguration into a typed value. The
// stored artifact is a typed struct in memory and a plain map once it has
// round-tripped through the database, so both shapes are handled via JSON. The
// result is a fresh copy and never aliases the stored configuration.
func reinterpret[T any](source any) (T, bool) {
	var out T
	if source == nil {
		return out, false
	}
	raw, err := json.Marshal(source)
	if err != nil {
		return out, false
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, false
	}
	return out, true
}

// hasCredential reports whether a credential field carries a value. A
// present-but-empty string counts as absent.
func hasCredential(value *string) bool {
	return value != nil && *value != ""
}

// inheritLLMProviderCredential carries a persisted upstream credential forward
// when an LLM provider update omits it.
func inheritLLMProviderCredential(incoming *api.LLMProviderConfiguration, storedSource any) {
	if incoming == nil {
		return
	}
	stored, ok := reinterpret[api.LLMProviderConfiguration](storedSource)
	if !ok || stored.Spec.Upstream.Auth == nil || !hasCredential(stored.Spec.Upstream.Auth.Value) {
		return
	}

	if incoming.Spec.Upstream.Auth == nil {
		// Auth omitted entirely — inherit the stored block wholesale. `stored`
		// is a fresh copy from reinterpret, so this does not alias storage.
		incoming.Spec.Upstream.Auth = stored.Spec.Upstream.Auth
		return
	}
	if string(incoming.Spec.Upstream.Auth.Type) == authTypeNone {
		return // Explicit removal.
	}
	if !hasCredential(incoming.Spec.Upstream.Auth.Value) {
		incoming.Spec.Upstream.Auth.Value = stored.Spec.Upstream.Auth.Value
	}
}

// inheritLLMProxyCredentials carries persisted upstream credentials forward when
// an LLM proxy update omits them, for the primary provider and for each
// additionalProviders[] entry.
//
// An auth block belongs to the specific provider id it was stored against, so
// inheritance is keyed on that id in both cases — never on list position, and
// never unconditionally. An update that repoints a provider inherits nothing for
// it and must supply its own credential.
func inheritLLMProxyCredentials(incoming *api.LLMProxyConfiguration, storedSource any) {
	if incoming == nil {
		return
	}
	stored, ok := reinterpret[api.LLMProxyConfiguration](storedSource)
	if !ok {
		return
	}

	if incoming.Spec.Provider.Id == stored.Spec.Provider.Id {
		inheritLLMUpstreamAuth(&incoming.Spec.Provider.Auth, stored.Spec.Provider.Auth)
	}

	if incoming.Spec.AdditionalProviders == nil || stored.Spec.AdditionalProviders == nil {
		return
	}
	storedByID := make(map[string]*api.LLMUpstreamAuth, len(*stored.Spec.AdditionalProviders))
	for _, sp := range *stored.Spec.AdditionalProviders {
		storedByID[sp.Id] = sp.Auth
	}
	incomingAdditional := *incoming.Spec.AdditionalProviders
	for i := range incomingAdditional {
		if storedAuth, found := storedByID[incomingAdditional[i].Id]; found {
			inheritLLMUpstreamAuth(&incomingAdditional[i].Auth, storedAuth)
		}
	}
}

// inheritLLMUpstreamAuth applies the inheritance rules to a single
// *api.LLMUpstreamAuth field, in place.
func inheritLLMUpstreamAuth(incoming **api.LLMUpstreamAuth, stored *api.LLMUpstreamAuth) {
	if incoming == nil || stored == nil || !hasCredential(stored.Value) {
		return
	}
	if *incoming == nil {
		*incoming = stored
		return
	}
	if string((*incoming).Type) == authTypeNone {
		return // Explicit removal.
	}
	if !hasCredential((*incoming).Value) {
		(*incoming).Value = stored.Value
	}
}

// inheritMCPProxyCredential carries a persisted upstream credential forward when
// an MCP proxy update omits it.
func inheritMCPProxyCredential(incoming *api.MCPProxyConfiguration, storedSource any) {
	if incoming == nil {
		return
	}
	stored, ok := reinterpret[api.MCPProxyConfiguration](storedSource)
	if !ok || stored.Spec.Upstream.Auth == nil || !hasCredential(stored.Spec.Upstream.Auth.Value) {
		return
	}

	if incoming.Spec.Upstream.Auth == nil {
		incoming.Spec.Upstream.Auth = stored.Spec.Upstream.Auth
		return
	}
	if string(incoming.Spec.Upstream.Auth.Type) == authTypeNone {
		return // Explicit removal.
	}
	if !hasCredential(incoming.Spec.Upstream.Auth.Value) {
		incoming.Spec.Upstream.Auth.Value = stored.Spec.Upstream.Auth.Value
	}
}
