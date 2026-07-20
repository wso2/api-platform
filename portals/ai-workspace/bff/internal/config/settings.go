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
	"strconv"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/configinterpolate"
)

// EnvPrefix namespaces the AI Workspace's environment variables. It mirrors the
// Platform API's APIP_CP_ and the Developer Portal's APIP_DP_.
//
// It is a naming convention, not a binding: the environment reaches the config only
// through the {{ env "NAME" }} tokens written in config.toml, which name the variable
// explicitly. By convention a key's variable is its dotted path uppercased, with dots
// as underscores, prefixed — so [oidc] client_id ships as
// '{{ env "APIP_AIW_OIDC_CLIENT_ID" }}'. A token may name any variable, and a key with
// no token cannot be set from the environment at all.
//
// The prefix also namespaces the runtime config the SPA reads (see runtimeKey).
const EnvPrefix = "APIP_AIW_"

// aiWorkspaceConfigKey is the top-level TOML table all AI Workspace settings live
// under (e.g. [ai_workspace], [ai_workspace.platform_api]). It mirrors the Platform
// API's platformAPIConfigKey: this namespacing lets an AI Workspace config file
// coexist with sibling services' sections ([platform_api], ...) in a shared
// deployment config, the same file convention as the Platform API's [platform_api]
// table. Every key below this cut (log_level, platform_api.url, oidc.*, ...) is
// resolved relative to [ai_workspace], not the file root.
const aiWorkspaceConfigKey = "ai_workspace"

// defaultFileSourceAllowlist is the AI Workspace's default set of directories a
// {{ file "..." }} token may read from. Overridable via the shared
// APIP_CONFIG_FILE_SOURCE_ALLOWLIST env var (see configinterpolate.ResolveAllowlist).
var defaultFileSourceAllowlist = []string{
	"/etc/ai-workspace",
	"/secrets/ai-workspace",
}

// settings is the fully-resolved configuration: the config.toml values under
// [ai_workspace], with every {{ env }} / {{ file }} token expanded and flattened to
// dotted paths relative to that table. A key is its path under [ai_workspace] joined
// with dots — [ai_workspace.platform_api] url becomes "platform_api.url" — and a key
// directly under [ai_workspace] keeps its bare name ("domain"). Sibling top-level
// tables belonging to other services (e.g. [platform_api]) are ignored.
type settings map[string]string

// loadSettings reads config.toml and expands its interpolation tokens. config.toml is
// the only source of configuration: there is no implicit environment overlay, so a
// value comes from the environment exactly when the key's token asks for it, e.g.
//
//	log_level = '{{ env "APIP_AIW_LOG_LEVEL" "info" }}'
//
// One mechanism therefore covers both ordinary settings and secrets, and every source
// a value can come from is visible in the file itself rather than implied by a naming
// rule. Interpolation fails closed: an unset {{ env }} variable with no default, or an
// unreadable/disallowed {{ file }} path, aborts startup rather than silently yielding
// an empty credential.
//
// A missing config.toml is not an error — the built-in defaults still apply — but the
// required keys (platform_api_url) then have no value, so Load fails on them.
func loadSettings(tomlPath string) (settings, error) {
	raw, err := parseTOML(tomlPath)
	if err != nil {
		return nil, err
	}

	// Expand walks the whole tree, so a token works at any depth.
	expanded, stats, err := configinterpolate.Expand(raw, configinterpolate.Options{
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

	s := settings{}
	flatten(s, "", cut(expanded, aiWorkspaceConfigKey))
	return s, nil
}

// cut returns the subtree of tree rooted at key, or an empty tree when key is
// absent (a missing [ai_workspace] table simply leaves every key on its default,
// same as a missing config file). A key present but not a table is also treated as
// absent — flatten only ever descends into map[string]any nodes.
func cut(tree map[string]any, key string) map[string]any {
	if sub, ok := tree[key].(map[string]any); ok {
		return sub
	}
	return map[string]any{}
}

// parseTOML decodes the config file with the stdlib subset parser (see toml.go).
// A missing file yields an empty tree rather than an error, leaving every key on
// its default — Load then fails on the required ones.
func parseTOML(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("failed to read config file %q: %w", path, err)
	}

	raw, err := parseTOMLSubset(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file %q: %w", path, err)
	}
	return raw, nil
}

// flatten collapses the decoded tree into dotted keys ([oidc] client_id ->
// "oidc.client_id"), stringifying scalars so a value may be written either quoted or
// bare — tls.enabled = true and tls.enabled = "true" both reach getbool as "true".
// Arrays have no config key and are rejected by the parser before reaching here.
func flatten(dst settings, prefix string, tree map[string]any) {
	for k, v := range tree {
		key := strings.ToLower(k)
		if prefix != "" {
			key = prefix + "." + key
		}
		switch val := v.(type) {
		case map[string]any:
			flatten(dst, key, val)
		case string:
			dst[key] = val
		case bool, int64, float64:
			dst[key] = fmt.Sprint(val)
		}
	}
}

// get returns the value for key, or def when it is unset or empty.
func (s settings) get(key, def string) string {
	if v, ok := s[key]; ok && v != "" {
		return v
	}
	return def
}

// getbool parses key as a boolean. A malformed value fails startup rather than
// being silently replaced by the default.
func (s settings) getbool(key string, def bool) (bool, error) {
	v, ok := s[key]
	if !ok || v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return false, fmt.Errorf("invalid boolean for %s=%q: %w", key, v, err)
	}
	return b, nil
}

// getdur parses key as a Go duration. A malformed, zero, or negative value fails
// startup — every duration setting is a lifetime or timeout, where <= 0 is never
// meaningful.
func (s settings) getdur(key string, def time.Duration) (time.Duration, error) {
	v, ok := s[key]
	if !ok || v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil {
		return 0, fmt.Errorf("invalid duration for %s=%q: %w", key, v, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid duration for %s=%q: must be positive", key, v)
	}
	return d, nil
}
