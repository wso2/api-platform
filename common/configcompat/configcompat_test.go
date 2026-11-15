/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package configcompat

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestMigrateDestinationGuard_MovesTheDeprecatedTable(t *testing.T) {
	guard := map[string]any{"enabled": true, "preset": "public_only"}
	raw := map[string]any{"controller": map[string]any{"http_client": map[string]any{
		LegacyDestinationGuardKey: guard,
		"timeouts":                map[string]any{"dial": "5s"},
	}}}

	path, err := MigrateDestinationGuard(raw, "controller")
	if err != nil {
		t.Fatalf("MigrateDestinationGuard: %v", err)
	}
	if path != "controller.http_client."+LegacyDestinationGuardKey {
		t.Fatalf("deprecated path = %q", path)
	}
	hc := raw["controller"].(map[string]any)["http_client"].(map[string]any)
	if !reflect.DeepEqual(hc[DestinationGuardKey], guard) {
		t.Fatalf("destination_guard = %v, want the moved table", hc[DestinationGuardKey])
	}
	if _, still := hc[LegacyDestinationGuardKey]; still {
		t.Fatal("the deprecated key must be removed")
	}
	if hc["timeouts"] == nil {
		t.Fatal("sibling keys must be untouched")
	}
}

func TestMigrateDestinationGuard_TopLevelHTTPClient(t *testing.T) {
	raw := map[string]any{"http_client": map[string]any{LegacyDestinationGuardKey: map[string]any{"max_redirects": 3}}}
	path, err := MigrateDestinationGuard(raw)
	if err != nil || path != "http_client."+LegacyDestinationGuardKey {
		t.Fatalf("path=%q err=%v", path, err)
	}
}

func TestMigrateDestinationGuard_NoOpWhenAbsent(t *testing.T) {
	for _, raw := range []map[string]any{
		{},
		{"platform_api": map[string]any{}},
		{"http_client": map[string]any{DestinationGuardKey: map[string]any{"enabled": true}}},
	} {
		path, err := MigrateDestinationGuard(raw)
		if err != nil || path != "" {
			t.Fatalf("raw=%v: path=%q err=%v, want a no-op", raw, path, err)
		}
	}
}

func TestMigrateDestinationGuard_BothNamesIsAnError(t *testing.T) {
	raw := map[string]any{"http_client": map[string]any{
		LegacyDestinationGuardKey: map[string]any{},
		DestinationGuardKey:       map[string]any{},
	}}
	_, err := MigrateDestinationGuard(raw)
	if err == nil || !strings.Contains(err.Error(), "both set") {
		t.Fatalf("err = %v, want a both-set error", err)
	}
}

func TestAliasLegacyDestinationGuard_ResolvesTheOldPath(t *testing.T) {
	guard := map[string]any{"max_redirects": 3}
	raw := map[string]any{"policy_engine": map[string]any{"http_client": map[string]any{DestinationGuardKey: guard}}}
	AliasLegacyDestinationGuard(raw, "policy_engine")
	hc := raw["policy_engine"].(map[string]any)["http_client"].(map[string]any)
	if !reflect.DeepEqual(hc[LegacyDestinationGuardKey], guard) {
		t.Fatalf("%s = %v, want the destination_guard table", LegacyDestinationGuardKey, hc[LegacyDestinationGuardKey])
	}
	if !reflect.DeepEqual(hc[DestinationGuardKey], guard) {
		t.Fatal("the destination_guard table must stay where it is")
	}
}

func TestAliasLegacyDestinationGuard_NoOpWithoutTheTableOrWhenTaken(t *testing.T) {
	taken := map[string]any{"enabled": false}
	for _, raw := range []map[string]any{
		{},
		{"http_client": map[string]any{"timeouts": map[string]any{}}},
		{"http_client": map[string]any{DestinationGuardKey: map[string]any{"enabled": true}, LegacyDestinationGuardKey: taken}},
	} {
		before := fmt.Sprint(raw)
		AliasLegacyDestinationGuard(raw)
		if after := fmt.Sprint(raw); after != before {
			t.Fatalf("raw changed from %s to %s, want a no-op", before, after)
		}
	}
}
