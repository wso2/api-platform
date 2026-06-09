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

	commonconstants "github.com/wso2/api-platform/common/constants"
	commonmodels "github.com/wso2/api-platform/common/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// CustomClaims represents the JWT claims structure used in local JWT (non-IDP) mode.
type CustomClaims struct {
	Audience      string   `json:"aud"`
	Email         string   `json:"email"`
	FirstName     string   `json:"firstName"`
	LastName      string   `json:"lastName"`
	JTI           string   `json:"jti"`
	Organization  string   `json:"organization"`
	Scope         string   `json:"scope"`
	Username      string   `json:"username"`
	jwt.RegisteredClaims
}

// AuthConfig holds the configuration for the local JWT (non-IDP) authentication path.
type AuthConfig struct {
	SecretKey             string
	TokenIssuer           string
	SkipPaths             []string
	SkipValidation        bool   // Skip signature validation — dev/local mode only
	OrganizationClaimName string // JWT claim holding the org ID (default: "organization")
}

// PlatformClaimNames holds the JWT claim names used to extract platform-specific values.
type PlatformClaimNames struct {
	OrganizationClaim  string // active org UUID for this token (e.g. "organization")
	OrgNameClaim       string // org display name (e.g. "org_name")
	OrgHandleClaim     string // org URL-safe handle (e.g. "org_handle")
	UserIDClaim        string
	UsernameClaim      string
	EmailClaim         string
	ScopeClaim         string
	RolesClaimPath     string
	RoleMappings       []string // "idp-value=platform-role" pairs
}

// LocalJWTAuthMiddleware returns a Gin middleware for locally-issued JWT validation.
// Used only when IDP mode is disabled.
func LocalJWTAuthMiddleware(config AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, path := range config.SkipPaths {
			if strings.HasPrefix(c.Request.URL.Path, path) {
				c.Next()
				return
			}
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format. Expected: Bearer <token>"})
			c.Abort()
			return
		}

		if err := validateLocalJWT(c, tokenString, config); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		c.Next()
	}
}

// validateLocalJWT handles locally-issued JWT validation (non-IDP mode).
// When SkipValidation is true the signature is not verified — intended for local development only.
func validateLocalJWT(c *gin.Context, tokenString string, config AuthConfig) error {
	mapClaims := jwt.MapClaims{}

	if config.SkipValidation {
		parser := jwt.NewParser(jwt.WithoutClaimsValidation())
		token, _, parseErr := parser.ParseUnverified(tokenString, mapClaims)
		if parseErr != nil {
			return fmt.Errorf("invalid JWT format: %v", parseErr)
		}
		var ok bool
		mapClaims, ok = token.Claims.(jwt.MapClaims)
		if !ok {
			return fmt.Errorf("invalid token claims")
		}
	} else {
		token, err := jwt.ParseWithClaims(tokenString, mapClaims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(config.SecretKey), nil
		})
		if err != nil {
			return fmt.Errorf("invalid token: %w", err)
		}
		var ok bool
		mapClaims, ok = token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			return fmt.Errorf("invalid token claims")
		}
		if config.TokenIssuer != "" {
			iss, _ := mapClaims["iss"].(string)
			if iss != config.TokenIssuer {
				return fmt.Errorf("invalid token issuer")
			}
		}
	}

	orgClaimName := config.OrganizationClaimName
	if orgClaimName == "" {
		orgClaimName = "organization"
	}
	org := getStringClaim(mapClaims, orgClaimName)
	if org == "" {
		return fmt.Errorf("token missing required '%s' claim", orgClaimName)
	}

	sub, _ := mapClaims["sub"].(string)
	claimsObj := &CustomClaims{
		Organization: org,
		Username:     getStringClaim(mapClaims, "username"),
		Email:        getStringClaim(mapClaims, "email"),
		Scope:        getStringClaim(mapClaims, "scope"),
		Audience:     audienceToString(mapClaims),
		JTI:          getStringClaim(mapClaims, "jti"),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: sub,
		},
	}

	c.Set("user_id", sub)
	c.Set("username", claimsObj.Username)
	c.Set("email", claimsObj.Email)
	c.Set("first_name", getStringClaim(mapClaims, "firstName"))
	c.Set("last_name", getStringClaim(mapClaims, "lastName"))
	c.Set("organization", org)
	c.Set("scope", claimsObj.Scope)
	c.Set("audience", claimsObj.Audience)
	c.Set("claims", claimsObj)
	// Local JWT tokens carry fine-grained scopes, not role names — platform_roles is empty
	// here because scope-based validation takes precedence in resolveEffectiveScopes.
	c.Set("platform_roles", []string{})

	return nil
}

