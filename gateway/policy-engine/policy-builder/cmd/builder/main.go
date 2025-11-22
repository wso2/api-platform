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
	DefaultManifestFile = "policies.yaml"
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
	flag.Parse()

	// Setup logging
	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Print banner
	printBanner()

	fmt.Printf("Output Directory: %s\n", *outputDir)
	fmt.Printf("Runtime Directory: %s\n", *runtimeDir)

	// Ensure output directory exists
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		errors.FatalError(errors.NewGenerationError("failed to create output directory", err))
	}

	// Phase 1: Discovery
	fmt.Println("========================================")
	fmt.Println("PHASE 1: DISCOVERY")
	fmt.Println("========================================")
	fmt.Printf("Using manifest: %s\n", *manifestPath)

	policies, err := discovery.DiscoverPoliciesFromManifest(*manifestPath, "")
	if err != nil {
		errors.FatalError(err)
	}
	fmt.Printf("✓ Loaded manifest: %d policies declared\n", len(policies))
	fmt.Println()

	// Print discovered policies
	for i, p := range policies {
		fmt.Printf("  %d. %s v%s\n", i+1, p.Name, p.Version)
		fmt.Printf("     Path: %s\n", p.Path)
	}
	fmt.Println()

	// Phase 2: Validation
	fmt.Println("========================================")
	fmt.Println("PHASE 2: VALIDATION")
	fmt.Println("========================================")
	validationResult, err := validation.ValidatePolicies(policies)
	if err != nil {
		fmt.Println(validation.FormatValidationErrors(validationResult))
		errors.FatalError(err)
	}
	fmt.Printf("✓ All policies validated successfully\n\n")

	// Phase 3: Code Generation
	fmt.Println("========================================")
	fmt.Println("PHASE 3: CODE GENERATION")
	fmt.Println("========================================")
	if err := generation.GenerateCode(*runtimeDir, policies); err != nil {
		errors.FatalError(err)
	}
	fmt.Println()

	// Phase 4: Compilation
	fmt.Println("========================================")
	fmt.Println("PHASE 4: COMPILATION")
	fmt.Println("========================================")

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
	fmt.Println()

	// Phase 5: Packaging
	fmt.Println("========================================")
	fmt.Println("PHASE 5: PACKAGING")
	fmt.Println("========================================")
	if err := packaging.GenerateDockerfile(*outputDir, policies, BuilderVersion); err != nil {
		errors.FatalError(err)
	}
	fmt.Println()

	// Success summary
	printSummary(policies, binaryPath, *outputDir)
}

// printBanner displays the builder banner
func printBanner() {
	banner := `
═══════════════════════════════════════════════════════════

Policy Engine Builder
Version: ` + BuilderVersion + `

═══════════════════════════════════════════════════════════
`
	fmt.Println(banner)
}

// printSummary displays the final build summary
func printSummary(policies []*types.DiscoveredPolicy, binaryPath, outputDir string) {
	fmt.Println("========================================")
	fmt.Println("BUILD COMPLETE")
	fmt.Println("========================================")
	fmt.Printf("✓ Compiled %d policies into binary\n", len(policies))
	fmt.Printf("✓ Binary: %s\n", binaryPath)
	fmt.Printf("✓ Dockerfile: %s/Dockerfile\n", outputDir)
	fmt.Printf("✓ Build instructions: %s/BUILD.md\n\n", outputDir)

	fmt.Println("Next Steps:")
	fmt.Println("1. Review the generated BUILD.md for Docker build instructions")
	fmt.Println("2. Build the Docker image:")
	fmt.Printf("   cd %s && docker build -t policy-engine:custom .\n", outputDir)
	fmt.Println("3. Run the container:")
	fmt.Println("   docker run -p 9001:9001 -p 9002:9002 policy-engine:custom")
	fmt.Println()
}
