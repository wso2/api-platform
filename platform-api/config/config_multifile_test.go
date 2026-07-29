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
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}

// loadTwoWithKeys sets the required secret env vars, writes a base file (with the
// valid-keys secret tokens prepended) plus an overlay, and loads both through
// LoadConfig — the multi-file analogue of loadWithKeys.
func loadTwoWithKeys(t *testing.T, baseExtra, overlay string) (*Server, error) {
	t.Helper()
	t.Setenv("APIP_CP_ENCRYPTION_KEY", validInlineKey)
	t.Setenv("APIP_CP_AUTH_JWT_PUBLIC_KEY_FILE", validJWTPublicKeyFile)
	dir := t.TempDir()
	base := writeMultiTOML(t, dir, "base.toml", validKeysBase+baseExtra)
	over := writeMultiTOML(t, dir, "overlay.toml", overlay)
	return LoadConfig(base, over)
}

// TestLoadConfig_MultiFile_DeepMergeLastWins verifies that when the same nested
// table is set across two files, keys deep-merge and a key present in both files
// takes the later file's value (last-wins), while a key only in the base survives.
func TestLoadConfig_MultiFile_DeepMergeLastWins(t *testing.T) {
	cfg, err := loadTwoWithKeys(t,
		"\n[platform_api.logging]\nlevel = \"info\"\nformat = \"json\"\n",
		"\n[platform_api.logging]\nlevel = \"debug\"\n",
	)
	require.NoError(t, err)

	assert.Equal(t, "debug", cfg.Logging.Level, "later file must win for a shared key")
	assert.Equal(t, "json", cfg.Logging.Format, "sibling key in a merged table must survive")
}

// TestLoadConfig_MultiFile_ArrayReplaceNotAppend verifies koanf semantics for list
// values: a later file replaces the base list wholesale rather than appending.
func TestLoadConfig_MultiFile_ArrayReplaceNotAppend(t *testing.T) {
	cfg, err := loadTwoWithKeys(t,
		"\n[platform_api.server.cors]\nallowed_origins = [\"https://a.example.com\", \"https://b.example.com\"]\n",
		"\n[platform_api.server.cors]\nallowed_origins = [\"https://c.example.com\"]\n",
	)
	require.NoError(t, err)

	assert.Equal(t, []string{"https://c.example.com"}, cfg.Listeners.CORS.AllowedOrigins,
		"array value must be replaced by the later file, not appended")
}

// TestLoadConfig_MultiFile_TypeMismatchFails verifies that a genuinely invalid
// cross-file override — a non-boolean string for a boolean field — still fails. The
// loader does not use koanf StrictMerge (see LoadConfig), so this is caught by the
// weakly-typed unmarshal after the merge rather than at merge time.
func TestLoadConfig_MultiFile_TypeMismatchFails(t *testing.T) {
	dir := t.TempDir()
	base := writeMultiTOML(t, dir, "base.toml", "[platform_api.auth]\nscope_validation = true\n")
	over := writeMultiTOML(t, dir, "overlay.toml", "[platform_api.auth]\nscope_validation = \"maybe\"\n")

	_, err := LoadConfig(base, over)
	require.Error(t, err, "a non-coercible cross-file override must still fail (at unmarshal)")
}

// TestLoadConfig_MultiFile_NumericOverriddenByEnvToken verifies that a numeric field
// set natively in the base can be overridden by an {{ env }} interpolation token (a
// TOML string) in an overlay — the token resolves and coerces to the numeric field.
// This is the reason the loader does not use koanf StrictMerge: a strict type check
// would reject the int-vs-string collision before interpolation runs.
func TestLoadConfig_MultiFile_NumericOverriddenByEnvToken(t *testing.T) {
	t.Setenv("APIP_TEST_HTTP_PORT", "9250")
	cfg, err := loadTwoWithKeys(t,
		"\n[platform_api.server.http]\nport = 9243\n",
		"\n[platform_api.server.http]\nport = '{{ env \"APIP_TEST_HTTP_PORT\" \"9243\" }}'\n",
	)
	require.NoError(t, err)

	assert.Equal(t, 9250, cfg.Listeners.HTTP.Port,
		"an {{ env }} token in an overlay must override a native numeric base value")
}