// PlatformClaimsMiddleware extracts platform-specific values from the AuthContext set by
// common/authenticators.AuthMiddleware (IDP mode) and populates the per-key context entries
// that handlers rely on.
func PlatformClaimsMiddleware(claimNames PlatformClaimNames) gin.HandlerFunc {
	roleMapping := parseRoleMappings(claimNames.RoleMappings)

	return func(c *gin.Context) {
		authCtxVal, exists := c.Get(commonconstants.AuthContextKey)
		if !exists {
			c.Next()
			return
		}
		authCtx, ok := authCtxVal.(commonmodels.AuthContext)
		if !ok {
			c.Next()
			return
		}

		var mapClaims jwt.MapClaims
		switch v := authCtx.Claims.(type) {
		case jwt.MapClaims:
			mapClaims = v
		case map[string]interface{}:
			mapClaims = jwt.MapClaims(v)
		default:
			c.Next()
			return
		}

		org       := getStringClaim(mapClaims, claimNames.OrganizationClaim)
		orgName   := getStringClaim(mapClaims, claimNames.OrgNameClaim)
		orgHandle := getStringClaim(mapClaims, claimNames.OrgHandleClaim)

		userID := authCtx.UserID
		if claimNames.UserIDClaim != "" {
			if v := getStringClaim(mapClaims, claimNames.UserIDClaim); v != "" {
				userID = v
			}
		}

		username := getStringClaim(mapClaims, claimNames.UsernameClaim)
		email := getStringClaim(mapClaims, claimNames.EmailClaim)
		scope := getStringClaim(mapClaims, claimNames.ScopeClaim)
		aud := audienceToString(mapClaims)
		jti, _ := mapClaims["jti"].(string)

		sub, _ := mapClaims["sub"].(string)
		claimsObj := &CustomClaims{
			Organization: org,
			Username:     username,
			Email:        email,
			Scope:        scope,
			Audience:     aud,
			JTI:          jti,
			RegisteredClaims: jwt.RegisteredClaims{
				Subject: sub,
			},
		}

		platformRoles := resolvePlatformRoles(mapClaims, claimNames.RolesClaimPath, roleMapping)

		c.Set("user_id", userID)
		c.Set("username", username)
		c.Set("email", email)
		// Try OIDC-standard names first, fall back to WSO2/Asgardeo attribute names.
		firstName := getStringClaim(mapClaims, "given_name")
		if firstName == "" {
			firstName = getStringClaim(mapClaims, "firstName")
		}
		lastName := getStringClaim(mapClaims, "family_name")
		if lastName == "" {
			lastName = getStringClaim(mapClaims, "lastName")
		}
		c.Set("first_name", firstName)
		c.Set("last_name", lastName)
		c.Set("organization", org)
		c.Set("org_name", orgName)
		c.Set("org_handle", orgHandle)
		c.Set("scope", scope)
		c.Set("audience", aud)
		c.Set("claims", claimsObj)
		c.Set("platform_roles", platformRoles)

		c.Next()
	}
}

// resolvePlatformRoles extracts role values from the token at claimPath and maps them
// to platform roles using the provided mapping. Raw IDP values are used when mapping is empty.
func resolvePlatformRoles(claims jwt.MapClaims, claimPath string, mapping map[string]string) []string {
	if claimPath == "" {
		return nil
	}
	idpRoles := extractClaimByPath(claims, claimPath)
	if len(mapping) == 0 {
		return idpRoles
	}
	var platformRoles []string
	seen := make(map[string]struct{})
	for _, idpRole := range idpRoles {
		if platformRole, ok := mapping[idpRole]; ok {
			if _, dup := seen[platformRole]; !dup {
				platformRoles = append(platformRoles, platformRole)
				seen[platformRole] = struct{}{}
			}
		}
	}
	return platformRoles
}

// extractClaimByPath navigates a dot-notation path through jwt.MapClaims and returns the
// leaf value as a string slice. Supports both flat claims and nested paths (e.g. "realm_access.roles").
func extractClaimByPath(claims jwt.MapClaims, path string) []string {
	parts := strings.SplitN(path, ".", 2)
	val, ok := claims[parts[0]]
	if !ok {
		return nil
	}
	if len(parts) == 1 {
		return toStringSlice(val)
	}
	nested, ok := val.(map[string]interface{})
	if !ok {
		return nil
	}
	return extractNestedByPath(nested, parts[1])
}

func extractNestedByPath(obj map[string]interface{}, path string) []string {
	parts := strings.SplitN(path, ".", 2)
	val, ok := obj[parts[0]]
	if !ok {
		return nil
	}
	if len(parts) == 1 {
		return toStringSlice(val)
	}
	nested, ok := val.(map[string]interface{})
	if !ok {
		return nil
	}
	return extractNestedByPath(nested, parts[1])
}

