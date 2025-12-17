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

package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"go.uber.org/zap"
)

// Context keys
const (
	AuthUserKey  = "auth_user"
	AuthRolesKey = "auth_roles"
	// AuthScopesKey may be set by IDP middleware to pass OAuth/OIDC scopes
	// granted for the current request. If present, authorization will validate
	// these scopes directly against resource required scopes.
	AuthScopesKey = "auth_scopes"
	// AuthSkipKey is set to true in the context when the request matches the
	// auth whitelist and should bypass both authentication and authorization.
	AuthSkipKey = "auth_skip"
)

// AuthMiddleware verifies credentials and prepares authentication context.
func AuthMiddleware(cfg *config.Config, logger *zap.Logger) gin.HandlerFunc {
	// Choose middleware implementation. In future this can select IDP middleware.
	basic := BasicAuthMiddleware(cfg, logger)

	// Resources which do not require authentication. Keys should be defined as
	// "METHOD /path". Populate as needed.
	var authWhitelist = map[string]struct{}{
		"GET /health": {},
	}

	return func(c *gin.Context) {
		// Determine request resource key (prefer full path)
		resourcePath := c.FullPath()
		logger.Info("Processing authentication request", zap.String("resource path", resourcePath))
		if resourcePath == "" {
			resourcePath = c.Request.URL.Path
			logger.Info("Processing authentication request", zap.String("resource path", resourcePath))
		}
		methodKey := c.Request.Method + " " + resourcePath

		// If resource is whitelisted, mark skip and continue without auth
		if _, ok := authWhitelist[methodKey]; ok {
			c.Set(AuthSkipKey, true)
			c.Next()
			return
		}

		// For now, use basic auth. If IDP gets implemented, select based on cfg.
		basic(c)
	}
}
