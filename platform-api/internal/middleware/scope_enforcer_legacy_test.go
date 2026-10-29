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

// scope_enforcer_legacy_test.go — ScopeEnforcer with a legacy scope map wired
// in: a request holding only a pre-tree name is admitted exactly where the
// permission it maps to is accepted and nowhere else, in both validation modes,
// and HasEffectiveScope sees the same translated set as the enforcer did.

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// enforcerTestLegacy translates pre-tree names onto the permissions
// enforcerTestSpec declares. ap:organization:create is a verb scope folded
// into :manage; ap:gateway:token:read is a sub-resource folded into its
// parent's :read.
var enforcerTestLegacy = LegacyScopeMap{
	"ap:organization:create":   {"ap:organization:manage"},
	"ap:gateway:token:read":    {"ap:gateway:read"},
	"ap:secret:create":         {"ap:secret:manage"},
	"ap:legacy_key:all:manage": {"ap:secret:manage"},
}

func legacyConfig() ScopeEnforcerConfig {
	cfg := enabledConfig()
	cfg.LegacyScopes = enforcerTestLegacy
	return cfg
}

// TestScopeEnforcer_TranslatesLegacyScopes: a token carrying only pre-tree
// names is authorized exactly as one holding the permission each name maps
// to — including through the parent entries the generated accepted list
// carries — while a legacy name that maps elsewhere, or is unknown to the
// mapping, still grants nothing.
func TestScopeEnforcer_TranslatesLegacyScopes(t *testing.T) {
	tests := []struct {
		name         string
		scope        string
		method, path string
		want         int
	}{
		{"legacy verb scope admits the manage operation", "ap:organization:create", http.MethodPost, "/api/v0.9/organizations", http.StatusOK},
		{"legacy verb scope admits the read operation via :manage", "ap:organization:create", http.MethodGet, "/api/v0.9/organizations", http.StatusForbidden},
		{"legacy sub-resource read admits the parent read", "ap:gateway:token:read", http.MethodGet, "/api/v0.9/gateways/g1", http.StatusOK},
		{"legacy sub-resource read does not admit the parent delete", "ap:gateway:token:read", http.MethodDelete, "/api/v0.9/gateways/g1", http.StatusForbidden},
		{"legacy scope of another resource", "ap:secret:create", http.MethodPost, "/api/v0.9/organizations", http.StatusForbidden},
		{"legacy name the mapping does not know", "ap:organization:update", http.MethodPost, "/api/v0.9/organizations", http.StatusForbidden},
		{"tree permission still matches directly", "ap:organization:manage", http.MethodPost, "/api/v0.9/organizations", http.StatusOK},
		{"both names held", "ap:organization:create ap:organization:manage", http.MethodPost, "/api/v0.9/organizations", http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec, reached := serveWithChain(t, legacyConfig(), tc.scope, tc.method, tc.path)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
			if reached != (tc.want == http.StatusOK) {
				t.Errorf("handler reached = %v, want %v", reached, tc.want == http.StatusOK)
			}
		})
	}
}

// Note: "GET /organizations" in enforcerTestSpec accepts only
// ap:organization:read, so ap:organization:create → ap:organization:manage is
// correctly denied there; the real spec lists :manage on every read operation.

// TestScopeEnforcer_LegacyScopesDisabled: with no mapping configured, a
// pre-tree name is just an undeclared string.
func TestScopeEnforcer_LegacyScopesDisabled(t *testing.T) {
	rec, reached := serveWithChain(t, enabledConfig(), "ap:organization:create", http.MethodPost, "/api/v0.9/organizations")
	if rec.Code != http.StatusForbidden || reached {
		t.Errorf("status = %d, reached = %v; want 403 and the handler not reached", rec.Code, reached)
	}
}

// TestScopeEnforcer_TranslatesLegacyScopesInRoleMode: an operator's
// role-to-scope mapping may still grant pre-tree names; the expanded roles are
// translated the same way a scope claim is.
func TestScopeEnforcer_TranslatesLegacyScopesInRoleMode(t *testing.T) {
	var reached bool
	mux := newTestMux(&reached)
	enforcer, err := ScopeEnforcer(newTestRegistry(t), ScopeEnforcerConfig{
		ValidationMode: ValidationModeRole,
		Enabled:        true,
		Routes:         mux,
		SkipPaths:      testSkipPaths,
		LegacyScopes:   enforcerTestLegacy,
	})
	if err != nil {
		t.Fatalf("ScopeEnforcer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v0.9/organizations", nil)
	req = req.WithContext(context.WithValue(req.Context(), keyRoles, []string{"ap:organization:create"}))
	rec := httptest.NewRecorder()
	enforcer(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !reached {
		t.Errorf("status = %d, reached = %v; want 200 and the handler reached", rec.Code, reached)
	}
}

// TestHasEffectiveScope_SeesTranslatedScopes: a handler's own check — the
// API-key ownership override is the real case — sees the same translated set
// the enforcer matched against, so a legacy-named override is honoured without
// the handler knowing the old name. The set is recorded even when enforcement
// is disabled, and HasEffectiveScope falls back to the raw claim when the
// enforcer never ran at all.
func TestHasEffectiveScope_SeesTranslatedScopes(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		var seen bool
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/v0.9/secrets", func(w http.ResponseWriter, r *http.Request) {
			seen = HasEffectiveScope(r, ValidationModeScope, "ap:secret:manage")
			w.WriteHeader(http.StatusOK)
		})
		enforcer, err := ScopeEnforcer(newTestRegistry(t), ScopeEnforcerConfig{
			ValidationMode: ValidationModeScope,
			Enabled:        enabled,
			Routes:         mux,
			LegacyScopes:   enforcerTestLegacy,
		})
		if err != nil {
			t.Fatalf("ScopeEnforcer: %v", err)
		}

		req := WithScope(httptest.NewRequest(http.MethodGet, "/api/v0.9/secrets", nil), "ap:legacy_key:all:manage")
		rec := httptest.NewRecorder()
		enforcer(mux).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("enabled=%v: status = %d, want 200", enabled, rec.Code)
		}
		if !seen {
			t.Errorf("enabled=%v: HasEffectiveScope did not see the translated scope", enabled)
		}
	}

	raw := WithScope(httptest.NewRequest(http.MethodGet, "/api/v0.9/secrets", nil), "ap:legacy_key:all:manage ap:secret:read")
	if HasEffectiveScope(raw, ValidationModeScope, "ap:secret:manage") {
		t.Error("without the enforcer, a legacy name was treated as its permission")
	}
	if !HasEffectiveScope(raw, ValidationModeScope, "ap:secret:read") {
		t.Error("without the enforcer, the raw scope claim was not consulted")
	}
}
