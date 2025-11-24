/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Parser handles parsing of API configuration files
type Parser struct{}

// NewParser creates a new configuration parser
func NewParser() *Parser {
	return &Parser{}
}

// ParseYAML parses YAML content into an API configuration
func (p *Parser) ParseYAML(data []byte, config interface{}) error {
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	return nil
}

// ParseJSON parses JSON content into an API configuration
func (p *Parser) ParseJSON(data []byte, config interface{}) error {
	if err := json.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	return nil
}

// Parse attempts to parse data as either YAML or JSON
func (p *Parser) Parse(data []byte, contentType string, config interface{}) error {
	switch contentType {
	case "application/yaml", "application/x-yaml", "text/yaml":
		return p.ParseYAML(data, config)
	case "application/json":
		return p.ParseJSON(data, config)
	default:
		// Try YAML first, then JSON
		if err := p.ParseYAML(data, config); err == nil {
			return nil
		}

		if err := p.ParseJSON(data, config); err == nil {
			return nil
		}

		return fmt.Errorf("failed to parse as YAML or JSON")
	}
}
