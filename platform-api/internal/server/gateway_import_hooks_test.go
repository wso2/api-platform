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

// externalImportHookPlugin is an external plugin implementing both gateway import hooks.
type externalImportHookPlugin struct {
	*fakeExternalPlugin
}

func (*externalImportHookPlugin) AdmitGatewayImports(context.Context, pdk.GatewayImportBatch) map[string]pdk.GatewayImportRejection {
	return nil
}

func (*externalImportHookPlugin) GatewayArtifactsImported(context.Context, pdk.GatewayImportedBatch) {
}

// internalImportAdmitterPlugin is an internal plugin implementing only the admitter.
type internalImportAdmitterPlugin struct {
	*fakePlugin
}

func (*internalImportAdmitterPlugin) AdmitGatewayImports(context.Context, pdk.GatewayImportBatch) map[string]pdk.GatewayImportRejection {
	return nil
}

func TestInitPlugins_CollectsGatewayImportHooksFromBothTiers(t *testing.T) {
	ext := &externalImportHookPlugin{&fakeExternalPlugin{name: "api-cloud", spec: specWithScopes}}
	internal := &internalImportAdmitterPlugin{&fakePlugin{name: "in-tree", spec: specWithScopes}}

	wiring, err := initPlugins(testLogger(), http.NewServeMux(), emptyRegistry(t),
		&plugin.Deps{}, &pdk.Deps{}, []plugin.Plugin{internal}, []pdk.Plugin{ext})
	if err != nil {
		t.Fatalf("initPlugins: unexpected error: %v", err)
	}

	// Internal plugins come first, then external ones: the order the hooks run in.
	if len(wiring.importAdmitters) != 2 {
		t.Fatalf("admitters = %d, want 2 (one per tier)", len(wiring.importAdmitters))
	}
	if wiring.importAdmitters[0] != pdk.GatewayImportAdmitter(internal) {
		t.Errorf("first admitter is not the internal plugin")
	}
	// The external plugin is collected itself, not its externalPlugin wrapper.
	if wiring.importAdmitters[1] != pdk.GatewayImportAdmitter(ext) {
		t.Errorf("second admitter is not the external plugin")
	}
	if len(wiring.importObservers) != 1 || wiring.importObservers[0] != pdk.GatewayImportObserver(ext) {
		t.Errorf("observers = %v, want only the external plugin", wiring.importObservers)
	}
}

func TestInitPlugins_PluginsWithoutImportHooksContributeNone(t *testing.T) {
	wiring, err := initPlugins(testLogger(), http.NewServeMux(), emptyRegistry(t),
		&plugin.Deps{}, &pdk.Deps{},
		[]plugin.Plugin{&fakePlugin{name: "in-tree", spec: specWithScopes}},
		[]pdk.Plugin{&fakeExternalPlugin{name: "api-cloud", spec: specWithScopes}})
	if err != nil {
		t.Fatalf("initPlugins: unexpected error: %v", err)
	}
	if len(wiring.importAdmitters) != 0 || len(wiring.importObservers) != 0 {
		t.Fatalf("expected no import hooks, got %d admitters and %d observers",
			len(wiring.importAdmitters), len(wiring.importObservers))
	}
}
