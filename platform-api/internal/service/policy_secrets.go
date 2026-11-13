/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

// Write-only policy parameters, applied to the two shapes the services hold
// them in: the wire types a response is built from, and the model types an
// update is merged into.
//
// The policysecret package decides what a write-only parameter is and what to
// do with one. This file is only the traversal — finding every parameter map in
// a provider, a proxy or an API, and handing it over. The split is deliberate:
// the rules are worth testing on their own, and the traversal is worth reading
// on its own, because what it misses is what leaks.
//
// Two rules hold everywhere below:
//
//   - Blanking happens on the response, never on the stored model. A response
//     is a copy; the stored configuration behind it is what the deployment
//     artifact is built from, and it must keep its credentials in full.
//   - Preserving happens on the update, before validation. The empty string a
//     reader was shown fails a policy's own `minLength` constraint, so an
//     update that is validated before the stored value is merged back in is
//     rejected for a parameter the caller never touched.

package service

import (
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/policysecret"
)

// policySecretResolver returns the write-only resolver for an organization.
//
// A store that was never wired resolves nothing. That is for tests constructing
// a service directly; server startup always wires one, and a read that reaches
// a real store whose manifests cannot be read fails rather than answering with
// an empty set — see policysecret.Store.For.
func policySecretResolver(store *policysecret.Store, orgUUID string) (policysecret.Resolver, error) {
	if store == nil {
		return policysecret.Nop(), nil
	}
	return store.For(orgUUID)
}

// Response side: empty every declared write-only parameter in a mapped payload.

// blankPolicySecrets empties the declared write-only parameters of a policy
// list on its way out.
func blankPolicySecrets(resolver policysecret.Resolver, policies *[]api.Policy) {
	if policies == nil {
		return
	}
	list := *policies
	for i := range list {
		if list[i].Params == nil {
			continue
		}
		paths := resolver.WriteOnlyPaths(list[i].Name, list[i].Version)
		if len(paths) == 0 {
			continue
		}
		blanked := policysecret.Blank(*list[i].Params, paths)
		list[i].Params = &blanked
	}
}

// blankOperationPolicySecrets is the per-path counterpart. The policy is named
// once and its parameters are carried per path, so the paths resolved for the
// policy apply to every one of them.
func blankOperationPolicySecrets(resolver policysecret.Resolver, policies *[]api.OperationPolicy) {
	if policies == nil {
		return
	}
	list := *policies
	for i := range list {
		paths := resolver.WriteOnlyPaths(list[i].Name, list[i].Version)
		if len(paths) == 0 {
			continue
		}
		for j := range list[i].Paths {
			list[i].Paths[j].Params = policysecret.Blank(list[i].Paths[j].Params, paths)
		}
	}
}

// blankLLMPolicySecrets covers the legacy flat policy list, which a provider
// stored before global and operation policies were split apart and which is
// still read back from configurations written then.
func blankLLMPolicySecrets(resolver policysecret.Resolver, policies *[]api.LLMPolicy) {
	if policies == nil {
		return
	}
	list := *policies
	for i := range list {
		paths := resolver.WriteOnlyPaths(list[i].Name, list[i].Version)
		if len(paths) == 0 {
			continue
		}
		for j := range list[i].Paths {
			list[i].Paths[j].Params = policysecret.Blank(list[i].Paths[j].Params, paths)
		}
	}
}

// Update side: carry stored write-only parameters forward onto an update that
// did not supply its own.
//
// Every function below matches a stored policy to an updated one by name and
// version. Matching on version is what keeps a stored secret from being
// reattached across a policy upgrade: a different version is a different
// parameter schema, and the same parameter name in it may mean something else.
// This mirrors the guard preserveAPIUpstreamAuth applies to an upstream
// credential when the auth type changes.

// preservePolicySecrets carries stored write-only parameters onto an updated
// policy list.
func preservePolicySecrets(resolver policysecret.Resolver, stored, updated []model.Policy) {
	for i := range updated {
		paths := resolver.WriteOnlyPaths(updated[i].Name, updated[i].Version)
		if len(paths) == 0 {
			continue
		}
		match := findStoredPolicy(stored, updated[i].Name, updated[i].Version)
		if match == nil || match.Params == nil {
			continue
		}
		if updated[i].Params == nil {
			// The caller sent no parameters at all. The stored secrets still
			// have to survive, so they are merged into an otherwise empty map
			// rather than the stored map being adopted wholesale.
			params := map[string]interface{}{}
			updated[i].Params = &params
		}
		policysecret.Preserve(*match.Params, *updated[i].Params, paths)
	}
}

// findStoredPolicy returns the stored policy of the same name and version, or
// nil when the update names one that was not there before.
func findStoredPolicy(stored []model.Policy, name, version string) *model.Policy {
	for i := range stored {
		if stored[i].Name == name && stored[i].Version == version {
			return &stored[i]
		}
	}
	return nil
}

