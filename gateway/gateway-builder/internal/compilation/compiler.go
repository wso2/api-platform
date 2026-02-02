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
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/errors"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

// CompileBinary compiles the policy engine binary with all discovered policies
func CompileBinary(srcDir string, options *types.CompilationOptions) error {
	slog.Info("Starting compilation phase", "phase", "compilation")
	slog.Debug("Compilation options",
		"outputPath", options.OutputPath,
		"enableUPX", options.EnableUPX,
		"cgoEnabled", options.CGOEnabled,
		"targetOS", options.TargetOS,
		"targetArch", options.TargetArch,
		"phase", "compilation")

	// Step 1: go mod download
	if err := runGoModDownload(srcDir); err != nil {
		return errors.NewCompilationError("go mod download failed", err)
	}

	// Step 2: go mod tidy
	if err := runGoModTidy(srcDir); err != nil {
		return errors.NewCompilationError("go mod tidy failed", err)
	}

	// Step 3: Compile binary
	if err := runGoBuild(srcDir, options); err != nil {
		return errors.NewCompilationError("go build failed", err)
	}

	// Step 4: Optional UPX compression
	if options.EnableUPX {
		if err := runUPXCompression(options.OutputPath); err != nil {
			// UPX failure is non-fatal
			slog.Warn("UPX compression failed",
				"error", err,
				"phase", "compilation")
		}
	}

	slog.Info("Binary compiled successfully",
		"path", options.OutputPath,
		"phase", "compilation")
	return nil
}

// runGoModDownload downloads module dependencies
func runGoModDownload(srcDir string) error {
	slog.Info("Running go mod download",
		"step", "mod-download",
		"phase", "compilation")

	// Debug: log go.mod path
	goModPath := filepath.Join(srcDir, "go.mod")
	slog.Debug("go mod download", "srcDir", srcDir, "goModPath", goModPath)

	cmd := exec.Command("go", "mod", "download")
	cmd.Dir = srcDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// On error, print the go.mod file for debugging
		printGoModForDebug(goModPath)
		return fmt.Errorf("go mod download failed: %w", err)
	}

	return nil
}

// runGoModTidy tidies module dependencies
func runGoModTidy(srcDir string) error {
	slog.Info("Running go mod tidy",
		"step", "mod-tidy",
		"phase", "compilation")

	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = srcDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	return nil
}

// runGoBuild compiles the Go binary
func runGoBuild(srcDir string, options *types.CompilationOptions) error {
	slog.Info("Running go build (static binary)",
		"step", "build",
		"phase", "compilation",
		"output", options.OutputPath)

	// Ensure output directory exists
	outputDir := filepath.Dir(options.OutputPath)
	slog.Debug("Creating output directory", "dir", outputDir, "phase", "compilation")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build arguments
	args := []string{"build"}

	// Add coverage instrumentation if enabled
	if options.EnableCoverage {
		slog.Info("Building with coverage instrumentation enabled",
			"step", "build",
			"phase", "compilation")
		args = append(args, "-cover")
	}

	// Add build tags
	if len(options.BuildTags) > 0 {
		tags := ""
		for i, tag := range options.BuildTags {
			if i > 0 {
				tags += ","
			}
			tags += tag
		}
		args = append(args, "-tags", tags)
	}

	// Add ldflags
	if options.LDFlags != "" {
		args = append(args, "-ldflags", options.LDFlags)
	}

	// Add output path
	args = append(args, "-o", options.OutputPath)

	// Add main package (cmd/policy-engine)
	args = append(args, "./cmd/policy-engine")

	slog.Debug("Build command", "args", args, "dir", srcDir, "phase", "compilation")

	// Create command
	cmd := exec.Command("go", args...)
	cmd.Dir = srcDir

	// Set environment for static binary
	cmd.Env = os.Environ()
	if options.CGOEnabled {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=1")
	} else {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	}
	if options.TargetOS != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("GOOS=%s", options.TargetOS))
	}
	if options.TargetArch != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("GOARCH=%s", options.TargetArch))
	}

	slog.Debug("Build environment",
		"CGO_ENABLED", options.CGOEnabled,
		"GOOS", options.TargetOS,
		"GOARCH", options.TargetArch,
		"phase", "compilation")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	return nil
}

// runUPXCompression compresses the binary with UPX
func runUPXCompression(binaryPath string) error {
	slog.Info("Running UPX compression (optional)",
		"step", "upx",
		"phase", "compilation")

	// Check if UPX is available
	if _, err := exec.LookPath("upx"); err != nil {
		return fmt.Errorf("upx not found in PATH")
	}

	cmd := exec.Command("upx", "--best", "--lzma", binaryPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("upx compression failed: %w", err)
	}

	slog.Info("Binary compressed with UPX",
		"path", binaryPath,
		"phase", "compilation")
	return nil
}

// printGoModForDebug prints the go.mod file contents for debugging
func printGoModForDebug(goModPath string) {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		slog.Error("Failed to read go.mod for debugging", "path", goModPath, "error", err)
		return
	}

	slog.Error("go.mod contents for debugging",
		"path", goModPath,
		"size", len(content),
		"content", string(content))
}
