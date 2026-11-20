/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package xds

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func TestCaseInsensitiveMatch(t *testing.T) {
	method := []*route.HeaderMatcher{{
		Name: ":method",
		HeaderMatchSpecifier: &route.HeaderMatcher_StringMatch{
			StringMatch: &matcher.StringMatcher{MatchPattern: &matcher.StringMatcher_Exact{Exact: "GET"}},
		},
	}}

	t.Run("regex gains the case-insensitive flag and keeps its header matches", func(t *testing.T) {
		m := &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_SafeRegex{SafeRegex: &matcher.RegexMatcher{Regex: "^/shop/secure/data/?$"}},
			Headers:       method,
		}
		guard, ok := caseInsensitiveMatch(m)
		require.True(t, ok)
		assert.Equal(t, "(?i)^/shop/secure/data/?$", guard.GetSafeRegex().GetRegex())
		assert.Equal(t, "^/shop/secure/data/?$", m.GetSafeRegex().GetRegex(), "the route's own match must not change")
		require.Len(t, guard.GetHeaders(), 1)
		assert.Equal(t, "GET", guard.GetHeaders()[0].GetStringMatch().GetExact())
	})

	t.Run("exact path turns off case sensitivity", func(t *testing.T) {
		m := &route.RouteMatch{PathSpecifier: &route.RouteMatch_Path{Path: "/shop/exact"}}
		guard, ok := caseInsensitiveMatch(m)
		require.True(t, ok)
		assert.Equal(t, "/shop/exact", guard.GetPath())
		require.NotNil(t, guard.GetCaseSensitive())
		assert.False(t, guard.GetCaseSensitive().GetValue())
		assert.Nil(t, m.GetCaseSensitive(), "the route's own match must not change")
	})

	t.Run("no guard when it could match nothing new", func(t *testing.T) {
		for name, m := range map[string]*route.RouteMatch{
			"path without letters":           {PathSpecifier: &route.RouteMatch_SafeRegex{SafeRegex: &matcher.RegexMatcher{Regex: "^/[^/]+/?$"}}},
			"regex already case-insensitive": {PathSpecifier: &route.RouteMatch_SafeRegex{SafeRegex: &matcher.RegexMatcher{Regex: "(?i)^/shop$"}}},
			"prefix already case-insensitive": {
				PathSpecifier: &route.RouteMatch_Prefix{Prefix: "/shop"},
				CaseSensitive: wrapperspb.Bool(false),
			},
			"no path specifier": {},
		} {
			_, ok := caseInsensitiveMatch(m)
			assert.False(t, ok, name)
		}
	})
}

