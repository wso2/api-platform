package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/policy-engine/policy-builder/internal/compilation"
	"github.com/policy-engine/policy-builder/internal/discovery"
	"github.com/policy-engine/policy-builder/internal/docker"
	"github.com/policy-engine/policy-builder/internal/generation"
	"github.com/policy-engine/policy-builder/internal/manifest"
	"github.com/policy-engine/policy-builder/internal/validation"
	"github.com/policy-engine/policy-builder/pkg/errors"
	"github.com/policy-engine/policy-builder/pkg/types"
)

const (
	DefaultManifestFile    = "policy.yaml"
	DefaultPolicyEngineSrc = "/workspace/policy-engine"

	// Policy Engine (builds from scratch - no base image)
	DefaultPolicyEngineOutputImage = "wso2/api-platform-policy-engine"

	// Gateway Controller (extends base image)
	DefaultGatewayControllerBaseImage   = "wso2/api-platform-gateway-controller:v1.0.0-m4"
	DefaultGatewayControllerOutputImage = "wso2/api-platform-gateway-controller"

	// Router (tags existing image)
	DefaultRouterBaseImage   = "wso2/api-platform-gateway-router:v1.0.0-m4"
	DefaultRouterOutputImage = "wso2/api-platform-gateway-router"

	// Image tag applied to all output images
	DefaultImageTag = "latest"

	// Build architecture (only arm64 and amd64 supported)
	DefaultBuildArch = "amd64"

	BuilderVersion = "v1.0.0"
)

