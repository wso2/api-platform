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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/clientca"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"gopkg.in/yaml.v3"
)

// ============ Fake certificate store ============

// fakeMtlsCertStore is a minimal, in-memory MtlsAuthCertificateStore for
// tests: no database involved, just the two lookups the validator needs.
type fakeMtlsCertStore struct {
	byUsage   map[string][]*models.StoredCertificate
	byName    map[string]*models.StoredCertificate
	listCalls int
}

func newFakeMtlsCertStore(certs ...*models.StoredCertificate) *fakeMtlsCertStore {
	s := &fakeMtlsCertStore{
		byUsage: make(map[string][]*models.StoredCertificate),
		byName:  make(map[string]*models.StoredCertificate),
	}
	for _, c := range certs {
		usage := c.Usage
		if usage == "" {
			usage = models.CertificateUsageUpstream
		}
		s.byUsage[usage] = append(s.byUsage[usage], c)
		s.byName[c.Name] = c
	}
	return s
}

func (s *fakeMtlsCertStore) GetCertificateByName(name string) (*models.StoredCertificate, error) {
	c, ok := s.byName[name]
	if !ok {
		return nil, fmt.Errorf("no certificate named %q", name)
	}
	return c, nil
}

func (s *fakeMtlsCertStore) ListCertificatesByUsage(usage string) ([]*models.StoredCertificate, error) {
	s.listCalls++
	return s.byUsage[usage], nil
}

func clientCA(name string) *models.StoredCertificate {
	return &models.StoredCertificate{Name: name, Usage: models.CertificateUsageDownstream, Role: models.CertificateRoleClient}
}

func relayCA(name string) *models.StoredCertificate {
	return &models.StoredCertificate{Name: name, Usage: models.CertificateUsageDownstream, Role: models.CertificateRoleRelay}
}

func upstreamCA(name string) *models.StoredCertificate {
	return &models.StoredCertificate{Name: name, Usage: models.CertificateUsageUpstream}
}

// ============ RestAPI builders ============

// mtlsPolicy builds an mtls-auth policy entry with the given params (nil for
// "no params at all", matching an omitted params: block).
func mtlsPolicy(params map[string]interface{}) api.Policy {
	p := api.Policy{Name: MtlsAuthPolicyName, Version: "v1"}
	if params != nil {
		p.Params = &params
	}
	return p
}

func namedPolicy(name string) api.Policy {
	return api.Policy{Name: name, Version: "v1"}
}

// restAPIWithAPILevelPolicies attaches the given policies at API level (spec.policies).
func restAPIWithAPILevelPolicies(policies ...api.Policy) *api.RestAPI {
	cfg := createValidRestAPIConfig()
	cfg.Spec.Policies = &policies
	return cfg
}

// restAPIWithOperationLevelPolicy attaches a single policy on the first (and
// only) operation createValidRestAPIConfig defines.
func restAPIWithOperationLevelPolicy(p api.Policy) *api.RestAPI {
	cfg := createValidRestAPIConfig()
	cfg.Spec.Operations[0].Policies = &[]api.Policy{p}
	return cfg
}

// hasError reports whether errs contains an entry with exactly this field and message.
func hasError(errs []ValidationError, field, message string) bool {
	for _, e := range errs {
		if e.Field == field && e.Message == message {
			return true
		}
	}
	return false
}

// hasWarning reports whether warnings contains an entry with this code and field.
func hasWarning(warnings []clientca.Warning, code, field string) bool {
	for _, w := range warnings {
		if w.Code == code && w.Field == field {
			return true
		}
	}
	return false
}

// ============ ValidateRestAPI: accept-list rejections ============

func TestMtlsAuthValidator_ValidateRestAPI_HTTPSDisabled(t *testing.T) {
	store := newFakeMtlsCertStore(clientCA("listener-partner-a"))
	v := NewMtlsAuthValidator(store, false, false, mtlsAuthTestSchema())
	apiConfig := restAPIWithAPILevelPolicies(mtlsPolicy(nil))

	errs := v.ValidateRestAPI(apiConfig)

	want := "mtls-auth requires the HTTPS listener, which is disabled on this gateway"
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 error, got %d: %+v", len(errs), errs)
	}
	if !hasError(errs, "spec.policies[0]", want) {
		t.Fatalf("expected error {field: spec.policies[0], message: %q}, got %+v", want, errs)
	}
}

