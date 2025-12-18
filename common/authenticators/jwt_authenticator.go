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
	"slices"
	"strings"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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
}

// NewJWTAuthenticator creates a new JWT authenticator
func NewJWTAuthenticator(config *models.AuthConfig, logger *zap.Logger) *JWTAuthenticator {
	return &JWTAuthenticator{
		config: config,
		logger: logger,
	}
}

// Authenticate verifies JWT token from context
func (j *JWTAuthenticator) Authenticate(ctx *gin.Context) (*AuthResult, error) {
	// Extract bearer token from Authorization header
	authHeader := ctx.GetHeader("Authorization")
	if authHeader == "" {
		return nil, errors.New("authorization header missing")
	}

	// Remove "Bearer " prefix
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return nil, errors.New("invalid authorization header format")
	}

	// Get JWKS URL from config
	if j.config.JWTConfig.IssuerURL == "" {
		return nil, errors.New("JWKS endpoint not configured")
	}

	// Create JWKS provider
	jwksProvider, err := keyfunc.Get(j.config.JWTConfig.JWKSUrl, keyfunc.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to get JWKS: %w", err)
	}

	claims := jwt.MapClaims{}
	validatedToken, err := jwt.ParseWithClaims(tokenString, claims, jwksProvider.Keyfunc)

	if err != nil {
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
	permissions := j.resolvePermissions(claims)
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
	if j.config.JWTConfig.ScopeClaim == "" {
		return []string{}
	}
	var permissions []string
	// Try string first
	if permissionClaimValue, ok := claims[j.config.JWTConfig.ScopeClaim].(string); ok {
		permissions = strings.Split(permissionClaimValue, " ")
	}

	// Try string array
	if permissionClaimArray, ok := claims[j.config.JWTConfig.ScopeClaim].([]any); ok {
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
		for _, perm := range permissions {
			if mappedPerm, ok := (*j.config.JWTConfig.PermissionMapping)[perm]; ok {
				j.logger.Sugar().Debugf("mapped perm %v", mappedPerm)
				mappedPermissions = append(mappedPermissions, mappedPerm...)
			} else {
				j.logger.Sugar().Debugf("unmapped perm %v", perm)
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
	authHeader := ctx.GetHeader(string("Authorization"))
	if authHeader == "" {
		return false
	}
	// Determine auth type from header
	canHandle := strings.HasPrefix(authHeader, "Bearer ")
	j.logger.Sugar().Debugf("can handle token %v", canHandle)
	return canHandle
}
