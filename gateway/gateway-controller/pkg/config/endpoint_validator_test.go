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
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// testEndpoint builds a valid Endpoint with the given upstream URLs.
func testEndpoint(name string, urls ...string) *api.EndpointConfiguration {
	endpoint := &api.EndpointConfiguration{
		ApiVersion: api.EndpointConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.EndpointConfigurationKindEndpoint,
		Metadata:   api.Metadata{Name: name},
	}
	for _, u := range urls {
		endpoint.Spec.Upstreams = append(endpoint.Spec.Upstreams, struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{Url: u})
	}
	return endpoint
}

func endpointErrorFields(errs []ValidationError) []string {
	fields := make([]string, 0, len(errs))
	for _, e := range errs {
		fields = append(fields, e.Field)
	}
	return fields
}

type endpointCertStore map[string]*models.StoredCertificate

func (s endpointCertStore) GetCertificateByName(name string) (*models.StoredCertificate, error) {
	if cert, ok := s[name]; ok {
		return cert, nil
	}
	return nil, errors.New("not found")
}

func TestEndpointValidator_AcceptsAValidEndpoint(t *testing.T) {
	endpoint := testEndpoint("tenant-a", "http://orders-tenant-a:9080", "http://orders-tenant-a-2:9080")
	basePath, connect, weight := "/v1", "2s", 80
	labels := map[string]string{"team": "orders"}
	display := "Orders backend for tenant A"
	endpoint.Metadata.Labels = &labels
	endpoint.Spec.DisplayName = &display
	endpoint.Spec.BasePath = &basePath
	endpoint.Spec.Timeout = &api.UpstreamTimeout{Connect: &connect}
	endpoint.Spec.Upstreams[0].Weight = &weight

	assert.Empty(t, NewEndpointValidator(nil).Validate(endpoint))
}

func TestEndpointValidator_RejectsANilEndpoint(t *testing.T) {
	assert.Len(t, NewEndpointValidator(nil).Validate(nil), 1)
}

func TestEndpointValidator_Envelope(t *testing.T) {
	endpoint := testEndpoint("tenant-a", "http://backend:9080")
	endpoint.ApiVersion = "gateway.api-platform.wso2.com/v2"
	endpoint.Kind = "RestApi"

	fields := endpointErrorFields(NewEndpointValidator(nil).Validate(endpoint))
	assert.ElementsMatch(t, []string{"apiVersion", "kind"}, fields)
}

// The name becomes part of a cluster name and is what a request header is
// checked against, so the strict handle rule is enforced here, not a looser one.
func TestEndpointValidator_NameRule(t *testing.T) {
	for _, name := range []string{
		"", "ab", strings.Repeat("a", 41), "Tenant-A", "tenant_a", "tenant.a",
		"-tenant", "tenant-", "tenant--a", "tenant a", "upstream_main",
	} {
		errs := NewEndpointValidator(nil).Validate(testEndpoint(name, "http://backend:9080"))
		assert.Containsf(t, endpointErrorFields(errs), "metadata.name", "name %q must be rejected", name)
	}
	for _, name := range []string{"abc", "tenant-a", "a1-b2", strings.Repeat("a", 40)} {
		errs := NewEndpointValidator(nil).Validate(testEndpoint(name, "http://backend:9080"))
		assert.Emptyf(t, errs, "name %q must be accepted", name)
	}
}

func TestEndpointValidator_Labels(t *testing.T) {
	endpoint := testEndpoint("tenant-a", "http://backend:9080")
	labels := map[string]string{"team name": "orders"}
	endpoint.Metadata.Labels = &labels

	assert.Equal(t, []string{"metadata.labels"}, endpointErrorFields(NewEndpointValidator(nil).Validate(endpoint)))
}

