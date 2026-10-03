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

package a2aagent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTripPlannerDefinition(t *testing.T) {
	definition := TripPlanner()
	require.NoError(t, definition.Validate())
	require.Equal(t, Name, definition.Name)
	require.Equal(t, Name, definition.Alias)
	require.False(t, definition.Shared, "the agent keeps tasks in memory, so each block needs its own")
	require.Equal(t, imgA2ATripPlanner, definition.Image.Ref)

	endpoint, ok := definition.Endpoint("http")
	require.True(t, ok)
	require.Equal(t, Port, endpoint.Port)
	require.True(t, endpoint.AwaitListening)

	require.NotNil(t, definition.Health)
	require.Equal(t, "/.well-known/agent-card.json", definition.Health.Path)
	require.Equal(t, "http://a2a-trip-planner:9099", definition.Env["TRIP_PUBLIC_URL"])
	require.Equal(t, "9099", definition.Env["TRIP_PORT"])
}

func TestTripPlannerImageOverride(t *testing.T) {
	t.Setenv(EnvImageA2ATripPlanner, "registry.example/a2a-trip-planner:pinned")
	require.Equal(t, "registry.example/a2a-trip-planner:pinned", TripPlanner().Image.Ref)

	t.Setenv(EnvImageA2ATripPlanner, "   ")
	require.Equal(t, imgA2ATripPlanner, TripPlanner().Image.Ref, "a blank override keeps the default")
}
