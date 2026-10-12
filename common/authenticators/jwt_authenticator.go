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
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/common/models"
)

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token has expired")
	ErrInvalidSignature = errors.New("invalid token signature")
)

const (
	// jwksHTTPTimeout bounds a single JWKS document fetch (connect + TLS + response).
	// Set explicitly rather than left at jwkset's own implicit 1-minute default, so the
	// deadline is visible here rather than relying on a library default.
	jwksHTTPTimeout = 15 * time.Second

	// jwksMaxResponseBytes bounds a JWKS document fetch. jwkset decodes the response body
	// directly (json.NewDecoder) with no size limit of its own, and the caller-supplied
	// HTTPClient's own body cap may be disabled (some callers deliberately turn it off so
	// per-call-site limits apply instead — see platform-api's NewUpstreamFetchClient) — so
	// this package enforces its own bound regardless of what the caller's client does.
	// JWKS documents are small (a handful of KB per key), so this is generous headroom.
	jwksMaxResponseBytes int64 = 1 << 20 // 1 MiB
)

// maxBytesRoundTripper wraps an http.RoundTripper so the response body a caller reads is
// capped at maxBytes, regardless of the underlying client's own configuration. Used to bound
// the JWKS fetch performed internally by jwkset.NewStorageFromHTTP, which has no size-limit
// option of its own.
type maxBytesRoundTripper struct {
	base     http.RoundTripper
	maxBytes int64
}

func (t *maxBytesRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}
	resp.Body = struct {
		io.Reader
		io.Closer
	}{io.LimitReader(resp.Body, t.maxBytes), resp.Body}
	return resp, nil
}

// newBoundedJWKSHTTPClient returns a shallow copy of client with its response body capped at
// jwksMaxResponseBytes and an explicit per-request timeout, reusing the original client's
// Transport (and therefore its SSRF dial-time guard and redirect re-validation) unchanged.
func newBoundedJWKSHTTPClient(client *http.Client) *http.Client {
	return &http.Client{
		Transport:     &maxBytesRoundTripper{base: client.Transport, maxBytes: jwksMaxResponseBytes},
		CheckRedirect: client.CheckRedirect,
		Jar:           client.Jar,
		Timeout:       jwksHTTPTimeout,
	}
}

// JWTAuthenticator implements JWT authentication
type JWTAuthenticator struct {
	config *models.AuthConfig
	logger *slog.Logger
	jwks   keyfunc.Keyfunc
}

// NewJWTAuthenticator creates a new JWT authenticator
func NewJWTAuthenticator(config *models.AuthConfig, logger *slog.Logger) (*JWTAuthenticator, error) {
	return newJWTAuthenticatorWithJWKS(config, logger, true)
}

