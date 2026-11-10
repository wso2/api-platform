/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package mcpserver

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/wso2/api-platform/common/authenticators"
	"github.com/wso2/api-platform/httpkit/httputil"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
)

// caller is who is calling a tool.
type caller struct {
	org    string // the Platform API's organization UUID
	user   string // the Platform API's internal user ID
	scopes map[string]bool
}

type callerKey struct{}

func callerFrom(ctx context.Context) (caller, bool) {
	c, ok := ctx.Value(callerKey{}).(caller)
	return c, ok
}

func withCaller(r *http.Request, c caller) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), callerKey{}, c))
}

// authenticate puts the caller in the request context, from what the Platform
// API's sign-in left on the request. It adds two checks: the token names an
// org, and in idp mode it was issued for the MCP audience. Both fail with 401,
// so the client fetches a new token; later failures are tool errors.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		org, _ := middleware.GetOrganizationFromRequest(r)
		if org == "" || (s.auth.Mode == config.AuthModeIDP && !s.hasAudience(r)) {
			apperror.WriteHTTP(w, apperror.Unauthorized.New(), "")
			return
		}
		user, err := s.identity.InternalUserID(r)
		if err != nil {
			s.logger.Error("MCP: cannot resolve the caller's user ID", "error", err)
			apperror.WriteHTTP(w, apperror.Internal.New(), "")
			return
		}
		next.ServeHTTP(w, withCaller(r, caller{
			org:    org,
			user:   user,
			scopes: toSet(middleware.ResolveEffectiveScopes(r, s.auth.Authorization.Mode)),
		}))
	})
}

// hasAudience reports whether the token's aud list holds the MCP audience. The
// sign-in checks auth.idp.audience; this is the MCP one.
func (s *Server) hasAudience(r *http.Request) bool {
	ac, ok := authenticators.GetAuthContext(r)
	if !ok {
		return false
	}
	// The sign-in stores a plain map; upstream's claims middleware accepts both.
	var claims jwt.MapClaims
	switch v := ac.Claims.(type) {
	case jwt.MapClaims:
		claims = v
	case map[string]any:
		claims = v
	default:
		return false
	}
	aud, err := claims.GetAudience()
	return err == nil && slices.Contains(aud, s.cfg.Auth.Audience)
}

func toSet(list []string) map[string]bool {
	set := make(map[string]bool, len(list))
	for _, v := range list {
		set[v] = true
	}
	return set
}

// discoveryEnabled: login discovery needs an identity provider to point at.
func (s *Server) discoveryEnabled() bool {
	return s.auth.Mode == config.AuthModeIDP
}

func (s *Server) publicURL(path string) string {
	return strings.TrimRight(s.cfg.PublicURL, "/") + path
}

// discovery serves the protected-resource metadata, so a client can find where
// to log in. Public data only.
func (s *Server) discovery(w http.ResponseWriter, _ *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"resource":                 s.publicURL(Path),
		"authorization_servers":    s.auth.IDP.Issuer,
		"scopes_supported":         s.allScopes(),
		"bearer_methods_supported": []string{"header"},
	})
}
