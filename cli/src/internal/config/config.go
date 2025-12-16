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
	"fmt"
	"os"
	"path/filepath"

	"github.com/wso2/api-platform/cli/utils"
	"gopkg.in/yaml.v3"
)

// Gateway represents a gateway configuration
type Gateway struct {
	Name     string `yaml:"name"`
	Server   string `yaml:"server"`
	Token    string `yaml:"token,omitempty"`
	Insecure bool   `yaml:"insecure,omitempty"`
}

// Config represents the apipctl configuration
type Config struct {
	Gateways      []Gateway `yaml:"gateways,omitempty"`
	ActiveGateway string    `yaml:"activeGateway,omitempty"`
	ConfigVersion string    `yaml:"configVersion"`
}

const (
	ConfigDirName  = ".apipctl"
	ConfigFileName = "config.yaml"
	ConfigVersion  = "1.0.0"
)

// GetConfigPath returns the path to the configuration file
func GetConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, ConfigDirName, ConfigFileName), nil
}

// LoadConfig loads the configuration from the config file
func LoadConfig() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	// Create config directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create a new default config
		config := &Config{
			ConfigVersion: ConfigVersion,
			Gateways:      []Gateway{},
		}
		if err := SaveConfig(config); err != nil {
			return nil, err
		}
		return config, nil
	}

	// Read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// SaveConfig saves the configuration to the config file
func SaveConfig(config *Config) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	// Ensure config directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// AddGateway adds a new gateway to the configuration
func (c *Config) AddGateway(gateway Gateway) error {
	// Encrypt token if present
	if gateway.Token != "" {
		encryptedToken, err := utils.EncryptToken(gateway.Token)
		if err != nil {
			return fmt.Errorf("failed to encrypt token: %w", err)
		}
		gateway.Token = encryptedToken
	}

	// Check if gateway with the same name already exists
	for i, g := range c.Gateways {
		if g.Name == gateway.Name {
			// Update existing gateway
			c.Gateways[i] = gateway
			return nil
		}
	}

	// Add new gateway
	c.Gateways = append(c.Gateways, gateway)

	// Set as active if it's the first gateway
	if len(c.Gateways) == 1 {
		c.ActiveGateway = gateway.Name
	}

	return nil
}

// GetGateway returns a gateway by name with decrypted token
func (c *Config) GetGateway(name string) (*Gateway, error) {
	for i := range c.Gateways {
		if c.Gateways[i].Name == name {
			// Create a copy to avoid modifying the config
			gateway := c.Gateways[i]
			// Decrypt token if present
			if gateway.Token != "" {
				decryptedToken, err := utils.DecryptToken(gateway.Token)
				if err != nil {
					return nil, fmt.Errorf("failed to decrypt token for gateway '%s': %w", name, err)
				}
				gateway.Token = decryptedToken
			}
			return &gateway, nil
		}
	}
	return nil, fmt.Errorf("gateway '%s' not found", name)
}

// GetActiveGateway returns the active gateway
func (c *Config) GetActiveGateway() (*Gateway, error) {
	if c.ActiveGateway == "" {
		return nil, fmt.Errorf("no active gateway set")
	}
	return c.GetGateway(c.ActiveGateway)
}

// SetActiveGateway sets the active gateway
func (c *Config) SetActiveGateway(name string) error {
	if _, err := c.GetGateway(name); err != nil {
		return err
	}
	c.ActiveGateway = name
	return nil
}
