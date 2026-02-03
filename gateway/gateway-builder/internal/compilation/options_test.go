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

package compilation

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

func TestBuildOptions_Default(t *testing.T) {
	// Ensure COVERAGE env is not set
	os.Unsetenv("COVERAGE")

	metadata := &types.BuildMetadata{
		Version:   "v1.0.0",
		GitCommit: "abc123",
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	opts := BuildOptions("/output/binary", metadata)

	assert.Equal(t, "/output/binary", opts.OutputPath)
	assert.False(t, opts.EnableUPX)
	assert.False(t, opts.CGOEnabled)
	assert.Equal(t, "linux", opts.TargetOS)
	assert.Equal(t, runtime.GOARCH, opts.TargetArch)
	assert.False(t, opts.EnableCoverage)
	assert.Contains(t, opts.LDFlags, "-s -w")
	assert.Contains(t, opts.LDFlags, "-X main.Version=v1.0.0")
	assert.Contains(t, opts.LDFlags, "-X main.GitCommit=abc123")
	assert.Contains(t, opts.LDFlags, "-X main.BuildDate=")
}

func TestBuildOptions_WithCoverageEnabled(t *testing.T) {
	// Set COVERAGE env
	os.Setenv("COVERAGE", "true")
	defer os.Unsetenv("COVERAGE")

	metadata := &types.BuildMetadata{
		Version:   "v2.0.0",
		GitCommit: "def456",
		Timestamp: time.Now(),
	}

	opts := BuildOptions("/output/binary", metadata)

	assert.True(t, opts.EnableCoverage)
	// Should NOT contain -s -w when coverage is enabled
	assert.NotContains(t, opts.LDFlags, "-s -w")
	assert.Contains(t, opts.LDFlags, "-X main.Version=v2.0.0")
}

func TestBuildOptions_CoverageEnvCaseInsensitive(t *testing.T) {
	os.Setenv("COVERAGE", "TRUE")
	defer os.Unsetenv("COVERAGE")

	metadata := &types.BuildMetadata{
		Version:   "v1.0.0",
		GitCommit: "abc123",
		Timestamp: time.Now(),
	}

	opts := BuildOptions("/output/binary", metadata)

	assert.True(t, opts.EnableCoverage)
}

func TestBuildOptions_CoverageEnvFalse(t *testing.T) {
	os.Setenv("COVERAGE", "false")
	defer os.Unsetenv("COVERAGE")

	metadata := &types.BuildMetadata{
		Version:   "v1.0.0",
		GitCommit: "abc123",
		Timestamp: time.Now(),
	}

	opts := BuildOptions("/output/binary", metadata)

	assert.False(t, opts.EnableCoverage)
	assert.Contains(t, opts.LDFlags, "-s -w")
}

func TestGenerateLDFlags_WithoutCoverage(t *testing.T) {
	metadata := &types.BuildMetadata{
		Version:   "v1.2.3",
		GitCommit: "deadbeef",
		Timestamp: time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC),
	}

	ldflags := generateLDFlags(metadata, false)

	assert.Contains(t, ldflags, "-s -w")
	assert.Contains(t, ldflags, "-X main.Version=v1.2.3")
	assert.Contains(t, ldflags, "-X main.GitCommit=deadbeef")
	assert.Contains(t, ldflags, "-X main.BuildDate=2025-06-15T14:30:00Z")
}

func TestGenerateLDFlags_WithCoverage(t *testing.T) {
	metadata := &types.BuildMetadata{
		Version:   "v3.0.0-beta",
		GitCommit: "feedface",
		Timestamp: time.Date(2025, 12, 25, 8, 0, 0, 0, time.UTC),
	}

	ldflags := generateLDFlags(metadata, true)

	// Should NOT have -s -w when coverage is enabled
	assert.NotContains(t, ldflags, "-s -w")
	assert.Contains(t, ldflags, "-X main.Version=v3.0.0-beta")
	assert.Contains(t, ldflags, "-X main.GitCommit=feedface")
	assert.Contains(t, ldflags, "-X main.BuildDate=2025-12-25T08:00:00Z")
}

func TestGenerateLDFlags_EmptyMetadata(t *testing.T) {
	metadata := &types.BuildMetadata{
		Version:   "",
		GitCommit: "",
		Timestamp: time.Time{},
	}

	ldflags := generateLDFlags(metadata, false)

	assert.Contains(t, ldflags, "-s -w")
	assert.Contains(t, ldflags, "-X main.Version=")
	assert.Contains(t, ldflags, "-X main.GitCommit=")
}

func TestBuildOptions_OutputPathVariations(t *testing.T) {
	tests := []struct {
		name       string
		outputPath string
	}{
		{"absolute path", "/usr/local/bin/policy-engine"},
		{"relative path", "./build/policy-engine"},
		{"nested path", "/very/deep/nested/path/binary"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("COVERAGE")
			metadata := &types.BuildMetadata{
				Version:   "v1.0.0",
				GitCommit: "abc",
				Timestamp: time.Now(),
			}

			opts := BuildOptions(tt.outputPath, metadata)

			assert.Equal(t, tt.outputPath, opts.OutputPath)
		})
	}
}

func TestBuildOptions_StaticBinaryDefaults(t *testing.T) {
	os.Unsetenv("COVERAGE")

	metadata := &types.BuildMetadata{
		Version:   "v1.0.0",
		GitCommit: "abc",
		Timestamp: time.Now(),
	}

	opts := BuildOptions("/output", metadata)

	// Verify static binary settings
	assert.False(t, opts.CGOEnabled, "CGO should be disabled for static binary")
	assert.Equal(t, "linux", opts.TargetOS, "Target OS should be linux")
	assert.Empty(t, opts.BuildTags, "No build tags by default")
	assert.False(t, opts.EnableUPX, "UPX should be disabled by default")
}
