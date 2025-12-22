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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/common/models"
	"go.uber.org/zap"
)

type staticKeyfunc struct {
	key any
}

func (s staticKeyfunc) Keyfunc(token *jwt.Token) (any, error) { return s.key, nil }

func (s staticKeyfunc) KeyfuncCtx(ctx context.Context) jwt.Keyfunc {
	return func(token *jwt.Token) (any, error) { return s.key, nil }
}

func (s staticKeyfunc) Storage() jwkset.Storage { return nil }

func (s staticKeyfunc) VerificationKeySet(ctx context.Context) (jwt.VerificationKeySet, error) {
	return jwt.VerificationKeySet{}, nil
}

func TestJWTAuthenticator_ResolvePermissions_WildcardMapping(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name              string
		scopeClaim        string
		claimValue        interface{}
		permissionMapping *map[string][]string
		expectedRoles     []string
		description       string
	}{
		{
			name:       "Wildcard mapping with no specific assignments",
			scopeClaim: "group",
			claimValue: []interface{}{"developer", "consumer", "admin"},
			permissionMapping: &map[string][]string{
				"admin": {"*"},
			},
			expectedRoles: []string{"admin", "admin", "admin"},
			description:   "All claim values map to admin because of wildcard",
		},
		{
			name:       "Wildcard mapping with specific assignments - specific takes precedence",
			scopeClaim: "group",
			claimValue: []interface{}{"developer", "consumer", "admin"},
			permissionMapping: &map[string][]string{
				"admin":     {"*"},
				"developer": {"developer"},
				"consumer":  {"consumer"},
			},
			expectedRoles: []string{"developer", "consumer", "admin"},
			description:   "developer->developer, consumer->consumer, admin->admin (wildcard)",
		},
		{
			name:       "No wildcard - specific mappings only",
			scopeClaim: "group",
			claimValue: []interface{}{"developer", "consumer"},
			permissionMapping: &map[string][]string{
				"developer": {"developer"},
			},
			expectedRoles: []string{"developer", "consumer"},
			description:   "developer maps to developer, consumer stays as is",
		},
		{
			name:       "Wildcard with string claim value",
			scopeClaim: "scope",
			claimValue: "developer consumer admin",
			permissionMapping: &map[string][]string{
				"admin":    {"*"},
				"consumer": {"consumer"},
			},
			expectedRoles: []string{"admin", "consumer", "admin"},
			description:   "consumer->consumer (specific), developer and admin->admin (wildcard)",
		},
		{
			name:       "Multiple values map to same role via wildcard",
			scopeClaim: "group",
			claimValue: []interface{}{"developer", "consumer", "admin"},
			permissionMapping: &map[string][]string{
				"developer": {"*"},
			},
			expectedRoles: []string{"developer", "developer", "developer"},
			description:   "All roles map to single local developer role",
		},
		{
			name:              "No permission mapping - returns original permissions",
			scopeClaim:        "group",
			claimValue:        []interface{}{"admin", "developer"},
			permissionMapping: nil,
			expectedRoles:     []string{"admin", "developer"},
			description:       "Without mapping, original claim values are returned",
		},
		{
			name:       "Empty permission mapping - returns original permissions",
			scopeClaim: "group",
			claimValue: []interface{}{"admin", "developer"},
			permissionMapping: &map[string][]string{
				"admin": {"*"},
			},
			expectedRoles: []string{"admin", "admin"},
			description:   "Empty mapping with wildcard maps all to admin",
		},
		{
			name:       "Wildcard with one-to-many mapping for specific role",
			scopeClaim: "group",
			claimValue: []interface{}{"admin", "developer", "other"},
			permissionMapping: &map[string][]string{
				"admin":     {"*"},
				"developer": {"developer"},
				"consumer":  {"admin"}, // admin claim maps to consumer specifically
			},
			expectedRoles: []string{"consumer", "developer", "admin"},
			description:   "admin->consumer (specific), developer->developer (specific), other->admin (wildcard)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &models.AuthConfig{
				JWTConfig: &models.IDPConfig{
					ScopeClaim:        tt.scopeClaim,
					PermissionMapping: tt.permissionMapping,
				},
			}

			authenticator, err := newJWTAuthenticatorWithJWKS(config, logger, false)
			assert.NoError(t, err)

			// Create claims with the test claim value
			claims := jwt.MapClaims{
				tt.scopeClaim: tt.claimValue,
			}

			roles := authenticator.resolvePermissions(claims)

			// For test case with multiple mappings to same claim, order may vary due to map iteration
			if tt.name == "Wildcard with one-to-many mapping for specific role" {
				assert.ElementsMatch(t, tt.expectedRoles, roles, tt.description)
			} else {
				assert.Equal(t, tt.expectedRoles, roles, tt.description)
			}
		})
	}
}

func TestJWTAuthenticator_ResolvePermissions_NoScopeClaim(t *testing.T) {
	logger := zap.NewNop()

	config := &models.AuthConfig{
		JWTConfig: &models.IDPConfig{
			ScopeClaim: "",
		},
	}

	authenticator, err := newJWTAuthenticatorWithJWKS(config, logger, false)
	assert.NoError(t, err)

	claims := jwt.MapClaims{
		"group": []interface{}{"role1", "role2"},
	}

	roles := authenticator.resolvePermissions(claims)

	assert.Empty(t, roles, "Should return empty array when ScopeClaim is not configured")
}

func TestJWTAuthenticator_ResolvePermissions_ClaimNotPresent(t *testing.T) {
	logger := zap.NewNop()

	config := &models.AuthConfig{
		JWTConfig: &models.IDPConfig{
			ScopeClaim: "group",
		},
	}

	authenticator, err := newJWTAuthenticatorWithJWKS(config, logger, false)
	assert.NoError(t, err)

	// Claims without the expected scope claim
	claims := jwt.MapClaims{
		"sub": "user123",
	}

	roles := authenticator.resolvePermissions(claims)

	assert.Empty(t, roles, "Should return empty array when claim is not present in JWT")
}

func TestJWTAuthenticator_ResolvePermissions_InvalidClaimType(t *testing.T) {
	logger := zap.NewNop()

	config := &models.AuthConfig{
		JWTConfig: &models.IDPConfig{
			ScopeClaim: "group",
		},
	}

	authenticator, err := newJWTAuthenticatorWithJWKS(config, logger, false)
	assert.NoError(t, err)

	// Claim with invalid type (number instead of string/array)
	claims := jwt.MapClaims{
		"group": 12345,
	}

	roles := authenticator.resolvePermissions(claims)

	assert.Empty(t, roles, "Should return empty array when claim type is invalid")
}

func TestJWTAuthenticator_Authenticate_ExpiredTokenRejected_WithIssuerValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	issuer := "https://issuer.example.com"

	secret := []byte("test-secret")
	expired := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": issuer,
		"sub": "user123",
		"exp": time.Now().Add(-1 * time.Minute).Unix(),
	})
	tokenString, err := expired.SignedString(secret)
	assert.NoError(t, err)

	config := &models.AuthConfig{JWTConfig: &models.IDPConfig{Enabled: true, IssuerURL: issuer}}

	a := &JWTAuthenticator{
		config: config,
		logger: logger,
		jwks:   staticKeyfunc{key: secret},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set(constants.AuthorizationHeader, constants.BearerPrefix+tokenString)
	c.Request = req

	_, err = a.Authenticate(c)
	assert.ErrorIs(t, err, ErrExpiredToken)
}
