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
	"net/http"
	"slices"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/plugin"
	"github.com/wso2/api-platform/platform-api/pdk"
)

// fakeExternalPlugin is a minimal pdk.Plugin that records the Deps it was handed.
type fakeExternalPlugin struct {
	name string
	spec string

	gotDeps      *pdk.Deps
	routesCalled bool
}

func (f *fakeExternalPlugin) Name() string { return f.name }

func (f *fakeExternalPlugin) Init(deps *pdk.Deps) error {
	f.gotDeps = deps
	return nil
}

func (f *fakeExternalPlugin) RegisterRoutes(*http.ServeMux) { f.routesCalled = true }

func (f *fakeExternalPlugin) OpenAPISpec() []byte { return []byte(f.spec) }

func (f *fakeExternalPlugin) Shutdown(context.Context) error { return nil }

// externalSkipPathPlugin carries an AuthSkipPaths method.
type externalSkipPathPlugin struct {
	*fakeExternalPlugin
	paths []string
}

func (f *externalSkipPathPlugin) AuthSkipPaths() []string { return f.paths }

// externalMiddlewarePlugin implements pdk.MiddlewareProvider.
type externalMiddlewarePlugin struct {
	*fakeExternalPlugin
	mw []pdk.PositionedMiddleware
}

func (f *externalMiddlewarePlugin) Middleware() []pdk.PositionedMiddleware { return f.mw }

// The premise of the two-tier model: an external plugin receives pdk.Deps
// (capabilities as public interfaces) and never the internal plugin.Deps, which
// carries raw repositories and the DB handle.
func TestExternalPlugin_InitReceivesPDKDepsNotInternalDeps(t *testing.T) {
	pdkDeps := &pdk.Deps{}
	ext := &fakeExternalPlugin{name: "api-cloud", spec: specWithScopes}

	wrapped := &externalPlugin{p: ext, pdkDeps: pdkDeps}
	if err := wrapped.Init(&plugin.Deps{}); err != nil {
		t.Fatalf("Init: unexpected error: %v", err)
	}

	if ext.gotDeps != pdkDeps {
		t.Fatal("external plugin did not receive the server-built pdk.Deps")
	}
}

// The same must hold through the real startup path, not only when the wrapper is
// constructed by hand.
func TestInitPlugins_ExternalTierReceivesPDKDeps(t *testing.T) {
	pdkDeps := &pdk.Deps{}
	ext := &fakeExternalPlugin{name: "api-cloud", spec: specWithScopes}

	wiring, err := initPlugins(testLogger(), http.NewServeMux(), emptyRegistry(t),
		&plugin.Deps{}, pdkDeps, nil, []pdk.Plugin{ext})
	if err != nil {
		t.Fatalf("initPlugins: unexpected error: %v", err)
	}
	if len(wiring.plugins) != 1 {
		t.Fatalf("expected the external plugin to be wired, got %d plugins", len(wiring.plugins))
	}
	if ext.gotDeps != pdkDeps {
		t.Fatal("external plugin did not receive the server-built pdk.Deps")
	}
	if !ext.routesCalled {
		t.Error("expected RegisterRoutes to be called on the external plugin")
	}
}

// External plugins go through the same spec guards as internal ones — the
// wrapper must not become a way around them.
func TestInitPlugins_ExternalTierIsHeldToTheSameGuards(t *testing.T) {
	tests := []struct {
		name string
		spec string
	}{
		{"empty spec", ""},
		{"malformed spec", specMalformed},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ext := &fakeExternalPlugin{name: "api-cloud", spec: tc.spec}

			_, err := initPlugins(testLogger(), http.NewServeMux(), emptyRegistry(t),
				&plugin.Deps{}, &pdk.Deps{}, nil, []pdk.Plugin{ext})
			if err == nil {
				t.Fatal("expected an external plugin with an unusable spec to abort startup")
			}
		})
	}
}

// Every route an external plugin mounts is authenticated: an AuthSkipPaths
// method on the plugin — even one returning the root prefix — must not widen the
// auth bypass (GO-AUTH-004).
func TestInitPlugins_ExternalTierCannotDeclareAuthSkipPaths(t *testing.T) {
	ext := &externalSkipPathPlugin{
		fakeExternalPlugin: &fakeExternalPlugin{name: "api-cloud", spec: specWithScopes},
		paths:              []string{"/"},
	}

	wiring, err := initPlugins(testLogger(), http.NewServeMux(), emptyRegistry(t),
		&plugin.Deps{}, &pdk.Deps{}, nil, []pdk.Plugin{ext})
	if err != nil {
		t.Fatalf("initPlugins: unexpected error: %v", err)
	}
	if len(wiring.authSkipPaths) != 0 {
		t.Fatalf("external plugin contributed skip paths %v; the external tier has no such hook", wiring.authSkipPaths)
	}
}

// Middleware, unlike auth skip paths above, IS forwarded for the external tier:
// externalPlugin.Middleware passes pdk.MiddlewareProvider through so both tiers
// reach the server's single wiring path. The asymmetry is deliberate — external
// middleware cannot bypass auth, because BeforePlatformChain runs before the
// platform's auth and cannot mark a request as authenticated, whereas a skip
// path would remove the auth check outright (GO-AUTH-004).
func TestInitPlugins_ExternalTierContributesMiddleware(t *testing.T) {
	var log []string

	ext := &externalMiddlewarePlugin{
		fakeExternalPlugin: &fakeExternalPlugin{name: "api-cloud", spec: specWithScopes},
		mw: []pdk.PositionedMiddleware{
			{Position: pdk.BeforePlatformChain, Wrap: recordingMiddleware(&log, "ext-pre")},
			{Position: pdk.AfterPlatformChain, Wrap: recordingMiddleware(&log, "ext-post")},
		},
	}

	wiring, err := initPlugins(testLogger(), http.NewServeMux(), emptyRegistry(t),
		&plugin.Deps{}, &pdk.Deps{}, nil, []pdk.Plugin{ext})
	if err != nil {
		t.Fatalf("initPlugins: unexpected error: %v", err)
	}

	drive(t, wiring.preChain)
	if want := []string{"ext-pre"}; !slices.Equal(log, want) {
		t.Errorf("preChain ran %v, want %v", log, want)
	}

	log = nil
	drive(t, wiring.postChain)
	if want := []string{"ext-post"}; !slices.Equal(log, want) {
		t.Errorf("postChain ran %v, want %v", log, want)
	}
}

// externalPlugin always satisfies plugin.MiddlewareProvider, because Middleware
// is a method on the wrapper itself rather than on the wrapped plugin — the nil
// return is the only thing that makes the server's interface assertion a no-op
// for an external plugin that does not implement pdk.MiddlewareProvider. That is
// a different code path from the internal tier's assertion, so cover it.
func TestInitPlugins_ExternalTierWithoutMiddlewareContributesNone(t *testing.T) {
	ext := &fakeExternalPlugin{name: "api-cloud", spec: specWithScopes}

	wiring, err := initPlugins(testLogger(), http.NewServeMux(), emptyRegistry(t),
		&plugin.Deps{}, &pdk.Deps{}, nil, []pdk.Plugin{ext})
	if err != nil {
		t.Fatalf("initPlugins: unexpected error: %v", err)
	}
	if len(wiring.preChain) != 0 || len(wiring.postChain) != 0 {
		t.Fatalf("expected no middleware from a plugin that does not provide any, got pre=%d post=%d",
			len(wiring.preChain), len(wiring.postChain))
	}
}
