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

package main

import (
	"flag"
	"os"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/logger"
	"github.com/wso2/api-platform/platform-api/internal/server"
)

func main() {
	configFile := flag.String("config", "", "path to config.toml file (optional; env vars take priority over the file)")
	flag.Parse()

	if *configFile != "" {
		config.SetConfigPath(*configFile)
	}

	cfg := config.GetConfig()

	// Initialize logger
	logConfig := logger.Config{
		Level:  cfg.LogLevel,
		Format: cfg.LogFormat,
	}
	slogger := logger.NewLogger(logConfig)

	slogger.Info("Initializing Platform API server...")
	srv, err := server.StartPlatformAPIServer(cfg, slogger)
	if err != nil {
		slogger.Error("Failed to create server", "error", err)
		os.Exit(1)
	}

	slogger.Info("Starting server",
		"http_enabled", cfg.HTTP.Enabled, "http_port", cfg.HTTP.Port,
		"https_enabled", cfg.HTTPS.Enabled, "https_port", cfg.HTTPS.Port)
	if err := srv.Start(cfg.HTTP, cfg.HTTPS); err != nil {
		slogger.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}
