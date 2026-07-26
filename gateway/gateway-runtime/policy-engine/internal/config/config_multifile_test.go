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

// writeMultiTOML writes contents to a uniquely-named .toml file under dir and
// returns its path.
func writeMultiTOML(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	return path
}

// TestLoad_MultiFile_DeepMergeLastWins verifies that when the same nested table is
// set across two files, keys deep-merge and a key present in both files takes the
// later file's value (last-wins), while a key only in the base survives.
func TestLoad_MultiFile_DeepMergeLastWins(t *testing.T) {
	dir := t.TempDir()
	base := writeMultiTOML(t, dir, "base.toml", `
[policy_engine.server]
extproc_port = 9111

[policy_engine.logging]
level = "info"
format = "json"
`)
	overlay := writeMultiTOML(t, dir, "overlay.toml", `
[policy_engine.logging]
level = "debug"
`)

	cfg, err := Load(base, overlay)
	require.NoError(t, err)

	// Overridden in the overlay.
	assert.Equal(t, "debug", cfg.PolicyEngine.Logging.Level, "later file must win for a shared key")
	// Only set in the base — must survive the merge.
	assert.Equal(t, 9111, cfg.PolicyEngine.Server.ExtProcPort, "base-only key must survive")
	// Set in the base, not touched by the overlay — deep-merge must keep it.
	assert.Equal(t, "json", cfg.PolicyEngine.Logging.Format, "sibling key in a merged table must survive")
}

// TestLoad_MultiFile_ArrayReplaceNotAppend verifies koanf semantics for list
// values: a later file replaces the base list wholesale rather than appending.
func TestLoad_MultiFile_ArrayReplaceNotAppend(t *testing.T) {
	dir := t.TempDir()
	base := writeMultiTOML(t, dir, "base.toml", `
[traffic_logging]
masked_headers = ["authorization", "cookie", "x-api-key"]
`)
	overlay := writeMultiTOML(t, dir, "overlay.toml", `
[traffic_logging]
masked_headers = ["authorization"]
`)

	cfg, err := Load(base, overlay)
	require.NoError(t, err)

	assert.Equal(t, []string{"authorization"}, cfg.TrafficLogging.MaskedHeaders,
		"array value must be replaced by the later file, not appended")
}

// TestLoad_MultiFile_StrictMergeTypeMismatchFails verifies that StrictMerge makes a
// type-mismatched override of the same key across files fail loudly.
func TestLoad_MultiFile_StrictMergeTypeMismatchFails(t *testing.T) {
	dir := t.TempDir()
	base := writeMultiTOML(t, dir, "base.toml", `
[policy_engine.server]
extproc_port = 9001
`)
	overlay := writeMultiTOML(t, dir, "overlay.toml", `
[policy_engine.server]
extproc_port = "not-a-number"
`)

	_, err := Load(base, overlay)
	require.Error(t, err, "a type-mismatched override across files must fail loudly under StrictMerge")
}

// TestLoad_MultiFile_InterpolationAfterMerge verifies that {{ env }} interpolation
// runs once, after all files are merged: a token declared in the base is resolved
// even when a later overlay overrides an unrelated key.
func TestLoad_MultiFile_InterpolationAfterMerge(t *testing.T) {
	dir := t.TempDir()
	base := writeMultiTOML(t, dir, "base.toml", `
[policy_engine.logging]
level = '{{ env "APIP_TEST_PE_LOG_LEVEL" "info" }}'
`)
	overlay := writeMultiTOML(t, dir, "overlay.toml", `
[policy_engine.logging]
format = "text"
`)

	t.Setenv("APIP_TEST_PE_LOG_LEVEL", "warn")

	cfg, err := Load(base, overlay)
	require.NoError(t, err)

	assert.Equal(t, "warn", cfg.PolicyEngine.Logging.Level, "token in base must resolve after merge")
	assert.Equal(t, "text", cfg.PolicyEngine.Logging.Format, "overlay override must apply alongside interpolation")
}

// TestLoad_MultiFile_OrderMatters verifies precedence follows argument order:
// swapping the two files flips which value wins.
func TestLoad_MultiFile_OrderMatters(t *testing.T) {
	dir := t.TempDir()
	a := writeMultiTOML(t, dir, "a.toml", `
[policy_engine.logging]
level = "debug"
`)
	b := writeMultiTOML(t, dir, "b.toml", `
[policy_engine.logging]
level = "error"
`)

	cfgAB, err := Load(a, b)
	require.NoError(t, err)
	assert.Equal(t, "error", cfgAB.PolicyEngine.Logging.Level, "last file (b) must win")

	cfgBA, err := Load(b, a)
	require.NoError(t, err)
	assert.Equal(t, "debug", cfgBA.PolicyEngine.Logging.Level, "last file (a) must win when order is swapped")
}
