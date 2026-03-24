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

package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-builder/internal/buildfile"
	"github.com/wso2/api-platform/gateway/gateway-builder/internal/compilation"
	"github.com/wso2/api-platform/gateway/gateway-builder/internal/discovery"
	"github.com/wso2/api-platform/gateway/gateway-builder/internal/docker"
	"github.com/wso2/api-platform/gateway/gateway-builder/internal/policyengine"
	"github.com/wso2/api-platform/gateway/gateway-builder/internal/validation"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/errors"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/fsutil"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

const (
	DefaultBuildFile            = "build.yaml"
	DefaultSystemBuildLockFile  = "system-build-lock.yaml"
	DefaultOutputDir            = "output"
	DefaultPolicyEngineSrc      = "/api-platform/gateway/gateway-runtime/policy-engine"
)

// Version information (set via ldflags during build)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func main() {
	defaultGatewayControllerBaseImage := "ghcr.io/wso2/api-platform/gateway-controller:" + Version
	defaultGatewayRuntimeBaseImage := "ghcr.io/wso2/api-platform/gateway-runtime:" + Version

	// Parse command-line flags
	buildFilePath := flag.String("build-file", DefaultBuildFile, "Path to build file")
	systemBuildLockPath := flag.String("system-build-lock", DefaultSystemBuildLockFile, "Path to system build lock file")
	policyEngineSrc := flag.String("policy-engine-src", DefaultPolicyEngineSrc, "Path to policy-engine runtime source directory")
	outputDir := flag.String("out-dir", DefaultOutputDir, "Output directory for generated Dockerfiles and artifacts")

	// Base image configuration
	gatewayControllerBaseImage := flag.String("gateway-controller-base-image", defaultGatewayControllerBaseImage,
		"Base image for gateway controller to extend (used in generated Dockerfile)")
	gatewayRuntimeBaseImage := flag.String("gateway-runtime-base-image", defaultGatewayRuntimeBaseImage,
		"Base gateway runtime image (used in generated Dockerfile)")

	// Logging flags
	logFormat := flag.String("log-format", "json", "Log format: text or json")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")
	flag.Parse()

	// Setup logging
	initLogger(*logFormat, *logLevel)

	// Resolve paths to absolute paths
	absBuildFilePath, err := filepath.Abs(*buildFilePath)
	if err != nil {
		slog.Error("Failed to resolve build file path", "path", *buildFilePath, "error", err)
		os.Exit(1)
	}
	buildFilePath = &absBuildFilePath

	var absSystemBuildLockPath string
	if *systemBuildLockPath != "" {
		absSystemBuildLockPath, err = filepath.Abs(*systemBuildLockPath)
		if err != nil {
			slog.Error("Failed to resolve system build lock path", "path", *systemBuildLockPath, "error", err)
			os.Exit(1)
		}
		systemBuildLockPath = &absSystemBuildLockPath
	}

	absPolicyEngineSrc, err := filepath.Abs(*policyEngineSrc)
	if err != nil {
		slog.Error("Failed to resolve policy-engine-src path", "path", *policyEngineSrc, "error", err)
		os.Exit(1)
	}
	policyEngineSrc = &absPolicyEngineSrc

	absOutputDir, err := filepath.Abs(*outputDir)
	if err != nil {
		slog.Error("Failed to resolve output directory path", "path", *outputDir, "error", err)
		os.Exit(1)
	}
	outputDir = &absOutputDir

	logFields := []any{
		"version", Version,
		"git_commit", GitCommit,
		"build_date", BuildDate,
		"build_file", *buildFilePath,
		"system_build_lock", *systemBuildLockPath,
	}
	slog.Info("Policy Builder starting", logFields...)

	var outBuildInfoPath string

	// Phase 1: Discovery
	slog.Info("Starting Phase 1: Discovery", "phase", "discovery")

	// Discover policies from build file
	policies, err := discovery.DiscoverPoliciesFromBuildFile(*buildFilePath, "")
	if err != nil {
		errors.FatalError(err)
	}
	slog.Info("Loaded build file",
		"count", len(policies),
		"phase", "discovery")

	// Discover system policies from system build lock if provided
	if *systemBuildLockPath != "" {
		systemPolicies, err := discovery.DiscoverPoliciesFromBuildFile(absSystemBuildLockPath, "")
		if err != nil {
			errors.FatalError(err)
		}
		slog.Info("Loaded system build lock",
			"count", len(systemPolicies),
			"phase", "discovery")
		// Merge system policies with regular policies
		policies = append(policies, systemPolicies...)
		slog.Info("Total policies after merging",
			"count", len(policies),
			"phase", "discovery")
	} else {
		slog.Info("No system build lock provided; skipping system policies", "phase", "discovery")
	}

	// Print discovered policies
	for i, p := range policies {
		slog.Info("Discovered policy",
			"index", i+1,
			"name", p.Name,
			"version", p.Version,
			"path", p.Path)
	}

	// Phase 2: Validation
	slog.Info("Starting Phase 2: Validation", "phase", "validation")
	validationResult, err := validation.ValidatePolicies(policies)
	if err != nil {
		fmt.Println(validation.FormatValidationErrors(validationResult))
		errors.FatalError(err)
	}
	slog.Info("All policies validated successfully", "phase", "validation")

	// Phase 3: Code Generation
	slog.Info("Starting Phase 3: Code Generation", "phase", "generation")
	if err := policyengine.GenerateCode(*policyEngineSrc, policies); err != nil {
		errors.FatalError(err)
	}

	// Phase 4: Compilation
	slog.Info("Starting Phase 4: Compilation", "phase", "compilation")

	// Read version information from environment variables (set by Dockerfile)
	// Fall back to builder's own version if not set
	policyEngineVersion := os.Getenv("VERSION")
	if policyEngineVersion == "" {
		policyEngineVersion = Version
	}
	policyEngineGitCommit := os.Getenv("GIT_COMMIT")
	if policyEngineGitCommit == "" {
		policyEngineGitCommit = GitCommit
	}
	// Build date can come from environment or use current timestamp
	buildDateStr := os.Getenv("BUILD_DATE")
	var buildTimestamp time.Time
	if buildDateStr != "" {
		// Try to parse the build date
		parsedTime, err := time.Parse(time.RFC3339, buildDateStr)
		if err == nil {
			buildTimestamp = parsedTime
		} else {
			buildTimestamp = time.Now().UTC()
		}
	} else {
		buildTimestamp = time.Now().UTC()
	}

	buildMetadata := &types.BuildMetadata{
		Timestamp: buildTimestamp,
		Version:   policyEngineVersion,
		GitCommit: policyEngineGitCommit,
		Policies:  make([]types.PolicyInfo, 0, len(policies)),
	}

	slog.Info("Build metadata for policy engine",
		"version", policyEngineVersion,
		"git_commit", policyEngineGitCommit,
		"build_date", buildTimestamp.Format(time.RFC3339),
		"phase", "compilation")

	for _, p := range policies {
		buildMetadata.Policies = append(buildMetadata.Policies, types.PolicyInfo{
			Name:    p.Name,
			Version: p.Version,
		})
	}

	// Create temp directory for compilation
	tempDir, err := os.MkdirTemp("", "policy-engine-build-*")
	if err != nil {
		errors.FatalError(errors.NewCompilationError("failed to create temp directory", err))
	}
	defer os.RemoveAll(tempDir)

	policyEngineBin := filepath.Join(tempDir, "policy-engine")
	compileOpts := compilation.BuildOptions(policyEngineBin, buildMetadata)

	if err := compilation.CompileBinary(*policyEngineSrc, compileOpts); err != nil {
		errors.FatalError(err)
	}

	// Phase 5: Dockerfile Generation
	slog.Info("Starting Phase 5: Dockerfile Generation", "phase", "dockerfile-generation")

	dockerfileGenerator := &docker.DockerfileGenerator{
		PolicyEngineBin:            compileOpts.OutputPath,
		Policies:                   policies,
		OutputDir:                  *outputDir,
		GatewayControllerBaseImage: *gatewayControllerBaseImage,
		GatewayRuntimeBaseImage:    *gatewayRuntimeBaseImage,
		BuilderVersion:             Version,
	}

	generateResult, err := dockerfileGenerator.GenerateAll()
	if err != nil || !generateResult.Success {
		for _, e := range generateResult.Errors {
			slog.Error("Dockerfile generation error", "error", e)
		}
		errors.FatalError(errors.NewDockerError("Dockerfile generation failed", err))
	}

	// Phase 6: Build Info Generation
	slog.Info("Starting Phase 6: Build Info Generation", "phase", "build-info")

	buildInfo := buildfile.CreateBuildInfo(Version, policies, *outputDir)

	outBuildInfoPath = filepath.Join(*outputDir, "build-info.json")
	if err := buildInfo.WriteToFile(outBuildInfoPath); err != nil {
		slog.Error("Failed to write build info file", "error", err)
		errors.FatalError(errors.NewGenerationError("failed to write build info", err))
	}

	slog.Info("Build info written", "path", outBuildInfoPath)

	// Print success summary
	printDockerfileGenerationSummary(generateResult, buildInfo, outBuildInfoPath)

	if err := buildfile.WriteBuildLockWithVersions(*buildFilePath, policies); err != nil {
		slog.Error("Failed to write build lock file with versions", "error", err)
	} else {
		buildLockPath := filepath.Join(filepath.Dir(*buildFilePath), "build-lock.yaml")
		slog.Info("Build lock file generated with versions", "path", buildLockPath)
		gcBuildLockDst := filepath.Join(*outputDir, "gateway-controller", "build-lock.yaml")
		if err := fsutil.CopyFile(buildLockPath, gcBuildLockDst); err != nil {
			errors.FatalError(errors.NewGenerationError("failed to copy build-lock.yaml into gateway-controller build context", err))
		}
		slog.Info("Copied build-lock.yaml into gateway-controller build context successfully", "dst", gcBuildLockDst)
	}
}

