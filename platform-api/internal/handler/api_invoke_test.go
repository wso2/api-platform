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
	"encoding/base64"
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

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/gatewayrelay"
	"github.com/wso2/api-platform/platform-api/internal/gatewayrelay/egress"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

const (
	invokeOrg  = "org-uuid-1"
	invokePath = "/api/v0.9/gateway-relay/invoke"
)

// fakeInvokeTargets answers GetAPIInvokeTarget from fixed values.
type fakeInvokeTargets struct {
	apiContext string
	vhost      string
	endpoints  []string
	err        error
	delay      time.Duration // how long the lookup takes
	gotAPI     string
	gotGateway string
	gotOrg     string
}

func (f *fakeInvokeTargets) GetAPIInvokeTarget(apiHandle, gatewayHandle, orgID string) (*service.APIInvokeTarget, error) {
	f.gotAPI, f.gotGateway, f.gotOrg = apiHandle, gatewayHandle, orgID
	time.Sleep(f.delay)
	if f.err != nil {
		return nil, f.err
	}
	return &service.APIInvokeTarget{Context: f.apiContext, Vhost: f.vhost, Endpoints: f.endpoints}, nil
}

// deadlineRelay records the deadline of the context Do is called with.
type deadlineRelay struct {
	deadline time.Time
	ok       bool
}

func (d *deadlineRelay) Do(ctx context.Context, _ gatewayrelay.Target, _ gatewayrelay.Request) (gatewayrelay.Relayed, error) {
	d.deadline, d.ok = ctx.Deadline()
	return gatewayrelay.Relayed{Status: http.StatusOK}, nil
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
// caller's own bearer token, and returns the recorded response. targets answer
// for both API kinds, and authorization is off.
func serveInvoke(t *testing.T, targets invokeTargets, relay relayer, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	return serveInvokeAs(t, targets, relay, config.Authorization{}, "", contentType, body)
}

// serveInvokeAs is serveInvoke with authorization authz and a token carrying scope.
func serveInvokeAs(t *testing.T, targets invokeTargets, relay relayer, authz config.Authorization, scope, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewAPIInvokeHandler(targets, targets, relay, authz, 1<<10, 5*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil))).RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, invokePath, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Authorization", "Bearer caller-token")
	req = middleware.WithScope(middleware.WithOrganization(req, invokeOrg), scope)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// invokeBody is a request naming pizza-api on prod-gw, plus fields (JSON members, or "").
func invokeBody(fields string) string {
	if fields == "" {
		return `{"apiType": "rest-api", "apiId": "pizza-api", "gatewayId": "prod-gw"}`
	}
	return `{"apiType": "rest-api", "apiId": "pizza-api", "gatewayId": "prod-gw", ` + fields + `}`
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
		"apiType": "rest-api",
		"apiId": "pizza-api",
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

func TestInvokeSendsTheDeploymentVhostAsHost(t *testing.T) {
	var gotHost string
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
	}))
	defer gateway.Close()

	targets := &fakeInvokeTargets{apiContext: "/pizza", vhost: "pizza.example.com", endpoints: []string{gateway.URL}}
	rec := serveInvoke(t, targets, newRelay(t, "*"), "application/json", invokeBody(`"path": "/"`))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "pizza.example.com", gotHost)
}

// The server's write deadline counts reading the request and finding the target,
// so request_timeout does too: it starts when the request arrives, and a slow
// lookup is taken from the gateway call rather than added to it.
func TestInvokeStartsTheRequestTimeoutBeforeTheRelay(t *testing.T) {
	relay := &deadlineRelay{}
	mux := http.NewServeMux()
	targets := &fakeInvokeTargets{apiContext: "/pizza", endpoints: []string{"https://gw.example.com"}, delay: 2 * time.Second}
	NewAPIInvokeHandler(targets, targets, relay, config.Authorization{}, 1<<10, 30*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil))).RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, invokePath, strings.NewReader(invokeBody(`"path": "/"`)))
	req.Header.Set("Content-Type", "application/json")
	req = middleware.WithOrganization(req, invokeOrg)

	before := time.Now()
	mux.ServeHTTP(httptest.NewRecorder(), req)

	require.True(t, relay.ok, "the relay must get a context with a deadline")
	// Counted from after the 2s lookup it would be at least before+32s.
	assert.WithinDuration(t, before.Add(30*time.Second), relay.deadline, time.Second)
}

