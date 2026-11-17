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

package featureusage

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// RouteResolver resolves the route pattern a request will match without
// serving it. *http.ServeMux implements it.
type RouteResolver interface {
	Handler(r *http.Request) (h http.Handler, pattern string)
}

// specOperation is one operation of the OpenAPI document, as a route.
type specOperation struct {
	method string // upper case
	path   string // base path + OpenAPI path, e.g. /api/v0.9/rest-apis/{restApiId}
}

// boundRoute is a binding resolved to the route that serves it.
type boundRoute struct {
	binding Binding
	feature *Feature
	method  string
	// path is the route pattern's path as the mux registers it, so its
	// parameter names are the ones the handler reads.
	path string
	// captureJSON says the binding reads the JSON request body.
	captureJSON bool
}

// RouteIndex maps the routes of bound operations to their bindings. Build it
// with NewRouteIndex; a nil *RouteIndex binds nothing.
type RouteIndex struct {
	byRoute map[string]*boundRoute // routeKey(method, normalized path)
	methods map[string]bool        // the methods any bound route uses
}

// Len returns the number of bound routes.
func (ri *RouteIndex) Len() int {
	if ri == nil {
		return 0
	}
	return len(ri.byRoute)
}

// NewRouteIndex resolves every binding to its route: the operation's method
// and path in spec (the OpenAPI document, with servers[0].url's path as the
// base path), checked against routes, the mux that serves them. It fails when
// the bindings do not agree with tax, the parsed taxonomy, since both are
// compiled in.
//
// A bound operation that spec does not declare, or that routes does not serve
// under the same path, is skipped and logged, not an error: a build can leave
// out a plugin's routes, and reporting must never stop the server from
// starting. The bindings_test.go checks catch that drift in the source tree.
func NewRouteIndex(spec []byte, routes RouteResolver, tax *Taxonomy, logger *slog.Logger) (*RouteIndex, error) {
	return newRouteIndex(spec, routes, tax, bindings, logger)
}

func newRouteIndex(spec []byte, routes RouteResolver, tax *Taxonomy, bs []Binding, logger *slog.Logger) (*RouteIndex, error) {
	if err := checkBindings(tax, bs); err != nil {
		return nil, err
	}
	ops, err := specOperations(spec)
	if err != nil {
		return nil, err
	}

	ri := &RouteIndex{byRoute: map[string]*boundRoute{}, methods: map[string]bool{}}
	var skipped []string
	for _, b := range bs {
		op, ok := ops[b.Op]
		if !ok {
			skipped = append(skipped, b.Op+" (not in the OpenAPI document)")
			continue
		}
		path, err := servedPath(routes, op)
		if err != nil {
			skipped = append(skipped, b.Op+" ("+err.Error()+")")
			continue
		}
		if missing := missingPathParams(b, path); missing != "" {
			return nil, fmt.Errorf("feature usage: binding for operation %q reads path parameter %q, which route %s %s does not have",
				b.Op, missing, op.method, path)
		}
		f, _ := tax.Feature(b.Feature) // checkBindings guarantees it
		br := &boundRoute{binding: b, feature: f, method: op.method, path: path}
		for _, src := range b.From {
			if src.kind == srcJSON {
				br.captureJSON = true
			}
		}
		key := routeKey(op.method, path)
		if prev, dup := ri.byRoute[key]; dup {
			return nil, fmt.Errorf("feature usage: operations %q and %q are both served by %s %s",
				prev.binding.Op, b.Op, op.method, path)
		}
		ri.byRoute[key] = br
		ri.methods[op.method] = true
	}
	if len(skipped) > 0 && logger != nil {
		logger.Warn("Feature usage: some bound operations are not served and will not be reported",
			slog.String("party", logParty), slog.Any("operations", skipped))
	}
	return ri, nil
}

