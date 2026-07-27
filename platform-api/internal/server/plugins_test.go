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

package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/plugin"
	"github.com/wso2/api-platform/platform-api/pdk"
	gohttpkit "github.com/wso2/go-httpkit/middleware"
)

// specWithScopes is a minimal OpenAPI document declaring one scoped operation.
const specWithScopes = `
openapi: 3.0.0
servers:
  - url: /api/v1
paths:
  /widgets:
    get:
      security:
        - OAuth2:
            - ap:widget_read
`

// specMalformed is not parseable as YAML.
const specMalformed = `{ openapi: 3.0.0`

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// emptyRegistry returns a usable, empty ScopeRegistry — the shape the server has
// built by the time it initializes plugins.
func emptyRegistry(t *testing.T) *middleware.ScopeRegistry {
	t.Helper()
	reg, err := middleware.LoadScopeRegistryFromBytes([]byte("openapi: 3.0.0\npaths: {}\n"))
	if err != nil {
		t.Fatalf("building an empty scope registry: %v", err)
	}
	return reg
}

// fakePlugin is a minimal internal-tier plugin whose spec is configurable.
type fakePlugin struct {
	name string
	spec string

	initCalled   bool
	routesCalled bool
}

func (f *fakePlugin) Name() string { return f.name }

func (f *fakePlugin) Init(*plugin.Deps) error {
	f.initCalled = true
	return nil
}

func (f *fakePlugin) RegisterRoutes(*http.ServeMux) { f.routesCalled = true }

func (f *fakePlugin) OpenAPISpec() []byte { return []byte(f.spec) }

func (f *fakePlugin) Shutdown(context.Context) error { return nil }

// skipPathPlugin additionally implements plugin.AuthSkipPathProvider.
type skipPathPlugin struct {
	*fakePlugin
	paths []string
}

func (s *skipPathPlugin) AuthSkipPaths() []string { return s.paths }

// middlewarePlugin additionally implements plugin.MiddlewareProvider.
type middlewarePlugin struct {
	*fakePlugin
	mw []pdk.PositionedMiddleware
}

func (m *middlewarePlugin) Middleware() []pdk.PositionedMiddleware { return m.mw }

// recordingMiddleware returns a middleware that appends tag to *log as a request
// passes through it. Chain composition has to be asserted this way round: func
// values are not comparable in Go, so a collected pdk.Middleware cannot be matched
// against the one the plugin handed in — only the order they actually run in is
// observable.
func recordingMiddleware(log *[]string, tag string) pdk.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*log = append(*log, tag)
			next.ServeHTTP(w, r)
		})
	}
}

// drive composes mws the way the server does — first element outermost, per
// gohttpkit.Chain — and sends one request through, so the middleware built by
// recordingMiddleware log the order they run in. It also fails loudly on a nil
// Wrap that was collected rather than skipped, since calling one panics.
func drive(t *testing.T, mws []pdk.Middleware) {
	t.Helper()
	chain := make([]func(http.Handler) http.Handler, 0, len(mws))
	for _, mw := range mws {
		chain = append(chain, mw)
	}
	handler := gohttpkit.Chain(chain...)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

// run calls initPlugins with the fixed test collaborators.
func run(t *testing.T, reg *middleware.ScopeRegistry, plugins ...plugin.Plugin) (*pluginWiring, error) {
	t.Helper()
	return initPlugins(testLogger(), http.NewServeMux(), reg, &plugin.Deps{}, &pdk.Deps{}, plugins, nil)
}

// GO-AUTH-007: a plugin's scopes must reach the shared registry, or the routes it
// mounts are served with no scope requirement.
func TestInitPlugins_ValidSpecMergesScopes(t *testing.T) {
	reg := emptyRegistry(t)
	p := &fakePlugin{name: "widgets", spec: specWithScopes}

	if _, err := run(t, reg, p); err != nil {
		t.Fatalf("initPlugins: unexpected error: %v", err)
	}
	if !p.initCalled || !p.routesCalled {
		t.Fatalf("expected Init and RegisterRoutes to be called, got init=%v routes=%v", p.initCalled, p.routesCalled)
	}

	scopes, ok := reg.Lookup("GET", "/api/v1/widgets")
	if !ok {
		t.Fatal("plugin scopes were not merged into the registry")
	}
	if len(scopes) != 1 || scopes[0] != "ap:widget_read" {
		t.Fatalf("unexpected merged scopes: %v", scopes)
	}
}

// A spec is mandatory and must load: absent or unparseable bytes mean the
// plugin's scopes silently never reach the registry, which is a wiring bug
// rather than a deliberate choice, so both abort startup.
func TestInitPlugins_MissingOrUnloadableSpecAbortsStartup(t *testing.T) {
	tests := []struct {
		name string
		spec string
	}{
		{"empty spec", ""},
		{"malformed spec", specMalformed},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reg := emptyRegistry(t)

			if _, err := run(t, reg, &fakePlugin{name: "widgets", spec: tc.spec}); err == nil {
				t.Fatal("expected startup to abort, got nil error")
			}
			if reg.Len() != 0 {
				t.Fatalf("expected no scopes merged from a rejected spec, got %d", reg.Len())
			}
		})
	}
}

