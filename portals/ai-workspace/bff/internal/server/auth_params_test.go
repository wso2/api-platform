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

package server

import (
	"net/url"
	"strings"
	"testing"
)

// The login link is attacker-reachable — anyone can hand a user a URL pointing at this
// endpoint — so what it is willing to forward to the IDP is the whole of its security
// surface. These pin both halves: the three parameters that make the provider chooser
// skippable (and an enterprise sign-in resolvable), and the refusal of everything else.
func TestForwardableAuthParams(t *testing.T) {
	t.Run("forwards the supported parameters", func(t *testing.T) {
		got := forwardableAuthParams(url.Values{
			"fidp":       {"google"},
			"login_hint": {"someone@example.com"},
			"username":   {"someone@example.com"},
		})
		if got.Get("fidp") != "google" {
			t.Errorf("fidp = %q, want google", got.Get("fidp"))
		}
		if got.Get("login_hint") != "someone@example.com" {
			t.Errorf("login_hint = %q, want someone@example.com", got.Get("login_hint"))
		}
		if got.Get("username") != "someone@example.com" {
			t.Errorf("username = %q, want someone@example.com", got.Get("username"))
		}
	})

	t.Run("drops every other parameter", func(t *testing.T) {
		// Each of these would materially change the authorization request if it got
		// through: a wider scope, a redirect to somewhere else, a silent probe for an
		// existing session, or a replayed state.
		got := forwardableAuthParams(url.Values{
			"fidp":          {"google"},
			"scope":         {"openid admin"},
			"redirect_uri":  {"https://evil.example.com/steal"},
			"prompt":        {"none"},
			"state":         {"attacker-chosen"},
			"client_id":     {"another-client"},
			"response_type": {"token"},
		})
		if len(got) != 1 {
			t.Fatalf("forwarded %v, want only fidp", got)
		}
	})

	t.Run("rejects values that could break out of the redirect", func(t *testing.T) {
		// Checked on both a provider-shaped and an address-shaped parameter: the same
		// validator guards every forwarded name, and username is the one an attacker
		// would most plausibly fill with something other than an email.
		for _, param := range []string{"fidp", "username"} {
			for name, value := range map[string]string{
				"CR injection":    "google\r\nLocation: https://evil.example.com",
				"LF injection":    "google\nSet-Cookie: a=b",
				"query injection": "google&scope=admin",
				"fragment":        "google#x",
				"space":           "goo gle",
				"quote":           `google"`,
				"over length":     strings.Repeat("a", 257),
			} {
				got := forwardableAuthParams(url.Values{param: {value}})
				if got.Get(param) != "" {
					t.Errorf("%s via %s: forwarded %q, want it dropped", name, param, got.Get(param))
				}
			}
		}
	})

	t.Run("ignores blank and whitespace-only values", func(t *testing.T) {
		got := forwardableAuthParams(url.Values{"fidp": {"   "}, "login_hint": {""}, "username": {" "}})
		if len(got) != 0 {
			t.Errorf("forwarded %v, want nothing", got)
		}
	})

	t.Run("takes only the first value when a parameter repeats", func(t *testing.T) {
		// A repeated parameter is the classic way to smuggle a second value past a
		// check that only inspects one of them.
		got := forwardableAuthParams(url.Values{"fidp": {"google", "evil"}})
		if v := got["fidp"]; len(v) != 1 || v[0] != "google" {
			t.Errorf("fidp = %v, want exactly [google]", v)
		}
	})
}
