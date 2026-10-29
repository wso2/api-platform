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

package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serve(c *Checker, method string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	c.Handler().ServeHTTP(rec, httptest.NewRequest(method, "/health", nil))
	return rec
}

func failing(err error) CheckFunc {
	return func(context.Context) error { return err }
}

func TestHandlerHealthy(t *testing.T) {
	c, _ := newTestChecker(testOptions())
	c.Register("database", passing)
	c.Register("cache", passing)

	rec := serve(c, http.MethodGet)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.JSONEq(t, `{
		"status": "ok",
		"timestamp": "2026-10-13T09:00:00Z",
		"checks": {
			"cache": {"status": "up"},
			"database": {"status": "up"}
		}
	}`, rec.Body.String())
}

func TestHandlerUnhealthy(t *testing.T) {
	c, _ := newTestChecker(testOptions())
	c.Register("database", failing(Reason("database unreachable")))
	c.Register("cache", passing)

	rec := serve(c, http.MethodGet)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "unhealthy", got["status"])
	assert.NotContains(t, got, "component")

	checks := got["checks"].(map[string]any)
	down := checks["database"].(map[string]any)
	assert.Equal(t, "down", down["status"])
	assert.Equal(t, "database unreachable", down["error"])
	assert.Equal(t, "2026-10-13T09:00:00Z", down["since"])

	up := checks["cache"].(map[string]any)
	assert.Equal(t, "up", up["status"])
	assert.NotContains(t, up, "error")
	assert.NotContains(t, up, "since")
}

func TestHandlerNoChecksRendersEmptyObject(t *testing.T) {
	c, _ := newTestChecker(testOptions())

	rec := serve(c, http.MethodGet)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"checks":{}`)
}

func TestHandlerHeadHasNoBody(t *testing.T) {
	tests := []struct {
		name       string
		check      CheckFunc
		wantStatus int
	}{
		{name: "healthy", check: passing, wantStatus: http.StatusOK},
		{name: "unhealthy", check: failing(errors.New("boom")), wantStatus: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newTestChecker(testOptions())
			c.Register("database", tt.check)

			rec := serve(c, http.MethodHead)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
			assert.Empty(t, rec.Body.Bytes())
		})
	}
}

func TestHandlerRejectsOtherMethods(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			c, _ := newTestChecker(testOptions())
			c.Register("database", passing)

			rec := serve(c, method)

			assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
			assert.Equal(t, "GET, HEAD", rec.Header().Get("Allow"))
			assert.Empty(t, rec.Body.Bytes())
		})
	}
}

func TestHandlerTimestampIsUTCWithSecondPrecision(t *testing.T) {
	c, clk := newTestChecker(testOptions())
	zone := time.FixedZone("UTC+5:30", 5*3600+30*60)
	clk.mu.Lock()
	clk.t = time.Date(2026, 10, 13, 14, 30, 7, 987654321, zone)
	clk.mu.Unlock()
	c.Register("database", failing(errors.New("boom")))

	rec := serve(c, http.MethodGet)

	var got struct {
		Timestamp string `json:"timestamp"`
		Checks    map[string]struct {
			Since string `json:"since"`
		} `json:"checks"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "2026-10-13T09:00:07Z", got.Timestamp)
	assert.Equal(t, "2026-10-13T09:00:07Z", got.Checks["database"].Since)
}

func TestHandlerNeverRendersRawCause(t *testing.T) {
	c, _ := newTestChecker(testOptions())
	c.Register("database", failing(errors.New("dial postgres://u:p@host/db: connection refused")))

	rec := serve(c, http.MethodGet)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.NotContains(t, rec.Body.String(), "postgres://u:p@host")
	assert.NotContains(t, rec.Body.String(), "connection refused")
	assert.Contains(t, rec.Body.String(), genericFailure)
}

func TestStatusCode(t *testing.T) {
	assert.Equal(t, http.StatusOK, StatusCode(Result{Healthy: true}))
	assert.Equal(t, http.StatusServiceUnavailable, StatusCode(Result{Healthy: false}))
}

