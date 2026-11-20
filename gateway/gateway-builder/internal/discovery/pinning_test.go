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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-builder/internal/testutils"
)

const (
	testModulePath = "github.com/example/policies/pinned"
	testModuleSum  = "h1:2YdNqRZ0DqUAXN1YoMm3UZ0fYtnb3wE7T8Yq1mQf7ac="
	otherModuleSum = "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	testCommitSHA  = "5d9c02f471e8a388f99294ce7d09bf11b131e26c"
	testWheelHash  = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
)

func writeBuildFile(t *testing.T, policies string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "build.yaml")
	testutils.WriteFile(t, path, "version: v1\npolicies:\n"+policies)
	return path
}

// stubGoEnv replaces the 'go env' lookup for the duration of a test.
func stubGoEnv(t *testing.T, env map[string]string) {
	t.Helper()
	orig := readGoEnv
	readGoEnv = func(keys ...string) (map[string]string, error) { return env, nil }
	t.Cleanup(func() { readGoEnv = orig })
}

// stubGoModDownload replaces 'go mod download -json' with a fixed response.
func stubGoModDownload(t *testing.T, path, version, dir, sum string) *int {
	t.Helper()
	calls := 0
	orig := runGoModDownload
	runGoModDownload = func(ctx context.Context, gomodule string) ([]byte, []byte, error) {
		calls++
		out, err := json.Marshal(map[string]string{"Path": path, "Version": version, "Dir": dir, "Sum": sum})
		return out, nil, err
	}
	t.Cleanup(func() { runGoModDownload = orig })
	return &calls
}

// createModuleCacheDir creates a directory shaped like a downloaded Go policy module.
func createModuleCacheDir(t *testing.T, name, version string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name+"@"+version)
	testutils.CreateDir(t, dir)
	testutils.CreatePolicyDefinitionYAML(t, dir, name, version)
	testutils.WriteGoMod(t, dir, testModulePath)
	testutils.WriteFile(t, filepath.Join(dir, "policy.go"), "package pinned\n")
	return dir
}

// ==== Go module version pinning ====

func TestParsePinnedGoModule_AcceptsExactVersions(t *testing.T) {
	cases := map[string]string{
		"github.com/example/p@v1.4.0":                                    "v1.4.0",
		"github.com/example/p@v0.0.0-20260101000000-0123456789ab":        "v0.0.0-20260101000000-0123456789ab",
		"github.com/example/p@v1.2.4-0.20260101000000-0123456789ab":      "v1.2.4-0.20260101000000-0123456789ab",
		"github.com/example/p@v1.0.0-rc.1":                               "v1.0.0-rc.1",
		"github.com/example/p/v2@v2.1.0":                                 "v2.1.0",
		"github.com/example/p@v2.0.0+incompatible":                       "v2.0.0+incompatible",
		"github.com/wso2/gateway-controllers/policies/basic-auth@v1.1.0": "v1.1.0",
	}
	for ref, wantVersion := range cases {
		t.Run(ref, func(t *testing.T) {
			_, version, err := ParsePinnedGoModule(ref)
			require.NoError(t, err)
			assert.Equal(t, wantVersion, version)
		})
	}
}

func TestParsePinnedGoModule_RejectsFloatingVersions(t *testing.T) {
	for _, ref := range []string{
		"github.com/example/p@v1",                                       // major only
		"github.com/example/p@v1.4",                                     // minor only
		"github.com/example/p@latest",                                   // query
		"github.com/example/p@upgrade",                                  // query
		"github.com/example/p@patch",                                    // query
		"github.com/example/p@main",                                     // branch
		"github.com/example/p@>=v1.0.0",                                 // range query
		"github.com/example/p@0123456789abcdef0123456789abcdef01234567", // raw commit, not a pseudo-version
		"github.com/example/p",                                          // no version at all
		"github.com/example/p@",                                         // empty version
		"github.com/example/p/v2@v1.0.0",                                // version does not match the /v2 path
	} {
		t.Run(ref, func(t *testing.T) {
			_, _, err := ParsePinnedGoModule(ref)
			require.Error(t, err)
		})
	}
}

