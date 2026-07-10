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
	Name        string     `yaml:"-"`
	Server      string     `yaml:"server"`
	Auth        AuthConfig `yaml:"auth,omitempty"`
	AdminServer string     `yaml:"adminServer,omitempty"`
}

// DevPortal represents a developer portal configuration.
type DevPortal struct {
	Name string     `yaml:"-"`
	URL  string     `yaml:"url"`
	Auth AuthConfig `yaml:"auth,omitempty"`
}

// AIWorkspace represents an AI workspace configuration.
type AIWorkspace struct {
	Name string     `yaml:"-"`
	URL  string     `yaml:"url"`
	Auth AuthConfig `yaml:"auth,omitempty"`
}

// Platform groups the CLI resources that belong to a single platform.
type Platform struct {
	Gateways          map[string]*Gateway     `yaml:"gateways,omitempty"`
	ActiveGateway     string                  `yaml:"activeGateway,omitempty"`
	DevPortals        map[string]*DevPortal   `yaml:"devportals,omitempty"`
	ActiveDevPortal   string                  `yaml:"activeDevPortal,omitempty"`
	AIWorkspaces      map[string]*AIWorkspace `yaml:"aiWorkspaces,omitempty"`
	ActiveAIWorkspace string                  `yaml:"activeAIWorkspace,omitempty"`
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

func normalizeAIWorkspaceAuth(aiWorkspace *AIWorkspace) {
	if aiWorkspace == nil {
		return
	}
	if aiWorkspace.Auth.Type == "" {
		aiWorkspace.Auth.Type = utils.AuthTypeAPIKey
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
	if platform.AIWorkspaces == nil {
		platform.AIWorkspaces = map[string]*AIWorkspace{}
	}
	for name, aiWorkspace := range platform.AIWorkspaces {
		if aiWorkspace == nil {
			aiWorkspace = &AIWorkspace{}
			platform.AIWorkspaces[name] = aiWorkspace
		}
		aiWorkspace.Name = name
		normalizeAIWorkspaceAuth(aiWorkspace)
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

// resourceSection describes one named-resource map on a Platform plus its
// companion "active" pointer. A single value of this type captures everything
// that varies between gateways, devportals and ai-workspaces (which map to
// read, which active field to track, the error label, and how to read/write a
// resource's name and per-kind defaults), so the generic CRUD helpers below are
// written once and reused across all three resource kinds.
//
// The accessors exist because Go generics abstract over behaviour, not struct
// fields: a type parameter T cannot reach `.Name`, `p.DevPortals`, etc.
// directly, so the section supplies small closures that do.
type resourceSection[T any] struct {
	label     string                        // resource name used in error messages
	items     func(*Platform) map[string]*T // the per-kind map on a Platform
	active    func(*Platform) string        // reads the "active" pointer
	setActive func(*Platform, string)       // writes the "active" pointer
	getName   func(*T) string               // reads a resource's name
	setName   func(*T, string)              // writes a resource's name
	normalize func(*T)                      // applies per-kind defaults (e.g. auth type)
}

// add stores resource under its trimmed name, making it active if no active
// resource is set yet.
func (s resourceSection[T]) add(c *Config, platformName string, resource *T) error {
	platformName = c.ResolvePlatform(platformName)
	platform := c.ensurePlatform(platformName)
	name := strings.TrimSpace(s.getName(resource))
	s.setName(resource, name)
	s.normalize(resource)
	s.items(platform)[name] = resource
	if s.active(platform) == "" {
		s.setActive(platform, name)
	}
	if c.CurrentPlatform == "" {
		c.CurrentPlatform = platformName
	}
	return nil
}

// get looks up a resource by name, stamping the name back onto the returned
// value so callers always see it populated.
func (s resourceSection[T]) get(c *Config, platformName, name string) (*T, error) {
	platformName = c.ResolvePlatform(platformName)
	platform := c.ensurePlatform(platformName)
	resource, ok := s.items(platform)[name]
	if !ok {
		return nil, fmt.Errorf("%s '%s' not found in platform '%s'", s.label, name, platformName)
	}
	s.setName(resource, name)
	return resource, nil
}

// getActive returns the platform's currently active resource of this kind.
func (s resourceSection[T]) getActive(c *Config, platformName string) (*T, error) {
	platformName = c.ResolvePlatform(platformName)
	platform := c.ensurePlatform(platformName)
	activeName := s.active(platform)
	if activeName == "" {
		return nil, fmt.Errorf("no active %s set for platform '%s'", s.label, platformName)
	}
	return s.get(c, platformName, activeName)
}

// setActiveByName marks an existing resource as active (failing if it does not
// exist) and switches the current platform to its owner.
func (s resourceSection[T]) setActiveByName(c *Config, platformName, name string) error {
	platformName = c.ResolvePlatform(platformName)
	if _, err := s.get(c, platformName, name); err != nil {
		return err
	}
	platform := c.ensurePlatform(platformName)
	s.setActive(platform, name)
	c.CurrentPlatform = platformName
	return nil
}

// remove deletes a resource, clearing the active pointer when it referenced the
// removed resource.
func (s resourceSection[T]) remove(c *Config, platformName, name string) error {
	platformName = c.ResolvePlatform(platformName)
	platform := c.ensurePlatform(platformName)
	if _, ok := s.items(platform)[name]; !ok {
		return fmt.Errorf("%s '%s' not found in platform '%s'", s.label, name, platformName)
	}
	delete(s.items(platform), name)
	if s.active(platform) == name {
		s.setActive(platform, "")
	}
	return nil
}

// Per-kind section descriptors. These are the only place the concrete fields of
// each resource are wired into the generic helpers.
var (
	gatewaySection = resourceSection[Gateway]{
		label:     "gateway",
		items:     func(p *Platform) map[string]*Gateway { return p.Gateways },
		active:    func(p *Platform) string { return p.ActiveGateway },
		setActive: func(p *Platform, n string) { p.ActiveGateway = n },
		getName:   func(g *Gateway) string { return g.Name },
		setName:   func(g *Gateway, n string) { g.Name = n },
		normalize: normalizeGatewayAuth,
	}

	devPortalSection = resourceSection[DevPortal]{
		label:     "devportal",
		items:     func(p *Platform) map[string]*DevPortal { return p.DevPortals },
		active:    func(p *Platform) string { return p.ActiveDevPortal },
		setActive: func(p *Platform, n string) { p.ActiveDevPortal = n },
		getName:   func(d *DevPortal) string { return d.Name },
		setName:   func(d *DevPortal, n string) { d.Name = n },
		normalize: normalizeDevPortalAuth,
	}

	aiWorkspaceSection = resourceSection[AIWorkspace]{
		label:     "ai-workspace",
		items:     func(p *Platform) map[string]*AIWorkspace { return p.AIWorkspaces },
		active:    func(p *Platform) string { return p.ActiveAIWorkspace },
		setActive: func(p *Platform, n string) { p.ActiveAIWorkspace = n },
		getName:   func(w *AIWorkspace) string { return w.Name },
		setName:   func(w *AIWorkspace, n string) { w.Name = n },
		normalize: normalizeAIWorkspaceAuth,
	}
)

// --- Gateway public API (thin, type-safe wrappers over gatewaySection) ---

func (c *Config) AddGatewayToPlatform(platformName string, gateway Gateway) error {
	return gatewaySection.add(c, platformName, &gateway)
}

func (c *Config) AddGateway(gateway Gateway) error {
	return c.AddGatewayToPlatform("", gateway)
}

func (c *Config) GetGatewayFromPlatform(platformName, name string) (*Gateway, error) {
	return gatewaySection.get(c, platformName, name)
}

func (c *Config) GetGateway(name string) (*Gateway, error) {
	return c.GetGatewayFromPlatform("", name)
}

func (c *Config) GetActiveGatewayFromPlatform(platformName string) (*Gateway, error) {
	return gatewaySection.getActive(c, platformName)
}

func (c *Config) GetActiveGateway() (*Gateway, error) {
	return c.GetActiveGatewayFromPlatform("")
}

func (c *Config) SetActiveGatewayForPlatform(platformName, name string) error {
	return gatewaySection.setActiveByName(c, platformName, name)
}

func (c *Config) SetActiveGateway(name string) error {
	return c.SetActiveGatewayForPlatform("", name)
}

func (c *Config) RemoveGatewayFromPlatform(platformName, name string) error {
	return gatewaySection.remove(c, platformName, name)
}

func (c *Config) RemoveGateway(name string) error {
	return c.RemoveGatewayFromPlatform("", name)
}

// --- DevPortal public API (thin, type-safe wrappers over devPortalSection) ---

func (c *Config) AddDevPortalToPlatform(platformName string, devPortal DevPortal) error {
	return devPortalSection.add(c, platformName, &devPortal)
}

func (c *Config) AddDevPortal(devPortal DevPortal) error {
	return c.AddDevPortalToPlatform("", devPortal)
}

func (c *Config) GetDevPortalFromPlatform(platformName, name string) (*DevPortal, error) {
	return devPortalSection.get(c, platformName, name)
}

func (c *Config) GetDevPortal(name string) (*DevPortal, error) {
	return c.GetDevPortalFromPlatform("", name)
}

func (c *Config) GetActiveDevPortalFromPlatform(platformName string) (*DevPortal, error) {
	return devPortalSection.getActive(c, platformName)
}

func (c *Config) GetActiveDevPortal() (*DevPortal, error) {
	return c.GetActiveDevPortalFromPlatform("")
}

func (c *Config) SetActiveDevPortalForPlatform(platformName, name string) error {
	return devPortalSection.setActiveByName(c, platformName, name)
}

func (c *Config) SetActiveDevPortal(name string) error {
	return c.SetActiveDevPortalForPlatform("", name)
}

func (c *Config) RemoveDevPortalFromPlatform(platformName, name string) error {
	return devPortalSection.remove(c, platformName, name)
}

func (c *Config) RemoveDevPortal(name string) error {
	return c.RemoveDevPortalFromPlatform("", name)
}

// --- AIWorkspace public API (thin, type-safe wrappers over aiWorkspaceSection) ---

func (c *Config) AddAIWorkspaceToPlatform(platformName string, aiWorkspace AIWorkspace) error {
	return aiWorkspaceSection.add(c, platformName, &aiWorkspace)
}

func (c *Config) AddAIWorkspace(aiWorkspace AIWorkspace) error {
	return c.AddAIWorkspaceToPlatform("", aiWorkspace)
}

func (c *Config) GetAIWorkspaceFromPlatform(platformName, name string) (*AIWorkspace, error) {
	return aiWorkspaceSection.get(c, platformName, name)
}

func (c *Config) GetAIWorkspace(name string) (*AIWorkspace, error) {
	return c.GetAIWorkspaceFromPlatform("", name)
}

func (c *Config) GetActiveAIWorkspaceFromPlatform(platformName string) (*AIWorkspace, error) {
	return aiWorkspaceSection.getActive(c, platformName)
}

func (c *Config) GetActiveAIWorkspace() (*AIWorkspace, error) {
	return c.GetActiveAIWorkspaceFromPlatform("")
}

func (c *Config) SetActiveAIWorkspaceForPlatform(platformName, name string) error {
	return aiWorkspaceSection.setActiveByName(c, platformName, name)
}

func (c *Config) SetActiveAIWorkspace(name string) error {
	return c.SetActiveAIWorkspaceForPlatform("", name)
}

func (c *Config) RemoveAIWorkspaceFromPlatform(platformName, name string) error {
	return aiWorkspaceSection.remove(c, platformName, name)
}

func (c *Config) RemoveAIWorkspace(name string) error {
	return c.RemoveAIWorkspaceFromPlatform("", name)
}

// RemovePlatform deletes a platform and all of its connections from the config.
// If the removed platform was the current one, the current selection is reset so
// it falls back to the default platform.
func (c *Config) RemovePlatform(platformName string) error {
	name := normalizePlatformName(platformName)
	if c.Platforms == nil || c.Platforms[name] == nil {
		return fmt.Errorf("platform '%s' not found", name)
	}
	delete(c.Platforms, name)
	if normalizePlatformName(c.CurrentPlatform) == name {
		c.CurrentPlatform = ""
	}
	return nil
}