func main() {
	// Parse command-line flags
	manifestPath := flag.String("manifest", DefaultManifestFile, "Path to policy manifest file")
	policyEngineSrc := flag.String("policy-engine-src", DefaultPolicyEngineSrc, "Path to policy-engine runtime source directory")

	// Image naming flags
	// Policy Engine (builds from scratch)
	policyEngineOutputImage := flag.String("policy-engine-output-image", DefaultPolicyEngineOutputImage,
		"Output image name for policy engine (repository path without tag)")

	// Gateway Controller (extends base image)
	gatewayControllerBaseImage := flag.String("gateway-controller-base-image", DefaultGatewayControllerBaseImage,
		"Base image for gateway controller to extend (must include tag)")
	gatewayControllerOutputImage := flag.String("gateway-controller-output-image", DefaultGatewayControllerOutputImage,
		"Output image name for gateway controller (repository path without tag)")

	// Router (tags existing image)
	routerBaseImage := flag.String("router-base-image", DefaultRouterBaseImage,
		"Base router image to tag (must include tag)")
	routerOutputImage := flag.String("router-output-image", DefaultRouterOutputImage,
		"Output image name for router (repository path without tag)")

	// Image tag applied to all output images
	imageTag := flag.String("image-tag", DefaultImageTag, "Tag for all output images")

	// Build architecture (only arm64 and amd64 supported)
	buildArch := flag.String("build-arch", DefaultBuildArch, "Target architecture for Docker builds (arm64 or amd64)")

	// Build control flags
	skipDockerBuild := flag.Bool("skip-docker-build", false, "Skip Docker image building (useful for testing)")

	// Logging flags
	logFormat := flag.String("log-format", "json", "Log format: text or json")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")
	flag.Parse()

	// Setup logging
	initLogger(*logFormat, *logLevel)

	// Validate build architecture
	if *buildArch != "arm64" && *buildArch != "amd64" {
		errors.FatalError(errors.NewValidationError(
			fmt.Sprintf("invalid build architecture '%s': only 'arm64' and 'amd64' are supported", *buildArch),
			nil))
	}

	slog.Info("Policy Builder starting",
		"version", BuilderVersion,
		"manifest", *manifestPath,
		"buildArch", *buildArch)

	// Create temp directory for build artifacts
	tempDir, err := os.MkdirTemp("", "gateway-builder-")
	if err != nil {
		errors.FatalError(errors.NewGenerationError("failed to create temp directory", err))
	}
	defer cleanupTempDir(tempDir)

	slog.Debug("Created temp directory", "path", tempDir)

	// Phase 1: Discovery
	slog.Info("Starting Phase 1: Discovery", "phase", "discovery")

	policies, err := discovery.DiscoverPoliciesFromManifest(*manifestPath, "")
	if err != nil {
		errors.FatalError(err)
	}
	slog.Info("Loaded manifest",
		"count", len(policies),
		"phase", "discovery")

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
	if err := generation.GenerateCode(*policyEngineSrc, policies); err != nil {
		errors.FatalError(err)
	}

	// Phase 4: Compilation
	slog.Info("Starting Phase 4: Compilation", "phase", "compilation")

	buildMetadata := &types.BuildMetadata{
		Timestamp:      time.Now().UTC(),
		BuilderVersion: BuilderVersion,
		Policies:       make([]types.PolicyInfo, 0, len(policies)),
	}

	for _, p := range policies {
		buildMetadata.Policies = append(buildMetadata.Policies, types.PolicyInfo{
			Name:    p.Name,
			Version: p.Version,
		})
	}

	binaryPath := filepath.Join(tempDir, "policy-engine")
	compileOpts := compilation.BuildOptions(binaryPath, buildMetadata)

	if err := compilation.CompileBinary(*policyEngineSrc, compileOpts); err != nil {
		errors.FatalError(err)
	}

	// Phase 5: Docker Build (NEW)
	if !*skipDockerBuild {
		// Pre-flight check: Docker availability
		if err := docker.CheckDockerAvailable(); err != nil {
			errors.FatalError(errors.NewDockerError("Docker is not available", err))
		}

		slog.Info("Starting Phase 5: Docker Build", "phase", "docker-build")

		stackBuilder := &docker.StackBuilder{
			TempDir:    tempDir,
			BinaryPath: binaryPath,
			Policies:   policies,

			// Policy Engine
			PolicyEngineOutputImage: *policyEngineOutputImage,

			// Gateway Controller
			GatewayControllerBaseImage:   *gatewayControllerBaseImage,
			GatewayControllerOutputImage: *gatewayControllerOutputImage,

			// Router
			RouterBaseImage:   *routerBaseImage,
			RouterOutputImage: *routerOutputImage,

			// Common
			ImageTag:       *imageTag,
			BuildArch:      *buildArch,
			BuilderVersion: BuilderVersion,
		}

		buildResult, err := stackBuilder.BuildAll()
		if err != nil || !buildResult.Success {
			for _, e := range buildResult.Errors {
				slog.Error("Docker build error", "error", e)
			}
			errors.FatalError(errors.NewDockerError("Docker build failed", err))
		}

		// Phase 6: Manifest Generation (NEW)
		slog.Info("Starting Phase 6: Manifest Generation", "phase", "manifest")

		imageManifest := manifest.ImageManifest{
			PolicyEngine:      buildResult.PolicyEngineImage,
			GatewayController: buildResult.GatewayControllerImage,
			Router:            buildResult.RouterImage,
		}

		buildManifest := manifest.CreateManifest(BuilderVersion, policies, imageManifest)

		// Print success summary with manifest
		printDockerBuildSummary(buildResult, buildManifest)
	} else {
		slog.Info("Skipping Docker build (--skip-docker-build=true)")
		fmt.Println("\nDocker build skipped. Binary available at:", binaryPath)
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

// cleanupTempDir removes the temporary directory
func cleanupTempDir(tempDir string) {
	slog.Debug("Cleaning up temp directory", "path", tempDir)
	if err := os.RemoveAll(tempDir); err != nil {
		slog.Warn("Failed to cleanup temp directory", "error", err)
	}
}

// printDockerBuildSummary displays the Docker build summary
func printDockerBuildSummary(result *docker.BuildResult, buildManifest *manifest.Manifest) {
	slog.Info("Build completed successfully", "phase", "complete")

	fmt.Println("\n========================================")
	fmt.Println("Gateway Stack Build Complete")
	fmt.Println("========================================")
	fmt.Println("\nBuilt Images:")
	fmt.Printf("  1. Policy Engine:      %s\n", result.PolicyEngineImage)
	fmt.Printf("  2. Gateway Controller: %s\n", result.GatewayControllerImage)
	fmt.Printf("  3. Router:             %s\n", result.RouterImage)

	// Print manifest as JSON
	fmt.Println("\nBuild Manifest:")
	manifestJSON, err := buildManifest.ToJSON()
	if err != nil {
		slog.Error("Failed to convert manifest to JSON", "error", err)
	} else {
		fmt.Println(manifestJSON)
	}

	fmt.Println("\nNext Steps:")
	fmt.Println("  1. Deploy the gateway stack:")
	fmt.Println("     docker-compose up -d")
	fmt.Println("  2. Verify images:")
	fmt.Println("     docker images | grep wso2/api-platform")
	fmt.Println()
}
