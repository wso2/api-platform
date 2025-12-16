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
)

// AuthMiddleware verifies credentials against locally configured users in config.
// It currently supports plain-text passwords and bcrypt-hashed passwords (when
// `password_hashed` is true).
func AuthMiddleware(cfg *config.Config, logger *zap.Logger) gin.HandlerFunc {
	// Choose middleware implementation. In future this can select IDP middleware.
	basic := BasicAuthMiddleware(cfg, logger)

	return func(c *gin.Context) {
		// For now, use basic auth. If IDP gets implemented, select based on cfg.
		basic(c)
	}
}
