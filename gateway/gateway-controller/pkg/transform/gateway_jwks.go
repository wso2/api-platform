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
	"path"
	"slices"
	"sort"
	"strings"

	versionutil "github.com/wso2/api-platform/common/version"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

// GatewayJWKSKind is the RuntimeDeployConfig kind of the gateway JWKS route. It
// is not a user-deployable kind: no StoredConfig ever carries it.
const GatewayJWKSKind = "GatewayJWKS"

// GatewayJWKSRuntimeConfig builds the gateway-owned JWKS route: one route,
// keyed constants.GatewayJWKSRouteKey, whose chain is the injected system
// policies followed by the JWKS system policy, which answers every request
// itself.
//
// It depends on no deployed resource, so it is built once and contributes to
// every snapshot, which is what keeps the endpoint present with nothing
// deployed. The Envoy side adds the matching route to every virtual host (see
// xds.Translator.SetGatewayJWKSRoute); this config supplies the policy-xDS side.
//
// ok is false when the JWKS system policy is not among the loaded definitions —
// a build without it — in which case no route is emitted on either side and the
// path falls through to the usual 404.
func GatewayJWKSRuntimeConfig(
	systemConfig *config.Config,
	policyDefinitions map[string]models.PolicyDefinition,
) (rdc *models.RuntimeDeployConfig, ok bool) {
	resolved, err := config.ResolvePolicyVersion(
		policyDefinitions, config.BuildLatestVersionIndex(policyDefinitions), constants.JWKS_SYSTEM_POLICY_NAME, "")
	if err != nil {
		return nil, false
	}

	chain := utils.InjectSystemPolicies([]policyenginev1.PolicyInstance{{
		Name:    constants.JWKS_SYSTEM_POLICY_NAME,
		Version: versionutil.MajorVersion(resolved),
		Enabled: true,
	}}, systemConfig, nil)

	return &models.RuntimeDeployConfig{
		Metadata: models.Metadata{
			UUID:        "gateway-jwks",
			Kind:        GatewayJWKSKind,
			Handle:      "gateway-jwks",
			DisplayName: "Gateway JWKS",
		},
		PolicyChainResolver: models.RouteKeyResolverName,
		Routes: map[string]*models.Route{
			constants.GatewayJWKSRouteKey: {
				Method:        "GET",
				Path:          constants.GatewayJWKSPath,
				OperationPath: constants.GatewayJWKSPath,
				Vhost:         "*",
				Upstream:      models.RouteUpstream{ClusterKey: constants.GatewayInternalSinkClusterName},
			},
		},
		PolicyChains: map[string]*models.PolicyChain{
			constants.GatewayJWKSRouteKey: sdkChainToModel(chain),
		},
	}, true
}

// GatewayJWKSConflicts lists the deployed routes whose path is the gateway JWKS
// path, as "handle (vhost)". Validation rejects that path for new deployments
// while the endpoint is enabled, but a resource stored before the endpoint
// existed was never checked against it — with the endpoint enabled, the gateway
// route shadows it. The controller reports these at startup so the operator can
// move the resource or set [signing] jwks_endpoint_enabled = false.
func GatewayJWKSConflicts(deployed []*models.RuntimeDeployConfig) []string {
	var conflicts []string
	for _, rdc := range deployed {
		for _, r := range rdc.Routes {
			p := r.Path
			if !strings.HasPrefix(p, "/") {
				p = "/" + p
			}
			if path.Clean(p) != constants.GatewayJWKSPath {
				continue
			}
			vhost := r.Vhost
			if vhost == "" {
				vhost = "default"
			}
			conflicts = append(conflicts, rdc.Metadata.Handle+" ("+vhost+")")
		}
	}
	sort.Strings(conflicts)
	return slices.Compact(conflicts)
}
