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

package image

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/internal/policy"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	BuildCmdLiteral = "build"
	BuildCmdExample = `# Build gateway image with policies (uses current directory)
ap gateway image build

# Build with custom name and version
ap gateway image build --name my-gateway --version 1.0.0

# Build with custom path containing manifest files
ap gateway image build --name my-gateway --path ./my-policies --repository myregistry

# Build with platform specification
ap gateway image build --name my-gateway --platform linux/amd64`
)

var (
	// Optional flags
	gatewayName              string
	gatewayVersion           string
	manifestPath             string
	imageRepository          string
	gatewayBuilder           string
	gatewayControllerBaseImg string
	routerBaseImg            string
	push                     bool
	noCache                  bool
	platform                 string
	outputDir                string

	// Computed values
	imageTag string
)

var buildCmd = &cobra.Command{
	Use:     BuildCmdLiteral,
	Short:   "Build gateway Docker image with policies",
	Long:    "Build a WSO2 API Platform Gateway Docker image with specified policies from manifest file.",
	Example: BuildCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runBuildCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	// Optional flags with defaults
	buildCmd.Flags().StringVar(&gatewayName, "name", "", "Gateway name (defaults to directory name)")
	buildCmd.Flags().StringVar(&gatewayVersion, "version", utils.DefaultGatewayVersion, "Gateway version")
	buildCmd.Flags().StringVarP(&manifestPath, "path", "p", ".", "Path to directory containing policy manifest files (default: current directory)")
	buildCmd.Flags().StringVar(&imageRepository, "repository", utils.DefaultImageRepository, "Docker image repository")
	buildCmd.Flags().StringVar(&gatewayBuilder, "gateway-builder", "", "Gateway builder image (defaults to repository/gateway-builder:version)")
	buildCmd.Flags().StringVar(&gatewayControllerBaseImg, "gateway-controller-base-image", utils.DefaultGatewayControllerImg, "Gateway controller base image (uses builder default if empty)")
	buildCmd.Flags().StringVar(&routerBaseImg, "router-base-image", utils.DefaultRouterImg, "Router base image (uses builder default if empty)")
	buildCmd.Flags().BoolVar(&push, "push", false, "Push image to registry after build")
	buildCmd.Flags().BoolVar(&noCache, "no-cache", false, "Build without using cache")
	buildCmd.Flags().StringVar(&platform, "platform", "", "Target platform (e.g., linux/amd64)")
	buildCmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory for build artifacts")
}

// initializeDefaults sets smart defaults for gateway name and constructs the image tag
func initializeDefaults() error {
	// Default gateway name from directory name if not provided
	if gatewayName == "" {
		absPath, err := filepath.Abs(manifestPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}
		gatewayName = filepath.Base(absPath)
	}

	// Default gateway builder if not provided (use repo without forcing a tag)
	if gatewayBuilder == "" {
		gatewayBuilder = utils.DefaultGatewayBuilderRepo
	}

	// Construct the full image tag: repository/name:version
	imageTag = fmt.Sprintf("%s/%s:%s", imageRepository, gatewayName, gatewayVersion)

	return nil
}

func runBuildCommand() error {
	fmt.Println("=== Gateway Image Build ===")
	fmt.Println()

	// Initialize computed values
	if err := initializeDefaults(); err != nil {
		return err
	}

	// Step 1: Check Docker availability
	fmt.Println("[1/8] Checking Docker Availability")
	if err := utils.IsDockerAvailable(); err != nil {
		return fmt.Errorf("Docker is not available: %w", err)
	}
	fmt.Println("  ✓ Docker is available")

	// Check docker buildx if platform is specified
	if platform != "" {
		if err := utils.IsDockerBuildxAvailable(); err != nil {
			return fmt.Errorf("docker buildx is not available but required for --platform flag: %w\n\nPlease install docker buildx or remove the --platform flag", err)
		}
		fmt.Println("  ✓ Docker buildx is available")
	}

	fmt.Println()
	return runUnifiedBuild()
}

// getManifestFilePath returns the full path to the manifest file
func getManifestFilePath(basePath string) (string, error) {
	// Check if path exists
	info, err := os.Stat(basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path does not exist: %s\n\nPlease provide a valid directory path using --path flag", basePath)
		}
		return "", fmt.Errorf("failed to access path: %w", err)
	}

	// If it's a directory, look for the manifest file
	if info.IsDir() {
		manifestFile := filepath.Join(basePath, utils.DefaultManifestFile)
		if _, err := os.Stat(manifestFile); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("manifest file '%s' not found in directory: %s\n\nExpected file: %s\n\nPlease create a %s file or specify a different path", utils.DefaultManifestFile, basePath, manifestFile, utils.DefaultManifestFile)
			}
			return "", fmt.Errorf("failed to access manifest file: %w", err)
		}
		return manifestFile, nil
	}

	// If it's a file, that's an error - we expect a directory
	return "", fmt.Errorf("--path must be a directory, not a file: %s\n\nPlease provide the directory path containing %s", basePath, utils.DefaultManifestFile)
}