func TestRenderBody(t *testing.T) {
	since := time.Date(2026, 10, 13, 8, 59, 30, 0, time.UTC)
	res := Result{
		Healthy:     false,
		Status:      UnhealthyStatus,
		EvaluatedAt: time.Date(2026, 10, 13, 9, 0, 0, 5, time.UTC),
		Checks: map[string]CheckResult{
			"database": {Status: StatusDown, Error: "database unreachable", Since: since, cause: errors.New("secret")},
			"cache":    {Status: StatusUp},
		},
	}

	got, err := RenderBody(res)

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"status": "unhealthy",
		"timestamp": "2026-10-13T09:00:00Z",
		"checks": {
			"cache": {"status": "up"},
			"database": {"status": "down", "error": "database unreachable", "since": "2026-10-13T08:59:30Z"}
		}
	}`, string(got))
	assert.Regexp(t, `^\{"status":"unhealthy","timestamp":`, string(got))
	assert.NotContains(t, string(got), "component")
}

// loopbackOnly returns options that serve only loopback clients, the default allowed set.
func loopbackOnly(t *testing.T) Options {
	t.Helper()
	set, err := ParseAllowedClients("platform_api.health", nil)
	require.NoError(t, err)
	opts := testOptions()
	opts.AllowedClients = set
	return opts
}

func serveFrom(c *Checker, method, remoteAddr string, header http.Header) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, "/health", nil)
	req.RemoteAddr = remoteAddr
	for k, v := range header {
		req.Header[k] = v
	}
	c.Handler().ServeHTTP(rec, req)
	return rec
}

func TestHandlerRefusesClientOutsideAllowedSet(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			c, _ := newTestChecker(loopbackOnly(t))
			calls := 0
			c.Register("database", func(context.Context) error {
				calls++
				return Reason("database unreachable")
			})

			rec := serveFrom(c, method, "203.0.113.9:51000", nil)

			assert.Equal(t, http.StatusForbidden, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
			assert.JSONEq(t, `{"status":"forbidden"}`, rec.Body.String())
			assert.Zero(t, calls, "a refused request must not run any check")
			assert.Nil(t, c.cached, "a refused request must not refresh the cache")
		})
	}
}

func TestHandlerRefusalIsNotLogged(t *testing.T) {
	var logs logCapture
	opts := logs.options()
	opts.AllowedClients = loopbackOnly(t).AllowedClients
	c, _ := newTestChecker(opts)
	c.Register("database", failing(Reason("database unreachable")))

	serveFrom(c, http.MethodGet, "203.0.113.9:51000", nil)

	assert.Empty(t, logs.health.String())
	assert.Empty(t, logs.debug.String())
}

func TestHandlerServesAllowedClient(t *testing.T) {
	c, _ := newTestChecker(loopbackOnly(t))
	c.Register("database", passing)

	for _, remoteAddr := range []string{"127.0.0.1:51000", "[::1]:51000", "[::ffff:127.0.0.1]:51000"} {
		rec := serveFrom(c, http.MethodGet, remoteAddr, nil)

		assert.Equal(t, http.StatusOK, rec.Code, remoteAddr)
		assert.Contains(t, rec.Body.String(), `"database":{"status":"up"}`, remoteAddr)
	}
}

func TestHandlerIgnoresForwardingHeaders(t *testing.T) {
	forged := http.Header{
		"X-Forwarded-For":          {"127.0.0.1"},
		"X-Real-Ip":                {"127.0.0.1"},
		"Forwarded":                {"for=127.0.0.1"},
		"X-Envoy-External-Address": {"127.0.0.1"},
	}
	c, _ := newTestChecker(loopbackOnly(t))
	c.Register("database", passing)

	assert.Equal(t, http.StatusForbidden, serveFrom(c, http.MethodGet, "203.0.113.9:51000", forged).Code)

	forged = http.Header{"X-Forwarded-For": {"203.0.113.9"}}
	assert.Equal(t, http.StatusOK, serveFrom(c, http.MethodGet, "127.0.0.1:51000", forged).Code)
}
