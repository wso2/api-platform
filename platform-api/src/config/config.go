/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
	"sync"

	"github.com/kelseyhightower/envconfig"
)

// Server holds the configuration parameters for the application.
type Server struct {
	LogLevel string `envconfig:"LOG_LEVEL" default:"DEBUG"`
	//Logger   logging.Logger

	// Server configurations
	Port string `envconfig:"PORT" default:"9243"`

	// Database configurations
	Database     Database `envconfig:"DATABASE"`
	DBSchemaPath string   `envconfig:"DB_SCHEMA_PATH" default:"./internal/database/schema.sql"`

	// LLM provider template bootstrap (used to seed defaults into the DB)
	LLMTemplateDefinitionsPath string `envconfig:"LLM_TEMPLATE_DEFINITIONS_PATH" default:"./resources/default-llm-provider-templates"`

	// JWT Authentication configurations
	JWT JWT `envconfig:"JWT"`

	// WebSocket configurations
	WebSocket WebSocket `envconfig:"WEBSOCKET"`

	// Default DevPortal configurations
	DefaultDevPortal DefaultDevPortal `envconfig:"DEFAULT_DEVPORTAL"`

	// Deployment configurations
	Deployments Deployments `envconfig:"DEPLOYMENTS"`
	// TLS configurations
	TLS TLS `envconfig:"TLS"`
}

// TLS holds TLS certificate configuration
type TLS struct {
	CertDir string `envconfig:"CERT_DIR" default:"./data/certs"`
}

// JWT holds JWT-specific configuration
type JWT struct {
	SecretKey      string   `envconfig:"SECRET_KEY" default:"your-secret-key-change-in-production"`
	Issuer         string   `envconfig:"ISSUER" default:"thunder"`
	SkipPaths      []string `envconfig:"SKIP_PATHS" default:"/health,/metrics,/api/internal/v1/ws/gateways/connect,/api/internal/v1/apis"`
	SkipValidation bool     `envconfig:"SKIP_VALIDATION" default:"true"` // Skip signature validation for development
}

// WebSocket holds WebSocket-specific configuration
type WebSocket struct {
	MaxConnections    int `envconfig:"WS_MAX_CONNECTIONS" default:"1000"`
	ConnectionTimeout int `envconfig:"WS_CONNECTION_TIMEOUT" default:"30"` // seconds
	RateLimitPerMin   int `envconfig:"WS_RATE_LIMIT_PER_MINUTE" default:"10"`
}

// Database holds database-specific configuration
type Database struct {
	Driver string `envconfig:"DRIVER" default:"sqlite3"`
	// DBPath is the file path for SQLite databases.
	// Use DATABASE_DB_PATH to override; keeping it distinct from the OS PATH variable.
	Path            string `envconfig:"DB_PATH" default:"./data/api_platform.db"`
	Host            string `envconfig:"HOST" default:"localhost"`
	Port            int    `envconfig:"PORT" default:"5432"`
	Name            string `envconfig:"NAME" default:"platform_api"`
	User            string `envconfig:"USER" default:""`
	Password        string `envconfig:"PASSWORD" default:""`
	SSLMode         string `envconfig:"SSL_MODE" default:"disable"`
	MaxOpenConns    int    `envconfig:"MAX_OPEN_CONNS" default:"25"`
	MaxIdleConns    int    `envconfig:"MAX_IDLE_CONNS" default:"10"`
	ConnMaxLifetime int    `envconfig:"CONN_MAX_LIFETIME" default:"300"` // seconds

	// ExecuteSchemaDDL controls whether to run the schema DDL (CREATE TABLE, etc.) on startup.
	// Set to false when the DB user lacks DDL privileges (e.g. deployed Postgres with restricted role).
	// Env: DATABASE_EXECUTE_SCHEMA_DDL (default: true)
	ExecuteSchemaDDL bool `envconfig:"EXECUTE_SCHEMA_DDL" default:"true"`
}

