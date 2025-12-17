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

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
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

	// Optional flags
	utils.AddStringFlag(buildCmd, utils.FlagGatewayBuilder, &buildGatewayBuilder, "", "Gateway builder name (optional)")
	utils.AddStringFlag(buildCmd, utils.FlagGatewayControllerImage, &buildGatewayControllerImage, "", "Gateway controller base image (optional)")
	utils.AddStringFlag(buildCmd, utils.FlagRouterBaseImage, &buildRouterBaseImage, "", "Router base image (optional)")

	// Mark required flags
	buildCmd.MarkFlagRequired(utils.FlagFile)
	buildCmd.MarkFlagRequired(utils.FlagDockerRegistry)
	buildCmd.MarkFlagRequired(utils.FlagImageTag)
}

func runBuildCommand() error {
	fmt.Println("=== Gateway Build ===")
	fmt.Println()

	// Load and validate the policy-manifest
	fmt.Printf("Loading policy-manifest from: %s\n", buildFilePath)
	manifest, err := gateway.LoadPolicyManifest(buildFilePath)
	if err != nil {
		return fmt.Errorf("failed to load policy-manifest: %w", err)
	}
	fmt.Println("✓ Policy-manifest validation successful")
	fmt.Println()

	// Convert manifest to JSON
	manifestYAML, err := os.ReadFile(buildFilePath)
	if err != nil {
		return fmt.Errorf("failed to read policy-manifest: %w", err)
	}

	manifestJSON, err := utils.ConvertYAMLToJSONPretty(manifestYAML)
	if err != nil {
		return fmt.Errorf("failed to convert policy-manifest to JSON: %w", err)
	}

	// Print the JSON representation
	fmt.Println("Policy Manifest (JSON):")
	fmt.Println(string(manifestJSON))
	fmt.Println()

	// Print flags and their values
	fmt.Println("Build Configuration:")
	fmt.Printf("  %-35s: %s\n", utils.FlagFile, buildFilePath)
	fmt.Printf("  %-35s: %s\n", utils.FlagDockerRegistry, buildDockerRegistry)
	fmt.Printf("  %-35s: %s\n", utils.FlagImageTag, buildImageTag)

	// Print optional flags
	if buildGatewayBuilder != "" {
		fmt.Printf("  %-35s: %s\n", utils.FlagGatewayBuilder, buildGatewayBuilder)
	} else {
		fmt.Printf("  %-35s: (not provided)\n", utils.FlagGatewayBuilder)
	}

	if buildGatewayControllerImage != "" {
		fmt.Printf("  %-35s: %s\n", utils.FlagGatewayControllerImage, buildGatewayControllerImage)
	} else {
		fmt.Printf("  %-35s: (not provided)\n", utils.FlagGatewayControllerImage)
	}

	if buildRouterBaseImage != "" {
		fmt.Printf("  %-35s: %s\n", utils.FlagRouterBaseImage, buildRouterBaseImage)
	} else {
		fmt.Printf("  %-35s: (not provided)\n", utils.FlagRouterBaseImage)
	}

	fmt.Println()

	// Print summary
	fmt.Printf("Total Policies: %d\n", len(manifest.Policies))
	for i, policy := range manifest.Policies {
		versionRes := policy.VersionResolution
		if versionRes == "" {
			versionRes = manifest.VersionResolution
			if versionRes == "" {
				versionRes = "(default)"
			}
		}
		fmt.Printf("  [%d] %s v%s (resolution: %s)", i+1, policy.Name, policy.Version, versionRes)
		if policy.FilePath != "" {
			fmt.Printf(" - local: %s", policy.FilePath)
		}
		fmt.Println()
	}
	fmt.Println()

	// Placeholder for PolicyHub call
	fmt.Println("=== Call PolicyHub ===")
	fmt.Println("(This will be implemented in the next phase)")

	return nil
}
