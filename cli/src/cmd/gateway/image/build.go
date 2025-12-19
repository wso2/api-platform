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
	"github.com/wso2/api-platform/cli/internal/policy"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	BuildCmdLiteral = "build"
	BuildCmdExample = `# Build gateway image with policies (uses current directory)
apipctl gateway image build --image-tag v0.2.0-policy1

# Build with custom path containing manifest files
apipctl gateway image build --image-tag v0.2.0 --path ./my-policies --image-repository myregistry/gateway

# Build in offline mode (uses manifest lock file)
apipctl gateway image build --image-tag v0.2.0 --offline

# Build with platform specification
apipctl gateway image build --image-tag v0.2.0 --platform linux/amd64`
)

var (
	// Required flags
	imageTag string

	// Optional flags
	manifestPath             string
	imageRepository          string
	gatewayBuilder           string
	gatewayControllerBaseImg string
	routerBaseImg            string
	push                     bool
	noCache                  bool
	platform                 string
	offline                  bool
	outputDir                string
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
	// Required flags
	buildCmd.Flags().StringVar(&imageTag, "image-tag", "", "Docker image tag (required)")
	buildCmd.MarkFlagRequired("image-tag")

	// Optional flags with defaults
	buildCmd.Flags().StringVarP(&manifestPath, "path", "p", ".", "Path to directory containing policy manifest files (default: current directory)")
	buildCmd.Flags().StringVar(&imageRepository, "image-repository", utils.DefaultImageRepository, "Docker image repository")
	buildCmd.Flags().StringVar(&gatewayBuilder, "gateway-builder", utils.DefaultGatewayBuilder, "Gateway builder image")
	buildCmd.Flags().StringVar(&gatewayControllerBaseImg, "gateway-controller-base-image", utils.DefaultGatewayControllerImg, "Gateway controller base image (uses builder default if empty)")
	buildCmd.Flags().StringVar(&routerBaseImg, "router-base-image", utils.DefaultRouterImg, "Router base image (uses builder default if empty)")
	buildCmd.Flags().BoolVar(&push, "push", false, "Push image to registry after build")
	buildCmd.Flags().BoolVar(&noCache, "no-cache", false, "Build without using cache")
	buildCmd.Flags().StringVar(&platform, "platform", "", "Target platform (e.g., linux/amd64)")
	buildCmd.Flags().BoolVar(&offline, "offline", false, "Build in offline mode using manifest lock file")
	buildCmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory for build artifacts")
}

func runBuildCommand() error {
	fmt.Println("\n=== Gateway Image Build ===\n")

	// Step 1: Check Docker availability
	fmt.Println("[1/6] Checking Docker Availability")
	if err := utils.IsDockerAvailable(); err != nil {
		return fmt.Errorf("Docker is not available: %w", err)
	}
	fmt.Println("  ✓ Docker is available")

	// Determine build mode
	if offline {
		fmt.Println("\n→ Building in OFFLINE mode\n")
		return runOfflineBuild()
	}

	fmt.Println("\n→ Building in ONLINE mode\n")
	return runOnlineBuild()
}

