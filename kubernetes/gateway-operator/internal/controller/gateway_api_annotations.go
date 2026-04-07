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
	// AnnK8sGatewayHelmValuesConfigMap references a ConfigMap in the Gateway namespace with key values.yaml.
	AnnK8sGatewayHelmValuesConfigMap = "gateway.api-platform.wso2.com/helm-values-configmap"

	// AnnK8sGatewayAPISelector is optional JSON for api v1alpha1.APISelector (REST API selection from RestApi CRs).
	AnnK8sGatewayAPISelector = "gateway.api-platform.wso2.com/api-selector"

	// AnnK8sGatewayControlPlaneHost sets registry.GatewayInfo.ControlPlaneHost when provided.
	AnnK8sGatewayControlPlaneHost = "gateway.api-platform.wso2.com/control-plane-host"

	// AnnK8sGatewayHelmValuesHash is set by the operator after a successful Helm install/upgrade
	// so repeated reconciles (status patches, periodic sync) do not run upgrade needlessly.
	AnnK8sGatewayHelmValuesHash = "gateway.api-platform.wso2.com/last-helm-values-hash"

	// AnnHTTPRouteAPIVersion overrides API version in generated api.yaml (default v1).
	AnnHTTPRouteAPIVersion = "gateway.api-platform.wso2.com/api-version"

	// AnnHTTPRouteContext sets APIConfigData.Context (must match gateway validation pattern).
	AnnHTTPRouteContext = "gateway.api-platform.wso2.com/context"

	// AnnHTTPRouteDisplayName overrides displayName.
	AnnHTTPRouteDisplayName = "gateway.api-platform.wso2.com/display-name"

	// HTTPRoute rest API handle for gateway-controller (default: derived from ns+name).
	AnnHTTPRouteAPIHandle = "gateway.api-platform.wso2.com/api-handle"

	// AnnHTTPRouteAPIPolicies is optional JSON or YAML for API-level policies ([]Policy), same logical
	// shape as RestApi spec.policies. Ignored when AnnHTTPRouteAPIPoliciesConfigMap is set.
	AnnHTTPRouteAPIPolicies = "gateway.api-platform.wso2.com/api-policies"

	// AnnHTTPRouteAPIPoliciesConfigMap names a ConfigMap in the HTTPRoute namespace whose data
	// includes policies.yaml, policies.yml, or policies.json (API-level policy list). Takes precedence over AnnHTTPRouteAPIPolicies.
	AnnHTTPRouteAPIPoliciesConfigMap = "gateway.api-platform.wso2.com/api-policies-configmap"

	// AnnHTTPRouteOperationPolicies is JSON or YAML map from operation key "METHOD:/path" to a policy list.
	// Keys are normalized (method uppercased, path with leading slash) — see HTTPRouteOperationPolicyKey.
	// Ignored when AnnHTTPRouteOperationPoliciesConfigMap is set.
	AnnHTTPRouteOperationPolicies = "gateway.api-platform.wso2.com/operation-policies"

	// AnnHTTPRouteOperationPoliciesConfigMap names a ConfigMap in the HTTPRoute namespace whose data
	// includes operation-policies.yaml, operation-policies.yml, or operation-policies.json (map: METHOD:/path → policy list).
	// Takes precedence over AnnHTTPRouteOperationPolicies.
	AnnHTTPRouteOperationPoliciesConfigMap = "gateway.api-platform.wso2.com/operation-policies-configmap"

	// AnnHTTPRouteLastDeployedParentGateway records the Gateway used for the last successful DeployRestAPI
	// as "namespace/name" so deletion can target the correct registry entry even if spec.parentRefs change.
	AnnHTTPRouteLastDeployedParentGateway = "gateway.api-platform.wso2.com/last-deployed-parent-gateway"

	// LabelAPIPolicyScope classifies APIPolicy CRs for HTTPRoute. Value LabelAPIPolicyScopeApiLevel means the
	// policy is merged into APIConfigData.policies (API-level). Policies referenced only from HTTPRoute rule
	// filters (ExtensionRef) should not carry this label.
	LabelAPIPolicyScope = "gateway.api-platform.wso2.com/policy-scope"
	// LabelAPIPolicyScopeApiLevel marks an APIPolicy as API-level for the target HTTPRoute (see LabelAPIPolicyScope).
	LabelAPIPolicyScopeApiLevel = "ApiLevel"
)
