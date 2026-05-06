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

// event-gateway-builder compiles user-defined policies into the event-gateway binary
// and generates a Dockerfile to produce a custom event-gateway image.
//
// Usage:
//
//	event-gateway-builder \
//	  -build-file    build.yaml \
//	  -gateway-src   /path/to/event-gateway/gateway-runtime \
//	  -out-dir       output \
//	  -base-image    ghcr.io/wso2/api-platform/event-gateway:<version>
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wso2/api-platform/event-gateway/gateway-builder/internal/compilation"
	"github.com/wso2/api-platform/event-gateway/gateway-builder/internal/discovery"
	"github.com/wso2/api-platform/event-gateway/gateway-builder/internal/docker"
	"github.com/wso2/api-platform/event-gateway/gateway-builder/internal/policyengine"
)

// Version information (set via ldflags during build)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

const (
	DefaultBuildFile  = "build.yaml"
	DefaultOutputDir  = "output"
	DefaultGatewaySrc = "/api-platform/event-gateway/gateway-runtime"
)

func main() {
	defaultBaseImage := "ghcr.io/wso2/api-platform/event-gateway:" + Version

	buildFilePath := flag.String("build-file", DefaultBuildFile, "Path to build.yaml")
	gatewaySrc := flag.String("gateway-src", DefaultGatewaySrc, "Path to event-gateway/gateway-runtime source directory")
	outputDir := flag.String("out-dir", DefaultOutputDir, "Output directory for generated Dockerfile and artifacts")
	baseImage := flag.String("base-image", defaultBaseImage, "Base event-gateway image to extend")
	gcFlags := flag.String("gcflags", "", "Go compiler flags passed via -gcflags (e.g. \"all=-N -l\" for debug builds)")
	generateOnly := flag.Bool("generate-only", false, "Only run code generation (Phase 2); skip compilation and Dockerfile generation. Use for local builds.")
	logFormat := flag.String("log-format", "json", "Log format: text or json")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")
	flag.Parse()

	initLogger(*logFormat, *logLevel)

	slog.Info("Event Gateway Builder starting",
		"version", Version,
		"git_commit", GitCommit,
		"build_date", BuildDate,
		"build_file", *buildFilePath,
		"gateway_src", *gatewaySrc,
	)

	// Resolve to absolute paths
	absBuildFile, err := filepath.Abs(*buildFilePath)
	if err != nil {
		fatalf("failed to resolve build-file path: %v", err)
	}
	absGatewaySrc, err := filepath.Abs(*gatewaySrc)
	if err != nil {
		fatalf("failed to resolve gateway-src path: %v", err)
	}
	absOutputDir, err := filepath.Abs(*outputDir)
	if err != nil {
		fatalf("failed to resolve out-dir path: %v", err)
	}

	// ── Phase 1: Discovery ──────────────────────────────────────────────────
	slog.Info("Phase 1: Discovery")
	policies, err := discovery.DiscoverPoliciesFromBuildFile(absBuildFile)
	if err != nil {
		fatalf("policy discovery failed: %v", err)
	}
	for i, p := range policies {
		slog.Info("Discovered policy", "index", i+1, "name", p.Name, "version", p.Version)
	}

	// ── Phase 2: Code Generation ────────────────────────────────────────────
	slog.Info("Phase 2: Code Generation")
	if err := policyengine.GenerateCode(absGatewaySrc, policies); err != nil {
		fatalf("code generation failed: %v", err)
	}

	// ── Phase 3: Compilation ────────────────────────────────────────────────
	if *generateOnly {
		slog.Info("Phase 3 & 4 skipped (-generate-only flag set). plugin_registry.go and go.mod updated.")
		return
	}
	slog.Info("Phase 3: Compilation")

	buildVersion := os.Getenv("VERSION")
	if buildVersion == "" {
		buildVersion = Version
	}
	buildCommit := os.Getenv("GIT_COMMIT")
	if buildCommit == "" {
		buildCommit = GitCommit
	}
	buildDateStr := os.Getenv("BUILD_DATE")
	if buildDateStr == "" {
		buildDateStr = BuildDate
	}
	if buildDateStr == "" || buildDateStr == "unknown" {
		buildDateStr = time.Now().UTC().Format(time.RFC3339)
	}

	tmpDir, err := os.MkdirTemp("", "event-gateway-build-*")
	if err != nil {
		fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath := filepath.Join(tmpDir, "event-gateway")
	compileOpts := compilation.DefaultOptions(binaryPath, buildVersion, buildCommit, buildDateStr)
	compileOpts.GCFlags = *gcFlags
	if err := compilation.CompileBinary(absGatewaySrc, compileOpts); err != nil {
		fatalf("compilation failed: %v", err)
	}

	// ── Phase 4: Dockerfile Generation ─────────────────────────────────────
	slog.Info("Phase 4: Dockerfile Generation")

	gen := &docker.DockerfileGenerator{
		EventGatewayBin: binaryPath,
		OutputDir:       absOutputDir,
		BaseImage:       *baseImage,
		BuilderVersion:  Version,
	}
	result, err := gen.Generate()
	if err != nil {
		fatalf("Dockerfile generation failed: %v", err)
	}

	printSummary(result, len(policies))
}

func printSummary(result *docker.GenerateResult, policyCount int) {
	fmt.Println("\n========================================")
	fmt.Println("Event Gateway Builder — Build Complete")
	fmt.Println("========================================")
	fmt.Printf("  Dockerfile:       %s\n", result.DockerfilePath)
	fmt.Printf("  Binary artifact:  %s\n", result.EventGatewayBin)
	fmt.Printf("  Output directory: %s\n", result.OutputDir)
	fmt.Printf("  Policies bundled: %d\n", policyCount)
}

func fatalf(format string, args ...any) {
	slog.Error(fmt.Sprintf(format, args...))
	os.Exit(1)
}

func initLogger(format, level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: logLevel}
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		opts.ReplaceAttr = func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		}
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}

func sanitizeLog(s string) string {
	return strings.NewReplacer("\n", "\\n", "\r", "\\r", "\t", "\\t").Replace(s)
}

var _ = sanitizeLog // suppress unused warning
