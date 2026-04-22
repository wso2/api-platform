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

package discovery

import (
	"archive/zip"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-builder/internal/testutils"
)

func TestParsePipPackageRef_MajorOnly(t *testing.T) {
	ref, err := ParsePipPackageRef("prompt-compressor~=0.0")

	require.NoError(t, err)
	assert.Equal(t, "prompt-compressor", ref.PackageName)
	assert.Equal(t, "0.0", ref.Version)
	assert.Empty(t, ref.IndexURL)
	assert.True(t, ref.IsVersionRange)
}

func TestParsePipPackageRef_MinorOnly(t *testing.T) {
	ref, err := ParsePipPackageRef("prompt-compressor~=1.2.0")

	require.NoError(t, err)
	assert.Equal(t, "prompt-compressor", ref.PackageName)
	assert.Equal(t, "1.2.0", ref.Version)
	assert.Empty(t, ref.IndexURL)
	assert.True(t, ref.IsVersionRange)
}

func TestParsePipPackageRef_MajorOnlyWithIndex(t *testing.T) {
	ref, err := ParsePipPackageRef("prompt-compressor~=1.0@https://private.pypi.org/simple/")

	require.NoError(t, err)
	assert.Equal(t, "prompt-compressor", ref.PackageName)
	assert.Equal(t, "1.0", ref.Version)
	assert.Equal(t, "https://private.pypi.org/simple/", ref.IndexURL)
	assert.True(t, ref.IsVersionRange)
}

func TestParsePipPackageRef_Invalid(t *testing.T) {
	testCases := []string{
		"",
		"prompt-compressor~=1.2",
		"prompt-compressor~=",
		"~=1.0",
		"prompt-compressor",
	}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			_, err := ParsePipPackageRef(tc)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid pip spec")
		})
	}
}

func TestParseIndexURL(t *testing.T) {
	version, indexURL := parseIndexURL("1.0.0@https://user:token@pypi.private.com/simple")

	assert.Equal(t, "1.0.0", version)
	assert.Equal(t, "https://user:token@pypi.private.com/simple", indexURL)
}

func TestParseVCSPipSpec(t *testing.T) {
	spec, err := parseVCSPipSpec(
		"git+https://github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v0#subdirectory=policies/prompt-compressor",
	)

	require.NoError(t, err)
	assert.Equal(t, "https://github.com/wso2/gateway-controllers.git", spec.RepoURL)
	assert.Equal(t, "policies/prompt-compressor/v0", spec.GitRef)
	assert.Equal(t, "policies/prompt-compressor", spec.Subdirectory)
}

func TestParseVCSPipSpec_WithCredentials(t *testing.T) {
	spec, err := parseVCSPipSpec(
		"git+https://user:token@github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v0#subdirectory=policies/prompt-compressor",
	)

	require.NoError(t, err)
	assert.Equal(t, "https://user:token@github.com/wso2/gateway-controllers.git", spec.RepoURL)
	assert.Equal(t, "policies/prompt-compressor/v0", spec.GitRef)
	assert.Equal(t, "policies/prompt-compressor", spec.Subdirectory)
}

func TestExpandShortURL(t *testing.T) {
	expanded, err := expandShortURL("github.com/wso2/gateway-controllers/policies/prompt-compressor@v1")

	require.NoError(t, err)
	assert.Equal(
		t,
		"git+https://github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v1#subdirectory=policies/prompt-compressor",
		expanded,
	)
}

func TestExpandShortURL_ThreeSegmentsError(t *testing.T) {
	_, err := expandShortURL("github.com/wso2/repo@v1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "short URL must have at least 4 path segments")
}

func TestClassifyVCSRef(t *testing.T) {
	testCases := map[string]vcsVersionType{
		"policies/foo/v1":     vcsVersionMajorOnly,
		"policies/foo/v1.1":   vcsVersionMinorOnly,
		"policies/foo/v1.1.3": vcsVersionExact,
		"main":                vcsVersionNone,
	}

	for ref, expected := range testCases {
		t.Run(ref, func(t *testing.T) {
			assert.Equal(t, expected, classifyVCSRef(ref))
		})
	}
}

func TestReadVersionFromWheel(t *testing.T) {
	whlPath := createWheelFixture(t, map[string]string{
		"test_policy/__init__.py": "",
		"test_policy-0.1.0.dist-info/METADATA": `Metadata-Version: 2.1
Name: test-policy
Version: 0.1.0
`,
	})

	version, err := readVersionFromWheel(whlPath)

	require.NoError(t, err)
	assert.Equal(t, "0.1.0", version)
}

func TestRebuildVCSPipSpec(t *testing.T) {
	spec, err := parseVCSPipSpec(
		"git+https://github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v0#subdirectory=policies/prompt-compressor",
	)
	require.NoError(t, err)

	rebuilt := rebuildVCSPipSpec(spec, "policies/prompt-compressor/v0.1.0")

	assert.Equal(
		t,
		"git+https://github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v0.1.0#subdirectory=policies/prompt-compressor",
		rebuilt,
	)
}

func TestResolveVCSVersion_MajorOnly(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := filepath.Join(t.TempDir(), "repo")
	testutils.CreateDir(t, repoDir)
	testutils.WriteFile(t, filepath.Join(repoDir, "README.md"), "fixture")

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "command failed: %v\n%s", args, string(output))
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "test-user")
	runGit("add", ".")
	runGit("commit", "-m", "init")
	runGit("tag", "policies/prompt-compressor/v0.1.0")
	runGit("tag", "policies/prompt-compressor/v0.3.1")
	runGit("tag", "policies/prompt-compressor/v0.3.5")
	runGit("tag", "policies/prompt-compressor/v1.0.0")

	resolved, err := resolveVCSVersion(repoDir, "policies/prompt-compressor/v0", vcsVersionMajorOnly)

	require.NoError(t, err)
	assert.Equal(t, "policies/prompt-compressor/v0.3.5", resolved)
}

func TestResolveVCSVersion_MinorOnly(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := filepath.Join(t.TempDir(), "repo")
	testutils.CreateDir(t, repoDir)
	testutils.WriteFile(t, filepath.Join(repoDir, "README.md"), "fixture")

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "command failed: %v\n%s", args, string(output))
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "test-user")
	runGit("add", ".")
	runGit("commit", "-m", "init")
	runGit("tag", "policies/prompt-compressor/v1.1.0")
	runGit("tag", "policies/prompt-compressor/v1.1.5")
	runGit("tag", "policies/prompt-compressor/v1.2.0")

	resolved, err := resolveVCSVersion(repoDir, "policies/prompt-compressor/v1.1", vcsVersionMinorOnly)

	require.NoError(t, err)
	assert.Equal(t, "policies/prompt-compressor/v1.1.5", resolved)
}

func createWheelFixture(t *testing.T, files map[string]string) string {
	t.Helper()

	whlPath := filepath.Join(t.TempDir(), "fixture.whl")
	whlFile, err := os.Create(whlPath)
	require.NoError(t, err)

	zw := zip.NewWriter(whlFile)
	for name, content := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, zw.Close())
	require.NoError(t, whlFile.Close())

	return whlPath
}
