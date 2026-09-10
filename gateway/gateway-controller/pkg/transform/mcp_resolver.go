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
	"fmt"
	"net/http"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

// MCPResolverName is the resolver the policy engine registers for MCP proxies. It is a
// wire value: the runtime advertises the same string in its xDS node metadata, and a
// route naming a resolver the runtime does not have is skipped at ingest.
const MCPResolverName = "mcp"

// mcpResolverOperation is the operation component of every MCP chain key. Constant because
// an MCP proxy has one chain whatever tool is invoked: Policies is a single proxy-level
// list, MCPTool carries none, and Policy has no path scoping. Per-tool behaviour lives in a
// policy's own params. Per-operation chains later means changing this and nothing else.
const mcpResolverOperation = "mcp"

// MaxRequestBodyBytes is deliberately left unset: the engine applies
// DefaultMaxResolverRequestBodyBytes (64 KiB) to any body-resolved route without one.
// Pinning the same number here would exempt MCP from a future change to that default.
// Set it only to choose a value deliberately different from the engine's.

// mcpResolutionApplies reports whether this config is an MCP proxy, and so whether its
// multiplexed route carries the MCP resolver. No resolver config accompanies it: the engine
// resolver reads none.
//
// It reads SourceConfiguration, not Configuration: by the time a transformer runs an MCP
// proxy is already desugared into a RestAPI, and only the source still identifies it.
func mcpResolutionApplies(cfg *models.StoredConfig) (bool, error) {
	if cfg == nil || cfg.Kind != string(models.KindMcp) {
		return false, nil
	}

	if _, ok := cfg.SourceConfiguration.(api.MCPProxyConfiguration); !ok {
		// The kind says MCP but the original configuration is missing or of another
		// type. Refusing is deliberate: silently falling back to no resolver would
		// deploy an MCP proxy whose policies never learn which tool was invoked, which
		// looks like working traffic until someone audits what the tool policies did.
		return false, fmt.Errorf("kind %q carries source configuration of type %T, expected api.MCPProxyConfiguration",
			cfg.Kind, cfg.SourceConfiguration)
	}

	return true, nil
}

// isMCPMultiplexedRoute reports whether an operation is the one MCP route that carries
// every logical operation, and therefore the only one whose chain a resolver selects.
//
// GET, DELETE, OPTIONS and the OAuth protected-resource route each mean exactly one
// thing, so they stay route-keyed. EffectiveResolverName already supports that mix
// within a single API.
func isMCPMultiplexedRoute(method, opPath string) bool {
	return method == http.MethodPost && opPath == constants.MCP_RESOURCE_PATH
}

// resolverAwarePolicies are the policies that read the resolver's facts instead of buffering
// the request body, and so have to be told which routes carry a resolver.
//
// Add a name here as each policy migrates. A policy absent from this set is simply never told,
// which is the safe default: it keeps buffering and parsing the body for itself.
var resolverAwarePolicies = map[string]bool{
	"mcp-acl-list":  true,
	"mcp-auth":      true,
	"mcp-authz":     true,
	"mcp-ratelimit": true,

	// The analytics collector, named by the constant it is injected under so the two cannot
	// drift. It is prepended to every chain, so on a resolver route it reads the operation
	// from the facts rather than parsing the body again.
	constants.ANALYTICS_SYSTEM_POLICY_NAME: true,
}

// paramBodyResolved is the parameter these policies read in GetPolicy to decide its
// ProcessingMode. A mismatch with the name that policy reads does not fail to compile — it
// silently leaves the policy buffering.
//
// It is deliberately absent from each policy's policy-definition.yaml, so an operator who
// writes it is rejected by validation while this injection still lands: validation runs on
// the operator's document before any transformer, and the engine merges runtime params
// without filtering them against the schema.
const paramBodyResolved = "bodyResolved"

// applyResolverAwareParams tells each resolver-aware policy on this route that its body is
// already resolved.
//
// A policy cannot work this out itself: PolicyMetadata says nothing about the route's
// resolver and ProcessingMode is fixed at chain-build time, so the controller has to state
// which kind of route this is.
//
// Absence is the default that matters. An older controller injects nothing, the parameter
// defaults to false, and the policy parses the body as before — so a new policy on an old
// gateway keeps working instead of losing its auth exemptions.
//
// It overwrites rather than defaults: only the controller knows this, so an
// operator-supplied value would be a second source of truth.
func applyResolverAwareParams(chain []policyenginev1.PolicyInstance) {
	for i := range chain {
		if !resolverAwarePolicies[chain[i].Name] {
			continue
		}
		if chain[i].Parameters == nil {
			chain[i].Parameters = map[string]interface{}{}
		}
		chain[i].Parameters[paramBodyResolved] = true
	}
}
