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

package config

import (
	"reflect"
	"strings"
	"testing"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

func scopeTestVHosts() VHostsConfig {
	return VHostsConfig{
		Main:    VHostEntry{Default: "*"},
		Sandbox: VHostEntry{Default: "sandbox-*"},
	}
}

func TestVHostsConfig_ServerName(t *testing.T) {
	tests := []struct {
		vhost  string
		want   string
		wantOK bool
	}{
		{vhost: "pay.example.com", want: "pay.example.com", wantOK: true},
		{vhost: " Pay.Example.COM ", want: "pay.example.com", wantOK: true},
		{vhost: "*.partners.example.com", want: "*.partners.example.com", wantOK: true},
		{vhost: "pay.example.com:8443", want: "pay.example.com", wantOK: true},
		{vhost: "pay.example.com:*", want: "pay.example.com", wantOK: true},
		{vhost: "*"},
		{vhost: "sandbox-*"},
		{vhost: "*."},
		{vhost: "pay-*"},
		{vhost: "*-pay.example.com"},
		{vhost: "pay.*.example.com"},
		{vhost: "*.*.example.com"},
		{vhost: "10.0.0.5"},
		{vhost: "10.0.0.5:8443"},
		{vhost: "::1"},
		{vhost: "[::1]:8443"},
		{vhost: ""},
		{vhost: "   "},
		{vhost: constants.VHostGatewayDefault},
		{vhost: " " + constants.VHostGatewayDefault + " "},
		{vhost: "pay.example.com."},
		{vhost: "pay.example.com.:8443"},
		{vhost: "pay..example.com"},
		{vhost: ".pay.example.com"},
		{vhost: "pay example.com"},
		{vhost: "pay_api.example.com"},
		{vhost: "pay.example.com/v1"},
		{vhost: "{{ env \"HOST\" }}"},
		{vhost: "pay.exa{mple.com"},
		{vhost: "pay.éxample.com"},
		{vhost: "a.example.com;b.example.com"},
	}
	vhosts := scopeTestVHosts()
	for _, tt := range tests {
		t.Run(tt.vhost, func(t *testing.T) {
			got, ok := vhosts.ServerName(tt.vhost)
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("ServerName(%q) = (%q, %v), want (%q, %v)", tt.vhost, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// A concrete default is shared by every API without its own hostname, so it
// is never scoped either.
func TestVHostsConfig_ServerName_ConcreteDefaults(t *testing.T) {
	vhosts := VHostsConfig{
		Main:    VHostEntry{Default: "api.example.com"},
		Sandbox: VHostEntry{Default: "sandbox.example.com"},
	}
	for _, vh := range []string{"api.example.com", "sandbox.example.com", "API.example.com", "Sandbox.Example.COM"} {
		if _, ok := vhosts.ServerName(vh); ok {
			t.Errorf("ServerName(%q) is scoped, want not scoped", vh)
		}
	}
	if got, ok := vhosts.ServerName("pay.example.com"); !ok || got != "pay.example.com" {
		t.Errorf("ServerName(pay.example.com) = (%q, %v)", got, ok)
	}
}

func TestVHostsConfig_Domains(t *testing.T) {
	vhosts := VHostsConfig{
		Main:    VHostEntry{Default: "*", Domains: []string{" api.example.com ", ""}},
		Sandbox: VHostEntry{Default: "sandbox-*", Domains: []string{" "}},
	}
	tests := map[string][]string{
		"*":               {"api.example.com"},
		"sandbox-*":       {"sandbox-*"},
		"pay.example.com": {"pay.example.com"},
	}
	for vhost, want := range tests {
		if got := vhosts.Domains(vhost); !reflect.DeepEqual(got, want) {
			t.Errorf("Domains(%q) = %v, want %v", vhost, got, want)
		}
	}
}

func TestRestAPIVhosts(t *testing.T) {
	withVhosts := func(main string, sandbox *string) api.APIConfigData {
		spec := createValidRestAPIConfig().Spec
		spec.Vhosts = &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{Main: main, Sandbox: sandbox}
		return spec
	}
	withSandboxUpstream := func(spec api.APIConfigData) api.APIConfigData {
		spec.Upstream.Sandbox = &api.Upstream{Ref: stringPtr("sandbox-backend")}
		return spec
	}

	tests := []struct {
		name        string
		spec        api.APIConfigData
		wantMain    []string
		wantSandbox string
		wantHas     bool
	}{
		{name: "no vhosts", spec: createValidRestAPIConfig().Spec, wantMain: []string{"*"}, wantSandbox: "sandbox-*"},
		{name: "several main entries", spec: withVhosts("a.example.com; b.example.com;a.example.com", nil),
			wantMain: []string{"a.example.com", "b.example.com"}, wantSandbox: "sandbox-*"},
		{name: "gateway-default sentinels are routed as written", spec: withVhosts(constants.VHostGatewayDefault, stringPtr(constants.VHostGatewayDefault)),
			wantMain: []string{constants.VHostGatewayDefault}, wantSandbox: constants.VHostGatewayDefault},
		{name: "sandbox kept as written", spec: withVhosts("a.example.com", stringPtr(" sb.example.com ")),
			wantMain: []string{"a.example.com"}, wantSandbox: " sb.example.com "},
		{name: "blank vhosts fall back to the defaults", spec: withVhosts(" ; ", stringPtr("  ")),
			wantMain: []string{"*"}, wantSandbox: "sandbox-*"},
		{name: "sandbox upstream by ref", spec: withSandboxUpstream(withVhosts("a.example.com", stringPtr("sb.example.com"))),
			wantMain: []string{"a.example.com"}, wantSandbox: "sb.example.com", wantHas: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			main, sandbox, has := RestAPIVhosts(tt.spec, scopeTestVHosts())
			if !reflect.DeepEqual(main, tt.wantMain) || sandbox != tt.wantSandbox || has != tt.wantHas {
				t.Fatalf("RestAPIVhosts = (%v, %q, %v), want (%v, %q, %v)", main, sandbox, has, tt.wantMain, tt.wantSandbox, tt.wantHas)
			}
		})
	}
}

func TestMtlsAuthValidator_HostnameScopeWarnings(t *testing.T) {
	vhostsOf := func(cfg *api.RestAPI, main string, sandbox *string) *api.RestAPI {
		cfg.Spec.Vhosts = &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{Main: main, Sandbox: sandbox}
		return cfg
	}
	sandboxUpstream := func(cfg *api.RestAPI) *api.RestAPI {
		cfg.Spec.Upstream.Sandbox = &api.Upstream{Url: stringPtr("http://sandbox:8080")}
		return cfg
	}
	mtlsAPI := func() *api.RestAPI { return restAPIWithAPILevelPolicies(mtlsPolicy(nil)) }

	tests := []struct {
		name         string
		apiConfig    *api.RestAPI
		httpsEnabled bool
		wantFields   []string
	}{
		{name: "no vhosts", apiConfig: mtlsAPI(), httpsEnabled: true, wantFields: []string{"spec.vhosts.main"}},
		{name: "own hostname", apiConfig: vhostsOf(mtlsAPI(), "pay.example.com", nil), httpsEnabled: true},
		{name: "wildcard hostname", apiConfig: vhostsOf(mtlsAPI(), "*.pay.example.com", nil), httpsEnabled: true},
		{name: "one unscopable entry among several", apiConfig: vhostsOf(mtlsAPI(), "pay.example.com;10.0.0.5", nil),
			httpsEnabled: true, wantFields: []string{"spec.vhosts.main"}},
		{name: "sandbox upstream on the sandbox default", apiConfig: sandboxUpstream(vhostsOf(mtlsAPI(), "pay.example.com", nil)),
			httpsEnabled: true, wantFields: []string{"spec.vhosts.sandbox"}},
		{name: "sandbox upstream with its own hostname", apiConfig: sandboxUpstream(vhostsOf(mtlsAPI(), "pay.example.com", stringPtr("sb.example.com"))),
			httpsEnabled: true},
		{name: "operation-level attachment", apiConfig: restAPIWithOperationLevelPolicy(mtlsPolicy(nil)),
			httpsEnabled: true, wantFields: []string{"spec.vhosts.main"}},
		{name: "no mtls-auth", apiConfig: createValidRestAPIConfig(), httpsEnabled: true},
		{name: "HTTPS disabled", apiConfig: mtlsAPI(), httpsEnabled: false},
		{name: "gateway-default sentinel", apiConfig: vhostsOf(mtlsAPI(), constants.VHostGatewayDefault, nil),
			httpsEnabled: true, wantFields: []string{"spec.vhosts.main"}},
		{name: "trailing dot", apiConfig: vhostsOf(mtlsAPI(), "pay.example.com.", nil),
			httpsEnabled: true, wantFields: []string{"spec.vhosts.main"}},
		{name: "non-DNS characters", apiConfig: vhostsOf(mtlsAPI(), "pay_api.example.com", nil),
			httpsEnabled: true, wantFields: []string{"spec.vhosts.main"}},
		{name: "sandbox with surrounding whitespace", apiConfig: sandboxUpstream(vhostsOf(mtlsAPI(), "pay.example.com", stringPtr(" sb.example.com "))),
			httpsEnabled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewMtlsAuthValidator(newFakeMtlsCertStore(clientCA("partner-a")), tt.httpsEnabled, false, mtlsAuthTestSchema()).
				WithVHosts(scopeTestVHosts())
			warnings := v.HostnameScopeWarnings(*tt.apiConfig)

			var gotFields []string
			for _, w := range warnings {
				if w.Code != WarningCodeMTLSHostnameNotScoped {
					continue
				}
				gotFields = append(gotFields, w.Field)
				if want := "give it its own " + strings.TrimPrefix(w.Field, "spec."); !strings.Contains(w.Message, want) {
					t.Errorf("warning for %s = %q, want it to contain %q", w.Field, w.Message, want)
				}
			}
			if !reflect.DeepEqual(gotFields, tt.wantFields) {
				t.Fatalf("MTLS_HOSTNAME_NOT_SCOPED fields = %v, want %v", gotFields, tt.wantFields)
			}
		})
	}
}

// Without router vhosts the validator raises no hostname warning.
func TestMtlsAuthValidator_HostnameScopeWarnings_NoVHosts(t *testing.T) {
	v := NewMtlsAuthValidator(newFakeMtlsCertStore(clientCA("partner-a")), true, false, mtlsAuthTestSchema())
	warnings := v.HostnameScopeWarnings(*restAPIWithAPILevelPolicies(mtlsPolicy(nil)))
	for _, w := range warnings {
		if w.Code == WarningCodeMTLSHostnameNotScoped {
			t.Fatalf("unexpected %s warning: %+v", w.Code, w)
		}
	}
}
