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

// Package config holds event-gateway-controller-specific configuration,
// moved out of gateway/gateway-controller/pkg/config/config.go. This binary
// loads core config via gateway-controller's own config.Load(...) and then
// additionally loads this EventGatewayConfig section from the same
// config.toml file's "event_gateway" key.
package config

import (
	"fmt"
	"net/url"
	"strings"

	toml "github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// EventGatewayConfig holds event gateway specific configurations.
type EventGatewayConfig struct {
	Enabled               bool   `koanf:"enabled"`
	WebSubHubURL          string `koanf:"websub_hub_url"`
	WebSubHubPort         int    `koanf:"websub_hub_port"`
	RouterHost            string `koanf:"router_host"`
	WebSubHubListenerPort int    `koanf:"websub_hub_listener_port"`
	TimeoutSeconds        int    `koanf:"timeout_seconds"`
}

// DefaultEventGatewayConfig returns the default configuration (matches the
// defaults core used to apply under Router.EventGateway before the split).
func DefaultEventGatewayConfig() EventGatewayConfig {
	return EventGatewayConfig{
		Enabled:               false,
		WebSubHubURL:          "http://host.docker.internal",
		WebSubHubPort:         9098,
		RouterHost:            "localhost",
		WebSubHubListenerPort: 8083,
		TimeoutSeconds:        30,
	}
}

// Load reads the "router.event_gateway" section from the same config.toml
// file this binary's main.go loads via gateway-controller (core)'s
// config.LoadConfig(...), applying the same defaults core used to apply.
func Load(configPath string) (EventGatewayConfig, error) {
	cfg := DefaultEventGatewayConfig()

	k := koanf.New(".")
	if err := k.Load(file.Provider(configPath), toml.Parser()); err != nil {
		return cfg, fmt.Errorf("failed to load config file: %w", err)
	}
	if err := k.Unmarshal("router.event_gateway", &cfg); err != nil {
		return cfg, fmt.Errorf("failed to unmarshal router.event_gateway: %w", err)
	}
	return cfg, nil
}

// Validate validates the event gateway configuration. Only called when Enabled.
func (c *EventGatewayConfig) Validate() error {
	if c.WebSubHubPort < 1 || c.WebSubHubPort > 65535 {
		return fmt.Errorf("router.event_gateway.websub_hub_port must be between 1 and 65535, got: %d", c.WebSubHubPort)
	}
	if c.WebSubHubListenerPort < 1 || c.WebSubHubListenerPort > 65535 {
		return fmt.Errorf("router.event_gateway.websub_hub_listener_port must be between 1 and 65535, got: %d", c.WebSubHubListenerPort)
	}

	// Validate WebSubHubURL if provided - must be a valid http(s) URL
	if strings.TrimSpace(c.WebSubHubURL) != "" {
		u, err := url.Parse(c.WebSubHubURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("router.event_gateway.websub_hub_url must be a valid URL with http or https scheme, got: %s", c.WebSubHubURL)
		}
		if u.Host == "" {
			return fmt.Errorf("router.event_gateway.websub_hub_url must include a valid host, got: %s", c.WebSubHubURL)
		}
	}
	if c.TimeoutSeconds <= 0 {
		return fmt.Errorf("router.event_gateway.timeout_seconds must be positive, got: %d", c.TimeoutSeconds)
	}
	return nil
}
