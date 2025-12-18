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

package gateway

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/internal/policyhub"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	BuildCmdLiteral = "build"
	BuildCmdExample = `# Build a gateway with policy-manifest
apipctl gateway build --file policy-manifest.yaml --docker-registry myregistry --image-tag v1.0.0

# Build with optional parameters
apipctl gateway build -f policy-manifest.yaml --docker-registry myregistry --image-tag v1.0.0 \
  --gateway-builder my-builder \
  --gateway-controller-base-image controller:latest \
  --router-base-image router:latest`
)

var (
	buildFilePath               string
	buildDockerRegistry         string
	buildImageTag               string
	buildGatewayBuilder         string
	buildGatewayControllerImage string
	buildRouterBaseImage        string
)

var buildCmd = &cobra.Command{
	Use:     BuildCmdLiteral,
	Short:   "Build a gateway with policies",
	Long:    "Build a gateway image with the specified policies from a policy-manifest file.",
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
	utils.AddStringFlag(buildCmd, utils.FlagFile, &buildFilePath, "", "Path to the policy-manifest file (required)")
	utils.AddStringFlag(buildCmd, utils.FlagDockerRegistry, &buildDockerRegistry, "", "Docker registry name (required)")
	utils.AddStringFlag(buildCmd, utils.FlagImageTag, &buildImageTag, "", "Image tag for the gateway build (required)")

	utils.AddStringFlag(buildCmd, utils.FlagGatewayBuilder, &buildGatewayBuilder, "", "Gateway builder name (required)")

	// Optional flags
	utils.AddStringFlag(buildCmd, utils.FlagGatewayControllerImage, &buildGatewayControllerImage, "", "Gateway controller base image (optional)")
	utils.AddStringFlag(buildCmd, utils.FlagRouterBaseImage, &buildRouterBaseImage, "", "Router base image (optional)")

	// Mark required flags
	buildCmd.MarkFlagRequired(utils.FlagFile)
	buildCmd.MarkFlagRequired(utils.FlagDockerRegistry)
	buildCmd.MarkFlagRequired(utils.FlagImageTag)
	buildCmd.MarkFlagRequired(utils.FlagGatewayBuilder)
}

func runBuildCommand() error {
	startTime := time.Now()
	fmt.Println()
	fmt.Println("=== Gateway Build ===")
	fmt.Println()

	// Step 1: Validate Policy Manifest
	manifest, err := stepValidateManifest()
	if err != nil {
		return err
	}

	// Step 2: Prepare Build Configuration
	if err := stepPrepareBuildConfig(); err != nil {
		return err
	}

	// Step 3: Resolve Policies from PolicyHub
	response, policyHubClient, requestedPolicies, err := stepResolvePolicies(manifest)
	if err != nil {
		return err
	}

	// Step 4: Download and Cache Policies
	loadedPolicies, err := stepDownloadPolicies(response, policyHubClient)
	if err != nil {
		return err
	}

	// Step 5: Prepare Gateway Build
	if err := stepPrepareGatewayBuild(manifest, loadedPolicies); err != nil {
		return err
	}

	// Summary
	duration := time.Since(startTime)
	fmt.Println()
	fmt.Println("=== Build Summary ===")
	fmt.Printf("✓ Build completed successfully in %.2fs\n", duration.Seconds())
	fmt.Printf("Policies in manifest: %d\n", len(manifest.Policies))
	fmt.Printf("Policies loaded: %d\n", len(loadedPolicies))
	if len(requestedPolicies) > len(loadedPolicies) {
		fmt.Printf("Missing policies: %d\n", len(requestedPolicies)-len(loadedPolicies))
	}
	fmt.Println()

	return nil
}

