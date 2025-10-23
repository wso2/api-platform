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
	"sync"

	"github.com/kelseyhightower/envconfig"
)

// Server holds the configuration parameters for the application.
type Server struct {
	LogLevel string `envconfig:"LOG_LEVEL" default:"DEBUG"`
	//Logger   logging.Logger

	// Database configurations
	Database     Database `envconfig:"DATABASE"`
	DBSchemaPath string   `envconfig:"DB_SCHEMA_PATH" default:"./internal/database/schema.sql"`

	// JWT Authentication configurations
	JWT JWT `envconfig:"JWT"`

	// WebSocket configurations
	WebSocket WebSocket `envconfig:"WEBSOCKET"`
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
	Driver   string `envconfig:"DRIVER" default:"sqlite3"`
	Path     string `envconfig:"PATH" default:"./data/api_platform.db"`
	Host     string `envconfig:"HOST" default:"localhost"`
	Port     int    `envconfig:"PORT" default:"5432"`
	Name     string `envconfig:"NAME" default:"platform_api"`
	User     string `envconfig:"USER" default:""`
	Password string `envconfig:"PASSWORD" default:""`
	SSLMode  string `envconfig:"SSL_MODE" default:"disable"`

	// SQLite specific settings
	MaxOpenConns    int `envconfig:"MAX_OPEN_CONNS" default:"25"`
	MaxIdleConns    int `envconfig:"MAX_IDLE_CONNS" default:"10"`
	ConnMaxLifetime int `envconfig:"CONN_MAX_LIFETIME" default:"300"` // seconds
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
	})
	if err != nil {
		panic(err)
	}
	settingInstance.Database.Driver = "sqlite3"
	settingInstance.Database.Path = "./data/api_platform.db"
	return settingInstance
}