func TestTranslateConfigs_CaseVariantGuards(t *testing.T) {
	shop := makeRestAPIWithOperations("uuid-shop", "shop", "/shop", []api.Operation{
		{Method: api.Ptr(api.OperationMethodGET), Path: api.Ptr("/*")},
		{Method: api.Ptr(api.OperationMethodPOST), Path: api.Ptr("/*")},
		{Method: api.Ptr(api.OperationMethodGET), Path: api.Ptr("/secure/data")},
		{Method: api.Ptr(api.OperationMethodGET), Path: api.Ptr("/items/{id}")},
		{Method: api.Ptr(api.OperationMethodGET), Path: api.Ptr("/users")},
		{Method: api.Ptr(api.OperationMethodGET), Path: api.Ptr("/Users")},
	})
	translate := func(t *testing.T, allowCaseVariants bool) []*route.Route {
		t.Helper()
		routerCfg := testRouterConfig()
		routerCfg.HTTPListener.AllowPathCaseVariants = allowCaseVariants
		translator, err := NewTranslator(createTestLogger(), routerCfg, nil, testConfig())
		require.NoError(t, err)
		resources, err := translator.TranslateConfigs([]*models.StoredConfig{shop}, "test-correlation-id")
		require.NoError(t, err)
		return apiVHostRoutes(t, resources, "localhost")
	}

	t.Run("a case variant of a route's path gets a 404 instead of the broader route", func(t *testing.T) {
		routes := translate(t, false)
		tests := []struct {
			method, path, want string
		}{
			{"GET", "/shop/secure/data", "GET|/shop/secure/data"},
			{"GET", "/shop/Secure/data", "guard"},
			{"GET", "/shop/SECURE/DATA", "guard"},
			{"GET", "/shop/other", "GET|/shop/*"},
			{"GET", "/shop/items/Abc", "GET|/shop/items/{id}"},
			{"GET", "/shop/Items/abc", "guard"},
			// A route of equal specificity spelt that way wins over the other's guard.
			{"GET", "/shop/users", "GET|/shop/users"},
			{"GET", "/shop/Users", "GET|/shop/Users"},
			{"GET", "/shop/USERS", "guard"},
			// The guard matches the route's own methods only.
			{"POST", "/shop/Secure/data", "POST|/shop/*"},
			// The API context is part of the guarded path.
			{"GET", "/SHOP/other", "guard"},
			{"GET", "/elsewhere", "no-api-found"},
		}
		for _, tt := range tests {
			got := firstMatchingRoute(t, routes, tt.method, tt.path)
			require.NotNil(t, got, "%s %s matched no route", tt.method, tt.path)
			if tt.want == "guard" {
				assert.True(t, isCaseVariantGuard(got), "%s %s: want a case-variant guard, got %q", tt.method, tt.path, got.GetName())
				assert.Equal(t, uint32(404), got.GetDirectResponse().GetStatus())
				assert.Contains(t, got.GetTypedPerFilterConfig(), constants.ExtProcFilterName,
					"the guard must not consult the policy engine")
				continue
			}
			assert.Equal(t, tt.want, strings.SplitN(got.GetName(), "|localhost", 2)[0], "%s %s", tt.method, tt.path)
		}
	})

	t.Run("allow_path_case_variants keeps the previous fall-through", func(t *testing.T) {
		routes := translate(t, true)
		for _, r := range routes {
			assert.False(t, isCaseVariantGuard(r), "unexpected guard %q", r.GetName())
		}
		got := firstMatchingRoute(t, routes, "GET", "/shop/Secure/data")
		require.NotNil(t, got)
		assert.Equal(t, "GET|/shop/*", strings.SplitN(got.GetName(), "|localhost", 2)[0])
	})
}

func makeRestAPIWithOperations(uuid, name, ctx string, ops []api.Operation) *models.StoredConfig {
	cfg := makeRestAPI(uuid, name, ctx)
	rest := cfg.Configuration.(api.RestAPI)
	rest.Spec.Operations = ops
	cfg.Configuration = rest
	cfg.SourceConfiguration = rest
	return cfg
}

func apiVHostRoutes(t *testing.T, resources map[resource.Type][]types.Resource, vhost string) []*route.Route {
	t.Helper()
	for _, res := range resources[resource.RouteType] {
		for _, vh := range res.(*route.RouteConfiguration).VirtualHosts {
			if vh.Name == vhost {
				return vh.Routes
			}
		}
	}
	require.Failf(t, "virtual host not found", "%q", vhost)
	return nil
}

// firstMatchingRoute picks the route Envoy would, from the path and :method matchers the
// translator emits: the first, in order, whose matchers all accept the request.
func firstMatchingRoute(t *testing.T, routes []*route.Route, method, path string) *route.Route {
	t.Helper()
	for _, r := range routes {
		m := r.GetMatch()
		caseSensitive := m.GetCaseSensitive() == nil || m.GetCaseSensitive().GetValue()
		fold := func(s string) string {
			if caseSensitive {
				return s
			}
			return strings.ToLower(s)
		}
		var pathOK bool
		switch ps := m.GetPathSpecifier().(type) {
		case *route.RouteMatch_SafeRegex:
			pathOK = regexp.MustCompile(ps.SafeRegex.GetRegex()).MatchString(path)
		case *route.RouteMatch_Path:
			pathOK = fold(path) == fold(ps.Path)
		case *route.RouteMatch_Prefix:
			pathOK = strings.HasPrefix(fold(path), fold(ps.Prefix))
		}
		if !pathOK {
			continue
		}
		headersOK := true
		for _, h := range m.GetHeaders() {
			if h.GetName() == ":method" && h.GetStringMatch().GetExact() != method {
				headersOK = false
			}
		}
		if headersOK {
			return r
		}
	}
	return nil
}

