/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package xdsclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestClientState_String tests the String() method for ClientState
func TestClientState_String(t *testing.T) {
	tests := []struct {
		name     string
		state    ClientState
		expected string
	}{
		{
			name:     "Disconnected state",
			state:    StateDisconnected,
			expected: "Disconnected",
		},
		{
			name:     "Connecting state",
			state:    StateConnecting,
			expected: "Connecting",
		},
		{
			name:     "Connected state",
			state:    StateConnected,
			expected: "Connected",
		},
		{
			name:     "Reconnecting state",
			state:    StateReconnecting,
			expected: "Reconnecting",
		},
		{
			name:     "Stopped state",
			state:    StateStopped,
			expected: "Stopped",
		},
		{
			name:     "Unknown state",
			state:    ClientState(999),
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.state.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}
