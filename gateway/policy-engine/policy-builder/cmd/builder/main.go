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
	"github.com/policy-engine/policy-builder/internal/manifest"
	"github.com/policy-engine/policy-builder/internal/policyengine"
	"github.com/policy-engine/policy-builder/internal/validation"
	"github.com/policy-engine/policy-builder/pkg/errors"
	"github.com/policy-engine/policy-builder/pkg/types"
)

const (
	DefaultManifestFile    = "policies.yaml"
	DefaultOutputDir       = "output"
	DefaultPolicyEngineSrc = "policy-engine"

	// Gateway Controller (extends base image)
	DefaultGatewayControllerBaseImage = "wso2/api-platform-gateway-controller:v1.0.0-m4"

	// Router (uses base image)
	DefaultRouterBaseImage = "wso2/api-platform-gateway-router:v1.0.0-m4"

	BuilderVersion = "v1.0.0"
)

func main() {
	// Parse command-line flags
	manifestPath := flag.String("manifest", DefaultManifestFile, "Path to policy manifest file")
	policyEngineSrc := flag.String("policy-engine-src", DefaultPolicyEngineSrc, "Path to policy-engine runtime source directory")
	outputDir := flag.String("out-dir", DefaultOutputDir, "Output directory for generated Dockerfiles and artifacts")

	// Base image configuration
	gatewayControllerBaseImage := flag.String("gateway-controller-base-image", DefaultGatewayControllerBaseImage,
		"Base image for gateway controller to extend (used in generated Dockerfile)")
	routerBaseImage := flag.String("router-base-image", DefaultRouterBaseImage,
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

	slog.Info("Policy Builder starting",
		"version", BuilderVersion,
		"manifest", *manifestPath)

	var outManifestPath string

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
	if err := policyengine.GenerateCode(*policyEngineSrc, policies); err != nil {
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

	policyEngineBin := filepath.Join(*outputDir, "policy-engine", "policy-engine")
	compileOpts := compilation.BuildOptions(policyEngineBin, buildMetadata)

	if err := compilation.CompileBinary(*policyEngineSrc, compileOpts); err != nil {
		errors.FatalError(err)
	}

	// Phase 5: Dockerfile Generation
	slog.Info("Starting Phase 5: Dockerfile Generation", "phase", "dockerfile-generation")

	dockerfileGenerator := &docker.DockerfileGenerator{
		PolicyEngineBin:            policyEngineBin,
		Policies:                   policies,
		OutputDir:                  *outputDir,
		GatewayControllerBaseImage: *gatewayControllerBaseImage,
		RouterBaseImage:            *routerBaseImage,
		BuilderVersion:             BuilderVersion,
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

	buildManifest := manifest.CreateManifest(BuilderVersion, policies, *outputDir)

	// Write manifest to file
	outManifestPath = filepath.Join(*outputDir, "build-manifest.json")
	if err := buildManifest.WriteToFile(outManifestPath); err != nil {
		slog.Error("Failed to write manifest file", "error", err)
		errors.FatalError(errors.NewGenerationError("failed to write manifest", err))
	}

	slog.Info("Build manifest written", "path", outManifestPath)

	// Print success summary with manifest
	printDockerfileGenerationSummary(generateResult, buildManifest, outManifestPath)
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

	fmt.Println("\nNext Steps:")
	fmt.Println("  1. Build the images:")
	fmt.Printf("     cd %s/policy-engine && docker build -t <image-name> .\n", result.OutputDir)
	fmt.Printf("     cd %s/gateway-controller && docker build -t <image-name> .\n", result.OutputDir)
	fmt.Printf("     cd %s/router && docker build -t <image-name> .\n", result.OutputDir)
	fmt.Println("  2. Deploy the gateway stack:")
	fmt.Println("     docker-compose up -d")
	fmt.Println()
}
