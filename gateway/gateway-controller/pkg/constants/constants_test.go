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

package constants

import "testing"

// TestConstants verifies that all constants are defined with expected values
func TestConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		// XDS/Envoy Constants
		{"TransportSocketPrefix", TransportSocketPrefix, "ts"},
		{"LoadBalancerIDKey", LoadBalancerIDKey, "lb_id"},
		{"TransportSocketMatchKey", TransportSocketMatchKey, "envoy.transport_socket_match"},

		// TLS Protocol Versions
		{"TLSVersion10", TLSVersion10, "TLS1_0"},
		{"TLSVersion11", TLSVersion11, "TLS1_1"},
		{"TLSVersion12", TLSVersion12, "TLS1_2"},
		{"TLSVersion13", TLSVersion13, "TLS1_3"},

		// ALPN Protocol Names
		{"ALPNProtocolHTTP2", ALPNProtocolHTTP2, "h2"},
		{"ALPNProtocolHTTP11", ALPNProtocolHTTP11, "http/1.1"},

		// TLS Cipher Configuration
		{"CipherSuiteSeparator", CipherSuiteSeparator, ","},

		// Network Configuration
		{"HTTPDefaultPort", HTTPDefaultPort, uint32(80)},
		{"HTTPSDefaultPort", HTTPSDefaultPort, uint32(443)},

		// URL Schemes
		{"SchemeHTTP", SchemeHTTP, "http"},
		{"SchemeHTTPS", SchemeHTTPS, "https"},

		// Localhost
		{"LocalhostIP", LocalhostIP, "127.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}
}
