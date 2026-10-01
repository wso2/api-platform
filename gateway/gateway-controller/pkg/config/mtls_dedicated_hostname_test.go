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
	"fmt"
	"reflect"
	"testing"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

const (
	dedicatedMainMessage = "this gateway requires every mtls-auth API to have its own hostname " +
		"(an exact name or a leading *.); set vhosts.main"
	dedicatedSandboxMessage = "this gateway requires every mtls-auth API to have its own hostname " +
		"(an exact name or a leading *.); set vhosts.sandbox"
)

func withVhosts(cfg *api.RestAPI, main string, sandbox *string) *api.RestAPI {
	cfg.Spec.Vhosts = &struct {
		Main    string  `json:"main" yaml:"main"`
		Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
	}{Main: main, Sandbox: sandbox}
	return cfg
}

func withSandboxUpstream(cfg *api.RestAPI) *api.RestAPI {
	cfg.Spec.Upstream.Sandbox = &api.Upstream{Url: stringPtr("http://sandbox:8080")}
	return cfg
}

// hostnameErrors returns the validation errors on spec.vhosts.* that
// ValidateRestAPI reports.
func hostnameErrors(v *MtlsAuthValidator, cfg *api.RestAPI) []ValidationError {
	var found []ValidationError
	for _, e := range v.ValidateRestAPI(cfg) {
		if e.Field == "spec.vhosts.main" || e.Field == "spec.vhosts.sandbox" {
			found = append(found, e)
		}
	}
	return found
}

func dedicatedHostnameValidator(required bool, relays ...string) *MtlsAuthValidator {
	certs := []*models.StoredCertificate{clientCA("partner-a")}
	for _, name := range relays {
		certs = append(certs, relayCA(name))
	}
	return NewMtlsAuthValidator(newFakeMtlsCertStore(certs...), true, false, mtlsAuthTestSchema()).
		WithVHosts(scopeTestVHosts()).
		WithDedicatedHostnameRequired(required)
}

func TestMtlsAuthValidator_DedicatedHostnameRequired(t *testing.T) {
	mtlsAPI := func() *api.RestAPI { return restAPIWithAPILevelPolicies(mtlsPolicy(nil)) }

	tests := []struct {
		name      string
		apiConfig *api.RestAPI
		want      []ValidationError
	}{
		{name: "no vhosts", apiConfig: mtlsAPI(),
			want: []ValidationError{{Field: "spec.vhosts.main", Message: dedicatedMainMessage}}},
		{name: "sandbox upstream on the sandbox default",
			apiConfig: withSandboxUpstream(withVhosts(mtlsAPI(), "pay.example.com", nil)),
			want:      []ValidationError{{Field: "spec.vhosts.sandbox", Message: dedicatedSandboxMessage}}},
		{name: "main and sandbox both on defaults", apiConfig: withSandboxUpstream(mtlsAPI()),
			want: []ValidationError{
				{Field: "spec.vhosts.main", Message: dedicatedMainMessage},
				{Field: "spec.vhosts.sandbox", Message: dedicatedSandboxMessage},
			}},
		{name: "IP address", apiConfig: withVhosts(mtlsAPI(), "10.0.0.5", nil),
			want: []ValidationError{{Field: "spec.vhosts.main", Message: dedicatedMainMessage}}},
		{name: "gateway-default sentinel", apiConfig: withVhosts(mtlsAPI(), constants.VHostGatewayDefault, nil),
			want: []ValidationError{{Field: "spec.vhosts.main", Message: dedicatedMainMessage}}},
		{name: "trailing dot", apiConfig: withVhosts(mtlsAPI(), "pay.example.com.", nil),
			want: []ValidationError{{Field: "spec.vhosts.main", Message: dedicatedMainMessage}}},
		{name: "non-DNS characters", apiConfig: withVhosts(mtlsAPI(), "pay_api.example.com", nil),
			want: []ValidationError{{Field: "spec.vhosts.main", Message: dedicatedMainMessage}}},
		{name: "one unscopable entry among several", apiConfig: withVhosts(mtlsAPI(), "pay.example.com;10.0.0.5", nil),
			want: []ValidationError{{Field: "spec.vhosts.main", Message: dedicatedMainMessage}}},
		{name: "own hostname", apiConfig: withVhosts(mtlsAPI(), "pay.example.com", nil)},
		{name: "wildcard hostname", apiConfig: withVhosts(mtlsAPI(), "*.pay.example.com", nil)},
		{name: "sandbox upstream with its own hostname",
			apiConfig: withSandboxUpstream(withVhosts(mtlsAPI(), "pay.example.com", stringPtr("sb.example.com")))},
		{name: "no mtls-auth", apiConfig: createValidRestAPIConfig()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := dedicatedHostnameValidator(true)
			if got := hostnameErrors(v, tt.apiConfig); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("hostname errors = %+v, want %+v", got, tt.want)
			}
			if w := v.HostnameScopeWarnings(*tt.apiConfig); len(w) != 0 {
				t.Fatalf("HostnameScopeWarnings = %+v, want none while a dedicated hostname is required", w)
			}
		})
	}
}

