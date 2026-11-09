/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package configinterpolate

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expandLeaf renders a single string leaf through Expand and returns the resolved
// value (or the error). It keeps the env/file table tests concise.
func expandLeaf(t *testing.T, leaf string, opts Options) (string, error) {
	t.Helper()
	out, _, err := Expand(map[string]any{"v": leaf}, opts)
	if err != nil {
		return "", err
	}
	s, ok := out["v"].(string)
	require.True(t, ok, "leaf did not render to a string: %#v", out["v"])
	return s, nil
}

func TestEnvFunc(t *testing.T) {
	t.Run("set returns value", func(t *testing.T) {
		t.Setenv("CI_KEY", "the-value")
		got, err := expandLeaf(t, `{{ env "CI_KEY" }}`, Options{})
		require.NoError(t, err)
		assert.Equal(t, "the-value", got)
	})

	t.Run("unset with fallback returns fallback", func(t *testing.T) {
		os.Unsetenv("CI_MISSING")
		got, err := expandLeaf(t, `{{ env "CI_MISSING" "8080" }}`, Options{})
		require.NoError(t, err)
		assert.Equal(t, "8080", got)
	})

	t.Run("unset without fallback fails closed", func(t *testing.T) {
		os.Unsetenv("CI_MISSING")
		_, err := expandLeaf(t, `{{ env "CI_MISSING" }}`, Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `required env var "CI_MISSING" is not found`)
	})

	t.Run("set-empty is treated as unset (uses fallback)", func(t *testing.T) {
		t.Setenv("CI_EMPTY", "")
		got, err := expandLeaf(t, `{{ env "CI_EMPTY" "fallback" }}`, Options{})
		require.NoError(t, err)
		assert.Equal(t, "fallback", got)
	})

	t.Run("set-empty without fallback fails closed", func(t *testing.T) {
		t.Setenv("CI_EMPTY", "")
		_, err := expandLeaf(t, `{{ env "CI_EMPTY" }}`, Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `required env var "CI_EMPTY" is not found`)
	})
}