func TestTranslateConfigs_CaseVariantGuardsOnlyWhereNeeded(t *testing.T) {
	translate := func(t *testing.T, apis ...*models.StoredConfig) []*route.Route {
		t.Helper()
		translator, err := NewTranslator(createTestLogger(), testRouterConfig(), nil, testConfig())
		require.NoError(t, err)
		resources, err := translator.TranslateConfigs(apis, "test-correlation-id")
		require.NoError(t, err)
		return apiVHostRoutes(t, resources, "localhost")
	}
	guards := func(routes []*route.Route) []*route.Route {
		var out []*route.Route
		for _, r := range routes {
			if isCaseVariantGuard(r) {
				out = append(out, r)
			}
		}
		return out
	}

	t.Run("no guard when no route could catch a variant", func(t *testing.T) {
		routes := translate(t, makeRestAPIWithOperations("uuid-plain", "plain", "/plain", []api.Operation{
			{Method: api.Ptr(api.OperationMethodGET), Path: api.Ptr("/secure/data")},
			{Method: api.Ptr(api.OperationMethodGET), Path: api.Ptr("/users")},
		}))
		assert.Empty(t, guards(routes))
		got := firstMatchingRoute(t, routes, "GET", "/plain/Secure/data")
		require.NotNil(t, got)
		assert.Equal(t, "no-api-found", got.GetName(), "a variant nothing matches still gets the vhost's 404")
	})

	t.Run("a broader route of another API counts", func(t *testing.T) {
		routes := translate(t,
			makeRestAPIWithOperations("uuid-outer", "outer", "/outer", []api.Operation{
				{Method: api.Ptr(api.OperationMethodGET), Path: api.Ptr("/*")},
			}),
			makeRestAPIWithOperations("uuid-inner", "inner", "/outer/inner", []api.Operation{
				{Method: api.Ptr(api.OperationMethodGET), Path: api.Ptr("/secure/data")},
			}),
		)
		got := firstMatchingRoute(t, routes, "GET", "/outer/inner/Secure/data")
		require.NotNil(t, got)
		assert.True(t, isCaseVariantGuard(got), "got %q", got.GetName())
	})

	t.Run("routes of equal specificity share one guard", func(t *testing.T) {
		ops := []api.Operation{{Method: api.Ptr(api.OperationMethodGET), Path: api.Ptr("/*")}}
		for _, p := range []string{"/secure/data", "/alerts/active", "/orders/open"} {
			ops = append(ops, api.Operation{Method: api.Ptr(api.OperationMethodGET), Path: api.Ptr(p)})
		}
		routes := translate(t, makeRestAPIWithOperations("uuid-shared", "shared", "/shared", ops))
		require.Len(t, guards(routes), 1)
		for _, p := range []string{"/secure/data", "/alerts/active", "/orders/open"} {
			got := firstMatchingRoute(t, routes, "GET", "/shared"+p)
			require.NotNil(t, got)
			assert.Equal(t, "GET|/shared"+p, strings.SplitN(got.GetName(), "|localhost", 2)[0])
			got = firstMatchingRoute(t, routes, "GET", "/shared"+strings.ToUpper(p))
			require.NotNil(t, got)
			assert.True(t, isCaseVariantGuard(got), "%s: got %q", p, got.GetName())
		}
	})

	t.Run("a large API's guards stay under the regex size budget", func(t *testing.T) {
		ops := []api.Operation{{Method: api.Ptr(api.OperationMethodGET), Path: api.Ptr("/*")}}
		for i := 0; i < 60; i++ {
			ops = append(ops, api.Operation{Method: api.Ptr(api.OperationMethodGET), Path: api.Ptr(fmt.Sprintf("/resource/item%02d", i))})
		}
		routes := translate(t, makeRestAPIWithOperations("uuid-large", "large", "/large", ops))
		g := guards(routes)
		require.Greater(t, len(g), 1, "60 paths should not fit one guard")
		assert.Less(t, len(g), 60, "guards should be shared")
		for _, r := range g {
			assert.LessOrEqual(t, regexProgramSize(r.GetMatch().GetSafeRegex().GetRegex()), caseVariantGuardProgramBudget, r.GetName())
		}
		for i := 0; i < 60; i++ {
			got := firstMatchingRoute(t, routes, "GET", fmt.Sprintf("/large/RESOURCE/item%02d", i))
			require.NotNil(t, got)
			assert.True(t, isCaseVariantGuard(got), "item%02d: got %q", i, got.GetName())
		}
	})
}

