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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/wso2/api-platform/common/authenticators"

	"github.com/golang-jwt/jwt/v5"
)

// contextKey is an unexported type for context keys in this package.
type contextKey string

const (
	keyUserID        contextKey = "user_id"
	keyUsername      contextKey = "username"
	keyEmail         contextKey = "email"
	keyFirstName     contextKey = "first_name"
	keyLastName      contextKey = "last_name"
	keyOrganization  contextKey = "organization"
	keyOrgName       contextKey = "org_name"
	keyOrgHandle     contextKey = "org_handle"
	keyIdpOrgRef     contextKey = "idp_org_ref"
	keyScope         contextKey = "scope"
	keyAudience      contextKey = "audience"
	keyClaims        contextKey = "claims"
	keyPlatformRoles contextKey = "platform_roles"
)

// CustomClaims represents the JWT claims structure used in local JWT (non-IDP) mode.
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

// AuthConfig holds the configuration for the local JWT (non-IDP) authentication path.
type AuthConfig struct {
	SecretKey             string
	TokenIssuer           string
	SkipPaths             []string
	SkipValidation        bool
	OrganizationClaimName string
}

// PlatformClaimNames holds the JWT claim names used to extract platform-specific values.
type PlatformClaimNames struct {
	OrganizationClaim string
	OrgNameClaim      string
	OrgHandleClaim    string
	UserIDClaim       string
	UsernameClaim     string
	EmailClaim        string
	ScopeClaim        string
	RolesClaimPath    string
	RoleScopeMap      map[string][]string
}

// writeJSONError is a helper to write a JSON error body without depending on httputil.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// LocalJWTAuthMiddleware returns a middleware for locally-issued JWT validation.
// Used only when IDP mode is disabled.
func LocalJWTAuthMiddleware(config AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, path := range config.SkipPaths {
				if strings.HasPrefix(r.URL.Path, path) {
					next.ServeHTTP(w, r)
					return
				}
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSONError(w, http.StatusUnauthorized, "Authorization header is required")
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				writeJSONError(w, http.StatusUnauthorized, "Invalid authorization header format. Expected: Bearer <token>")
				return
			}

			enriched, err := validateLocalJWT(r, tokenString, config)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, err.Error())
				return
			}

			next.ServeHTTP(w, enriched)
		})
	}
}

// validateLocalJWT handles locally-issued JWT validation (non-IDP mode).
// Returns the request enriched with identity context values on success.
func validateLocalJWT(r *http.Request, tokenString string, config AuthConfig) (*http.Request, error) {
	mapClaims := jwt.MapClaims{}

	if config.SkipValidation {
		parser := jwt.NewParser(jwt.WithoutClaimsValidation())
		token, _, parseErr := parser.ParseUnverified(tokenString, mapClaims)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid JWT format: %v", parseErr)
		}
		var ok bool
		mapClaims, ok = token.Claims.(jwt.MapClaims)
		if !ok {
			return nil, fmt.Errorf("invalid token claims")
		}
	} else {
		token, err := jwt.ParseWithClaims(tokenString, mapClaims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(config.SecretKey), nil
		})
		if err != nil {
			return nil, fmt.Errorf("invalid token: %w", err)
		}
		var ok bool
		mapClaims, ok = token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			return nil, fmt.Errorf("invalid token claims")
		}
		if config.TokenIssuer != "" {
			iss, _ := mapClaims["iss"].(string)
			if iss != config.TokenIssuer {
				return nil, fmt.Errorf("invalid token issuer")
			}
		}
	}

	orgClaimName := config.OrganizationClaimName
	if orgClaimName == "" {
		orgClaimName = "organization"
	}
	org := getStringClaim(mapClaims, orgClaimName)
	if org == "" {
		return nil, fmt.Errorf("token missing required '%s' claim", orgClaimName)
	}

	sub, _ := mapClaims["sub"].(string)
	username := getStringClaim(mapClaims, "username")
	if username == "" {
		username = sub
	}
	claimsObj := &CustomClaims{
		Organization: org,
		Username:     username,
		Email:        getStringClaim(mapClaims, "email"),
		Scope:        getStringClaim(mapClaims, "scope"),
		Audience:     audienceToString(mapClaims),
		JTI:          getStringClaim(mapClaims, "jti"),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: sub,
		},
	}

	ctx := r.Context()
	ctx = context.WithValue(ctx, keyUserID, resolveUserID(mapClaims, ""))
	ctx = context.WithValue(ctx, keyUsername, claimsObj.Username)
	ctx = context.WithValue(ctx, keyEmail, claimsObj.Email)
	ctx = context.WithValue(ctx, keyFirstName, getStringClaim(mapClaims, "firstName"))
	ctx = context.WithValue(ctx, keyLastName, getStringClaim(mapClaims, "lastName"))
	ctx = context.WithValue(ctx, keyOrganization, org)
	ctx = context.WithValue(ctx, keyScope, claimsObj.Scope)
	ctx = context.WithValue(ctx, keyAudience, claimsObj.Audience)
	ctx = context.WithValue(ctx, keyClaims, claimsObj)
	ctx = context.WithValue(ctx, keyPlatformRoles, []string{})
	return r.WithContext(ctx), nil
}

