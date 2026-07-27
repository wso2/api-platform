/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTOML writes contents to a uniquely-named .toml file under dir and returns
// its path.
func writeTOML(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	return path
}

// TestLoadConfig_MultiFile_DeepMergeLastWins verifies that when the same nested
// table is set across two files, keys deep-merge and a key present in both files
// takes the later file's value (last-wins), while a key only in the base survives.
func TestLoadConfig_MultiFile_DeepMergeLastWins(t *testing.T) {
	dir := t.TempDir()
	base := writeTOML(t, dir, "base.toml", `
[controller.server]
api_port = 9999

[controller.logging]
level = "info"
format = "json"
`)
	overlay := writeTOML(t, dir, "overlay.toml", `
[controller.logging]
level = "debug"
`)

	cfg, err := LoadConfig(base, overlay)
	require.NoError(t, err)

	// Overridden in the overlay.
	assert.Equal(t, "debug", cfg.Controller.Logging.Level, "later file must win for a shared key")
	// Only set in the base — must survive the merge.
	assert.EqualValues(t, 9999, cfg.Controller.Server.APIPort, "base-only key must survive")
	// Set in the base, not touched by the overlay — deep-merge must keep it.
	assert.Equal(t, "json", cfg.Controller.Logging.Format, "sibling key in a merged table must survive")
}

// TestLoadConfig_MultiFile_ArrayReplaceNotAppend verifies koanf semantics for
// list values: a later file replaces the base list wholesale rather than
// appending to it.
func TestLoadConfig_MultiFile_ArrayReplaceNotAppend(t *testing.T) {
	dir := t.TempDir()
	base := writeTOML(t, dir, "base.toml", `
[collector]
ignore_path_prefixes = ["/health", "/metrics", "/ready"]
`)
	overlay := writeTOML(t, dir, "overlay.toml", `
[collector]
ignore_path_prefixes = ["/livez"]
`)

	cfg, err := LoadConfig(base, overlay)
	require.NoError(t, err)

	assert.Equal(t, []string{"/livez"}, cfg.Collector.IgnorePathPrefixes,
		"array value must be replaced by the later file, not appended")
}

// TestLoadConfig_MultiFile_TypeMismatchFails verifies that a genuinely invalid
// cross-file override — a non-numeric string for a numeric field — still fails. The
// loader does not use koanf StrictMerge (see LoadConfig), so this is caught by the
// weakly-typed unmarshal after the merge rather than at merge time.
func TestLoadConfig_MultiFile_TypeMismatchFails(t *testing.T) {
	dir := t.TempDir()
	base := writeTOML(t, dir, "base.toml", `
[controller.server]
api_port = 8080
`)
	overlay := writeTOML(t, dir, "overlay.toml", `
[controller.server]
api_port = "not-a-number"
`)

	_, err := LoadConfig(base, overlay)
	require.Error(t, err, "a non-coercible cross-file override must still fail (at unmarshal)")
}

// TestLoadConfig_MultiFile_NumericOverriddenByEnvToken verifies that a numeric field
// set natively in the base can be overridden by an {{ env }} interpolation token (a
// TOML string) in an overlay — the token resolves and coerces to the numeric field.
// This is the reason the loader does not use koanf StrictMerge: a strict type check
// would reject the int-vs-string collision before interpolation runs.
func TestLoadConfig_MultiFile_NumericOverriddenByEnvToken(t *testing.T) {
	dir := t.TempDir()
	base := writeTOML(t, dir, "base.toml", `
[controller.server]
api_port = 8080
`)
	overlay := writeTOML(t, dir, "overlay.toml", `
[controller.server]
api_port = '{{ env "APIP_TEST_API_PORT" "9090" }}'
`)
	t.Setenv("APIP_TEST_API_PORT", "9091")

	cfg, err := LoadConfig(base, overlay)
	require.NoError(t, err)
	assert.EqualValues(t, 9091, cfg.Controller.Server.APIPort,
		"an {{ env }} token in an overlay must override a native numeric base value")
}

// TestLoadConfig_MultiFile_EnvTokenResolvingToNonNumberFails verifies that
// interpolation does not bypass the type check: a numeric field whose {{ env }}
// token resolves to a non-numeric value still fails loudly, at the weakly-typed
// unmarshal after the token is expanded. (An unset/empty required token is a
// separate fail-closed path, covered by interpolate_test.go.)
func TestLoadConfig_MultiFile_EnvTokenResolvingToNonNumberFails(t *testing.T) {
	dir := t.TempDir()
	base := writeTOML(t, dir, "base.toml", `
[controller.server]
api_port = 8080
`)
	overlay := writeTOML(t, dir, "overlay.toml", `
[controller.server]
api_port = '{{ env "APIP_TEST_API_PORT" }}'
`)
	t.Setenv("APIP_TEST_API_PORT", "bar")

	_, err := LoadConfig(base, overlay)
	require.Error(t, err, "an env token resolving to a non-number must still fail (at unmarshal)")
}

// TestLoadConfig_MultiFile_InterpolationAfterMerge verifies that {{ env }}
// interpolation runs once, after all files are merged: a token declared in the
// base is resolved even when a later overlay overrides an unrelated key.
func TestLoadConfig_MultiFile_InterpolationAfterMerge(t *testing.T) {
	dir := t.TempDir()
	base := writeTOML(t, dir, "base.toml", `
[controller.logging]
level = '{{ env "APIP_TEST_LOG_LEVEL" "info" }}'
`)
	overlay := writeTOML(t, dir, "overlay.toml", `
[controller.logging]
format = "text"
`)

	t.Setenv("APIP_TEST_LOG_LEVEL", "warn")

	cfg, err := LoadConfig(base, overlay)
	require.NoError(t, err)

	assert.Equal(t, "warn", cfg.Controller.Logging.Level, "token in base must resolve after merge")
	assert.Equal(t, "text", cfg.Controller.Logging.Format, "overlay override must apply alongside interpolation")
}

// TestLoadConfig_MultiFile_OrderMatters verifies precedence follows argument
// order: swapping the two files flips which value wins.
func TestLoadConfig_MultiFile_OrderMatters(t *testing.T) {
	dir := t.TempDir()
	a := writeTOML(t, dir, "a.toml", `
[controller.logging]
level = "debug"
`)
	b := writeTOML(t, dir, "b.toml", `
[controller.logging]
level = "error"
`)

	cfgAB, err := LoadConfig(a, b)
	require.NoError(t, err)
	assert.Equal(t, "error", cfgAB.Controller.Logging.Level, "last file (b) must win")

	cfgBA, err := LoadConfig(b, a)
	require.NoError(t, err)
	assert.Equal(t, "debug", cfgBA.Controller.Logging.Level, "last file (a) must win when order is swapped")
}

// TestLoadConfig_MultiFile_SingleFileUnchanged is a regression guard that the
// common single-file case still loads and validates as before.
func TestLoadConfig_MultiFile_SingleFileUnchanged(t *testing.T) {
	dir := t.TempDir()
	single := writeTOML(t, dir, "config.toml", `
[controller.logging]
level = "info"
`)

	cfg, err := LoadConfig(single)
	require.NoError(t, err)
	assert.Equal(t, "info", cfg.Controller.Logging.Level)
}

// TestLoadConfig_NoFiles verifies LoadConfig rejects an empty path list.
func TestLoadConfig_NoFiles(t *testing.T) {
	_, err := LoadConfig()
	require.Error(t, err, "LoadConfig must require at least one config file path")
}