func TestEndpointValidator_BackendRulesAreThoseOfUpstreamDefinitions(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(e *api.EndpointConfiguration)
		field  string
	}{
		{"no upstreams", func(e *api.EndpointConfiguration) { e.Spec.Upstreams = nil }, "spec.upstreams"},
		{"empty url", func(e *api.EndpointConfiguration) { e.Spec.Upstreams[0].Url = "" }, "spec.upstreams[0].url"},
		{"non-http scheme", func(e *api.EndpointConfiguration) { e.Spec.Upstreams[0].Url = "ftp://backend:21" }, "spec.upstreams[0].url"},
		{"file scheme", func(e *api.EndpointConfiguration) { e.Spec.Upstreams[0].Url = "file:///etc/passwd" }, "spec.upstreams[0].url"},
		{"url with a path", func(e *api.EndpointConfiguration) { e.Spec.Upstreams[0].Url = "http://backend:9080/v1" }, "spec.upstreams[0].url"},
		{"url with a query", func(e *api.EndpointConfiguration) { e.Spec.Upstreams[0].Url = "http://backend:9080?x=1" }, "spec.upstreams[0].url"},
		{"url without a host", func(e *api.EndpointConfiguration) { e.Spec.Upstreams[0].Url = "http://" }, "spec.upstreams[0].url"},
		{"weight out of range", func(e *api.EndpointConfiguration) { w := 101; e.Spec.Upstreams[0].Weight = &w }, "spec.upstreams[0].weight"},
		{"negative weight", func(e *api.EndpointConfiguration) { w := -1; e.Spec.Upstreams[0].Weight = &w }, "spec.upstreams[0].weight"},
		{"basePath without leading slash", func(e *api.EndpointConfiguration) { p := "v1"; e.Spec.BasePath = &p }, "spec.basePath"},
		{"basePath with trailing slash", func(e *api.EndpointConfiguration) { p := "/v1/"; e.Spec.BasePath = &p }, "spec.basePath"},
		{"bad connect timeout", func(e *api.EndpointConfiguration) { c := "soon"; e.Spec.Timeout = &api.UpstreamTimeout{Connect: &c} }, "spec.timeout.connect"},
		{"zero connect timeout", func(e *api.EndpointConfiguration) { c := "0s"; e.Spec.Timeout = &api.UpstreamTimeout{Connect: &c} }, "spec.timeout.connect"},
		{"empty displayName", func(e *api.EndpointConfiguration) { d := ""; e.Spec.DisplayName = &d }, "spec.displayName"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := testEndpoint("tenant-a", "http://backend:9080")
			tc.mutate(endpoint)
			assert.Contains(t, endpointErrorFields(NewEndpointValidator(nil).Validate(endpoint)), tc.field)
		})
	}
}

// One cluster has one transport socket, so a mix would dial the plaintext
// targets over TLS.
func TestEndpointValidator_RejectsMixedSchemes(t *testing.T) {
	endpoint := testEndpoint("tenant-a", "https://backend-a:9443", "http://backend-b:9080")
	assert.Contains(t, endpointErrorFields(NewEndpointValidator(nil).Validate(endpoint)), "spec.upstreams")

	assert.Empty(t, NewEndpointValidator(nil).Validate(testEndpoint("tenant-a", "https://a:9443", "https://b:9443")))
}

func TestEndpointValidator_TLSBlock(t *testing.T) {
	store := endpointCertStore{
		"partner-identity": {Name: "partner-identity", Usage: models.CertificateUsageIdentity},
		"partner-ca":       {Name: "partner-ca", Usage: models.CertificateUsageUpstream},
		"client-ca":        {Name: "client-ca", Usage: models.CertificateUsageDownstream},
	}
	validator := NewEndpointValidator(NewUpstreamTLSValidator(store, false))

	valid := testEndpoint("tenant-a", "https://backend:9443")
	valid.Spec.Tls = &map[string]interface{}{
		"identity":       "partner-identity",
		"trustedCAs":     []interface{}{"partner-ca"},
		"verifyHostName": true,
	}
	require.Empty(t, validator.Validate(valid))

	tests := []struct {
		name  string
		url   string
		tls   map[string]interface{}
		field string
	}{
		{"unknown identity", "https://backend:9443", map[string]interface{}{"identity": "missing"}, "spec.tls.identity"},
		{"identity of the wrong usage", "https://backend:9443", map[string]interface{}{"identity": "partner-ca"}, "spec.tls.identity"},
		{"unknown trusted CA", "https://backend:9443", map[string]interface{}{"trustedCAs": []interface{}{"missing"}}, "spec.tls.trustedCAs[0]"},
		{"client authority as trusted CA", "https://backend:9443", map[string]interface{}{"trustedCAs": []interface{}{"client-ca"}}, "spec.tls.trustedCAs[0]"},
		{"empty trustedCAs", "https://backend:9443", map[string]interface{}{"trustedCAs": []interface{}{}}, "spec.tls.trustedCAs"},
		{"unknown key", "https://backend:9443", map[string]interface{}{"insecure": true}, "spec.tls.insecure"},
		{"tls on an http target", "http://backend:9080", map[string]interface{}{"identity": "partner-identity"}, "spec.upstreams[0].url"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := testEndpoint("tenant-a", tc.url)
			endpoint.Spec.Tls = &tc.tls
			assert.Contains(t, endpointErrorFields(validator.Validate(endpoint)), tc.field)
		})
	}
}

// With router-wide verification off, a per-endpoint trust setting could never
// be honoured, so it is refused rather than silently ignored.
func TestEndpointValidator_TLSTrustRefusedWhenVerificationIsDisabled(t *testing.T) {
	store := endpointCertStore{"partner-ca": {Name: "partner-ca", Usage: models.CertificateUsageUpstream}}
	validator := NewEndpointValidator(NewUpstreamTLSValidator(store, true))

	endpoint := testEndpoint("tenant-a", "https://backend:9443")
	endpoint.Spec.Tls = &map[string]interface{}{"trustedCAs": []interface{}{"partner-ca"}}
	assert.Contains(t, endpointErrorFields(validator.Validate(endpoint)), "spec.tls")
}
