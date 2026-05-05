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

// Package compilation provides functions to compile the event-gateway binary
// after policies have been code-generated into it.
package compilation

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Options controls how the event-gateway binary is compiled.
type Options struct {
	// OutputPath is the absolute path for the compiled binary.
	OutputPath string
	// LDFlags are the linker flags passed via -ldflags.
	LDFlags string
	// GCFlags are the compiler flags passed via -gcflags (e.g. "all=-N -l" for debug builds).
	GCFlags string
	// TargetArch is the GOARCH for the build (defaults to runtime.GOARCH).
	TargetArch string
	// CGOEnabled controls CGO_ENABLED (defaults to "0").
	CGOEnabled bool
}

// DefaultOptions creates sensible compilation options for the event-gateway.
func DefaultOptions(outputPath, version, gitCommit, buildDate string) *Options {
	targetArch := os.Getenv("TARGETARCH")
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}

	ldflags := fmt.Sprintf(
		"-X main.Version=%s -X main.GitCommit=%s -X main.BuildDate=%s",
		version, gitCommit, buildDate,
	)

	return &Options{
		OutputPath: outputPath,
		LDFlags:    ldflags,
		TargetArch: targetArch,
		CGOEnabled: false,
	}
}

// CompileBinary compiles the event-gateway binary from srcDir.
// srcDir should be the root of the event-gateway/gateway-runtime module.
func CompileBinary(srcDir string, opts *Options) error {
	slog.Info("Compiling event-gateway binary",
		"outputPath", opts.OutputPath,
		"targetArch", opts.TargetArch)

	if err := runGoModDownload(srcDir); err != nil {
		return fmt.Errorf("go mod download failed: %w", err)
	}

	if err := runGoModTidy(srcDir); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	if err := runGoBuild(srcDir, opts); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	slog.Info("event-gateway binary compiled", "path", opts.OutputPath)
	return nil
}

func runGoModDownload(srcDir string) error {
	cmd := exec.Command("go", "mod", "download")
	cmd.Dir = srcDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod download: %w", err)
	}
	return nil
}

func runGoModTidy(srcDir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = srcDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy: %w", err)
	}
	return nil
}

func runGoBuild(srcDir string, opts *Options) error {
	args := []string{"build"}
	if opts.GCFlags != "" {
		args = append(args, "-gcflags", opts.GCFlags)
	}
	args = append(args,
		"-ldflags", opts.LDFlags,
		"-o", opts.OutputPath,
		"./cmd/event-gateway",
	)

	cmd := exec.Command("go", args...)
	cmd.Dir = srcDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	env := append(os.Environ(),
		"GOOS=linux",
		fmt.Sprintf("GOARCH=%s", opts.TargetArch),
	)
	if opts.CGOEnabled {
		env = append(env, "CGO_ENABLED=1")
	} else {
		env = append(env, "CGO_ENABLED=0")
	}
	cmd.Env = env

	slog.Info("Running go build",
		"args", strings.Join(args, " "),
		"GOARCH", opts.TargetArch)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build: %w", err)
	}
	return nil
}
