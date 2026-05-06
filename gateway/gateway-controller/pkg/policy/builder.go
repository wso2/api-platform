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

package policy

import (
	"log/slog"
	"strings"
	"time"

	versionutil "github.com/wso2/api-platform/common/version"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	policyv1alpha "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

// DerivePolicyFromAPIConfig derives a policy configuration from an API stored config.
// Handles both RestApi and WebSubApi kinds. This is a shared utility used by:
// - APIDeploymentService (WebSocket event path)
// - APIServer handlers (REST API path) - TODO: Refactor this to use the implementation
// - main.go startup (loading existing configs)
//
// Policy execution order: System Policies -> API Level Policies -> Operation Level Policies
// Each level does not override the previous one; policies are executed in the given order.
func DerivePolicyFromAPIConfig(cfg *models.StoredConfig, routerConfig *config.RouterConfig, systemConfig *config.Config, policyDefinitions map[string]models.PolicyDefinition) *models.StoredPolicyConfig {
	// Pre-compute latest version index once for all ResolvePolicyVersion calls in this function.
	latestVersions := config.BuildLatestVersionIndex(policyDefinitions)

	// Collect API-level policies (validate policy version exists, pass major-only to engine)
	apiPolicies := make(map[string]policyenginev1.PolicyInstance)
	if cfg.GetPolicies() != nil {
		for _, p := range *cfg.GetPolicies() {
			resolved, err := config.ResolvePolicyVersion(policyDefinitions, latestVersions, p.Name, p.Version)
			if err != nil {
				slog.Error("Failed to resolve policy version for API-level policy", "policy_name", p.Name, "error", err)
				continue
			}
			apiPolicies[p.Name] = ConvertAPIPolicyToModel(p, policyv1alpha.LevelAPI, versionutil.MajorVersion(resolved))
		}
	}

	routes := make([]policyenginev1.PolicyChain, 0)

	switch cfgTyped := cfg.Configuration.(type) {
	case api.WebSubAPI:
		apiData := cfgTyped.Spec
		for _, ch := range apiData.Hub.Channels {
			var finalPolicies []policyenginev1.PolicyInstance

			// Policy execution order: Hub Level Policies -> Channel Level Policies
			// Start with hub-level policies
			if apiData.Hub.Policies != nil {
				finalPolicies = make([]policyenginev1.PolicyInstance, 0, len(*apiData.Hub.Policies))
				for _, p := range *apiData.Hub.Policies {
					// Only append if the policy was successfully resolved (exists in apiPolicies map)
					if v, ok := apiPolicies[p.Name]; ok {
						finalPolicies = append(finalPolicies, v)
					}
				}
			}

			// Append operation-level policies (they don't override, just execute after API-level)
			if ch.Policies != nil && len(*ch.Policies) > 0 {
				for _, opPolicy := range *ch.Policies {
					resolved, err := config.ResolvePolicyVersion(policyDefinitions, latestVersions, opPolicy.Name, opPolicy.Version)
					if err != nil {
						slog.Error("Failed to resolve policy version for operation-level policy", "policy_name", opPolicy.Name, "channel_name", ch.Name, "error", err)
						continue
					}
					finalPolicies = append(finalPolicies, ConvertAPIPolicyToModel(opPolicy, policyv1alpha.LevelRoute, versionutil.MajorVersion(resolved)))
				}
			}

			routeKey := xds.GenerateRouteName("SUB", apiData.Context, apiData.Version, ch.Name, routerConfig.GatewayHost)
			props := make(map[string]any)
			injectedPolicies := utils.InjectSystemPolicies(finalPolicies, systemConfig, props)

			routes = append(routes, policyenginev1.PolicyChain{
				RouteKey: routeKey,
				Policies: injectedPolicies,
			})
		}

	case api.RestAPI:
		apiData := cfgTyped.Spec
		for _, op := range apiData.Operations {
			var finalPolicies []policyenginev1.PolicyInstance

			// Policy execution order: API Level Policies -> Operation Level Policies
			// Start with API-level policies
			if apiData.Policies != nil {
				finalPolicies = make([]policyenginev1.PolicyInstance, 0, len(*apiData.Policies))
				for _, p := range *apiData.Policies {
					// Only append if the policy was successfully resolved (exists in apiPolicies map)
					if v, ok := apiPolicies[p.Name]; ok {
						finalPolicies = append(finalPolicies, v)
					}
				}
			}

			// Append operation-level policies (they don't override, just execute after API-level)
			if op.Policies != nil && len(*op.Policies) > 0 {
				for _, opPolicy := range *op.Policies {
					resolved, err := config.ResolvePolicyVersion(policyDefinitions, latestVersions, opPolicy.Name, opPolicy.Version)
					if err != nil {
						slog.Error("Failed to resolve policy version for operation-level policy", "policy_name", opPolicy.Name, "operation_method", op.Method, "operation_path", op.Path, "error", err)
						continue
					}
					finalPolicies = append(finalPolicies, ConvertAPIPolicyToModel(opPolicy, policyv1alpha.LevelRoute, versionutil.MajorVersion(resolved)))
				}
			}

			// Determine effective vhosts
			effectiveMainVHost := routerConfig.VHosts.Main.Default
			effectiveSandboxVHost := routerConfig.VHosts.Sandbox.Default
			if apiData.Vhosts != nil {
				if strings.TrimSpace(apiData.Vhosts.Main) != "" {
					effectiveMainVHost = apiData.Vhosts.Main
				}
				if apiData.Vhosts.Sandbox != nil && strings.TrimSpace(*apiData.Vhosts.Sandbox) != "" {
					effectiveSandboxVHost = *apiData.Vhosts.Sandbox
				}
			}

			vhosts := []string{effectiveMainVHost}
			if apiData.Upstream.Sandbox != nil {
				vhosts = append(vhosts, effectiveSandboxVHost)
			}

			// Populate props for system policies (currently no-op but maintains structure for future use)
			props := make(map[string]any)
			// populatePropsForSystemPolicies(cfg.SourceConfiguration, props)

			for _, vhost := range vhosts {
				injectedPolicies := utils.InjectSystemPolicies(finalPolicies, systemConfig, props)

				routes = append(routes, policyenginev1.PolicyChain{
					RouteKey: xds.GenerateRouteName(string(op.Method), apiData.Context, apiData.Version, op.Path, vhost),
					Policies: injectedPolicies,
				})
			}
		}
	}

	// If there are no policies at all, return nil
	policyCount := 0
	for _, r := range routes {
		policyCount += len(r.Policies)
	}
	if policyCount == 0 {
		return nil
	}

	displayName := cfg.DisplayName
	apiVersion := cfg.Version
	apiContext, err := cfg.GetContext()
	if err != nil {
		slog.Error("Failed to get context", "error", err)
		return nil
	}

	now := time.Now().Unix()
	return &models.StoredPolicyConfig{
		ID: cfg.UUID + "-policies",
		Configuration: policyenginev1.Configuration{
			Routes: routes,
			Metadata: policyenginev1.Metadata{
				CreatedAt:       now,
				UpdatedAt:       now,
				ResourceVersion: 0,
				APIName:         displayName,
				Version:         apiVersion,
				Context:         apiContext,
			},
		},
		Version: 0,
	}
}

// ConvertAPIPolicyToModel converts generated api.Policy to policyenginev1.PolicyInstance
func ConvertAPIPolicyToModel(p api.Policy, attachedTo policyv1alpha.Level, resolvedVersion string) policyenginev1.PolicyInstance {
	paramsMap := make(map[string]interface{})
	if p.Params != nil {
		for k, v := range *p.Params {
			paramsMap[k] = v
		}
	}

	if attachedTo != "" {
		paramsMap["attachedTo"] = string(attachedTo)
	}

	return policyenginev1.PolicyInstance{
		Name:               p.Name,
		Version:            resolvedVersion,
		Enabled:            true,
		ExecutionCondition: p.ExecutionCondition,
		Parameters:         paramsMap,
	}
}
