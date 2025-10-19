/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// CustomClaims represents the JWT claims structure from Thunder
type CustomClaims struct {
	Audience     string `json:"aud"`
	Email        string `json:"email"`
	FirstName    string `json:"firstName"`
	LastName     string `json:"lastName"`
	JTI          string `json:"jti"`
	Organization string `json:"organization"`
	Scope        string `json:"scope"`
	Username     string `json:"username"`
	jwt.RegisteredClaims
}

// AuthConfig holds the configuration for JWT authentication
type AuthConfig struct {
	SecretKey      string
	TokenIssuer    string
	SkipPaths      []string // Paths to skip authentication
	SkipValidation bool     // Skip token signature validation (for development)
}

// AuthMiddleware creates a JWT authentication middleware
func AuthMiddleware(config AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip authentication for specified paths
		for _, path := range config.SkipPaths {
			if c.Request.URL.Path == path {
				c.Next()
				return
			}
		}

		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header is required",
			})
			c.Abort()
			return
		}

		// Check if the header starts with "Bearer "
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization header format. Expected: Bearer <token>",
			})
			c.Abort()
			return
		}

		var claims *CustomClaims

		// Parse token without validation if SkipValidation is true
		if config.SkipValidation {
			// Parse without validation - just decode the JWT structure
			parser := jwt.NewParser(jwt.WithoutClaimsValidation())
			token, _, parseErr := parser.ParseUnverified(tokenString, &CustomClaims{})

			if parseErr != nil {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": fmt.Sprintf("Invalid JWT format: %v", parseErr),
				})
				c.Abort()
				return
			}

			var ok bool
			claims, ok = token.Claims.(*CustomClaims)
			if !ok {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid token claims",
				})
				c.Abort()
				return
			}
		} else {
			// Parse and validate the token with signature verification
			token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
				// Validate the signing method
				if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(config.SecretKey), nil
			})

			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": fmt.Sprintf("Invalid token: %v", err),
				})
				c.Abort()
				return
			}

			var ok bool
			claims, ok = token.Claims.(*CustomClaims)
			if !ok || !token.Valid {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid token claims",
				})
				c.Abort()
				return
			}

			// Validate issuer if configured
			if config.TokenIssuer != "" && claims.Issuer != config.TokenIssuer {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid token issuer",
				})
				c.Abort()
				return
			}
		}

		// Validate that organization claim exists
		if claims.Organization == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Token missing required 'organization' claim",
			})
			c.Abort()
			return
		}

		// Set claims in context for use in handlers
		c.Set("user_id", claims.Subject) // Use sub as user ID
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("first_name", claims.FirstName)
		c.Set("last_name", claims.LastName)
		c.Set("organization", claims.Organization)
		c.Set("scope", claims.Scope)
		c.Set("audience", claims.Audience)
		c.Set("claims", claims)

		c.Next()
	}
}

// GetOrganizationFromContext extracts the organization claim from the Gin context
func GetOrganizationFromContext(c *gin.Context) (string, bool) {
	organization, exists := c.Get("organization")
	if !exists {
		return "", false
	}
	orgStr, ok := organization.(string)
	return orgStr, ok
}

// GetUserIDFromContext extracts the user ID from the Gin context
func GetUserIDFromContext(c *gin.Context) (string, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", false
	}
	userIDStr, ok := userID.(string)
	return userIDStr, ok
}

// GetUsernameFromContext extracts the username from the Gin context
func GetUsernameFromContext(c *gin.Context) (string, bool) {
	username, exists := c.Get("username")
	if !exists {
		return "", false
	}
	usernameStr, ok := username.(string)
	return usernameStr, ok
}

// GetEmailFromContext extracts the email from the Gin context
func GetEmailFromContext(c *gin.Context) (string, bool) {
	email, exists := c.Get("email")
	if !exists {
		return "", false
	}
	emailStr, ok := email.(string)
	return emailStr, ok
}

// GetFirstNameFromContext extracts the first name from the Gin context
func GetFirstNameFromContext(c *gin.Context) (string, bool) {
	firstName, exists := c.Get("first_name")
	if !exists {
		return "", false
	}
	firstNameStr, ok := firstName.(string)
	return firstNameStr, ok
}

// GetLastNameFromContext extracts the last name from the Gin context
func GetLastNameFromContext(c *gin.Context) (string, bool) {
	lastName, exists := c.Get("last_name")
	if !exists {
		return "", false
	}
	lastNameStr, ok := lastName.(string)
	return lastNameStr, ok
}

// GetScopeFromContext extracts the scope from the Gin context
func GetScopeFromContext(c *gin.Context) (string, bool) {
	scope, exists := c.Get("scope")
	if !exists {
		return "", false
	}
	scopeStr, ok := scope.(string)
	return scopeStr, ok
}

// GetAudienceFromContext extracts the audience from the Gin context
func GetAudienceFromContext(c *gin.Context) (string, bool) {
	audience, exists := c.Get("audience")
	if !exists {
		return "", false
	}
	audienceStr, ok := audience.(string)
	return audienceStr, ok
}

// GetRolesFromContext extracts the roles from the Gin context
// Note: Since scope is a string in your JWT, this function parses it
func GetRolesFromContext(c *gin.Context) ([]string, bool) {
	scope, exists := GetScopeFromContext(c)
	if !exists {
		return nil, false
	}

	// Parse scope string - assuming space-separated scopes like "openid profile admin"
	if scope == "" {
		return []string{}, true
	}

	roles := strings.Fields(scope) // Split by whitespace
	return roles, true
}

// GetClaimsFromContext extracts the full claims object from the Gin context
func GetClaimsFromContext(c *gin.Context) (*CustomClaims, bool) {
	claims, exists := c.Get("claims")
	if !exists {
		return nil, false
	}
	claimsObj, ok := claims.(*CustomClaims)
	return claimsObj, ok
}

// RequireScope creates a middleware that requires specific scopes
func RequireScope(requiredScopes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, exists := GetScopeFromContext(c)
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "No scope found in token",
			})
			c.Abort()
			return
		}

		// Check if user has any of the required scopes
		hasScope := false
		scopes := strings.Fields(scope)
		for _, userScope := range scopes {
			for _, requiredScope := range requiredScopes {
				if userScope == requiredScope {
					hasScope = true
					break
				}
			}
			if hasScope {
				break
			}
		}

		if !hasScope {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient privileges",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireOrganization creates a middleware that requires access to a specific organization
func RequireOrganization(organizationParam string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get organization from token
		tokenOrg, exists := GetOrganizationFromContext(c)
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "No organization found in token",
			})
			c.Abort()
			return
		}

		// Get organization from URL parameter
		requestedOrg := c.Param(organizationParam)
		if requestedOrg == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Organization parameter is required",
			})
			c.Abort()
			return
		}

		// Check if token organization matches requested organization
		if tokenOrg != requestedOrg {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied for the requested organization",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
