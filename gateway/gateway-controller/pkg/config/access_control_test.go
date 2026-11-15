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
)

func TestArtifactAccessControlPolicy_DefaultsAreEnabledAndPermitPrivate(t *testing.T) {
	cfg := defaultConfig()
	policy, enabled, err := cfg.ArtifactAccessControlPolicy()
	if err != nil {
		t.Fatalf("defaults must be valid: %v", err)
	}
	if !enabled {
		t.Fatal("access control is on by default")
	}
	if policy.BlockPrivate || policy.BlockLoopback || !policy.BlockLinkLocal {
		t.Fatalf("defaults must permit private networks and refuse link-local, got %+v", policy)
	}
}

func TestArtifactAccessControlPolicy_RejectsInvalidSettings(t *testing.T) {
	for name, ac := range map[string]AccessControlConfig{
		"allow with no hostnames": {Enabled: true, Mode: "allow"},
		"invalid pattern":         {Enabled: true, Hostnames: []string{"(unclosed"}},
		"unknown mode":            {Enabled: true, Mode: "maybe"},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := defaultConfig()
			cfg.Controller.Server.NetworkSecurity.AccessControl = ac
			_, _, err := cfg.ArtifactAccessControlPolicy()
			if err == nil || !strings.Contains(err.Error(), "controller.server.network_security.access_control") {
				t.Fatalf("got %v, want an error naming the section", err)
			}
		})
	}
}

func TestArtifactAccessControlPolicy_DisabledIsNotApplied(t *testing.T) {
	cfg := defaultConfig()
	cfg.Controller.Server.NetworkSecurity.AccessControl = AccessControlConfig{Enabled: false, Mode: "allow"}
	if _, enabled, err := cfg.ArtifactAccessControlPolicy(); err != nil || enabled {
		t.Fatalf("got enabled=%v err=%v, want a disabled, valid configuration", enabled, err)
	}
}
