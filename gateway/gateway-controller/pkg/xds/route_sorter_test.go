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
	"testing"

	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"github.com/stretchr/testify/assert"
)

func TestSortRoutesByPriority_PathMatchType(t *testing.T) {
	// Exact > Regex > Prefix
	exactRoute := &route.Route{
		Name: "exact",
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Path{Path: "/api/users"},
		},
	}
	regexRoute := &route.Route{
		Name: "regex",
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_SafeRegex{
				SafeRegex: &matcher.RegexMatcher{Regex: "^/api/users/[0-9]+$"},
			},
		},
	}
	prefixRoute := &route.Route{
		Name: "prefix",
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{Prefix: "/api"},
		},
	}

	routes := []*route.Route{prefixRoute, exactRoute, regexRoute}
	sorted := SortRoutesByPriority(routes)

	assert.Equal(t, "exact", sorted[0].Name, "Exact match should be first")
	assert.Equal(t, "regex", sorted[1].Name, "Regex match should be second")
	assert.Equal(t, "prefix", sorted[2].Name, "Prefix match should be last")
}

func TestSortRoutesByPriority_PathLength(t *testing.T) {
	// Longer paths should have higher priority
	shortPath := &route.Route{
		Name: "short",
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Path{Path: "/api"},
		},
	}
	longPath := &route.Route{
		Name: "long",
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Path{Path: "/api/users/profile"},
		},
	}

	routes := []*route.Route{shortPath, longPath}
	sorted := SortRoutesByPriority(routes)

	assert.Equal(t, "long", sorted[0].Name, "Longer path should be first")
	assert.Equal(t, "short", sorted[1].Name, "Shorter path should be second")
}

func TestSortRoutesByPriority_HeaderMatches(t *testing.T) {
	// More headers = higher priority
	noHeaders := &route.Route{
		Name: "no-headers",
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Path{Path: "/api"},
		},
	}
	oneHeader := &route.Route{
		Name: "one-header",
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Path{Path: "/api"},
			Headers: []*route.HeaderMatcher{
				{
					Name: "X-Custom",
					HeaderMatchSpecifier: &route.HeaderMatcher_StringMatch{
						StringMatch: &matcher.StringMatcher{
							MatchPattern: &matcher.StringMatcher_Exact{Exact: "value"},
						},
					},
				},
			},
		},
	}
	twoHeaders := &route.Route{
		Name: "two-headers",
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Path{Path: "/api"},
			Headers: []*route.HeaderMatcher{
				{
					Name: "X-Custom",
					HeaderMatchSpecifier: &route.HeaderMatcher_StringMatch{
						StringMatch: &matcher.StringMatcher{
							MatchPattern: &matcher.StringMatcher_Exact{Exact: "value"},
						},
					},
				},
				{
					Name: ":method",
					HeaderMatchSpecifier: &route.HeaderMatcher_StringMatch{
						StringMatch: &matcher.StringMatcher{
							MatchPattern: &matcher.StringMatcher_Exact{Exact: "GET"},
						},
					},
				},
			},
		},
	}

	routes := []*route.Route{noHeaders, twoHeaders, oneHeader}
	sorted := SortRoutesByPriority(routes)

	assert.Equal(t, "two-headers", sorted[0].Name, "Two headers should be first")
	assert.Equal(t, "one-header", sorted[1].Name, "One header should be second")
	assert.Equal(t, "no-headers", sorted[2].Name, "No headers should be last")
}

func TestSortRoutesByPriority_PrefixRootPath(t *testing.T) {
	// "/" prefix should have lowest priority (count = 0)
	rootPrefix := &route.Route{
		Name: "root",
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{Prefix: "/"},
		},
	}
	apiPrefix := &route.Route{
		Name: "api",
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{Prefix: "/api"},
		},
	}

	routes := []*route.Route{rootPrefix, apiPrefix}
	sorted := SortRoutesByPriority(routes)

	assert.Equal(t, "api", sorted[0].Name, "/api prefix should be first")
	assert.Equal(t, "root", sorted[1].Name, "/ prefix should be last")
}

func TestSortRoutesByPriority_StableSort(t *testing.T) {
	// Routes with same priority should maintain relative order
	route1 := &route.Route{
		Name: "route1",
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Path{Path: "/api"},
		},
	}
	route2 := &route.Route{
		Name: "route2",
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Path{Path: "/api"},
		},
	}

	routes := []*route.Route{route1, route2}
	sorted := SortRoutesByPriority(routes)

	// Stable sort should maintain original order for equal elements
	assert.Equal(t, "route1", sorted[0].Name)
	assert.Equal(t, "route2", sorted[1].Name)
}

func TestSortRoutesByPriority_EmptyAndSingleRoute(t *testing.T) {
	// Empty slice
	empty := SortRoutesByPriority([]*route.Route{})
	assert.Empty(t, empty)

	// Single route
	single := &route.Route{Name: "single"}
	result := SortRoutesByPriority([]*route.Route{single})
	assert.Len(t, result, 1)
	assert.Equal(t, "single", result[0].Name)
}

func TestSortRoutesByPriority_ComplexScenario(t *testing.T) {
	// A complex scenario mixing all criteria
	catchAll := &route.Route{
		Name: "catch-all",
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{Prefix: "/"},
		},
	}
	apiPrefix := &route.Route{
		Name: "api-prefix",
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{Prefix: "/api/v1"},
		},
	}
	exactUserPath := &route.Route{
		Name: "exact-user",
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Path{Path: "/api/v1/users"},
			Headers: []*route.HeaderMatcher{
				{
					Name: ":method",
					HeaderMatchSpecifier: &route.HeaderMatcher_StringMatch{
						StringMatch: &matcher.StringMatcher{
							MatchPattern: &matcher.StringMatcher_Exact{Exact: "GET"},
						},
					},
				},
			},
		},
	}
	regexUserPath := &route.Route{
		Name: "regex-user",
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_SafeRegex{
				SafeRegex: &matcher.RegexMatcher{Regex: "^/api/v1/users/[0-9]+$"},
			},
		},
	}

	routes := []*route.Route{catchAll, apiPrefix, regexUserPath, exactUserPath}
	sorted := SortRoutesByPriority(routes)

	// Expected order: exact with headers > regex > prefix (longer) > prefix (shorter/root)
	assert.Equal(t, "exact-user", sorted[0].Name, "Exact path with headers should be first")
	assert.Equal(t, "regex-user", sorted[1].Name, "Regex path should be second")
	assert.Equal(t, "api-prefix", sorted[2].Name, "Longer prefix should be third")
	assert.Equal(t, "catch-all", sorted[3].Name, "Root catch-all should be last")
}
