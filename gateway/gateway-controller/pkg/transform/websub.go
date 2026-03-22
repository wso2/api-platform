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
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// WebSubAPITransformer transforms a StoredConfig (WebSubApi kind) into a RuntimeDeployConfig.
type WebSubAPITransformer struct {
	routerConfig      *config.RouterConfig
	systemConfig      *config.Config
	policyDefinitions map[string]models.PolicyDefinition
}

// NewWebSubAPITransformer creates a new WebSubAPITransformer.
func NewWebSubAPITransformer(
	routerConfig *config.RouterConfig,
	systemConfig *config.Config,
	policyDefinitions map[string]models.PolicyDefinition,
) *WebSubAPITransformer {
	return &WebSubAPITransformer{
		routerConfig:      routerConfig,
		systemConfig:      systemConfig,
		policyDefinitions: policyDefinitions,
	}
}

// Transform converts a StoredConfig with WebSubAPI configuration into a RuntimeDeployConfig.
func (t *WebSubAPITransformer) Transform(cfg *models.StoredConfig) (*models.RuntimeDeployConfig, error) {
	webSubCfg, ok := cfg.Configuration.(api.WebSubAPI)
	if !ok {
		return nil, fmt.Errorf("configuration is not a WebSubAPI")
	}
	apiData := webSubCfg.Spec

	// Extract project ID from labels
	projectID := ""
	if labels := cfg.GetLabels(); labels != nil {
		if pid, exists := (*labels)["project-id"]; exists {
			projectID = pid
		}
	}

	rdc := &models.RuntimeDeployConfig{
		Metadata: models.Metadata{
			Kind:        cfg.Kind,
			Handle:      cfg.Handle,
			Name:        apiData.DisplayName,
			Version:     apiData.Version,
			DisplayName: apiData.DisplayName,
			ProjectID:   projectID,
		},
		Context:             apiData.Context,
		PolicyChainResolver: "route-key",
		Routes:              make(map[string]*models.Route),
		PolicyChains:        make(map[string]*models.PolicyChain),
		UpstreamClusters:    make(map[string]*models.UpstreamCluster),
	}

	// Use the RestAPITransformer's collectAPIPolicies via a temporary instance (same package, unexported helpers)
	restT := &RestAPITransformer{
		routerConfig:      t.routerConfig,
		systemConfig:      t.systemConfig,
		policyDefinitions: t.policyDefinitions,
	}
	apiPolicies := restT.collectAPIPolicies(apiData.Policies)

	// Determine effective vhost
	effectiveMainVHost := t.routerConfig.VHosts.Main.Default
	if apiData.Vhosts != nil {
		if strings.TrimSpace(apiData.Vhosts.Main) != "" {
			effectiveMainVHost = apiData.Vhosts.Main
		}
	}

	clusterKey := constants.WEBSUBHUB_INTERNAL_CLUSTER_NAME

	// Build routes and policy chains for each channel
	for _, ch := range apiData.Channels {
		chName := ch.Name
		if !strings.HasPrefix(chName, "/") {
			chName = "/" + chName
		}

		routeKey := xds.GenerateRouteName(string(ch.Method), apiData.Context, apiData.Version, chName, effectiveMainVHost)

		rdc.Routes[routeKey] = &models.Route{
			Method:        string(ch.Method),
			Path:          xds.ConstructFullPath(apiData.Context, apiData.Version, chName),
			OperationPath: chName,
			Vhost:         effectiveMainVHost,
			Upstream: models.RouteUpstream{
				ClusterKey: clusterKey,
			},
		}

		// Build policy chain: API-level + channel-level + system policies
		chain := restT.buildPolicyChain(apiPolicies, apiData.Policies, ch.Policies)
		injected := utils.InjectSystemPolicies(chain, t.systemConfig, nil)
		rdc.PolicyChains[routeKey] = sdkChainToModel(injected)
	}

	// Add hub route (internal POST endpoint for WebSub subscriptions)
	hubRouteKey := xds.GenerateRouteName("POST", apiData.Context, apiData.Version, constants.WEBSUB_PATH, effectiveMainVHost)
	rdc.Routes[hubRouteKey] = &models.Route{
		Method:        "POST",
		Path:          xds.ConstructFullPath(apiData.Context, apiData.Version, constants.WEBSUB_PATH),
		OperationPath: constants.WEBSUB_PATH,
		Vhost:         effectiveMainVHost,
		Upstream: models.RouteUpstream{
			ClusterKey: clusterKey,
		},
	}
	// Hub route gets only system policies
	hubChain := utils.InjectSystemPolicies(nil, t.systemConfig, nil)
	rdc.PolicyChains[hubRouteKey] = sdkChainToModel(hubChain)

	// Add upstream cluster for WebSubHub
	rdc.UpstreamClusters[clusterKey] = &models.UpstreamCluster{
		BasePath: "/",
	}

	return rdc, nil
}
