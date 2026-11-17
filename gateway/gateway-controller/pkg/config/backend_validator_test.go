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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/clientca"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

type backendEndpoint = struct {
	Url    string `json:"url" yaml:"url"`
	Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
}

func validBackend() *api.BackendRequest {
	basePath := "/v1"
	labels := map[string]string{"team": "orders"}
	return &api.BackendRequest{
		ApiVersion: api.BackendRequestApiVersionGatewayChoreolabsComv1,
		Kind:       api.BackendRequestKindBackend,
		Metadata:   api.Metadata{Name: "tenant-a", Labels: &labels},
		Spec: api.BackendConfiguration{
			BasePath:  &basePath,
			Endpoints: []backendEndpoint{{Url: "http://orders-tenant-a:9080"}},
		},
	}
}

func fields(errs []ValidationError) []string {
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, e.Field)
	}
	return out
}

func TestBackendValidator_AcceptsAValidBackend(t *testing.T) {
	assert.Empty(t, NewBackendValidator(nil).Validate(validBackend()))
}

// The display name and description limits count characters, as the schema's maxLength does, so
// multibyte text up to the limit is accepted.
func TestBackendValidator_LengthLimitsCountCharacters(t *testing.T) {
	b := validBackend()
	displayName := strings.Repeat("é", backendDisplayNameMaxLength)
	description := strings.Repeat("é", backendDescriptionMaxLength)
	b.Spec.DisplayName, b.Spec.Description = &displayName, &description
	assert.Empty(t, NewBackendValidator(nil).Validate(b))

	tooLong := displayName + "é"
	b.Spec.DisplayName = &tooLong
	assert.Equal(t, []string{"spec.displayName"}, fields(NewBackendValidator(nil).Validate(b)))
}

// An explicit port must be a valid socket port; an omitted port stays valid.
func TestBackendValidator_EndpointPortRange(t *testing.T) {
	for url, ok := range map[string]bool{
		"http://orders:65535": true,
		"http://orders:1":     true,
		"http://orders":       true,
		"http://orders:65536": false,
		"http://orders:0":     false,
	} {
		b := validBackend()
		b.Spec.Endpoints = []backendEndpoint{{Url: url}}
		errs := NewBackendValidator(nil).Validate(b)
		if ok {
			assert.Empty(t, errs, url)
		} else {
			assert.Equal(t, []string{"spec.endpoints[0].url"}, fields(errs), url)
		}
	}
}

func TestBackendValidator_NameRule(t *testing.T) {
	for _, name := range []string{"ab", strings.Repeat("a", 41), "Tenant-A", "tenant_a", "tenant--a", "-a1", "a1-", "backend.x"} {
		e := validBackend()
		e.Metadata.Name = name
		assert.Contains(t, fields(NewBackendValidator(nil).Validate(e)), "metadata.name", "name %q", name)
	}
	for _, name := range []string{"abc", "tenant-a", "a1-b2", strings.Repeat("a", 40)} {
		e := validBackend()
		e.Metadata.Name = name
		assert.Empty(t, NewBackendValidator(nil).Validate(e), "name %q", name)
	}
}

func TestBackendValidator_ReportsSpecErrorsUnderSpec(t *testing.T) {
	e := validBackend()
	bad := "/v1/"
	e.Spec.BasePath = &bad
	e.Spec.Endpoints = []backendEndpoint{{Url: "ftp://x:21"}}
	got := fields(NewBackendValidator(nil).Validate(e))
	assert.Contains(t, got, "spec.basePath")
	assert.Contains(t, got, "spec.endpoints[0].url")
	for _, f := range got {
		assert.NotContains(t, f, "upstreamDefinitions", "errors must point at the Backend's own fields")
	}
}

func TestBackendValidator_RequiresAnEndpoint(t *testing.T) {
	e := validBackend()
	e.Spec.Endpoints = nil
	assert.Contains(t, fields(NewBackendValidator(nil).Validate(e)), "spec.endpoints")
}

