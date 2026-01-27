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
	"sort"

	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
)

type EnvoyRoutes []*route.Route

func (x EnvoyRoutes) Len() int      { return len(x) }
func (x EnvoyRoutes) Swap(i, j int) { x[i], x[j] = x[j], x[i] }

// Returns true if route i should come AFTER route j (for ascending order).
// Since we use sort.Reverse for descending order, this effectively means
// route i has LOWER priority than route j when Less returns true.
//
// Priority order (highest to lowest):
// 1. Path match type: Exact > Regex > Prefix
// 2. Path length (longer paths have higher priority)
// 3. Number of header matches (more headers = higher priority)
// 4. Number of exact header matches
// 5. Number of query parameter matches
// 6. Number of exact query parameter matches
func (x EnvoyRoutes) Less(i, j int) bool {
	matchI := x[i].GetMatch()
	matchJ := x[j].GetMatch()

	// 1. Sort based on path match type
	// Exact > Regex > Prefix
	pathTypeI := getPathMatchType(matchI)
	pathTypeJ := getPathMatchType(matchJ)

	if pathTypeI != pathTypeJ {
		// Lower pathType value means higher priority (Exact=0, Regex=1, Prefix=2)
		// Less returns true if i should come after j
		// So if pathTypeI > pathTypeJ, i has lower priority and should come after j
		return pathTypeI > pathTypeJ
	}

	// Equal path match type case

	// 2. Sort based on characters in a matching path.
	// Longer paths have higher priority
	pCountI := pathMatchCount(matchI)
	pCountJ := pathMatchCount(matchJ)
	if pCountI != pCountJ {
		// More characters = higher priority
		// If pCountI < pCountJ, i has lower priority
		return pCountI < pCountJ
	}

	// Equal path count case

	// 3. Sort based on the number of Header matches.
	// When the number is same, sort based on number of Exact Header matches.
	hCountI := len(matchI.GetHeaders())
	hCountJ := len(matchJ.GetHeaders())
	if hCountI != hCountJ {
		// More headers = higher priority
		return hCountI < hCountJ
	}

	hExactI := numberOfExactHeaderMatches(matchI.GetHeaders())
	hExactJ := numberOfExactHeaderMatches(matchJ.GetHeaders())
	if hExactI != hExactJ {
		// More exact matches = higher priority
		return hExactI < hExactJ
	}

	// Equal header case

	// 4. Sort based on the number of Query param matches.
	// When the number is same, sort based on number of Exact Query param matches.
	qCountI := len(matchI.GetQueryParameters())
	qCountJ := len(matchJ.GetQueryParameters())
	if qCountI != qCountJ {
		// More query params = higher priority
		return qCountI < qCountJ
	}

	qExactI := numberOfExactQueryParamMatches(matchI.GetQueryParameters())
	qExactJ := numberOfExactQueryParamMatches(matchJ.GetQueryParameters())
	return qExactI < qExactJ
}

// pathMatchType represents the type of path matching.
// Lower values indicate higher priority.
const (
	pathMatchTypeExact  = 0
	pathMatchTypeRegex  = 1
	pathMatchTypePrefix = 2
	pathMatchTypeNone   = 3
)

// getPathMatchType returns the path match type for a route match.
// Returns a numeric value where lower = higher priority.
func getPathMatchType(match *route.RouteMatch) int {
	if match == nil {
		return pathMatchTypeNone
	}

	switch match.GetPathSpecifier().(type) {
	case *route.RouteMatch_Path:
		return pathMatchTypeExact
	case *route.RouteMatch_SafeRegex:
		return pathMatchTypeRegex
	case *route.RouteMatch_Prefix:
		return pathMatchTypePrefix
	default:
		return pathMatchTypeNone
	}
}

// pathMatchCount returns the length of the path pattern.
// Longer paths have higher priority as they are more specific.
func pathMatchCount(match *route.RouteMatch) int {
	if match == nil {
		return 0
	}

	switch ps := match.GetPathSpecifier().(type) {
	case *route.RouteMatch_Path:
		return len(ps.Path)
	case *route.RouteMatch_SafeRegex:
		if ps.SafeRegex != nil {
			return len(ps.SafeRegex.GetRegex())
		}
		return 0
	case *route.RouteMatch_Prefix:
		// Special case: "/" prefix should have 0 count
		// as it matches all paths which equals to no path match
		if ps.Prefix == "/" {
			return 0
		}
		return len(ps.Prefix)
	default:
		return 0
	}
}

// numberOfExactHeaderMatches counts headers that use exact string matching.
func numberOfExactHeaderMatches(headers []*route.HeaderMatcher) int {
	var count int
	for _, header := range headers {
		if header == nil {
			continue
		}
		if sm := header.GetStringMatch(); sm != nil {
			if sm.GetExact() != "" {
				count++
			}
		}
	}
	return count
}

// numberOfExactQueryParamMatches counts query parameters that use exact string matching.
func numberOfExactQueryParamMatches(queryParams []*route.QueryParameterMatcher) int {
	var count int
	for _, qp := range queryParams {
		if qp == nil {
			continue
		}
		if sm := qp.GetStringMatch(); sm != nil {
			if sm.GetExact() != "" {
				count++
			}
		}
	}
	return count
}

// SortRoutesByPriority sorts routes by match precedence in descending order
func SortRoutesByPriority(routes []*route.Route) []*route.Route {
	if len(routes) <= 1 {
		return routes
	}
	// Sort in descending order (highest priority first)
	sort.Stable(sort.Reverse(EnvoyRoutes(routes)))
	return routes
}
