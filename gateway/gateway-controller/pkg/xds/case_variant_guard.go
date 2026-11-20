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
	"math"
	"regexp"
	"regexp/syntax"
	"sort"
	"strings"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	extproc "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"google.golang.org/protobuf/proto"
	anypb "google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

// caseVariantGuardSuffix ends the name of a route that refuses case variants of another route's
// path. The guard keeps its route's METHOD|PATH|VHOST prefix so it sorts and groups with it.
const caseVariantGuardSuffix = "|case-variant-guard"

// caseVariantGuardProgramBudget bounds a combined guard regex. Envoy rejects the whole route
// configuration when one regex exceeds re2.max_program_size (400 in the gateway runtime), and the
// size measured here is Go's compiled program, not RE2's, so it stays well under that.
const caseVariantGuardProgramBudget = 200

func isCaseVariantGuard(r *route.Route) bool {
	return strings.HasSuffix(r.GetName(), caseVariantGuardSuffix)
}

// appendCaseVariantGuards adds 404 routes for case variants of route paths. Route paths match
// case-sensitively, so without them a request for "/Secure/data" misses "/secure/data" and reaches
// a broader route such as "/*" without the policies of the route it was aimed at.
//
// A route is guarded only when another route could catch one of its variants: a variant nothing
// else matches already gets the vhost's 404. Guarded routes of equal path specificity and equal
// non-path matchers share one case-insensitive regex (split to stay under Envoy's regex size cap),
// because every guard is a regex Envoy evaluates in turn for each request. SortRoutesByPriority
// places a guard after every route of equal path specificity, so a route whose path really is
// "/Secure/data" still wins.
func appendCaseVariantGuards(routes []*route.Route) ([]*route.Route, error) {
	catchers := newVariantCatchers(routes)
	groups := map[string][]guardMember{}
	var keys []string
	guarded := routes
	for _, r := range routes {
		if r.GetName() == "" || !catchers.catchVariantOf(r) {
			continue
		}
		member, key, ok := combinableGuardMember(r)
		if !ok {
			match, ok := caseInsensitiveMatch(r.GetMatch())
			if !ok {
				continue
			}
			guard, err := buildNotFoundRoute(r.GetName()+caseVariantGuardSuffix, match)
			if err != nil {
				return nil, err
			}
			guarded = append(guarded, guard)
			continue
		}
		if _, seen := groups[key]; !seen {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], member)
	}

	sort.Strings(keys)
	for _, key := range keys {
		members := groups[key]
		sort.Slice(members, func(i, j int) bool { return members[i].body < members[j].body })
		for _, chunk := range chunkGuardMembers(members) {
			match := proto.Clone(chunk[0].route.GetMatch()).(*route.RouteMatch)
			match.CaseSensitive = nil
			match.PathSpecifier = &route.RouteMatch_SafeRegex{
				SafeRegex: &matcher.RegexMatcher{Regex: combinedGuardRegex(chunk)},
			}
			guard, err := buildNotFoundRoute(chunk[0].route.GetName()+caseVariantGuardSuffix, match)
			if err != nil {
				return nil, err
			}
			guarded = append(guarded, guard)
		}
	}
	return guarded, nil
}

// guardMember is a route whose case variants a combined guard refuses; body is its path regex
// without anchors.
type guardMember struct {
	route *route.Route
	body  string
}

// combinableGuardMember returns r as a member of a combined guard, with the key of the group it
// joins: its path specificity and its non-path matchers. It returns false for a route whose path
// cannot be combined (no semantic path in its name, or a prefix match), which gets its own guard.
func combinableGuardMember(r *route.Route) (guardMember, string, bool) {
	path, ok := semanticPathFromRouteName(r.GetName())
	if !ok {
		return guardMember{}, "", false
	}
	m := r.GetMatch()
	var body string
	switch ps := m.GetPathSpecifier().(type) {
	case *route.RouteMatch_SafeRegex:
		re := ps.SafeRegex.GetRegex()
		if len(re) < 2 || !strings.HasPrefix(re, "^") || !strings.HasSuffix(re, "$") {
			return guardMember{}, "", false
		}
		body = re[1 : len(re)-1]
	case *route.RouteMatch_Path:
		if cs := m.GetCaseSensitive(); cs != nil && !cs.GetValue() {
			return guardMember{}, "", false
		}
		body = regexp.QuoteMeta(ps.Path)
	default:
		return guardMember{}, "", false
	}
	if strings.ToLower(body) == strings.ToUpper(body) {
		return guardMember{}, "", false
	}

	rest := proto.Clone(m).(*route.RouteMatch)
	rest.PathSpecifier = nil
	rest.CaseSensitive = nil
	others, err := proto.MarshalOptions{Deterministic: true}.Marshal(rest)
	if err != nil {
		return guardMember{}, "", false
	}
	return guardMember{route: r, body: body}, specificityKey(path) + "\x00" + string(others), true
}

