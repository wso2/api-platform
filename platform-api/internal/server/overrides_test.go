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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/plugin"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/pdk"
)

// Core patterns the tests decorate. The literal-string shape matters: overrides
// are matched against these exactly.
const (
	corePatternGateway  = "GET /api/v0.9/gateways/{gatewayId}"
	corePatternGateways = "GET /api/v0.9/gateways"
)

// overridePlugin is an internal-tier plugin that additionally implements
// plugin.RouteOverrideProvider.
type overridePlugin struct {
	*fakePlugin
	overrides []pdk.RouteOverride
}

func (o *overridePlugin) RouteOverrides() []pdk.RouteOverride { return o.overrides }

// externalOverridePlugin implements pdk.RouteOverrideProvider, the external-tier
// mirror that externalPlugin forwards.
type externalOverridePlugin struct {
	*fakeExternalPlugin
	overrides []pdk.RouteOverride
}

func (o *externalOverridePlugin) RouteOverrides() []pdk.RouteOverride { return o.overrides }

// routeRecordingPlugin registers a route of its own on the shared mux, the way a
// real plugin does.
type routeRecordingPlugin struct {
	*fakePlugin
	pattern string
}

func (p *routeRecordingPlugin) RegisterRoutes(mux *http.ServeMux) {
	p.routesCalled = true
	mux.HandleFunc(p.pattern, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("plugin-route"))
	})
}

