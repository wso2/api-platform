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

func enabledFeatureUsage() FeatureUsage {
	c := defaultFeatureUsage()
	c.Enabled = true
	c.CollectorURL = "http://api-platform-internal-gateway-gateway-runtime.openchoreo-data-plane:8080/moesif-collector"
	c.Token = FeatureUsageToken{
		TokenURL:     "http://platform-idp.platform-idp:8090/oauth2/token",
		ClientID:     "platform-api",
		ClientSecret: "s3cr3t-value",
	}
	return c
}

// Off by default, and the disabled defaults need no URLs or credentials.
func TestFeatureUsageIsOffByDefault(t *testing.T) {
	cfg := defaultConfig()
	if cfg.FeatureUsage.Enabled {
		t.Fatal("feature usage reporting must be off unless a deployment turns it on")
	}
	if cfg.FeatureUsage.Configured() {
		t.Fatal("no collector may be configured by default")
	}
	if err := validateFeatureUsageConfig(&cfg.FeatureUsage); err != nil {
		t.Fatalf("unconfigured defaults rejected: %v", err)
	}
}

// Configuring only part of a collector is an error, not a silent no-op.
func TestFeatureUsagePartialConfigurationIsRejected(t *testing.T) {
	for name, c := range map[string]FeatureUsage{
		"collector URL only": {Enabled: true, CollectorURL: "http://collector/moesif-collector"},
		"token client only":  {Enabled: true, Token: FeatureUsageToken{ClientID: "platform-api", ClientSecret: "s3cr3t-value"}},
	} {
		c := c
		d := defaultFeatureUsage()
		c.QueueSize, c.Workers, c.RequestTimeout, c.MaxAttempts, c.RetryBackoff, c.OverallTimeout =
			d.QueueSize, d.Workers, d.RequestTimeout, d.MaxAttempts, d.RetryBackoff, d.OverallTimeout
		if err := validateFeatureUsageConfig(&c); err == nil {
			t.Errorf("%s: a partial collector configuration was accepted", name)
		}
	}
	// Turned off, nothing is checked.
	off := FeatureUsage{Enabled: false, CollectorURL: "not a url"}
	if err := validateFeatureUsageConfig(&off); err != nil {
		t.Errorf("a disabled section was validated: %v", err)
	}
}

func TestValidateFeatureUsageConfig(t *testing.T) {
	ok := enabledFeatureUsage()
	if err := validateFeatureUsageConfig(&ok); err != nil {
		t.Fatalf("a valid enabled config was rejected: %v", err)
	}

	tests := map[string]func(c *FeatureUsage){
		"no collector URL":          func(c *FeatureUsage) { c.CollectorURL = "" },
		"non-http collector URL":    func(c *FeatureUsage) { c.CollectorURL = "file:///etc/passwd" },
		"collector URL with user":   func(c *FeatureUsage) { c.CollectorURL = "https://user:pw@collector/moesif" },
		"collector URL with query":  func(c *FeatureUsage) { c.CollectorURL = "https://collector/moesif?key=x" },
		"relative collector URL":    func(c *FeatureUsage) { c.CollectorURL = "/moesif-collector" },
		"no token URL":              func(c *FeatureUsage) { c.Token.TokenURL = "" },
		"no client ID":              func(c *FeatureUsage) { c.Token.ClientID = " " },
		"no client secret":          func(c *FeatureUsage) { c.Token.ClientSecret = "" },
		"zero queue":                func(c *FeatureUsage) { c.QueueSize = 0 },
		"unbounded queue":           func(c *FeatureUsage) { c.QueueSize = maxFeatureUsageQueueSize + 1 },
		"zero workers":              func(c *FeatureUsage) { c.Workers = 0 },
		"zero request timeout":      func(c *FeatureUsage) { c.RequestTimeout = 0 },
		"zero attempts":             func(c *FeatureUsage) { c.MaxAttempts = 0 },
		"too many attempts":         func(c *FeatureUsage) { c.MaxAttempts = maxFeatureUsageMaxAttempts + 1 },
		"negative backoff":          func(c *FeatureUsage) { c.RetryBackoff = -time.Second },
		"overall shorter than call": func(c *FeatureUsage) { c.OverallTimeout = time.Second },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			c := enabledFeatureUsage()
			mutate(&c)
			err := validateFeatureUsageConfig(&c)
			if err == nil {
				t.Fatalf("accepted a config with %s", name)
			}
			if strings.Contains(err.Error(), "pw@") || strings.Contains(err.Error(), "s3cr3t-value") {
				t.Errorf("error echoes a credential: %v", err)
			}
		})
	}
}
