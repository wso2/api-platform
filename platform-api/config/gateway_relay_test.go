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

package config

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultGatewayRelayConfigIsValid(t *testing.T) {
	cfg := defaultConfig()
	if err := validateGatewayRelayConfig(&cfg.GatewayRelay, cfg.Listeners.Timeouts.Write); err != nil {
		t.Fatalf("default gateway_relay config rejected: %v", err)
	}
}

func TestGatewayRelayConfigRejectsUnboundedOrConflictingSettings(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(*GatewayRelay)
		want   string
	}{
		"no timeout":          {func(c *GatewayRelay) { c.RequestTimeout = 0 }, "request_timeout"},
		"no request bound":    {func(c *GatewayRelay) { c.MaxRequestBytes = 0 }, "max_request_bytes"},
		"no response bound":   {func(c *GatewayRelay) { c.MaxResponseBytes = 0 }, "max_response_bytes"},
		"no concurrency":      {func(c *GatewayRelay) { c.MaxConcurrent = 0 }, "max_concurrent"},
		"negative queue":      {func(c *GatewayRelay) { c.MaxPending = -1 }, "max_pending"},
		"CA and skip-verify":  {func(c *GatewayRelay) { c.CAFile, c.TLSSkipVerify = "/ca.pem", true }, "mutually exclusive"},
		"outlives the server": {func(c *GatewayRelay) { c.RequestTimeout = 2 * time.Minute }, "server.timeouts.write"},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := defaultConfig()
			tc.mutate(&cfg.GatewayRelay)
			err := validateGatewayRelayConfig(&cfg.GatewayRelay, cfg.Listeners.Timeouts.Write)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want an error mentioning %q", err, tc.want)
			}
		})
	}
}
