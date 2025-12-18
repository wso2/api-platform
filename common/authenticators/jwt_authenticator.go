package authenticators

import (
	"errors"
	"fmt"
	"strings"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/wso2/api-platform/common/models"
)

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token has expired")
	ErrInvalidSignature = errors.New("invalid token signature")
)

// JWTAuthenticator implements JWT authentication
type JWTAuthenticator struct {
	config *models.AuthConfig
}

// NewJWTAuthenticator creates a new JWT authenticator
func NewJWTAuthenticator(config *models.AuthConfig) *JWTAuthenticator {
	return &JWTAuthenticator{
		config: config,
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
	jwksProvider, err := keyfunc.Get(j.config.JWTConfig.IssuerURL, keyfunc.Options{})
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
	if *j.config.JWTConfig.Audience != "" {
		audience, err := claims.GetAudience()
		if err != nil {
			return nil, fmt.Errorf("failed to get audience: %w", err)
		}
		validAudience := false
		for _, aud := range audience {
			if aud == *j.config.JWTConfig.Audience {
				validAudience = true
				break
			}
		}
		if !validAudience {
			return nil, errors.New("invalid audience")
		}
	}
	permissions := j.resolvePermissions(claims)
	return &AuthResult{
		Success: true,
		UserID:  claims["sub"].(string),
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
	if permissionClaimArray, ok := claims[j.config.JWTConfig.ScopeClaim].([]interface{}); ok {
		permissions := make([]string, 0, len(permissionClaimArray))
		for _, perm := range permissionClaimArray {
			if permStr, ok := perm.(string); ok {
				permissions = append(permissions, permStr)
			}
		}
	}
	if j.config.JWTConfig.PermissionMapping != nil {
		mappedPermissions := []string{}
		for _, perm := range permissions {
			if mappedPerm, ok := (*j.config.JWTConfig.PermissionMapping)[perm]; ok {
				mappedPermissions = append(mappedPermissions, mappedPerm...)
			} else {
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
	return strings.HasPrefix(authHeader, "Bearer ")
}
