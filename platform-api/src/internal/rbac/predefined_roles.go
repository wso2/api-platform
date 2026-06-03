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

package rbac

// Canonical platform role names. IDPs and Thunder groups are mapped to these via configuration.
const (
	RoleAdmin     = "admin"
	RoleDeveloper = "developer"
	RolePublisher = "publisher"
	RoleOperator  = "operator"
	RoleViewer    = "viewer"
)

// adminPermissions is the full set of platform permissions, including the
// per-resource manage scopes that act as root scopes covering all write operations.
var adminPermissions = []Permission{
	GatewayManage, GatewayCreate, GatewayRead, GatewayUpdate, GatewayDelete, GatewayTokenManage, GatewayTokenRead, GatewayTokenCreate, GatewayTokenDelete, GatewayPolicyManage, GatewayPolicyRead, GatewayPolicyCreate, GatewayPolicyDelete, GatewayArtifactsRead, GatewayManifestRead, GatewayStatusRead,
	APIManage, APICreate, APIRead, APIUpdate, APIDelete, APIPublish, APIImport,
	APIGatewayManage, APIGatewayCreate, APIGatewayRead,
	APIDeploymentManage, APIDeploymentCreate, APIDeploymentRead, APIDeploymentDelete, APIDeploymentUndeploy, APIDeploymentRestore,
	ProjectManage, ProjectCreate, ProjectRead, ProjectUpdate, ProjectDelete,
	ApplicationManage, ApplicationCreate, ApplicationRead, ApplicationUpdate, ApplicationDelete, ApplicationAPIKeyManage, ApplicationAPIKeyCreate, ApplicationAPIKeyRead, ApplicationAPIKeyDelete, ApplicationAssociationsManage, ApplicationAssociationsCreate, ApplicationAssociationsRead, ApplicationAssociationsDelete, ApplicationAssociationsAPIKeyRead,
	DevPortalManage, DevPortalCreate, DevPortalRead, DevPortalUpdate, DevPortalDelete,
	SubscriptionManage, SubscriptionCreate, SubscriptionRead, SubscriptionUpdate, SubscriptionDelete,
	SubscriptionPlanManage, SubscriptionPlanCreate, SubscriptionPlanRead, SubscriptionPlanUpdate, SubscriptionPlanDelete,
	APIKeyManage, APIKeyCreate, APIKeyRead, APIKeyUpdate, APIKeyDelete,
	LLMTemplateManage, LLMTemplateCreate, LLMTemplateRead, LLMTemplateUpdate, LLMTemplateDelete,
	LLMProviderManage, LLMProviderCreate, LLMProviderRead, LLMProviderUpdate, LLMProviderDelete, LLMProviderDeploymentManage, LLMProviderDeploymentCreate, LLMProviderDeploymentRead, LLMProviderDeploymentDelete, LLMProviderDeploymentUndeploy, LLMProviderDeploymentRestore, LLMProviderAPIKeyManage, LLMProviderAPIKeyCreate, LLMProviderAPIKeyRead, LLMProviderAPIKeyDelete,
	LLMProxyManage, LLMProxyCreate, LLMProxyRead, LLMProxyUpdate, LLMProxyDelete, LLMProxyDeploymentManage, LLMProxyDeploymentCreate, LLMProxyDeploymentRead, LLMProxyDeploymentDelete, LLMProxyDeploymentUndeploy, LLMProxyDeploymentRestore, LLMProxyAPIKeyManage, LLMProxyAPIKeyCreate, LLMProxyAPIKeyRead, LLMProxyAPIKeyDelete,
	MCPProxyManage, MCPProxyCreate, MCPProxyRead, MCPProxyUpdate, MCPProxyDelete, MCPProxyDeploymentManage, MCPProxyDeploymentCreate, MCPProxyDeploymentRead, MCPProxyDeploymentDelete, MCPProxyDeploymentUndeploy, MCPProxyDeploymentRestore,
	WebSubAPIManage, WebSubAPICreate, WebSubAPIRead, WebSubAPIUpdate, WebSubAPIDelete, WebSubAPIDeploymentManage, WebSubAPIDeploymentCreate, WebSubAPIDeploymentRead, WebSubAPIDeploymentDelete, WebSubAPIDeploymentUndeploy, WebSubAPIDeploymentRestore, WebSubAPIPublish, WebSubAPIKeyManage, WebSubAPIKeyCreate, WebSubAPIKeyUpdate, WebSubAPIKeyDelete,
	WebBrokerAPIManage, WebBrokerAPICreate, WebBrokerAPIRead, WebBrokerAPIUpdate, WebBrokerAPIDelete, WebBrokerAPIDeploymentManage, WebBrokerAPIDeploymentCreate, WebBrokerAPIDeploymentRead, WebBrokerAPIDeploymentDelete, WebBrokerAPIDeploymentUndeploy, WebBrokerAPIDeploymentRestore, WebBrokerAPIPublish, WebBrokerAPIKeyManage, WebBrokerAPIKeyCreate, WebBrokerAPIKeyUpdate, WebBrokerAPIKeyDelete,
	GitRead,
}