// lookup returns the bound route r will be served by, and the route's path.
func (ri *RouteIndex) lookup(routes RouteResolver, r *http.Request) *boundRoute {
	if ri == nil || !ri.methods[r.Method] {
		return nil
	}
	_, pattern := routes.Handler(r)
	method, path, ok := splitPattern(pattern)
	if !ok {
		return nil
	}
	if method == "" {
		method = r.Method
	}
	return ri.byRoute[routeKey(method, path)]
}

// servedPath returns the path of the route pattern routes matches for op, or
// an error when op is not served under its OpenAPI path.
func servedPath(routes RouteResolver, op specOperation) (string, error) {
	if routes == nil {
		return op.path, nil
	}
	// A concrete request for the operation, every parameter filled in.
	concrete := pathParamName.ReplaceAllString(op.path, "x")
	req := &http.Request{Method: op.method, URL: &url.URL{Path: concrete}, Header: http.Header{}}
	_, pattern := routes.Handler(req)
	method, path, ok := splitPattern(pattern)
	if !ok || (method != "" && method != op.method) || normalizeParams(path) != normalizeParams(op.path) {
		return "", fmt.Errorf("no route serves %s %s", op.method, op.path)
	}
	return path, nil
}

// missingPathParams returns the first path parameter b reads that path lacks.
func missingPathParams(b Binding, path string) string {
	have := map[string]bool{}
	for _, name := range pathParams(path) {
		have[name] = true
	}
	for _, name := range sortedKeys(b.From) {
		if src := b.From[name]; src.kind == srcPath && !have[src.name] {
			return src.name
		}
	}
	return ""
}

// splitPattern splits a ServeMux pattern ("[METHOD ][HOST]/PATH") into its
// method (empty when the pattern matches every method) and path.
func splitPattern(pattern string) (method, path string, ok bool) {
	if i := strings.IndexByte(pattern, ' '); i >= 0 {
		method, pattern = pattern[:i], strings.TrimLeft(pattern[i+1:], " ")
	}
	if i := strings.IndexByte(pattern, '/'); i > 0 {
		pattern = pattern[i:] // drop a host
	}
	if !strings.HasPrefix(pattern, "/") {
		return "", "", false
	}
	return method, pattern, true
}

// pathParamName matches a {name} path parameter, but not the {$} anchor.
var pathParamName = regexp.MustCompile(`\{[^/{}$][^/{}]*\}`)

// normalizeParams drops parameter names, which are not part of a route's
// identity: /gateways/{gatewayId} and /gateways/{id} are the same route.
func normalizeParams(path string) string { return pathParamName.ReplaceAllString(path, "{}") }

func routeKey(method, path string) string { return method + " " + normalizeParams(path) }

// specOperations reads the operationId, method and path of every operation in
// an OpenAPI document. Paths get servers[0].url's path as a prefix, which is
// how the server registers them.
func specOperations(spec []byte) (map[string]specOperation, error) {
	var doc struct {
		Servers []struct {
			URL string `yaml:"url"`
		} `yaml:"servers"`
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(spec, &doc); err != nil {
		return nil, fmt.Errorf("feature usage: parse the OpenAPI document: %w", err)
	}
	base := ""
	if len(doc.Servers) > 0 {
		if u, err := url.Parse(doc.Servers[0].URL); err == nil {
			base = strings.TrimRight(u.Path, "/")
		}
	}
	ops := map[string]specOperation{}
	for path, item := range doc.Paths {
		for field, node := range item {
			method := strings.ToUpper(field)
			switch method {
			case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			default:
				continue // path-item metadata such as parameters
			}
			var op struct {
				OperationID string `yaml:"operationId"`
			}
			if err := node.Decode(&op); err != nil || op.OperationID == "" {
				continue
			}
			if prev, dup := ops[op.OperationID]; dup {
				return nil, fmt.Errorf("feature usage: operationId %q is declared by both %s %s and %s %s",
					op.OperationID, prev.method, prev.path, method, base+path)
			}
			ops[op.OperationID] = specOperation{method: method, path: base + path}
		}
	}
	return ops, nil
}
