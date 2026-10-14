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

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/util/testpki"
	"github.com/wso2/api-platform/tests/framework/testbench/services/tlsbackend"
)

func TestServicesServeOnlyTheTLSBackendsWhenTheirEnvironmentIsSet(t *testing.T) {
	set, err := testpki.Generate(time.Now())
	require.NoError(t, err)
	server, err := set.Get("backend-server-a")
	require.NoError(t, err)
	authority, err := set.Get("ca-a")
	require.NoError(t, err)
	value, err := tlsbackend.Encode([]tlsbackend.Backend{{
		Name: "a", Port: tlsbackend.PortA, Certificate: string(server.CertPEM),
		PrivateKey: string(server.KeyPEM), ClientCAs: string(authority.CertPEM),
	}})
	require.NoError(t, err)

	t.Setenv(tlsbackend.EnvBackends, value)
	got, err := services()
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "tls-backend-a", got[0].Name())
}

func TestServicesRejectAnEmptyTLSBackendEnvironment(t *testing.T) {
	t.Setenv(tlsbackend.EnvBackends, "")
	_, err := services()
	require.Error(t, err)
}

func TestServicesDefaultToTheGeneralPurposeSet(t *testing.T) {
	got, err := services()
	require.NoError(t, err)
	for _, svc := range got {
		require.NotContains(t, svc.Name(), "tls-backend")
	}
	require.Greater(t, len(got), 1)
}