func TestMtlsAuthValidator_ValidateRestAPI_OperationLevelFieldPath(t *testing.T) {
	store := newFakeMtlsCertStore(clientCA("listener-partner-a"))
	v := NewMtlsAuthValidator(store, true, false, mtlsAuthTestSchema())

	acceptParams := map[string]interface{}{"accept": []interface{}{
		map[string]interface{}{"ca": "listener-partner-b"}, // not in the pool
	}}
	apiConfig := restAPIWithOperationLevelPolicy(mtlsPolicy(acceptParams))

	errs := v.ValidateRestAPI(apiConfig)

	want := "no client-CA authority named listener-partner-b exists on this gateway"
	if !hasError(errs, "spec.operations[0].policies[0].params.accept[0].ca", want) {
		t.Fatalf("expected operation-level field path, got %+v", errs)
	}
}

// ============ ResolveMtlsAuthForResponse ============

func TestMtlsAuthValidator_ResolveMtlsAuthForResponse_AcceptOmitted_TwoAuthorities(t *testing.T) {
	store := newFakeMtlsCertStore(
		clientCA("listener-partner-a"),
		clientCA("listener-partner-b"),
		relayCA("listener-edge-lb"),
		upstreamCA("listener-backend-trust"),
	)
	v := NewMtlsAuthValidator(store, true, false, mtlsAuthTestSchema())

	original := restAPIWithAPILevelPolicies(mtlsPolicy(nil))
	resolved, warnings := v.ResolveMtlsAuthForResponse(*original)

	// Input untouched: the original policy's Params must remain nil.
	if (*original.Spec.Policies)[0].Params != nil {
		t.Fatalf("expected original policy Params to remain nil, got %v", *(*original.Spec.Policies)[0].Params)
	}

	resolvedPolicies := *resolved.Spec.Policies
	accept, ok := (*resolvedPolicies[0].Params)["accept"].([]interface{})
	if !ok {
		t.Fatalf("expected resolved accept to be []interface{}, got %T", (*resolvedPolicies[0].Params)["accept"])
	}
	if len(accept) != 2 {
		t.Fatalf("expected exactly 2 resolved accept entries, got %d: %+v", len(accept), accept)
	}
	wantNames := []string{"listener-partner-a", "listener-partner-b"}
	for i, entryRaw := range accept {
		entry, ok := entryRaw.(map[string]interface{})
		if !ok {
			t.Fatalf("accept[%d] = %T, want map[string]interface{}", i, entryRaw)
		}
		if entry["ca"] != wantNames[i] {
			t.Errorf("accept[%d].ca = %v, want %v", i, entry["ca"], wantNames[i])
		}
	}

	if !hasWarning(warnings, WarningCodeMTLSAcceptInheritsPool, "spec.policies[0].params.accept") {
		t.Fatalf("expected MTLS_ACCEPT_INHERITS_POOL warning for spec.policies[0].params.accept, got %+v", warnings)
	}
}

func TestMtlsAuthValidator_ResolveMtlsAuthForResponse_NarrowedEntry_NoUnnarrowedWarning(t *testing.T) {
	v := NewMtlsAuthValidator(newFakeMtlsCertStore(clientCA("listener-partner-a"), clientCA("listener-partner-b")), true, false, mtlsAuthTestSchema())

	acceptParams := map[string]interface{}{"accept": []interface{}{
		map[string]interface{}{"ca": "listener-partner-a", "match": map[string]interface{}{"uriSANs": []interface{}{"urn:x"}}},
	}}
	_, warnings := v.ResolveMtlsAuthForResponse(*restAPIWithAPILevelPolicies(mtlsPolicy(acceptParams)))

	if hasWarning(warnings, WarningCodeMTLSAcceptUnnarrowed, "spec.policies[0].params.accept[0]") {
		t.Fatalf("expected no MTLS_ACCEPT_UNNARROWED for a narrowed entry, got %+v", warnings)
	}
}