// GO-AUTH-004: skip-path matching is a prefix match, so an over-broad prefix
// bypasses authentication for routes far beyond the plugin's own.
func TestInitPlugins_RejectsUnsafeAuthSkipPaths(t *testing.T) {
	unsafe := []string{
		"",              // matches every request
		"/",             // matches every request
		"public",        // no leading slash
		"/pub/../admin", // traversal
	}

	for _, path := range unsafe {
		t.Run("path="+path, func(t *testing.T) {
			p := &skipPathPlugin{
				fakePlugin: &fakePlugin{name: "widgets", spec: specWithScopes},
				paths:      []string{path},
			}

			if _, err := run(t, emptyRegistry(t), p); err == nil {
				t.Fatalf("expected skip path %q to abort startup", path)
			}
		})
	}
}

// The narrow prefixes a plugin is allowed to declare must still come through, or
// the guard above would be enforced by breaking legitimate public routes.
func TestInitPlugins_CollectsValidAuthSkipPaths(t *testing.T) {
	p := &skipPathPlugin{
		fakePlugin: &fakePlugin{name: "widgets", spec: specWithScopes},
		paths:      []string{"/api/v1/widgets/health", "/api/v1/widgets/callback"},
	}

	wiring, err := run(t, emptyRegistry(t), p)
	if err != nil {
		t.Fatalf("initPlugins: unexpected error: %v", err)
	}
	if len(wiring.authSkipPaths) != 2 ||
		wiring.authSkipPaths[0] != "/api/v1/widgets/health" ||
		wiring.authSkipPaths[1] != "/api/v1/widgets/callback" {
		t.Fatalf("unexpected skip paths: %v", wiring.authSkipPaths)
	}
}

// Sorting plugin middleware into the two chain positions is a security boundary,
// not just plumbing: BeforePlatformChain runs before CORS and auth with no
// identity in the context, while AfterPlatformChain runs after auth,
// organization resolution, and scope enforcement, and is documented to read the
// authenticated org from context (GO-AUTH-005). Middleware landing in the wrong
// position fails silently — an audit or per-tenant rate limiter would simply see
// no identity — so assert placement explicitly. Order within a position is part
// of the same contract (see pdk.MiddlewareProvider): plugin registration order.
func TestInitPlugins_MiddlewareIsPlacedByPositionInRegistrationOrder(t *testing.T) {
	var log []string

	first := &middlewarePlugin{
		fakePlugin: &fakePlugin{name: "first", spec: specWithScopes},
		mw: []pdk.PositionedMiddleware{
			{Position: pdk.BeforePlatformChain, Wrap: recordingMiddleware(&log, "first-pre")},
			{Position: pdk.AfterPlatformChain, Wrap: recordingMiddleware(&log, "first-post")},
		},
	}
	second := &middlewarePlugin{
		fakePlugin: &fakePlugin{name: "second", spec: specWithScopes},
		mw: []pdk.PositionedMiddleware{
			{Position: pdk.AfterPlatformChain, Wrap: recordingMiddleware(&log, "second-post")},
			{Position: pdk.BeforePlatformChain, Wrap: recordingMiddleware(&log, "second-pre")},
		},
	}

	wiring, err := run(t, emptyRegistry(t), first, second)
	if err != nil {
		t.Fatalf("initPlugins: unexpected error: %v", err)
	}

	// Each plugin declares its two positions in a different order, so a switch
	// that ignored Position entirely could not produce both of these.
	drive(t, wiring.preChain)
	if want := []string{"first-pre", "second-pre"}; !slices.Equal(log, want) {
		t.Errorf("preChain ran %v, want %v", log, want)
	}

	log = nil
	drive(t, wiring.postChain)
	if want := []string{"first-post", "second-post"}; !slices.Equal(log, want) {
		t.Errorf("postChain ran %v, want %v", log, want)
	}
}

// A malformed entry aborts startup rather than being dropped. Skipping one
// silently would discard the middleware — and the plugin author's intent — with
// no signal: a panic recovery or IP allow-list would simply never run. A nil
// Wrap is malformed for the same reason and is not an opt-out; the documented
// way to contribute nothing is an empty slice (see pdk.MiddlewareProvider).
func TestInitPlugins_MalformedMiddlewareEntryAbortsStartup(t *testing.T) {
	// Stands in for any real middleware: these entries exist only to be valid
	// company for the malformed one below.
	passthrough := pdk.Middleware(func(next http.Handler) http.Handler { return next })

	tests := []struct {
		name string
		bad  pdk.PositionedMiddleware
	}{
		{"nil wrap", pdk.PositionedMiddleware{Position: pdk.BeforePlatformChain, Wrap: nil}},
		// ChainPosition is an int with exactly two valid values, so an
		// out-of-range positive and a negative both reach the default arm.
		{"position above range", pdk.PositionedMiddleware{Position: 99, Wrap: passthrough}},
		{"negative position", pdk.PositionedMiddleware{Position: -1, Wrap: passthrough}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// A well-formed entry precedes the bad one, so the abort is
			// attributable to the malformed entry rather than to the plugin
			// contributing nothing usable at all.
			p := &middlewarePlugin{
				fakePlugin: &fakePlugin{name: "widgets", spec: specWithScopes},
				mw: []pdk.PositionedMiddleware{
					{Position: pdk.BeforePlatformChain, Wrap: passthrough},
					tc.bad,
				},
			}

			wiring, err := run(t, emptyRegistry(t), p)
			if err == nil {
				t.Fatalf("expected a %s to abort startup, got nil error", tc.name)
			}
			if wiring != nil {
				t.Errorf("expected no wiring on an aborted startup, got %+v", wiring)
			}
		})
	}
}
