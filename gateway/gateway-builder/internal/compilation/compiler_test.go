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

package compilation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-builder/internal/testutils"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

func TestCompileBinary_InvalidSrcDir(t *testing.T) {
	options := &types.CompilationOptions{
		OutputPath: "/tmp/output",
		CGOEnabled: false,
		TargetOS:   "linux",
		TargetArch: "amd64",
	}

	err := CompileBinary("/nonexistent/path", options)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "go mod download failed")
}

func TestCompileBinary_NoGoMod(t *testing.T) {
	// Create a temp directory without go.mod
	tmpDir := t.TempDir()

	options := &types.CompilationOptions{
		OutputPath: filepath.Join(tmpDir, "output", "binary"),
		CGOEnabled: false,
		TargetOS:   "linux",
		TargetArch: "amd64",
	}

	err := CompileBinary(tmpDir, options)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "go mod download failed")
}

func TestRunGoModDownload_NoGoMod(t *testing.T) {
	tmpDir := t.TempDir()

	err := runGoModDownload(tmpDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "go mod download failed")
}

func TestRunGoModDownload_ValidModule(t *testing.T) {
	tmpDir := t.TempDir()
	testutils.WriteGoModWithVersion(t, tmpDir, "testmodule", "1.21")

	err := runGoModDownload(tmpDir)

	// Should succeed with a valid go.mod (no dependencies to download)
	assert.NoError(t, err)
}

func TestRunGoModTidy_NoGoMod(t *testing.T) {
	tmpDir := t.TempDir()

	err := runGoModTidy(tmpDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "go mod tidy failed")
}

func TestRunGoModTidy_ValidModule(t *testing.T) {
	tmpDir := t.TempDir()
	testutils.WriteGoModWithVersion(t, tmpDir, "testmodule", "1.21")

	err := runGoModTidy(tmpDir)

	assert.NoError(t, err)
}

func TestRunGoBuild_NoSource(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod but no source files
	testutils.WriteGoModWithVersion(t, tmpDir, "testmodule", "1.21")

	options := &types.CompilationOptions{
		OutputPath: filepath.Join(tmpDir, "output", "binary"),
		CGOEnabled: false,
		TargetOS:   "linux",
		TargetArch: "amd64",
	}

	err := runGoBuild(tmpDir, options)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "go build failed")
}

func TestRunGoBuild_CreatesOutputDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	testutils.WriteGoModWithVersion(t, tmpDir, "testmodule", "1.21")

	// Output dir that doesn't exist yet
	outputDir := filepath.Join(tmpDir, "deep", "nested", "output")
	outputPath := filepath.Join(outputDir, "binary")

	options := &types.CompilationOptions{
		OutputPath: outputPath,
		CGOEnabled: false,
		TargetOS:   "linux",
		TargetArch: "amd64",
	}

	// Will fail because no source, but should create directory first
	_ = runGoBuild(tmpDir, options)

	// Verify directory was created
	_, err := os.Stat(outputDir)
	assert.NoError(t, err, "output directory should be created")
}

func TestRunGoBuild_WithBuildTags(t *testing.T) {
	tmpDir := t.TempDir()
	testutils.WriteGoModWithVersion(t, tmpDir, "testmodule", "1.21")

	options := &types.CompilationOptions{
		OutputPath: filepath.Join(tmpDir, "binary"),
		BuildTags:  []string{"integration", "debug"},
		CGOEnabled: false,
		TargetOS:   "linux",
		TargetArch: "amd64",
	}

	// Will fail but exercises the build tags path
	err := runGoBuild(tmpDir, options)
	assert.Error(t, err) // No source to build
}

func TestRunGoBuild_WithLDFlags(t *testing.T) {
	tmpDir := t.TempDir()
	testutils.WriteGoModWithVersion(t, tmpDir, "testmodule", "1.21")

	options := &types.CompilationOptions{
		OutputPath: filepath.Join(tmpDir, "binary"),
		LDFlags:    "-s -w -X main.Version=test",
		CGOEnabled: false,
		TargetOS:   "linux",
		TargetArch: "amd64",
	}

	err := runGoBuild(tmpDir, options)
	assert.Error(t, err) // No source to build
}

func TestRunGoBuild_WithCoverage(t *testing.T) {
	tmpDir := t.TempDir()
	testutils.WriteGoModWithVersion(t, tmpDir, "testmodule", "1.21")

	options := &types.CompilationOptions{
		OutputPath:     filepath.Join(tmpDir, "binary"),
		EnableCoverage: true,
		CGOEnabled:     false,
		TargetOS:       "linux",
		TargetArch:     "amd64",
	}

	err := runGoBuild(tmpDir, options)
	assert.Error(t, err) // No source to build
}

func TestRunGoBuild_WithCGOEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	testutils.WriteGoModWithVersion(t, tmpDir, "testmodule", "1.21")

	options := &types.CompilationOptions{
		OutputPath: filepath.Join(tmpDir, "binary"),
		CGOEnabled: true,
		TargetOS:   "linux",
		TargetArch: "amd64",
	}

	err := runGoBuild(tmpDir, options)
	assert.Error(t, err) // No source to build
}

func TestRunUPXCompression_NotFound(t *testing.T) {
	// Test when UPX is not in PATH
	// Save original PATH and set empty
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", "/nonexistent")
	defer os.Setenv("PATH", originalPath)

	err := runUPXCompression("/some/binary")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "upx not found")
}

func TestRunUPXCompression_InvalidBinary(t *testing.T) {
	// Skip if UPX is not installed
	if _, err := lookupUPX(); err != nil {
		t.Skip("UPX not installed, skipping test")
	}

	// Create a file that's not a valid binary
	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "fake-binary")
	err := os.WriteFile(fakeBinary, []byte("not a real binary"), 0755)
	require.NoError(t, err)

	err = runUPXCompression(fakeBinary)

	// UPX should fail on invalid binary
	assert.Error(t, err)
}

func TestPrintGoModForDebug_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")

	content := `module example.com/test

go 1.21

require github.com/stretchr/testify v1.8.0
`
	testutils.WriteFile(t, goModPath, content)

	// Should not panic, just logs
	printGoModForDebug(goModPath)
}

func TestPrintGoModForDebug_FileNotFound(t *testing.T) {
	// Should not panic, just logs error
	printGoModForDebug("/nonexistent/go.mod")
}

func TestCompileBinary_FullPipeline_InvalidSource(t *testing.T) {
	tmpDir := t.TempDir()
	testutils.WriteGoModWithVersion(t, tmpDir, "testmodule", "1.21")

	// Create cmd/policy-engine directory but no main.go
	cmdDir := filepath.Join(tmpDir, "cmd", "policy-engine")
	testutils.CreateDir(t, cmdDir)

	options := &types.CompilationOptions{
		OutputPath: filepath.Join(tmpDir, "output", "binary"),
		CGOEnabled: false,
		TargetOS:   "linux",
		TargetArch: "amd64",
		EnableUPX:  false,
	}

	err := CompileBinary(tmpDir, options)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "go build failed")
}

// lookupUPX is a helper to check if UPX is installed
func lookupUPX() (string, error) {
	return lookupPath("upx")
}

func lookupPath(name string) (string, error) {
	path, ok := os.LookupEnv("PATH")
	if !ok {
		return "", os.ErrNotExist
	}
	for _, dir := range filepath.SplitList(path) {
		fullPath := filepath.Join(dir, name)
		if _, statErr := os.Stat(fullPath); statErr == nil {
			return fullPath, nil
		}
	}
	return "", os.ErrNotExist
}