// TestMtlsAuthValidator_ResolveMtlsAuthForResponse_AcceptNamesRelayAuthority
// covers the same authority pooled under two names — once as a relay, once
// as a client entry an API names in accept.
func TestMtlsAuthValidator_ResolveMtlsAuthForResponse_AcceptNamesRelayAuthority(t *testing.T) {
	lbPEM, _, _ := generateXDSTestCA(t)
	partnerPEM, _, _ := generateXDSTestCA(t)

	pooled := func(name, role string, pem []byte) *models.StoredCertificate {
		return &models.StoredCertificate{Name: name, Usage: models.CertificateUsageDownstream, Role: role, Certificate: pem}
	}
	acceptOf := func(ca string) *api.RestAPI {
		return restAPIWithAPILevelPolicies(mtlsPolicy(map[string]interface{}{"accept": []interface{}{
			map[string]interface{}{"ca": ca},
		}}))
	}
	const field = "spec.policies[0].params.accept[0].ca"

	tests := []struct {
		name        string
		pool        []*models.StoredCertificate
		acceptCA    string
		wantWarning bool
	}{
		{
			name: "same authority stored with different surrounding bytes",
			pool: []*models.StoredCertificate{
				pooled("edge-lb-relay", models.CertificateRoleRelay, lbPEM),
				pooled("edge-lb-client", models.CertificateRoleClient, append([]byte("\n"), lbPEM...)),
			},
			acceptCA:    "edge-lb-client",
			wantWarning: true,
		},
		{
			name: "a different authority than every relay",
			pool: []*models.StoredCertificate{
				pooled("edge-lb-relay", models.CertificateRoleRelay, lbPEM),
				pooled("partner-a", models.CertificateRoleClient, partnerPEM),
			},
			acceptCA:    "partner-a",
			wantWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewMtlsAuthValidator(newFakeMtlsCertStore(tt.pool...), true, false, mtlsAuthTestSchema())
			_, warnings := v.ResolveMtlsAuthForResponse(*acceptOf(tt.acceptCA))
			if got := hasWarning(warnings, WarningCodeMTLSAcceptNamesRelayAuthority, field); got != tt.wantWarning {
				t.Fatalf("MTLS_ACCEPT_NAMES_RELAY_AUTHORITY at %s present = %v, want %v; warnings %+v", field, got, tt.wantWarning, warnings)
			}
		})
	}
}

// ============ ValidateMTLSStartupInvariant ============

func storedConfigWithRestAPI(handle string, restAPI *api.RestAPI) *models.StoredConfig {
	return &models.StoredConfig{
		Handle:        handle,
		Kind:          models.KindRestApi,
		Configuration: *restAPI,
	}
}

func TestValidateMTLSStartupInvariant_HTTPSDisabled_APILevelMTLS_Errors(t *testing.T) {
	apiConfig := restAPIWithAPILevelPolicies(mtlsPolicy(nil))
	configs := []*models.StoredConfig{storedConfigWithRestAPI("mtls-api", apiConfig)}

	err := ValidateMTLSStartupInvariant(configs, false, false)

	if err == nil {
		t.Fatal("expected an error when https is disabled and a stored config attaches mtls-auth at API level")
	}
}

func TestValidateMTLSStartupInvariant_HTTPSDisabled_OperationLevelMTLS_Errors(t *testing.T) {
	apiConfig := restAPIWithOperationLevelPolicy(mtlsPolicy(nil))
	configs := []*models.StoredConfig{storedConfigWithRestAPI("mtls-api", apiConfig)}

	err := ValidateMTLSStartupInvariant(configs, false, false)

	if err == nil {
		t.Fatal("expected an error when https is disabled and a stored config attaches mtls-auth at operation level")
	}
}

func TestValidateMTLSStartupInvariant_HTTPSEnabled_NoError(t *testing.T) {
	apiConfig := restAPIWithAPILevelPolicies(mtlsPolicy(nil))
	configs := []*models.StoredConfig{storedConfigWithRestAPI("mtls-api", apiConfig)}

	if err := ValidateMTLSStartupInvariant(configs, true, false); err != nil {
		t.Fatalf("expected no error when https is enabled, got: %v", err)
	}
}

func TestValidateMTLSStartupInvariant_NoMTLSConfigs_NoError(t *testing.T) {
	plainConfig := createValidRestAPIConfig()
	configs := []*models.StoredConfig{storedConfigWithRestAPI("plain-api", plainConfig)}

	if err := ValidateMTLSStartupInvariant(configs, false, false); err != nil {
		t.Fatalf("expected no error when no stored config attaches mtls-auth, got: %v", err)
	}
}

// ============ headerTrustAny: the header-relay trust_any relaxation ============