// specificityKey identifies the routes compareSemanticPaths ranks as equally specific.
func specificityKey(path string) string {
	var b strings.Builder
	for _, segment := range splitSemanticPath(path) {
		fmt.Fprintf(&b, "%d", segment.kind)
		if segment.kind == semanticSegmentMixed {
			fmt.Fprintf(&b, ":%d", segment.literalCount)
		}
		b.WriteByte('/')
	}
	return b.String()
}

func combinedGuardRegex(members []guardMember) string {
	if len(members) == 1 {
		return "(?i)^" + members[0].body + "$"
	}
	bodies := make([]string, len(members))
	for i, m := range members {
		bodies[i] = m.body
	}
	return "(?i)^(?:" + strings.Join(bodies, "|") + ")$"
}

// chunkGuardMembers splits members, in order, into groups whose combined regex stays within
// caseVariantGuardProgramBudget. Each member is parsed once; a group grows while an estimate of its
// size fits (the members' own sizes, less the literal prefix each shares with the one before it,
// which the parser factors out of the alternation), and is then measured exactly and split if the
// estimate was short. A member too large alone still gets a group of its own.
func chunkGuardMembers(members []guardMember) [][]guardMember {
	overhead := regexProgramSize(combinedGuardRegex([]guardMember{{body: ""}}))
	var chunks [][]guardMember
	var current []guardMember
	var prevPrefix []rune
	estimate := 0
	for _, m := range members {
		size, prefix := guardMemberSize(m, overhead)
		cost := size + 1
		if len(current) > 0 {
			cost -= commonPrefixLength(prevPrefix, prefix)
			if estimate+cost > caseVariantGuardProgramBudget {
				chunks = append(chunks, splitOversizedChunk(current)...)
				current, cost = nil, size+1
			}
		}
		if len(current) == 0 {
			estimate = overhead
		}
		current = append(current, m)
		estimate += cost
		prevPrefix = prefix
	}
	if len(current) > 0 {
		chunks = append(chunks, splitOversizedChunk(current)...)
	}
	return chunks
}

// guardMemberSize returns the program size m adds to a combined guard, and the case-folded
// literal prefix of its path.
func guardMemberSize(m guardMember, overhead int) (int, []rune) {
	size := regexProgramSize(combinedGuardRegex([]guardMember{m})) - overhead
	parsed, err := syntax.Parse("(?i)"+m.body, syntax.Perl)
	if err != nil {
		return size, nil
	}
	if parsed.Op == syntax.OpConcat && len(parsed.Sub) > 0 {
		parsed = parsed.Sub[0]
	}
	if parsed.Op != syntax.OpLiteral {
		return size, nil
	}
	return size, []rune(strings.ToLower(string(parsed.Rune)))
}

func commonPrefixLength(a, b []rune) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

// splitOversizedChunk halves chunk until each part's combined regex is within the budget.
func splitOversizedChunk(chunk []guardMember) [][]guardMember {
	if len(chunk) == 1 || regexProgramSize(combinedGuardRegex(chunk)) <= caseVariantGuardProgramBudget {
		return [][]guardMember{chunk}
	}
	half := len(chunk) / 2
	return append(splitOversizedChunk(chunk[:half]), splitOversizedChunk(chunk[half:])...)
}

func regexProgramSize(re string) int {
	parsed, err := syntax.Parse(re, syntax.Perl)
	if err != nil {
		return math.MaxInt
	}
	prog, err := syntax.Compile(parsed.Simplify())
	if err != nil {
		return math.MaxInt
	}
	return len(prog.Inst)
}

// variantCatcher is a route that can match paths it was not written for: one with a parameter,
// mixed or wildcard segment, or a non-product route whose path is not one exact spelling.
type variantCatcher struct {
	route    *route.Route
	segments []string // nil for a route without a semantic path, which is assumed to match anything
}

// variantCatchers indexes catchers by their first segment, lowercased, so a route is checked only
// against catchers under the same first segment and those whose first segment is not a literal.
type variantCatchers struct {
	byFirstSegment map[string][]variantCatcher
	anyFirst       []variantCatcher
}

func newVariantCatchers(routes []*route.Route) variantCatchers {
	c := variantCatchers{byFirstSegment: map[string][]variantCatcher{}}
	for _, r := range routes {
		if isCaseVariantGuard(r) {
			continue
		}
		path, ok := semanticPathFromRouteName(r.GetName())
		if !ok {
			m := r.GetMatch()
			if _, exact := m.GetPathSpecifier().(*route.RouteMatch_Path); exact &&
				(m.GetCaseSensitive() == nil || m.GetCaseSensitive().GetValue()) {
				continue
			}
			c.anyFirst = append(c.anyFirst, variantCatcher{route: r})
			continue
		}
		segments := pathSegments(path)
		if !hasNonLiteralSegment(segments) {
			continue
		}
		catcher := variantCatcher{route: r, segments: segments}
		if isLiteralSegment(segments[0]) {
			first := strings.ToLower(segments[0])
			c.byFirstSegment[first] = append(c.byFirstSegment[first], catcher)
		} else {
			c.anyFirst = append(c.anyFirst, catcher)
		}
	}
	return c
}

