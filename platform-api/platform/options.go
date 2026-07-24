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

package platform

import (
	"errors"
	"log/slog"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/pdk"
)

// errNoConfig is returned by New when configuration could not be resolved.
var errNoConfig = errors.New("platform: no configuration resolved")

// Option configures an App. Options are applied in order; later options that set
// the same field win.
type Option func(*App)

// WithConfigPath loads platform-api config from a TOML file (env > file >
// defaults). It points the shared config loader at the given path; the config is
// materialized in New. WithConfig takes precedence over this.
func WithConfigPath(path string) Option {
	return func(a *App) {
		if path != "" {
			config.SetConfigPath(path)
		}
	}
}

// WithConfig supplies an already-built config, taking precedence over
// WithConfigPath.
func WithConfig(cfg *config.Server) Option {
	return func(a *App) { a.cfg = cfg }
}

// WithLogger supplies the logger. When omitted, one is derived from the config's
// log level and format.
func WithLogger(l *slog.Logger) Option {
	return func(a *App) { a.logger = l }
}

// WithPlugin registers wrapper-owned extension modules — the one way a wrapper
// adds routes, scopes, and hooks. Each plugin implements pdk.Plugin and receives
// pdk.Deps in Init. Multiple calls accumulate; nil plugins are ignored.
func WithPlugin(plugins ...pdk.Plugin) Option {
	return func(a *App) {
		for _, p := range plugins {
			if p != nil {
				a.plugins = append(a.plugins, p)
			}
		}
	}
}
