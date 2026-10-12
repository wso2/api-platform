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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
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