// initLogger sets up the slog logger based on format and level
func initLogger(format, level string) {
	// Determine log level
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	// Create handler based on format
	var handler slog.Handler
	handlerOpts := &slog.HandlerOptions{
		Level: logLevel,
	}

	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, handlerOpts)
	} else {
		// Text handler with custom formatting for cleaner output
		handlerOpts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
			// Remove time for cleaner CLI output (CI/CD can use JSON if needed)
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		}
		handler = slog.NewTextHandler(os.Stdout, handlerOpts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// printDockerfileGenerationSummary displays the Dockerfile generation summary
func printDockerfileGenerationSummary(result *docker.GenerateResult, buildInfo *buildfile.BuildInfo, buildInfoPath string) {
	slog.Info("Dockerfile generation completed successfully", "phase", "complete")

	fmt.Println("\n========================================")
	fmt.Println("Gateway Dockerfiles Generated")
	fmt.Println("========================================")
	fmt.Println("\nGenerated Dockerfiles:")
	fmt.Printf("  1. Gateway Runtime:    %s\n", result.GatewayRuntimeDockerfile)
	fmt.Printf("  2. Gateway Controller: %s\n", result.GatewayControllerDockerfile)

	fmt.Printf("Build Info: %s\n", buildInfoPath)

	fmt.Println("\nBuild Info:")
	infoJSON, err := buildInfo.ToJSON()
	if err != nil {
		slog.Error("Failed to convert build info to JSON", "error", err)
	} else {
		fmt.Println(infoJSON)
	}
}