// preserveGlobalPolicySecrets is the provider and proxy global-policy
// counterpart.
func preserveGlobalPolicySecrets(resolver policysecret.Resolver, stored, updated []model.GlobalPolicy) {
	for i := range updated {
		paths := resolver.WriteOnlyPaths(updated[i].Name, updated[i].Version)
		if len(paths) == 0 {
			continue
		}
		for j := range stored {
			if stored[j].Name != updated[i].Name || stored[j].Version != updated[i].Version {
				continue
			}
			if updated[i].Params == nil {
				updated[i].Params = map[string]interface{}{}
			}
			policysecret.Preserve(stored[j].Params, updated[i].Params, paths)
			break
		}
	}
}

// preserveOperationPolicySecrets is the per-path counterpart.
//
// Paths are matched on the path expression itself, not on position: an update
// that adds, removes or reorders paths still has each surviving path's secrets
// carried forward, and a newly added path has none to carry.
func preserveOperationPolicySecrets(resolver policysecret.Resolver, stored, updated []model.OperationPolicy) {
	for i := range updated {
		paths := resolver.WriteOnlyPaths(updated[i].Name, updated[i].Version)
		if len(paths) == 0 {
			continue
		}
		for j := range stored {
			if stored[j].Name != updated[i].Name || stored[j].Version != updated[i].Version {
				continue
			}
			for u := range updated[i].Paths {
				storedPath := findStoredOperationPath(stored[j].Paths, updated[i].Paths[u].Path)
				if storedPath == nil {
					continue
				}
				if updated[i].Paths[u].Params == nil {
					updated[i].Paths[u].Params = map[string]interface{}{}
				}
				policysecret.Preserve(storedPath.Params, updated[i].Paths[u].Params, paths)
			}
			break
		}
	}
}

// findStoredOperationPath returns the stored entry for a path expression.
func findStoredOperationPath(stored []model.OperationPolicyPath, path string) *model.OperationPolicyPath {
	for i := range stored {
		if stored[i].Path == path {
			return &stored[i]
		}
	}
	return nil
}

// preserveLLMPolicySecrets is the legacy flat-list counterpart, kept so that a
// provider still carrying one does not lose its secrets on update.
func preserveLLMPolicySecrets(resolver policysecret.Resolver, stored, updated []model.LLMPolicy) {
	for i := range updated {
		paths := resolver.WriteOnlyPaths(updated[i].Name, updated[i].Version)
		if len(paths) == 0 {
			continue
		}
		for j := range stored {
			if stored[j].Name != updated[i].Name || stored[j].Version != updated[i].Version {
				continue
			}
			for u := range updated[i].Paths {
				storedPath := findStoredLLMPath(stored[j].Paths, updated[i].Paths[u].Path)
				if storedPath == nil {
					continue
				}
				if updated[i].Paths[u].Params == nil {
					updated[i].Paths[u].Params = map[string]interface{}{}
				}
				policysecret.Preserve(storedPath.Params, updated[i].Paths[u].Params, paths)
			}
			break
		}
	}
}

// findStoredLLMPath returns the stored entry for a path expression.
func findStoredLLMPath(stored []model.LLMPolicyPath, path string) *model.LLMPolicyPath {
	for i := range stored {
		if stored[i].Path == path {
			return &stored[i]
		}
	}
	return nil
}

// blankProviderPolicySecrets empties the declared write-only parameters in a
// mapped provider response.
//
// Every policy-bearing field of the response is covered here, the deprecated
// flat list included: a provider written before global and operation policies
// were split apart still reads back through it.
func (s *LLMProviderService) blankProviderPolicySecrets(orgUUID string, resp *api.LLMProvider) error {
	if resp == nil {
		return nil
	}
	resolver, err := policySecretResolver(s.policySecrets, orgUUID)
	if err != nil {
		return err
	}
	blankPolicySecrets(resolver, resp.GlobalPolicies)
	blankOperationPolicySecrets(resolver, resp.OperationPolicies)
	blankLLMPolicySecrets(resolver, resp.Policies)
	return nil
}

// blankProxyPolicySecrets is the proxy counterpart.
func (s *LLMProxyService) blankProxyPolicySecrets(orgUUID string, resp *api.LLMProxy) error {
	if resp == nil {
		return nil
	}
	resolver, err := policySecretResolver(s.policySecrets, orgUUID)
	if err != nil {
		return err
	}
	blankPolicySecrets(resolver, resp.GlobalPolicies)
	blankOperationPolicySecrets(resolver, resp.OperationPolicies)
	return nil
}

// storedPolicyScopes returns a stored configuration's policies in the two
// canonical lists, splitting a legacy flat list exactly as the read path does.
//
// The split has to match the read, not the storage: a provider written before
// the lists were separated is read back as global and operation policies, so
// that is the shape the caller echoes on update, and matching against the
// unsplit stored list would find nothing and drop every secret in it.
func storedPolicyScopes(global []model.GlobalPolicy, operation []model.OperationPolicy,
	legacy []model.LLMPolicy) ([]model.GlobalPolicy, []model.OperationPolicy) {
	if len(global) == 0 && len(operation) == 0 && len(legacy) > 0 {
		return splitLegacyPoliciesForRead(legacy)
	}
	return global, operation
}

