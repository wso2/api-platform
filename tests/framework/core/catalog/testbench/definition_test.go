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
	"testing"

	"github.com/stretchr/testify/require"
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