func runUnifiedBuild() error {
	// Step 2: Read Policy Manifest
	fmt.Println("[2/6] Reading Policy Manifest")

	// Get manifest file path
	manifestFilePath, err := getManifestFilePath(manifestPath)
	if err != nil {
		return err
	}

	manifest, err := policy.ParseManifest(manifestFilePath)
	if err != nil {
		return fmt.Errorf("failed to parse manifest file at '%s': %w", manifestFilePath, err)
	}
	fmt.Printf("  ✓ Loaded manifest with %d policies\n\n", len(manifest.Policies))

	// Step 3: Validate Manifest and Separate Policies
	fmt.Println("[3/6] Validating manifest")
	localPolicies, hubPolicies := policy.SeparatePolicies(manifest)
	fmt.Printf("  → Hub policies: %d\n", len(hubPolicies))
	fmt.Printf("  → Local policies: %d\n\n", len(localPolicies))

	// Step 4: Process Local Policies
	fmt.Println("[4/6] Processing Local Policies")
	var processed []policy.ProcessedPolicy
	if len(localPolicies) > 0 {
		processed, err = policy.ProcessLocalPolicies(localPolicies)
		if err != nil {
			return fmt.Errorf("failed to process local policies: %w", err)
		}
		fmt.Printf("  ✓ Processed %d local policies\n\n", len(processed))
	}

	// Step 5: Prepare workspace and copy manifest + policies
	fmt.Println("[5/6] Preparing workspace and copying policies")
	tempDir, err := utils.SetupTempGatewayWorkspace(manifestFilePath)
	if err != nil {
		return fmt.Errorf("failed to setup workspace: %w", err)
	}

	// Policies were copied into the workspace by SetupTempGatewayWorkspace

	fmt.Printf("  ✓ Workspace ready: %s\n\n", tempDir)

	// Step 6: Run Docker build (gateway-builder + image build)
	fmt.Println("[6/6] Building Gateway Images")
	if err := runDockerBuild(); err != nil {
		return fmt.Errorf("failed to build gateway images: %w", err)
	}
	fmt.Println("  ✓ All images built successfully")

	// Copy generated lock file from workspace back to manifest location (if present)
	tempGatewayImageBuildDir, err := utils.GetTempGatewayImageBuildDir()
	if err != nil {
		return fmt.Errorf("failed to get temp gateway image build directory path: %w", err)
	}
	workspaceLockPath := filepath.Join(tempGatewayImageBuildDir, utils.DefaultManifestLockFile)
	if _, err := os.Stat(workspaceLockPath); err == nil {
		// Copy to manifest directory
		manifestLockDst := filepath.Join(filepath.Dir(manifestFilePath), utils.DefaultManifestLockFile)
		lockContent, err := os.ReadFile(workspaceLockPath)
		if err != nil {
			return fmt.Errorf("failed to read workspace lock file: %w", err)
		}
		if err := os.WriteFile(manifestLockDst, lockContent, 0644); err != nil {
			return fmt.Errorf("failed to write manifest lock file to %s: %w", manifestLockDst, err)
		}
		fmt.Printf("  ✓ Copied lock file to manifest location: %s\n\n", manifestLockDst)
	} else {
		fmt.Println("  → No lock file generated by builder")
	}

	// Display Summary
	displayBuildSummary(manifest, manifestFilePath, filepath.Join(tempGatewayImageBuildDir, utils.DefaultManifestLockFile), processed, tempGatewayImageBuildDir)

	return nil
}

func displayBuildSummary(manifest *policy.PolicyManifest, manifestFilePath, lockFile string, processed []policy.ProcessedPolicy, workspaceDir string) {
	fmt.Println("=== Build Summary ===")
	fmt.Println()

	// Images built
	fmt.Printf("✓ Built gateway images with %d policies:\n", len(processed))
	fmt.Printf("  • %s/%s-policy-engine:%s\n", imageRepository, gatewayName, gatewayVersion)
	fmt.Printf("  • %s/%s-gateway-controller:%s\n", imageRepository, gatewayName, gatewayVersion)
	fmt.Printf("  • %s/%s-router:%s\n", imageRepository, gatewayName, gatewayVersion)
	fmt.Println()

	// Where images are
	if push {
		fmt.Printf("✓ Images pushed to registry: %s\n", imageRepository)
		if platform != "" {
			fmt.Printf("  Platform: %s\n", platform)
		}
	} else {
		fmt.Println("✓ Images available in local Docker")
		if platform != "" {
			fmt.Printf("  Platform: %s\n", platform)
		}
	}
	fmt.Println()

	// Workspace and output
	// Workspace output
	outputPath := filepath.Join(workspaceDir, "output")
	fmt.Printf("✓ Temporary Build output: %s\n", outputPath)
	fmt.Printf("✓ Manifest lock file: %s\n", lockFile)
	if outputDir != "" {
		fmt.Printf("✓ Output artifacts copied to: %s\n", outputDir)
	}
	// Explain workspace cleanup behavior
	fmt.Printf("\nNote: Workspace may be cleared on the next run of this command.\n")
	fmt.Println()
}

// runDockerBuild executes the docker build process for gateway images
func runDockerBuild() error {
	tempGatewayImageBuildDir, err := utils.GetTempGatewayImageBuildDir()
	if err != nil {
		return fmt.Errorf("failed to get temp gateway image build directory: %w", err)
	}

	// Create logs directory
	logsDir := filepath.Join(tempGatewayImageBuildDir, "logs")
	if err := utils.EnsureDir(logsDir); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	logFilePath := filepath.Join(logsDir, "docker.log")

	// Prepare build configuration
	config := gateway.DockerBuildConfig{
		TempDir:                    tempGatewayImageBuildDir,
		GatewayBuilder:             gatewayBuilder,
		GatewayControllerBaseImage: gatewayControllerBaseImg,
		RouterBaseImage:            routerBaseImg,
		ImageRepository:            imageRepository,
		GatewayName:                gatewayName,
		GatewayVersion:             gatewayVersion,
		Platform:                   platform,
		NoCache:                    noCache,
		Push:                       push,
		LogFilePath:                logFilePath,
		OutputCopyDir:              outputDir,
	}

	// Run the build
	if err := gateway.BuildGatewayImages(config); err != nil {
		return err
	}

	return nil
}
