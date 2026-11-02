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

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/gatewayrelay"
	"github.com/wso2/api-platform/platform-api/internal/gatewayrelay/egress"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
)

const (
	invokeOrg  = "org-uuid-1"
	invokePath = "/api/v0.9/gateway-relay/invoke"
)

// fakeInvokeTargets answers GetAPIInvokeTarget from fixed values.
type fakeInvokeTargets struct {
	apiContext string
	endpoints  []string
	err        error
	gotAPI     string
	gotGateway string
	gotOrg     string
}

func (f *fakeInvokeTargets) GetAPIInvokeTarget(apiHandle, gatewayHandle, orgID string) (string, []string, error) {
	f.gotAPI, f.gotGateway, f.gotOrg = apiHandle, gatewayHandle, orgID
	return f.apiContext, f.endpoints, f.err
}

// failingRelay returns err from every Do.
type failingRelay struct{ err error }

func (f failingRelay) Do(context.Context, gatewayrelay.Target, gatewayrelay.Request) (gatewayrelay.Relayed, error) {
	return gatewayrelay.Relayed{}, f.err
}

func newRelay(t *testing.T, allowHosts ...string) *gatewayrelay.Relay {
	t.Helper()
	policy, err := egress.Parse(egress.Spec{AllowHosts: allowHosts})
	require.NoError(t, err)
	relay, err := gatewayrelay.NewRelay(gatewayrelay.RelayOptions{
		RequestTimeout:   5 * time.Second,
		MaxRequestBytes:  1 << 10,
		MaxResponseBytes: 1 << 20,
		MaxConcurrent:    2,
		MaxPending:       2,
		Egress:           policy,
	})
	require.NoError(t, err)
	return relay
}

// serveInvoke sends body to the invoke route as the caller would, with the
// caller's own bearer token, and returns the recorded response.
func serveInvoke(t *testing.T, targets invokeTargets, relay relayer, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewAPIInvokeHandler(targets, relay, 1<<10, slog.New(slog.NewTextHandler(io.Discard, nil))).RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, invokePath, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Authorization", "Bearer caller-token")
	req = middleware.WithOrganization(req, invokeOrg)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// invokeBody is a request naming pizza-api on prod-gw, plus fields (JSON members, or "").
func invokeBody(fields string) string {
	if fields == "" {
		return `{"restApiId": "pizza-api", "gatewayId": "prod-gw"}`
	}
	return `{"restApiId": "pizza-api", "gatewayId": "prod-gw", ` + fields + `}`
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	return body.Code
}

func TestInvokeRelaysTheRequestAndReturnsTheGatewayResponse(t *testing.T) {
	var gotURI, gotBody string
	var gotHeaders http.Header
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.URL.RequestURI()
		gotHeaders = r.Header.Clone()
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("X-Order-Id", "42")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":42}`))
	}))
	defer gateway.Close()

	targets := &fakeInvokeTargets{apiContext: "/pizza/v1", endpoints: []string{gateway.URL}}
	rec := serveInvoke(t, targets, newRelay(t, "*"), "application/json", `{
		"restApiId": "pizza-api",
		"gatewayId": "prod-gw",
		"method": "post",
		"path": "/orders",
		"query": [{"name": "dryRun", "value": "true"}],
		"headers": [{"name": "X-API-Key", "value": "test-key"}],
		"body": "{\"item\":\"pizza\"}"
	}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, invokeOrg, targets.gotOrg, "the organization comes from the token, not the request")
	assert.Equal(t, "pizza-api", targets.gotAPI)
	assert.Equal(t, "prod-gw", targets.gotGateway)

	assert.Equal(t, "/pizza/v1/orders?dryRun=true", gotURI)
	assert.Equal(t, `{"item":"pizza"}`, gotBody)
	assert.Equal(t, "test-key", gotHeaders.Get("X-API-Key"))
	assert.Empty(t, gotHeaders.Get("Authorization"), "the caller's token must never reach the gateway")

	var res struct {
		Outcome  string `json:"outcome"`
		Response struct {
			Status       int    `json:"status"`
			Body         string `json:"body"`
			BodyEncoding string `json:"bodyEncoding"`
			Truncated    bool   `json:"truncated"`
			Headers      []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"headers"`
		} `json:"response"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	assert.Equal(t, "response", res.Outcome)
	assert.Equal(t, http.StatusCreated, res.Response.Status)
	assert.Equal(t, `{"id":42}`, res.Response.Body)
	assert.Equal(t, "utf8", res.Response.BodyEncoding)
	assert.False(t, res.Response.Truncated)
	assert.Contains(t, res.Response.Headers, struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}{"X-Order-Id", "42"})
}

func TestInvokeReturnsAGatewayErrorStatusAsASuccessfulRelay(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer gateway.Close()

	rec := serveInvoke(t, &fakeInvokeTargets{apiContext: "/pizza", endpoints: []string{gateway.URL}},
		newRelay(t, "*"), "application/json", invokeBody(`"path": "/"`))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":500`)
}

func TestInvokeRefusesARequestThatIsNotJSON(t *testing.T) {
	// Otherwise a page on another site could send it with no CORS preflight.
	for _, contentType := range []string{"", "text/plain", "application/x-www-form-urlencoded"} {
		rec := serveInvoke(t, &fakeInvokeTargets{}, failingRelay{}, contentType, invokeBody(`"path": "/"`))
		assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code, "Content-Type %q", contentType)
	}
}