// With trust_any true, a disabled HTTPS listener does not refuse mtls-auth,
// since a relayed header can arrive over plaintext.
func TestMtlsAuthValidator_ValidateRestAPI_HeaderTrustAny_RelaxesHTTPSRequirement(t *testing.T) {
	store := newFakeMtlsCertStore(clientCA("listener-partner-a"))
	v := NewMtlsAuthValidator(store, false, true, mtlsAuthTestSchema())
	apiConfig := restAPIWithAPILevelPolicies(mtlsPolicy(nil))

	errs := v.ValidateRestAPI(apiConfig)

	want := "mtls-auth requires the HTTPS listener, which is disabled on this gateway"
	if hasError(errs, "spec.policies[0]", want) {
		t.Fatalf("expected the HTTPS-listener refusal to be relaxed when trust_any is true, got %+v", errs)
	}
}

// An API without mtls-auth never carries HEADER_CERT_BYPASS_ACTIVE, even
// with trust_any true.
func TestMtlsAuthValidator_ResolveMtlsAuthForResponse_HeaderTrustAny_NoMTLSAuth_NoBypassWarning(t *testing.T) {
	store := newFakeMtlsCertStore(clientCA("listener-partner-a"))
	v := NewMtlsAuthValidator(store, true, true, mtlsAuthTestSchema())
	plainConfig := createValidRestAPIConfig()

	_, warnings := v.ResolveMtlsAuthForResponse(*plainConfig)

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings for an API that never attaches mtls-auth, got %+v", warnings)
	}
}

// ============ ValidateMTLSStartupInvariant: the headerTrustAny relaxation ============

func TestValidateMTLSStartupInvariant_HTTPSDisabled_HeaderTrustAny_NoError(t *testing.T) {
	apiConfig := restAPIWithAPILevelPolicies(mtlsPolicy(nil))
	configs := []*models.StoredConfig{storedConfigWithRestAPI("mtls-api", apiConfig)}

	if err := ValidateMTLSStartupInvariant(configs, false, true); err != nil {
		t.Fatalf("expected no error when https is disabled but trust_any is true, got: %v", err)
	}
}

func TestMtlsAuthValidator_ValidateRestAPI_EmptyPool_ParamsStillValidated(t *testing.T) {
	v := NewMtlsAuthValidator(newFakeMtlsCertStore(), true, false, mtlsAuthTestSchema())
	cfg := restAPIWithAPILevelPolicies(mtlsPolicy(map[string]interface{}{"acept": []interface{}{}}))

	errs := v.ValidateRestAPI(cfg)

	if !hasError(errs, "spec.policies[0]",
		"mtls-auth requires at least one client authority; add one with POST /certificates and usage: downstream") {
		t.Fatalf("expected the empty-pool error, got %+v", errs)
	}
	if !hasError(errs, "spec.policies[0].params.acept", "unknown parameter acept") {
		t.Fatalf("expected the misspelled parameter to be reported alongside the empty pool, got %+v", errs)
	}
}

func TestMtlsAuthValidator_ValidateRestAPI_AcceptNamesNonDownstreamUsage(t *testing.T) {
	store := newFakeMtlsCertStore(
		clientCA("listener-partner-a"),
		&models.StoredCertificate{Name: "out-identity-a", Usage: models.CertificateUsageIdentity},
		&models.StoredCertificate{Name: "listener-odd", Usage: "archive"},
	)
	v := NewMtlsAuthValidator(store, true, false, mtlsAuthTestSchema())

	tests := []struct {
		ca      string
		message string
	}{
		{ca: "out-identity-a", message: "out-identity-a is a gateway identity (usage: identity); accept takes usage: downstream authorities"},
		{ca: "listener-odd", message: "listener-odd has an unrecognized usage; accept takes usage: downstream authorities"},
	}
	for _, tt := range tests {
		t.Run(tt.ca, func(t *testing.T) {
			cfg := restAPIWithAPILevelPolicies(mtlsPolicy(map[string]interface{}{
				"accept": []interface{}{map[string]interface{}{"ca": tt.ca}},
			}))
			errs := v.ValidateRestAPI(cfg)
			if !hasError(errs, "spec.policies[0].params.accept[0].ca", tt.message) {
				t.Fatalf("expected %q, got %+v", tt.message, errs)
			}
		})
	}
}

const mtlsAuthDefinitionPath = "testdata/mtls-auth-policy-definition.yaml"

var (
	mtlsAuthDefinitionOnce sync.Once
	mtlsAuthDefinition     models.PolicyDefinition
)

