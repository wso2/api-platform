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

package testbench

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/testbench/services/backend"
	"github.com/wso2/api-platform/tests/framework/testbench/services/oidc"
)

func TestTestbenchDefinition(t *testing.T) {
	definition := Testbench()
	require.Equal(t, "testbench", definition.Name)
	require.True(t, definition.Shared)
	require.NotNil(t, definition.Health)
	for _, endpoint := range []string{"backend", "jwks", "echo", "analytics"} {
		_, ok := definition.Endpoint(endpoint)
		require.True(t, ok, endpoint)
	}
}

// Every service shares the one testbench container, so two services on one port means
// whichever binds second fails to start, and the scenarios using it get the other one.
func TestTestbenchEndpointsHaveDistinctPorts(t *testing.T) {
	seen := map[int]string{}
	for _, endpoint := range Testbench().Endpoints {
		other, taken := seen[endpoint.Port]
		require.False(t, taken, "%s and %s both use port %d", other, endpoint.Name, endpoint.Port)
		seen[endpoint.Port] = endpoint.Name
	}
}

func TestTheIdentityProviderServesHTTPSWithTheSharedCertificate(t *testing.T) {
	definition := Testbench()
	endpoint, ok := definition.Endpoint("oidc")
	require.True(t, ok)
	require.Equal(t, oidc.Port, endpoint.Port)
	require.Equal(t, "https", endpoint.Scheme)

	pair := shared.IdentityProviderTLS()
	require.Equal(t, string(pair.CertPEM), definition.Env[oidc.EnvTLSCert])
	require.Equal(t, string(pair.PrivateKeyPEM), definition.Env[oidc.EnvTLSKey])
	require.Equal(t, "https://testbench:3015/oauth2/token", definition.Env[oidc.EnvIssuer])
	require.Equal(t, OIDCIssuer, definition.Env[oidc.EnvIssuer])

	block, _ := pem.Decode(pair.CertPEM)
	require.NotNil(t, block)
	certificate, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	require.NoError(t, certificate.VerifyHostname(shared.IdentityProviderHost))
	require.Same(t, pair, shared.IdentityProviderTLS(), "the certificate must be generated once per run")

	_, err = oidc.New(pair.CertPEM, pair.PrivateKeyPEM, OIDCIssuer)
	require.NoError(t, err, "the identity provider must accept the certificate it is given")
}

func TestImageForModuleFollowsTheFrameworkRelease(t *testing.T) {
	dependency := func(version string, replace *debug.Module) *debug.BuildInfo {
		return &debug.BuildInfo{Deps: []*debug.Module{
			{Path: "github.com/stretchr/testify", Version: "v1.12.1"},
			{Path: frameworkModule, Version: version, Replace: replace},
		}}
	}

	require.Equal(t, "ghcr.io/wso2/api-platform/testbench:0.3.1", imageForModule(dependency("v0.3.1", nil)))
	require.Equal(t, "ghcr.io/wso2/api-platform/testbench:12.0.4", imageForModule(dependency("v12.0.4", nil)))
}

func TestImageForModuleUsesTheLocalImageWhenNoReleaseIsInUse(t *testing.T) {
	const local = "ghcr.io/wso2/api-platform/testbench:test"
	withVersion := func(version string, replace *debug.Module) *debug.BuildInfo {
		return &debug.BuildInfo{Deps: []*debug.Module{{Path: frameworkModule, Version: version, Replace: replace}}}
	}

	require.Equal(t, local, imageForModule(&debug.BuildInfo{}), "the framework is the main module")
	require.Equal(t, local, imageForModule(withVersion("v0.0.0-20261012010203-abcdef123456", nil)), "pseudo-version")
	require.Equal(t, local, imageForModule(withVersion("v0.3.1-rc.1", nil)), "pre-release")
	require.Equal(t, local, imageForModule(withVersion("v0.3.1", &debug.Module{Path: "../fw"})), "replaced module")
	require.Equal(t, local, imageForModule(withVersion("(devel)", nil)), "development build")
}

// stubTopology serves one testbench instance to the resolver.
type stubTopology struct{ inst *components.Instance }

func (s stubTopology) URL(component, endpoint string) (string, error) {
	if component != "testbench" || s.inst == nil {
		return "", fmt.Errorf("no component %q", component)
	}
	return s.inst.URL(endpoint)
}

