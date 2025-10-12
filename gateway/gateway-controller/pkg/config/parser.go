package config

import (
	"encoding/json"
	"fmt"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"gopkg.in/yaml.v3"
)

// Parser handles parsing of API configuration files
type Parser struct{}

// NewParser creates a new configuration parser
func NewParser() *Parser {
	return &Parser{}
}

// ParseYAML parses YAML content into an API configuration
func (p *Parser) ParseYAML(data []byte) (*api.APIConfiguration, error) {
	var config api.APIConfiguration

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &config, nil
}

// ParseJSON parses JSON content into an API configuration
func (p *Parser) ParseJSON(data []byte) (*api.APIConfiguration, error) {
	var config api.APIConfiguration

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &config, nil
}

// Parse attempts to parse data as either YAML or JSON
func (p *Parser) Parse(data []byte, contentType string) (*api.APIConfiguration, error) {
	switch contentType {
	case "application/yaml", "application/x-yaml", "text/yaml":
		return p.ParseYAML(data)
	case "application/json":
		return p.ParseJSON(data)
	default:
		// Try YAML first, then JSON
		config, err := p.ParseYAML(data)
		if err == nil {
			return config, nil
		}

		config, err = p.ParseJSON(data)
		if err == nil {
			return config, nil
		}

		return nil, fmt.Errorf("failed to parse as YAML or JSON")
	}
}
