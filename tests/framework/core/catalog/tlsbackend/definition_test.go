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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package tlsbackend

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/util/testpki"
	service "github.com/wso2/api-platform/tests/framework/testbench/services/tlsbackend"
)

func TestTLSBackendDefinition(t *testing.T) {
	definition := TLSBackend()
	require.NoError(t, definition.Validate())
	require.Equal(t, "tls-backend", definition.Name)
	require.Equal(t, testpki.TLSBackendHost, definition.Alias)
	require.True(t, definition.Shared)
	for name, port := range map[string]int{"a": service.PortA, "b": service.PortB, "wronghost": service.PortWrongHost, "optional": service.PortOptional} {
		endpoint, ok := definition.Endpoint(name)
		require.True(t, ok, name)
		require.Equal(t, port, endpoint.Port, name)
		require.Equal(t, "https", endpoint.Scheme, name)
	}
}

func TestEnvironmentBuildsOneServicePerEndpoint(t *testing.T) {
	definition := TLSBackend()
	services, err := service.FromEnv(definition.Env[service.EnvBackends])
	require.NoError(t, err)
	require.Len(t, services, len(definition.Endpoints))
	for i, svc := range services {
		require.Equal(t, definition.Endpoints[i].Port, svc.Port())
	}
}

func TestBackendsTrustTheAuthoritiesTheirPartnersUse(t *testing.T) {
	set, err := testpki.Generate(time.Now())
	require.NoError(t, err)
	list, err := backends(set)
	require.NoError(t, err)
	require.Len(t, list, 4)

	want := map[string][2]string{
		"a":         {"backend-server-a", "ca-a"},
		"b":         {"backend-server-b", "ca-b"},
		"wronghost": {"backend-server-wronghost", "ca-a"},
		"optional":  {"backend-server-a", "ca-a"},
	}
	for _, b := range list {
		server, err := set.Get(want[b.Name][0])
		require.NoError(t, err)
		authority, err := set.Get(want[b.Name][1])
		require.NoError(t, err)
		require.Equal(t, string(server.CertPEM), b.Certificate, b.Name)
		require.Equal(t, string(authority.CertPEM), b.ClientCAs, b.Name)
	}

	_, err = backends(&testpki.Set{})
	require.Error(t, err)
}
