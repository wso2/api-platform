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
	"fmt"
	"os"
	"strings"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/builtins"
	"github.com/wso2/api-platform/platform-api/internal/logger"
	"github.com/wso2/api-platform/platform-api/internal/server"
)

// stringSliceFlag collects a repeatable string flag into a slice, preserving the
// order in which the flags were supplied on the command line.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string { return strings.Join(*s, ", ") }

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	// -config is repeatable: files are merged in the order given with last-wins
	// precedence. Env values reach config only through explicit {{ env }}
	// interpolation tokens in the file(s) — there is no independent env provider —
	// so at least one config file is required and there is no default path.
	var configFiles stringSliceFlag
	flag.Var(&configFiles, "config",
		"Path to a configuration file (required; repeatable, merged in order with last-wins precedence)")
	flag.Parse()

	if len(configFiles) == 0 {
		fmt.Fprintf(os.Stderr, "Error: -config flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -config <path-to-config.toml> [-config <overlay.toml> ...]\n", os.Args[0])
		os.Exit(1)
	}

	config.SetConfigPaths(configFiles...)

	cfg := config.GetConfig()

	// Initialize logger
	logConfig := logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	}
	slogger := logger.NewLogger(logConfig)

	slogger.Info("Initializing Platform API server...")
	// Built-in (internal-tier) plugins are supplied here; the OSS entry point runs
	// no external-tier (pdk) plugins — those come from wrapper modules via the
	// platform façade. builtins.Plugins() is build-tag selected in its own package
	// so this entry point stays buildable as a single file (`go build ./cmd/main.go`).
	srv, err := server.StartPlatformAPIServer(cfg, slogger, builtins.Plugins(), nil)
	if err != nil {
		slogger.Error("Failed to create server", "error", err)
		os.Exit(1)
	}

	slogger.Info("Starting server",
		"http_enabled", cfg.Listeners.HTTP.Enabled, "http_port", cfg.Listeners.HTTP.Port,
		"https_enabled", cfg.Listeners.HTTPS.Enabled, "https_port", cfg.Listeners.HTTPS.Port)
	if err := srv.Start(cfg.Listeners, cfg.Listeners.Timeouts); err != nil {
		slogger.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}
