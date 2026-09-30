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

	"gopkg.in/yaml.v3"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

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