// A base64 body can be within the read limit yet over max_request_bytes once
// decoded; it is refused with the same 413, not as an invalid request.
func TestInvokeRefusesABodyOverTheLimitOnceDecoded(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString(make([]byte, 2<<10)) // 2 KiB decoded, over the 1 KiB limit
	targets := &fakeInvokeTargets{apiContext: "/pizza", endpoints: []string{"https://gw.example.com"}}
	rec := serveInvoke(t, targets, newRelay(t, "*"), "application/json",
		invokeBody(`"method": "POST", "path": "/", "bodyEncoding": "base64", "body": "`+encoded+`"`))

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, rec.Body.String())
	assert.Equal(t, apperror.CodeCommonPayloadTooLarge, errorCode(t, rec))
}

// JSON escaping doubles each quote in a text body, so a body within
// max_request_bytes may be up to twice as long on the wire, and is still accepted.
func TestInvokeAcceptsAnEscapedTextBodyWithinTheLimit(t *testing.T) {
	const maxRequestBytes = 64 << 10
	var gotLen int
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotLen = len(b)
	}))
	defer gateway.Close()
	policy, err := egress.Parse(egress.Spec{AllowHosts: []string{"*"}})
	require.NoError(t, err)
	relay, err := gatewayrelay.NewRelay(gatewayrelay.RelayOptions{
		RequestTimeout: 5 * time.Second, MaxRequestBytes: maxRequestBytes, MaxResponseBytes: 1 << 10,
		MaxConcurrent: 1, MaxPending: 1, Egress: policy,
	})
	require.NoError(t, err)
	mux := http.NewServeMux()
	targets := &fakeInvokeTargets{apiContext: "/pizza", endpoints: []string{gateway.URL}}
	NewAPIInvokeHandler(targets, targets, relay, config.Authorization{}, maxRequestBytes, 5*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil))).RegisterRoutes(mux)

	quotes := strings.Repeat(`\"`, maxRequestBytes-1<<10) // each decodes to one byte
	req := httptest.NewRequest(http.MethodPost, invokePath, strings.NewReader(invokeBody(`"method": "POST", "path": "/", "body": "`+quotes+`"`)))
	req.Header.Set("Content-Type", "application/json")
	req = middleware.WithOrganization(req, invokeOrg)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, maxRequestBytes-1<<10, gotLen)
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
		"the old restApiId":    {`{"restApiId": "pizza-api", "gatewayId": "prod-gw"}`, http.StatusBadRequest, apperror.CodeCommonValidationFailed},
		"no gateway named":     {`{"apiType": "rest-api", "apiId": "pizza-api"}`, http.StatusBadRequest, apperror.CodeCommonValidationFailed},
		"no API named":         {`{"apiType": "rest-api", "gatewayId": "prod-gw"}`, http.StatusBadRequest, apperror.CodeCommonValidationFailed},
		"no API type":          {`{"apiId": "pizza-api", "gatewayId": "prod-gw"}`, http.StatusBadRequest, apperror.CodeCommonValidationFailed},
		"unknown API type":     {`{"apiType": "mcp-proxy", "apiId": "pizza-api", "gatewayId": "prod-gw"}`, http.StatusBadRequest, apperror.CodeCommonValidationFailed},
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

// graphqlInvokeBody is a GraphQL request naming countries on prod-gw, plus fields (JSON members, or "").
func graphqlInvokeBody(fields string) string {
	body := `{"apiType": "graphql-api", "apiId": "countries", "gatewayId": "prod-gw", "body": "{\"query\":\"{ __typename }\"}"`
	if fields != "" {
		body += ", " + fields
	}
	return body + "}"
}

