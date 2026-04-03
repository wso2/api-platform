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

# Build with custom name
ap gateway image build --name my-gateway

# Build with custom path containing build files
ap gateway image build --name my-gateway --path ./my-policies --repository myregistry

# Build with platform specification
ap gateway image build --name my-gateway --platform linux/amd64`
)

var (
	// Optional flags
	gatewayName              string
	gatewayVersion           string
	buildFilePath            string
	imageRepository          string
	gatewayBuilder           string
	gatewayControllerBaseImg string
	gatewayRuntimeBaseImg    string
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
	Long:    "Build a WSO2 API Platform Gateway Docker image with specified policies from build file.",
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
	buildCmd.Flags().StringVarP(&buildFilePath, "path", "p", ".", "Path to directory containing build files (default: current directory)")
	buildCmd.Flags().StringVar(&imageRepository, "repository", utils.DefaultImageRepository, "Docker image repository")
	buildCmd.Flags().BoolVar(&push, "push", false, "Push image to registry after build")
	buildCmd.Flags().BoolVar(&noCache, "no-cache", false, "Build without using cache")
	buildCmd.Flags().StringVar(&platform, "platform", "", "Target platform (e.g., linux/amd64)")
	buildCmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory for build artifacts")
}

// initializeDefaults sets smart defaults for gateway name and constructs the image tag
func initializeDefaults(buildFile *policy.BuildFile) error {
	// Default gateway name from directory name if not provided
	if gatewayName == "" {
		absPath, err := filepath.Abs(buildFilePath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}
		gatewayName = filepath.Base(absPath)
	}

	// Default gateway version from build file if not provided via flag
	if buildFile.Gateway.Version == "" {
		return fmt.Errorf("gateway version is required: set gateway.version in build.yaml")
	}
	gatewayVersion = buildFile.Gateway.Version

	// Use custom images from build file if provided, otherwise construct from defaults
	if buildFile.Gateway.Images.Builder != "" {
		gatewayBuilder = buildFile.Gateway.Images.Builder
	} else {
		gatewayBuilder = fmt.Sprintf(utils.DefaultGatewayBuilder, gatewayVersion)
	}

	if buildFile.Gateway.Images.Controller != "" {
		gatewayControllerBaseImg = buildFile.Gateway.Images.Controller
	} else {
		gatewayControllerBaseImg = fmt.Sprintf(utils.DefaultGatewayController, gatewayVersion)
	}

	if buildFile.Gateway.Images.Runtime != "" {
		gatewayRuntimeBaseImg = buildFile.Gateway.Images.Runtime
	} else {
		gatewayRuntimeBaseImg = fmt.Sprintf(utils.DefaultGatewayRuntime, gatewayVersion)
	}

	// Construct the full image tag: repository/name:version
	imageTag = fmt.Sprintf("%s/%s:%s", imageRepository, gatewayName, gatewayVersion)

	return nil
}

func runBuildCommand() error {
	fmt.Println("=== Gateway Image Build ===")
	fmt.Println()

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

// getBuildFilePath returns the full path to the build file
func getBuildFilePath(basePath string) (string, error) {
	// Check if path exists
	info, err := os.Stat(basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path does not exist: %s\n\nPlease provide a valid directory path using --path flag", basePath)
		}
		return "", fmt.Errorf("failed to access path: %w", err)
	}

	// If it's a directory, look for the build file
	if info.IsDir() {
		buildFile := filepath.Join(basePath, utils.DefaultBuildFile)
		if _, err := os.Stat(buildFile); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("build file '%s' not found in directory: %s\n\nExpected file: %s\n\nPlease create a %s file or specify a different path", utils.DefaultBuildFile, basePath, buildFile, utils.DefaultBuildFile)
			}
			return "", fmt.Errorf("failed to access build file: %w", err)
		}
		return buildFile, nil
	}

	// If it's a file, that's an error - we expect a directory
	return "", fmt.Errorf("--path must be a directory, not a file: %s\n\nPlease provide the directory path containing %s", basePath, utils.DefaultBuildFile)
}

func runUnifiedBuild() error {
	// Step 2: Read Build File
	fmt.Println("[2/6] Reading Build File")

	// Get build file path
	resolvedBuildFilePath, err := getBuildFilePath(buildFilePath)
	if err != nil {
		return err
	}

	buildFile, err := policy.ParseBuildFile(resolvedBuildFilePath)
	if err != nil {
		return fmt.Errorf("failed to parse build file at '%s': %w", resolvedBuildFilePath, err)
	}
	fmt.Printf("  ✓ Loaded build file with %d policies\n\n", len(buildFile.Policies))

	// Initialize computed values
	if err := initializeDefaults(buildFile); err != nil {
		return err
	}

	// Display resolved images used for the build
	fmt.Println("  Resolved images:")
	fmt.Printf("    • Builder:            %s\n", gatewayBuilder)
	fmt.Printf("    • Gateway Controller: %s\n", gatewayControllerBaseImg)
	fmt.Printf("    • Gateway Runtime:    %s\n\n", gatewayRuntimeBaseImg)

	// Step 3: Validate Build File and Separate Policies
	fmt.Println("[3/6] Validating build file")
	localPolicies, hubPolicies := policy.SeparatePolicies(buildFile)
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

	// Step 5: Prepare workspace and copy build file + policies
	fmt.Println()
	fmt.Println("[5/6] Preparing workspace and copying policies")
	tempDir, err := utils.SetupTempGatewayWorkspace(resolvedBuildFilePath)
	if err != nil {
		return fmt.Errorf("failed to setup workspace: %w", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("  ✓ Workspace ready: %s\n\n", tempDir)

	// Step 6: Run Docker build (gateway-builder + image build)
	fmt.Println("[6/6] Building Gateway Images")
	if err := runDockerBuild(tempDir); err != nil {
		return fmt.Errorf("failed to build gateway images: %w", err)
	}
	fmt.Println("  ✓ All images built successfully")

	// Copy build-manifest.yaml back to user's directory
	lockSrc := filepath.Join(tempDir, "build-manifest.yaml")
	lockDst := filepath.Join(filepath.Dir(resolvedBuildFilePath), "build-manifest.yaml")
	if lockData, err := os.ReadFile(lockSrc); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not read build manifest file: %v\n", err)
	} else if err := os.WriteFile(lockDst, lockData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not write build manifest file: %v\n", err)
	} else {
		fmt.Printf("  ✓ Build manifest file written: build-manifest.yaml\n")
	}

	// Display Summary
	displayBuildSummary(processed)

	return nil
}

func displayBuildSummary(processed []policy.ProcessedPolicy) {
	fmt.Println("=== Build Summary ===")
	fmt.Println()

	// Images built
	fmt.Printf("✓ Built gateway images with %d policies:\n", len(processed))
	fmt.Printf("  • %s/%s-gateway-runtime:%s\n", imageRepository, gatewayName, gatewayVersion)
	fmt.Printf("  • %s/%s-gateway-controller:%s\n", imageRepository, gatewayName, gatewayVersion)
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

	if outputDir != "" {
		fmt.Printf("✓ Output artifacts copied to: %s\n", outputDir)
		fmt.Println()
	}
}

// runDockerBuild executes the docker build process for gateway images
func runDockerBuild(tempDir string) error {
	// Create logs directory
	logsDir := filepath.Join(tempDir, "logs")
	if err := utils.EnsureDir(logsDir); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	logFilePath := filepath.Join(logsDir, "docker.log")

	// Prepare build configuration
	config := gateway.DockerBuildConfig{
		TempDir:                    tempDir,
		GatewayBuilder:             gatewayBuilder,
		GatewayControllerBaseImage: gatewayControllerBaseImg,
		GatewayRuntimeBaseImage:    gatewayRuntimeBaseImg,
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
