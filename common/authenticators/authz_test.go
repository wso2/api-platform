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

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestAuthorizationMiddleware_NoResourceRoles_AllowsAllRequests(t *testing.T) {
	// Scenario: When idp.enabled=true but roles_claim and role_mapping are not provided
	// Authorization should be bypassed - all authenticated calls are allowed
	router := setupTestRouter()
	logger := zap.NewNop()

	// Config with no ResourceRoles defined (empty map)
	config := models.AuthConfig{
		ResourceRoles: map[string][]string{},
	}

	router.Use(func(c *gin.Context) {
		// Simulate that user is authenticated with some roles
		c.Set(constants.AuthContextKey, models.AuthContext{Roles: []string{"developer", "consumer"}})
		c.Next()
	})
	router.Use(AuthorizationMiddleware(config, logger))
	router.GET("/api/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}

func TestAuthorizationMiddleware_NoResourceRoles_NilMap_AllowsAllRequests(t *testing.T) {
	// Scenario: When ResourceRoles is nil (not initialized)
	// Authorization should be bypassed
	router := setupTestRouter()
	logger := zap.NewNop()

	// Config with nil ResourceRoles
	config := models.AuthConfig{
		ResourceRoles: nil,
	}

	router.Use(func(c *gin.Context) {
		// Simulate authenticated user
		c.Set(constants.AuthContextKey, models.AuthContext{Roles: []string{"admin"}})
		c.Next()
	})
	router.Use(AuthorizationMiddleware(config, logger))
	router.POST("/api/resources", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "created"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/resources", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "created")
}

func TestAuthorizationMiddleware_WithResourceRoles_MatchingRole_Allowed(t *testing.T) {
	// Scenario: When roles_claim and role_mapping are provided
	// Authorization happens - user with matching role is allowed
	router := setupTestRouter()
	logger := zap.NewNop()

	// Config with specific ResourceRoles mapping
	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/users": {"developer", "admin"},
		},
	}

	router.Use(func(c *gin.Context) {
		// User has developer role which is allowed
		c.Set(constants.AuthContextKey, models.AuthContext{Roles: []string{"developer"}})
		c.Next()
	})
	router.Use(AuthorizationMiddleware(config, logger))
	router.GET("/api/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}

func TestAuthorizationMiddleware_WithResourceRoles_NoMatchingRole_Forbidden(t *testing.T) {
	// Scenario: Authorization is enabled and user doesn't have required role
	router := setupTestRouter()
	logger := zap.NewNop()

	// Config with specific ResourceRoles mapping
	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/users": {"admin"},
		},
	}

	router.Use(func(c *gin.Context) {
		// User has developer role but endpoint requires admin
		c.Set(constants.AuthContextKey, models.AuthContext{Roles: []string{"developer"}})
		c.Next()
	})
	router.Use(AuthorizationMiddleware(config, logger))
	router.GET("/api/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "forbidden")
}

func TestAuthorizationMiddleware_WithResourceRoles_MultipleRoles_OneMatches_Allowed(t *testing.T) {
	// Scenario: User has multiple roles, one of them matches the required role
	router := setupTestRouter()
	logger := zap.NewNop()

	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"POST /api/resources": {"admin", "developer"},
		},
	}

	router.Use(func(c *gin.Context) {
		// User has developer, consumer, and admin roles
		c.Set(constants.AuthContextKey, models.AuthContext{Roles: []string{"developer", "consumer", "admin"}})
		c.Next()
	})
	router.Use(AuthorizationMiddleware(config, logger))
	router.POST("/api/resources", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "created"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/resources", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "created")
}

func TestAuthorizationMiddleware_ResourceNotDefined_Forbidden(t *testing.T) {
	// Scenario: Resource is not in the ResourceRoles map
	// Should be forbidden (secure by default)
	router := setupTestRouter()
	logger := zap.NewNop()

	// Config with specific resources defined
	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/users": {"admin"},
		},
	}

	router.Use(func(c *gin.Context) {
		c.Set(constants.AuthContextKey, models.AuthContext{Roles: []string{"admin"}})
		c.Next()
	})
	router.Use(AuthorizationMiddleware(config, logger))
	router.GET("/api/products", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/products", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAuthorizationMiddleware_NoUserRoles_Forbidden(t *testing.T) {
	// Scenario: User is authenticated but has no roles
	router := setupTestRouter()
	logger := zap.NewNop()

	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/users": {"developer"},
		},
	}

	router.Use(func(c *gin.Context) {
		// User has no roles
		c.Set(constants.AuthContextKey, models.AuthContext{Roles: []string{}})
		c.Next()
	})
	router.Use(AuthorizationMiddleware(config, logger))
	router.GET("/api/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAuthorizationMiddleware_RolesNotSetInContext_Forbidden(t *testing.T) {
	// Scenario: Roles were not set in context (authentication may have failed)
	router := setupTestRouter()
	logger := zap.NewNop()

	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/users": {"developer"},
		},
	}

	// No middleware to set roles
	router.Use(AuthorizationMiddleware(config, logger))
	router.GET("/api/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAuthorizationMiddleware_AuthSkipped_BypassesAuthorization(t *testing.T) {
	// Scenario: Authentication was skipped (e.g., public endpoint)
	// Authorization should also be skipped
	router := setupTestRouter()
	logger := zap.NewNop()

	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/public": {"admin"}, // Requires admin but will be skipped
		},
	}

	router.Use(func(c *gin.Context) {
		// Simulate that authentication was skipped
		c.Set(constants.AuthzSkipKey, true)
		c.Next()
	})
	router.Use(AuthorizationMiddleware(config, logger))
	router.GET("/api/public", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "public access"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/public", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "public access")
}

