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
	// AuthSkipKey is set to true in the context when the request matches the
	// auth whitelist and should bypass both authentication and authorization.
	AuthSkipKey = "auth_skip"
)

// AuthMiddleware verifies credentials and prepares authentication context.
func AuthMiddleware(cfg *config.Config, logger *zap.Logger) gin.HandlerFunc {
	basic := BasicAuthMiddleware(cfg, logger)

	// Using "allowList" as a culturally sensitive and descriptive term.
	// A slice is easier to read for small sets of data.
	var allowList = []string{
		"GET /health",
	}

	return func(c *gin.Context) {
		resourcePath := c.FullPath()
		if resourcePath == "" {
			resourcePath = c.Request.URL.Path
		}
		methodKey := c.Request.Method + " " + resourcePath

		// Check if the resource is in the allowList
		isAllowed := false
		for _, path := range allowList {
			if path == methodKey {
				isAllowed = true
				break
			}
		}
		if isAllowed {
			c.Set(AuthSkipKey, true)
			c.Next()
			return
		}
		basic(c)
	}
}