// A GraphQL API has one route on the gateway: a POST to its context, with no trailing slash.
func TestInvokeGraphQLPostsToTheAPIContext(t *testing.T) {
	var gotMethod, gotURI, gotBody, gotHost string
	var gotHeaders http.Header
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotURI, gotHost = r.Method, r.URL.RequestURI(), r.Host
		gotHeaders = r.Header.Clone()
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"__typename":"Query"}}`))
	}))
	defer gateway.Close()

	targets := &fakeInvokeTargets{apiContext: "/countries/v1/graphql", endpoints: []string{gateway.URL}}
	rec := serveInvoke(t, targets, newRelay(t, "*"), "application/json", graphqlInvokeBody(
		`"query": [{"name": "apikey", "value": "query-key"}], "headers": [{"name": "Content-Type", "value": "application/json"}, {"name": "X-API-Key", "value": "test-key"}]`))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, invokeOrg, targets.gotOrg)
	assert.Equal(t, "countries", targets.gotAPI)
	assert.Equal(t, "prod-gw", targets.gotGateway)

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/countries/v1/graphql?apikey=query-key", gotURI)
	assert.Equal(t, `{"query":"{ __typename }"}`, gotBody)
	assert.Equal(t, "test-key", gotHeaders.Get("X-API-Key"))
	assert.Equal(t, "application/json", gotHeaders.Get("Content-Type"))
	assert.Empty(t, gotHeaders.Get("Authorization"), "the caller's token must never reach the gateway")
	assert.Equal(t, strings.TrimPrefix(gateway.URL, "http://"), gotHost, "a GraphQL API is served on the gateway's default vhost")
	assert.Contains(t, rec.Body.String(), `"status":200`)
	assert.Contains(t, rec.Body.String(), `__typename`)
}

func TestInvokeRoutesEachKindToItsOwnTargets(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer gateway.Close()
	restTargets := &fakeInvokeTargets{apiContext: "/pizza", endpoints: []string{gateway.URL}}
	graphqlTargets := &fakeInvokeTargets{apiContext: "/countries", endpoints: []string{gateway.URL}}
	mux := http.NewServeMux()
	NewAPIInvokeHandler(restTargets, graphqlTargets, newRelay(t, "*"), config.Authorization{}, 1<<10, 5*time.Second,
		slog.New(slog.NewTextHandler(io.Discard, nil))).RegisterRoutes(mux)

	for _, body := range []string{invokeBody(""), graphqlInvokeBody("")} {
		req := httptest.NewRequest(http.MethodPost, invokePath, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, middleware.WithOrganization(req, invokeOrg))
		require.Equal(t, http.StatusOK, rec.Code, "%s: %s", body, rec.Body.String())
	}

	assert.Equal(t, "pizza-api", restTargets.gotAPI)
	assert.Equal(t, "countries", graphqlTargets.gotAPI)
}

func TestInvokeGraphQLRefusesAMalformedRequest(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		code string
	}{
		"a method of its own": {graphqlInvokeBody(`"method": "POST"`), apperror.CodeCommonValidationFailed},
		"a path of its own":   {graphqlInvokeBody(`"path": "/"`), apperror.CodeCommonValidationFailed},
		"no body":             {`{"apiType": "graphql-api", "apiId": "countries", "gatewayId": "prod-gw"}`, apperror.CodeCommonValidationFailed},
		"forbidden header":    {graphqlInvokeBody(`"headers": [{"name": "Cookie", "value": "a=1"}]`), apperror.CodeGraphQLAPIInvokeRequestInvalid},
	} {
		t.Run(name, func(t *testing.T) {
			targets := &fakeInvokeTargets{apiContext: "/countries", endpoints: []string{"https://gw.example.com"}}
			rec := serveInvoke(t, targets, newRelay(t, "*"), "application/json", tc.body)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.Equal(t, tc.code, errorCode(t, rec))
		})
	}
}

func TestInvokeGraphQLMapsFailuresToItsOwnCodes(t *testing.T) {
	for name, tc := range map[string]struct {
		targets *fakeInvokeTargets
		relay   relayer
		status  int
		code    string
	}{
		"GraphQL API not found": {
			&fakeInvokeTargets{err: apperror.GraphQLAPINotFound.New()}, failingRelay{},
			http.StatusNotFound, apperror.CodeGraphQLAPINotFound,
		},
		"not deployed to the gateway": {
			&fakeInvokeTargets{err: apperror.GraphQLAPIInvokeGatewayNotDeployed.New()}, failingRelay{},
			http.StatusNotFound, apperror.CodeGraphQLAPIInvokeGatewayNotDeployed,
		},
		"gateway without an endpoint": {
			&fakeInvokeTargets{apiContext: "/countries"}, failingRelay{},
			http.StatusUnprocessableEntity, apperror.CodeGraphQLAPIInvokeGatewayNoEndpoint,
		},
		"gateway endpoint not http": {
			&fakeInvokeTargets{apiContext: "/countries", endpoints: []string{"wss://gw.example.com"}}, failingRelay{},
			http.StatusUnprocessableEntity, apperror.CodeGraphQLAPIInvokeGatewayUnsupportedScheme,
		},
		"outside the allow list": {
			&fakeInvokeTargets{apiContext: "/countries", endpoints: []string{"https://internal.example.com:9443"}}, newRelay(t, "https://gw.example.com"),
			http.StatusForbidden, apperror.CodeGraphQLAPIInvokeTargetNotAllowed,
		},
		"busy": {
			&fakeInvokeTargets{apiContext: "/countries", endpoints: []string{"https://gw.example.com"}}, failingRelay{err: gatewayrelay.ErrBusy},
			http.StatusServiceUnavailable, apperror.CodeGraphQLAPIInvokeBusy,
		},
		"timeout": {
			&fakeInvokeTargets{apiContext: "/countries", endpoints: []string{"https://gw.example.com"}}, failingRelay{err: fmt.Errorf("relay: %w", context.DeadlineExceeded)},
			http.StatusGatewayTimeout, apperror.CodeGraphQLAPIInvokeUpstreamTimeout,
		},
		"unreachable": {
			&fakeInvokeTargets{apiContext: "/countries", endpoints: []string{"https://gw.example.com"}}, failingRelay{err: fmt.Errorf("dial tcp 10.0.0.7:9443: connection refused")},
			http.StatusBadGateway, apperror.CodeGraphQLAPIInvokeUpstreamUnreachable,
		},
	} {
		t.Run(name, func(t *testing.T) {
			rec := serveInvoke(t, tc.targets, tc.relay, "application/json", graphqlInvokeBody(""))
			assert.Equal(t, tc.status, rec.Code, rec.Body.String())
			assert.Equal(t, tc.code, errorCode(t, rec))
			assert.NotContains(t, rec.Body.String(), "10.0.0.7")
			assert.NotContains(t, rec.Body.String(), "internal.example.com")
		})
	}
}

// The route admits either kind's read or manage scope; each request needs its own kind's.
func TestInvokeNeedsTheReadOrManageScopeOfTheAPIsKind(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer gateway.Close()
	enforced := config.Authorization{Enabled: true, Mode: middleware.ValidationModeScope}

	for name, tc := range map[string]struct {
		scope  string
		body   string
		status int
	}{
		"REST API, its read scope":        {"ap:rest_api:read", invokeBody(""), http.StatusOK},
		"REST API, its manage scope":      {"ap:rest_api:manage", invokeBody(""), http.StatusOK},
		"REST API, GraphQL scopes only":   {"ap:graphql_api:read ap:graphql_api:manage", invokeBody(""), http.StatusForbidden},
		"GraphQL API, its read scope":     {"ap:graphql_api:read", graphqlInvokeBody(""), http.StatusOK},
		"GraphQL API, its manage scope":   {"ap:graphql_api:manage", graphqlInvokeBody(""), http.StatusOK},
		"GraphQL API, REST scopes only":   {"ap:rest_api:read ap:rest_api:manage", graphqlInvokeBody(""), http.StatusForbidden},
		"GraphQL API, an unrelated scope": {"ap:gateway:read", graphqlInvokeBody(""), http.StatusForbidden},
	} {
		t.Run(name, func(t *testing.T) {
			targets := &fakeInvokeTargets{apiContext: "/api", endpoints: []string{gateway.URL}}
			rec := serveInvokeAs(t, targets, newRelay(t, "*"), enforced, tc.scope, "application/json", tc.body)
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			if tc.status == http.StatusForbidden {
				assert.Equal(t, apperror.CodeCommonForbidden, errorCode(t, rec))
				assert.Empty(t, targets.gotAPI, "a refused caller learns nothing about the API")
			}
		})
	}
}

func TestInvokeChecksNoScopeWhileAuthorizationIsOff(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer gateway.Close()
	targets := &fakeInvokeTargets{apiContext: "/countries", endpoints: []string{gateway.URL}}

	rec := serveInvokeAs(t, targets, newRelay(t, "*"), config.Authorization{Enabled: false}, "", "application/json", graphqlInvokeBody(""))

	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