func TestAuthorizationMiddleware_DifferentMethodsSamePathDifferentRoles(t *testing.T) {
	// Scenario: Same path but different methods require different roles
	logger := zap.NewNop()

	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/users":    {"consumer", "developer", "admin"},
			"POST /api/users":   {"admin"},
			"DELETE /api/users": {"admin"},
		},
	}

	// Test GET with developer role - should succeed
	router1 := setupTestRouter()
	router1.Use(func(c *gin.Context) {
		c.Set(constants.AuthContextKey, models.AuthContext{Roles: []string{"developer"}})
		c.Next()
	})
	router1.Use(AuthorizationMiddleware(config, logger))
	router1.GET("/api/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "list users"})
	})

	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/api/users", nil)
	router1.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Test POST with developer role - should fail
	router2 := setupTestRouter()
	router2.Use(func(c *gin.Context) {
		c.Set(constants.AuthContextKey, models.AuthContext{Roles: []string{"developer"}})
		c.Next()
	})
	router2.Use(AuthorizationMiddleware(config, logger))
	router2.POST("/api/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "created"})
	})

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/users", nil)
	router2.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusForbidden, w2.Code)

	// Test DELETE with admin role - should succeed
	router3 := setupTestRouter()
	router3.Use(func(c *gin.Context) {
		c.Set(constants.AuthContextKey, models.AuthContext{Roles: []string{"admin"}})
		c.Next()
	})
	router3.Use(AuthorizationMiddleware(config, logger))
	router3.DELETE("/api/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	})

	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("DELETE", "/api/users", nil)
	router3.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
}

func TestAuthorizationMiddleware_WildcardRoleMapping(t *testing.T) {
	// Scenario: Role mapping with wildcard - all values from claim should be accepted
	// This simulates: developer: ["*"]
	// In practice, this means any user with "developer" role can access resources
	// that list "developer" as allowed, regardless of other claim values
	router := setupTestRouter()
	logger := zap.NewNop()

	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/data": {"developer"}, // developer role is allowed
		},
	}

	// User has developer role (could have any value from IDP claim)
	router.Use(func(c *gin.Context) {
		c.Set(constants.AuthContextKey, models.AuthContext{Roles: []string{"developer"}})
		c.Next()
	})
	router.Use(AuthorizationMiddleware(config, logger))
	router.GET("/api/data", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "data"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/data", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "data")
}

func TestAuthorizationMiddleware_CaseSensitiveRoles(t *testing.T) {
	// Scenario: Roles should be case-sensitive
	logger := zap.NewNop()

	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/users": {"Admin"}, // Uppercase Admin
		},
	}

	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		// User has lowercase admin
		c.Set(constants.AuthContextKey, models.AuthContext{Roles: []string{"admin"}})
		c.Next()
	})
	router.Use(AuthorizationMiddleware(config, logger))
	router.GET("/api/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users", nil)
	router.ServeHTTP(w, req)

	// Should be forbidden due to case mismatch
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAuthorizationMiddleware_SkipAuthzFlag_BypassesAuthorization(t *testing.T) {
	// Scenario: When JWT authenticator sets skip_authz flag (no role claim configured)
	// Authorization should be bypassed even if ResourceRoles are defined
	router := setupTestRouter()
	logger := zap.NewNop()

	// Config with ResourceRoles defined (normally would enforce authorization)
	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/users": {"admin", "developer"}, // Strict requirements
		},
	}

	router.Use(func(c *gin.Context) {
		// Simulate JWT authenticator setting skip_authz flag when no role claim configured
		c.Set(constants.AuthzSkipKey, true)
		// User might not even have roles
		c.Set(constants.AuthContextKey, models.AuthContext{Roles: []string{}})
		c.Next()
	})
	router.Use(AuthorizationMiddleware(config, logger))
	router.GET("/api/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users", nil)
	router.ServeHTTP(w, req)

	// Should succeed because skip_authz flag bypasses authorization
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}
