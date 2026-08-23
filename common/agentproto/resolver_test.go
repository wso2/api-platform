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

package agentproto

import (
	"encoding/json"
	"testing"
)

// ResolverConfig travels over xDS from the controller to the policy engine, so
// its JSON keys are a wire contract rather than an implementation detail. A
// rename here is invisible at compile time on both sides and shows up only as
// routes that resolve to nothing.
func TestResolverConfigWireShape(t *testing.T) {
	tests := []struct {
		name   string
		config ResolverConfig
		want   string
	}{
		{
			name:   "HTTP+JSON route carries its operation",
			config: ResolverConfig{ProtocolVersion: V1_0, Transport: TransportHTTPJSON, Operation: SendMessage},
			want:   `{"protocolVersion":"1.0","transport":"HTTP+JSON","operation":"SendMessage"}`,
		},
		{
			// The JSON-RPC endpoint reads its method from the request body, so
			// the field is omitted rather than sent empty — an empty operation
			// on the wire would be indistinguishable from one the controller
			// failed to fill in.
			name:   "JSONRPC route omits the operation",
			config: ResolverConfig{ProtocolVersion: V1_0, Transport: TransportJSONRPC},
			want:   `{"protocolVersion":"1.0","transport":"JSONRPC"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := json.Marshal(tt.config)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(encoded) != tt.want {
				t.Errorf("encoded as %s, want %s", encoded, tt.want)
			}

			var decoded ResolverConfig
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if decoded != tt.config {
				t.Errorf("round-tripped to %+v, want %+v", decoded, tt.config)
			}
		})
	}
}

// The transport values are the management API's protocolBinding spellings, so an
// operator reading a route's resolver_config sees what they wrote in the Agent.
func TestTransportValuesMatchTheConfiguredSpellings(t *testing.T) {
	if TransportJSONRPC != "JSONRPC" {
		t.Errorf("TransportJSONRPC = %q", TransportJSONRPC)
	}
	if TransportHTTPJSON != "HTTP+JSON" {
		t.Errorf("TransportHTTPJSON = %q", TransportHTTPJSON)
	}
	if ResolverName != "a2a" {
		t.Errorf("ResolverName = %q; the engine registers its factory under this name", ResolverName)
	}
}