func (s stubTopology) Component(name string) (*components.Instance, error) {
	if name != "testbench" || s.inst == nil {
		return nil, fmt.Errorf("no component %q", name)
	}
	return s.inst, nil
}

func newStubTopology(t *testing.T) stubTopology {
	t.Helper()
	inst, err := components.NewInstance(Testbench(), 0, 1, "localhost", map[int]int{backend.Port: 41000})
	require.NoError(t, err)
	return stubTopology{inst: inst}
}

func clearResolverEnv(t *testing.T) {
	t.Helper()
	t.Setenv(EnvURL, "")
	t.Setenv(EnvAccessToken, "")
	t.Setenv(EnvEndpointURLPrefix+"BACKEND", "")
	t.Setenv(EnvEndpointURLPrefix+"CONTENT_SAFETY", "")
}

func TestHostResolverDefaultsToTheContainer(t *testing.T) {
	clearResolverEnv(t)
	topo := newStubTopology(t)

	host, err := HostURL(topo, "backend")
	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:41000", host)

	upstream, err := UpstreamURL(topo, "backend")
	require.NoError(t, err)
	require.Equal(t, "http://testbench:3000", upstream)

	configured, err := Configured("backend")
	require.NoError(t, err)
	require.False(t, configured)
	require.Nil(t, AuthHeaders("backend"))
}

func TestHostResolverUsesTheConfiguredBaseURLForEveryEndpoint(t *testing.T) {
	clearResolverEnv(t)
	t.Setenv(EnvURL, " https://hosted.example.com/base/ ")

	for _, endpoint := range []string{"backend", "jwks", "content-safety"} {
		host, err := HostURL(nil, endpoint)
		require.NoError(t, err, endpoint)
		require.Equal(t, "https://hosted.example.com/base", host, endpoint)

		upstream, err := UpstreamURL(nil, endpoint)
		require.NoError(t, err, endpoint)
		require.Equal(t, host, upstream, endpoint)
	}
	configured, err := Configured("backend")
	require.NoError(t, err)
	require.True(t, configured)
}

func TestHostResolverEndpointOverrideWinsOverBaseURL(t *testing.T) {
	clearResolverEnv(t)
	t.Setenv(EnvURL, "https://hosted.example.com")
	t.Setenv(EnvEndpointURLPrefix+"CONTENT_SAFETY", "http://safety.example.com:8080")

	host, err := HostURL(nil, "content-safety")
	require.NoError(t, err)
	require.Equal(t, "http://safety.example.com:8080", host)

	host, err = HostURL(nil, "backend")
	require.NoError(t, err)
	require.Equal(t, "https://hosted.example.com", host)
}

func TestHostResolverRejectsMalformedURLsWithoutEchoingThem(t *testing.T) {
	const secret = "tok3n-in-the-url"
	for name, value := range map[string]string{
		"missing scheme":   "hosted.example.com/" + secret,
		"unsupported":      "ftp://hosted.example.com/" + secret,
		"no host":          "https:///" + secret,
		"credentials":      "https://user:" + secret + "@hosted.example.com",
		"query":            "https://hosted.example.com/?key=" + secret,
		"fragment":         "https://hosted.example.com/#" + secret,
		"unparseable":      "http://[::1/" + secret,
		"control sequence": "https://hosted.example.com/\x7f" + secret,
	} {
		t.Run(name, func(t *testing.T) {
			clearResolverEnv(t)
			t.Setenv(EnvURL, value)

			_, err := HostURL(nil, "backend")
			require.Error(t, err)
			require.Contains(t, err.Error(), EnvURL)
			require.NotContains(t, err.Error(), secret)

			_, err = UpstreamURL(nil, "backend")
			require.Error(t, err)

			_, err = Configured("backend")
			require.Error(t, err)
		})
	}
}

func TestHostResolverWithoutTopologyOrComponentFails(t *testing.T) {
	clearResolverEnv(t)

	_, err := HostURL(nil, "backend")
	require.ErrorContains(t, err, "no topology")
	_, err = UpstreamURL(nil, "backend")
	require.ErrorContains(t, err, "no topology")

	topo := newStubTopology(t)
	_, err = HostURL(topo, "no-such-endpoint")
	require.Error(t, err)
	_, err = UpstreamURL(topo, "no-such-endpoint")
	require.Error(t, err)
}