// mtlsAuthTestDefinition loads the shipped mtls-auth policy definition the
// way the controller's policy loader does: YAML to JSON to the model.
func mtlsAuthTestDefinition() models.PolicyDefinition {
	mtlsAuthDefinitionOnce.Do(func() {
		data, err := os.ReadFile(mtlsAuthDefinitionPath)
		if err != nil {
			panic("reading the mtls-auth policy definition: " + err.Error())
		}
		var generic map[string]interface{}
		if err := yaml.Unmarshal(data, &generic); err != nil {
			panic("parsing the mtls-auth policy definition: " + err.Error())
		}
		asJSON, err := json.Marshal(generic)
		if err != nil {
			panic("encoding the mtls-auth policy definition: " + err.Error())
		}
		if err := json.Unmarshal(asJSON, &mtlsAuthDefinition); err != nil {
			panic("decoding the mtls-auth policy definition: " + err.Error())
		}
	})
	return mtlsAuthDefinition
}

func mtlsAuthTestSchema() map[string]interface{} {
	return *mtlsAuthTestDefinition().Parameters
}

// mtlsAuthPolicyValidator is the deploy-time PolicyValidator the controller
// builds: the shipped mtls-auth definition plus the mtls-auth validator.
func mtlsAuthPolicyValidator(store MtlsAuthCertificateStore) *PolicyValidator {
	def := mtlsAuthTestDefinition()
	defs := map[string]models.PolicyDefinition{def.Name + "|" + def.Version: def}
	return NewPolicyValidator(defs, NewMtlsAuthValidator(store, true, false, MtlsAuthParameterSchema(defs)))
}

func mtlsAuthDeploy(params map[string]interface{}) *api.RestAPI {
	return restAPIWithAPILevelPolicies(mtlsPolicy(params))
}

func TestPolicyValidator_MtlsAuth_StringStatusCodeIsCoerced(t *testing.T) {
	pv := mtlsAuthPolicyValidator(newFakeMtlsCertStore(clientCA("listener-partner-a")))
	params := map[string]interface{}{
		"accept":              []interface{}{map[string]interface{}{"ca": "listener-partner-a"}},
		"onFailureStatusCode": "403",
	}

	if errs := pv.ValidateRestAPIPolicies(mtlsAuthDeploy(params)); len(errs) != 0 {
		t.Fatalf("expected a string status code to be coerced and deploy, got %+v", errs)
	}
	if got := params["onFailureStatusCode"]; fmt.Sprintf("%T:%v", got, got) != "float64:403" {
		t.Errorf("onFailureStatusCode = %#v, want the number 403", got)
	}
}

func TestPolicyValidator_MtlsAuth_SchemaErrorsTheValidatorDoesNotOwn(t *testing.T) {
	for name, tc := range map[string]struct {
		params  map[string]interface{}
		field   string
		message string
	}{
		"non-numeric status code": {
			params:  map[string]interface{}{"onFailureStatusCode": "abc"},
			field:   "spec.policies[0].params.onFailureStatusCode",
			message: "Invalid type. Expected: integer, given: string",
		},
		"status code below 400": {
			params:  map[string]interface{}{"onFailureStatusCode": 302},
			field:   "spec.policies[0].params.onFailureStatusCode",
			message: "Must be greater than or equal to 400",
		},
		"status code above 599": {
			params:  map[string]interface{}{"onFailureStatusCode": 600},
			field:   "spec.policies[0].params.onFailureStatusCode",
			message: "Must be less than or equal to 599",
		},
		"unsupported error message format": {
			params:  map[string]interface{}{"errorMessageFormat": "xml"},
			field:   "spec.policies[0].params.errorMessageFormat",
			message: "errorMessageFormat must be one of the following: \"json\", \"plain\", \"minimal\"",
		},
	} {
		t.Run(name, func(t *testing.T) {
			pv := mtlsAuthPolicyValidator(newFakeMtlsCertStore(clientCA("listener-partner-a")))
			errs := pv.ValidateRestAPIPolicies(mtlsAuthDeploy(tc.params))
			if len(errs) != 1 || errs[0].Field != tc.field || errs[0].Message != tc.message {
				t.Fatalf("expected exactly {field: %q, message: %q}, got %+v", tc.field, tc.message, errs)
			}
		})
	}
}