// PlatformClaimsMiddleware extracts platform-specific values from the AuthContext set by
// common/authenticators.AuthMiddleware (IDP mode) and populates per-key context entries.
func PlatformClaimsMiddleware(claimNames PlatformClaimNames) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCtx, ok := authenticators.GetAuthContext(r)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			var mapClaims jwt.MapClaims
			switch v := authCtx.Claims.(type) {
			case jwt.MapClaims:
				mapClaims = v
			case map[string]interface{}:
				mapClaims = jwt.MapClaims(v)
			default:
				next.ServeHTTP(w, r)
				return
			}

			org := getStringClaim(mapClaims, claimNames.OrganizationClaim)
			orgName := getStringClaim(mapClaims, claimNames.OrgNameClaim)
			orgHandle := getStringClaim(mapClaims, claimNames.OrgHandleClaim)

			userID := resolveUserID(mapClaims, claimNames.UserIDClaim)
			if userID == "" {
				userID = authCtx.UserID
			}

			username := getStringClaim(mapClaims, claimNames.UsernameClaim)
			email := getStringClaim(mapClaims, claimNames.EmailClaim)
			scope := getStringClaim(mapClaims, claimNames.ScopeClaim)
			aud := audienceToString(mapClaims)
			jti, _ := mapClaims["jti"].(string)

			sub, _ := mapClaims["sub"].(string)
			claimsObj := &CustomClaims{
				Organization:     org,
				Username:         username,
				Email:            email,
				Scope:            scope,
				Audience:         aud,
				JTI:              jti,
				RegisteredClaims: jwt.RegisteredClaims{Subject: sub},
			}

			firstName := getStringClaim(mapClaims, "given_name")
			if firstName == "" {
				firstName = getStringClaim(mapClaims, "firstName")
			}
			lastName := getStringClaim(mapClaims, "family_name")
			if lastName == "" {
				lastName = getStringClaim(mapClaims, "lastName")
			}

			platformRoles := resolvePlatformRoles(mapClaims, claimNames.RolesClaimPath, claimNames.RoleScopeMap)

			ctx := r.Context()
			ctx = context.WithValue(ctx, keyUserID, userID)
			ctx = context.WithValue(ctx, keyUsername, username)
			ctx = context.WithValue(ctx, keyEmail, email)
			ctx = context.WithValue(ctx, keyFirstName, firstName)
			ctx = context.WithValue(ctx, keyLastName, lastName)
			ctx = context.WithValue(ctx, keyOrganization, org)
			ctx = context.WithValue(ctx, keyOrgName, orgName)
			ctx = context.WithValue(ctx, keyOrgHandle, orgHandle)
			ctx = context.WithValue(ctx, keyScope, scope)
			ctx = context.WithValue(ctx, keyAudience, aud)
			ctx = context.WithValue(ctx, keyClaims, claimsObj)
			ctx = context.WithValue(ctx, keyPlatformRoles, platformRoles)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// resolvePlatformRoles extracts IDP role names from the token and expands them.
