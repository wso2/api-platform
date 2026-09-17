/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudconsole

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/knadh/koanf/parsers/toml/v2"
)

const (
	// EnvironmentParameter is the external resolver parameter selected by -cloud-env.
	EnvironmentParameter = "environment"

	// EnvironmentConfigPath is the repository-relative environment mapping.
	EnvironmentConfigPath = "tests/framework/core/catalog/cloudconsole/environments.toml"
)

var environmentLabelPattern = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

// EnvironmentConfig contains the configured APIP cloud environment domains.
type EnvironmentConfig struct {
	Environments map[string]EnvironmentEntry
}

// EnvironmentEntry contains non-secret endpoint data for one cloud environment.
type EnvironmentEntry struct {
	CloudBaseDomain string
}

// LoadEnvironmentConfig loads and validates an environment mapping from TOML.
func LoadEnvironmentConfig(path string) (EnvironmentConfig, error) {
	if strings.TrimSpace(path) == "" {
		return EnvironmentConfig{}, fmt.Errorf("cloud-console: environment config path is required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return EnvironmentConfig{}, fmt.Errorf("cloud-console: reading environment config %q: %w", path, err)
	}
	tree, err := toml.Parser().Unmarshal(raw)
	if err != nil {
		return EnvironmentConfig{}, fmt.Errorf("cloud-console: parsing environment config %q: %w", path, err)
	}
	value, ok := tree["environments"]
	if !ok {
		return EnvironmentConfig{}, fmt.Errorf("cloud-console: environment config %q has no [environments] table", path)
	}
	environments, ok := value.(map[string]any)
	if !ok || len(environments) == 0 {
		return EnvironmentConfig{}, fmt.Errorf("cloud-console: environment config %q has no environments", path)
	}

	config := EnvironmentConfig{Environments: make(map[string]EnvironmentEntry, len(environments))}
	for name, rawEntry := range environments {
		if err := validateEnvironmentName(name); err != nil {
			return EnvironmentConfig{}, fmt.Errorf("cloud-console: environment config %q: %w", path, err)
		}
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			return EnvironmentConfig{}, fmt.Errorf("cloud-console: environment %q in %q is not a table", name, path)
		}
		domain, ok := entry["cloud_base_domain"].(string)
		if !ok || strings.TrimSpace(domain) == "" {
			return EnvironmentConfig{}, fmt.Errorf("cloud-console: environment %q in %q has no cloud_base_domain", name, path)
		}
		if err := validateDomain(domain); err != nil {
			return EnvironmentConfig{}, fmt.Errorf("cloud-console: environment %q in %q: %w", name, path, err)
		}
		config.Environments[name] = EnvironmentEntry{CloudBaseDomain: strings.TrimSpace(domain)}
	}
	return config, nil
}

// Domain returns the configured cloud base domain for an environment.
func (c EnvironmentConfig) Domain(environment string) (string, error) {
	environment = strings.TrimSpace(environment)
	if err := validateEnvironmentName(environment); err != nil {
		return "", fmt.Errorf("cloud-console: %w", err)
	}
	entry, ok := c.Environments[environment]
	if !ok {
		return "", fmt.Errorf("cloud-console: environment %q is not configured", environment)
	}
	return entry.CloudBaseDomain, nil
}

func validateEnvironmentName(environment string) error {
	if environment == "" {
		return fmt.Errorf("environment is required")
	}
	if !environmentLabelPattern.MatchString(environment) {
		return fmt.Errorf("environment %q must be a single valid DNS label", environment)
	}
	return nil
}

func validateDomain(domain string) error {
	domain = strings.TrimSpace(domain)
	parsed, err := url.Parse("https://" + domain)
	if err != nil || parsed.Host != domain || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Port() != "" {
		return fmt.Errorf("cloud_base_domain %q must be a host name without scheme or path", domain)
	}
	if parsed.User != nil || strings.Contains(domain, "@") {
		return fmt.Errorf("cloud_base_domain %q must not contain credentials", domain)
	}
	return nil
}

func resolveEndpoints(repoRoot string, parameters map[string]string) (map[string]string, error) {
	if strings.TrimSpace(repoRoot) == "" {
		return nil, fmt.Errorf("repository root is required")
	}
	environment := strings.TrimSpace(parameters[EnvironmentParameter])
	if environment == "" {
		return nil, fmt.Errorf("parameter %q is required", EnvironmentParameter)
	}
	if err := validateEnvironmentName(environment); err != nil {
		return nil, fmt.Errorf("parameter %q: %w", EnvironmentParameter, err)
	}
	config, err := LoadEnvironmentConfig(filepath.Join(repoRoot, EnvironmentConfigPath))
	if err != nil {
		return nil, err
	}
	domain, err := config.Domain(environment)
	if err != nil {
		return nil, err
	}
	endpoints := map[string]string{
		EndpointAPIPBML: "https://" + environment + "-wso2cloud.gateway." + domain + "/apip-bml-apip-bml-endpoint/api/v0.9",
	}
	for name, endpoint := range endpoints {
		parsed, parseErr := url.Parse(endpoint)
		if parseErr != nil || parsed.Scheme == "" || parsed.Host == "" {
			return nil, fmt.Errorf("resolved endpoint %q is not an absolute URL", name)
		}
	}
	return endpoints, nil
}
