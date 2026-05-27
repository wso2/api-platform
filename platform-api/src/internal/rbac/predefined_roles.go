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

// RoleAdmin, RoleDeveloper, RoleViewer are the canonical platform role names.
// IDPs and Thunder groups are mapped to these names via configuration.
const (
	RoleAdmin     = "admin"
	RoleDeveloper = "developer"
	RoleViewer    = "viewer"
)

// adminPermissions is the full set of platform permissions.
var adminPermissions = []Permission{
	GatewayCreate, GatewayRead, GatewayUpdate, GatewayDelete, GatewayTokenManage, GatewayPolicyManage,
	APICreate, APIRead, APIUpdate, APIDelete, APIDeploy, APIPublish, APIImport,
	ProjectCreate, ProjectRead, ProjectUpdate, ProjectDelete,
	ApplicationCreate, ApplicationRead, ApplicationUpdate, ApplicationDelete, ApplicationKeyManage,
	DevPortalCreate, DevPortalRead, DevPortalUpdate, DevPortalDelete, DevPortalManage,
	SubscriptionCreate, SubscriptionRead, SubscriptionUpdate, SubscriptionDelete,
	SubscriptionPlanCreate, SubscriptionPlanRead, SubscriptionPlanUpdate, SubscriptionPlanDelete,
	APIKeyCreate, APIKeyRead, APIKeyUpdate, APIKeyDelete,
	LLMTemplateCreate, LLMTemplateRead, LLMTemplateUpdate, LLMTemplateDelete,
	LLMProviderCreate, LLMProviderRead, LLMProviderUpdate, LLMProviderDelete, LLMProviderDeploy, LLMProviderKeyManage,
	LLMProxyCreate, LLMProxyRead, LLMProxyUpdate, LLMProxyDelete, LLMProxyDeploy, LLMProxyKeyManage,
	MCPProxyCreate, MCPProxyRead, MCPProxyUpdate, MCPProxyDelete, MCPProxyDeploy,
	WebSubAPICreate, WebSubAPIRead, WebSubAPIUpdate, WebSubAPIDelete, WebSubAPIDeploy, WebSubAPIPublish, WebSubAPIKeyManage,
	WebBrokerAPICreate, WebBrokerAPIRead, WebBrokerAPIUpdate, WebBrokerAPIDelete, WebBrokerAPIDeploy, WebBrokerAPIPublish, WebBrokerAPIKeyManage,
	GitRead,
}

// developerPermissions covers all resource CRUD and deployment operations but
// excludes gateway management, devportal administration, and subscription plan administration
// which are reserved for admins.
var developerPermissions = []Permission{
	GatewayRead,
	APICreate, APIRead, APIUpdate, APIDelete, APIDeploy, APIPublish, APIImport,
	ProjectCreate, ProjectRead, ProjectUpdate, ProjectDelete,
	ApplicationCreate, ApplicationRead, ApplicationUpdate, ApplicationDelete, ApplicationKeyManage,
	DevPortalRead,
	SubscriptionCreate, SubscriptionRead, SubscriptionUpdate, SubscriptionDelete,
	SubscriptionPlanRead,
	APIKeyCreate, APIKeyRead, APIKeyUpdate, APIKeyDelete,
	LLMTemplateCreate, LLMTemplateRead, LLMTemplateUpdate, LLMTemplateDelete,
	LLMProviderCreate, LLMProviderRead, LLMProviderUpdate, LLMProviderDelete, LLMProviderDeploy, LLMProviderKeyManage,
	LLMProxyCreate, LLMProxyRead, LLMProxyUpdate, LLMProxyDelete, LLMProxyDeploy, LLMProxyKeyManage,
	MCPProxyCreate, MCPProxyRead, MCPProxyUpdate, MCPProxyDelete, MCPProxyDeploy,
	WebSubAPICreate, WebSubAPIRead, WebSubAPIUpdate, WebSubAPIDelete, WebSubAPIDeploy, WebSubAPIPublish, WebSubAPIKeyManage,
	WebBrokerAPICreate, WebBrokerAPIRead, WebBrokerAPIUpdate, WebBrokerAPIDelete, WebBrokerAPIDeploy, WebBrokerAPIPublish, WebBrokerAPIKeyManage,
	GitRead,
}

// viewerPermissions covers read-only access to all resources.
var viewerPermissions = []Permission{
	GatewayRead,
	APIRead,
	ProjectRead,
	ApplicationRead,
	DevPortalRead,
	SubscriptionRead,
	SubscriptionPlanRead,
	APIKeyRead,
	LLMTemplateRead,
	LLMProviderRead,
	LLMProxyRead,
	MCPProxyRead,
	WebSubAPIRead,
	WebBrokerAPIRead,
}

// RolePermissions maps a platform role name to its granted permissions.
// Used by both the Thunder resolver and the claims-based resolver.
var RolePermissions = map[string][]Permission{
	RoleAdmin:     adminPermissions,
	RoleDeveloper: developerPermissions,
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
