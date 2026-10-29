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

package health

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAllowedClientsDefaultsToLoopback(t *testing.T) {
	for _, entries := range [][]string{nil, {}} {
		a, err := ParseAllowedClients("controller.health", entries)
		require.NoError(t, err)
		require.NotNil(t, a)
		assert.Equal(t, []netip.Prefix{
			netip.MustParsePrefix("127.0.0.1/32"),
			netip.MustParsePrefix("::1/128"),
		}, a.Prefixes())
		assert.True(t, a.Allows("127.0.0.1:1234"))
		assert.True(t, a.Allows("[::1]:1234"))
		assert.False(t, a.Allows("127.0.0.2:1234"))
		assert.False(t, a.Allows("10.0.0.1:1234"))
	}
}

func TestParseAllowedClientsWildcard(t *testing.T) {
	a, err := ParseAllowedClients("controller.health", []string{"*"})
	require.NoError(t, err)
	assert.Nil(t, a)
	assert.True(t, a.Allows("203.0.113.9:1234"))
	assert.True(t, a.Allows("not an address"))
	assert.Nil(t, a.Prefixes())
}

func TestParseAllowedClientsWildcardWithOthers(t *testing.T) {
	for _, entries := range [][]string{{"*", "10.0.0.0/8"}, {"127.0.0.1", "*"}} {
		_, err := ParseAllowedClients("router.health", entries)
		require.Error(t, err)
		assert.Equal(t, `router.health.allowed_clients: "*" cannot be combined with other entries`, err.Error())
	}
}

func TestParseAllowedClientsInvalidEntry(t *testing.T) {
	tests := []struct {
		entries []string
		want    string
	}{
		{[]string{"localhost"}, "controller.health.allowed_clients[0] is not a valid IP address or CIDR range"},
		{[]string{"127.0.0.1", "10.0.0.0/33"}, "controller.health.allowed_clients[1] is not a valid IP address or CIDR range"},
		{[]string{"::1", "10.0.0.1", ""}, "controller.health.allowed_clients[2] is not a valid IP address or CIDR range"},
		{[]string{"fe80::1%eth0"}, "controller.health.allowed_clients[0] is not a valid IP address or CIDR range"},
		{[]string{"10.0.0.0/x"}, "controller.health.allowed_clients[0] is not a valid IP address or CIDR range"},
	}
	for _, tc := range tests {
		_, err := ParseAllowedClients("controller.health", tc.entries)
		require.Error(t, err, tc.entries)
		assert.Equal(t, tc.want, err.Error())
	}
}

func TestParseAllowedClientsNormalizesEntries(t *testing.T) {
	a, err := ParseAllowedClients("controller.health", []string{
		" 10.1.2.3/8 ", "fd00::5", "::ffff:192.168.1.1", "::ffff:172.16.0.0/108", "::ffff:0:0/80",
	})
	require.NoError(t, err)
	assert.Equal(t, []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("fd00::5/128"),
		netip.MustParsePrefix("192.168.1.1/32"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("::/80"),
	}, a.Prefixes())
}

func TestAllowsMatching(t *testing.T) {
	a, err := ParseAllowedClients("controller.health", []string{"10.0.0.0/8", "fc00::/7", "192.168.1.5", "::1"})
	require.NoError(t, err)

	tests := []struct {
		remote string
		want   bool
	}{
		{"10.0.0.1:80", true},
		{"10.255.255.255:80", true},
		{"11.0.0.1:80", false},
		{"[fd12::1]:80", true},
		{"[fe80::1]:80", false},
		{"192.168.1.5:80", true},
		{"192.168.1.6:80", false},
		{"[::1]:80", true},
		{"[::ffff:10.1.1.1]:80", true},
		{"[::ffff:11.1.1.1]:80", false},
		{"[fd00::1%eth0]:80", true},
		{"127.0.0.1:80", false},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, a.Allows(tc.remote), tc.remote)
	}
}

func TestAllowsMappedLoopback(t *testing.T) {
	a, err := ParseAllowedClients("controller.health", nil)
	require.NoError(t, err)
	assert.True(t, a.Allows("[::ffff:127.0.0.1]:1234"))
}

func TestAllowsRefusesMalformedRemoteAddr(t *testing.T) {
	a, err := ParseAllowedClients("controller.health", nil)
	require.NoError(t, err)
	for _, remote := range []string{"", "127.0.0.1", "localhost:80", "[::1", "::1", "garbage:80"} {
		assert.False(t, a.Allows(remote), remote)
	}
}

func TestPrefixesReturnsCopy(t *testing.T) {
	a, err := ParseAllowedClients("controller.health", []string{"10.0.0.0/8"})
	require.NoError(t, err)
	p := a.Prefixes()
	p[0] = netip.MustParsePrefix("0.0.0.0/0")
	assert.False(t, a.Allows("11.0.0.1:80"))
}
