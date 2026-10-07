/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package testproxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// platformStub stands in for the Platform API, answering the two reads the
// resolver performs and recording what it was asked with.
type platformStub struct {
	apiJSON      string
	gatewaysJSON string
	status       int

	calls      atomic.Int64
	lastAuth   string
	lastOrg    string
	lastPaths  []string
	perTokenOK map[string]bool
}

func (p *platformStub) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.calls.Add(1)
		p.lastAuth = r.Header.Get("Authorization")
		p.lastOrg = r.Header.Get("X-Org-Id")
		p.lastPaths = append(p.lastPaths, r.URL.Path)

		if p.perTokenOK != nil && !p.perTokenOK[strings.TrimPrefix(p.lastAuth, "Bearer ")] {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if p.status != 0 {
			w.WriteHeader(p.status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/gateways") {
			_, _ = w.Write([]byte(p.gatewaysJSON))
			return
		}
		_, _ = w.Write([]byte(p.apiJSON))
	})
}

func newTestResolver(t *testing.T, stub *platformStub, ttl time.Duration) (*Resolver, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)
	return NewResolver(srv.Client(), srv.URL, "/api/v0.9", ttl, 16), srv
}

const deployedGatewayJSON = `{"list":[
  {"id":"gw-prod","organizationId":"acme","endpoints":["https://gw.example.com:8443"],"isDeployed":true}
]}`

func TestResolveBuildsTheInvokeURLFromPlatformAPI(t *testing.T) {
	stub := &platformStub{apiJSON: `{"context":"/pizza/v1"}`, gatewaysJSON: deployedGatewayJSON}
	r, _ := newTestResolver(t, stub, time.Minute)

	target, err := r.Resolve(context.Background(), "tok", "acme", "api-1", "gw-prod")
	require.NoError(t, err)

	// The URL the relay dials is derived here, never taken from the browser.
	assert.Equal(t, "https://gw.example.com:8443/pizza/v1", target.URL.String())
	assert.Equal(t, "gw-prod", target.GatewayID)

	// The caller's own token is what establishes entitlement: Platform API is
	// the authorization boundary, not this package.
	assert.Equal(t, "Bearer tok", stub.lastAuth)
	assert.Equal(t, "acme", stub.lastOrg)
	assert.Equal(t, []string{"/api/v0.9/rest-apis/api-1", "/api/v0.9/rest-apis/api-1/gateways"}, stub.lastPaths)
}

func TestResolveRefusesAGatewayTheAPIIsNotDeployedTo(t *testing.T) {
	// The console only offers deployed gateways. Accepting an undeployed one
	// would let a caller aim the relay at any gateway merely associated with
	// the API.
	stub := &platformStub{
		apiJSON:      `{"context":"/pizza"}`,
		gatewaysJSON: `{"list":[{"id":"gw-prod","endpoints":["https://gw.example.com"],"isDeployed":false}]}`,
	}
	r, _ := newTestResolver(t, stub, time.Minute)

	_, err := r.Resolve(context.Background(), "tok", "acme", "api-1", "gw-prod")
	assert.ErrorIs(t, err, ErrGatewayNotFound)
}

func TestResolveAcceptsAGatewayWithADeploymentRecord(t *testing.T) {
	// The SPA treats either signal as deployed, so the server must not refuse a
	// gateway the picker just offered.
	stub := &platformStub{
		apiJSON:      `{"context":"/pizza"}`,
		gatewaysJSON: `{"list":[{"id":"gw-prod","endpoints":["https://gw.example.com"],"isDeployed":false,"deployment":{"status":"CREATED"}}]}`,
	}
	r, _ := newTestResolver(t, stub, time.Minute)

	target, err := r.Resolve(context.Background(), "tok", "acme", "api-1", "gw-prod")
	require.NoError(t, err)
	assert.Equal(t, "https://gw.example.com/pizza", target.URL.String())
}

func TestResolveRefusesAGatewayFromAnotherOrganization(t *testing.T) {
	stub := &platformStub{
		apiJSON:      `{"context":"/pizza"}`,
		gatewaysJSON: `{"list":[{"id":"gw-prod","organizationId":"other-tenant","endpoints":["https://gw.example.com"],"isDeployed":true}]}`,
	}
	r, _ := newTestResolver(t, stub, time.Minute)

	_, err := r.Resolve(context.Background(), "tok", "acme", "api-1", "gw-prod")
	assert.ErrorIs(t, err, ErrGatewayOtherOrg)
}

func TestResolveRefusesAnUnknownGateway(t *testing.T) {
	stub := &platformStub{apiJSON: `{"context":"/pizza"}`, gatewaysJSON: deployedGatewayJSON}
	r, _ := newTestResolver(t, stub, time.Minute)

	_, err := r.Resolve(context.Background(), "tok", "acme", "api-1", "gw-someone-elses")
	assert.ErrorIs(t, err, ErrGatewayNotFound)
}

func TestResolveCollapsesEveryPlatformAPIDenialToOneOutcome(t *testing.T) {
	// 401, 403 and 404 all mean the same thing here — the caller does not get
	// to use this API — and distinguishing them would tell a caller whether an
	// API id exists.
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			stub := &platformStub{status: status}
			r, _ := newTestResolver(t, stub, time.Minute)
			_, err := r.Resolve(context.Background(), "tok", "acme", "api-1", "gw-prod")
			assert.ErrorIs(t, err, ErrAPINotFound)
		})
	}
}