func toStringSlice(val interface{}) []string {
	switch v := val.(type) {
	case string:
		return strings.Fields(v)
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

// parseRoleMappings converts "idp-value=platform-role" pairs into a lookup map.
func parseRoleMappings(mappings []string) map[string]string {
	m := make(map[string]string, len(mappings))
	for _, entry := range mappings {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return m
}

// getStringClaim safely extracts a string value from jwt.MapClaims.
func getStringClaim(claims jwt.MapClaims, name string) string {
	if name == "" {
		return ""
	}
	v, _ := claims[name].(string)
	return v
}

// audienceToString converts the aud claim (string or array) to a space-separated string.
func audienceToString(claims jwt.MapClaims) string {
	switch v := claims["aud"].(type) {
	case string:
		return v
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, a := range v {
			if s, ok := a.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " ")
	}
	return ""
}

// GetOrganizationFromContext extracts the organization claim from the Gin context.
func GetOrganizationFromContext(c *gin.Context) (string, bool) {
	organization, exists := c.Get("organization")
	if !exists {
		return "", false
	}
	orgStr, ok := organization.(string)
	return orgStr, ok
}

// GetOrgNameFromContext extracts the organization display name from the Gin context.
func GetOrgNameFromContext(c *gin.Context) (string, bool) {
	v, exists := c.Get("org_name")
	if !exists {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// GetOrgHandleFromContext extracts the organization handle from the Gin context.
func GetOrgHandleFromContext(c *gin.Context) (string, bool) {
	v, exists := c.Get("org_handle")
	if !exists {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// GetUserIDFromContext extracts the user ID from the Gin context.
func GetUserIDFromContext(c *gin.Context) (string, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", false
	}
	userIDStr, ok := userID.(string)
	return userIDStr, ok
}

// GetUsernameFromContext extracts the username from the Gin context.
func GetUsernameFromContext(c *gin.Context) (string, bool) {
	username, exists := c.Get("username")
	if !exists {
		return "", false
	}
	usernameStr, ok := username.(string)
	return usernameStr, ok
}

// GetEmailFromContext extracts the email from the Gin context.
func GetEmailFromContext(c *gin.Context) (string, bool) {
	email, exists := c.Get("email")
	if !exists {
		return "", false
	}
	emailStr, ok := email.(string)
	return emailStr, ok
}

// GetFirstNameFromContext extracts the first name from the Gin context.
func GetFirstNameFromContext(c *gin.Context) (string, bool) {
	firstName, exists := c.Get("first_name")
	if !exists {
		return "", false
	}
	firstNameStr, ok := firstName.(string)
	return firstNameStr, ok
}

// GetLastNameFromContext extracts the last name from the Gin context.
func GetLastNameFromContext(c *gin.Context) (string, bool) {
	lastName, exists := c.Get("last_name")
	if !exists {
		return "", false
	}
	lastNameStr, ok := lastName.(string)
	return lastNameStr, ok
}

// GetScopeFromContext extracts the scope from the Gin context.
func GetScopeFromContext(c *gin.Context) (string, bool) {
	scope, exists := c.Get("scope")
	if !exists {
		return "", false
	}
	scopeStr, ok := scope.(string)
	return scopeStr, ok
}

// GetAudienceFromContext extracts the audience from the Gin context.
func GetAudienceFromContext(c *gin.Context) (string, bool) {
	audience, exists := c.Get("audience")
	if !exists {
		return "", false
	}
	audienceStr, ok := audience.(string)
	return audienceStr, ok
}

// GetRolesFromContext parses the space-separated scope string into individual roles.
func GetRolesFromContext(c *gin.Context) ([]string, bool) {
	scope, exists := GetScopeFromContext(c)
	if !exists {
		return nil, false
	}
	if scope == "" {
		return []string{}, true
	}
	return strings.Fields(scope), true
}

// GetClaimsFromContext extracts the full CustomClaims object from the Gin context.
func GetClaimsFromContext(c *gin.Context) (*CustomClaims, bool) {
	claims, exists := c.Get("claims")
	if !exists {
		return nil, false
	}
	claimsObj, ok := claims.(*CustomClaims)
	return claimsObj, ok
}

// RequireOrganization returns a middleware that aborts with 403 if the token's organization
// does not match the organization ID in the URL parameter named by organizationParam.
func RequireOrganization(organizationParam string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenOrg, exists := GetOrganizationFromContext(c)
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "No organization found in token"})
			c.Abort()
			return
		}

		requestedOrg := c.Param(organizationParam)
		if requestedOrg == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Organization parameter is required"})
			c.Abort()
			return
		}

		if tokenOrg != requestedOrg {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for the requested organization"})
			c.Abort()
			return
		}

		c.Next()
	}
}
