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

package middleware

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"platform-api/src/config"

	"github.com/gin-gonic/gin"
)

const (
	readOnlyTestOrgA = "11111111-1111-1111-1111-111111111111"
	readOnlyTestOrgB = "22222222-2222-2222-2222-222222222222"
)

// newReadOnlyTestRouter mimics the production chain: a stub authentication middleware
// that places the organization in the context (when setOrg is true), then ReadOnlyGuard,
// then routes registered at the exact templates the guard's exempt list matches on.
// Requests go through a real router so c.FullPath() is populated.
func newReadOnlyTestRouter(ro *config.ReadOnly, setOrg bool, org string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if setOrg {
			c.Set("organization", org)
		}
		c.Next()
	})
	router.Use(ReadOnlyGuard(ro, slog.New(slog.NewTextHandler(io.Discard, nil))))

	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	router.GET("/api/v1/projects", ok)
	router.HEAD("/api/v1/projects", ok)
	router.POST("/api/v1/projects", ok)
	router.PUT("/api/v1/projects/:id", ok)
	router.DELETE("/api/v1/projects/:id", ok)
	router.POST("/api/v1/rest-apis/validate-openapi", ok)
	router.POST("/api/internal/v1/deployments/fetch-batch", ok)
	router.POST("/api/internal/v1/apis/:apiId/gateway-deployments", ok)
	return router
}

func doReadOnlyRequest(router *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

func TestReadOnlyGuard(t *testing.T) {
	listMode := &config.ReadOnly{Organizations: []string{readOnlyTestOrgA}}
	allMode := &config.ReadOnly{AllOrganizations: true}
	disabled := &config.ReadOnly{}

	tests := []struct {
		name   string
		ro     *config.ReadOnly
		setOrg bool
		org    string
		method string
		path   string
		want   int
	}{
		{"disabled: write by listed org passes", disabled, true, readOnlyTestOrgA, http.MethodPost, "/api/v1/projects", http.StatusOK},
		{"list: GET by read-only org passes", listMode, true, readOnlyTestOrgA, http.MethodGet, "/api/v1/projects", http.StatusOK},
		{"list: HEAD by read-only org passes", listMode, true, readOnlyTestOrgA, http.MethodHead, "/api/v1/projects", http.StatusOK},
		{"list: POST by read-only org rejected", listMode, true, readOnlyTestOrgA, http.MethodPost, "/api/v1/projects", http.StatusServiceUnavailable},
		{"list: PUT by read-only org rejected", listMode, true, readOnlyTestOrgA, http.MethodPut, "/api/v1/projects/p1", http.StatusServiceUnavailable},
		{"list: DELETE by read-only org rejected", listMode, true, readOnlyTestOrgA, http.MethodDelete, "/api/v1/projects/p1", http.StatusServiceUnavailable},
		{"list: POST by other org passes", listMode, true, readOnlyTestOrgB, http.MethodPost, "/api/v1/projects", http.StatusOK},
		{"list: exempt read POST by read-only org passes", listMode, true, readOnlyTestOrgA, http.MethodPost, "/api/v1/rest-apis/validate-openapi", http.StatusOK},
		{"list: exempt internal POST by read-only org passes", listMode, true, readOnlyTestOrgA, http.MethodPost, "/api/internal/v1/deployments/fetch-batch", http.StatusOK},
		{"list: internal write without org falls through to handler", listMode, false, "", http.MethodPost, "/api/internal/v1/apis/a1/gateway-deployments", http.StatusOK},
		{"list: empty org claim passes", listMode, true, "", http.MethodPost, "/api/v1/projects", http.StatusOK},
		{"list: unmatched route stays 404", listMode, true, readOnlyTestOrgA, http.MethodPost, "/nope", http.StatusNotFound},
		{"all: POST by any org rejected", allMode, true, readOnlyTestOrgB, http.MethodPost, "/api/v1/projects", http.StatusServiceUnavailable},
		{"all: POST with empty org rejected", allMode, true, "", http.MethodPost, "/api/v1/projects", http.StatusServiceUnavailable},
		{"all: internal write without org rejected", allMode, false, "", http.MethodPost, "/api/internal/v1/apis/a1/gateway-deployments", http.StatusServiceUnavailable},
		{"all: exempt internal POST without org passes", allMode, false, "", http.MethodPost, "/api/internal/v1/deployments/fetch-batch", http.StatusOK},
		{"all: GET passes", allMode, true, readOnlyTestOrgB, http.MethodGet, "/api/v1/projects", http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := newReadOnlyTestRouter(tc.ro, tc.setOrg, tc.org)
			w := doReadOnlyRequest(router, tc.method, tc.path)
			if w.Code != tc.want {
				t.Fatalf("%s %s: got status %d, want %d (body: %s)", tc.method, tc.path, w.Code, tc.want, w.Body.String())
			}
			if tc.want != http.StatusServiceUnavailable {
				return
			}

			var body struct {
				Code        int    `json:"code"`
				Message     string `json:"message"`
				Description string `json:"description"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to decode 503 body %q: %v", w.Body.String(), err)
			}
			if body.Code != http.StatusServiceUnavailable {
				t.Errorf("body.code = %d, want 503", body.Code)
			}
			if body.Description != ReadOnlyErrorDescription {
				t.Errorf("body.description = %q, want %q", body.Description, ReadOnlyErrorDescription)
			}
		})
	}
}