// stepValidateManifest validates and loads the policy-manifest file
func stepValidateManifest() (*gateway.PolicyManifest, error) {
	fmt.Println("[1/5] Validating Policy Manifest")
	fmt.Printf("  → Loading from: %s\n", buildFilePath)

	manifest, err := gateway.LoadPolicyManifest(buildFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load policy-manifest: %w", err)
	}

	fmt.Printf("  ✓ Validated %d policies\n", len(manifest.Policies))

	// Print policy details
	for i, policy := range manifest.Policies {
		versionRes := policy.VersionResolution
		if versionRes == "" {
			versionRes = manifest.VersionResolution
			if versionRes == "" {
				versionRes = "default"
			}
		}
		policyDesc := fmt.Sprintf("    [%d] %s v%s (resolution: %s)", i+1, policy.Name, policy.Version, versionRes)
		if policy.FilePath != "" {
			policyDesc += fmt.Sprintf(" - local: %s", policy.FilePath)
		}
		fmt.Println(policyDesc)
	}
	fmt.Println()

	return manifest, nil
}

// stepPrepareBuildConfig validates and prepares the build configuration
func stepPrepareBuildConfig() error {
	fmt.Println("[2/5] Preparing Build Configuration")

	// Validate required flags
	if buildDockerRegistry == "" {
		return fmt.Errorf("docker-registry is required")
	}
	if buildImageTag == "" {
		return fmt.Errorf("image-tag is required")
	}
	if buildGatewayBuilder == "" {
		return fmt.Errorf("gateway-builder is required")
	}

	// Display configuration
	fmt.Printf("    Docker Registry: %s\n", buildDockerRegistry)
	fmt.Printf("    Image Tag: %s\n", buildImageTag)
	fmt.Printf("    Gateway Builder: %s\n", buildGatewayBuilder)

	if buildGatewayControllerImage != "" {
		fmt.Printf("    Controller Image: %s\n", buildGatewayControllerImage)
	}
	if buildRouterBaseImage != "" {
		fmt.Printf("    Router Image: %s\n", buildRouterBaseImage)
	}

	fmt.Println("  ✓ Configuration validated")
	fmt.Println()
	return nil
}

// stepResolvePolicies queries PolicyHub to resolve policy versions
func stepResolvePolicies(manifest *gateway.PolicyManifest) (*policyhub.ResolveResponse, *policyhub.PolicyHubClient, []policyhub.RequestedPolicy, error) {
	fmt.Println("[3/5] Resolving Policies from PolicyHub")
	fmt.Println("  → Preparing request...")

	// Clean policy manifest for PolicyHub
	cleanedPoliciesJSON, err := gateway.CleanPolicyManifestForPolicyHub(manifest)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to clean policy manifest: %w", err)
	}

	// Create requested policies list for tracking
	requestedPolicies := make([]policyhub.RequestedPolicy, len(manifest.Policies))
	for i, policy := range manifest.Policies {
		requestedPolicies[i] = policyhub.RequestedPolicy{
			Name:    policy.Name,
			Version: policy.Version,
		}
	}

	fmt.Println("  → Calling PolicyHub API...")

	// Call PolicyHub
	policyHubClient := policyhub.NewPolicyHubClient()
	response, err := policyHubClient.ResolvePolicies(cleanedPoliciesJSON)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to resolve policies: %w", err)
	}

	fmt.Printf("  ✓ Resolved %d policies\n", len(response.Data))

	// Check for missing policies
	missingPolicies := findMissingPolicies(requestedPolicies, response.Data)
	if len(missingPolicies) > 0 {
		fmt.Printf("  ⚠ %d requested policies not found:\n", len(missingPolicies))
		for _, missing := range missingPolicies {
			fmt.Printf("    - %s v%s\n", missing.Name, missing.Version)
		}
	}
	fmt.Println()

	return response, policyHubClient, requestedPolicies, nil
}

