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

func validTestConsole() TestConsoleConfig {
	return TestConsoleConfig{
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

func TestTestConsoleDefaultsAreBounded(t *testing.T) {
	// The relay ships on, so its defaults are what make it safe out of the box.
	// A zero anywhere here would be an unbounded relay in every fresh install.
	tc := defaultConfig().TestConsole
	if !tc.Enabled {
		t.Error("the test console relay should default to enabled")
	}
	if err := tc.validate(); err != nil {
		t.Fatalf("the shipped defaults must pass their own validation: %v", err)
	}
	// Off by default: it would disable gateway certificate verification.
	if tc.TLSSkipVerify {
		t.Error("test_console.tls_skip_verify must default to false")
	}
}

func TestTestConsoleValidateRejectsUnboundedValues(t *testing.T) {
	// Each of these is checked on its effective value rather than on Enabled
	// alone: a relay switched on with any one of them at zero has no bound at
	// all, which is worse than the feature being off.
	for _, tc := range []struct {
		name   string
		mutate func(*TestConsoleConfig)
	}{
		{"zero timeout", func(c *TestConsoleConfig) { c.RequestTimeout = 0 }},
		{"negative timeout", func(c *TestConsoleConfig) { c.RequestTimeout = -time.Second }},
		{"zero request ceiling", func(c *TestConsoleConfig) { c.MaxRequestBytes = 0 }},
		{"zero response ceiling", func(c *TestConsoleConfig) { c.MaxResponseBytes = 0 }},
		{"zero concurrency", func(c *TestConsoleConfig) { c.MaxConcurrent = 0 }},
		{"negative queue", func(c *TestConsoleConfig) { c.MaxPending = -1 }},
		{"negative cache ttl", func(c *TestConsoleConfig) { c.ResolveCacheTTL = -time.Second }},
		{"malformed deny cidr", func(c *TestConsoleConfig) { c.DenyCIDRs = []string{"10.0.0.0"} }},
		{"malformed allow cidr", func(c *TestConsoleConfig) { c.AllowCIDRs = []string{"nonsense"} }},
		{"contradictory tls settings", func(c *TestConsoleConfig) {
			c.TLSSkipVerify = true
			c.CAFile = "/etc/ca.pem"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validTestConsole()
			tc.mutate(&cfg)
			if err := cfg.validate(); err == nil {
				t.Error("expected startup to refuse this configuration")
			}
		})
	}
}

func TestTestConsoleValidateIgnoresBoundsWhenDisabled(t *testing.T) {
	// Nothing is constructed when the relay is off, so an unset bound is not a
	// misconfiguration — it is simply unused.
	if err := (TestConsoleConfig{Enabled: false}).validate(); err != nil {
		t.Errorf("a disabled relay should not require any bound to be set: %v", err)
	}
}

func TestTestConsoleValidateAcceptsWellFormedCIDRs(t *testing.T) {
	cfg := validTestConsole()
	cfg.DenyCIDRs = []string{"10.42.0.0/16", "fd00::/8"}
	cfg.AllowCIDRs = []string{"10.42.7.0/24"}
	if err := cfg.validate(); err != nil {
		t.Errorf("validate() = %v, want nil", err)
	}
}
