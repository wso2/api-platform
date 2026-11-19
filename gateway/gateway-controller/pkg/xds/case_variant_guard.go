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
	"strings"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	extproc "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	"google.golang.org/protobuf/proto"
	anypb "google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

// caseVariantGuardSuffix ends the name of a route that refuses case variants of another route's
// path. The guard keeps its route's METHOD|PATH|VHOST prefix so it sorts and groups with it.
const caseVariantGuardSuffix = "|case-variant-guard"

func isCaseVariantGuard(r *route.Route) bool {
	return strings.HasSuffix(r.GetName(), caseVariantGuardSuffix)
}

// appendCaseVariantGuards adds, for every route, a 404 route matching the same requests with the
// path compared case-insensitively. Route paths match case-sensitively, so without it a request
// for "/Secure/data" misses "/secure/data" and reaches a broader route such as "/*" without the
// policies of the route it was aimed at. SortRoutesByPriority places each guard after every route
// of equal path specificity, so a route whose path really is "/Secure/data" still wins.
func appendCaseVariantGuards(routes []*route.Route) ([]*route.Route, error) {
	guarded := routes
	for _, r := range routes {
		match, ok := caseInsensitiveMatch(r.GetMatch())
		if !ok || r.GetName() == "" {
			continue
		}
		guard, err := buildNotFoundRoute(r.GetName()+caseVariantGuardSuffix, match)
		if err != nil {
			return nil, err
		}
		guarded = append(guarded, guard)
	}
	return guarded, nil
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
