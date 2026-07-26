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
		"\n[platform_api.auth]\nskip_paths = [\"/health\", \"/metrics\", \"/ready\"]\n",
		"\n[platform_api.auth]\nskip_paths = [\"/livez\"]\n",
	)
	require.NoError(t, err)

	assert.Equal(t, []string{"/livez"}, cfg.Auth.SkipPaths,
		"array value must be replaced by the later file, not appended")
}

// TestLoadConfig_MultiFile_StrictMergeTypeMismatchFails verifies that StrictMerge
// makes a type-mismatched override of the same key across files fail loudly. The
// merge fails before validation, so no valid-keys base is required here.
func TestLoadConfig_MultiFile_StrictMergeTypeMismatchFails(t *testing.T) {
	dir := t.TempDir()
	base := writeMultiTOML(t, dir, "base.toml", "[platform_api.auth]\nscope_validation = true\n")
	over := writeMultiTOML(t, dir, "overlay.toml", "[platform_api.auth]\nscope_validation = \"maybe\"\n")

	_, err := LoadConfig(base, over)
	require.Error(t, err, "a type-mismatched override across files must fail loudly under StrictMerge")
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