// DefaultDevPortal holds default DevPortal configuration for new organizations
type DefaultDevPortal struct {
	Enabled       bool   `envconfig:"ENABLED" default:"true"`
	Name          string `envconfig:"NAME" default:"Default DevPortal"`
	Identifier    string `envconfig:"IDENTIFIER" default:"default"`
	APIUrl        string `envconfig:"API_URL" default:"http://localhost:3001"`
	Hostname      string `envconfig:"HOSTNAME" default:"devportal.local"`
	APIKey        string `envconfig:"API_KEY" default:"default-api-key"`
	HeaderKeyName string `envconfig:"HEADER_KEY_NAME" default:"x-wso2-api-key"`
	Timeout       int    `envconfig:"TIMEOUT" default:"10"` // seconds

	// Role mapping configuration for DevPortal integrations
	RoleClaimName         string `envconfig:"ROLE_CLAIM_NAME" default:"roles"`
	GroupsClaimName       string `envconfig:"GROUPS_CLAIM_NAME" default:"groups"`
	OrganizationClaimName string `envconfig:"ORGANIZATION_CLAIM_NAME" default:"organizationID"`
	AdminRole             string `envconfig:"ADMIN_ROLE" default:"admin"`
	SubscriberRole        string `envconfig:"SUBSCRIBER_ROLE" default:"Internal/subscriber"`
	SuperAdminRole        string `envconfig:"SUPER_ADMIN_ROLE" default:"superAdmin"`
}

// Deployments holds deployment-specific configuration
type Deployments struct {
	MaxPerAPIGateway int `envconfig:"MAX_PER_API_GATEWAY" default:"20"`
}

// package-level variable and mutex for thread safety
var (
	processOnce     sync.Once
	settingInstance *Server
)

// GetConfig initializes and returns a singleton instance of the Settings struct.
// It uses sync.Once to ensure that the initialization logic is executed only once,
// making it safe for concurrent use. If there is an error during the initialization,
// the function will panic.
//
// Returns:
//
//	*Settings - A pointer to the singleton instance of the Settings struct. from environment variables.
func GetConfig() *Server {
	var err error
	processOnce.Do(func() {
		settingInstance = &Server{}
		err = envconfig.Process("", settingInstance)
		if err == nil {
			// Validate default devportal configuration
			err = validateDefaultDevPortalConfig(&settingInstance.DefaultDevPortal)
		}
	})
	if err != nil {
		panic(err)
	}
	return settingInstance
}

// validateDefaultDevPortalConfig validates default DevPortal configuration
//
// When default DevPortal is enabled, this function ensures that required
// fields are provided.
//
// Parameters:
//   - cfg: default DevPortal configuration to validate
//
// Returns:
//   - error: Validation error if configuration is invalid, nil otherwise
func validateDefaultDevPortalConfig(cfg *DefaultDevPortal) error {
	// If default DevPortal is not enabled, no validation needed
	if !cfg.Enabled {
		return nil
	}

	// When enabled, required fields must be provided
	if cfg.Name == "" {
		return fmt.Errorf("default DevPortal is enabled but DEFAULT_DEVPORTAL_NAME is not configured")
	}

	if cfg.Identifier == "" {
		return fmt.Errorf("default DevPortal is enabled but DEFAULT_DEVPORTAL_IDENTIFIER is not configured")
	}

	if cfg.APIUrl == "" {
		return fmt.Errorf("default DevPortal is enabled but DEFAULT_DEVPORTAL_API_URL is not configured")
	}

	if cfg.Hostname == "" {
		return fmt.Errorf("default DevPortal is enabled but DEFAULT_DEVPORTAL_HOSTNAME is not configured")
	}

	if cfg.APIKey == "" {
		return fmt.Errorf("default DevPortal is enabled but DEFAULT_DEVPORTAL_API_KEY is not configured")
	}

	// Header key name is always required since we use header mode
	if cfg.HeaderKeyName == "" {
		return fmt.Errorf("default DevPortal header key name is not configured")
	}

	return nil
}
