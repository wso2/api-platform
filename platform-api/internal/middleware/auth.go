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
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2/api-platform/common/authenticators"
	"github.com/wso2/api-platform/platform-api/internal/apperror"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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
	SecretKey      string
	TokenIssuer    string
	SkipPaths      []string
	SkipValidation bool
	// ClaimMappings is the same claim-name mapping used by IDP mode
	// (PlatformClaimsMiddleware) and by the file-mode login endpoint when it
	// signs tokens — one mapping shared by issuance and validation.
	ClaimMappings ClaimMappings
}

// ClaimMappings holds the JWT claim names used to extract identity values,
// shared by the local-JWT (external_token/file) and IDP auth paths.
type ClaimMappings struct {
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

// writeAuthError writes the unified 401 response. The auth middleware runs
// ahead of routing so it can't return an error through the MapErrors chain;
// instead it logs the specific failure reason internally and serializes
// through the same apperror.WriteHTTP the mapper uses, so both the log shape
// and the wire format have a single owner. Every authentication failure —
// missing header, malformed header, invalid or expired token — produces the
// identical payload (apperror.Unauthorized) per the unified-auth rule in
// error-handling.md; reason is internal-only and must never contain a raw
// token.
func writeAuthError(w http.ResponseWriter, reason string) {
	writeError(w, apperror.Unauthorized.New(), reason)
}

// writeError logs a pre-routing failure (all 4xx — WARN, no stack, per the
// severity split in error_mapper.go) and serializes it through the shared
// apperror.WriteHTTP. reason is internal-only and must never contain a raw
// token.
func writeError(w http.ResponseWriter, appErr *apperror.Error, reason string) {
	slog.Warn("request failed",
		"trackingId", uuid.NewString(),
		"code", appErr.Code,
		"status", appErr.HTTPStatus,
		"reason", reason)
	apperror.WriteHTTP(w, appErr, "")
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
				writeAuthError(w, "authorization header missing")
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				writeAuthError(w, "authorization header is not a Bearer token")
				return
			}

			enriched, err := validateLocalJWT(r, tokenString, config)
			if err != nil {
				writeAuthError(w, "local JWT validation failed: "+err.Error())
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

	orgClaimName := config.ClaimMappings.OrganizationClaim
	if orgClaimName == "" {
		orgClaimName = "organization"
	}
	org := getStringClaim(mapClaims, orgClaimName)
	if org == "" {
		return nil, fmt.Errorf("token missing required '%s' claim", orgClaimName)
	}
	orgName := getStringClaim(mapClaims, config.ClaimMappings.OrgNameClaim)
	orgHandle := getStringClaim(mapClaims, config.ClaimMappings.OrgHandleClaim)

	sub, _ := mapClaims["sub"].(string)
	username := getStringClaim(mapClaims, config.ClaimMappings.UsernameClaim)
	if username == "" {
		username = sub
	}
	claimsObj := &CustomClaims{
		Organization: org,
		Username:     username,
		Email:        getStringClaim(mapClaims, config.ClaimMappings.EmailClaim),
		Scope:        getStringClaim(mapClaims, config.ClaimMappings.ScopeClaim),
		Audience:     audienceToString(mapClaims),
		JTI:          getStringClaim(mapClaims, "jti"),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: sub,
		},
	}

	platformRoles := resolvePlatformRoles(mapClaims, config.ClaimMappings.RolesClaimPath, config.ClaimMappings.RoleScopeMap)

	ctx := r.Context()
	ctx = context.WithValue(ctx, keyUserID, resolveUserID(mapClaims, config.ClaimMappings.UserIDClaim))
	ctx = context.WithValue(ctx, keyUsername, claimsObj.Username)
	ctx = context.WithValue(ctx, keyEmail, claimsObj.Email)
	ctx = context.WithValue(ctx, keyFirstName, getStringClaim(mapClaims, "firstName"))
	ctx = context.WithValue(ctx, keyLastName, getStringClaim(mapClaims, "lastName"))
	ctx = context.WithValue(ctx, keyOrganization, org)
	ctx = context.WithValue(ctx, keyOrgName, orgName)
	ctx = context.WithValue(ctx, keyOrgHandle, orgHandle)
	ctx = context.WithValue(ctx, keyScope, claimsObj.Scope)
	ctx = context.WithValue(ctx, keyAudience, claimsObj.Audience)
	ctx = context.WithValue(ctx, keyClaims, claimsObj)
	ctx = context.WithValue(ctx, keyPlatformRoles, platformRoles)
	return r.WithContext(ctx), nil
}