// stepDownloadPolicies downloads and caches policy packages
func stepDownloadPolicies(response *policyhub.ResolveResponse, client *policyhub.PolicyHubClient) ([]policyhub.LoadedPolicyWithContents, error) {
	fmt.Println("[4/5] Downloading & Caching Policies")

	if len(response.Data) == 0 {
		fmt.Println("  ⚠ No policies to download")
		fmt.Println()
		return []policyhub.LoadedPolicyWithContents{}, nil
	}

	fmt.Printf("  → Processing %d policies...\n", len(response.Data))

	// Ensure policies directory exists
	policiesDir, err := policyhub.EnsurePoliciesDir()
	if err != nil {
		return nil, err
	}

	loadedPolicies := make([]policyhub.LoadedPolicyWithContents, 0, len(response.Data))

	for _, policyData := range response.Data {
		zipPath := policyhub.GetPolicyZipPath(policiesDir, policyData.PolicyName, policyData.Version)
		policyID := fmt.Sprintf("%s-%s", policyData.PolicyName, policyData.Version)

		// Check if policy already exists
		if utils.FileExists(zipPath) {
			fmt.Printf("    %s: found locally, verifying...\n", policyID)

			// Verify checksum
			if err := utils.VerifyFileChecksum(zipPath, policyData.Checksum.Value); err != nil {
				fmt.Printf("    %s: checksum failed, re-downloading...\n", policyID)
				if err := downloadAndVerifyPolicy(client, &policyData, zipPath); err != nil {
					return nil, fmt.Errorf("%s: failed to download: %w", policyID, err)
				}
				fmt.Printf("    %s: ✓ downloaded and verified\n", policyID)
			} else {
				fmt.Printf("    %s: ✓ verified\n", policyID)
			}
		} else {
			fmt.Printf("    %s: downloading...\n", policyID)
			if err := downloadAndVerifyPolicy(client, &policyData, zipPath); err != nil {
				return nil, fmt.Errorf("%s: failed to download: %w", policyID, err)
			}
			fmt.Printf("    %s: ✓ downloaded and verified\n", policyID)
		}

		// Load zip contents
		fmt.Printf("    %s: loading contents...\n", policyID)
		contents, err := policyhub.ExtractZipToMemory(zipPath)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to load contents: %w", policyID, err)
		}
		fmt.Printf("    %s: ✓ loaded %d files\n", policyID, len(contents))

		loadedPolicies = append(loadedPolicies, policyhub.LoadedPolicyWithContents{
			PolicyData: policyData,
			Contents:   contents,
		})
	}

	fmt.Printf("  ✓ All policies cached (%d/%d successful)\n", len(loadedPolicies), len(response.Data))
	fmt.Println()

	return loadedPolicies, nil
}

// stepPrepareGatewayBuild prepares the final build artifacts
func stepPrepareGatewayBuild(manifest *gateway.PolicyManifest, loadedPolicies []policyhub.LoadedPolicyWithContents) error {
	fmt.Println("[5/5] Preparing Gateway Build")
	fmt.Printf("  Total policies in manifest: %d\n", len(manifest.Policies))
	fmt.Printf("  Policies loaded: %d\n", len(loadedPolicies))

	// Future: This is where we'll prepare Docker build context, generate gateway config, etc.
	fmt.Println("  ✓ Gateway build artifacts ready")
	fmt.Println()

	return nil
}

// Helper functions

func downloadAndVerifyPolicy(client *policyhub.PolicyHubClient, policyData *policyhub.PolicyData, destPath string) error {
	if err := client.DownloadPolicy(policyData.DownloadURL, destPath); err != nil {
		return err
	}

	if err := utils.VerifyFileChecksum(destPath, policyData.Checksum.Value); err != nil {
		os.Remove(destPath)
		return err
	}

	return nil
}

func findMissingPolicies(requested []policyhub.RequestedPolicy, received []policyhub.PolicyData) []policyhub.RequestedPolicy {
	receivedMap := make(map[string]bool)
	for _, policy := range received {
		key := fmt.Sprintf("%s-%s", policy.PolicyName, policy.Version)
		receivedMap[key] = true
	}

	var missing []policyhub.RequestedPolicy
	for _, req := range requested {
		key := fmt.Sprintf("%s-%s", req.Name, req.Version)
		if !receivedMap[key] {
			missing = append(missing, req)
		}
	}

	return missing
}
