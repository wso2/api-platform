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

// scope_alternatives_test.go — end-to-end proof, through the real ScopeEnforcer,
// router and shipped files, that every scope an operation declares admits it
// (own node, parents, root, cross-resource grants), that every pre-tree name
// mapping onto one of them admits it too, and that everything else is denied.

package server

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/middleware"
)

var pathParamRe = regexp.MustCompile(`\{[^}]*\}`) // registry paths carry normalized, nameless {} placeholders

// TestEveryScopeAlternativeAdmitsThroughTheRealEnforcer drives the real
// enforcer over the real router, the merged registry and the shipped legacy
// mapping, for every operation of both shipped specs. It is the end-to-end
// statement of the authorization model:
//
//   - every permission an operation declares admits it — the node's own, each
//     parent's, the root's (ap:manage, ap:read) and any cross-resource grant;
//   - every pre-tree name that legacy-scope-mapping.yaml maps onto one of those
//     admits it too, so an old token or role file reaches exactly what a tree
//     token does, parent privilege included;
//   - a pre-tree name mapping elsewhere, a tree permission the operation does
//     not declare, and an empty scope claim are all denied.
//
// next is a sentinel rather than the mux: the handlers are constructed with nil
// services, so admitting a request into one would panic. The mux is still the
// route matcher, so pattern resolution is the production path.
func TestEveryScopeAlternativeAdmitsThroughTheRealEnforcer(t *testing.T) {
	mux := http.NewServeMux()
	registerAllRoutes(mux)
	registry := loadMergedRegistry(t)
	legacy := loadShippedLegacyScopeMap(t)

	enforcer, err := middleware.ScopeEnforcer(registry, middleware.ScopeEnforcerConfig{
		ValidationMode: middleware.ValidationModeScope,
		Enabled:        true,
		Routes:         mux,
		LegacyScopes:   legacy,
	})
	if err != nil {
		t.Fatalf("ScopeEnforcer: %v", err)
	}

	decide := func(method, path, scope string) int {
		admitted := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			admitted = true
			w.WriteHeader(http.StatusOK)
		})
		rec := httptest.NewRecorder()
		enforcer(next).ServeHTTP(rec, middleware.WithScope(httptest.NewRequest(method, path, nil), scope))
		if admitted != (rec.Code == http.StatusOK) {
			t.Fatalf("%s %s with %q: admitted=%v but status=%d", method, path, scope, admitted, rec.Code)
		}
		return rec.Code
	}

	legacyByTarget := map[string][]string{}
	for name, targets := range legacy {
		for _, target := range targets {
			legacyByTarget[target] = append(legacyByTarget[target], name)
		}
	}

	ops, admitChecks, denyChecks := 0, 0, 0
	for _, op := range sortedOperations(registry) {
		accepted, _ := registry.Lookup(op.Method, op.Path)
		acceptedSet := setOf(accepted)
		path := pathParamRe.ReplaceAllString(op.Path, "x1")

		// An operation the spec declares but no handler serves never reaches the
		// enforcer's scope check: the router resolves no pattern, the request
		// passes through, and the mux answers 404. Nothing to verify here;
		// TestScopeRegistryCoversEveryRegisteredRoute owns route coverage.
		probe, _ := http.NewRequest(op.Method, path, nil)
		if _, pattern := mux.Handler(probe); pattern == "" {
			t.Logf("%s %s is declared by the spec but registered by no handler — skipped", op.Method, op.Path)
			continue
		}
		ops++

		if got := decide(op.Method, path, ""); got != http.StatusForbidden {
			t.Errorf("%s %s admits an empty scope claim (status %d)", op.Method, op.Path, got)
		}
		denyChecks++

		for _, s := range accepted {
			if got := decide(op.Method, path, s); got != http.StatusOK {
				t.Errorf("%s %s denies %q, which it declares (status %d)", op.Method, op.Path, s, got)
			}
			admitChecks++
			for _, name := range legacyByTarget[s] {
				if got := decide(op.Method, path, name); got != http.StatusOK {
					t.Errorf("%s %s denies pre-tree %q, which maps onto declared %q (status %d)", op.Method, op.Path, name, s, got)
				}
				admitChecks++
			}
		}

		for s := range registry.AllScopes() {
			if acceptedSet[s] {
				continue
			}
			if got := decide(op.Method, path, s); got != http.StatusForbidden {
				t.Errorf("%s %s admits %q, which it does not declare (status %d)", op.Method, op.Path, s, got)
			}
			denyChecks++
		}
		for name, targets := range legacy {
			reaches := false
			for _, target := range targets {
				reaches = reaches || acceptedSet[target]
			}
			if reaches {
				continue
			}
			if got := decide(op.Method, path, name); got != http.StatusForbidden {
				t.Errorf("%s %s admits pre-tree %q, which maps onto %v, none of which it declares (status %d)", op.Method, op.Path, name, targets, got)
			}
			denyChecks++
		}
	}
	t.Logf("%d operations: %d admitting alternatives verified, %d denials verified", ops, admitChecks, denyChecks)
}
