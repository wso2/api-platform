package it

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/cucumber/godog"
)

const ConfigTagPrefix = "@config-"

// GatewayConfigManager handles configuration switching for the gateway
type GatewayConfigManager struct {
	registry       *ConfigProfileRegistry
	composeManager *ComposeManager
	currentProfile string
	mu             sync.Mutex
}

// NewGatewayConfigManager creates a new config manager
func NewGatewayConfigManager(cm *ComposeManager) *GatewayConfigManager {
	return &GatewayConfigManager{
		registry:       NewConfigProfileRegistry(),
		composeManager: cm,
		currentProfile: "default", // Assume default on startup
	}
}

// EnsureConfig checks the scenario tags and restarts the gateway if a different config is required
func (m *GatewayConfigManager) EnsureConfig(ctx context.Context, sc *godog.Scenario) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	requiredProfile := m.extractConfigTag(sc)
	if requiredProfile == "" {
		requiredProfile = "default"
	}

	if m.currentProfile == requiredProfile {
		return nil // No restart needed
	}

	log.Printf("Switching gateway config from '%s' to '%s'...", m.currentProfile, requiredProfile)

	profile, ok := m.registry.Get(requiredProfile)
	if !ok {
		return fmt.Errorf("unknown config profile: %s", requiredProfile)
	}

	// Restart gateway-controller with new env vars
	if err := m.composeManager.RestartGatewayController(ctx, profile.EnvVars); err != nil {
		return fmt.Errorf("failed to restart gateway with profile %s: %w", requiredProfile, err)
	}

	m.currentProfile = requiredProfile
	log.Printf("Switched to '%s' profile successfully", requiredProfile)
	return nil
}

// extractConfigTag finds the first tag starting with @config- and returns the suffix
func (m *GatewayConfigManager) extractConfigTag(sc *godog.Scenario) string {
	for _, tag := range sc.Tags {
		if strings.HasPrefix(tag.Name, ConfigTagPrefix) {
			return strings.TrimPrefix(tag.Name, ConfigTagPrefix)
		}
	}
	return ""
}
