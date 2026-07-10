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
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wso2/api-platform/common/models"
)

func TestAuthMiddleware_NoAuthenticatorsConfigured_AllowsAllRequests(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{
		BasicAuth: &models.BasicAuth{Enabled: false},
		JWTConfig: &models.IDPConfig{Enabled: false},
	}

	mw, err := AuthMiddleware(config, logger)
	assert.NoError(t, err)

	var gotSkip bool
	var gotCtx models.AuthContext
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSkip = GetAuthzSkip(r)
		gotCtx, _ = GetAuthContext(r)
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, gotSkip)
	assert.True(t, gotCtx.Authenticated)
}

func TestAuthMiddleware_JWTEnabled_MissingJWKS_FailsAtCreation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{
		JWTConfig: &models.IDPConfig{
			Enabled:   true,
			IssuerURL: "https://issuer.example.com",
			JWKSUrl:   "",
		},
	}
	_, err := AuthMiddleware(config, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize JWT authenticator")
}

func TestAuthMiddleware_JWTEnabled_NoIssuer_MissingJWKS_FailsAtCreation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{
		JWTConfig: &models.IDPConfig{
			Enabled:   true,
			IssuerURL: "",
			JWKSUrl:   "",
		},
	}
	_, err := AuthMiddleware(config, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize JWT authenticator")
}

func TestAuthMiddleware_JWTEnabled_NoIssuer_WithJWKS_FailsAtCreation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{
		JWTConfig: &models.IDPConfig{
			Enabled:   true,
			IssuerURL: "",
			JWKSUrl:   "https://jwks.example.com/keys",
		},
	}
	_, err := AuthMiddleware(config, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize JWT authenticator")
}

func TestAuthMiddleware_BasicAuthEnabled_NoCredentials_Unauthorized(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{
		BasicAuth: &models.BasicAuth{
			Enabled: true,
			Users: []models.User{
				{Username: "testuser", Password: "testpass", Roles: []string{"developer"}},
			},
		},
		JWTConfig: &models.IDPConfig{Enabled: false},
	}

	mw, err := AuthMiddleware(config, logger)
	assert.NoError(t, err)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/protected", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "no valid authentication credentials provided")
}

func TestAuthMiddleware_SkipPaths_NoAuthRequired(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{
		BasicAuth: &models.BasicAuth{
			Enabled: true,
			Users: []models.User{
				{Username: "testuser", Password: "testpass", Roles: []string{"developer"}},
			},
		},
		SkipPaths: []string{"/health", "/metrics"},
	}

	mw, err := AuthMiddleware(config, logger)
	assert.NoError(t, err)

	var gotSkip bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSkip = GetAuthzSkip(r)
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, gotSkip)
}

func TestAuthMiddleware_NilBasicAuth_NilJWTConfig_AllowsAllRequests(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := models.AuthConfig{BasicAuth: nil, JWTConfig: nil}

	mw, err := AuthMiddleware(config, logger)
	assert.NoError(t, err)

	var gotSkip bool
	var gotCtx models.AuthContext
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSkip = GetAuthzSkip(r)
		gotCtx, _ = GetAuthContext(r)
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/open", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, gotSkip)
	assert.True(t, gotCtx.Authenticated)
}
