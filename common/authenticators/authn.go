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
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/common/models"
	"go.uber.org/zap"
)

var (
	ErrNoAuthenticator = errors.New("no suitable authenticator found")
)

// AuthMiddleware creates a unified authentication middleware supporting both Basic and Bearer auth
func AuthMiddleware(config models.AuthConfig, logger *zap.Logger) (gin.HandlerFunc, error) {
	// Initialize authenticators once at startup (middleware creation time).
	// Any configuration errors (e.g., JWT JWKS init failures) should fail fast here
	// rather than per-request.
	authenticators := []Authenticator{}

	// Add Basic authenticator if configured
	if config.BasicAuth != nil && config.BasicAuth.Enabled && len(config.BasicAuth.Users) > 0 {
		authenticators = append(authenticators, NewBasicAuthenticator(config, logger))
	}

	// Add JWT authenticator if configured
	if config.JWTConfig != nil && config.JWTConfig.Enabled && config.JWTConfig.IssuerURL != "" {
		jwtAuthenticator, err := NewJWTAuthenticator(&config, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize JWT authenticator: %w", err)
		}
		authenticators = append(authenticators, jwtAuthenticator)
	}

	// No authenticators configured => run in no-auth mode.
	// This disables both authentication and authorization (via AuthzSkipKey).
	if len(authenticators) == 0 {
		return func(c *gin.Context) {
			authCtx := models.AuthContext{
				Authenticated: true,
				UserID:        "sys_noauth_user",
				Roles:         []string{},
				Claims:        map[string]any{},
			}
			c.Set(constants.AuthContextKey, authCtx)
			c.Set(constants.AuthzSkipKey, true)
			c.Next()
		}, nil
	}

	return func(c *gin.Context) {
		// Skip authentication for specified paths
		for _, path := range config.SkipPaths {
			if strings.HasPrefix(c.Request.URL.Path, path) {
				c.Set(constants.AuthzSkipKey, true)
				c.Next()
				return
			}
		}

		// Find suitable authenticator
		var selectedAuth Authenticator
		for _, auth := range authenticators {
			if auth.CanHandle(c) {
				selectedAuth = auth
				break
			}
		}

		if selectedAuth == nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "no valid authentication credentials provided",
			})
			c.Abort()
			return
		}

		// Authenticate
		result, err := selectedAuth.Authenticate(c)
		if err != nil {
			logger.Sugar().Errorf("Authentication error: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "authentication failed",
			})
			c.Abort()
			return
		}
		logger.Sugar().Debugf("Authentication result %v", result)
		logger.Sugar().Debugf("Authentication roles %v", result.Roles)

		claims := result.Claims
		if claims == nil {
			claims = map[string]any{}
		}
		// Set authentication context
		authCtx := models.AuthContext{
			Authenticated: result.Success,
			UserID:        result.UserID,
			Roles:         result.Roles,
			Claims:        claims,
		}
		c.Set(constants.AuthContextKey, authCtx)

		c.Next()
	}, nil
}