// writeSecret creates a file under dir and returns its absolute path.
func writeSecret(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

func TestFileFunc(t *testing.T) {
	root := t.TempDir()

	t.Run("reads file within allowlist and trims trailing whitespace", func(t *testing.T) {
		p := writeSecret(t, root, "token", "s3cr3t\n")
		got, err := expandLeaf(t, `{{ file "`+p+`" }}`, Options{FileAllowlist: []string{root}})
		require.NoError(t, err)
		assert.Equal(t, "s3cr3t", got)
	})

	t.Run("empty allowlist rejects file()", func(t *testing.T) {
		p := writeSecret(t, root, "token2", "x")
		_, err := expandLeaf(t, `{{ file "`+p+`" }}`, Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file interpolation not permitted")
	})

	t.Run("path outside allowlist is rejected", func(t *testing.T) {
		other := t.TempDir()
		p := writeSecret(t, other, "token", "x")
		_, err := expandLeaf(t, `{{ file "`+p+`" }}`, Options{FileAllowlist: []string{root}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not in an allowed source directory")
	})

	t.Run("traversal sequence is rejected", func(t *testing.T) {
		bad := root + "/../etc/passwd"
		_, err := expandLeaf(t, `{{ file "`+bad+`" }}`, Options{FileAllowlist: []string{root}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not in an allowed source directory")
	})

	t.Run("partial prefix does not match (separator suffix)", func(t *testing.T) {
		// A sibling dir sharing a name prefix must not be considered inside root.
		sibling := root + "-other"
		require.NoError(t, os.MkdirAll(sibling, 0o755))
		t.Cleanup(func() { os.RemoveAll(sibling) })
		p := writeSecret(t, sibling, "token", "x")
		_, err := expandLeaf(t, `{{ file "`+p+`" }}`, Options{FileAllowlist: []string{root}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not in an allowed source directory")
	})

	t.Run("missing file is required (fails closed)", func(t *testing.T) {
		p := filepath.Join(root, "does-not-exist")
		_, err := expandLeaf(t, `{{ file "`+p+`" }}`, Options{FileAllowlist: []string{root}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not found")
	})

	t.Run("oversize file is rejected without leaking the limit", func(t *testing.T) {
		p := writeSecret(t, root, "big", strings.Repeat("A", 100))
		_, err := expandLeaf(t, `{{ file "`+p+`" }}`, Options{FileAllowlist: []string{root}, MaxFileBytes: 10})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds the maximum allowed size")
		// The message names the file, and t.TempDir() puts a random number in its path that
		// can itself contain "10". Only the rest of the message must not leak the limit.
		assert.NotContains(t, strings.ReplaceAll(err.Error(), p, ""), "10")
	})
}

// TestFileFunc_K8sSymlinkShape reproduces the Kubernetes Secret mount layout:
// <root>/token -> ..data/token, and <root>/..data -> ..<timestamp>_data. The
// resolved target stays under the allowlist root and must be accepted.
func TestFileFunc_K8sSymlinkShape(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "..2026_01_01_data")
	require.NoError(t, os.Mkdir(dataDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "token"), []byte("k8s-secret\n"), 0o600))
	require.NoError(t, os.Symlink("..2026_01_01_data", filepath.Join(root, "..data")))
	require.NoError(t, os.Symlink(filepath.Join("..data", "token"), filepath.Join(root, "token")))

	got, err := expandLeaf(t, `{{ file "`+filepath.Join(root, "token")+`" }}`, Options{FileAllowlist: []string{root}})
	require.NoError(t, err)
	assert.Equal(t, "k8s-secret", got)
}

// TestFileFunc_SymlinkEscapeRejected ensures a symlink inside the allowlist that
// points outside the allowlist is rejected by the resolved re-check.
func TestFileFunc_SymlinkEscapeRejected(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := writeSecret(t, outside, "real", "leaked")
	link := filepath.Join(root, "token")
	require.NoError(t, os.Symlink(secret, link))

	_, err := expandLeaf(t, `{{ file "`+link+`" }}`, Options{FileAllowlist: []string{root}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not in an allowed source directory")
}

func TestReadAllowedFile(t *testing.T) {
	root := t.TempDir()
	allow := []string{root}

	t.Run("reads a file inside the allowlist as it is, without trimming", func(t *testing.T) {
		p := writeSecret(t, root, "template.json", "{\"a\":1}\n")
		got, err := ReadAllowedFile(p, allow, 0)
		require.NoError(t, err)
		assert.Equal(t, "{\"a\":1}\n", string(got))
	})

	t.Run("a file at the limit is read and one byte over is rejected", func(t *testing.T) {
		p := writeSecret(t, root, "limit", strings.Repeat("A", 10))
		got, err := ReadAllowedFile(p, allow, 10)
		require.NoError(t, err)
		assert.Len(t, got, 10)

		p = writeSecret(t, root, "over", strings.Repeat("A", 11))
		_, err = ReadAllowedFile(p, allow, 10)
		require.ErrorContains(t, err, "exceeds the maximum allowed size")
	})

	t.Run("a limit of zero uses the default", func(t *testing.T) {
		p := writeSecret(t, root, "default-limit", strings.Repeat("A", int(DefaultMaxFileBytes)))
		got, err := ReadAllowedFile(p, allow, 0)
		require.NoError(t, err)
		assert.Len(t, got, int(DefaultMaxFileBytes))

		p = writeSecret(t, root, "default-over", strings.Repeat("A", int(DefaultMaxFileBytes)+1))
		_, err = ReadAllowedFile(p, allow, 0)
		require.ErrorContains(t, err, "exceeds the maximum allowed size")
	})

	// Rejected paths: the error names the path, never the content, the allowlist or the limit.
	outside := t.TempDir()
	secret := writeSecret(t, outside, "secret.json", `{"token":"leaked-content"}`)
	link := filepath.Join(root, "link.json")
	require.NoError(t, os.Symlink(secret, link))
	sibling := root + "-other"
	require.NoError(t, os.MkdirAll(sibling, 0o755))
	t.Cleanup(func() { os.RemoveAll(sibling) })
	siblingFile := writeSecret(t, sibling, "x.json", `{"token":"leaked-content"}`)

	tests := []struct {
		name string
		path string
		want string
	}{
		{"outside the allowlist", secret, "is not in an allowed source directory"},
		{"a .. escape", root + "/../" + filepath.Base(outside) + "/secret.json", "is not in an allowed source directory"},
		{"a symlink inside pointing outside", link, "is not in an allowed source directory"},
		{"a null byte", root + "/template.json\x00.txt", "is not in an allowed source directory"},
		{"a directory sharing the allowlist's name as a prefix", siblingFile, "is not in an allowed source directory"},
		{"a relative path", "template.json", "is not in an allowed source directory"},
		{"a missing file", filepath.Join(root, "missing.json"), "is not found"},
	}
	for _, tt := range tests {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			got, err := ReadAllowedFile(tt.path, allow, 10)
			require.ErrorContains(t, err, tt.want)
			assert.Nil(t, got)
			assert.NotContains(t, err.Error(), "leaked-content")
			// The message names the caller's path; apart from that it holds neither the allowlist nor the limit.
			rest := strings.ReplaceAll(err.Error(), strconv.Quote(tt.path), "")
			assert.NotContains(t, rest, root, "the allowlist is not named")
			assert.NotRegexp(t, `\b10\b`, rest, "the limit is not named")
		})
	}

	t.Run("an empty allowlist rejects every path", func(t *testing.T) {
		p := writeSecret(t, root, "any.json", "x")
		_, err := ReadAllowedFile(p, nil, 0)
		require.Error(t, err)
	})
}