// PlatformClaimsMiddleware extracts platform-specific values from the AuthContext set by
// common/authenticators.AuthMiddleware (IDP mode) and populates per-key context entries.
func PlatformClaimsMiddleware(claimNames ClaimMappings) func(http.Handler) http.Handler {
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
	val, ok := resolveClaimPath(map[string]interface{}(claims), path)
	if !ok {
		return nil
	}
	return toStringSlice(val)
}

// resolveClaimPath walks a dot-separated path into nested claim objects and
// returns the raw value found there. A path with no "." is a single flat
// claim lookup, so every claim_mappings field — not just roles — can point at
// either a top-level claim ("org_id") or a nested one ("realm_access.org_id").
func resolveClaimPath(obj map[string]interface{}, path string) (interface{}, bool) {
	if path == "" {
		return nil, false
	}
	parts := strings.SplitN(path, ".", 2)
	val, ok := obj[parts[0]]
	if !ok {
		return nil, false
	}
	if len(parts) == 1 {
		return val, true
	}
	nested, ok := val.(map[string]interface{})
	if !ok {
		return nil, false
	}
	return resolveClaimPath(nested, parts[1])
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

// getStringClaim resolves name as a claim path (see resolveClaimPath) and
// returns the value as a string. name may be a flat claim ("email") or a
// dot-separated path into a nested claim ("realm_access.email").
func getStringClaim(claims jwt.MapClaims, name string) string {
	if name == "" {
		return ""
	}
	val, ok := resolveClaimPath(map[string]interface{}(claims), name)
	if !ok {
		return ""
	}
	s, _ := val.(string)
	return s
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

// OrgUUIDResolver maps a token's organization claim to the platform
// organization UUID, returning true when a matching organization exists.
type OrgUUIDResolver func(orgClaim string) (string, bool)

// OrganizationResolverMiddleware resolves the organization claim populated by the
// authentication middleware into the platform organization UUID, and stores that
// UUID back under the organization context key. Downstream handlers therefore
// scope their queries by the correct UUID regardless of whether the token carries
// the platform UUID (file-based auth) or the IDP's organization id (IDP auth).
//
// When the claim does not resolve to an existing organization (e.g. during
// registration of a brand-new organization), the context is left holding the
// raw claim, so GetOrganizationFromRequest still returns the IDP's original
// organization reference in that case.
func OrganizationResolverMiddleware(resolve OrgUUIDResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claim, ok := GetOrganizationFromRequest(r)
			if !ok || resolve == nil {
				next.ServeHTTP(w, r)
				return
			}
			ctx := r.Context()
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

// GetSubClaimFromRequest extracts the token's OIDC "sub" (subject) claim from
// the request context. This is the raw IdP subject identifier — distinct from
// GetUserIDFromRequest, which returns the configured-claim/user_id/sub
// precedence value used historically for audit columns. Prefer this for keying
// the internal-UUID mapping (see service.IdentityService).
func GetSubClaimFromRequest(r *http.Request) (string, bool) {
	claims, ok := GetClaimsFromRequest(r)
	if !ok || claims == nil || claims.Subject == "" {
		return "", false
	}
	return claims.Subject, true
}

// GetActorIdentityFromRequest resolves the raw identity-provider identifier for
// the actor behind r, preferring the token's "sub" claim and falling back to
// GetUserIDFromRequest (configured-claim/user_id/sub) when sub is unavailable
// (e.g. non-OIDC IdPs, or test/internal callers that only set the legacy
// context key). ok is false only when neither source has a value — callers
// that must reject unauthenticated requests should treat that as 401.
// This mirrors the precedence used by service.IdentityService.InternalUserID;
// use it at call sites that need the raw id before mapping (e.g. to
// distinguish "claim missing" from "mapping failed").
func GetActorIdentityFromRequest(r *http.Request) (string, bool) {
	if sub, ok := GetSubClaimFromRequest(r); ok {
		return sub, true
	}
	return GetUserIDFromRequest(r)
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
				writeError(w, apperror.Forbidden.New(), "no organization claim in token")
				return
			}

			requestedOrg := r.PathValue(organizationParam)
			if requestedOrg == "" {
				writeError(w, apperror.ValidationFailed.New("Organization parameter is required"), "missing organization path parameter")
				return
			}

			if tokenOrg != requestedOrg {
				writeError(w, apperror.Forbidden.New(), "token organization does not match requested organization")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// NewTestContextMiddleware creates an http.Handler middleware for integration tests.
// It reads X-Test-Org, X-Test-User, and X-Test-Scope request headers and injects the
// values into the request context so that GetOrganizationFromRequest /
// GetUsernameFromRequest / GetScopeFromRequest work without a real JWT. Never use this
// in production code.
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
		if scope := r.Header.Get("X-Test-Scope"); scope != "" {
			ctx = context.WithValue(ctx, keyScope, scope)
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