func TestBackendValidator_RejectsMixedSchemes(t *testing.T) {
	e := validBackend()
	e.Spec.Endpoints = []backendEndpoint{{Url: "https://a:443"}, {Url: "http://b:80"}}
	assert.Contains(t, fields(NewBackendValidator(nil).Validate(e)), "spec.endpoints")
}

func TestBackendValidator_EnvelopeAndLabels(t *testing.T) {
	e := validBackend()
	e.Kind = "RestApi"
	e.ApiVersion = "v2"
	labels := map[string]string{"bad key": "x"}
	e.Metadata.Labels = &labels
	got := fields(NewBackendValidator(nil).Validate(e))
	assert.Contains(t, got, "kind")
	assert.Contains(t, got, "apiVersion")
	assert.Contains(t, got, "metadata.labels")
}

func TestBackendValidator_MessagesNameTheBackendsFields(t *testing.T) {
	e := validBackend()
	e.Spec.Endpoints = []backendEndpoint{{Url: "http://orders-tenant-a:9080/v1"}}
	errs := NewBackendValidator(nil).Validate(e)
	require.Len(t, errs, 1)
	assert.Equal(t, "spec.endpoints[0].url", errs[0].Field)
	assert.Contains(t, errs[0].Message, "spec.basePath")
	assert.NotContains(t, errs[0].Message, "upstreamDefinitions")

	tls := map[string]interface{}{}
	e = validBackend()
	e.Spec.Tls = &tls
	errs = NewBackendValidator(NewUpstreamTLSValidator(newFakeMtlsCertStore(), false)).Validate(e)
	require.Len(t, errs, 1)
	assert.Equal(t, "spec.endpoints[0].url", errs[0].Field)
	assert.Contains(t, errs[0].Message, "every endpoint of a Backend with tls must be https://")
}

// tlsBackend is a valid Backend with the given tls block (none when nil) and one endpoint per url.
func tlsBackend(tls map[string]interface{}, urls ...string) *api.BackendRequest {
	e := validBackend()
	e.Spec.Endpoints = nil
	for _, u := range urls {
		e.Spec.Endpoints = append(e.Spec.Endpoints, backendEndpoint{Url: u})
	}
	if tls != nil {
		e.Spec.Tls = &tls
	}
	return e
}

func warningsAt(warnings []clientca.Warning) []string {
	out := make([]string, 0, len(warnings))
	for _, w := range warnings {
		out = append(out, w.Code+"@"+w.Field)
	}
	return out
}

func TestBackendValidator_Warnings(t *testing.T) {
	store := newFakeMtlsCertStore(
		gatewayIdentityCert("gw-client"),
		expiredGatewayIdentityCert("gw-retired"),
		&models.StoredCertificate{Name: "payments-root-ca", Usage: models.CertificateUsageUpstream},
	)
	hasBundle := func() bool { return true }
	emptyBundle := func() bool { return false }
	privateCA := []interface{}{"payments-root-ca"}

	cases := []struct {
		name            string
		backend         *api.BackendRequest
		verificationOff bool
		bundle          func() bool
		want            []string
	}{
		{name: "https with the gateway bundle", backend: tlsBackend(nil, "https://api.vendor.example.com"), bundle: hasBundle},
		{name: "plaintext", backend: tlsBackend(nil, "http://orders:9080"), bundle: emptyBundle},
		{
			name:    "hostname verification off",
			backend: tlsBackend(map[string]interface{}{"trustedCAs": privateCA, "verifyHostName": false}, "https://10.20.0.15:8443"),
			bundle:  hasBundle,
			want:    []string{"TLS_VERIFY_HOSTNAME_DISABLED@spec.tls.verifyHostName"},
		},
		{
			name:    "expired identity",
			backend: tlsBackend(map[string]interface{}{"identity": "gw-retired"}, "https://partner-gw.example.net"),
			bundle:  hasBundle,
			want:    []string{"TLS_IDENTITY_EXPIRED@spec.tls.identity"},
		},
		{
			name:    "tls block without trustedCAs and an empty bundle",
			backend: tlsBackend(map[string]interface{}{"identity": "gw-client"}, "https://a.example.com", "https://b.example.com"),
			bundle:  emptyBundle,
			want:    []string{"TLS_NO_TRUST_SOURCE@spec.tls"},
		},
		{
			name:    "no tls block and an empty bundle",
			backend: tlsBackend(nil, "https://a.example.com"),
			bundle:  emptyBundle,
			want:    []string{"TLS_NO_TRUST_SOURCE@spec.endpoints"},
		},
		{
			name:    "trustedCAs with an empty bundle",
			backend: tlsBackend(map[string]interface{}{"trustedCAs": privateCA}, "https://ledger.payments.internal:8443"),
			bundle:  emptyBundle,
		},
		{
			name:            "verification disabled with an empty bundle",
			backend:         tlsBackend(map[string]interface{}{"identity": "gw-client"}, "https://a.example.com"),
			verificationOff: true,
			bundle:          emptyBundle,
		},
		{name: "no bundle check configured", backend: tlsBackend(nil, "https://a.example.com")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := NewBackendValidator(NewUpstreamTLSValidator(store, c.verificationOff))
			if c.bundle != nil {
				v = v.WithTrustBundleCheck(c.bundle)
			}
			require.Empty(t, v.Validate(c.backend), "the fixture must be a valid Backend")
			assert.Equal(t, c.want, nilIfEmpty(warningsAt(v.Warnings(c.backend))))
		})
	}

	assert.Nil(t, NewBackendValidator(nil).Warnings(tlsBackend(nil, "https://a.example.com")),
		"without a tls validator there is nothing to check against")
}

