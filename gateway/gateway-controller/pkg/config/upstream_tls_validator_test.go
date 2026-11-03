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
	"testing"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// ============ Fixtures ============

func gatewayIdentityCert(name string) *models.StoredCertificate {
	return &models.StoredCertificate{
		Name: name, Usage: models.CertificateUsageIdentity,
		NotAfter: time.Now().Add(365 * 24 * time.Hour),
	}
}

func expiredGatewayIdentityCert(name string) *models.StoredCertificate {
	return &models.StoredCertificate{
		Name: name, Usage: models.CertificateUsageIdentity,
		NotAfter: time.Now().Add(-24 * time.Hour),
	}
}

// tlsUpstreamDef builds an upstreamDefinitions entry named "partner" with the
// given tls params, or none when nil, and one target per url.
func tlsUpstreamDef(tls map[string]interface{}, urls ...string) api.UpstreamDefinition {
	def := api.UpstreamDefinition{Name: "partner"}
	if tls != nil {
		def.Tls = &tls
	}
	for _, u := range urls {
		def.Upstreams = append(def.Upstreams, struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{Url: u})
	}
	return def
}

// restAPIWithUpstreamDefs attaches defs to a minimal valid RestAPI, pointing
// upstream.main at the first one by ref.
func restAPIWithUpstreamDefs(defs ...api.UpstreamDefinition) *api.RestAPI {
	cfg := createValidRestAPIConfig()
	cfg.Spec.UpstreamDefinitions = &defs
	if len(defs) > 0 {
		cfg.Spec.Upstream.Main = api.Upstream{Ref: stringPtr(defs[0].Name)}
	}
	return cfg
}

// restAPIWithInlineUpstreamTLS attaches a tls block directly to
// spec.upstream.main, which is never valid.
func restAPIWithInlineUpstreamTLS(tls map[string]interface{}) *api.RestAPI {
	cfg := createValidRestAPIConfig()
	cfg.Spec.Upstream.Main.Tls = &tls
	return cfg
}

// ============ ValidateRestAPI: refusals ============

func TestUpstreamTLSValidator_ValidateRestAPI_RefusalOutline(t *testing.T) {
	store := newFakeMtlsCertStore(
		gatewayIdentityCert("out-identity-a"),
		upstreamCA("out-backend-ca"),
	)
	validator := NewUpstreamTLSValidator(store, false)

	tests := []struct {
		name    string
		def     api.UpstreamDefinition
		field   string
		message string
	}{
		{
			name:    "identity that is not a string",
			def:     tlsUpstreamDef(map[string]interface{}{"identity": 42.0}, "https://mtls-backend-a:8443"),
			field:   "spec.upstreamDefinitions[0].tls.identity",
			message: "identity must be a string",
		},
		{
			name:    "trustedCAs that is not a list",
			def:     tlsUpstreamDef(map[string]interface{}{"trustedCAs": "out-backend-ca"}, "https://mtls-backend-a:8443"),
			field:   "spec.upstreamDefinitions[0].tls.trustedCAs",
			message: "trustedCAs must be a list of strings",
		},
		{
			name:    "trustedCAs element that is not a string",
			def:     tlsUpstreamDef(map[string]interface{}{"trustedCAs": []interface{}{"out-backend-ca", true}}, "https://mtls-backend-a:8443"),
			field:   "spec.upstreamDefinitions[0].tls.trustedCAs[1]",
			message: "trustedCAs must be a list of strings",
		},
		{
			name:    "verifyHostName that is not a boolean",
			def:     tlsUpstreamDef(map[string]interface{}{"verifyHostName": "false"}, "https://mtls-backend-a:8443"),
			field:   "spec.upstreamDefinitions[0].tls.verifyHostName",
			message: "verifyHostName must be true or false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validator.ValidateRestAPI(restAPIWithUpstreamDefs(tt.def))
			if !hasError(errs, tt.field, tt.message) {
				t.Fatalf("expected error field=%q message=%q, got %+v", tt.field, tt.message, errs)
			}
		})
	}
}