// catchVariantOf reports whether a route other than r could match a case variant of r's path,
// which is when r needs a guard. Only the paths are compared, so it errs towards a guard.
func (c variantCatchers) catchVariantOf(r *route.Route) bool {
	path, ok := semanticPathFromRouteName(r.GetName())
	if !ok {
		return true
	}
	segments := pathSegments(path)
	lists := [][]variantCatcher{c.anyFirst}
	if isLiteralSegment(segments[0]) {
		lists = append(lists, c.byFirstSegment[strings.ToLower(segments[0])])
	} else {
		for _, list := range c.byFirstSegment {
			lists = append(lists, list)
		}
	}
	for _, list := range lists {
		for _, catcher := range list {
			if catcher.route == r {
				continue
			}
			if catcher.segments == nil || segmentsMayOverlap(catcher.segments, segments) {
				return true
			}
		}
	}
	return false
}

func pathSegments(path string) []string {
	return strings.Split(strings.TrimPrefix(path, "/"), "/")
}

func isLiteralSegment(segment string) bool {
	return segment != "*" && !strings.ContainsAny(segment, "{}")
}

func hasNonLiteralSegment(segments []string) bool {
	for _, s := range segments {
		if !isLiteralSegment(s) {
			return true
		}
	}
	return false
}

// segmentsMayOverlap reports whether some request could match both paths with case ignored: a
// literal segment must equal the other's literal ignoring case, and a wildcard matches the rest.
func segmentsMayOverlap(a, b []string) bool {
	for i := 0; ; i++ {
		if (i < len(a) && a[i] == "*") || (i < len(b) && b[i] == "*") {
			return true
		}
		if i == len(a) || i == len(b) {
			return len(a) == len(b)
		}
		if isLiteralSegment(a[i]) && isLiteralSegment(b[i]) && !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}
}

// caseInsensitiveMatch returns a copy of m whose path is compared case-insensitively, or false
// when that copy would match nothing m does not: a path with no letters, or one already
// case-insensitive.
func caseInsensitiveMatch(m *route.RouteMatch) (*route.RouteMatch, bool) {
	if m == nil {
		return nil, false
	}
	var path string
	switch ps := m.GetPathSpecifier().(type) {
	case *route.RouteMatch_SafeRegex:
		path = ps.SafeRegex.GetRegex()
		if strings.HasPrefix(path, "(?i)") {
			return nil, false
		}
	case *route.RouteMatch_Path:
		path = ps.Path
	case *route.RouteMatch_Prefix:
		path = ps.Prefix
	case *route.RouteMatch_PathSeparatedPrefix:
		path = ps.PathSeparatedPrefix
	default:
		return nil, false
	}
	if strings.ToLower(path) == strings.ToUpper(path) {
		return nil, false
	}

	guard := proto.Clone(m).(*route.RouteMatch)
	if re := guard.GetSafeRegex(); re != nil {
		re.Regex = "(?i)" + re.Regex
		return guard, true
	}
	if cs := m.GetCaseSensitive(); cs != nil && !cs.GetValue() {
		return nil, false
	}
	guard.CaseSensitive = wrapperspb.Bool(false)
	return guard, true
}

// buildNotFoundRoute builds a route answering the gateway's 404 for match without consulting the
// policy engine.
func buildNotFoundRoute(name string, match *route.RouteMatch) (*route.Route, error) {
	extProcDisabledAny, err := anypb.New(&extproc.ExtProcPerRoute{
		Override: &extproc.ExtProcPerRoute_Disabled{Disabled: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ExtProcPerRoute for route %q: %w", name, err)
	}
	return &route.Route{
		Name:  name,
		Match: match,
		Action: &route.Route_DirectResponse{
			DirectResponse: &route.DirectResponseAction{
				Status: 404,
				Body: &core.DataSource{
					Specifier: &core.DataSource_InlineString{
						// TODO: (renuka) handle error codes in a separate issue: https://github.com/wso2/api-platform/issues/1637
						InlineString: `{"error":"Not Found"}`,
					},
				},
			},
		},
		ResponseHeadersToAdd: []*core.HeaderValueOption{
			{
				Header: &core.HeaderValue{
					Key:   "content-type",
					Value: "application/json",
				},
			},
		},
		TypedPerFilterConfig: map[string]*anypb.Any{
			constants.ExtProcFilterName: extProcDisabledAny,
		},
	}, nil
}