func resolvePlatformRoles(claims jwt.MapClaims, claimPath string, roleScopeMap map[string][]string) []string {
	if claimPath == "" {
		return nil
	}
	idpRoles := extractClaimByPath(claims, claimPath)
	if roleScopeMap == nil {
		return idpRoles
	}
	seen := make(map[string]struct{})
	var effectiveScopes []string
	for _, idpRole := range idpRoles {
		for _, scope := range roleScopeMap[idpRole] {
			if _, dup := seen[scope]; !dup {
				effectiveScopes = append(effectiveScopes, scope)
				seen[scope] = struct{}{}
			}
		}
	}
	return effectiveScopes
}

func extractClaimByPath(claims jwt.MapClaims, path string) []string {
	return extractByPath(map[string]interface{}(claims), path)
}

func extractByPath(obj map[string]interface{}, path string) []string {
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
	return extractByPath(nested, parts[1])
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

func getStringClaim(claims jwt.MapClaims, name string) string {
	if name == "" {
		return ""
	}
	v, _ := claims[name].(string)
	return v
}

// resolveUserID returns the stable user identifier used for audit fields
// (createdBy/updatedBy/etc.). It prefers an explicitly configured claim name,
// then the conventional "user_id" claim, and finally falls back to "sub".
// Returns an empty string only when none of these are present.
func resolveUserID(claims jwt.MapClaims, configuredClaim string) string {
	if v := getStringClaim(claims, configuredClaim); v != "" {
		return v
	}
	if v := getStringClaim(claims, "user_id"); v != "" {
		return v
	}
	sub, _ := claims["sub"].(string)
	return sub
}

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

// --- Context accessor helpers ---

func getStringFromCtx(r *http.Request, key contextKey) (string, bool) {
	v, _ := r.Context().Value(key).(string)
	return v, v != ""
}

// GetOrganizationFromRequest extracts the organization claim from the request context.
func GetOrganizationFromRequest(r *http.Request) (string, bool) {
	return getStringFromCtx(r, keyOrganization)
}

// GetOrgNameFromRequest extracts the organization display name from the request context.
func GetOrgNameFromRequest(r *http.Request) (string, bool) {
	return getStringFromCtx(r, keyOrgName)
}

// GetOrgHandleFromRequest extracts the organization handle from the request context.
func GetOrgHandleFromRequest(r *http.Request) (string, bool) {
	return getStringFromCtx(r, keyOrgHandle)
}

// GetIdpOrgRefFromRequest extracts the raw organization claim carried by the
// token (the IDP's organization id in IDP mode). Unlike the organization key —
// which OrganizationResolverMiddleware rewrites to the platform UUID — this
// always reflects the value the token asserted. Returns false when unset.
func GetIdpOrgRefFromRequest(r *http.Request) (string, bool) {
	return getStringFromCtx(r, keyIdpOrgRef)
}

// OrgUUIDResolver maps a token's organization claim to the platform
// organization UUID, returning true when a matching organization exists.
type OrgUUIDResolver func(orgClaim string) (string, bool)

// OrganizationResolverMiddleware resolves the organization claim populated by the
// authentication middleware into the platform organization UUID, and stores that
// UUID back under the organization context key. Downstream handlers therefore
// scope their queries by the correct UUID regardless of whether the token carries
// the platform UUID (file-based auth) or the IDP's organization id (IDP auth).
//
// The raw claim is preserved under a separate key (see GetIdpOrgRefFromRequest)
// for callers that need the original IDP reference — notably organization
// registration, where no organization exists yet to resolve against.
func OrganizationResolverMiddleware(resolve OrgUUIDResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claim, ok := GetOrganizationFromRequest(r)
			if !ok || resolve == nil {
				next.ServeHTTP(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), keyIdpOrgRef, claim)
			if uuid, found := resolve(claim); found {
				ctx = context.WithValue(ctx, keyOrganization, uuid)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserIDFromRequest extracts the user ID from the request context.
func GetUserIDFromRequest(r *http.Request) (string, bool) {
	return getStringFromCtx(r, keyUserID)
}

// GetUsernameFromRequest extracts the username from the request context.
func GetUsernameFromRequest(r *http.Request) (string, bool) {
	return getStringFromCtx(r, keyUsername)
}

// GetEmailFromRequest extracts the email from the request context.
func GetEmailFromRequest(r *http.Request) (string, bool) {
	return getStringFromCtx(r, keyEmail)
}

// GetFirstNameFromRequest extracts the first name from the request context.
func GetFirstNameFromRequest(r *http.Request) (string, bool) {
	return getStringFromCtx(r, keyFirstName)
}

// GetLastNameFromRequest extracts the last name from the request context.
func GetLastNameFromRequest(r *http.Request) (string, bool) {
	return getStringFromCtx(r, keyLastName)
}

// GetScopeFromRequest extracts the scope from the request context.
func GetScopeFromRequest(r *http.Request) (string, bool) {
	return getStringFromCtx(r, keyScope)
}

// GetAudienceFromRequest extracts the audience from the request context.
func GetAudienceFromRequest(r *http.Request) (string, bool) {
	return getStringFromCtx(r, keyAudience)
}

// GetRolesFromRequest parses the space-separated scope string into individual roles.
func GetRolesFromRequest(r *http.Request) ([]string, bool) {
	scope, ok := GetScopeFromRequest(r)
	if !ok {
		return nil, false
	}
	if scope == "" {
		return []string{}, true
	}
	return strings.Fields(scope), true
}

// GetClaimsFromRequest extracts the full CustomClaims object from the request context.
func GetClaimsFromRequest(r *http.Request) (*CustomClaims, bool) {
	claims, ok := r.Context().Value(keyClaims).(*CustomClaims)
	return claims, ok
}

// GetPlatformRolesFromRequest extracts platform roles from the request context.
func GetPlatformRolesFromRequest(r *http.Request) ([]string, bool) {
	roles, ok := r.Context().Value(keyPlatformRoles).([]string)
	return roles, ok
}

// RequireOrganization returns a middleware that aborts with 403 if the token's
// organization does not match the organization ID in the URL path value.
func RequireOrganization(organizationParam string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenOrg, exists := GetOrganizationFromRequest(r)
			if !exists {
				writeJSONError(w, http.StatusForbidden, "No organization found in token")
				return
			}

			requestedOrg := r.PathValue(organizationParam)
			if requestedOrg == "" {
				writeJSONError(w, http.StatusBadRequest, "Organization parameter is required")
				return
			}

			if tokenOrg != requestedOrg {
				writeJSONError(w, http.StatusForbidden, "Access denied for the requested organization")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// NewTestContextMiddleware creates an http.Handler middleware for integration tests.
// It reads X-Test-Org and X-Test-User request headers and injects the values into
// the request context so that GetOrganizationFromRequest / GetUsernameFromRequest work
// without a real JWT. Never use this in production code.
func NewTestContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if org := r.Header.Get("X-Test-Org"); org != "" {
			ctx = context.WithValue(ctx, keyOrganization, org)
		}
		if user := r.Header.Get("X-Test-User"); user != "" {
			ctx = context.WithValue(ctx, keyUsername, user)
			ctx = context.WithValue(ctx, keyUserID, user)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// WithOrganization is a helper for tests to inject an organization into the request context.
func WithOrganization(r *http.Request, org string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), keyOrganization, org))
}

// WithUserID is a helper for tests to inject a user ID into the request context.
func WithUserID(r *http.Request, id string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), keyUserID, id))
}

// --- Compatibility shims for common/authenticators ---