// preserveProviderPolicySecrets carries a stored provider's write-only policy
// parameters onto an update that echoed them back empty.
//
// Called after migrateLegacyProviderPoliciesInPlace, so the update is already
// in canonical form and only the two canonical lists need matching.
func (s *LLMProviderService) preserveProviderPolicySecrets(orgUUID string,
	existing *model.LLMProviderConfig, updated *model.LLMProviderConfig) error {
	if existing == nil || updated == nil {
		return nil
	}
	resolver, err := policySecretResolver(s.policySecrets, orgUUID)
	if err != nil {
		return err
	}
	storedGlobal, storedOperation := storedPolicyScopes(
		existing.GlobalPolicies, existing.OperationPolicies, existing.Policies)
	preserveGlobalPolicySecrets(resolver, storedGlobal, updated.GlobalPolicies)
	preserveOperationPolicySecrets(resolver, storedOperation, updated.OperationPolicies)
	preserveLLMPolicySecrets(resolver, existing.Policies, updated.Policies)
	return nil
}

// preserveProxyPolicySecrets is the proxy counterpart.
func (s *LLMProxyService) preserveProxyPolicySecrets(orgUUID string,
	existing *model.LLMProxyConfig, updated *model.LLMProxyConfig) error {
	if existing == nil || updated == nil {
		return nil
	}
	resolver, err := policySecretResolver(s.policySecrets, orgUUID)
	if err != nil {
		return err
	}
	storedGlobal, storedOperation := storedPolicyScopes(
		existing.GlobalPolicies, existing.OperationPolicies, existing.Policies)
	preserveGlobalPolicySecrets(resolver, storedGlobal, updated.GlobalPolicies)
	preserveOperationPolicySecrets(resolver, storedOperation, updated.OperationPolicies)
	preserveLLMPolicySecrets(resolver, existing.Policies, updated.Policies)
	return nil
}

// blankAPIPolicySecrets empties the declared write-only parameters in a mapped
// REST API response.
//
// An API carries policies at three levels, and all three are covered here: the
// API-wide list, and the per-operation and per-channel lists. A credential set
// on an operation is no less a credential than one set API-wide.
func (s *APIService) blankAPIPolicySecrets(orgUUID string, resp *api.RESTAPI) error {
	if resp == nil {
		return nil
	}
	resolver, err := policySecretResolver(s.policySecrets, orgUUID)
	if err != nil {
		return err
	}
	blankPolicySecrets(resolver, resp.Policies)
	if resp.Operations != nil {
		operations := *resp.Operations
		for i := range operations {
			blankPolicySecrets(resolver, operations[i].Request.Policies)
		}
	}
	if resp.Channels != nil {
		channels := *resp.Channels
		for i := range channels {
			blankPolicySecrets(resolver, channels[i].Request.Policies)
		}
	}
	return nil
}

// preserveAPIPolicySecrets carries a stored API's write-only policy parameters
// onto an update that echoed them back empty.
//
// Operations and channels are matched on method and path (or name), the same
// identity the gateway routes on, so reordering them does not move a credential
// from one to another.
func (s *APIService) preserveAPIPolicySecrets(orgUUID string, existing, updated *model.API) error {
	if existing == nil || updated == nil {
		return nil
	}
	resolver, err := policySecretResolver(s.policySecrets, orgUUID)
	if err != nil {
		return err
	}
	preservePolicySecrets(resolver, existing.Configuration.Policies, updated.Configuration.Policies)
	for i := range updated.Configuration.Operations {
		if updated.Configuration.Operations[i].Request == nil {
			continue
		}
		storedRequest := findStoredOperationRequest(
			existing.Configuration.Operations, updated.Configuration.Operations[i].Request)
		if storedRequest == nil {
			continue
		}
		preservePolicySecrets(resolver,
			storedRequest.Policies, updated.Configuration.Operations[i].Request.Policies)
	}
	for i := range updated.Channels {
		if updated.Channels[i].Request == nil {
			continue
		}
		storedRequest := findStoredChannelRequest(existing.Channels, updated.Channels[i].Request)
		if storedRequest == nil {
			continue
		}
		preservePolicySecrets(resolver, storedRequest.Policies, updated.Channels[i].Request.Policies)
	}
	return nil
}

// findStoredOperationRequest returns the stored operation addressing the same
// method and path.
func findStoredOperationRequest(stored []model.Operation,
	updated *model.OperationRequest) *model.OperationRequest {
	for i := range stored {
		request := stored[i].Request
		if request == nil {
			continue
		}
		if request.Method == updated.Method && request.Path == updated.Path {
			return request
		}
	}
	return nil
}

// findStoredChannelRequest returns the stored channel addressing the same
// method and name.
func findStoredChannelRequest(stored []model.Channel,
	updated *model.ChannelRequest) *model.ChannelRequest {
	for i := range stored {
		request := stored[i].Request
		if request == nil {
			continue
		}
		if request.Method == updated.Method && request.Name == updated.Name {
			return request
		}
	}
	return nil
}
