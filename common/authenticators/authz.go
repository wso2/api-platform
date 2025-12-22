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
package authenticators

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/common/constants"
	commonerrors "github.com/wso2/api-platform/common/errors"
	"github.com/wso2/api-platform/common/models"
	"go.uber.org/zap"
)

// AuthorizationMiddleware enforces resource->roles mapping stored in this package.
func AuthorizationMiddleware(config models.AuthConfig, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip authorization if authentication was skipped
		if v, ok := c.Get(constants.AuthzSkipKey); ok {
			if skipped, ok2 := v.(bool); ok2 && skipped {
				c.Next()
				return
			}
		}

		// Use config.ResourceRoles if provided, else fallback to DefaultResourceRoles
		resourceRoles := config.ResourceRoles
		logger.Sugar().Debugf("Resource roles %v", resourceRoles)
		if len(resourceRoles) == 0 {
			c.Next()
			return
		}
		// Retrieve user roles from context (set by auth middleware)
		var userRoles []string
		if v, ok := c.Get(constants.AuthContextKey); ok {
			if ac, ok2 := v.(models.AuthContext); ok2 {
				userRoles = ac.Roles
			}
		}
		logger.Sugar().Debugf("User roles %v", userRoles)

		// Determine resource key
		resourcePath := c.FullPath()
		if resourcePath == "" {
			// FullPath may be empty for some middleware ordering; fallback to raw path
			resourcePath = c.Request.URL.Path
		}

		// Try METHOD + path first
		methodKey := c.Request.Method + " " + resourcePath
		logger.Sugar().Debugf("method key %v", methodKey)
		allowed, found := resourceRoles[methodKey]
		if !found {
			// Resource not defined -> reject
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": commonerrors.ErrForbidden.Error()})
			return
		}

		// Check for role intersection
		allowedSet := make(map[string]struct{}, len(allowed))
		for _, r := range allowed {
			allowedSet[r] = struct{}{}
		}

		for _, ur := range userRoles {
			if _, ok := allowedSet[ur]; ok {
				c.Next()
				return
			}
		}

		// No matching role -> forbidden
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": commonerrors.ErrForbidden.Error()})
	}
}