func TestResolveRefusesAGatewayWithNoUsableEndpoint(t *testing.T) {
	stub := &platformStub{
		apiJSON:      `{"context":"/pizza"}`,
		gatewaysJSON: `{"list":[{"id":"gw-prod","endpoints":[],"isDeployed":true}]}`,
	}
	r, _ := newTestResolver(t, stub, time.Minute)

	_, err := r.Resolve(context.Background(), "tok", "acme", "api-1", "gw-prod")
	assert.ErrorIs(t, err, ErrGatewayNoEndpoint)
}

func TestResolveRefusesANonHTTPEndpointScheme(t *testing.T) {
	// A gateway record can carry a wss:// endpoint; the relay speaks HTTP only.
	stub := &platformStub{
		apiJSON:      `{"context":"/events"}`,
		gatewaysJSON: `{"list":[{"id":"gw-prod","endpoints":["wss://events.example.com:8444"],"isDeployed":true}]}`,
	}
	r, _ := newTestResolver(t, stub, time.Minute)

	_, err := r.Resolve(context.Background(), "tok", "acme", "api-1", "gw-prod")
	assert.ErrorIs(t, err, ErrUnsupportedScheme)
}

func TestResolveCachesPerCallerNotPerTarget(t *testing.T) {
	// A cache keyed by (org, api, gateway) alone would let one caller's
	// successful resolution serve a second caller who is not entitled to that
	// API — a confused deputy with a TTL.
	stub := &platformStub{
		apiJSON:      `{"context":"/pizza"}`,
		gatewaysJSON: deployedGatewayJSON,
		perTokenOK:   map[string]bool{"entitled": true},
	}
	r, _ := newTestResolver(t, stub, time.Minute)

	_, err := r.Resolve(context.Background(), "entitled", "acme", "api-1", "gw-prod")
	require.NoError(t, err)

	_, err = r.Resolve(context.Background(), "not-entitled", "acme", "api-1", "gw-prod")
	assert.ErrorIs(t, err, ErrAPINotFound, "the second caller must be checked, not served from the first's entry")
}

func TestResolveServesTheSameCallerFromCache(t *testing.T) {
	stub := &platformStub{apiJSON: `{"context":"/pizza"}`, gatewaysJSON: deployedGatewayJSON}
	r, _ := newTestResolver(t, stub, time.Minute)

	_, err := r.Resolve(context.Background(), "tok", "acme", "api-1", "gw-prod")
	require.NoError(t, err)
	after := stub.calls.Load()

	_, err = r.Resolve(context.Background(), "tok", "acme", "api-1", "gw-prod")
	require.NoError(t, err)
	assert.Equal(t, after, stub.calls.Load(), "a repeat try-out should not re-query Platform API")
}

func TestResolveReChecksOnceTheCacheEntryExpires(t *testing.T) {
	// The TTL is also the window in which a just-revoked entitlement still
	// resolves, so it has to actually expire.
	stub := &platformStub{apiJSON: `{"context":"/pizza"}`, gatewaysJSON: deployedGatewayJSON}
	r, _ := newTestResolver(t, stub, time.Minute)

	now := time.Now()
	r.nowFunc = func() time.Time { return now }

	_, err := r.Resolve(context.Background(), "tok", "acme", "api-1", "gw-prod")
	require.NoError(t, err)
	after := stub.calls.Load()

	now = now.Add(2 * time.Minute)
	_, err = r.Resolve(context.Background(), "tok", "acme", "api-1", "gw-prod")
	require.NoError(t, err)
	assert.Greater(t, stub.calls.Load(), after)
}

func TestBuildTargetComposesEndpointAndContext(t *testing.T) {
	for _, tc := range []struct {
		name     string
		endpoint string
		context  string
		want     string
	}{
		{"plain", "https://gw.example.com", "/pizza", "https://gw.example.com/pizza"},
		{"trailing slashes trimmed", "https://gw.example.com/", "/pizza/", "https://gw.example.com/pizza"},
		{"endpoint carries a base path", "https://gw.example.com/api/v1", "/pizza", "https://gw.example.com/api/v1/pizza"},
		{"root context", "https://gw.example.com", "/", "https://gw.example.com"},
		{"empty context defaults to root", "https://gw.example.com", "", "https://gw.example.com"},
		// The spec types endpoints as URLs, but the SPA tolerates a bare host
		// and assumes https; refusing here would reject a record the console
		// would have rendered happily.
		{"bare host assumes https", "gw.example.com:8443", "/pizza", "https://gw.example.com:8443/pizza"},
		{"plain http is honoured", "http://gw.internal:9090", "/pizza", "http://gw.internal:9090/pizza"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target, err := buildTarget(gateway{ID: "gw", Endpoints: []string{tc.endpoint}}, tc.context)
			require.NoError(t, err)
			assert.Equal(t, tc.want, target.URL.String())
		})
	}
}

func TestBuildTargetDropsCredentialsAndQueryFromTheEndpoint(t *testing.T) {
	// None of these belongs on an invoke URL, and userinfo in particular would
	// otherwise be attached to every relayed request.
	target, err := buildTarget(gateway{
		ID:        "gw",
		Endpoints: []string{"https://user:pass@gw.example.com/base?a=1#frag"},
	}, "/pizza")
	require.NoError(t, err)
	assert.Equal(t, "https://gw.example.com/base/pizza", target.URL.String())
}