// newJWTAuthenticatorWithJWKS creates a new JWT authenticator with optional JWKS initialization
// This is useful for testing where JWKS is not needed
func newJWTAuthenticatorWithJWKS(config *models.AuthConfig, logger *slog.Logger, initJWKS bool) (*JWTAuthenticator, error) {
	if config.JWTConfig != nil && config.JWTConfig.DisableAuthorization {
		logger.Warn("authorization is explicitly disabled for IDP auth (disable_authorization=true); " +
			"every authenticated token is granted access to all routes without a per-route role check")
	}
	var jwks keyfunc.Keyfunc
	if config.JWTConfig != nil && initJWKS {
		if config.JWTConfig.IssuerURL == "" {
			return nil, errors.New("issuer URL not configured")
		}
		// Get Issuer URL from config
		if config.JWTConfig.JWKSUrl == "" {
			return nil, errors.New("JWKS endpoint not configured")
		}
		// config.HTTPClient must be the caller's SSRF-guarded shared client (see
		// ssrf-prevention.md) — jwkset.NewStorageFromHTTP falls back to
		// http.DefaultClient when Client is nil, which would silently fetch the
		// JWKS endpoint with no SSRF protection.
		if config.HTTPClient == nil {
			return nil, errors.New("HTTP client not configured for JWKS fetching")
		}

		// Create JWKS storage with custom validation options to skip X5TS256 validation
		// This is required for some OIDC providers like Asgardeo that may have X5TS256 mismatches
		ctx := context.Background()
		storageOptions := jwkset.HTTPClientStorageOptions{
			Client:          newBoundedJWKSHTTPClient(config.HTTPClient),
			Ctx:             ctx,
			HTTPTimeout:     jwksHTTPTimeout, // explicit, not jwkset's implicit 1-minute default
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
func (j *JWTAuthenticator) Authenticate(r *http.Request) (*AuthResult, error) {
	// Extract bearer token from Authorization header
	authHeader := r.Header.Get(constants.AuthorizationHeader)
	if authHeader == "" {
		return nil, errors.New("authorization header missing")
	}

	// Remove "Bearer " prefix
	tokenString := strings.TrimPrefix(authHeader, constants.BearerPrefix)
	if tokenString == authHeader {
		return nil, errors.New("invalid authorization header format")
	}

	claims := jwt.MapClaims{}
	leeway := 60 * time.Second
	if j.config.JWTConfig.JWTLeeway != nil {
		leeway = *j.config.JWTConfig.JWTLeeway
	}
	validatedToken, err := jwt.ParseWithClaims(tokenString, claims, j.jwks.Keyfunc, jwt.WithLeeway(leeway))

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		if errors.Is(err, jwt.ErrTokenNotValidYet) {
			return nil, fmt.Errorf("%w: token not yet valid (clock skew?)", ErrInvalidToken)
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

	// Validate audience if configured. The token is accepted when its "aud"
	// claim contains at least one of the configured audiences.
	if j.config.JWTConfig.Audience != nil && len(*j.config.JWTConfig.Audience) > 0 {
		audience, err := claims.GetAudience()
		if err != nil {
			return nil, fmt.Errorf("failed to get audience: %w", err)
		}
		matched := false
		for _, expected := range *j.config.JWTConfig.Audience {
			if expected != "" && slices.Contains(audience, expected) {
				matched = true
				break
			}
		}
		if !matched {
			return nil, errors.New("invalid audience")
		}
	}

	// Authorization is skipped ONLY when it has been explicitly disabled via DisableAuthorization (authentication-only)
	var permissions []string
	skipAuthz := j.config.JWTConfig.DisableAuthorization
	if j.config.JWTConfig.ScopeClaim == "" {
		j.logger.Debug("no scope/roles claim configured; caller resolves to zero roles")
		permissions = []string{}
	} else {
		permissions = j.resolvePermissions(claims)
	}
	subject, err := claims.GetSubject()
	if err != nil {
		return nil, fmt.Errorf("failed to get subject: %w", err)
	}
	return &AuthResult{
		Success:           true,
		UserID:            subject,
		Roles:             permissions,
		Claims:            claims,
		SkipAuthorization: skipAuthz,
	}, nil
}

// Its outbound inverse is MapRolesToScopes (role_mapping.go): every scope this gateway advertises is built there.
// If you change how the mapping is interpreted here that function must follow, or clients are told to request scopes that resolve to nothing.
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
	j.logger.Debug("permissions", slog.Any("permissions", permissions))
	j.logger.Debug("permission mapping", slog.Any("permissionMapping", j.config.JWTConfig.PermissionMapping))
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
		j.logger.Debug("wildcard role", slog.String("wildcardRole", wildcardRole))

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
						j.logger.Debug("mapped claim value to role", slog.String("claimValue", perm), slog.String("role", role))
						mappedPermissions = append(mappedPermissions, role)
						mapped = true
					}
				}
			}

			// If no specific mapping found and wildcard role exists, use wildcard
			if !mapped && wildcardRole != "" {
				j.logger.Debug("mapped claim value to wildcard role", slog.String("claimValue", perm), slog.String("wildcardRole", wildcardRole))
				mappedPermissions = append(mappedPermissions, wildcardRole)
			} else if !mapped {
				// No mapping and no wildcard, keep original
				j.logger.Debug("unmapped claim value", slog.String("claimValue", perm))
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
func (j *JWTAuthenticator) CanHandle(r *http.Request) bool {
	authHeader := r.Header.Get(constants.AuthorizationHeader)
	if authHeader == "" {
		return false
	}
	canHandle := strings.HasPrefix(authHeader, constants.BearerPrefix)
	j.logger.Debug("can handle token", slog.Bool("canHandle", canHandle))
	return canHandle
}