// developerPermissions covers all resource CRUD and deployment operations but
// excludes gateway management, devportal administration, and subscription plan administration
// which are reserved for admins.
var developerPermissions = []Permission{
	GatewayRead,
	APICreate, APIRead, APIUpdate, APIDelete, APIPublish, APIImport,
	APIGatewayCreate, APIGatewayRead,
	APIDeploymentCreate, APIDeploymentRead, APIDeploymentDelete, APIDeploymentUndeploy, APIDeploymentRestore,
	ProjectCreate, ProjectRead, ProjectUpdate, ProjectDelete,
	ApplicationCreate, ApplicationRead, ApplicationUpdate, ApplicationDelete, ApplicationAPIKeyManage, ApplicationAPIKeyCreate, ApplicationAPIKeyRead, ApplicationAPIKeyDelete, ApplicationAssociationsManage, ApplicationAssociationsCreate, ApplicationAssociationsRead, ApplicationAssociationsDelete, ApplicationAssociationsAPIKeyRead,
	DevPortalRead,
	SubscriptionCreate, SubscriptionRead, SubscriptionUpdate, SubscriptionDelete,
	SubscriptionPlanRead,
	APIKeyCreate, APIKeyRead, APIKeyUpdate, APIKeyDelete,
	LLMTemplateCreate, LLMTemplateRead, LLMTemplateUpdate, LLMTemplateDelete,
	LLMProviderCreate, LLMProviderRead, LLMProviderUpdate, LLMProviderDelete, LLMProviderDeploymentManage, LLMProviderDeploymentCreate, LLMProviderDeploymentRead, LLMProviderDeploymentDelete, LLMProviderDeploymentUndeploy, LLMProviderDeploymentRestore, LLMProviderAPIKeyManage, LLMProviderAPIKeyCreate, LLMProviderAPIKeyRead, LLMProviderAPIKeyDelete,
	LLMProxyCreate, LLMProxyRead, LLMProxyUpdate, LLMProxyDelete, LLMProxyDeploymentManage, LLMProxyDeploymentCreate, LLMProxyDeploymentRead, LLMProxyDeploymentDelete, LLMProxyDeploymentUndeploy, LLMProxyDeploymentRestore, LLMProxyAPIKeyManage, LLMProxyAPIKeyCreate, LLMProxyAPIKeyRead, LLMProxyAPIKeyDelete,
	MCPProxyCreate, MCPProxyRead, MCPProxyUpdate, MCPProxyDelete, MCPProxyDeploymentManage, MCPProxyDeploymentCreate, MCPProxyDeploymentRead, MCPProxyDeploymentDelete, MCPProxyDeploymentUndeploy, MCPProxyDeploymentRestore,
	WebSubAPICreate, WebSubAPIRead, WebSubAPIUpdate, WebSubAPIDelete, WebSubAPIDeploymentManage, WebSubAPIDeploymentCreate, WebSubAPIDeploymentRead, WebSubAPIDeploymentDelete, WebSubAPIDeploymentUndeploy, WebSubAPIDeploymentRestore, WebSubAPIPublish, WebSubAPIKeyManage, WebSubAPIKeyCreate, WebSubAPIKeyUpdate, WebSubAPIKeyDelete,
	WebBrokerAPICreate, WebBrokerAPIRead, WebBrokerAPIUpdate, WebBrokerAPIDelete, WebBrokerAPIDeploymentManage, WebBrokerAPIDeploymentCreate, WebBrokerAPIDeploymentRead, WebBrokerAPIDeploymentDelete, WebBrokerAPIDeploymentUndeploy, WebBrokerAPIDeploymentRestore, WebBrokerAPIPublish, WebBrokerAPIKeyManage, WebBrokerAPIKeyCreate, WebBrokerAPIKeyUpdate, WebBrokerAPIKeyDelete,
	GitRead,
}

