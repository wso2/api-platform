/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

// Package platform is the public runtime façade a wrapper module imports to build
// and run the platform-api server. The split is clean — platform runs the server,
// pdk says what a plugin is and what it can reach.
//
// This package deliberately re-exports nothing from pdk. A wrapper's main imports
// platform to construct and run the App; the packages that implement plugins
// import pdk directly, so every contract type has exactly one name and pdk's own
// documentation is what a plugin author reads. The façade works entirely in terms
// of that external surface and never exposes internal/plugin.
package platform

import (
	"log/slog"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/logger"
	"github.com/wso2/api-platform/platform-api/internal/server"
	"github.com/wso2/api-platform/platform-api/pdk"
)

// App is a configured, ready-to-run platform-api server.
type App struct {
	cfg     *config.Server
	logger  *slog.Logger
	plugins []pdk.Plugin
}

// New builds an App from the given options. Configuration and logger resolve
// eagerly, so config/logger errors surface here rather than from Run.
func New(opts ...Option) (*App, error) {
	a := &App{}
	for _, opt := range opts {
		opt(a)
	}

	// WithConfig takes precedence; otherwise load config (WithConfigPath, applied
	// above, has already pointed the loader at the wrapper's file if set).
	if a.cfg == nil {
		a.cfg = config.GetConfig()
	}
	if a.cfg == nil {
		return nil, errNoConfig
	}

	if a.logger == nil {
		a.logger = logger.NewLogger(logger.Config{
			Level:  a.cfg.Logging.Level,
			Format: a.cfg.Logging.Format,
		})
	}

	return a, nil
}

// Run starts the server and blocks until it exits (signal or fatal error).
// Wrappers only ever supply external (pdk) plugins; built-ins are supplied
// directly by cmd/main.go, so the internal tier passed here is nil.
func (a *App) Run() error {
	srv, err := server.StartPlatformAPIServer(a.cfg, a.logger, nil, a.plugins)
	if err != nil {
		return err
	}
	return srv.Start(a.cfg.Listeners, a.cfg.Listeners.Timeouts)
}
