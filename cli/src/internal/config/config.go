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
	"sort"
	"strings"

	"github.com/wso2/api-platform/cli/utils"
	"gopkg.in/yaml.v3"
)

const DefaultPlatform = "default"

type AuthConfig struct {
	Type     string `yaml:"type,omitempty"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
	Token    string `yaml:"token,omitempty"`
	APIKey   string `yaml:"apiKey,omitempty"`
}

// Gateway represents a gateway configuration.
type Gateway struct {
	Name   string     `yaml:"-"`
	Server string     `yaml:"server"`
	Auth   AuthConfig `yaml:"auth,omitempty"`
	AdminServer string `yaml:"adminServer,omitempty"`
}

// DevPortal represents a developer portal configuration.
type DevPortal struct {
	Name string     `yaml:"-"`
	URL  string     `yaml:"url"`
	Auth AuthConfig `yaml:"auth,omitempty"`
}

// Platform groups the CLI resources that belong to a single platform.
type Platform struct {
	Gateways        map[string]*Gateway   `yaml:"gateways,omitempty"`
	ActiveGateway   string                `yaml:"activeGateway,omitempty"`
	DevPortals      map[string]*DevPortal `yaml:"devportals,omitempty"`
	ActiveDevPortal string                `yaml:"activeDevPortal,omitempty"`
}

// Config represents the ap configuration file.
type Config struct {
	CurrentPlatform string               `yaml:"currentPlatform,omitempty"`
	Platforms       map[string]*Platform `yaml:"platforms,omitempty"`
}

func normalizePlatformName(platform string) string {
	platform = strings.TrimSpace(platform)
	if platform == "" {
		return DefaultPlatform
	}
	return platform
}

func normalizeGatewayAuth(gateway *Gateway) {
	if gateway == nil {
		return
	}
	if gateway.Auth.Type == "" {
		gateway.Auth.Type = utils.AuthTypeNone
	}
}

func normalizeDevPortalAuth(devPortal *DevPortal) {
	if devPortal == nil {
		return
	}
	if devPortal.Auth.Type == "" {
		devPortal.Auth.Type = utils.AuthTypeAPIKey
	}
}

func normalizePlatform(platform *Platform) {
	if platform == nil {
		return
	}
	if platform.Gateways == nil {
		platform.Gateways = map[string]*Gateway{}
	}
	for name, gateway := range platform.Gateways {
		if gateway == nil {
			gateway = &Gateway{}
			platform.Gateways[name] = gateway
		}
		gateway.Name = name
		normalizeGatewayAuth(gateway)
	}
	if platform.DevPortals == nil {
		platform.DevPortals = map[string]*DevPortal{}
	}
	for name, devPortal := range platform.DevPortals {
		if devPortal == nil {
			devPortal = &DevPortal{}
			platform.DevPortals[name] = devPortal
		}
		devPortal.Name = name
		normalizeDevPortalAuth(devPortal)
	}
}

func (c *Config) ensurePlatform(platform string) *Platform {
	if c.Platforms == nil {
		c.Platforms = map[string]*Platform{}
	}
	platform = normalizePlatformName(platform)
	if c.Platforms[platform] == nil {
		c.Platforms[platform] = &Platform{}
	}
	normalizePlatform(c.Platforms[platform])
	return c.Platforms[platform]
}

// AddPlatform creates the platform if it does not already exist.
func (c *Config) AddPlatform(platform string) string {
	platform = normalizePlatformName(platform)
	c.ensurePlatform(platform)
	if strings.TrimSpace(c.CurrentPlatform) == "" {
		c.CurrentPlatform = DefaultPlatform
	}
	return platform
}

// SetCurrentPlatform switches the active platform, creating it if necessary.
func (c *Config) SetCurrentPlatform(platform string) string {
	platform = c.AddPlatform(platform)
	c.CurrentPlatform = platform
	return platform
}

// GetCurrentPlatform returns the active platform name.
func (c *Config) GetCurrentPlatform() string {
	if strings.TrimSpace(c.CurrentPlatform) == "" {
		return DefaultPlatform
	}
	return normalizePlatformName(c.CurrentPlatform)
}

// ListPlatforms returns all configured platform names in sorted order.
func (c *Config) ListPlatforms() []string {
	if c.Platforms == nil {
		return []string{DefaultPlatform}
	}
	names := make([]string, 0, len(c.Platforms))
	for name := range c.Platforms {
		names = append(names, normalizePlatformName(name))
	}
	sort.Strings(names)
	return names
}

// ResolvePlatform resolves an optional platform flag against currentPlatform.
func (c *Config) ResolvePlatform(platform string) string {
	platform = strings.TrimSpace(platform)
	if platform != "" {
		return normalizePlatformName(platform)
	}
	if strings.TrimSpace(c.CurrentPlatform) != "" {
		return normalizePlatformName(c.CurrentPlatform)
	}
	return DefaultPlatform
}

// GetConfigPath returns the path to the configuration file.
func GetConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, utils.ConfigPath), nil
}

// LoadConfig loads the configuration from the config file.
func LoadConfig() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config := &Config{
			CurrentPlatform: DefaultPlatform,
			Platforms: map[string]*Platform{
				DefaultPlatform: {
					Gateways:   map[string]*Gateway{},
					DevPortals: map[string]*DevPortal{},
				},
			},
		}
		if err := SaveConfig(config); err != nil {
			return nil, err
		}
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if strings.TrimSpace(config.CurrentPlatform) == "" {
		config.CurrentPlatform = DefaultPlatform
	}
	if config.Platforms == nil {
		config.Platforms = map[string]*Platform{}
	}
	for name, platform := range config.Platforms {
		normalizePlatform(platform)
		config.Platforms[normalizePlatformName(name)] = platform
	}
	config.ensurePlatform(config.CurrentPlatform)

	return &config, nil
}

// SaveConfig saves the configuration to the config file.
func SaveConfig(config *Config) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if strings.TrimSpace(config.CurrentPlatform) == "" {
		config.CurrentPlatform = DefaultPlatform
	}
	config.ensurePlatform(config.CurrentPlatform)

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (c *Config) AddGatewayToPlatform(platformName string, gateway Gateway) error {
	platformName = c.ResolvePlatform(platformName)
	platform := c.ensurePlatform(platformName)
	gateway.Name = strings.TrimSpace(gateway.Name)
	normalizeGatewayAuth(&gateway)
	platform.Gateways[gateway.Name] = &gateway
	if platform.ActiveGateway == "" {
		platform.ActiveGateway = gateway.Name
	}
	if c.CurrentPlatform == "" {
		c.CurrentPlatform = platformName
	}
	return nil
}

func (c *Config) AddGateway(gateway Gateway) error {
	return c.AddGatewayToPlatform("", gateway)
}

func (c *Config) AddDevPortalToPlatform(platformName string, devPortal DevPortal) error {
	platformName = c.ResolvePlatform(platformName)
	platform := c.ensurePlatform(platformName)
	devPortal.Name = strings.TrimSpace(devPortal.Name)
	normalizeDevPortalAuth(&devPortal)
	platform.DevPortals[devPortal.Name] = &devPortal
	if platform.ActiveDevPortal == "" {
		platform.ActiveDevPortal = devPortal.Name
	}
	if c.CurrentPlatform == "" {
		c.CurrentPlatform = platformName
	}
	return nil
}

func (c *Config) AddDevPortal(devPortal DevPortal) error {
	return c.AddDevPortalToPlatform("", devPortal)
}

func (c *Config) GetGatewayFromPlatform(platformName, name string) (*Gateway, error) {
	platformName = c.ResolvePlatform(platformName)
	platform := c.ensurePlatform(platformName)
	gateway, ok := platform.Gateways[name]
	if !ok {
		return nil, fmt.Errorf("gateway '%s' not found in platform '%s'", name, platformName)
	}
	gateway.Name = name
	return gateway, nil
}

func (c *Config) GetGateway(name string) (*Gateway, error) {
	return c.GetGatewayFromPlatform("", name)
}

func (c *Config) GetActiveGatewayFromPlatform(platformName string) (*Gateway, error) {
	platformName = c.ResolvePlatform(platformName)
	platform := c.ensurePlatform(platformName)
	if platform.ActiveGateway == "" {
		return nil, fmt.Errorf("no active gateway set for platform '%s'", platformName)
	}
	return c.GetGatewayFromPlatform(platformName, platform.ActiveGateway)
}

func (c *Config) GetActiveGateway() (*Gateway, error) {
	return c.GetActiveGatewayFromPlatform("")
}

func (c *Config) SetActiveGatewayForPlatform(platformName, name string) error {
	platformName = c.ResolvePlatform(platformName)
	if _, err := c.GetGatewayFromPlatform(platformName, name); err != nil {
		return err
	}
	platform := c.ensurePlatform(platformName)
	platform.ActiveGateway = name
	c.CurrentPlatform = platformName
	return nil
}

func (c *Config) SetActiveGateway(name string) error {
	return c.SetActiveGatewayForPlatform("", name)
}

func (c *Config) RemoveGatewayFromPlatform(platformName, name string) error {
	platformName = c.ResolvePlatform(platformName)
	platform := c.ensurePlatform(platformName)
	if _, ok := platform.Gateways[name]; !ok {
		return fmt.Errorf("gateway '%s' not found in platform '%s'", name, platformName)
	}
	delete(platform.Gateways, name)
	if platform.ActiveGateway == name {
		platform.ActiveGateway = ""
	}
	return nil
}

func (c *Config) RemoveGateway(name string) error {
	return c.RemoveGatewayFromPlatform("", name)
}

func (c *Config) GetDevPortalFromPlatform(platformName, name string) (*DevPortal, error) {
	platformName = c.ResolvePlatform(platformName)
	platform := c.ensurePlatform(platformName)
	devPortal, ok := platform.DevPortals[name]
	if !ok {
		return nil, fmt.Errorf("devportal '%s' not found in platform '%s'", name, platformName)
	}
	devPortal.Name = name
	return devPortal, nil
}

func (c *Config) GetDevPortal(name string) (*DevPortal, error) {
	return c.GetDevPortalFromPlatform("", name)
}

func (c *Config) GetActiveDevPortalFromPlatform(platformName string) (*DevPortal, error) {
	platformName = c.ResolvePlatform(platformName)
	platform := c.ensurePlatform(platformName)
	if platform.ActiveDevPortal == "" {
		return nil, fmt.Errorf("no active devportal set for platform '%s'", platformName)
	}
	return c.GetDevPortalFromPlatform(platformName, platform.ActiveDevPortal)
}

func (c *Config) GetActiveDevPortal() (*DevPortal, error) {
	return c.GetActiveDevPortalFromPlatform("")
}

func (c *Config) SetActiveDevPortalForPlatform(platformName, name string) error {
	platformName = c.ResolvePlatform(platformName)
	if _, err := c.GetDevPortalFromPlatform(platformName, name); err != nil {
		return err
	}
	platform := c.ensurePlatform(platformName)
	platform.ActiveDevPortal = name
	c.CurrentPlatform = platformName
	return nil
}

func (c *Config) SetActiveDevPortal(name string) error {
	return c.SetActiveDevPortalForPlatform("", name)
}

func (c *Config) RemoveDevPortalFromPlatform(platformName, name string) error {
	platformName = c.ResolvePlatform(platformName)
	platform := c.ensurePlatform(platformName)
	if _, ok := platform.DevPortals[name]; !ok {
		return fmt.Errorf("devportal '%s' not found in platform '%s'", name, platformName)
	}
	delete(platform.DevPortals, name)
	if platform.ActiveDevPortal == name {
		platform.ActiveDevPortal = ""
	}
	return nil
}

func (c *Config) RemoveDevPortal(name string) error {
	return c.RemoveDevPortalFromPlatform("", name)
}
