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

package opa

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func ask(t *testing.T, path, body string, headers map[string]string) (int, string) {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, DataPath+path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	New().Handler().ServeHTTP(rec, req)
	return rec.Code, strings.TrimSpace(rec.Body.String())
}

func TestService(t *testing.T) {
	s := New()
	if s.Name() != "opa" || s.Port() != Port || s.Stateful() {
		t.Fatalf("unexpected identity: %s %d %v", s.Name(), s.Port(), s.Stateful())
	}
}

func TestPolicyRule(t *testing.T) {
	cases := []struct {
		name, body, want string
		status           int
	}{
		{"get is allowed", `{"input":{"requestMethod":"GET","transportHeaders":{}}}`, `{"result":true}`, 200},
		{"post is denied", `{"input":{"requestMethod":"POST","transportHeaders":{}}}`, `{"result":false}`, 200},
		{"post with admin user is allowed", `{"input":{"requestMethod":"POST","transportHeaders":{"x-test-user":"admin"}}}`, `{"result":true}`, 200},
		{"forced allow", `{"input":{"requestMethod":"POST","transportHeaders":{"x-opa-decision":"allow"}}}`, `{"result":true}`, 200},
		{"forced deny", `{"input":{"requestMethod":"GET","transportHeaders":{"x-opa-decision":"deny"}}}`, `{"result":false}`, 200},
		{"forced undefined", `{"input":{"requestMethod":"GET","transportHeaders":{"x-opa-decision":"undefined"}}}`, `{}`, 200},
		{"forced invalid", `{"input":{"requestMethod":"GET","transportHeaders":{"x-opa-decision":"invalid"}}}`, `{"result":"yes"}`, 200},
		{"forced error", `{"input":{"requestMethod":"GET","transportHeaders":{"x-opa-decision":"error"}}}`, "", 500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := ask(t, "authz/policy/allow", tc.body, nil)
			if status != tc.status {
				t.Fatalf("status %d, want %d (%s)", status, tc.status, body)
			}
			if tc.want != "" && body != tc.want {
				t.Fatalf("body %s, want %s", body, tc.want)
			}
		})
	}
}

func TestContextRule(t *testing.T) {
	full := `{"input":{"requestMethod":"GET","requestPath":"/a","transportHeaders":{"h":"v"},"apiContext":{"apiName":"x"}}}`
	if _, body := ask(t, "authz/context/allow", full, nil); body != `{"result":true}` {
		t.Fatalf("complete input must be allowed, got %s", body)
	}
	for name, body := range map[string]string{
		"no method":  `{"input":{"requestPath":"/a","transportHeaders":{},"apiContext":{"apiName":"x"}}}`,
		"no path":    `{"input":{"requestMethod":"GET","transportHeaders":{},"apiContext":{"apiName":"x"}}}`,
		"no headers": `{"input":{"requestMethod":"GET","requestPath":"/a","apiContext":{"apiName":"x"}}}`,
		"no api":     `{"input":{"requestMethod":"GET","requestPath":"/a","transportHeaders":{}}}`,
	} {
		if _, got := ask(t, "authz/context/allow", body, nil); got != `{"result":false}` {
			t.Fatalf("%s must be denied, got %s", name, got)
		}
	}
}

func TestTokenRules(t *testing.T) {
	with := `{"input":{"apiContext":{"accessToken":"tok"}}}`
	without := `{"input":{"apiContext":{}}}`
	for path, want := range map[string][2]string{
		"authz/token/allow":   {`{"result":true}`, `{"result":false}`},
		"authz/notoken/allow": {`{"result":false}`, `{"result":true}`},
	} {
		if _, got := ask(t, path, with, nil); got != want[0] {
			t.Fatalf("%s with token: %s", path, got)
		}
		if _, got := ask(t, path, without, nil); got != want[1] {
			t.Fatalf("%s without token: %s", path, got)
		}
	}
}

func TestSecuredRule(t *testing.T) {
	if status, _ := ask(t, "authz/secured/allow", `{"input":{}}`, nil); status != http.StatusUnauthorized {
		t.Fatalf("missing token: status %d", status)
	}
	if status, _ := ask(t, "authz/secured/allow", `{"input":{}}`, map[string]string{"Authorization": "Bearer wrong"}); status != http.StatusUnauthorized {
		t.Fatalf("wrong token: status %d", status)
	}
	status, body := ask(t, "authz/secured/allow", `{"input":{}}`, map[string]string{"Authorization": "Bearer " + AccessKey})
	if status != 200 || body != `{"result":true}` {
		t.Fatalf("valid token: %d %s", status, body)
	}
}

func TestSlowRule(t *testing.T) {
	start := time.Now()
	status, body := ask(t, "authz/slow/allow", `{"input":{}}`, nil)
	if status != 200 || body != `{"result":true}` || time.Since(start) < SlowDelay {
		t.Fatalf("slow rule: %d %s after %s", status, body, time.Since(start))
	}
}

func TestUndefinedAndInvalid(t *testing.T) {
	if status, body := ask(t, "authz/unknown/allow", `{"input":{}}`, nil); status != 200 || body != `{}` {
		t.Fatalf("unknown document: %d %s", status, body)
	}
	if status, _ := ask(t, "authz/policy/allow", `not json`, nil); status != http.StatusBadRequest {
		t.Fatalf("invalid body: status %d", status)
	}
}