func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

func dynamicEndpointDefinition(version string, annotated bool) models.PolicyDefinition {
	props := map[string]interface{}{
		"targetUpstream": map[string]interface{}{"type": "string"},
	}
	if annotated {
		props["targetUpstream"] = map[string]interface{}{"type": "string", BackendRoutingAnnotation: BackendRoutingName}
		props["backendExpression"] = map[string]interface{}{"type": "string", BackendRoutingAnnotation: BackendRoutingExpression}
	}
	params := map[string]interface{}{"type": "object", "properties": props}
	return models.PolicyDefinition{Name: "dynamic-endpoint", Version: version, Parameters: &params}
}

func TestBackendRoutingIndex(t *testing.T) {
	idx := BuildBackendRoutingIndex(map[string]models.PolicyDefinition{
		"dynamic-endpoint|v1.1.0": dynamicEndpointDefinition("v1.1.0", true),
		"dynamic-endpoint|v0.9.0": dynamicEndpointDefinition("v0.9.0", false),
		"set-headers|v1.0.0":      {Name: "set-headers", Version: "v1.0.0"},
	})

	assert.True(t, idx.RoutesToBackends("dynamic-endpoint", "v1", map[string]interface{}{"backendExpression": "request.Headers['x-backend'][0]"}))
	assert.True(t, idx.RoutesToBackends("dynamic-endpoint", "v1.1.0", map[string]interface{}{"targetUpstream": "tenant-a"}),
		"targetUpstream may name a Backend")
	assert.False(t, idx.RoutesToBackends("dynamic-endpoint", "v0", map[string]interface{}{"targetUpstream": "alt"}),
		"a version that does not annotate targetUpstream keeps today's route shape")
	assert.False(t, idx.RoutesToBackends("dynamic-endpoint", "v1", map[string]interface{}{"backendExpression": "  "}))
	assert.False(t, idx.RoutesToBackends("dynamic-endpoint", "v0", map[string]interface{}{"backendExpression": "x"}),
		"a version without the annotation never diverts")
	assert.False(t, idx.RoutesToBackends("set-headers", "v1", map[string]interface{}{"backendExpression": "x"}))

	refs := idx.StaticBackendRefs("dynamic-endpoint", "v1", map[string]interface{}{
		"targetUpstream": " tenant-a ", "backendExpression": "request.Headers['x-backend'][0]"})
	require.Len(t, refs, 1)
	assert.Equal(t, StaticBackendRef{Param: "targetUpstream", Backend: "tenant-a"}, refs[0])
	assert.Empty(t, idx.StaticBackendRefs("dynamic-endpoint", "v1", map[string]interface{}{"backendExpression": "x"}))
}
