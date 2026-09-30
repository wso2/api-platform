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

package config

import (
	"strings"
	"testing"
)

const (
	testOrgA = "11111111-1111-1111-1111-111111111111"
	testOrgB = "22222222-2222-2222-2222-222222222222"
)

// setReadOnlyEnv pins both read-only env vars for a test so values from the outer
// environment cannot leak into LoadConfig. An empty value is skipped by the env
// provider, which is equivalent to the variable being unset.
func setReadOnlyEnv(t *testing.T, orgs, all string) {
	t.Helper()
	t.Setenv("READ_ONLY_ORGANIZATIONS", orgs)
	t.Setenv("READ_ONLY_ALL_ORGANIZATIONS", all)
}

func TestLoadConfig_ReadOnly_DefaultsOff(t *testing.T) {
	setReadOnlyEnv(t, "", "")

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.ReadOnly.Enabled() {
		t.Fatalf("expected read-only mode to be disabled by default, got %+v", cfg.ReadOnly)
	}
}

func TestLoadConfig_ReadOnly_EnvListIsTrimmedCanonicalisedAndDeduped(t *testing.T) {
	setReadOnlyEnv(t, " "+strings.ToUpper(testOrgA)+" , "+testOrgB+","+testOrgA+" ,, ", "")

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	want := []string{testOrgA, testOrgB}
	got := cfg.ReadOnly.Organizations
	if len(got) != len(want) {
		t.Fatalf("organizations: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("organizations[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
	if !cfg.ReadOnly.Enabled() {
		t.Error("expected read-only mode to be enabled when organizations are listed")
	}
	if cfg.ReadOnly.AllOrganizations {
		t.Error("expected all_organizations to remain false")
	}
}

func TestLoadConfig_ReadOnly_InvalidUUIDFailsStartup(t *testing.T) {
	setReadOnlyEnv(t, "my-org-handle", "")

	_, err := LoadConfig("")
	if err == nil {
		t.Fatal("expected LoadConfig to fail for a non-UUID organization entry")
	}
	if !strings.Contains(err.Error(), "read_only.organizations") {
		t.Errorf("error should name the offending key, got: %v", err)
	}
	if !strings.Contains(err.Error(), "my-org-handle") {
		t.Errorf("error should include the offending entry, got: %v", err)
	}
}

func TestLoadConfig_ReadOnly_AllOrganizations(t *testing.T) {
	setReadOnlyEnv(t, "", "true")

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if !cfg.ReadOnly.AllOrganizations || !cfg.ReadOnly.Enabled() {
		t.Fatalf("expected all-organizations read-only mode, got %+v", cfg.ReadOnly)
	}
}

func TestReadOnly_Enabled(t *testing.T) {
	var nilRO *ReadOnly
	if nilRO.Enabled() {
		t.Error("nil receiver should not be enabled")
	}
	if (&ReadOnly{}).Enabled() {
		t.Error("empty config should not be enabled")
	}
	if !(&ReadOnly{Organizations: []string{testOrgA}}).Enabled() {
		t.Error("listed organizations should enable read-only mode")
	}
	if !(&ReadOnly{AllOrganizations: true}).Enabled() {
		t.Error("all_organizations should enable read-only mode")
	}
}

func TestReadOnly_IsReadOnlyOrg(t *testing.T) {
	list := &ReadOnly{Organizations: []string{testOrgA}}
	all := &ReadOnly{AllOrganizations: true}
	var nilRO *ReadOnly

	tests := []struct {
		name string
		ro   *ReadOnly
		org  string
		want bool
	}{
		{"nil receiver", nilRO, testOrgA, false},
		{"disabled", &ReadOnly{}, testOrgA, false},
		{"listed exact", list, testOrgA, true},
		{"listed uppercase", list, strings.ToUpper(testOrgA), true},
		{"listed padded", list, "  " + testOrgA + " ", true},
		{"unlisted", list, testOrgB, false},
		{"empty org in list mode", list, "", false},
		{"all organizations", all, testOrgB, true},
		{"empty org in all-organizations mode", all, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.ro.IsReadOnlyOrg(tc.org); got != tc.want {
				t.Errorf("IsReadOnlyOrg(%q) = %v, want %v", tc.org, got, tc.want)
			}
		})
	}
}
