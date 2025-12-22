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
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/common/models"
	"go.uber.org/zap"
)

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token has expired")
	ErrInvalidSignature = errors.New("invalid token signature")
)

// JWTAuthenticator implements JWT authentication
type JWTAuthenticator struct {
	config *models.AuthConfig
	logger *zap.Logger
	jwks   keyfunc.Keyfunc
}

// NewJWTAuthenticator creates a new JWT authenticator
func NewJWTAuthenticator(config *models.AuthConfig, logger *zap.Logger) (*JWTAuthenticator, error) {
	return newJWTAuthenticatorWithJWKS(config, logger, true)
}

// newJWTAuthenticatorWithJWKS creates a new JWT authenticator with optional JWKS initialization
// This is useful for testing where JWKS is not needed
func newJWTAuthenticatorWithJWKS(config *models.AuthConfig, logger *zap.Logger, initJWKS bool) (*JWTAuthenticator, error) {
	var jwks keyfunc.Keyfunc
	if config.JWTConfig != nil && initJWKS {
		if config.JWTConfig.IssuerURL == "" {
			return nil, errors.New("issuer URL not configured")
		}
		// Get Issuer URL from config
		if config.JWTConfig.JWKSUrl == "" {
			return nil, errors.New("JWKS endpoint not configured")
		}

		// Create JWKS storage with custom validation options to skip X5TS256 validation
		// This is required for some OIDC providers like Asgardeo that may have X5TS256 mismatches
		ctx := context.Background()
		storageOptions := jwkset.HTTPClientStorageOptions{
			Ctx:             ctx,
			RefreshInterval: 10 * time.Minute,
			ValidateOptions: jwkset.JWKValidateOptions{
				SkipAll: true, // Skip JWK metadata validation to handle provider inconsistencies (JWT signature validation still occurs)
			},
		}

		storage, err := jwkset.NewStorageFromHTTP(config.JWTConfig.JWKSUrl, storageOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to create JWKS storage: %w", err)
		}

		// Create keyfunc with the custom storage
		keyfuncOptions := keyfunc.Options{
			Ctx:     ctx,
			Storage: storage,
		}
		tempjwksProvider, err := keyfunc.New(keyfuncOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to create JWKS provider: %w", err)
		}
		jwks = tempjwksProvider
	}
	return &JWTAuthenticator{
		config: config,
		logger: logger,
		jwks:   jwks,
	}, nil
}

// Authenticate verifies JWT token from context
func (j *JWTAuthenticator) Authenticate(ctx *gin.Context) (*AuthResult, error) {
	// Extract bearer token from Authorization header
	authHeader := ctx.GetHeader(constants.AuthorizationHeader)
	if authHeader == "" {
		return nil, errors.New("authorization header missing")
	}

	// Remove "Bearer " prefix
	tokenString := strings.TrimPrefix(authHeader, constants.BearerPrefix)
	if tokenString == authHeader {
		return nil, errors.New("invalid authorization header format")
	}

	claims := jwt.MapClaims{}
	validatedToken, err := jwt.ParseWithClaims(tokenString, claims, j.jwks.Keyfunc)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	if !validatedToken.Valid {
		return nil, ErrInvalidToken
	}

	// Validate issuer if configured
	if j.config.JWTConfig.IssuerURL != "" {
		issuer, err := claims.GetIssuer()
		if err != nil {
			return nil, fmt.Errorf("failed to get issuer: %w", err)
		}
		if issuer != j.config.JWTConfig.IssuerURL {
			return nil, fmt.Errorf("invalid issuer: expected %s, got %s", j.config.JWTConfig.IssuerURL, issuer)
		}
	}

	// Validate audience if configured
	if j.config.JWTConfig.Audience != nil && *j.config.JWTConfig.Audience != "" {
		audience, err := claims.GetAudience()
		if err != nil {
			return nil, fmt.Errorf("failed to get audience: %w", err)
		}
		validAudience := slices.Contains(audience, *j.config.JWTConfig.Audience)
		if !validAudience {
			return nil, errors.New("invalid audience")
		}
	}

	// If no role claim is configured, set flag to skip authorization
	// This allows authentication-only mode where all authenticated users can access resources
	var permissions []string
	if j.config.JWTConfig.ScopeClaim == "" {
		j.logger.Debug("No role claim configured, setting skip_authz flag")
		ctx.Set(constants.AuthzSkipKey, true)
		permissions = []string{}
	} else {
		permissions = j.resolvePermissions(claims)
	}
	subject, err := claims.GetSubject()
	if err != nil {
		return nil, fmt.Errorf("failed to get subject: %w", err)
	}
	return &AuthResult{
		Success: true,
		UserID:  subject,
		Roles:   permissions,
		Claims:  claims,
	}, nil

}