// TestPolicyValidator_MtlsAuth_EachProblemReportedOnce runs every
// mtls-auth-specific message through the full deploy-time validator and
// requires it once, with no generic schema error alongside it.
func TestPolicyValidator_MtlsAuth_EachProblemReportedOnce(t *testing.T) {
	store := func() MtlsAuthCertificateStore {
		return newFakeMtlsCertStore(clientCA("listener-partner-a"), relayCA("listener-edge-lb"), upstreamCA("listener-backend-trust"))
	}
	entry := func(fields map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{"accept": []interface{}{fields}}
	}
	for name, tc := range map[string]struct {
		params  map[string]interface{}
		field   string
		message string
	}{
		"empty accept": {
			params: map[string]interface{}{"accept": []interface{}{}},
			field:  "spec.policies[0].params.accept", message: "omit accept to inherit every pooled authority, or list at least one entry",
		},
		"accept as an object": {
			params: map[string]interface{}{"accept": map[string]interface{}{"ca": "listener-partner-a"}},
			field:  "spec.policies[0].params.accept", message: "accept must be a list of entries; omit it to inherit every pooled authority",
		},
		"accept entry as a string": {
			params: map[string]interface{}{"accept": []interface{}{"listener-partner-a"}},
			field:  "spec.policies[0].params.accept[0]", message: "each accept entry must be an object naming ca",
		},
		"missing ca": {
			params: entry(map[string]interface{}{"match": map[string]interface{}{"uriSANs": []interface{}{"urn:x"}}}),
			field:  "spec.policies[0].params.accept[0].ca", message: "ca is required and must name an authority in this gateway's client-CA pool",
		},
		"ca as a number": {
			params: entry(map[string]interface{}{"ca": 7}),
			field:  "spec.policies[0].params.accept[0].ca", message: "ca is required and must name an authority in this gateway's client-CA pool",
		},
		"unknown ca": {
			params: entry(map[string]interface{}{"ca": "listener-partner-b"}),
			field:  "spec.policies[0].params.accept[0].ca", message: "no client-CA authority named listener-partner-b exists on this gateway",
		},
		"relay ca": {
			params: entry(map[string]interface{}{"ca": "listener-edge-lb"}),
			field:  "spec.policies[0].params.accept[0].ca", message: "listener-edge-lb is a relay (front proxy) entry and cannot be accepted as a client",
		},
		"upstream ca": {
			params: entry(map[string]interface{}{"ca": "listener-backend-trust"}),
			field:  "spec.policies[0].params.accept[0].ca", message: "listener-backend-trust is a backend trust certificate (usage: upstream); accept takes usage: downstream authorities",
		},
		"empty uriSANs": {
			params: entry(map[string]interface{}{"ca": "listener-partner-a", "match": map[string]interface{}{"uriSANs": []interface{}{}}}),
			field:  "spec.policies[0].params.accept[0].match.uriSANs", message: "list at least one non-empty SAN, or remove match to accept any certificate from this authority",
		},
		"blank dnsSAN": {
			params: entry(map[string]interface{}{"ca": "listener-partner-a", "match": map[string]interface{}{"dnsSANs": []interface{}{"a", ""}}}),
			field:  "spec.policies[0].params.accept[0].match.dnsSANs[1]", message: "list at least one non-empty SAN, or remove match to accept any certificate from this authority",
		},
		"non-string dnsSAN": {
			params: entry(map[string]interface{}{"ca": "listener-partner-a", "match": map[string]interface{}{"dnsSANs": []interface{}{"a", 7}}}),
			field:  "spec.policies[0].params.accept[0].match.dnsSANs[1]", message: "list at least one non-empty SAN, or remove match to accept any certificate from this authority",
		},
		"uriSANs as a scalar": {
			params: entry(map[string]interface{}{"ca": "listener-partner-a", "match": map[string]interface{}{"uriSANs": "urn:x"}}),
			field:  "spec.policies[0].params.accept[0].match.uriSANs", message: "uriSANs must be a list",
		},
		"empty match": {
			params: entry(map[string]interface{}{"ca": "listener-partner-a", "match": map[string]interface{}{}}),
			field:  "spec.policies[0].params.accept[0].match", message: "match must list uriSANs or dnsSANs; remove it to accept any certificate from this authority",
		},
		"match as a scalar": {
			params: entry(map[string]interface{}{"ca": "listener-partner-a", "match": "urn:x"}),
			field:  "spec.policies[0].params.accept[0].match", message: "match must be an object listing uriSANs or dnsSANs",
		},
		"unknown match key": {
			params: entry(map[string]interface{}{"ca": "listener-partner-a", "match": map[string]interface{}{"uriSANs": []interface{}{"urn:x"}, "ipSANs": []interface{}{"10.0.0.1"}}}),
			field:  "spec.policies[0].params.accept[0].match.ipSANs", message: "unknown parameter ipSANs",
		},
		"empty thumbprints": {
			params: entry(map[string]interface{}{"ca": "listener-partner-a", "thumbprints": []interface{}{}}),
			field:  "spec.policies[0].params.accept[0].thumbprints", message: "list at least one thumbprint, or remove thumbprints to accept any certificate from this authority",
		},
		"malformed thumbprint": {
			params: entry(map[string]interface{}{"ca": "listener-partner-a", "thumbprints": []interface{}{"zz"}}),
			field:  "spec.policies[0].params.accept[0].thumbprints[0]", message: "a thumbprint is the SHA-256 of the certificate as 64 hex characters (colons and a sha256: prefix are accepted)",
		},
		"non-string thumbprint": {
			params: entry(map[string]interface{}{"ca": "listener-partner-a", "thumbprints": []interface{}{7}}),
			field:  "spec.policies[0].params.accept[0].thumbprints[0]", message: "a thumbprint is the SHA-256 of the certificate as 64 hex characters (colons and a sha256: prefix are accepted)",
		},
		"thumbprints as a scalar": {
			params: entry(map[string]interface{}{"ca": "listener-partner-a", "thumbprints": "9f86d081"}),
			field:  "spec.policies[0].params.accept[0].thumbprints", message: "thumbprints must be a list",
		},
		"singular thumbprint": {
			params: entry(map[string]interface{}{"ca": "listener-partner-a", "thumbprint": "9f86d081"}),
			field:  "spec.policies[0].params.accept[0].thumbprint", message: "unknown parameter thumbprint; the field is thumbprints",
		},
		"unknown entry key": {
			params: entry(map[string]interface{}{"ca": "listener-partner-a", "role": "client"}),
			field:  "spec.policies[0].params.accept[0].role", message: "unknown parameter role",
		},
		"unknown top-level key": {
			params: map[string]interface{}{"mode": "strict"},
			field:  "spec.policies[0].params.mode", message: "unknown parameter mode",
		},
		"headerName written by an author": {
			params: map[string]interface{}{"headerName": "X-Client-Cert"},
			field:  "spec.policies[0].params.headerName", message: "unknown parameter headerName",
		},
		"trustAny written by an author": {
			params: map[string]interface{}{"trustAny": true},
			field:  "spec.policies[0].params.trustAny", message: "unknown parameter trustAny",
		},
		"forwardCertificate as a string": {
			params: map[string]interface{}{"forwardCertificate": "no"},
			field:  "spec.policies[0].params.forwardCertificate", message: "forwardCertificate must be true or false",
		},
	} {
		t.Run(name, func(t *testing.T) {
			errs := mtlsAuthPolicyValidator(store()).ValidateRestAPIPolicies(mtlsAuthDeploy(tc.params))
			if len(errs) != 1 {
				t.Fatalf("expected exactly one error, got %d: %+v", len(errs), errs)
			}
			if errs[0].Field != tc.field || errs[0].Message != tc.message {
				t.Fatalf("expected {field: %q, message: %q}, got %+v", tc.field, tc.message, errs[0])
			}
			if strings.Contains(errs[0].Message, "Additional property") || strings.Contains(errs[0].Message, "Invalid type") {
				t.Fatalf("a generic schema error surfaced: %+v", errs[0])
			}
		})
	}
}

func TestPolicyValidator_MtlsAuth_SchemaErrorOnAFieldTheValidatorDoesNotCheckIsKept(t *testing.T) {
	pv := mtlsAuthPolicyValidator(newFakeMtlsCertStore(clientCA("listener-partner-a")))
	params := map[string]interface{}{
		"accept":       []interface{}{map[string]interface{}{"ca": "listener-partner-a", "thumbprints": []interface{}{"zz"}}},
		"errorMessage": 7,
	}

	errs := pv.ValidateRestAPIPolicies(mtlsAuthDeploy(params))

	if len(errs) != 2 {
		t.Fatalf("expected the thumbprint error and the errorMessage type error, got %+v", errs)
	}
	if !hasError(errs, "spec.policies[0].params.errorMessage", "Invalid type. Expected: string, given: integer") {
		t.Errorf("expected the schema's errorMessage type error, got %+v", errs)
	}
}
