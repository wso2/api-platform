/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package health

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAllowedClientsDefaultsToLoopback(t *testing.T) {
	for _, entries := range [][]string{nil, {}} {
		set, err := ParseAllowedClients("platform_api.health", entries)
		require.NoError(t, err)

		assert.True(t, set.Allows("127.0.0.1:41000"))
		assert.True(t, set.Allows("[::1]:41000"))
		assert.True(t, set.Allows("[::ffff:127.0.0.1]:41000"))
		assert.False(t, set.Allows("127.0.0.2:41000"))
		assert.False(t, set.Allows("10.0.0.5:41000"))
		assert.False(t, set.Allows("[fd00::5]:41000"))
	}
}

func TestParseAllowedClientsWildcardAllowsEveryone(t *testing.T) {
	set, err := ParseAllowedClients("platform_api.health", []string{" * "})
	require.NoError(t, err)

	assert.True(t, set.Allows("203.0.113.9:443"))
	assert.True(t, set.Allows("[2001:db8::1]:443"))
	assert.True(t, set.Allows("not an address"))
}

func TestAllowedClientsNilAllowsEveryone(t *testing.T) {
	var set *AllowedClients

	assert.True(t, set.Allows("203.0.113.9:443"))
}

func TestAllowedClientsMatchesAddressesAndRanges(t *testing.T) {
	set, err := ParseAllowedClients("platform_api.health",
		[]string{"10.0.0.0/8", " 192.168.1.10 ", "fc00::/7", "::ffff:172.16.0.0/108", "::ffff:198.51.100.7"})
	require.NoError(t, err)

	tests := []struct {
		remoteAddr string
		want       bool
	}{
		{"10.1.2.3:5000", true},
		{"[::ffff:10.1.2.3]:5000", true},
		{"11.0.0.1:5000", false},
		{"192.168.1.10:5000", true},
		{"192.168.1.11:5000", false},
		{"[fd00::5]:5000", true},
		{"[fe80::1%eth0]:5000", false},
		{"[2001:db8::1]:5000", false},
		{"172.16.3.4:5000", true},
		{"172.32.0.1:5000", false},
		{"198.51.100.7:5000", true},
		{"127.0.0.1:5000", false},
	}
	for _, tt := range tests {
		t.Run(tt.remoteAddr, func(t *testing.T) {
			assert.Equal(t, tt.want, set.Allows(tt.remoteAddr))
		})
	}
}

func TestAllowedClientsIgnoresZone(t *testing.T) {
	set, err := ParseAllowedClients("platform_api.health", []string{"fe80::/10"})
	require.NoError(t, err)

	assert.True(t, set.Allows("[fe80::1%eth0]:5000"))
}

func TestAllowedClientsRefusesMalformedRemoteAddr(t *testing.T) {
	set, err := ParseAllowedClients("platform_api.health", nil)
	require.NoError(t, err)

	for _, remoteAddr := range []string{"", "127.0.0.1", "::1", "localhost:80", "[::1", "127.0.0.1:80:90"} {
		assert.False(t, set.Allows(remoteAddr), remoteAddr)
	}
}

func TestParseAllowedClientsErrors(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		wantErr string
	}{
		{"wildcard with other entries", []string{"127.0.0.1", "*"},
			`platform_api.health.allowed_clients: "*" cannot be combined with other entries`},
		{"wildcard first", []string{"*", "10.0.0.0/8"},
			`platform_api.health.allowed_clients: "*" cannot be combined with other entries`},
		{"host name", []string{"127.0.0.1", "localhost"},
			"platform_api.health.allowed_clients[1] is not a valid IP address or CIDR range"},
		{"bad prefix length", []string{"10.0.0.0/33"},
			"platform_api.health.allowed_clients[0] is not a valid IP address or CIDR range"},
		{"mapped prefix wider than IPv4", []string{"::ffff:0:0/90"},
			"platform_api.health.allowed_clients[0] is not a valid IP address or CIDR range"},
		{"empty entry", []string{"127.0.0.1", ""},
			"platform_api.health.allowed_clients[1] is not a valid IP address or CIDR range"},
		{"port", []string{"127.0.0.1:80"},
			"platform_api.health.allowed_clients[0] is not a valid IP address or CIDR range"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set, err := ParseAllowedClients("platform_api.health", tt.entries)

			assert.Nil(t, set)
			require.EqualError(t, err, tt.wantErr)
		})
	}
}
