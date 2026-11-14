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
)

func TestAccessControlDefaults(t *testing.T) {
	ac := defaultConfig().Listeners.NetworkSecurity.AccessControl
	if !ac.Enabled || ac.Mode != "deny" || ac.DenyPrivateNetworks || len(ac.Hostnames) != 0 {
		t.Fatalf("default access control = %+v, want enabled deny mode, no hostnames, private networks permitted", ac)
	}
	if err := validateListenersConfig(&ServerListeners{NetworkSecurity: NetworkSecurity{AccessControl: ac}}); err != nil {
		t.Fatalf("the defaults must validate: %v", err)
	}
}

func TestAccessControlValidation(t *testing.T) {
	cases := map[string]AccessControl{
		"unknown mode":       {Enabled: true, Mode: "block"},
		"allow with none":    {Enabled: true, Mode: "allow"},
		"invalid regex":      {Enabled: true, Hostnames: []string{"api.(example"}},
		"unsupported scheme": {Enabled: true, AllowedSchemes: []string{"ftp"}},
	}
	for name, ac := range cases {
		t.Run(name, func(t *testing.T) {
			err := validateListenersConfig(&ServerListeners{NetworkSecurity: NetworkSecurity{AccessControl: ac}})
			if err == nil || !strings.Contains(err.Error(), "server.network_security.access_control") {
				t.Fatalf("validateListenersConfig = %v, want an error naming the section", err)
			}
		})
	}
}