// ============ ValidateRestAPI: usage-mismatch and additional cases ============

func TestUpstreamTLSValidator_ValidateRestAPI_TlsEmptyBlock_Valid(t *testing.T) {
	store := newFakeMtlsCertStore()
	validator := NewUpstreamTLSValidator(store, false)

	errs := validator.ValidateRestAPI(restAPIWithUpstreamDefs(
		tlsUpstreamDef(map[string]interface{}{}, "https://mtls-backend-a:8443"),
	))
	if len(errs) != 0 {
		t.Fatalf("expected an empty tls block to be valid, got errors: %+v", errs)
	}
}

func TestUpstreamTLSValidator_ValidateRestAPI_MixedHttpHttpsTargets_NamesTheHttpOne(t *testing.T) {
	store := newFakeMtlsCertStore(gatewayIdentityCert("out-identity-a"))
	validator := NewUpstreamTLSValidator(store, false)

	def := tlsUpstreamDef(map[string]interface{}{"identity": "out-identity-a"},
		"https://mtls-backend-a:8443", "http://echo-backend:80")

	errs := validator.ValidateRestAPI(restAPIWithUpstreamDefs(def))

	want := "tls is configured but this target is http://; every target of a definition with tls must be https://"
	// The error must name upstreams[1], the http:// target.
	if hasError(errs, "spec.upstreamDefinitions[0].upstreams[0].url", want) {
		t.Fatalf("did not expect an error on the https:// target (index 0), got %+v", errs)
	}
	if !hasError(errs, "spec.upstreamDefinitions[0].upstreams[1].url", want) {
		t.Fatalf("expected error naming the http:// target at index 1, got %+v", errs)
	}
}

func TestUpstreamTLSValidator_ValidateRestAPI_WrongTypedTrustedCAs_NoEmptyListError(t *testing.T) {
	validator := NewUpstreamTLSValidator(newFakeMtlsCertStore(), false)

	for name, value := range map[string]interface{}{
		"not a list":               "out-backend-ca",
		"list of only non-strings": []interface{}{1.0},
	} {
		t.Run(name, func(t *testing.T) {
			errs := validator.ValidateRestAPI(restAPIWithUpstreamDefs(
				tlsUpstreamDef(map[string]interface{}{"trustedCAs": value}, "https://mtls-backend-a:8443"),
			))
			if hasError(errs, "spec.upstreamDefinitions[0].tls.trustedCAs",
				"omit trustedCAs to use the gateway trust bundle, or list at least one certificate") {
				t.Fatalf("a wrong-typed trustedCAs must not also be reported as an empty list, got %+v", errs)
			}
		})
	}
}

func TestUpstreamTLSValidator_ValidateRestAPI_TrustedCAsUnknownUsage_Rejected(t *testing.T) {
	store := newFakeMtlsCertStore(&models.StoredCertificate{Name: "out-odd", Usage: "archive"})
	validator := NewUpstreamTLSValidator(store, false)

	errs := validator.ValidateRestAPI(restAPIWithUpstreamDefs(tlsUpstreamDef(map[string]interface{}{
		"trustedCAs": []interface{}{"out-odd"},
	}, "https://mtls-backend-a:8443")))

	want := "out-odd has an unrecognized usage; trustedCAs takes usage: upstream certificates"
	if !hasError(errs, "spec.upstreamDefinitions[0].tls.trustedCAs[0]", want) {
		t.Fatalf("expected error field=%q message=%q, got %+v", "spec.upstreamDefinitions[0].tls.trustedCAs[0]", want, errs)
	}
}

func TestUpstreamTLSValidator_ValidateRestAPI_UppercaseHTTPSScheme_Accepted(t *testing.T) {
	store := newFakeMtlsCertStore(gatewayIdentityCert("out-identity-a"))
	validator := NewUpstreamTLSValidator(store, false)

	errs := validator.ValidateRestAPI(restAPIWithUpstreamDefs(
		tlsUpstreamDef(map[string]interface{}{"identity": "out-identity-a"}, "HTTPS://mtls-backend-a:8443"),
	))
	if len(errs) != 0 {
		t.Fatalf("expected an HTTPS:// target to be accepted, got %+v", errs)
	}
}