// A relay entry makes the listener ask every connection, but it is the
// admin's pool choice, so an API with its own hostname is still accepted.
func TestMtlsAuthValidator_DedicatedHostnameRequired_RelayEntryDoesNotRefuse(t *testing.T) {
	v := dedicatedHostnameValidator(true, "front-proxy")
	cfg := withVhosts(restAPIWithAPILevelPolicies(mtlsPolicy(nil)), "pay.example.com", nil)
	if errs := v.ValidateRestAPI(cfg); len(errs) != 0 {
		t.Fatalf("ValidateRestAPI = %+v, want no errors", errs)
	}
}

// Off, an API without its own hostname deploys and carries the warning.
func TestMtlsAuthValidator_DedicatedHostnameNotRequired(t *testing.T) {
	v := dedicatedHostnameValidator(false)
	cfg := restAPIWithAPILevelPolicies(mtlsPolicy(nil))
	if errs := hostnameErrors(v, cfg); len(errs) != 0 {
		t.Fatalf("hostname errors = %+v, want none", errs)
	}
	warnings := v.HostnameScopeWarnings(*cfg)
	if len(warnings) != 1 || warnings[0].Code != WarningCodeMTLSHostnameNotScoped || warnings[0].Field != "spec.vhosts.main" {
		t.Fatalf("HostnameScopeWarnings = %+v, want one %s on spec.vhosts.main", warnings, WarningCodeMTLSHostnameNotScoped)
	}
}

// The requirement follows the listener: with HTTPS disabled no certificate
// is requested, so nothing is refused on hostname grounds.
func TestMtlsAuthValidator_DedicatedHostnameRequired_HTTPSDisabled(t *testing.T) {
	v := NewMtlsAuthValidator(newFakeMtlsCertStore(clientCA("partner-a")), false, true, mtlsAuthTestSchema()).
		WithVHosts(scopeTestVHosts()).
		WithDedicatedHostnameRequired(true)
	if errs := hostnameErrors(v, restAPIWithAPILevelPolicies(mtlsPolicy(nil))); len(errs) != 0 {
		t.Fatalf("hostname errors = %+v, want none", errs)
	}
}

// Through the deploy-time API validator, the refusal is a validation error
// like every other mtls-auth one.
func TestAPIValidator_DedicatedHostnameRequired(t *testing.T) {
	def := mtlsAuthTestDefinition()
	defs := map[string]models.PolicyDefinition{def.Name + "|" + def.Version: def}
	mv := NewMtlsAuthValidator(newFakeMtlsCertStore(clientCA("partner-a")), true, false, MtlsAuthParameterSchema(defs)).
		WithVHosts(scopeTestVHosts())

	validator := NewAPIValidator()
	validator.SetPolicyValidator(NewPolicyValidator(defs, mv))

	cfg := restAPIWithAPILevelPolicies(mtlsPolicy(nil))
	if errs := validator.Validate(cfg); len(errs) != 0 {
		t.Fatalf("with the setting off, Validate = %+v, want no errors", errs)
	}

	mv.WithDedicatedHostnameRequired(true)
	want := []ValidationError{{Field: "spec.vhosts.main", Message: dedicatedMainMessage}}
	if errs := validator.Validate(restAPIWithAPILevelPolicies(mtlsPolicy(nil))); !reflect.DeepEqual(errs, want) {
		t.Fatalf("with the setting on, Validate = %+v, want %+v", errs, want)
	}
}

func TestUndedicatedHostnames(t *testing.T) {
	got := UndedicatedHostnames(*withSandboxUpstream(restAPIWithAPILevelPolicies(mtlsPolicy(nil))), scopeTestVHosts())
	want := []UndedicatedHostname{
		{Field: "spec.vhosts.main", Setting: "vhosts.main"},
		{Field: "spec.vhosts.sandbox", Setting: "vhosts.sandbox"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UndedicatedHostnames = %+v, want %+v", got, want)
	}
	if got := UndedicatedHostnames(*createValidRestAPIConfig(), scopeTestVHosts()); got != nil {
		t.Fatalf("UndedicatedHostnames without mtls-auth = %+v, want nil", got)
	}
}

// With client_certificate_request all_connections the listener asks every
// connection, so no hostname is warned about or refused, whether or not a
// dedicated hostname is required.
func TestMtlsAuthValidator_AllConnectionsAsked(t *testing.T) {
	apis := map[string]*api.RestAPI{
		"no vhosts":              restAPIWithAPILevelPolicies(mtlsPolicy(nil)),
		"IP address":             withVhosts(restAPIWithAPILevelPolicies(mtlsPolicy(nil)), "10.0.0.5", nil),
		"sandbox on its default": withSandboxUpstream(withVhosts(restAPIWithAPILevelPolicies(mtlsPolicy(nil)), "pay.example.com", nil)),
	}
	for _, required := range []bool{false, true} {
		for name, cfg := range apis {
			t.Run(fmt.Sprintf("%s, dedicated hostname required %t", name, required), func(t *testing.T) {
				v := dedicatedHostnameValidator(required).WithAllConnectionsAsked(true)
				if errs := hostnameErrors(v, cfg); len(errs) != 0 {
					t.Fatalf("hostname errors = %+v, want none", errs)
				}
				if w := v.HostnameScopeWarnings(*cfg); len(w) != 0 {
					t.Fatalf("HostnameScopeWarnings = %+v, want none", w)
				}
			})
		}
	}
}
