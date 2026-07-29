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
	"strings"

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
// 2. Path segment specificity (compared left-to-right)
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

	// 2. Compare semantic path segments from left to right. An earlier literal constraint
	// must not be outweighed by longer literals in a later segment.
	if specificity := comparePathSpecificity(x[i], x[j]); specificity != 0 {
		return specificity < 0
	}

	// Equal path specificity case

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

// comparePathSpecificity returns a positive value when left is more specific, a negative value
// when right is more specific, and zero when they are equally specific. Product routes carry their
// semantic path in the route name (METHOD|PATH|VHOST, optionally followed by a discriminator), so
// compare those paths directly. Routes outside that contract retain the historical matcher-length
// fallback.
func comparePathSpecificity(left, right *route.Route) int {
	leftPath, leftOK := semanticPathFromRouteName(left.GetName())
	rightPath, rightOK := semanticPathFromRouteName(right.GetName())
	if leftOK && rightOK {
		return compareSemanticPaths(leftPath, rightPath)
	}

	return compareInts(pathMatchCount(left.GetMatch()), pathMatchCount(right.GetMatch()))
}

func semanticPathFromRouteName(name string) (string, bool) {
	parts := strings.SplitN(name, "|", 4)
	if len(parts) < 3 || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

// pathMatchCount returns the historical matcher-length score used for routes that do not carry a
// semantic path in their name.
func pathMatchCount(match *route.RouteMatch) int {
	switch ps := match.GetPathSpecifier().(type) {
	case *route.RouteMatch_Path:
		return len(ps.Path)
	case *route.RouteMatch_SafeRegex:
		return len(ps.SafeRegex.GetRegex())
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

type semanticSegmentKind int

const (
	// Wildcards have the lowest priority because they can consume any remaining segments.
	semanticSegmentWildcard semanticSegmentKind = iota
	// A terminal marker beats a wildcard rooted at the same path (/a/b before /a/b/*), but a
	// further constrained segment remains more specific than ending the path.
	semanticSegmentTerminal
	semanticSegmentParameter
	semanticSegmentMixed
	semanticSegmentLiteral
)

type semanticSegment struct {
	kind         semanticSegmentKind
	literalCount int
}

// compareSemanticPaths compares slash-delimited path segments from left to right. Position is
// significant: a literal in an earlier segment beats a parameter in that segment regardless of
// how many literal characters appear later in the competing path. Parameter names do not affect
// specificity.
func compareSemanticPaths(left, right string) int {
	leftSegments := splitSemanticPath(left)
	rightSegments := splitSemanticPath(right)
	segmentCount := max(len(leftSegments), len(rightSegments))
	terminalSegment := semanticSegment{kind: semanticSegmentTerminal}

	for i := 0; i < segmentCount; i++ {
		leftSegment := terminalSegment
		if i < len(leftSegments) {
			leftSegment = leftSegments[i]
		}
		rightSegment := terminalSegment
		if i < len(rightSegments) {
			rightSegment = rightSegments[i]
		}

		if comparison := compareInts(int(leftSegment.kind), int(rightSegment.kind)); comparison != 0 {
			return comparison
		}
		if leftSegment.kind == semanticSegmentMixed {
			if comparison := compareInts(leftSegment.literalCount, rightSegment.literalCount); comparison != 0 {
				return comparison
			}
		}
	}

	return 0
}

func splitSemanticPath(path string) []semanticSegment {
	rawSegments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	segments := make([]semanticSegment, 0, len(rawSegments))
	for _, rawSegment := range rawSegments {
		segments = append(segments, classifySemanticSegment(rawSegment))
	}
	return segments
}

func classifySemanticSegment(segment string) semanticSegment {
	if segment == "*" {
		return semanticSegment{kind: semanticSegmentWildcard}
	}

	literalCount, parameterCount, inParameter := 0, 0, false
	for i := 0; i < len(segment); i++ {
		switch segment[i] {
		case '{':
			if !inParameter {
				parameterCount++
				inParameter = true
			}
		case '}':
			inParameter = false
		default:
			if !inParameter {
				literalCount++
			}
		}
	}

	switch {
	case parameterCount == 0:
		return semanticSegment{kind: semanticSegmentLiteral, literalCount: literalCount}
	case literalCount == 0:
		return semanticSegment{kind: semanticSegmentParameter}
	default:
		return semanticSegment{kind: semanticSegmentMixed, literalCount: literalCount}
	}
}

func compareInts(left, right int) int {
	switch {
	case left > right:
		return 1
	case left < right:
		return -1
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
