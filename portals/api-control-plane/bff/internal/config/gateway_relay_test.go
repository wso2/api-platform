/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package config

import (
	"testing"
	"time"
)

func validGatewayRelay() GatewayRelayConfig {
	return GatewayRelayConfig{
		Enabled:          true,
		RequestTimeout:   30 * time.Second,
		MaxRequestBytes:  1 << 20,
		MaxResponseBytes: 1 << 20,
		MaxConcurrent:    8,
		MaxPending:       8,
		ResolveCacheTTL:  time.Minute,
		ResolveCacheSize: 64,
	}
}

func TestGatewayRelayDefaultsAreBounded(t *testing.T) {
	// The relay ships on, so its defaults are what make it safe out of the box.
	// A zero anywhere here would be an unbounded relay in every fresh install.
	cfg := defaultConfig()
	tc := cfg.GatewayRelay
	if !tc.Enabled {
		t.Error("the gateway relay relay should default to enabled")
	}
	if err := cfg.validateGatewayRelay(); err != nil {
		t.Fatalf("the shipped defaults must pass their own validation: %v", err)
	}
	// The shipped default must parse, or the server would refuse to wire the
	// relay even though validation passed.
	if _, err := tc.Egress.Policy(); err != nil {
		t.Errorf("the shipped default egress block must parse: %v", err)
	}
	// Off by default: it would disable gateway certificate verification.
	if tc.TLSSkipVerify {
		t.Error("gateway_relay.tls_skip_verify must default to false")
	}
}

func TestGatewayRelayValidateRejectsUnboundedValues(t *testing.T) {
	// Each of these is checked on its effective value rather than on Enabled
	// alone: a relay switched on with any one of them at zero has no bound at
	// all, which is worse than the feature being off.
	for _, tc := range []struct {
		name   string
		mutate func(*GatewayRelayConfig)
	}{
		{"zero timeout", func(c *GatewayRelayConfig) { c.RequestTimeout = 0 }},
		{"negative timeout", func(c *GatewayRelayConfig) { c.RequestTimeout = -time.Second }},
		{"zero request ceiling", func(c *GatewayRelayConfig) { c.MaxRequestBytes = 0 }},
		{"zero response ceiling", func(c *GatewayRelayConfig) { c.MaxResponseBytes = 0 }},
		{"zero concurrency", func(c *GatewayRelayConfig) { c.MaxConcurrent = 0 }},
		{"negative queue", func(c *GatewayRelayConfig) { c.MaxPending = -1 }},
		{"negative cache ttl", func(c *GatewayRelayConfig) { c.ResolveCacheTTL = -time.Second }},
		// allow_hosts entries that would not do what they read as doing. A
		// bare "*" is NOT here: it is the open setting, and an operator who
		// means it is allowed to say it.
		{"empty allow host", func(c *GatewayRelayConfig) { c.Egress.AllowHosts = []string{""} }},
		{"unsupported scheme", func(c *GatewayRelayConfig) { c.Egress.AllowHosts = []string{"ftp://gw.example.com"} }},
		{"allow host with a path", func(c *GatewayRelayConfig) { c.Egress.AllowHosts = []string{"gw.example.com/pizza"} }},
		{"allow host port out of range", func(c *GatewayRelayConfig) { c.Egress.AllowHosts = []string{"gw.example.com:70000"} }},
		{"unbracketed ipv6 allow host", func(c *GatewayRelayConfig) { c.Egress.AllowHosts = []string{"fd00::1:8443"} }},
		// A typo must not quietly land on the permissive environment.
		{"contradictory tls settings", func(c *GatewayRelayConfig) {
			c.TLSSkipVerify = true
			c.CAFile = "/etc/ca.pem"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tcc := validGatewayRelay()
			tc.mutate(&tcc)
			cfg := &Config{GatewayRelay: tcc}
			if err := cfg.validateGatewayRelay(); err == nil {
				t.Error("expected startup to refuse this configuration")
			}
		})
	}
}

func TestGatewayRelayValidateIgnoresBoundsWhenDisabled(t *testing.T) {
	// Nothing is constructed when the relay is off, so an unset bound is not a
	// misconfiguration — it is simply unused.
	cfg := &Config{GatewayRelay: GatewayRelayConfig{Enabled: false}}
	if err := cfg.validateGatewayRelay(); err != nil {
		t.Errorf("a disabled relay should not require any bound to be set: %v", err)
	}
}

func TestGatewayRelayValidateAcceptsANarrowedEgressPolicy(t *testing.T) {
	tcc := validGatewayRelay()
	tcc.Egress = EgressConfig{AllowHosts: []string{
		"https://gw.example.com:443",
		"https://*.gw.svc.cluster.local:9443",
		"gw-*.internal",
	}}
	cfg := &Config{GatewayRelay: tcc}
	if err := cfg.validateGatewayRelay(); err != nil {
		t.Fatalf("validateGatewayRelay() = %v, want nil", err)
	}
}

func TestGatewayRelayValidateAcceptsALoopbackAllowHost(t *testing.T) {
	// Reaching localhost is a thing an operator may choose; nothing in the
	// policy second-guesses it.
	tcc := validGatewayRelay()
	tcc.Egress = EgressConfig{AllowHosts: []string{"http://localhost:8080"}}
	cfg := &Config{GatewayRelay: tcc}
	if err := cfg.validateGatewayRelay(); err != nil {
		t.Fatalf("validateGatewayRelay() = %v, want nil", err)
	}
}

func TestGatewayRelayValidateAcceptsAnEmptyAllowHosts(t *testing.T) {
	// Empty is a valid configuration, not a malformed one: it reaches
	// nothing. Startup warns rather than refusing, so an operator who has not
	// configured egress yet still gets a server that boots.
	tcc := validGatewayRelay()
	tcc.Egress = EgressConfig{}
	cfg := &Config{GatewayRelay: tcc}
	if err := cfg.validateGatewayRelay(); err != nil {
		t.Fatalf("validateGatewayRelay() = %v, want nil", err)
	}
}
