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

package controller

const (
	// AnnAPIGatewayDeleteImmediately, set to "true" on an APIGateway, uninstalls its release
	// as soon as the APIGateway is deleted, skipping the operator's deletion grace period.
	AnnAPIGatewayDeleteImmediately = "gateway.api-platform.wso2.com/delete-immediately"

	// AnnK8sGatewayHelmValuesConfigMap references a ConfigMap in the Gateway namespace with key values.yaml.
	AnnK8sGatewayHelmValuesConfigMap = "gateway.api-platform.wso2.com/helm-values-configmap"

	// AnnK8sGatewayAPISelector is optional JSON for api v1alpha1.APISelector (REST API selection from RestApi CRs).
	AnnK8sGatewayAPISelector = "gateway.api-platform.wso2.com/api-selector"

	// AnnK8sGatewayControlPlaneHost sets registry.GatewayInfo.ControlPlaneHost when provided.
	AnnK8sGatewayControlPlaneHost = "gateway.api-platform.wso2.com/control-plane-host"

	// AnnK8sGatewayHelmValuesHash is set by the operator after a successful Helm install/upgrade
	// so repeated reconciles (status patches, periodic sync) do not run upgrade needlessly.
	AnnK8sGatewayHelmValuesHash = "gateway.api-platform.wso2.com/last-helm-values-hash"

	// AnnK8sGatewayServiceType, when present on Gateway.spec.infrastructure.annotations,
	// overrides gateway.gatewayRuntime.service.type in the Helm values. Accepted values:
	// ClusterIP, NodePort, LoadBalancer, ExternalName.
	AnnK8sGatewayServiceType = "gateway.api-platform.wso2.com/service-type"

	// AnnHTTPRouteAPIVersion overrides API version in generated api.yaml (default v1).
	AnnHTTPRouteAPIVersion = "gateway.api-platform.wso2.com/api-version"

	// AnnHTTPRouteContext sets APIConfigData.Context (must match gateway validation pattern).
	AnnHTTPRouteContext = "gateway.api-platform.wso2.com/context"

	// AnnHTTPRouteDisplayName overrides displayName.
	AnnHTTPRouteDisplayName = "gateway.api-platform.wso2.com/display-name"

	// AnnProjectID identifies the owning project for analytics/routing metadata propagation.
	AnnProjectID = "gateway.api-platform.wso2.com/project-id"

	// HTTPRoute rest API handle for gateway-controller (default: derived from ns+name).
	AnnHTTPRouteAPIHandle = "gateway.api-platform.wso2.com/api-handle"

	// AnnHTTPRouteDeployedGateways records every Gateway the route is deployed to, as JSON
	// {"namespace/name": {"hash": "...", "handle": "..."}}. The hash lets the reconciler skip
	// redeploying unchanged config (which would re-snapshot and churn xDS); the handle lets it
	// remove the old API when the api-handle annotation changes.
	AnnHTTPRouteDeployedGateways = "gateway.api-platform.wso2.com/deployed-gateways"

	// AnnHTTPRouteLastDeployedParentGateway is the legacy single-Gateway record ("namespace/name").
	// It seeds AnnHTTPRouteDeployedGateways on upgrade and is still written for routes deployed to
	// exactly one Gateway, so a downgraded operator can find the deployment.
	AnnHTTPRouteLastDeployedParentGateway = "gateway.api-platform.wso2.com/last-deployed-parent-gateway"

	// AnnHTTPRouteLastDeployedConfigHash is the legacy payload hash paired with
	// AnnHTTPRouteLastDeployedParentGateway, kept on the same terms.
	AnnHTTPRouteLastDeployedConfigHash = "gateway.api-platform.wso2.com/last-deployed-config-hash"
)
