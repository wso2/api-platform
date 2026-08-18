/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied.  See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package llmusage

import "testing"

func TestPathsMatch(t *testing.T) {
	tests := []struct {
		name        string
		requestPath string
		pattern     string
		want        bool
	}{
		{"root wildcard matches anything", "/chat/completions", "/*", true},
		{"exact match", "/responses", "/responses", true},
		{"prefix wildcard matches", "/chat/completions", "/chat/*", true},
		{"prefix wildcard matches deeper", "/chat/completions/stream", "/chat/*", true},
		{"different path does not match", "/responses", "/chat/completions", false},
		{"specific pattern does not match wildcard request", "/chat/*", "/chat/completions", false},
		{"empty pattern does not match", "/responses", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pathsMatch(tt.requestPath, tt.pattern); got != tt.want {
				t.Errorf("pathsMatch(%q, %q) = %v, want %v", tt.requestPath, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestPreferMoreSpecificPath(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		current   string
		want      bool
	}{
		{"exact beats wildcard", "/v1/exact", "/v1/*", true},
		{"wildcard loses to exact", "/v1/*", "/v1/exact", false},
		{"longer exact wins", "/v1/chat/completions", "/v1/chat", true},
		{"shorter exact loses", "/v1/chat", "/v1/chat/completions", false},
		{"longer wildcard wins over shorter wildcard", "/v1/chat/*", "/v1/*", true},
		{"equal length does not displace", "/v1/aaa", "/v1/bbb", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := preferMoreSpecificPath(tt.candidate, tt.current); got != tt.want {
				t.Errorf("preferMoreSpecificPath(%q, %q) = %v, want %v",
					tt.candidate, tt.current, got, tt.want)
			}
		})
	}
}

// Only a trailing wildcard is supported. An embedded one used to reduce to the
// prefix before it, so "/v1/*/usage" covered every route under "/v1/" and this
// resource's field locations were applied to unrelated requests.
func TestPathsMatch_OnlyTrailingWildcardIsHonoured(t *testing.T) {
	cases := []struct {
		pattern, requestPath string
		want                 bool
		why                  string
	}{
		{"/v1/*/usage", "/v1/chat/completions", false, "embedded wildcard must not cover unrelated routes"},
		{"/v1/*/usage", "/v1/embeddings", false, "embedded wildcard must not cover unrelated routes"},
		{"/v1/*/usage", "/v1/foo/usage", false, "embedded wildcards are not supported at all"},
		{"/v1/**", "/v1/anything", false, "repeated wildcards are rejected"},
		{"/v1/*/*", "/v1/a/b", false, "repeated wildcards are rejected"},

		// The supported forms keep working.
		{"/v1/*", "/v1/chat/completions", true, "trailing wildcard covers the prefix"},
		{"/*", "/anything", true, "catch-all"},
		{"/responses", "/responses", true, "exact match, as the shipped templates use"},
		{"/responses", "/responses/create", false, "exact pattern must not prefix-match"},
	}

	for _, c := range cases {
		if got := pathsMatch(c.requestPath, c.pattern); got != c.want {
			t.Errorf("pathsMatch(%q, %q) = %v, want %v — %s",
				c.requestPath, c.pattern, got, c.want, c.why)
		}
	}
}
