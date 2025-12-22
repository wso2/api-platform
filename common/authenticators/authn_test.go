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
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/common/models"
	"go.uber.org/zap"
)

func TestAuthMiddleware_NoAuthenticatorsConfigured_AllowsAllRequests(t *testing.T) {
	// Scenario: Both basic.enabled and idp.enabled are false
	// Middleware should enable no-auth mode and allow all requests.
	gin.SetMode(gin.TestMode)
	router := gin.New()
	logger := zap.NewNop()

	config := models.AuthConfig{
		BasicAuth: &models.BasicAuth{Enabled: false},
		JWTConfig: &models.IDPConfig{Enabled: false},
	}

	middleware, err := AuthMiddleware(config, logger)
	assert.NoError(t, err)

	router.Use(middleware)
	router.GET("/api/test", func(c *gin.Context) {
		skipAuthz, exists := c.Get(constants.AuthzSkipKey)
		assert.True(t, exists)
		assert.True(t, skipAuthz.(bool))
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}

func TestAuthMiddleware_JWTEnabled_MissingJWKS_FailsAtCreation(t *testing.T) {
	// Scenario: JWT is enabled but JWKS URL is not configured.
	// Should fail at middleware creation time (startup), not per-request.
	logger := zap.NewNop()

	config := models.AuthConfig{
		JWTConfig: &models.IDPConfig{
			Enabled:   true,
			IssuerURL: "https://issuer.example.com",
			JWKSUrl:   "", // triggers constructor error deterministically
		},
	}

	_, err := AuthMiddleware(config, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize JWT authenticator")
}

func TestAuthMiddleware_BasicAuthEnabled_NoCredentials_Unauthorized(t *testing.T) {
	// Scenario: Basic auth is enabled, but no credentials provided
	// Should return 401 Unauthorized
	gin.SetMode(gin.TestMode)
	router := gin.New()
	logger := zap.NewNop()

	config := models.AuthConfig{
		BasicAuth: &models.BasicAuth{
			Enabled: true,
			Users: []models.User{
				{
					Username: "testuser",
					Password: "testpass",
					Roles:    []string{"developer"},
				},
			},
		},
		JWTConfig: &models.IDPConfig{
			Enabled: false,
		},
	}

	middleware, err := AuthMiddleware(config, logger)
	assert.NoError(t, err)

	router.Use(middleware)
	router.GET("/api/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "protected"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/protected", nil)
	// No Authorization header
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "no valid authentication credentials provided")
}

func TestAuthMiddleware_SkipPaths_NoAuthRequired(t *testing.T) {
	// Scenario: Path is in SkipPaths, should bypass authentication even if auth is enabled
	gin.SetMode(gin.TestMode)
	router := gin.New()
	logger := zap.NewNop()

	config := models.AuthConfig{
		BasicAuth: &models.BasicAuth{
			Enabled: true,
			Users: []models.User{
				{
					Username: "testuser",
					Password: "testpass",
					Roles:    []string{"developer"},
				},
			},
		},
		SkipPaths: []string{"/health", "/metrics"},
	}

	middleware, err := AuthMiddleware(config, logger)
	assert.NoError(t, err)

	router.Use(middleware)
	router.GET("/health", func(c *gin.Context) {
		skipAuthz, exists := c.Get(constants.AuthzSkipKey)
		assert.True(t, exists)
		assert.True(t, skipAuthz.(bool))
		c.JSON(http.StatusOK, gin.H{"message": "healthy"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
}

func TestAuthMiddleware_NilBasicAuth_NilJWTConfig_AllowsAllRequests(t *testing.T) {
	// Scenario: Auth configs are nil (not just disabled)
	// Middleware should enable no-auth mode and allow all requests.
	gin.SetMode(gin.TestMode)
	router := gin.New()
	logger := zap.NewNop()

	config := models.AuthConfig{BasicAuth: nil, JWTConfig: nil}

	middleware, err := AuthMiddleware(config, logger)
	assert.NoError(t, err)

	router.Use(middleware)
	router.GET("/api/open", func(c *gin.Context) {
		skipAuthz, exists := c.Get(constants.AuthzSkipKey)
		assert.True(t, exists)
		assert.True(t, skipAuthz.(bool))
		c.JSON(http.StatusOK, gin.H{"message": "open access"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/open", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "open access")
}
