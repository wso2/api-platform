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

// externalSkipPathPlugin carries an AuthSkipPaths method that pdk declares no
// interface for.
type externalSkipPathPlugin struct {
	*fakeExternalPlugin
	paths []string
}

func (f *externalSkipPathPlugin) AuthSkipPaths() []string { return f.paths }

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

// External plugins go through the same scope guard as internal ones — the
// wrapper must not become a way around it.
func TestInitPlugins_ExternalTierIsHeldToTheSameGuards(t *testing.T) {
	t.Run("spec without scopes", func(t *testing.T) {
		ext := &fakeExternalPlugin{name: "api-cloud", spec: specWithoutScopes}

		_, err := initPlugins(testLogger(), http.NewServeMux(), emptyRegistry(t),
			&plugin.Deps{}, &pdk.Deps{}, nil, []pdk.Plugin{ext})
		if err == nil {
			t.Fatal("expected an external plugin with no declared scopes to abort startup")
		}
	})
}

// The external tier has no auth-skip-path hook: every route it mounts is
// authenticated. A plugin that happens to carry an AuthSkipPaths method — even
// one returning the root prefix — must not widen the auth bypass (GO-AUTH-004).
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
