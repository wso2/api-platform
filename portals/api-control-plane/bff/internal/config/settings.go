/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package config

import (
	"fmt"
	"log/slog"

	tomlparser "github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"

	"api-control-plane-bff/internal/configinterpolate"
)

// EnvPrefix namespaces this BFF's environment variables, alongside the Platform
// API's APIP_CP_, the AI Workspace's APIP_AIW_, and the API Portal's APIP_AP_.
//
// It is a naming convention, not a binding: the environment reaches config only
// through the {{ env "NAME" }} tokens written in config.toml. A key with no token
// cannot be set from the environment at all.
const EnvPrefix = "APIP_ACP_"

// apiControlPlaneConfigKey is the top-level TOML table all of this BFF's settings
// live under (e.g. [api_control_plane], [api_control_plane.control_plane]). This
// namespacing lets its config file coexist with sibling components' sections
// ([platform_api], [ai_workspace], ...) in a shared deployment config.
// loadConfigKoanf cuts to this table before interpolating, so a shared file's
// other sections — and their own {{ env }}/{{ file }} tokens — are never touched.
const apiControlPlaneConfigKey = "api_control_plane"

// defaultFileSourceAllowlist is this BFF's default set of directories a
// {{ file "..." }} token may read from. Overridable via the shared
// APIP_CONFIG_FILE_SOURCE_ALLOWLIST env var (see configinterpolate.ResolveAllowlist).
var defaultFileSourceAllowlist = []string{
	"/etc/api-control-plane",
	"/secrets/api-control-plane",
}

// loadConfigKoanf reads config.toml, expands its {{ env }} / {{ file }}
// interpolation tokens, and returns a koanf instance rooted at the
// [api_control_plane] subtree.
//
// config.toml is the only source of configuration: there is no implicit
// environment overlay, so a value comes from the environment exactly when the
// key's token asks for it. Interpolation fails closed — an unset {{ env }}
// variable with no default, or an unreadable/disallowed {{ file }} path, aborts
// startup rather than silently yielding an empty credential.
//
// A missing config.toml is not an error: the returned instance is empty, so every
// key falls back to defaultConfig() and Load fails only on the required ones.
func loadConfigKoanf(tomlPaths ...string) (*koanf.Koanf, error) {
	// Deliberately NOT koanf StrictMerge: strict merging compares the raw parsed
	// types across files, but an {{ env }} / {{ file }} interpolation token is a
	// string until it is resolved after the merge — so strict merging would reject
	// a numeric/bool field that one file sets natively and another overrides with
	// a token. Cross-file type errors are instead caught downstream by the
	// weakly-typed unmarshal and validation.
	k := koanf.New(".")

	for _, tomlPath := range tomlPaths {
		if err := k.Load(file.Provider(tomlPath), tomlparser.Parser()); err != nil {
			return nil, fmt.Errorf("failed to parse config file %q: %w", tomlPath, err)
		}
	}

	// Narrow to this component's own subtree BEFORE interpolating — see the
	// apiControlPlaneConfigKey doc comment for why.
	k = k.Cut(apiControlPlaneConfigKey)

	expanded, stats, err := configinterpolate.Expand(k.Raw(), configinterpolate.Options{
		FileAllowlist: configinterpolate.ResolveAllowlist(defaultFileSourceAllowlist),
	})
	if err != nil {
		return nil, fmt.Errorf("config interpolation failed: %w", err)
	}
	if stats.Fields > 0 {
		// Counts only — a resolved value is a secret and is never logged.
		slog.Info("config interpolation complete",
			slog.Int("env_refs", stats.EnvRefs),
			slog.Int("file_refs", stats.FileRefs),
			slog.Int("fields", stats.Fields))
	}

	out := koanf.New(".")
	if err := out.Load(confmap.Provider(expanded, "."), nil); err != nil {
		return nil, fmt.Errorf("failed to reload interpolated config: %w", err)
	}
	return out, nil
}
