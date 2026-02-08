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

	"github.com/wso2/api-platform/gateway/gateway-builder/internal/compilation"
	"github.com/wso2/api-platform/gateway/gateway-builder/internal/discovery"
	"github.com/wso2/api-platform/gateway/gateway-builder/internal/docker"
	"github.com/wso2/api-platform/gateway/gateway-builder/internal/manifest"
	"github.com/wso2/api-platform/gateway/gateway-builder/internal/policyengine"
	"github.com/wso2/api-platform/gateway/gateway-builder/internal/validation"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/errors"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

const (
	DefaultManifestFile                 = "policy-manifest.yaml"
	DefaultSystemPolicyManifestLockFile = "system-policy-manifest-lock.yaml"
	DefaultOutputDir                    = "output"
	DefaultPolicyEngineSrc              = "/api-platform/gateway/gateway-runtime/policy-engine"
)

// Version information (set via ldflags during build)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func main() {
	defaultGatewayControllerBaseImage := "ghcr.io/wso2/api-platform/gateway-controller:" + Version
	defaultRouterBaseImage := "ghcr.io/wso2/api-platform/gateway-router:" + Version

	// Parse command-line flags
	manifestPath := flag.String("manifest", DefaultManifestFile, "Path to policy manifest file")
	systemManifestLockPath := flag.String("system-manifest-lock", DefaultSystemPolicyManifestLockFile, "Path to system policy manifest lock file")
	policyEngineSrc := flag.String("policy-engine-src", DefaultPolicyEngineSrc, "Path to policy-engine runtime source directory")
	outputDir := flag.String("out-dir", DefaultOutputDir, "Output directory for generated Dockerfiles and artifacts")

	// Base image configuration
	gatewayControllerBaseImage := flag.String("gateway-controller-base-image", defaultGatewayControllerBaseImage,
		"Base image for gateway controller to extend (used in generated Dockerfile)")
	routerBaseImage := flag.String("router-base-image", defaultRouterBaseImage,
		"Base router image (used in generated Dockerfile)")

	// Logging flags
	logFormat := flag.String("log-format", "json", "Log format: text or json")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")
	flag.Parse()

	// Setup logging
	initLogger(*logFormat, *logLevel)

	// Resolve paths to absolute paths
	absManifestPath, err := filepath.Abs(*manifestPath)
	if err != nil {
		slog.Error("Failed to resolve manifest path", "path", *manifestPath, "error", err)
		os.Exit(1)
	}
	manifestPath = &absManifestPath

	var absSystemManifestLockPath string
	if *systemManifestLockPath != "" {
		absSystemManifestLockPath, err = filepath.Abs(*systemManifestLockPath)
		if err != nil {
			slog.Error("Failed to resolve system manifest lock path", "path", *systemManifestLockPath, "error", err)
			os.Exit(1)
		}
		systemManifestLockPath = &absSystemManifestLockPath
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
		"manifest", *manifestPath,
		"system_manifest_lock", *systemManifestLockPath,
	}
	slog.Info("Policy Builder starting", logFields...)

	var outManifestPath string

	// Phase 1: Discovery
	slog.Info("Starting Phase 1: Discovery", "phase", "discovery")

	// Discover policies from main manifest
	policies, err := discovery.DiscoverPoliciesFromManifest(*manifestPath, "")
	if err != nil {
		errors.FatalError(err)
	}
	slog.Info("Loaded manifest",
		"count", len(policies),
		"phase", "discovery")

	// Discover system policies from system manifest if provided
	if *systemManifestLockPath != "" {
		systemPolicies, err := discovery.DiscoverPoliciesFromManifest(absSystemManifestLockPath, "")
		if err != nil {
			errors.FatalError(err)
		}
		slog.Info("Loaded system manifest",
			"count", len(systemPolicies),
			"phase", "discovery")
		// Merge system policies with regular policies
		policies = append(policies, systemPolicies...)
		slog.Info("Total policies after merging",
			"count", len(policies),
			"phase", "discovery")
	} else {
		slog.Info("No system manifest provided; skipping system policies", "phase", "discovery")
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
		RouterBaseImage:            *routerBaseImage,
	}

	generateResult, err := dockerfileGenerator.GenerateAll()
	if err != nil || !generateResult.Success {
		for _, e := range generateResult.Errors {
			slog.Error("Dockerfile generation error", "error", e)
		}
		errors.FatalError(errors.NewDockerError("Dockerfile generation failed", err))
	}

	// Phase 6: Manifest Generation
	slog.Info("Starting Phase 6: Manifest Generation", "phase", "manifest")

	buildManifest := manifest.CreateManifest(Version, policies, *outputDir)

	// Write manifest to file
	outManifestPath = filepath.Join(*outputDir, "build-manifest.json")
	if err := buildManifest.WriteToFile(outManifestPath); err != nil {
		slog.Error("Failed to write manifest file", "error", err)
		errors.FatalError(errors.NewGenerationError("failed to write manifest", err))
	}

	slog.Info("Build manifest written", "path", outManifestPath)

	// Print success summary with manifest
	printDockerfileGenerationSummary(generateResult, buildManifest, outManifestPath)

	if err := manifest.WriteManifestLockWithVersions(*manifestPath, policies); err != nil {
		slog.Warn("Failed to write policy lock file with versions", "error", err)
	} else {
		slog.Info("Policy lock file generated with versions", "path", filepath.Join(filepath.Dir(*manifestPath), "policy-manifest-lock.yaml"))
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
func printDockerfileGenerationSummary(result *docker.GenerateResult, buildManifest *manifest.Manifest, manifestPath string) {
	slog.Info("Dockerfile generation completed successfully", "phase", "complete")

	fmt.Println("\n========================================")
	fmt.Println("Gateway Dockerfiles Generated")
	fmt.Println("========================================")
	fmt.Println("\nGenerated Dockerfiles:")
	fmt.Printf("  1. Policy Engine:      %s\n", result.PolicyEngineDockerfile)
	fmt.Printf("  2. Gateway Controller: %s\n", result.GatewayControllerDockerfile)
	fmt.Printf("  3. Router:             %s\n", result.RouterDockerfile)

	fmt.Printf("Manifest: %s\n", manifestPath)

	// Print manifest as JSON
	fmt.Println("\nBuild Manifest:")
	manifestJSON, err := buildManifest.ToJSON()
	if err != nil {
		slog.Error("Failed to convert manifest to JSON", "error", err)
	} else {
		fmt.Println(manifestJSON)
	}
}
