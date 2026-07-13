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
	"testing"
)

// The whole grammar a config.toml may use: comments, tables, nested tables, dotted keys,
// both string forms and bare scalars.
func TestDecodeTOML_SupportedGrammar(t *testing.T) {
	tree, err := decodeTOML("# a comment\r\n" + `
log_level = "info"      # trailing comment
debug     = false
port      = 5380
ratio     = 1.5
oidc.enabled = true

[platform_api]
url = 'https://platform-api:9243'
tls_skip_verify = true

[oidc.claim_mappings]
role_claim_name = "platform_role"
`)
	if err != nil {
		t.Fatalf("decodeTOML() error = %v", err)
	}

	want := map[string]any{
		"log_level": "info",
		"debug":     false,
		"port":      int64(5380),
		"ratio":     1.5,
		"oidc": map[string]any{
			"enabled": true,
			"claim_mappings": map[string]any{
				"role_claim_name": "platform_role",
			},
		},
		"platform_api": map[string]any{
			"url":             "https://platform-api:9243",
			"tls_skip_verify": true,
		},
	}
	if !reflect.DeepEqual(tree, want) {
		t.Errorf("decodeTOML() =\n%#v\nwant\n%#v", tree, want)
	}
}

// A literal string is taken verbatim: the {{ env }} tokens carry JSON defaults whose
// escaped quotes must survive to configinterpolate, which does the unescaping.
func TestDecodeTOML_LiteralStringKeepsBackslashes(t *testing.T) {
	tree, err := decodeTOML(`versions = '{{ env "V" "[{\"version\":\"1.2\"}]" }}'  # a comment`)
	if err != nil {
		t.Fatalf("decodeTOML() error = %v", err)
	}
	want := `{{ env "V" "[{\"version\":\"1.2\"}]" }}`
	if got := tree["versions"]; got != want {
		t.Errorf("versions = %q, want %q", got, want)
	}
}

// A basic string expands the standard escapes, and a '#' inside it is content, not the
// start of a comment.
func TestDecodeTOML_BasicStringEscapes(t *testing.T) {
	tree, err := decodeTOML(`greeting = "a\tb\n\"c\" \\ é #4"`)
	if err != nil {
		t.Fatalf("decodeTOML() error = %v", err)
	}
	want := "a\tb\n\"c\" \\ é #4"
	if got := tree["greeting"]; got != want {
		t.Errorf("greeting = %q, want %q", got, want)
	}
}

// A missing config file is not an error, but a malformed one must fail startup rather
// than start the BFF on a half-read config.
func TestDecodeTOML_Rejects(t *testing.T) {
	reject := func(name, src string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			if tree, err := decodeTOML(src); err == nil {
				t.Errorf("decodeTOML(%q) = %#v, want an error", src, tree)
			}
		})
	}

	reject("unquoted string", `log_level = info`)
	reject("array", `scopes = ["a", "b"]`)
	reject("inline table", `oidc = { enabled = true }`)
	reject("array of tables", "[[gateways]]\nname = 'a'")
	reject("multi-line string", "key = \"\"\"\nline\n\"\"\"")
	reject("unterminated string", `url = "https://example.com`)
	reject("unterminated literal", `url = 'https://example.com`)
	reject("missing value", `url =`)
	reject("no equals sign", `url`)
	reject("text after value", `url = "a" junk`)
	reject("text after table", `[oidc] junk`)
	reject("duplicate key", "url = 'a'\nurl = 'b'")
	reject("table over a value", "oidc = 'a'\n[oidc]\nenabled = true")
	reject("unknown escape", `url = "a\qb"`)
}
