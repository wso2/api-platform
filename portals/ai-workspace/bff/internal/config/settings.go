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
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/configinterpolate"
)

// EnvPrefix namespaces the environment variables that override configuration keys.
// The prefix is stripped and the remainder lowercased to give the config key, e.g.
// APIP_AIW_LOG_LEVEL -> log_level, APIP_AIW_PLATFORM_API_URL -> platform_api_url.
// It mirrors the Platform API's APIP_CP_ and the Developer Portal's APIP_DP_.
//
// Two variables are deliberately unprefixed: APIP_DEMO_MODE (a standalone runtime
// flag shared across the stack) and the shared APIP_CONFIG_FILE_SOURCE_ALLOWLIST.
// The bare names inside {{ env "NAME" }} tokens are also read unprefixed — the
// token names an arbitrary environment variable, it is not a config key.
const EnvPrefix = "APIP_AIW_"

// defaultFileSourceAllowlist is the AI Workspace's default set of directories a
// {{ file "..." }} token may read from. Overridable via the shared
// APIP_CONFIG_FILE_SOURCE_ALLOWLIST env var (see configinterpolate.ResolveAllowlist).
var defaultFileSourceAllowlist = []string{
	"/etc/ai-workspace",
	"/secrets/ai-workspace",
}

// settings is the fully-resolved flat configuration: config.toml values overlaid
// with APIP_AIW_* environment variables, with every {{ env }} / {{ file }} token
// expanded. Keys are the flat config.toml key names (e.g. "platform_api_url").
type settings map[string]string

// loadSettings reads config.toml (when present), overlays the APIP_AIW_* env vars,
// then expands interpolation tokens across the merged result. Environment variables
// always win over the file. A missing config.toml is not an error — the BFF can be
// configured entirely from the environment.
//
// Interpolation runs after the merge so a value supplied either way may reference a
// secret, and it fails closed: an unset {{ env }} var or an unreadable/disallowed
// {{ file }} path aborts startup rather than silently yielding an empty credential.
func loadSettings(tomlPath string) (settings, error) {
	raw := map[string]any{}

	fileValues, err := parseFlatTOML(tomlPath)
	if err != nil {
		return nil, err
	}
	for k, v := range fileValues {
		raw[k] = v
	}

	// Env overlay. An empty value is treated as unset so that a `${VAR:-}`
	// placeholder in docker-compose does not shadow a value set in config.toml.
	for _, kv := range os.Environ() {
		name, value, ok := strings.Cut(kv, "=")
		if !ok || value == "" || !strings.HasPrefix(name, EnvPrefix) {
			continue
		}
		raw[strings.ToLower(strings.TrimPrefix(name, EnvPrefix))] = value
	}

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

	s := make(settings, len(expanded))
	for k, v := range expanded {
		if str, ok := v.(string); ok {
			s[k] = str
		}
	}
	return s, nil
}

// parseFlatTOML reads simple `key = value` lines from the config file. It is
// deliberately a naive line parser rather than a full TOML decoder: the BFF's
// config surface is flat by design (no nested tables), so table headers and
// comments are skipped. A missing file yields an empty map, not an error.
func parseFlatTOML(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil // env-only configuration
		}
		return nil, fmt.Errorf("failed to read config file %q: %w", path, err)
	}
	defer f.Close()

	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(key))] = strings.Trim(strings.TrimSpace(val), `"'`)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("failed to read config file %q: %w", path, err)
	}
	return out, nil
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

// getdur parses key as a Go duration. A malformed value fails startup.
func (s settings) getdur(key string, def time.Duration) (time.Duration, error) {
	v, ok := s[key]
	if !ok || v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil {
		return 0, fmt.Errorf("invalid duration for %s=%q: %w", key, v, err)
	}
	return d, nil
}