// A block without a testbench component (the AI workspace CLI block, for one) still creates APIs
// whose upstream names the testbench, so the upstream address must not need the component.
func TestUpstreamURLDoesNotNeedTheTestbenchInTheBlock(t *testing.T) {
	clearResolverEnv(t)
	topo := stubTopology{}

	upstream, err := UpstreamURL(topo, "backend")
	require.NoError(t, err)
	require.Equal(t, "http://testbench:3000", upstream)

	upstream, err = UpstreamURL(topo, "mcp")
	require.NoError(t, err)
	require.Equal(t, "http://testbench:3009", upstream)

	_, err = UpstreamURL(topo, "no-such-endpoint")
	require.ErrorContains(t, err, "no endpoint")

	_, err = HostURL(topo, "backend")
	require.Error(t, err, "the test process cannot reach a testbench the block does not run")
}

func TestAuthHeadersCarryTheBearerTokenToAHostedEndpointOnly(t *testing.T) {
	clearResolverEnv(t)
	t.Setenv(EnvAccessToken, "  abc.def  ")

	require.Nil(t, AuthHeaders("backend"), "the container is not sent the token")

	t.Setenv(EnvEndpointURLPrefix+"JWKS", "https://jwks.example.com")
	require.Equal(t, map[string]string{"Authorization": "Bearer abc.def"}, AuthHeaders("jwks"))
	require.Nil(t, AuthHeaders("backend"), "an endpoint that is not hosted is not sent the token")

	t.Setenv(EnvURL, "https://hosted.example.com")
	require.Equal(t, map[string]string{"Authorization": "Bearer abc.def"}, AuthHeaders("backend"))

	t.Setenv(EnvAccessToken, "   ")
	require.Nil(t, AuthHeaders("backend"))
}

func TestTheTokenIsNeverSentInClearText(t *testing.T) {
	const token = "tok3n-must-not-leak"
	for _, hosted := range []string{"http://hosted.example.com", "http://10.0.0.5:3000", "http://testbench.svc:3000"} {
		t.Run(hosted, func(t *testing.T) {
			clearResolverEnv(t)
			t.Setenv(EnvURL, hosted)
			t.Setenv(EnvAccessToken, token)

			_, err := HostURL(nil, "backend")
			require.ErrorContains(t, err, EnvURL)
			require.ErrorContains(t, err, EnvAccessToken)
			require.NotContains(t, err.Error(), token)
			_, err = UpstreamURL(nil, "backend")
			require.Error(t, err)
			require.Nil(t, AuthHeaders("backend"))
		})
	}

	for _, local := range []string{"http://localhost:3000", "http://127.0.0.1:3000", "http://[::1]:3000", "https://hosted.example.com"} {
		t.Run(local, func(t *testing.T) {
			clearResolverEnv(t)
			t.Setenv(EnvURL, local)
			t.Setenv(EnvAccessToken, token)

			_, err := HostURL(nil, "backend")
			require.NoError(t, err)
			require.NotNil(t, AuthHeaders("backend"))
		})
	}

	clearResolverEnv(t)
	t.Setenv(EnvURL, "http://hosted.example.com")
	_, err := HostURL(nil, "backend")
	require.NoError(t, err, "plain http is allowed when there is no token to protect")
}

func TestWithAuthKeepsACallersAuthorization(t *testing.T) {
	clearResolverEnv(t)
	t.Setenv(EnvURL, "https://hosted.example.com")
	t.Setenv(EnvAccessToken, "abc.def")

	merged := WithAuth("backend", map[string]string{"X-Scenario": "one"})
	require.Equal(t, map[string]string{"X-Scenario": "one", "Authorization": "Bearer abc.def"}, merged)

	require.Equal(t, map[string]string{"Authorization": "Bearer abc.def"}, WithAuth("backend", nil))

	for _, name := range []string{"Authorization", "authorization", "AUTHORIZATION"} {
		own := map[string]string{name: "Basic own", "X-Scenario": "one"}
		require.Equal(t, own, WithAuth("backend", own), name)
	}

	caller := map[string]string{"X-Scenario": "one"}
	WithAuth("backend", caller)
	require.Len(t, caller, 1, "the caller's map is not modified")

	t.Setenv(EnvAccessToken, "")
	require.Equal(t, caller, WithAuth("backend", caller))
	require.Nil(t, WithAuth("backend", nil))
}
