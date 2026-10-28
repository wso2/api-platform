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

package otelcollector

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCollectorDefinition(t *testing.T) {
	definition := Collector()
	require.NoError(t, definition.Validate())
	require.Equal(t, Name, definition.Name)
	require.Equal(t, Name, definition.Alias)
	require.False(t, definition.Shared, "the collector keeps spans in memory, so each block needs its own")
	require.Equal(t, imgOTelCollector, definition.Image.Ref)

	for name, port := range map[string]int{"grpc": GRPCPort, "http": HTTPPort, "api": APIPort} {
		endpoint, ok := definition.Endpoint(name)
		require.True(t, ok, name)
		require.Equal(t, port, endpoint.Port, name)
		require.True(t, endpoint.AwaitListening, name)
	}

	require.NotNil(t, definition.Health)
	require.Equal(t, "api", definition.Health.Endpoint)
	require.Equal(t, "/test/health", definition.Health.Path)
}

func TestCollectorImageOverride(t *testing.T) {
	t.Setenv(EnvImageOTelCollector, "registry.example/mock-otlp-collector:pinned")
	require.Equal(t, "registry.example/mock-otlp-collector:pinned", Collector().Image.Ref)

	t.Setenv(EnvImageOTelCollector, "   ")
	require.Equal(t, imgOTelCollector, Collector().Image.Ref, "a blank override keeps the default")
}