func TestTranslateConfigs_CaseVariantGuardForRootOperation(t *testing.T) {
	translator, err := NewTranslator(createTestLogger(), testRouterConfig(), nil, testConfig())
	require.NoError(t, err)
	resources, err := translator.TranslateConfigs([]*models.StoredConfig{
		makeRestAPIWithOperations("uuid-tenant", "tenant", "/", []api.Operation{
			{Method: api.Ptr(api.OperationMethodGET), Path: api.Ptr("/{tenant}")},
		}),
		makeRestAPIWithOperations("uuid-shop", "shop", "/shop", []api.Operation{
			{Method: api.Ptr(api.OperationMethodGET), Path: api.Ptr("/")},
		}),
	}, "test-correlation-id")
	require.NoError(t, err)
	routes := apiVHostRoutes(t, resources, "localhost")

	for _, path := range []string{"/shop", "/shop/"} {
		got := firstMatchingRoute(t, routes, "GET", path)
		require.NotNil(t, got, path)
		assert.Equal(t, "GET|/shop/", strings.SplitN(got.GetName(), "|localhost", 2)[0], path)
	}
	got := firstMatchingRoute(t, routes, "GET", "/SHOP")
	require.NotNil(t, got)
	assert.True(t, isCaseVariantGuard(got), "GET /SHOP: got %q", got.GetName())
	got = firstMatchingRoute(t, routes, "GET", "/acme")
	require.NotNil(t, got)
	assert.Equal(t, "GET|/{tenant}", strings.SplitN(got.GetName(), "|localhost", 2)[0])
}

func TestSegmentsMayOverlap(t *testing.T) {
	for _, tt := range []struct {
		a, b string
		want bool
	}{
		{"/shop/*", "/shop/secure/data", true},
		{"/SHOP/*", "/shop/secure/data", true},
		{"/other/*", "/shop/secure/data", false},
		{"/shop/{id}", "/shop/secure", true},
		{"/shop/{id}", "/shop/secure/data", false},
		{"/shop/v{n}/x", "/shop/v1/x", true},
		{"/shop/items/{id}", "/shop/orders/{id}", false},
		// A trailing slash is optional in a route's match.
		{"/{tenant}", "/shop/", true},
		{"/shop/{id}/", "/shop/data", true},
		{"/{a}/{b}", "/shop/", false},
	} {
		assert.Equal(t, tt.want, segmentsMayOverlap(pathSegments(tt.a), pathSegments(tt.b)), "%s vs %s", tt.a, tt.b)
	}
}

func TestAppendCaseVariantGuards_LongExactPathKeepsExactMatcher(t *testing.T) {
	method := []*route.HeaderMatcher{{
		Name: ":method",
		HeaderMatchSpecifier: &route.HeaderMatcher_StringMatch{
			StringMatch: &matcher.StringMatcher{MatchPattern: &matcher.StringMatcher_Exact{Exact: "GET"}},
		},
	}}
	longPath := "/shop/" + strings.Repeat("segment", 60)
	routes := []*route.Route{
		{
			Name:  "GET|" + longPath + "|localhost",
			Match: &route.RouteMatch{PathSpecifier: &route.RouteMatch_Path{Path: longPath}, Headers: method},
		},
		{
			Name: "GET|/shop/*|localhost",
			Match: &route.RouteMatch{
				PathSpecifier: &route.RouteMatch_SafeRegex{SafeRegex: &matcher.RegexMatcher{Regex: "^/shop(?:/.*)?$"}},
				Headers:       method,
			},
		},
	}
	require.Greater(t, regexProgramSize("(?i)^"+regexp.QuoteMeta(longPath)+"$"), caseVariantGuardProgramBudget)

	guarded, err := appendCaseVariantGuards(routes)
	require.NoError(t, err)
	var guards []*route.Route
	for _, r := range guarded {
		if isCaseVariantGuard(r) {
			guards = append(guards, r)
		}
	}
	require.Len(t, guards, 1)
	m := guards[0].GetMatch()
	assert.Equal(t, longPath, m.GetPath(), "the guard should use the exact matcher, not a regex")
	require.NotNil(t, m.GetCaseSensitive())
	assert.False(t, m.GetCaseSensitive().GetValue())
	assert.Nil(t, m.GetSafeRegex())
	require.Len(t, m.GetHeaders(), 1)
}