// ============ ResolveWarnings ============

func TestUpstreamTLSValidator_ResolveWarnings_VerifyHostNameDefaultTrue_NoWarning(t *testing.T) {
	store := newFakeMtlsCertStore(gatewayIdentityCert("out-identity-a"))
	validator := NewUpstreamTLSValidator(store, false)

	cfg := restAPIWithUpstreamDefs(tlsUpstreamDef(map[string]interface{}{
		"identity": "out-identity-a",
	}, "https://mtls-backend-a:8443"))

	warnings := validator.ResolveWarnings(*cfg)
	if hasWarning(warnings, WarningCodeTLSVerifyHostNameDisabled, "spec.upstreamDefinitions[0].tls.verifyHostName") {
		t.Fatalf("did not expect a verifyHostName warning when unset (defaults true), got %+v", warnings)
	}
}

func TestUpstreamTLSValidator_ResolveWarnings_IdentityExpiredSinceUpload(t *testing.T) {
	store := newFakeMtlsCertStore(expiredGatewayIdentityCert("out-identity-a"))
	validator := NewUpstreamTLSValidator(store, false)

	cfg := restAPIWithUpstreamDefs(tlsUpstreamDef(map[string]interface{}{
		"identity": "out-identity-a",
	}, "https://mtls-backend-a:8443"))

	warnings := validator.ResolveWarnings(*cfg)
	if !hasWarning(warnings, WarningCodeTLSIdentityExpired, "spec.upstreamDefinitions[0].tls.identity") {
		t.Fatalf("expected %s warning at spec.upstreamDefinitions[0].tls.identity, got %+v",
			WarningCodeTLSIdentityExpired, warnings)
	}
}

func TestUpstreamTLSValidator_ResolveWarnings_IdentityNotExpired_NoWarning(t *testing.T) {
	store := newFakeMtlsCertStore(gatewayIdentityCert("out-identity-a"))
	validator := NewUpstreamTLSValidator(store, false)

	cfg := restAPIWithUpstreamDefs(tlsUpstreamDef(map[string]interface{}{
		"identity": "out-identity-a",
	}, "https://mtls-backend-a:8443"))

	warnings := validator.ResolveWarnings(*cfg)
	if hasWarning(warnings, WarningCodeTLSIdentityExpired, "spec.upstreamDefinitions[0].tls.identity") {
		t.Fatalf("did not expect an identity-expired warning for a not-yet-expired identity, got %+v", warnings)
	}
}

func TestUpstreamTLSValidator_TrustRefusedWhileVerificationDisabled(t *testing.T) {
	store := newFakeMtlsCertStore(gatewayIdentityCert("out-identity-a"), upstreamCA("out-backend-ca"))
	validator := NewUpstreamTLSValidator(store, true)

	for _, tls := range []map[string]interface{}{
		{"trustedCAs": []interface{}{"out-backend-ca"}},
		{"verifyHostName": false},
	} {
		errs := validator.ValidateRestAPI(restAPIWithUpstreamDefs(tlsUpstreamDef(tls, "https://mtls-backend-a:8443")))
		if len(errs) == 0 {
			t.Fatalf("expected a refusal for %v while verification is disabled", tls)
		}
		found := false
		for _, e := range errs {
			if e.Field == "spec.upstreamDefinitions[0].tls" && e.Message == "per-upstream trust cannot be enforced while router.upstream.tls.disable_ssl_verification is on" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected the verification-disabled refusal, got %v", errs)
		}
	}

	// An identity alone asks for nothing Envoy cannot honour with verification off.
	if errs := validator.ValidateRestAPI(restAPIWithUpstreamDefs(tlsUpstreamDef(map[string]interface{}{"identity": "out-identity-a"}, "https://mtls-backend-a:8443"))); len(errs) != 0 {
		t.Errorf("an identity alone must deploy while verification is disabled, got %v", errs)
	}
}

// ============ ValidateAgent ============

