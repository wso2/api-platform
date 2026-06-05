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
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// OrgIDPConfig holds per-org IDP configuration loaded from a YAML file.
// Fields that can be derived from the OIDC discovery document (authorization_endpoint,
// end_session_endpoint, PKCE support, etc.) are intentionally omitted — the server
// fetches those at request time from DiscoveryURL.
type OrgIDPConfig struct {
	OrgHandle    string `yaml:"orgHandle"`
	IDPType      string `yaml:"idpType"`
	DiscoveryURL string `yaml:"discoveryUrl"`
	ClientID     string `yaml:"clientId"`
	// Issuer is the expected JWT iss claim value for tokens issued by this org's IDP.
	// Used by the auth middleware to route tokens to the correct JWKS endpoint.
	// If empty, issuer-based routing is disabled for this org.
	Issuer        string            `yaml:"issuer"`
	ClaimMappings map[string]string `yaml:"claimMappings"`
}

// LoadOrgIDPConfigs reads and parses a YAML file containing per-org IDP configurations.
func LoadOrgIDPConfigs(path string) ([]OrgIDPConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading org IDP config file %q: %w", path, err)
	}
	var configs []OrgIDPConfig
	if err := yaml.Unmarshal(data, &configs); err != nil {
		return nil, fmt.Errorf("parsing org IDP config file %q: %w", path, err)
	}
	return configs, nil
}