func (j *JWTAuthenticator) resolvePermissions(claims jwt.MapClaims) []string {
	var permissions []string
	// Try string first
	if permissionClaimValue, ok := claims[j.config.JWTConfig.ScopeClaim].(string); ok {
		permissions = strings.Split(permissionClaimValue, " ")
	} else if permissionClaimArray, ok := claims[j.config.JWTConfig.ScopeClaim].([]any); ok {
		permissions = make([]string, 0, len(permissionClaimArray))
		for _, perm := range permissionClaimArray {
			if permStr, ok := perm.(string); ok {
				permissions = append(permissions, permStr)
			}
		}
	}
	j.logger.Sugar().Debugf("permissions %v", permissions)
	j.logger.Sugar().Debugf("permission mapping %v", j.config.JWTConfig.PermissionMapping)
	if j.config.JWTConfig.PermissionMapping != nil {
		mappedPermissions := []string{}

		// Find if there's a wildcard role (role with "*" mapping)
		var wildcardRole string
		for role, claimValues := range *j.config.JWTConfig.PermissionMapping {
			for _, claimValue := range claimValues {
				if claimValue == "*" {
					wildcardRole = role
					break
				}
			}
			if wildcardRole != "" {
				break
			}
		}
		j.logger.Sugar().Debugf("wildcard role: %v", wildcardRole)

		for _, perm := range permissions {
			mapped := false
			// Check each role's claim values to see if this permission matches
			for role, claimValues := range *j.config.JWTConfig.PermissionMapping {
				for _, claimValue := range claimValues {
					if claimValue == "*" {
						// Skip wildcard entries during specific matching
						continue
					}
					if claimValue == perm {
						j.logger.Sugar().Debugf("mapped claim value %v to role %v", perm, role)
						mappedPermissions = append(mappedPermissions, role)
						mapped = true
					}
				}
			}

			// If no specific mapping found and wildcard role exists, use wildcard
			if !mapped && wildcardRole != "" {
				j.logger.Sugar().Debugf("mapped claim value %v to wildcard role %v", perm, wildcardRole)
				mappedPermissions = append(mappedPermissions, wildcardRole)
			} else if !mapped {
				// No mapping and no wildcard, keep original
				j.logger.Sugar().Debugf("unmapped claim value %v", perm)
				mappedPermissions = append(mappedPermissions, perm)
			}
		}
		return mappedPermissions
	}
	return permissions
}

// Name returns the authenticator name
func (j *JWTAuthenticator) Name() string {
	return "JWTAuthenticator"
}

// CanHandle checks if credentials in context are JWTCredentials
func (j *JWTAuthenticator) CanHandle(ctx *gin.Context) bool {
	authHeader := ctx.GetHeader(constants.AuthorizationHeader)
	if authHeader == "" {
		return false
	}
	// Determine auth type from header
	canHandle := strings.HasPrefix(authHeader, constants.BearerPrefix)
	j.logger.Sugar().Debugf("can handle token %v", canHandle)
	return canHandle
}
