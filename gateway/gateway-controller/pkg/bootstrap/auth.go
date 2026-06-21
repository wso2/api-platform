/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package bootstrap

import (
	"strings"

	commonmodels "github.com/wso2/api-platform/common/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

// GenerateAuthConfig produces the auth middleware configuration for the management API.
// managementAPIBasePath is the versioned prefix (e.g. "/api/management/v0.9").
// additionalRoles is merged with the base role map; extensions use this to protect
// their own routes. Pass nil when no extra routes are needed.
func GenerateAuthConfig(cfg *config.Config, managementAPIBasePath string, additionalRoles map[string][]string) commonmodels.AuthConfig {
	// prefixed builds a resource key of the form "<METHOD> <managementAPIBasePath><path>"
	// matching the actual routes registered via RegisterHandlersWithOptions(BaseURL=managementAPIBasePath).
	prefixed := func(methodAndPath string) string {
		idx := strings.Index(methodAndPath, " ")
		if idx < 0 {
			return methodAndPath
		}
		return methodAndPath[:idx+1] + managementAPIBasePath + methodAndPath[idx+1:]
	}

	relativeRoles := map[string][]string{
		"POST /rest-apis":       {"admin", "developer"},
		"GET /rest-apis":        {"admin", "developer"},
		"GET /rest-apis/:id":    {"admin", "developer"},
		"PUT /rest-apis/:id":    {"admin", "developer"},
		"DELETE /rest-apis/:id": {"admin", "developer"},

		"POST /websub-apis":       {"admin", "developer"},
		"GET /websub-apis":        {"admin", "developer"},
		"GET /websub-apis/:id":    {"admin", "developer"},
		"PUT /websub-apis/:id":    {"admin", "developer"},
		"DELETE /websub-apis/:id": {"admin", "developer"},

		"POST /webbroker-apis":       {"admin", "developer"},
		"GET /webbroker-apis":        {"admin", "developer"},
		"GET /webbroker-apis/:id":    {"admin", "developer"},
		"DELETE /webbroker-apis/:id": {"admin", "developer"},

		"GET /certificates":         {"admin", "developer"},
		"POST /certificates":        {"admin", "developer"},
		"DELETE /certificates/:id":  {"admin"},
		"POST /certificates/reload": {"admin"},

		"GET /policies": {"admin", "developer"},

		"POST /mcp-proxies":       {"admin", "developer"},
		"GET /mcp-proxies":        {"admin", "developer"},
		"GET /mcp-proxies/:id":    {"admin", "developer"},
		"PUT /mcp-proxies/:id":    {"admin", "developer"},
		"DELETE /mcp-proxies/:id": {"admin", "developer"},

		"POST /llm-provider-templates":       {"admin"},
		"GET /llm-provider-templates":        {"admin"},
		"GET /llm-provider-templates/:id":    {"admin"},
		"PUT /llm-provider-templates/:id":    {"admin"},
		"DELETE /llm-provider-templates/:id": {"admin"},

		"POST /llm-providers":       {"admin"},
		"GET /llm-providers":        {"admin", "developer"},
		"GET /llm-providers/:id":    {"admin", "developer"},
		"PUT /llm-providers/:id":    {"admin"},
		"DELETE /llm-providers/:id": {"admin"},

		"POST /llm-proxies":       {"admin", "developer"},
		"GET /llm-proxies":        {"admin", "developer"},
		"GET /llm-proxies/:id":    {"admin", "developer"},
		"PUT /llm-proxies/:id":    {"admin", "developer"},
		"DELETE /llm-proxies/:id": {"admin", "developer"},

		"POST /rest-apis/:id/api-keys":                        {"admin", "consumer"},
		"GET /rest-apis/:id/api-keys":                         {"admin", "consumer"},
		"PUT /rest-apis/:id/api-keys/:apiKeyName":             {"admin", "consumer"},
		"POST /rest-apis/:id/api-keys/:apiKeyName/regenerate": {"admin", "consumer"},
		"DELETE /rest-apis/:id/api-keys/:apiKeyName":          {"admin", "consumer"},

		"POST /llm-providers/:id/api-keys":                        {"admin", "consumer"},
		"GET /llm-providers/:id/api-keys":                         {"admin", "consumer"},
		"PUT /llm-providers/:id/api-keys/:apiKeyName":             {"admin", "consumer"},
		"POST /llm-providers/:id/api-keys/:apiKeyName/regenerate": {"admin", "consumer"},
		"DELETE /llm-providers/:id/api-keys/:apiKeyName":          {"admin", "consumer"},

		"POST /llm-proxies/:id/api-keys":                        {"admin", "consumer"},
		"GET /llm-proxies/:id/api-keys":                         {"admin", "consumer"},
		"PUT /llm-proxies/:id/api-keys/:apiKeyName":             {"admin", "consumer"},
		"POST /llm-proxies/:id/api-keys/:apiKeyName/regenerate": {"admin", "consumer"},
		"DELETE /llm-proxies/:id/api-keys/:apiKeyName":          {"admin", "consumer"},

		"POST /websub-apis/:id/api-keys":                        {"admin", "consumer"},
		"GET /websub-apis/:id/api-keys":                         {"admin", "consumer"},
		"PUT /websub-apis/:id/api-keys/:apiKeyName":             {"admin", "consumer"},
		"POST /websub-apis/:id/api-keys/:apiKeyName/regenerate": {"admin", "consumer"},
		"DELETE /websub-apis/:id/api-keys/:apiKeyName":          {"admin", "consumer"},

		"POST /websub-apis/:id/secrets":                        {"admin", "consumer"},
		"GET /websub-apis/:id/secrets":                         {"admin", "consumer"},
		"DELETE /websub-apis/:id/secrets/:secretName":          {"admin", "consumer"},
		"POST /websub-apis/:id/secrets/:secretName/regenerate": {"admin", "consumer"},

		"POST /webbroker-apis/:id/api-keys":                        {"admin", "consumer"},
		"GET /webbroker-apis/:id/api-keys":                         {"admin", "consumer"},
		"PUT /webbroker-apis/:id/api-keys/:apiKeyName":             {"admin", "consumer"},
		"POST /webbroker-apis/:id/api-keys/:apiKeyName/regenerate": {"admin", "consumer"},
		"DELETE /webbroker-apis/:id/api-keys/:apiKeyName":          {"admin", "consumer"},

		// Root-level subscription endpoints
		"POST /subscriptions":                   {"admin", "developer"},
		"GET /subscriptions":                    {"admin", "developer"},
		"GET /subscriptions/:subscriptionId":    {"admin", "developer"},
		"PUT /subscriptions/:subscriptionId":    {"admin", "developer"},
		"DELETE /subscriptions/:subscriptionId": {"admin", "developer"},

		// Subscription plan endpoints
		"POST /subscription-plans":           {"admin", "developer"},
		"GET /subscription-plans":            {"admin", "developer"},
		"GET /subscription-plans/:planId":    {"admin", "developer"},
		"PUT /subscription-plans/:planId":    {"admin", "developer"},
		"DELETE /subscription-plans/:planId": {"admin", "developer"},

		"POST /secrets":       {"admin"},
		"GET /secrets":        {"admin"},
		"GET /secrets/:id":    {"admin"},
		"PUT /secrets/:id":    {"admin"},
		"DELETE /secrets/:id": {"admin"},
	}

	// Merge extension-provided roles before building the DefaultResourceRoles map.
	for methodAndPath, roles := range additionalRoles {
		relativeRoles[methodAndPath] = roles
	}

	// Populate both the versioned and legacy (unprefixed) keys so the auth
	// middleware matches either route form. The legacy form is deprecated and
	// will be removed in a future release.
	DefaultResourceRoles := make(map[string][]string, len(relativeRoles)*2)
	for methodAndPath, roles := range relativeRoles {
		DefaultResourceRoles[prefixed(methodAndPath)] = roles
		DefaultResourceRoles[methodAndPath] = roles
	}

	basicAuth := commonmodels.BasicAuth{Enabled: false}
	idpAuth := commonmodels.IDPConfig{Enabled: false}
	if cfg.Controller.Auth.Basic.Enabled {
		users := make([]commonmodels.User, len(cfg.Controller.Auth.Basic.Users))
		for i, authUser := range cfg.Controller.Auth.Basic.Users {
			users[i] = commonmodels.User{
				Username:       authUser.Username,
				Password:       authUser.Password,
				PasswordHashed: authUser.PasswordHashed,
				Roles:          authUser.Roles,
			}
		}
		basicAuth = commonmodels.BasicAuth{Enabled: true, Users: users}
	}
	if cfg.Controller.Auth.IDP.Enabled {
		idpAuth = commonmodels.IDPConfig{
			Enabled:           true,
			IssuerURL:         cfg.Controller.Auth.IDP.Issuer,
			JWKSUrl:           cfg.Controller.Auth.IDP.JWKSURL,
			ScopeClaim:        cfg.Controller.Auth.IDP.RolesClaim,
			PermissionMapping: &cfg.Controller.Auth.IDP.RoleMapping,
		}
	}
	return commonmodels.AuthConfig{
		BasicAuth:     &basicAuth,
		JWTConfig:     &idpAuth,
		ResourceRoles: DefaultResourceRoles,
	}
}
