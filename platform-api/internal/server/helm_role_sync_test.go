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

// helm_role_sync_test.go — the Helm chart's built-in admin role grants exactly
// what the shipped role-to-scope-mapping.yaml's ap_admin grants. The chart says
// it is kept in sync with that file; this is what makes the claim hold.

package server

import (
	"os"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/platform-api/internal/middleware"
)

const helmValuesPath = "../../../kubernetes/helm/platform-api-helm-chart/values.yaml"

// helmRoles finds every {name, scopes} role entry anywhere in the chart's
// values, so the test does not depend on the exact key path above it.
func helmRoles(node any, out map[string][]string) {
	switch v := node.(type) {
	case map[string]any:
		if name, ok := v["name"].(string); ok {
			if raw, ok := v["scopes"].([]any); ok {
				for _, s := range raw {
					if str, ok := s.(string); ok {
						out[name] = append(out[name], str)
					}
				}
				return
			}
		}
		for _, child := range v {
			helmRoles(child, out)
		}
	case []any:
		for _, child := range v {
			helmRoles(child, out)
		}
	}
}

func TestHelmAdminRoleMatchesShippedRoleMapping(t *testing.T) {
	data, err := os.ReadFile(helmValuesPath)
	if err != nil {
		t.Skipf("Helm chart not present: %v", err)
	}
	var values any
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatalf("parse %q: %v", helmValuesPath, err)
	}
	chart := map[string][]string{}
	helmRoles(values, chart)
	chartAdmin, ok := chart["ap_admin"]
	if !ok {
		t.Fatalf("%q defines no ap_admin role", helmValuesPath)
	}

	shipped, err := middleware.LoadRoleScopeMap(realRoleScopeMapPath)
	if err != nil {
		t.Fatalf("LoadRoleScopeMap(%q): %v", realRoleScopeMapPath, err)
	}
	want, got := append([]string(nil), shipped["ap_admin"]...), append([]string(nil), chartAdmin...)
	sort.Strings(want)
	sort.Strings(got)
	if strings.Join(want, " ") != strings.Join(got, " ") {
		t.Errorf("Helm ap_admin grants %v; shipped role-to-scope-mapping.yaml ap_admin grants %v — the chart says it is kept in sync", got, want)
	}
}
