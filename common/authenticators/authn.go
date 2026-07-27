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
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/common/models"
	"github.com/wso2/go-httpkit/httputil"
)

var (
	ErrNoAuthenticator = errors.New("no suitable authenticator found")
)

// unexported context key types prevent collisions across packages.
type authContextKeyType struct{}
type authzSkipKeyType struct{}

// GetAuthContext retrieves the AuthContext stored by AuthMiddleware.
func GetAuthContext(r *http.Request) (models.AuthContext, bool) {
	ac, ok := r.Context().Value(authContextKeyType{}).(models.AuthContext)
	return ac, ok
}

// WithAuthContext returns a new context with the provided AuthContext stored
// under the same key used by AuthMiddleware. This is intended for use in tests
// and other contexts where the auth middleware has not run.
func WithAuthContext(ctx context.Context, authCtx models.AuthContext) context.Context {
	return context.WithValue(ctx, authContextKeyType{}, authCtx)
}

// GetAuthzSkip returns true when the authz middleware should skip role checks.
func GetAuthzSkip(r *http.Request) bool {
	skip, _ := r.Context().Value(authzSkipKeyType{}).(bool)
	return skip
}

// AuthMiddleware creates a unified authentication middleware supporting both Basic and Bearer auth.
// Initialize authenticators once at startup (middleware creation time).
// Any configuration errors (e.g., JWT JWKS init failures) should fail fast here
// rather than per-request.
func AuthMiddleware(config models.AuthConfig, logger *slog.Logger) (func(http.Handler) http.Handler, error) {
	var authenticators []Authenticator

	if config.BasicAuth != nil && config.BasicAuth.Enabled && len(config.BasicAuth.Users) > 0 {
		authenticators = append(authenticators, NewBasicAuthenticator(config, logger))
	}

	if config.JWTConfig != nil && config.JWTConfig.Enabled {
		jwtAuthenticator, err := NewJWTAuthenticator(&config, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize JWT authenticator: %w", err)
		}
		authenticators = append(authenticators, jwtAuthenticator)
	}

	// No authenticators configured => run in no-auth mode.
	// This disables both authentication and authorization (via authzSkipKey).
	if len(authenticators) == 0 {
		logger.Warn("no authentication method is configured — running with authentication and authorization DISABLED; " +
			"every request is treated as an authenticated admin. Enable basic auth or an IDP to secure this service.")
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authCtx := models.AuthContext{
					Authenticated: true,
					UserID:        "sys_noauth_user",
					Roles:         []string{},
					Claims:        map[string]any{},
				}
				ctx := context.WithValue(r.Context(), authContextKeyType{}, authCtx)
				ctx = context.WithValue(ctx, authzSkipKeyType{}, true)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}, nil
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication for specified paths
			for _, path := range config.SkipPaths {
				if strings.HasPrefix(r.URL.Path, path) {
					ctx := context.WithValue(r.Context(), authzSkipKeyType{}, true)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// Find suitable authenticator
			var selectedAuth Authenticator
			for _, auth := range authenticators {
				if auth.CanHandle(r) {
					selectedAuth = auth
					break
				}
			}

			if selectedAuth == nil {
				httputil.WriteError(w, http.StatusUnauthorized, constants.AuthzSkipKey, "no valid authentication credentials provided")
				return
			}

			result, err := selectedAuth.Authenticate(r)
			if err != nil {
				logger.Info("authentication failed",
					slog.String("authenticator", selectedAuth.Name()),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("client_ip", r.RemoteAddr),
				)
				logger.Debug("authentication failure detail",
					slog.String("authenticator", selectedAuth.Name()),
					slog.Any("error", err),
				)
				httputil.WriteError(w, http.StatusUnauthorized, "authentication_failed", "authentication failed")
				return
			}
			logger.Debug("Authentication result", slog.Any("result", result))
			logger.Debug("Authentication roles", slog.Any("roles", result.Roles))

			claims := result.Claims
			if claims == nil {
				claims = map[string]any{}
			}
			authCtx := models.AuthContext{
				Authenticated: result.Success,
				UserID:        result.UserID,
				Roles:         result.Roles,
				Claims:        claims,
			}
			ctx := context.WithValue(r.Context(), authContextKeyType{}, authCtx)
			if result.SkipAuthorization {
				ctx = context.WithValue(ctx, authzSkipKeyType{}, true)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}