func TestLoadBuildFile_RejectsMajorOnlyGomodule(t *testing.T) {
	path := writeBuildFile(t, `  - name: basic-auth
    gomodule: github.com/wso2/gateway-controllers/policies/basic-auth@v1
    checksum: `+testModuleSum+`
`)

	_, err := LoadBuildFile(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not pin an exact version")
}

func TestLoadBuildFile_RequiresGomoduleChecksum(t *testing.T) {
	path := writeBuildFile(t, `  - name: basic-auth
    gomodule: github.com/wso2/gateway-controllers/policies/basic-auth@v1.1.0
`)

	_, err := LoadBuildFile(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum is required for gomodule entries")
}

func TestLoadBuildFile_RejectsMalformedGomoduleChecksum(t *testing.T) {
	for _, sum := range []string{"sha256:abcd", "h1:not-base64!", "h1:AAAA"} {
		t.Run(sum, func(t *testing.T) {
			path := writeBuildFile(t, `  - name: basic-auth
    gomodule: github.com/wso2/gateway-controllers/policies/basic-auth@v1.1.0
    checksum: "`+sum+`"
`)
			_, err := LoadBuildFile(path)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "checksum")
		})
	}
}

func TestLoadBuildFile_RejectsChecksumOnFilePathEntry(t *testing.T) {
	path := writeBuildFile(t, `  - name: local
    filePath: ./local
    checksum: `+testModuleSum+`
`)

	_, err := LoadBuildFile(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum is only used with gomodule and indexed pipPackage entries")
}

func TestLoadBuildFile_AcceptsPinnedGomodule(t *testing.T) {
	path := writeBuildFile(t, `  - name: basic-auth
    gomodule: github.com/wso2/gateway-controllers/policies/basic-auth@v1.1.0
    checksum: `+testModuleSum+`
`)

	bf, err := LoadBuildFile(path)

	require.NoError(t, err)
	assert.Equal(t, testModuleSum, bf.Policies[0].Checksum)
}

// ==== Go module checksum verification ====

func TestDiscoverPoliciesFromBuildFile_PinnedGomodule_HappyPath(t *testing.T) {
	stubGoEnv(t, map[string]string{})
	dir := createModuleCacheDir(t, "pinned", "v1.2.3")
	calls := stubGoModDownload(t, testModulePath, "v1.2.3", dir, testModuleSum)

	path := writeBuildFile(t, `  - name: pinned
    gomodule: `+testModulePath+`@v1.2.3
    checksum: `+testModuleSum+`
`)

	policies, err := DiscoverPoliciesFromBuildFile(path, "")

	require.NoError(t, err)
	require.Len(t, policies, 1)
	assert.Equal(t, 1, *calls)
	assert.Equal(t, testModulePath, policies[0].GoModulePath)
	assert.Equal(t, "v1.2.3", policies[0].GoModuleVersion)
	assert.Equal(t, testModuleSum, policies[0].GoModuleSum)
	assert.False(t, policies[0].IsFilePathEntry)
}

func TestDiscoverPoliciesFromBuildFile_PinnedGomodule_ChecksumMismatch(t *testing.T) {
	stubGoEnv(t, map[string]string{})
	dir := createModuleCacheDir(t, "pinned", "v1.2.3")
	stubGoModDownload(t, testModulePath, "v1.2.3", dir, otherModuleSum)

	path := writeBuildFile(t, `  - name: pinned
    gomodule: `+testModulePath+`@v1.2.3
    checksum: `+testModuleSum+`
`)

	_, err := DiscoverPoliciesFromBuildFile(path, "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
	assert.Contains(t, err.Error(), testModuleSum)
	assert.Contains(t, err.Error(), otherModuleSum)
}

func TestDiscoverPoliciesFromBuildFile_PinnedGomodule_MissingSum(t *testing.T) {
	stubGoEnv(t, map[string]string{})
	dir := createModuleCacheDir(t, "pinned", "v1.2.3")
	stubGoModDownload(t, testModulePath, "v1.2.3", dir, "")

	path := writeBuildFile(t, `  - name: pinned
    gomodule: `+testModulePath+`@v1.2.3
    checksum: `+testModuleSum+`
`)

	_, err := DiscoverPoliciesFromBuildFile(path, "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "did not report a checksum")
}

func TestDiscoverPoliciesFromBuildFile_PinnedGomodule_VersionMismatch(t *testing.T) {
	stubGoEnv(t, map[string]string{})
	dir := createModuleCacheDir(t, "pinned", "v1.2.4")
	stubGoModDownload(t, testModulePath, "v1.2.4", dir, testModuleSum)

	path := writeBuildFile(t, `  - name: pinned
    gomodule: `+testModulePath+`@v1.2.3
    checksum: `+testModuleSum+`
`)

	_, err := DiscoverPoliciesFromBuildFile(path, "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "returned "+testModulePath+"@v1.2.4")
}

func TestDiscoverPoliciesFromBuildFile_RefusesDisabledChecksumDatabase(t *testing.T) {
	stubGoEnv(t, map[string]string{"GOSUMDB": "off"})
	calls := stubGoModDownload(t, testModulePath, "v1.2.3", t.TempDir(), testModuleSum)

	path := writeBuildFile(t, `  - name: pinned
    gomodule: `+testModulePath+`@v1.2.3
    checksum: `+testModuleSum+`
`)

	_, err := DiscoverPoliciesFromBuildFile(path, "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "GOSUMDB=off")
	assert.Equal(t, 0, *calls, "no module may be downloaded with checksum verification disabled")
}

func TestCheckGoChecksumEnv(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{name: "defaults", env: map[string]string{"GOSUMDB": "sum.golang.org"}},
		{name: "sumdb off", env: map[string]string{"GOSUMDB": "off"}, wantErr: "GOSUMDB=off"},
		{name: "insecure flag", env: map[string]string{"GOFLAGS": "-mod=mod -insecure"}, wantErr: "-insecure"},
		{name: "nosumdb only warns", env: map[string]string{"GONOSUMDB": "github.com/example"}},
		{name: "goprivate only warns", env: map[string]string{"GOPRIVATE": "github.com/example/*"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubGoEnv(t, tt.env)
			err := checkGoChecksumEnv([]string{testModulePath})
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestCheckGoChecksumEnv_SkippedWithoutRemoteModules(t *testing.T) {
	orig := readGoEnv
	readGoEnv = func(keys ...string) (map[string]string, error) {
		t.Fatal("go env must not be consulted when no gomodule entries exist")
		return nil, nil
	}
	t.Cleanup(func() { readGoEnv = orig })

	assert.NoError(t, checkGoChecksumEnv(nil))
}

// ==== Python package pinning ====

func TestLoadBuildFile_PipPackagePinning(t *testing.T) {
	tests := []struct {
		name     string
		spec     string
		checksum string
		wantErr  string
	}{
		{
			name: "short URL with commit SHA",
			spec: "github.com/wso2/gateway-controllers/policies/prompt-compressor@" + testCommitSHA,
		},
		{
			name:    "short URL with major-only tag",
			spec:    "github.com/wso2/gateway-controllers/policies/prompt-compressor@v0",
			wantErr: "must pin a full git commit SHA",
		},
		{
			name:    "short URL with exact tag",
			spec:    "github.com/wso2/gateway-controllers/policies/prompt-compressor@v0.9.0",
			wantErr: "must pin a full git commit SHA",
		},
		{
			name:    "short URL with abbreviated SHA",
			spec:    "github.com/wso2/gateway-controllers/policies/prompt-compressor@5d9c02f",
			wantErr: "must pin a full git commit SHA",
		},
		{
			name: "git URL with commit SHA",
			spec: "git+https://github.com/wso2/gateway-controllers.git@" + testCommitSHA + "#subdirectory=policies/prompt-compressor",
		},
		{
			name:    "git URL with tag",
			spec:    "git+https://github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v0.9.0#subdirectory=policies/prompt-compressor",
			wantErr: "must pin a full git commit SHA",
		},
		{
			name:     "git URL with redundant checksum",
			spec:     "git+https://github.com/wso2/gateway-controllers.git@" + testCommitSHA,
			checksum: testWheelHash,
			wantErr:  "checksum is not used for git references",
		},
		{
			name:     "indexed exact version with checksum",
			spec:     "my-policy==1.2.3",
			checksum: testWheelHash,
		},
		{
			name:    "indexed exact version without checksum",
			spec:    "my-policy==1.2.3",
			wantErr: "checksum is required for indexed pipPackage entries",
		},
		{
			name:     "indexed version range",
			spec:     "my-policy~=1.0",
			checksum: testWheelHash,
			wantErr:  "uses a version range",
		},
		{
			name:     "indexed with malformed checksum",
			spec:     "my-policy==1.2.3",
			checksum: "h1:AAAA",
			wantErr:  "must start with \"sha256:\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := "  - name: p\n    pipPackage: \"" + tt.spec + "\"\n"
			if tt.checksum != "" {
				entry += "    checksum: \"" + tt.checksum + "\"\n"
			}
			_, err := LoadBuildFile(writeBuildFile(t, entry))
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestExpandShortURL_CommitSHA(t *testing.T) {
	expanded, err := expandShortURL("github.com/wso2/gateway-controllers/policies/prompt-compressor@" + testCommitSHA)

	require.NoError(t, err)
	assert.Equal(t,
		"git+https://github.com/wso2/gateway-controllers.git@"+testCommitSHA+"#subdirectory=policies/prompt-compressor",
		expanded)
}

// createLocalPackageIndex builds a PEP 503 "simple" index on disk holding one
// wheel and returns the index URL and the wheel's sha256.
func createLocalPackageIndex(t *testing.T) (indexURL, checksum string) {
	t.Helper()

	whl := createWheelFixture(t, map[string]string{
		"pinned_policy/__init__.py":                   "",
		"pinned_policy/policy.py":                     "def get_policy(metadata, params):\n    return None\n",
		"pinned_policy/policy-definition.yaml":        "name: pinned-policy\nversion: v1.0.0\n",
		"pinned_policy-1.0.0.dist-info/METADATA":      "Metadata-Version: 2.1\nName: pinned-policy\nVersion: 1.0.0\n\n",
		"pinned_policy-1.0.0.dist-info/WHEEL":         "Wheel-Version: 1.0\nGenerator: test\nRoot-Is-Purelib: true\nTag: py3-none-any\n",
		"pinned_policy-1.0.0.dist-info/top_level.txt": "pinned_policy\n",
		"pinned_policy-1.0.0.dist-info/RECORD":        "",
	})
	data, err := os.ReadFile(whl)
	require.NoError(t, err)
	digest := sha256.Sum256(data)

	root := t.TempDir()
	pkgDir := filepath.Join(root, "simple", "pinned-policy")
	testutils.CreateDir(t, pkgDir)
	wheelName := "pinned_policy-1.0.0-py3-none-any.whl"
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, wheelName), data, 0o644))
	testutils.WriteFile(t, filepath.Join(pkgDir, "index.html"),
		fmt.Sprintf("<!DOCTYPE html><html><body><a href=\"%s\">%s</a></body></html>\n", wheelName, wheelName))

	return "file://" + filepath.ToSlash(filepath.Join(root, "simple")) + "/", "sha256:" + hex.EncodeToString(digest[:])
}

func requirePip(t *testing.T) {
	t.Helper()
	if exe, _ := resolvePipExecutable(); exe == "" {
		t.Skip("pip not available")
	}
}

func TestFetchPinnedPipPackage_IndexedChecksumMatches(t *testing.T) {
	requirePip(t)
	indexURL, checksum := createLocalPackageIndex(t)

	info, err := FetchPinnedPipPackage("pinned-policy==1.0.0@"+indexURL, checksum)

	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(info.Dir)) })
	assert.Equal(t, "1.0.0", info.Version)
	assert.Equal(t, checksum, info.Checksum)
	assert.Equal(t, "pinned_policy", info.TopLevelModule)
}

func TestFetchPinnedPipPackage_IndexedChecksumMismatch(t *testing.T) {
	requirePip(t)
	indexURL, _ := createLocalPackageIndex(t)

	_, err := FetchPinnedPipPackage("pinned-policy==1.0.0@"+indexURL, testWheelHash)

	require.Error(t, err)
	assert.True(t, strings.Contains(strings.ToUpper(err.Error()), "HASH"), "expected a hash error, got: %v", err)
}

func TestFetchPinnedPipPackage_ChecksumRequiresExactVersion(t *testing.T) {
	_, err := FetchPinnedPipPackage("pinned-policy~=1.0", testWheelHash)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exact '==' version")
}