// publisherPermissions covers reading APIs across all types and publishing them to DevPortals.
// It cannot create or edit API definitions, deploy to gateways, or manage credentials.
var publisherPermissions = []Permission{
	GatewayRead,
	APIRead, APIPublish,
	APIDeploymentRead,
	ProjectRead,
	DevPortalRead,
	SubscriptionRead,
	SubscriptionPlanRead,
	LLMProviderRead,
	LLMProxyRead,
	MCPProxyRead,
	WebSubAPIRead, WebSubAPIPublish,
	WebBrokerAPIRead, WebBrokerAPIPublish,
}

// operatorPermissions covers runtime lifecycle operations across all deployable resource types:
// gateway associations, deployments, undeploys, and restores. It cannot create or edit API
// definitions, publish to DevPortals, or manage credentials.
var operatorPermissions = []Permission{
	GatewayRead,
	APIRead,
	APIGatewayRead, APIGatewayCreate,
	APIDeploymentCreate, APIDeploymentRead, APIDeploymentDelete, APIDeploymentUndeploy, APIDeploymentRestore,
	ProjectRead,
	LLMProviderRead, LLMProviderDeploymentManage, LLMProviderDeploymentCreate, LLMProviderDeploymentRead, LLMProviderDeploymentDelete, LLMProviderDeploymentUndeploy, LLMProviderDeploymentRestore,
	LLMProxyRead, LLMProxyDeploymentManage, LLMProxyDeploymentCreate, LLMProxyDeploymentRead, LLMProxyDeploymentDelete, LLMProxyDeploymentUndeploy, LLMProxyDeploymentRestore,
	MCPProxyRead, MCPProxyDeploymentManage, MCPProxyDeploymentCreate, MCPProxyDeploymentRead, MCPProxyDeploymentDelete, MCPProxyDeploymentUndeploy, MCPProxyDeploymentRestore,
	WebSubAPIRead, WebSubAPIDeploymentManage, WebSubAPIDeploymentCreate, WebSubAPIDeploymentRead, WebSubAPIDeploymentDelete, WebSubAPIDeploymentUndeploy, WebSubAPIDeploymentRestore,
	WebBrokerAPIRead, WebBrokerAPIDeploymentManage, WebBrokerAPIDeploymentCreate, WebBrokerAPIDeploymentRead, WebBrokerAPIDeploymentDelete, WebBrokerAPIDeploymentUndeploy, WebBrokerAPIDeploymentRestore,
}

// viewerPermissions covers read-only access to all resources.
var viewerPermissions = []Permission{
	GatewayRead, GatewayTokenRead, GatewayPolicyRead, GatewayArtifactsRead, GatewayManifestRead, GatewayStatusRead,
	APIRead, APIGatewayRead, APIDeploymentRead,
	ProjectRead,
	ApplicationRead, ApplicationAPIKeyRead,
	DevPortalRead,
	SubscriptionRead,
	SubscriptionPlanRead,
	APIKeyRead,
	LLMTemplateRead,
	LLMProviderRead, LLMProviderDeploymentRead,
	LLMProxyRead, LLMProxyDeploymentRead,
	MCPProxyRead, MCPProxyDeploymentRead,
	WebSubAPIRead, WebSubAPIDeploymentRead,
	WebBrokerAPIRead, WebBrokerAPIDeploymentRead,
}

// RolePermissions maps a platform role name to its granted permissions.
// Used by both the Thunder resolver and the claims-based resolver.
var RolePermissions = map[string][]Permission{
	RoleAdmin:     adminPermissions,
	RoleDeveloper: developerPermissions,
	RolePublisher: publisherPermissions,
	RoleOperator:  operatorPermissions,
	RoleViewer:    viewerPermissions,
}

// PermissionsForRoles returns the deduplicated set of permissions granted to the given roles.
func PermissionsForRoles(roles []string) map[Permission]struct{} {
	result := make(map[Permission]struct{})
	for _, role := range roles {
		for _, perm := range RolePermissions[role] {
			result[perm] = struct{}{}
		}
	}
	return result
}
