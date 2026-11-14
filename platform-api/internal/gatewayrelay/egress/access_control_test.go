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

package egress

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/httpkit/netguard"
)

func policyWithAccessControl(t *testing.T, gc netguard.GuardConfig, hosts ...string) *Policy {
	t.Helper()
	access, err := gc.Build()
	require.NoError(t, err)
	p, err := Parse(Spec{AllowHosts: hosts, AccessControl: &access})
	require.NoError(t, err)
	return p
}

// TestAccessControlNarrowsAllowHosts proves a gateway target must pass both
// allow_hosts and server.network_security.access_control.
func TestAccessControlNarrowsAllowHosts(t *testing.T) {
	p := policyWithAccessControl(t, netguard.GuardConfig{
		Enabled: true, Mode: netguard.ModeDeny, Hostnames: []string{`blocked\.gw\.internal`},
	}, "*.gw.internal")

	assert.NoError(t, check(t, p, "https://ok.gw.internal/pizza"), "positive control: both permit it")

	err := check(t, p, "https://blocked.gw.internal/pizza")
	assert.True(t, errors.Is(err, ErrTargetRefusedByAccessControl), "got %v", err)

	err = check(t, p, "https://other.example.com/pizza")
	assert.True(t, errors.Is(err, ErrTargetNotAllowed), "allow_hosts still applies first, got %v", err)
}

func TestAccessControlRefusesIPLiteralTargets(t *testing.T) {
	p := policyWithAccessControl(t, netguard.GuardConfig{Enabled: true, DenyPrivateNetworks: true}, "*")
	err := check(t, p, "http://10.1.2.3:8080/pizza")
	assert.True(t, errors.Is(err, ErrTargetRefusedByAccessControl), "got %v", err)
	assert.NoError(t, check(t, p, "https://8.8.8.8/pizza"), "positive control: a public literal passes")
}

// TestAccessControlAddressRulesApplyAtDial proves the dial combines the relay's
// own ranges with access control's, and that an allow-mode host rule (which no
// resolved IP would ever match) does not leak into the per-address check.
func TestAccessControlAddressRulesApplyAtDial(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()

	permit := policyWithAccessControl(t, netguard.GuardConfig{
		Enabled: true, Mode: netguard.ModeAllow, Hostnames: []string{`127\.0\.0\.1`},
	}, "*")
	conn, err := permit.DialContext(time.Second)(context.Background(), "tcp", ln.Addr().String())
	require.NoError(t, err, "an allow-mode host rule must not refuse the resolved address")
	conn.Close()

	deny := policyWithAccessControl(t, netguard.GuardConfig{Enabled: true, DenyPrivateNetworks: true}, "*")
	_, err = deny.DialContext(time.Second)(context.Background(), "tcp", ln.Addr().String())
	assert.True(t, errors.Is(err, ErrAddressNotAllowed), "got %v", err)

	// The relay's own fixed ranges survive the merge.
	assert.NotEmpty(t, deny.netguardPolicy.DenyCIDRs)
}
