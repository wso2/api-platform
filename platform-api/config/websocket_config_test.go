/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package config

import (
	"strings"
	"testing"
)

func TestWebSocketMaxConnectionsPerGatewayDefaultsToUnlimited(t *testing.T) {
	if got := defaultConfig().Listeners.WebSocket.MaxConnectionsPerGateway; got != 0 {
		t.Fatalf("default max_connections_per_gateway = %d, want 0 (unlimited)", got)
	}
}

func TestValidateListenersConfigWebSocketMaxConnectionsPerGateway(t *testing.T) {
	for _, tc := range []struct {
		name    string
		limit   int
		wantErr bool
	}{
		{name: "unlimited", limit: 0},
		{name: "positive", limit: 4},
		{name: "negative", limit: -1, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateListenersConfig(&ServerListeners{WebSocket: WebSocket{MaxConnectionsPerGateway: tc.limit}})
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "max_connections_per_gateway") {
					t.Fatalf("validateListenersConfig = %v, want an error naming max_connections_per_gateway", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateListenersConfig = %v, want nil", err)
			}
		})
	}
}