// agentWithUpstreamDefs attaches defs to a minimal valid Agent, pointing its
// upstream at the first one by ref.
func agentWithUpstreamDefs(defs ...api.UpstreamDefinition) *api.AgentConfiguration {
	cfg := validAgent()
	cfg.Spec.UpstreamDefinitions = &defs
	if len(defs) > 0 {
		cfg.Spec.Upstream = api.AgentConfigData_Upstream{Ref: stringPtr(defs[0].Name)}
	}
	return &cfg
}

func TestUpstreamTLSValidator_ValidateAgent_ChecksTheTlsBlock(t *testing.T) {
	store := newFakeMtlsCertStore(gatewayIdentityCert("out-identity-a"), upstreamCA("out-backend-ca"))
	validator := NewUpstreamTLSValidator(store, false)

	tests := []struct {
		name    string
		agent   *api.AgentConfiguration
		field   string
		message string
	}{
		{
			name:    "identity naming a missing certificate",
			agent:   agentWithUpstreamDefs(tlsUpstreamDef(map[string]interface{}{"identity": "out-missing"}, "https://mtls-backend-a:8443")),
			field:   "spec.upstreamDefinitions[0].tls.identity",
			message: "no gateway identity named out-missing exists on this gateway",
		},
		{
			name:    "trustedCAs naming a gateway identity",
			agent:   agentWithUpstreamDefs(tlsUpstreamDef(map[string]interface{}{"trustedCAs": []interface{}{"out-identity-a"}}, "https://mtls-backend-a:8443")),
			field:   "spec.upstreamDefinitions[0].tls.trustedCAs[0]",
			message: "out-identity-a is a gateway identity (usage: identity); trustedCAs takes usage: upstream certificates",
		},
		{
			name:    "tls on an http target",
			agent:   agentWithUpstreamDefs(tlsUpstreamDef(map[string]interface{}{"identity": "out-identity-a"}, "http://echo-backend:80")),
			field:   "spec.upstreamDefinitions[0].upstreams[0].url",
			message: "tls is configured but this target is http://; every target of a definition with tls must be https://",
		},
		{
			name: "tls on the inline upstream",
			agent: func() *api.AgentConfiguration {
				cfg := validAgent()
				tls := map[string]interface{}{"identity": "out-identity-a"}
				cfg.Spec.Upstream.Tls = &tls
				return &cfg
			}(),
			field:   "spec.upstream.tls",
			message: "tls is not supported on an inline upstream; move it to upstreamDefinitions and reference it",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validator.ValidateAgent(tt.agent)
			if !hasError(errs, tt.field, tt.message) {
				t.Fatalf("expected error field=%q message=%q, got %+v", tt.field, tt.message, errs)
			}
		})
	}
}

func TestUpstreamTLSValidator_ValidateAgent_ValidTlsBlock(t *testing.T) {
	store := newFakeMtlsCertStore(gatewayIdentityCert("out-identity-a"), upstreamCA("out-backend-ca"))
	validator := NewUpstreamTLSValidator(store, false)

	agent := agentWithUpstreamDefs(tlsUpstreamDef(map[string]interface{}{
		"identity": "out-identity-a", "trustedCAs": []interface{}{"out-backend-ca"},
	}, "https://mtls-backend-a:8443"))
	if errs := validator.ValidateAgent(agent); len(errs) != 0 {
		t.Fatalf("expected no errors, got %+v", errs)
	}
}

func TestAgentValidator_WithUpstreamTLSValidator_RefusesAMissingIdentity(t *testing.T) {
	validator := NewAgentValidator().WithUpstreamTLSValidator(NewUpstreamTLSValidator(newFakeMtlsCertStore(), false))

	agent := agentWithUpstreamDefs(tlsUpstreamDef(map[string]interface{}{"identity": "out-missing"}, "https://mtls-backend-a:8443"))
	errs := validator.Validate(agent)
	if !hasError(errs, "spec.upstreamDefinitions[0].tls.identity", "no gateway identity named out-missing exists on this gateway") {
		t.Fatalf("expected the missing identity to be refused, got %+v", errs)
	}
}
