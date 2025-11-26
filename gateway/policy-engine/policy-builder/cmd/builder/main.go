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
	"github.com/policy-engine/policy-builder/internal/generation"
	"github.com/policy-engine/policy-builder/internal/packaging"
	"github.com/policy-engine/policy-builder/internal/validation"
	"github.com/policy-engine/policy-builder/pkg/errors"
	"github.com/policy-engine/policy-builder/pkg/types"
)

const (
	DefaultManifestFile = "policy.yaml"
	DefaultOutputDir    = "/output"
	DefaultRuntimeDir   = "/workspace/policy-engine"
	BuilderVersion      = "v1.0.0"
)

func main() {
	// Parse command-line flags
	manifestPath := flag.String("manifest", DefaultManifestFile, "Path to policy manifest file")
	outputDir := flag.String("output-dir", DefaultOutputDir, "Directory for build output (binary and Dockerfile)")
	runtimeDir := flag.String("runtime-dir", DefaultRuntimeDir, "Path to policy-engine runtime source directory")
	debug := flag.Bool("debug", false, "Enable debug logging")
	logFormat := flag.String("log-format", "json", "Log format: text or json")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")
	flag.Parse()

	// Setup logging
	initLogger(*logFormat, *logLevel, *debug)

	slog.Info("Policy Builder starting",
		"version", BuilderVersion,
		"manifest", *manifestPath)

	// Ensure output directory exists
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		errors.FatalError(errors.NewGenerationError("failed to create output directory", err))
	}

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
	if err := generation.GenerateCode(*runtimeDir, policies); err != nil {
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

	binaryPath := filepath.Join(*outputDir, "policy-engine")
	compileOpts := compilation.BuildOptions(binaryPath, buildMetadata)

	if err := compilation.CompileBinary(*runtimeDir, compileOpts); err != nil {
		errors.FatalError(err)
	}

	// Phase 5: Packaging
	slog.Info("Starting Phase 5: Packaging", "phase", "packaging")
	if err := packaging.GenerateDockerfile(*outputDir, policies, BuilderVersion); err != nil {
		errors.FatalError(err)
	}

	// Success summary
	printSummary(policies, binaryPath, *outputDir)
}

// initLogger sets up the slog logger based on format and level
func initLogger(format, level string, debug bool) {
	// Determine log level
	var logLevel slog.Level
	if debug {
		logLevel = slog.LevelDebug
	} else {
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

// printSummary displays the final build summary
func printSummary(policies []*types.DiscoveredPolicy, binaryPath, outputDir string) {
	// Log actual events
	slog.Info("Build completed successfully",
		"phase", "complete",
		"policies", len(policies),
		"binary", binaryPath)
	slog.Info("Generated artifacts",
		"dockerfile", outputDir+"/Dockerfile",
		"build_instructions", outputDir+"/BUILD.md")

	// Print instructional content (not logged)
	fmt.Println("\nNext Steps:")
	fmt.Println("1. Review the generated BUILD.md for Docker build instructions")
	fmt.Println("2. Build the Docker image:")
	fmt.Printf("   cd %s && docker build -t policy-engine:custom .\n", outputDir)
	fmt.Println("3. Run the container:")
	fmt.Println("   docker run -p 9001:9001 -p 9002:9002 policy-engine:custom")
	fmt.Println()
}
