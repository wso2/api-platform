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

package goenv

import (
	"fmt"
	"strings"
	"testing"
)

func TestRedactCredentials(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "user and token",
			in:   "reading https://bot:s3cret@proxy.example.com/github.com/acme/p/@v/list: 401",
			want: "reading https://<redacted-credentials>@proxy.example.com/github.com/acme/p/@v/list: 401",
		},
		{
			name: "token only",
			in:   "git ls-remote https://ghp_abc123@github.com/acme/p",
			want: "git ls-remote https://<redacted-credentials>@github.com/acme/p",
		},
		{
			name: "several URLs",
			in:   "GOPROXY=https://a:b@one.example.com,http://c:d@two.example.com,direct",
			want: "GOPROXY=https://<redacted-credentials>@one.example.com,http://<redacted-credentials>@two.example.com,direct",
		},
		{
			name: "no credentials",
			in:   "github.com/acme/p@v1.0.0: invalid version: unknown revision v1.0.0",
			want: "github.com/acme/p@v1.0.0: invalid version: unknown revision v1.0.0",
		},
		{
			name: "at sign in path is not userinfo",
			in:   "https://proxy.golang.org/github.com/acme/p/@v/v1.0.0.zip",
			want: "https://proxy.golang.org/github.com/acme/p/@v/v1.0.0.zip",
		},
		{
			name: "raw at sign in the password",
			in:   "https://user:p@ss@host.example.com/x",
			want: "https://<redacted-credentials>@host.example.com/x",
		},
		{
			name: "email-style username",
			in:   "https://ci@example.com:s3cret@proxy.example.com/github.com/acme/p/@v/list",
			want: "https://<redacted-credentials>@proxy.example.com/github.com/acme/p/@v/list",
		},
		{
			name: "query token in a Go error line",
			in:   "reading https://goproxy.acme.com/github.com/acme/p/@v/list?access_token=s3cret: 401 Unauthorized",
			want: "reading https://goproxy.acme.com/github.com/acme/p/@v/list?<redacted-query>: 401 Unauthorized",
		},
		{
			name: "go-get lookup is kept",
			in:   "https://git.acme.com/acme/p?go-get=1: 404 Not Found",
			want: "https://git.acme.com/acme/p?go-get=1: 404 Not Found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RedactCredentials(tt.in); got != tt.want {
				t.Fatalf("RedactCredentials(%q)\n got %q\nwant %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestModuleSettingsLogFields(t *testing.T) {
	env := map[string]string{
		"GOPRIVATE": "github.com/acme/*",
		"GOPROXY":   "https://bot:s3cret@proxy.example.com,direct",
		"GOAUTH":    "mytool --token abc",
		"HOME":      "/root",
	}
	getenv := func(k string) string { return env[k] }
	exists := func(path string) bool { return path == "/root/.netrc" }

	fields := ModuleSettingsLogFields(getenv, exists)
	got := fmt.Sprint(fields...)

	for _, want := range []string{"GOPRIVATE", "github.com/acme/*", "<redacted-credentials>", "GOAUTH", "(set)", "netrc_present"} {
		if !strings.Contains(got, want) {
			t.Errorf("log fields %v missing %q", fields, want)
		}
	}
	for _, leaked := range []string{"s3cret", "abc"} {
		if strings.Contains(got, leaked) {
			t.Errorf("log fields %v leak %q", fields, leaked)
		}
	}
	if fields[len(fields)-1] != true {
		t.Errorf("netrc_present = %v, want true", fields[len(fields)-1])
	}
}

func TestModuleSettingsLogFieldsRedactsEveryEntry(t *testing.T) {
	env := map[string]string{
		// scheme-less entry (Go adds https://), email-style username, query token, pipe and comma separators
		"GOPROXY":   "bot:pw1@proxy.example.com,https://ci@example.com:pw2@other.example.com|https://goproxy.acme.com/?access_token=pw3,direct",
		"GOSUMDB":   "sum.acme.com+k3y/b64= https://u:pw4@sum.acme.com",
		"GOVCS":     "private:git|hg,public:git",
		"GOPRIVATE": "github.com/acme/*,*.corp.example.com",
	}
	fields := ModuleSettingsLogFields(func(k string) string { return env[k] }, func(string) bool { return false })

	got := map[string]any{}
	for i := 0; i+1 < len(fields); i += 2 {
		got[fields[i].(string)] = fields[i+1]
	}
	want := map[string]string{
		"GOPROXY":   "<redacted-credentials>@proxy.example.com,https://<redacted-credentials>@other.example.com|https://goproxy.acme.com/?<redacted>,direct",
		"GOSUMDB":   "sum.acme.com+k3y/b64= https://<redacted-credentials>@sum.acme.com",
		"GOVCS":     "private:git|hg,public:git",
		"GOPRIVATE": "github.com/acme/*,*.corp.example.com",
	}
	for key, w := range want {
		if got[key] != w {
			t.Errorf("%s logged as %q, want %q", key, got[key], w)
		}
	}
	for _, secret := range []string{"pw1", "pw2", "pw3", "pw4"} {
		if strings.Contains(fmt.Sprint(fields...), secret) {
			t.Errorf("log fields leak %q: %v", secret, fields)
		}
	}
}

func TestModuleSettingsLogFieldsNetrcEnvOverridesHome(t *testing.T) {
	env := map[string]string{"NETRC": "/run/secrets/netrc", "HOME": "/root"}
	var checked string
	fields := ModuleSettingsLogFields(func(k string) string { return env[k] }, func(path string) bool {
		checked = path
		return false
	})
	if checked != "/run/secrets/netrc" {
		t.Errorf("checked %q, want /run/secrets/netrc", checked)
	}
	if fields[len(fields)-1] != false {
		t.Errorf("netrc_present = %v, want false", fields[len(fields)-1])
	}
}