// TestLoadConfig_MultiFile_EnvTokenResolvingToNonNumberFails verifies that
// interpolation does not bypass the type check: a numeric field whose {{ env }}
// token resolves to a non-numeric value still fails loudly, at the weakly-typed
// unmarshal after the token is expanded. (An unset/empty required token is a
// separate fail-closed interpolation path.)
func TestLoadConfig_MultiFile_EnvTokenResolvingToNonNumberFails(t *testing.T) {
	t.Setenv("APIP_TEST_HTTP_PORT", "bar")
	_, err := loadTwoWithKeys(t,
		"\n[platform_api.server.http]\nport = 9243\n",
		"\n[platform_api.server.http]\nport = '{{ env \"APIP_TEST_HTTP_PORT\" }}'\n",
	)
	require.Error(t, err, "an env token resolving to a non-number must still fail (at unmarshal)")
}

// TestLoadConfig_MultiFile_InterpolationAfterMerge verifies that {{ env }}
// interpolation runs once, after all files are merged (and after the platform_api
// subtree Cut): a token declared in the base resolves even when a later overlay
// overrides an unrelated key.
func TestLoadConfig_MultiFile_InterpolationAfterMerge(t *testing.T) {
	t.Setenv("APIP_TEST_PAPI_LOG", "warn")
	cfg, err := loadTwoWithKeys(t,
		"\n[platform_api.logging]\nlevel = '{{ env \"APIP_TEST_PAPI_LOG\" \"info\" }}'\n",
		"\n[platform_api.logging]\nformat = \"text\"\n",
	)
	require.NoError(t, err)

	assert.Equal(t, "warn", cfg.Logging.Level, "token in base must resolve after merge")
	assert.Equal(t, "text", cfg.Logging.Format, "overlay override must apply alongside interpolation")
}

// TestLoadConfig_EmptyPathFails verifies that an explicit empty-string path (e.g.
// the binary invoked with `-config=`) is rejected at the path check with a clear
// error, rather than being skipped and surfacing a confusing downstream validation
// error (or, if defaults were ever valid, silently booting on them).
func TestLoadConfig_EmptyPathFails(t *testing.T) {
	_, err := LoadConfig("")
	require.Error(t, err, `LoadConfig("") must reject an empty config path, not skip it`)
	assert.Contains(t, err.Error(), "config path must not be empty",
		"the failure must be the explicit empty-path check, not an unrelated downstream error")
}

// TestLoadConfig_MultiFile_OrderMatters verifies precedence follows argument order:
// swapping the two files flips which value wins. The secret tokens live in file a,
// so both orderings still validate because a is present in both.
func TestLoadConfig_MultiFile_OrderMatters(t *testing.T) {
	t.Setenv("APIP_CP_ENCRYPTION_KEY", validInlineKey)
	t.Setenv("APIP_CP_AUTH_JWT_PUBLIC_KEY_FILE", validJWTPublicKeyFile)
	dir := t.TempDir()
	a := writeMultiTOML(t, dir, "a.toml", validKeysBase+"\n[platform_api.logging]\nlevel = \"debug\"\n")
	b := writeMultiTOML(t, dir, "b.toml", "[platform_api.logging]\nlevel = \"error\"\n")

	cfgAB, err := LoadConfig(a, b)
	require.NoError(t, err)
	assert.Equal(t, "error", cfgAB.Logging.Level, "last file (b) must win")

	cfgBA, err := LoadConfig(b, a)
	require.NoError(t, err)
	assert.Equal(t, "debug", cfgBA.Logging.Level, "last file (a) must win when order is swapped")
}