func runOnlineBuild() error {
	// Ensure cleanup happens on exit
	defer func() {
		if err := utils.CleanTempDir(); err != nil {
			fmt.Printf("Warning: Failed to clean temp directory: %v\n", err)
		}
	}()

	// Step 2: Read Policy Manifest
	fmt.Println("[2/6] Reading Policy Manifest")

	// Get manifest file path
	manifestFilePath, err := getManifestFilePath(manifestPath)
	if err != nil {
		return err
	}

	manifest, err := policy.ParseManifest(manifestFilePath)
	if err != nil {
		return fmt.Errorf("failed to parse manifest file at '%s': %w\n\nMake sure the file exists and is a valid YAML file", manifestFilePath, err)
	}
	fmt.Printf("  ✓ Loaded manifest with %d policies\n\n", len(manifest.Policies))

	// Step 3: Separate local and hub policies
	localPolicies, hubPolicies := policy.SeparatePolicies(manifest)
	fmt.Printf("  → Local policies: %d\n", len(localPolicies))
	fmt.Printf("  → Hub policies: %d\n\n", len(hubPolicies))

	var allProcessed []policy.ProcessedPolicy

	// Step 4: Process Local Policies
	fmt.Println("[3/6] Processing Local Policies")
	if len(localPolicies) > 0 {
		processed, err := policy.ProcessLocalPolicies(localPolicies)
		if err != nil {
			return fmt.Errorf("failed to process local policies: %w", err)
		}
		allProcessed = append(allProcessed, processed...)
	} else {
		fmt.Println("  → No local policies to process\n")
	}

	// Step 5: Resolve and Download Hub Policies
	fmt.Println("[4/6] Resolving Hub Policies from PolicyHub")
	if len(hubPolicies) > 0 {
		processed, err := policy.ProcessHubPolicies(hubPolicies, manifest.VersionResolution)
		if err != nil {
			return fmt.Errorf("failed to process hub policies: %w", err)
		}
		allProcessed = append(allProcessed, processed...)
	} else {
		fmt.Println("  → No hub policies to resolve\n")
	}

	// Step 6: Generate Lock File
	fmt.Println("[5/6] Generating Manifest Lock File")
	lockFilePath := filepath.Join(manifestPath, utils.DefaultManifestLockFile)
	if err := policy.GenerateLockFile(allProcessed, lockFilePath); err != nil {
		return fmt.Errorf("failed to generate lock file: %w", err)
	}
	fmt.Printf("  ✓ Generated lock file: %s\n\n", lockFilePath)

	// Step 7: Display Summary
	fmt.Println("[6/6] Build Preparation Complete")
	displayBuildSummary(manifest, manifestFilePath, lockFilePath, allProcessed)

	return nil
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

func runOfflineBuild() error {
	// Ensure cleanup happens on exit
	defer func() {
		if err := utils.CleanTempDir(); err != nil {
			fmt.Printf("Warning: Failed to clean temp directory: %v\n", err)
		}
	}()

	// Step 2: Read Manifest and Lock Files
	fmt.Println("[2/4] Reading Manifest and Lock Files")

	// Get manifest file path
	manifestFilePath, err := getManifestFilePath(manifestPath)
	if err != nil {
		return err
	}

	// Read manifest file (needed for local policy paths)
	manifest, err := policy.ParseManifest(manifestFilePath)
	if err != nil {
		return fmt.Errorf("failed to parse manifest file at '%s': %w\n\nMake sure the file exists and is a valid YAML file", manifestFilePath, err)
	}

	// Read lock file
	lockFilePath := filepath.Join(manifestPath, utils.DefaultManifestLockFile)
	if _, err := os.Stat(lockFilePath); os.IsNotExist(err) {
		return fmt.Errorf("lock file '%s' not found in directory: %s\n\nExpected file: %s\n\nPlease run the build command in ONLINE mode first (without --offline flag) to generate the lock file", utils.DefaultManifestLockFile, manifestPath, lockFilePath)
	}

	lockFile, err := policy.ParseLockFile(lockFilePath)
	if err != nil {
		return fmt.Errorf("failed to parse lock file at '%s': %w\n\nThe lock file may be corrupted. Try regenerating it by running in ONLINE mode (without --offline flag)", lockFilePath, err)
	}
	fmt.Printf("  ✓ Loaded manifest and lock file with %d policies\n\n", len(lockFile.Policies))

	// Step 3: Verify All Policies
	fmt.Println("[3/4] Verifying Policies")
	verified, err := policy.VerifyOfflinePolicies(lockFile, manifest)
	if err != nil {
		return fmt.Errorf("policy verification failed: %w", err)
	}

	// Step 4: Display Summary
	fmt.Println("[4/4] Build Preparation Complete")
	displayOfflineBuildSummary(manifestFilePath, lockFilePath, verified)

	return nil
}

func displayOfflineBuildSummary(manifestFile, lockFile string, verified []policy.ProcessedPolicy) {
	fmt.Println("\n=== Build Summary ===")

	fmt.Println("\nConfiguration:")
	fmt.Printf("  Image Tag:                    %s\n", imageTag)
	fmt.Printf("  Manifest File:                %s\n", manifestFile)
	fmt.Printf("  Manifest Lock File:           %s\n", lockFile)
	fmt.Printf("  Image Repository:             %s\n", imageRepository)
	fmt.Printf("  Gateway Builder:              %s\n", gatewayBuilder)

	if gatewayControllerBaseImg != "" {
		fmt.Printf("  Gateway Controller Base:      %s\n", gatewayControllerBaseImg)
	} else {
		fmt.Printf("  Gateway Controller Base:      (using builder default)\n")
	}

	if routerBaseImg != "" {
		fmt.Printf("  Router Base Image:            %s\n", routerBaseImg)
	} else {
		fmt.Printf("  Router Base Image:            (using builder default)\n")
	}

	if platform != "" {
		fmt.Printf("  Platform:                     %s\n", platform)
	} else {
		fmt.Printf("  Platform:                     (using host platform)\n")
	}

	fmt.Printf("  Push to Registry:             %t\n", push)
	fmt.Printf("  No Cache:                     %t\n", noCache)
	fmt.Printf("  Offline Mode:                 %t\n", offline)

	if outputDir != "" {
		fmt.Printf("  Output Directory:             %s\n", outputDir)
	}

	// Display policy information
	fmt.Println("\nPolicies:")
	fmt.Printf("  Total Policies:               %d\n", len(verified))

	hubCount := 0
	localCount := 0
	for _, p := range verified {
		if p.IsLocal {
			localCount++
		} else {
			hubCount++
		}
	}

	fmt.Printf("  Hub Policies (from cache):    %d\n", hubCount)
	fmt.Printf("  Local Policies:               %d\n", localCount)

	fmt.Println("\nVerified Policies:")
	for _, p := range verified {
		source := "cache"
		if p.IsLocal {
			source = "local"
		}
		fmt.Printf("  [%s] %s %s (checksum: %s...)\n", source, p.Name, p.Version, p.Checksum[:20])
	}

	fmt.Println("\n✓ All policies verified and ready for build")
	fmt.Println()
}

func displayBuildSummary(manifest *policy.PolicyManifest, manifestFilePath, lockFile string, processed []policy.ProcessedPolicy) {
	fmt.Println("\n=== Build Summary ===")

	fmt.Println("\nConfiguration:")
	fmt.Printf("  Image Tag:                    %s\n", imageTag)
	fmt.Printf("  Manifest File:                %s\n", manifestFilePath)
	fmt.Printf("  Lock File:                    %s\n", lockFile)
	fmt.Printf("  Image Repository:             %s\n", imageRepository)
	fmt.Printf("  Gateway Builder:              %s\n", gatewayBuilder)

	if gatewayControllerBaseImg != "" {
		fmt.Printf("  Gateway Controller Base:      %s\n", gatewayControllerBaseImg)
	} else {
		fmt.Printf("  Gateway Controller Base:      (using builder default)\n")
	}

	if routerBaseImg != "" {
		fmt.Printf("  Router Base Image:            %s\n", routerBaseImg)
	} else {
		fmt.Printf("  Router Base Image:            (using builder default)\n")
	}

	if platform != "" {
		fmt.Printf("  Platform:                     %s\n", platform)
	} else {
		fmt.Printf("  Platform:                     (using host platform)\n")
	}

	fmt.Printf("  Push to Registry:             %t\n", push)
	fmt.Printf("  No Cache:                     %t\n", noCache)
	fmt.Printf("  Offline Mode:                 %t\n", offline)

	if outputDir != "" {
		fmt.Printf("  Output Directory:             %s\n", outputDir)
	}

	// Display policy information
	fmt.Println("\nPolicies:")
	fmt.Printf("  Total Policies:               %d\n", len(processed))

	hubCount := 0
	localCount := 0
	for _, p := range processed {
		if p.IsLocal {
			localCount++
		} else {
			hubCount++
		}
	}

	fmt.Printf("  Hub Policies:                 %d\n", hubCount)
	fmt.Printf("  Local Policies:               %d\n", localCount)

	fmt.Println("\nLoaded Policies:")
	for _, p := range processed {
		source := "hub"
		if p.IsLocal {
			source = "local"
		}
		fmt.Printf("  [%s] %s %s (checksum: %s...)\n", source, p.Name, p.Version, p.Checksum[:20])
	}

	fmt.Println("\n✓ All policies loaded and ready for build")
	fmt.Println()
}