// coreRecorder returns a recorder holding two representative core routes: one
// with a wildcard, one without. calls counts how many times each core handler
// actually ran, so a short-circuiting decorator is observable.
func coreRecorder(calls map[string]int) *router.Recorder {
	rec := router.NewRecorder()
	rec.HandleFunc(corePatternGateway, func(w http.ResponseWriter, r *http.Request) {
		calls[corePatternGateway]++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "999") // deliberately stale
		_, _ = w.Write([]byte(`{"id":"` + r.PathValue("gatewayId") + `"}`))
	})
	rec.HandleFunc(corePatternGateways, func(w http.ResponseWriter, _ *http.Request) {
		calls[corePatternGateways]++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"list":[]}`))
	})
	return rec
}

// startup runs the two startup steps a route override spans — initPlugins, then
// installCoreRoutes — against a fresh mux, and returns the mux so a test can
// drive real requests through it.
func startup(t *testing.T, core *router.Recorder, plugins ...plugin.Plugin) (*http.ServeMux, error) {
	t.Helper()
	mux := http.NewServeMux()
	wiring, err := initPlugins(testLogger(), mux, emptyRegistry(t), &plugin.Deps{}, &pdk.Deps{}, plugins, nil)
	if err != nil {
		return mux, err
	}
	return mux, installCoreRoutes(testLogger(), mux, core, wiring.overrides)
}

// get drives one request through the mux and returns the recorder.
func get(mux *http.ServeMux, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

// The base case: the decorator's response is what the client sees, and the core
// handler it wrapped still runs — exactly once, not twice, which is what a
// re-matching implementation would produce.
func TestInstallCoreRoutes_OverrideDecoratesTheCoreHandler(t *testing.T) {
	calls := map[string]int{}
	p := &overridePlugin{
		fakePlugin: &fakePlugin{name: "cloud", spec: specWithScopes},
		overrides: []pdk.RouteOverride{{
			Pattern: corePatternGateway,
			Wrap: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					res := pdk.Invoke(next, r)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(res.Status)
					_, _ = w.Write([]byte(`{"enriched":true}`))
				})
			},
		}},
	}

	mux, err := startup(t, coreRecorder(calls), p)
	if err != nil {
		t.Fatalf("startup: unexpected error: %v", err)
	}

	rec := get(mux, "/api/v0.9/gateways/gw-1")
	if rec.Body.String() != `{"enriched":true}` {
		t.Fatalf("expected the decorator's body, got %q", rec.Body.String())
	}
	if calls[corePatternGateway] != 1 {
		t.Fatalf("expected the core handler to run exactly once, ran %d times", calls[corePatternGateway])
	}
}

// Recording and re-installing core routes must be invisible to every route no
// plugin claimed, so compare an un-overridden route against the no-plugin
// baseline byte for byte.
func TestInstallCoreRoutes_UnoverriddenRouteIsUnchanged(t *testing.T) {
	baselineMux, err := startup(t, coreRecorder(map[string]int{}))
	if err != nil {
		t.Fatalf("baseline startup: unexpected error: %v", err)
	}
	baseline := get(baselineMux, "/api/v0.9/gateways")

	p := &overridePlugin{
		fakePlugin: &fakePlugin{name: "cloud", spec: specWithScopes},
		overrides: []pdk.RouteOverride{{
			Pattern: corePatternGateway, // the OTHER route
			Wrap: func(http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, _ = w.Write([]byte("should not be reachable from the list route"))
				})
			},
		}},
	}
	mux, err := startup(t, coreRecorder(map[string]int{}), p)
	if err != nil {
		t.Fatalf("startup: unexpected error: %v", err)
	}

	got := get(mux, "/api/v0.9/gateways")
	if got.Code != baseline.Code || got.Body.String() != baseline.Body.String() {
		t.Fatalf("un-overridden route changed: got %d %q, baseline %d %q",
			got.Code, got.Body.String(), baseline.Code, baseline.Body.String())
	}
	if got.Header().Get("Content-Type") != baseline.Header().Get("Content-Type") {
		t.Fatalf("un-overridden route headers changed: %v vs %v", got.Header(), baseline.Header())
	}
}

// The whole reason the wrapped handler is registered under the SAME pattern:
// the mux resolves wildcards before the decorator runs, so r.PathValue is still
// correct inside the core handler. A nested-mux implementation loses this.
func TestInstallCoreRoutes_PathValueResolvesInsideTheCoreHandler(t *testing.T) {
	var seenBefore string
	p := &overridePlugin{
		fakePlugin: &fakePlugin{name: "cloud", spec: specWithScopes},
		overrides: []pdk.RouteOverride{{
			Pattern: corePatternGateway,
			Wrap: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					seenBefore = r.PathValue("gatewayId")
					next.ServeHTTP(w, r)
				})
			},
		}},
	}

	mux, err := startup(t, coreRecorder(map[string]int{}), p)
	if err != nil {
		t.Fatalf("startup: unexpected error: %v", err)
	}

	rec := get(mux, "/api/v0.9/gateways/gw-42")
	if seenBefore != "gw-42" {
		t.Fatalf("expected the decorator to see the path value, got %q", seenBefore)
	}
	// The core handler echoes r.PathValue("gatewayId") into its body.
	if rec.Body.String() != `{"id":"gw-42"}` {
		t.Fatalf("expected the path value to resolve inside the core handler, got %q", rec.Body.String())
	}
}

// A decorator that reshapes the body must not leak the core handler's
// Content-Length, which no longer describes what is being sent.
func TestInstallCoreRoutes_ReshapedResponseDropsStaleContentLength(t *testing.T) {
	p := &overridePlugin{
		fakePlugin: &fakePlugin{name: "cloud", spec: specWithScopes},
		overrides: []pdk.RouteOverride{{
			Pattern: corePatternGateway,
			Wrap: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					res := pdk.Invoke(next, r)
					res.Body = []byte(`{"id":"gw-1","environment":"prod"}`)
					res.Status = http.StatusAccepted
					pdk.WriteCaptured(w, res)
				})
			},
		}},
	}

	mux, err := startup(t, coreRecorder(map[string]int{}), p)
	if err != nil {
		t.Fatalf("startup: unexpected error: %v", err)
	}

	rec := get(mux, "/api/v0.9/gateways/gw-1")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected the decorator's status 202, got %d", rec.Code)
	}
	if rec.Body.String() != `{"id":"gw-1","environment":"prod"}` {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Length"); got != "" {
		t.Fatalf("expected the core handler's stale Content-Length to be dropped, got %q", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected the core handler's Content-Type to be forwarded, got %q", got)
	}
}

// A decorator may answer without delegating — a cache hit, a precondition
// failure — and the core handler must then not run at all.
func TestInstallCoreRoutes_OverrideCanShortCircuit(t *testing.T) {
	calls := map[string]int{}
	p := &overridePlugin{
		fakePlugin: &fakePlugin{name: "cloud", spec: specWithScopes},
		overrides: []pdk.RouteOverride{{
			Pattern: corePatternGateway,
			Wrap: func(http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusTeapot)
				})
			},
		}},
	}

	mux, err := startup(t, coreRecorder(calls), p)
	if err != nil {
		t.Fatalf("startup: unexpected error: %v", err)
	}

	if rec := get(mux, "/api/v0.9/gateways/gw-1"); rec.Code != http.StatusTeapot {
		t.Fatalf("expected the decorator's status, got %d", rec.Code)
	}
	if calls[corePatternGateway] != 0 {
		t.Fatalf("expected the core handler never to run, ran %d times", calls[corePatternGateway])
	}
}

// A pattern core does not register would decorate nothing at all, silently.
// Patterns are matched exactly, so a version or wildcard-name slip lands here.
func TestInstallCoreRoutes_UnknownPatternAbortsStartup(t *testing.T) {
	for _, pattern := range []string{
		"GET /api/v1/gateways/{gatewayId}", // wrong version
		"GET /api/v0.9/gateways/{id}",      // wrong wildcard name
		"POST /api/v0.9/gateways",          // wrong method
	} {
		t.Run(pattern, func(t *testing.T) {
			p := &overridePlugin{
				fakePlugin: &fakePlugin{name: "cloud", spec: specWithScopes},
				overrides:  []pdk.RouteOverride{{Pattern: pattern, Wrap: passthrough}},
			}

			_, err := startup(t, coreRecorder(map[string]int{}), p)
			if err == nil {
				t.Fatal("expected startup to abort, got nil error")
			}
			if !strings.Contains(err.Error(), pattern) || !strings.Contains(err.Error(), "cloud") {
				t.Fatalf("error should name the pattern and the plugin, got: %v", err)
			}
		})
	}
}

// One unmatched pattern per error would mean fixing a typo, restarting, and
// finding the next one — so the error names every unmatched override at once.
func TestInstallCoreRoutes_UnknownPatternErrorNamesEveryOverride(t *testing.T) {
	patterns := []string{
		"GET /api/v1/gateways/{gatewayId}",
		"GET /api/v0.9/gateways/{id}",
		"POST /api/v0.9/gateways",
	}
	cloud := &overridePlugin{
		fakePlugin: &fakePlugin{name: "cloud", spec: specWithScopes},
		overrides: []pdk.RouteOverride{
			{Pattern: patterns[0], Wrap: passthrough},
			{Pattern: patterns[1], Wrap: passthrough},
		},
	}
	audit := &overridePlugin{
		fakePlugin: &fakePlugin{name: "audit", spec: specWithScopes},
		overrides:  []pdk.RouteOverride{{Pattern: patterns[2], Wrap: passthrough}},
	}

	_, err := startup(t, coreRecorder(map[string]int{}), cloud, audit)
	if err == nil {
		t.Fatal("expected startup to abort, got nil error")
	}
	for _, pattern := range patterns {
		if !strings.Contains(err.Error(), pattern) {
			t.Fatalf("error should name every unmatched pattern, %q missing from: %v", pattern, err)
		}
	}
	for _, plugin := range []string{"cloud", "audit"} {
		if !strings.Contains(err.Error(), plugin) {
			t.Fatalf("error should name every claiming plugin, %q missing from: %v", plugin, err)
		}
	}
}

// Two decorators on one route would have to be ordered by something no plugin
// author can see, so this is a startup error naming both claimants.
func TestInitPlugins_TwoPluginsClaimingOnePatternAbortStartup(t *testing.T) {
	first := &overridePlugin{
		fakePlugin: &fakePlugin{name: "cloud", spec: specWithScopes},
		overrides:  []pdk.RouteOverride{{Pattern: corePatternGateway, Wrap: passthrough}},
	}
	second := &overridePlugin{
		fakePlugin: &fakePlugin{name: "audit", spec: specWithScopes},
		overrides:  []pdk.RouteOverride{{Pattern: corePatternGateway, Wrap: passthrough}},
	}

	_, err := startup(t, coreRecorder(map[string]int{}), first, second)
	if err == nil {
		t.Fatal("expected startup to abort, got nil error")
	}
	if !strings.Contains(err.Error(), "cloud") || !strings.Contains(err.Error(), "audit") {
		t.Fatalf("error should name both plugins, got: %v", err)
	}
}

// The same plugin claiming a pattern twice is the same conflict, and must not
// silently keep the last declaration.
func TestInitPlugins_OnePluginClaimingOnePatternTwiceAbortsStartup(t *testing.T) {
	p := &overridePlugin{
		fakePlugin: &fakePlugin{name: "cloud", spec: specWithScopes},
		overrides: []pdk.RouteOverride{
			{Pattern: corePatternGateway, Wrap: passthrough},
			{Pattern: corePatternGateway, Wrap: passthrough},
		},
	}

	if _, err := startup(t, coreRecorder(map[string]int{}), p); err == nil {
		t.Fatal("expected startup to abort, got nil error")
	}
}

// A malformed override entry is a wiring bug the author gets no other signal
// about: the route would simply never be decorated.
func TestInitPlugins_MalformedOverrideAbortsStartup(t *testing.T) {
	tests := []struct {
		name     string
		override pdk.RouteOverride
	}{
		{"nil Wrap", pdk.RouteOverride{Pattern: corePatternGateway}},
		{"empty pattern", pdk.RouteOverride{Wrap: passthrough}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &overridePlugin{
				fakePlugin: &fakePlugin{name: "cloud", spec: specWithScopes},
				overrides:  []pdk.RouteOverride{tc.override},
			}

			if _, err := startup(t, coreRecorder(map[string]int{}), p); err == nil {
				t.Fatal("expected startup to abort, got nil error")
			}
		})
	}
}

// Overriding is optional: a plugin that does not implement the interface must
// leave every core route exactly as it was.
func TestInstallCoreRoutes_PluginWithoutOverridesIsANoOp(t *testing.T) {
	p := &fakePlugin{name: "widgets", spec: specWithScopes}

	mux, err := startup(t, coreRecorder(map[string]int{}), p)
	if err != nil {
		t.Fatalf("startup: unexpected error: %v", err)
	}

	if rec := get(mux, "/api/v0.9/gateways/gw-1"); rec.Body.String() != `{"id":"gw-1"}` {
		t.Fatalf("expected the untouched core response, got %q", rec.Body.String())
	}
}

// Both tiers must reach the same wiring path: an external plugin declares its
// override through pdk and externalPlugin forwards it unchanged.
func TestInstallCoreRoutes_ExternalPluginOverrideIsForwarded(t *testing.T) {
	ext := &externalOverridePlugin{
		fakeExternalPlugin: &fakeExternalPlugin{name: "api-cloud", spec: specWithScopes},
		overrides: []pdk.RouteOverride{{
			Pattern: corePatternGateway,
			Wrap: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					res := pdk.Invoke(next, r)
					res.Body = []byte(`{"from":"external"}`)
					pdk.WriteCaptured(w, res)
				})
			},
		}},
	}

	mux := http.NewServeMux()
	wiring, err := initPlugins(testLogger(), mux, emptyRegistry(t), &plugin.Deps{}, &pdk.Deps{},
		nil, []pdk.Plugin{ext})
	if err != nil {
		t.Fatalf("initPlugins: unexpected error: %v", err)
	}
	if err := installCoreRoutes(testLogger(), mux, coreRecorder(map[string]int{}), wiring.overrides); err != nil {
		t.Fatalf("installCoreRoutes: unexpected error: %v", err)
	}

	if rec := get(mux, "/api/v0.9/gateways/gw-1"); rec.Body.String() != `{"from":"external"}` {
		t.Fatalf("expected the external plugin's decorator to run, got %q", rec.Body.String())
	}
}

// A plugin route claiming a core pattern used to panic at plugin-registration
// time; now it surfaces when the core routes are installed. Either way it must
// not reach a running server, and the message must name the pattern.
func TestInstallCoreRoutes_PluginRouteCollidingWithCoreIsAnError(t *testing.T) {
	p := &routeRecordingPlugin{
		fakePlugin: &fakePlugin{name: "cloud", spec: specWithScopes},
		pattern:    corePatternGateway,
	}

	_, err := startup(t, coreRecorder(map[string]int{}), p)
	if err == nil {
		t.Fatal("expected startup to abort, got nil error")
	}
	if !strings.Contains(err.Error(), corePatternGateway) {
		t.Fatalf("error should name the colliding pattern, got: %v", err)
	}
}

// A Wrap returning nil would register a nil handler and panic on the first
// request to that route, long after startup.
func TestInstallCoreRoutes_NilWrappedHandlerAbortsStartup(t *testing.T) {
	p := &overridePlugin{
		fakePlugin: &fakePlugin{name: "cloud", spec: specWithScopes},
		overrides: []pdk.RouteOverride{{
			Pattern: corePatternGateway,
			Wrap:    func(http.Handler) http.Handler { return nil },
		}},
	}

	_, err := startup(t, coreRecorder(map[string]int{}), p)
	if err == nil {
		t.Fatal("expected startup to abort, got nil error")
	}
	if !strings.Contains(err.Error(), "cloud") {
		t.Fatalf("error should name the plugin, got: %v", err)
	}
}

// passthrough is a decorator that changes nothing, for tests that only care
// about validation.
func passthrough(next http.Handler) http.Handler { return next }
