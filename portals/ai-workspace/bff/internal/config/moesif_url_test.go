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

package config

import (
	"strings"
	"testing"
	"time"
)

// validatableConfig is the minimum that reaches the moesif_url check: everything
// validate() inspects before it, and nothing else.
func validatableConfig(moesifURL string) *Config {
	c := &Config{}
	c.Auth.Mode = AuthModeBasic
	c.Auth.Authorization.Mode = AuthzModeScope
	c.Server.HTTP.Enabled = true
	c.Server.HTTP.Port = 8080
	c.Session.Store = SessionStoreCookie
	c.Session.IdleTimeout = 30 * time.Minute
	c.Session.AbsoluteTTL = 8 * time.Hour
	c.ControlPlane.URL = "https://control-plane.example.com"
	c.ControlPlane.MoesifURL = moesifURL
	return c
}

// The moesif hop is the one upstream reached by forwarding the user's EXCHANGED
// token to a third party, so unlike control_plane.url it must not be plaintext.
func TestValidateRequiresHTTPSForMoesifURL(t *testing.T) {
	for _, tc := range []struct {
		name, url string
		wantErr   bool
	}{
		{"https is accepted", "https://analytics.example.com/cloud", false},
		{"unset is accepted", "", false},
		{"http is rejected", "http://analytics.example.com/cloud", true},
		{"scheme-relative is rejected", "//analytics.example.com/cloud", true},
		{"relative is rejected", "/cloud", true},
		{"no host is rejected", "https://", true},
		{"unparseable is rejected", "https://%zz", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validatableConfig(tc.url).validate()
			mentionsMoesif := err != nil && strings.Contains(err.Error(), "moesif_url")
			if mentionsMoesif != tc.wantErr {
				t.Fatalf("moesif_url = %q: validate() error = %v, want a moesif_url error: %v",
					tc.url, err, tc.wantErr)
			}
		})
	}
}

// moesif_tls_skip_verify no longer conflicts with the scheme — the scheme is always
// https — so setting it must warn rather than refuse to start.
func TestValidateAllowsMoesifSkipVerifyOnHTTPS(t *testing.T) {
	c := validatableConfig("https://analytics.example.com")
	c.ControlPlane.MoesifTLSSkipVerify = true
	c.ControlPlane.MoesifCAFile = "/etc/ssl/ca.pem"
	if err := c.validate(); err != nil && strings.Contains(err.Error(), "moesif") {
		t.Fatalf("validate() = %v, want no moesif error", err)
	}
}