func TestInvokeRefusesAMalformedOrOversizedBody(t *testing.T) {
	for name, tc := range map[string]struct {
		body   string
		status int
		code   string
	}{
		"unknown field":        {invokeBody(`"url": "https://evil.example.com"`), http.StatusBadRequest, apperror.CodeCommonValidationFailed},
		"no gateway named":     {`{"restApiId": "pizza-api"}`, http.StatusBadRequest, apperror.CodeCommonValidationFailed},
		"not JSON":             {`not json`, http.StatusBadRequest, apperror.CodeCommonValidationFailed},
		"over the limit":       {invokeBody(`"body": "` + strings.Repeat("a", 64<<10) + `"`), http.StatusRequestEntityTooLarge, apperror.CodeCommonPayloadTooLarge},
		"forbidden header":     {invokeBody(`"headers": [{"name": "Cookie", "value": "a=1"}]`), http.StatusBadRequest, apperror.CodeRESTAPIInvokeRequestInvalid},
		"path escapes the API": {invokeBody(`"path": "/../admin"`), http.StatusBadRequest, apperror.CodeRESTAPIInvokeRequestInvalid},
	} {
		t.Run(name, func(t *testing.T) {
			targets := &fakeInvokeTargets{apiContext: "/pizza", endpoints: []string{"https://gw.example.com"}}
			rec := serveInvoke(t, targets, newRelay(t, "*"), "application/json", tc.body)
			assert.Equal(t, tc.status, rec.Code, rec.Body.String())
			assert.Equal(t, tc.code, errorCode(t, rec))
		})
	}
}

func TestInvokeMapsTargetFailures(t *testing.T) {
	for name, tc := range map[string]struct {
		targets *fakeInvokeTargets
		status  int
		code    string
	}{
		"REST API not found": {
			&fakeInvokeTargets{err: apperror.RESTAPINotFound.New()},
			http.StatusNotFound, apperror.CodeRESTAPINotFound,
		},
		"not deployed to the gateway": {
			&fakeInvokeTargets{err: apperror.RESTAPIInvokeGatewayNotDeployed.New()},
			http.StatusNotFound, apperror.CodeRESTAPIInvokeGatewayNotDeployed,
		},
		"gateway without an endpoint": {
			&fakeInvokeTargets{apiContext: "/pizza"},
			http.StatusUnprocessableEntity, apperror.CodeRESTAPIInvokeGatewayNoEndpoint,
		},
		"gateway endpoint not http": {
			&fakeInvokeTargets{apiContext: "/events", endpoints: []string{"wss://gw.example.com"}},
			http.StatusUnprocessableEntity, apperror.CodeRESTAPIInvokeGatewayUnsupportedScheme,
		},
		"lookup failure": {
			&fakeInvokeTargets{err: fmt.Errorf("database is down")},
			http.StatusInternalServerError, apperror.CodeCommonInternalError,
		},
	} {
		t.Run(name, func(t *testing.T) {
			rec := serveInvoke(t, tc.targets, failingRelay{}, "application/json", invokeBody(`"path": "/"`))
			assert.Equal(t, tc.status, rec.Code, rec.Body.String())
			assert.Equal(t, tc.code, errorCode(t, rec))
		})
	}
}

func TestInvokeRefusesATargetOutsideTheAllowList(t *testing.T) {
	targets := &fakeInvokeTargets{apiContext: "/pizza", endpoints: []string{"https://internal.example.com:9443"}}
	rec := serveInvoke(t, targets, newRelay(t, "https://gw.example.com"), "application/json", invokeBody(`"path": "/"`))

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, apperror.CodeRESTAPIInvokeTargetNotAllowed, errorCode(t, rec))
	assert.NotContains(t, rec.Body.String(), "internal.example.com", "the resolved address goes to the log only")
}

func TestInvokeMapsRelayFailures(t *testing.T) {
	for name, tc := range map[string]struct {
		err    error
		status int
		code   string
	}{
		"busy":        {gatewayrelay.ErrBusy, http.StatusServiceUnavailable, apperror.CodeRESTAPIInvokeBusy},
		"timeout":     {fmt.Errorf("relay: %w", context.DeadlineExceeded), http.StatusGatewayTimeout, apperror.CodeRESTAPIInvokeUpstreamTimeout},
		"unreachable": {fmt.Errorf("dial tcp 10.0.0.7:9443: connection refused"), http.StatusBadGateway, apperror.CodeRESTAPIInvokeUpstreamUnreachable},
	} {
		t.Run(name, func(t *testing.T) {
			targets := &fakeInvokeTargets{apiContext: "/pizza", endpoints: []string{"https://gw.example.com"}}
			rec := serveInvoke(t, targets, failingRelay{err: tc.err}, "application/json", invokeBody(`"path": "/"`))
			assert.Equal(t, tc.status, rec.Code, rec.Body.String())
			assert.Equal(t, tc.code, errorCode(t, rec))
			assert.NotContains(t, rec.Body.String(), "10.0.0.7")
		})
	}
}
