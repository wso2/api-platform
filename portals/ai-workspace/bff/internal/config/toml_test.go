/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package config

import (
	"reflect"
	"strings"
	"testing"
)

// The parser accepts the full shape the shipped configs use: root keys, nested
// tables, both string quoting styles, scalars, comments, and CRLF endings.
func TestParseTOMLSubset(t *testing.T) {
	doc := strings.ReplaceAll(`# top comment
domain = "app.example.com"     # trailing comment
debug  = true
retries = 1_000
ratio   = 0.5

[platform_api]
url = 'https://api.example.com'

[oidc]
client_secret = '{{ env "APIP_AIW_OIDC_CLIENT_SECRET" }}'
greeting      = "line1\nline2 \"quoted\" é"

[oidc.claim_mappings]
organization_claim_name = "org_id"
`, "\n", "\r\n")

	got, err := parseTOMLSubset([]byte(doc))
	if err != nil {
		t.Fatalf("parseTOMLSubset() error = %v", err)
	}
	want := map[string]any{
		"domain":  "app.example.com",
		"debug":   true,
		"retries": int64(1000),
		"ratio":   0.5,
		"platform_api": map[string]any{
			"url": "https://api.example.com",
		},
		"oidc": map[string]any{
			"client_secret": `{{ env "APIP_AIW_OIDC_CLIENT_SECRET" }}`,
			"greeting":      "line1\nline2 \"quoted\" é",
			"claim_mappings": map[string]any{
				"organization_claim_name": "org_id",
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseTOMLSubset() = %#v, want %#v", got, want)
	}
}

// A parent table may be declared after its child ([a.b] then [a]) — TOML allows
// it, and the template's table ordering must not become load-order sensitive.
func TestParseTOMLSubsetParentAfterChild(t *testing.T) {
	got, err := parseTOMLSubset([]byte("[a.b]\nx = 1\n[a]\ny = 2\n"))
	if err != nil {
		t.Fatalf("parseTOMLSubset() error = %v", err)
	}
	want := map[string]any{"a": map[string]any{"b": map[string]any{"x": int64(1)}, "y": int64(2)}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseTOMLSubset() = %#v, want %#v", got, want)
	}
}

// Every unsupported or malformed construct must be an error, never a silent
// misread — the parser's whole safety argument is that it fails closed.
func TestParseTOMLSubsetRejects(t *testing.T) {
	cases := map[string]string{
		"array value":          `skip_paths = ["/health"]`,
		"inline table":         `oidc = { client_id = "x" }`,
		"array of tables":      "[[servers]]\nname = \"a\"",
		"multiline basic":      "s = \"\"\"\nx\n\"\"\"",
		"multiline literal":    "s = '''\nx\n'''",
		"dotted key":           `oidc.client_id = "x"`,
		"quoted key":           `"my key" = "x"`,
		"duplicate key":        "a = 1\na = 2",
		"duplicate table":      "[t]\n[t]",
		"key vs table":         "[oidc.claim_mappings]\nx = 1\n[oidc]\nclaim_mappings = 1",
		"table vs key":         "oidc = 1\n[oidc]",
		"unterminated string":  `s = "abc`,
		"unterminated literal": `s = 'abc`,
		"unterminated header":  "[oidc",
		"bad escape":           `s = "a\qb"`,
		"bad unicode escape":   `s = "\uZZZZ"`,
		"surrogate escape":     `s = "\uD800"`,
		"bare garbage value":   "a = maybe",
		"trailing garbage":     `a = "x" y`,
		"missing value":        "a =",
		"no equals":            "just a line",
		"datetime":             "a = 2026-07-13T00:00:00Z",
	}
	for name, doc := range cases {
		if _, err := parseTOMLSubset([]byte(doc)); err == nil {
			t.Errorf("%s: parseTOMLSubset(%q) succeeded, want error", name, doc)
		}
	}
}
