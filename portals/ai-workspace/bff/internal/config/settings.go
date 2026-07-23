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
	"os"

	tomlparser "github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/wso2/api-platform/common/configinterpolate"
)

// EnvPrefix namespaces the AI Workspace's environment variables. It mirrors the
// Platform API's APIP_CP_ and the Developer Portal's APIP_DP_.
//
// It is a naming convention, not a binding: the environment reaches the config only
// through the {{ env "NAME" }} tokens written in config.toml, which name the variable
// explicitly. By convention a key's variable is its dotted path uppercased, with dots
// as underscores, prefixed — so [auth.oidc] client_id ships as
// '{{ env "APIP_AIW_AUTH_OIDC_CLIENT_ID" }}'. A token may name any variable, and a key
// with no token cannot be set from the environment at all.
//
// The prefix also namespaces the runtime config the SPA reads (see runtimeKey).
const EnvPrefix = "APIP_AIW_"

// aiWorkspaceConfigKey is the top-level TOML table all AI Workspace settings live
// under (e.g. [ai_workspace], [ai_workspace.control_plane]). It mirrors the Platform
// API's platformAPIConfigKey: this namespacing lets an AI Workspace config file
// coexist with sibling services' sections ([platform_api], ...) in a shared
// deployment config. loadConfigKoanf cuts to this table, so every key is resolved
// relative to [ai_workspace] and sibling tables are ignored.
const aiWorkspaceConfigKey = "ai_workspace"

// defaultFileSourceAllowlist is the AI Workspace's default set of directories a
// {{ file "..." }} token may read from. Overridable via the shared
// APIP_CONFIG_FILE_SOURCE_ALLOWLIST env var (see configinterpolate.ResolveAllowlist).
var defaultFileSourceAllowlist = []string{
	"/etc/ai-workspace",
	"/secrets/ai-workspace",
}

// loadConfigKoanf reads config.toml, expands its {{ env }} / {{ file }} interpolation
// tokens, and returns a koanf instance rooted at the [ai_workspace] subtree — the same
// koanf-based loading stack the Gateway and Platform API use.
//
// config.toml is the only source of configuration: there is no implicit environment
// overlay, so a value comes from the environment exactly when the key's token asks for
// it. Interpolation fails closed — an unset {{ env }} variable with no default, or an
// unreadable/disallowed {{ file }} path, aborts startup rather than silently yielding
// an empty credential.
//
// A missing config.toml is not an error: the returned instance is empty, so every key
// falls back to defaultConfig() and Load fails only on the required ones.
func loadConfigKoanf(tomlPath string) (*koanf.Koanf, error) {
	k := koanf.New(".")

	// Stat first so a missing file stays a no-op (defaults apply) rather than a koanf
	// load error; anything else (e.g. a permission problem) is surfaced.
	if _, statErr := os.Stat(tomlPath); statErr == nil {
		if err := k.Load(file.Provider(tomlPath), tomlparser.Parser()); err != nil {
			return nil, fmt.Errorf("failed to parse config file %q: %w", tomlPath, err)
		}
	} else if !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("failed to read config file %q: %w", tomlPath, statErr)
	}

	// Expand tokens across the whole tree, so a token works at any depth. Shared with
	// the Platform API via configinterpolate, operating on koanf's raw nested map.
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

	// Reload the expanded map into a fresh instance so no un-interpolated leaf survives,
	// then cut to the [ai_workspace] subtree.
	out := koanf.New(".")
	if err := out.Load(confmap.Provider(expanded, "."), nil); err != nil {
		return nil, fmt.Errorf("failed to reload interpolated config: %w", err)
	}
	return out.Cut(aiWorkspaceConfigKey), nil
}
