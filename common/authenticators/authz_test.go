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
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wso2/api-platform/common/models"
)

// injectAuthContext injects an AuthContext into a request's context for testing.
func injectAuthContext(r *http.Request, ac models.AuthContext) *http.Request {
	ctx := context.WithValue(r.Context(), authContextKeyType{}, ac)
	return r.WithContext(ctx)
}

// injectAuthzSkip sets the authz-skip flag for testing.
func injectAuthzSkip(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), authzSkipKeyType{}, true)
	return r.WithContext(ctx)
}

// buildMux constructs a ServeMux where the authz middleware runs INSIDE the
// route handler (after the ServeMux has matched the pattern and set r.Pattern).
// preHandler injects context values (auth context, skip flags) before authz runs.
func buildMux(
	pattern string,
	config models.AuthConfig,
	logger *slog.Logger,
	preHandler func(*http.Request) *http.Request,
) http.Handler {
	mux := http.NewServeMux()
	authzMW := AuthorizationMiddleware(config, logger)
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		// r.Pattern is set at this point — authz can read it
		if preHandler != nil {
			r = preHandler(r)
		}
		authzMW(ok).ServeHTTP(w, r)
	})
	return mux
}

func TestAuthorizationMiddleware_NoResourceRoles_AllowsAllRequests(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{ResourceRoles: map[string][]string{}}

	h := buildMux("GET /api/users", config, logger, func(r *http.Request) *http.Request {
		return injectAuthContext(r, models.AuthContext{Roles: []string{"developer", "consumer"}})
	})

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users", nil)
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthorizationMiddleware_NoResourceRoles_NilMap_AllowsAllRequests(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{ResourceRoles: nil}

	h := buildMux("POST /api/resources", config, logger, func(r *http.Request) *http.Request {
		return injectAuthContext(r, models.AuthContext{Roles: []string{"admin"}})
	})

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/resources", nil)
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthorizationMiddleware_WithResourceRoles_MatchingRole_Allowed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/users": {"developer", "admin"},
		},
	}

	h := buildMux("GET /api/users", config, logger, func(r *http.Request) *http.Request {
		return injectAuthContext(r, models.AuthContext{Roles: []string{"developer"}})
	})

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users", nil)
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthorizationMiddleware_WithResourceRoles_NoMatchingRole_Forbidden(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/users": {"admin"},
		},
	}

	h := buildMux("GET /api/users", config, logger, func(r *http.Request) *http.Request {
		return injectAuthContext(r, models.AuthContext{Roles: []string{"developer"}})
	})

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users", nil)
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "forbidden")
}

func TestAuthorizationMiddleware_WithResourceRoles_MultipleRoles_OneMatches_Allowed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"POST /api/resources": {"admin", "developer"},
		},
	}

	h := buildMux("POST /api/resources", config, logger, func(r *http.Request) *http.Request {
		return injectAuthContext(r, models.AuthContext{Roles: []string{"developer", "consumer", "admin"}})
	})

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/resources", nil)
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthorizationMiddleware_ResourceNotDefined_Forbidden(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/users": {"admin"},
		},
	}

	// register /api/products but config only defines /api/users
	h := buildMux("GET /api/products", config, logger, func(r *http.Request) *http.Request {
		return injectAuthContext(r, models.AuthContext{Roles: []string{"admin"}})
	})

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/products", nil)
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAuthorizationMiddleware_NoUserRoles_Forbidden(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/users": {"developer"},
		},
	}

	h := buildMux("GET /api/users", config, logger, func(r *http.Request) *http.Request {
		return injectAuthContext(r, models.AuthContext{Roles: []string{}})
	})

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users", nil)
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAuthorizationMiddleware_RolesNotSetInContext_Forbidden(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/users": {"developer"},
		},
	}

	// no pre-handler: auth context is never set
	h := buildMux("GET /api/users", config, logger, nil)

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users", nil)
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAuthorizationMiddleware_AuthSkipped_BypassesAuthorization(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/public": {"admin"},
		},
	}

	h := buildMux("GET /api/public", config, logger, func(r *http.Request) *http.Request {
		return injectAuthzSkip(r)
	})

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/public", nil)
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthorizationMiddleware_DifferentMethodsSamePathDifferentRoles(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/users":    {"consumer", "developer", "admin"},
			"POST /api/users":   {"admin"},
			"DELETE /api/users": {"admin"},
		},
	}

	// buildMux places authz inside the mux handler so r.Pattern is set when authz runs.
	run := func(method, role string) int {
		h := buildMux(method+" /api/users", config, logger, func(r *http.Request) *http.Request {
			return injectAuthContext(r, models.AuthContext{Roles: []string{role}})
		})
		rr := httptest.NewRecorder()
		req, _ := http.NewRequest(method, "/api/users", nil)
		h.ServeHTTP(rr, req)
		return rr.Code
	}

	assert.Equal(t, http.StatusOK, run("GET", "developer"))
	assert.Equal(t, http.StatusForbidden, run("POST", "developer"))
	assert.Equal(t, http.StatusOK, run("DELETE", "admin"))
}

func TestAuthorizationMiddleware_UnregisteredPath_Returns404(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/users": {"admin"},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/users", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	authzMW := AuthorizationMiddleware(config, logger)
	h := authzMW(mux)

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/foo", nil)
	req = injectAuthContext(req, models.AuthContext{Roles: []string{"admin"}})
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestAuthorizationMiddleware_CaseSensitiveRoles(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/users": {"Admin"},
		},
	}

	h := buildMux("GET /api/users", config, logger, func(r *http.Request) *http.Request {
		return injectAuthContext(r, models.AuthContext{Roles: []string{"admin"}})
	})

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users", nil)
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAuthorizationMiddleware_SkipAuthzFlag_BypassesAuthorization(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/users": {"admin", "developer"},
		},
	}

	h := buildMux("GET /api/users", config, logger, func(r *http.Request) *http.Request {
		r = injectAuthzSkip(r)
		return injectAuthContext(r, models.AuthContext{Roles: []string{}})
	})

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users", nil)
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthorizationMiddleware_WildcardRoleMapping(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET /api/data": {"developer"},
		},
	}

	h := buildMux("GET /api/data", config, logger, func(r *http.Request) *http.Request {
		return injectAuthContext(r, models.AuthContext{Roles: []string{"developer"}})
	})

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/data", nil)
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}
