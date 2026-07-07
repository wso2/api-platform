/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package aiws

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/aiworkspace"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	ApplyCmdLiteral = "apply"
	ApplyCmdExample = `# Create or update the AI workspace artifact from the current project
ap ai-workspace apply

# Apply a proxy or MCP artifact (--project-id is required for those kinds)
ap ai-workspace apply --project-id <project-id>

# Resolve ENV_CLI_* placeholders from a specific env file (defaults to .env in the project root)
ap ai-workspace apply --env-file ./values.env

# Apply from a specific project directory using a specific AI workspace
ap ai-workspace apply -f /path/to/project --project-id <project-id> --display-name my-workspace --platform eu`
)

var (
	applyProjectDir string
	applyProjectID  string
	applyEnvFile    string
	applyName       string
	applyPlatform   string
	applyInsecure   bool
	applyOutput     string
)

var applyCmd = &cobra.Command{
	Use:   ApplyCmdLiteral,
	Short: "Create or update an AI workspace artifact",
	Long: "Generate the payload from the project's metadata.yaml, runtime.yaml and definition.yaml, then " +
		"create or update the artifact on the WSO2 API Platform AI workspace. The artifact is identified by " +
		"metadata.name: apply looks it up first and updates it (PUT) when it already exists, otherwise it " +
		"creates it (POST) — mirroring `ap gateway apply`. The target endpoint is selected by the artifact kind " +
		"declared in the project (LlmProvider, LlmProxy, Mcp). For the LlmProxy and Mcp kinds --project-id is " +
		"required.",
	Example: ApplyCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runApplyCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(applyCmd, utils.FlagFile, &applyProjectDir, "", "Path to the project directory (defaults to current directory)")
	utils.AddStringFlag(applyCmd, utils.FlagProjectID, &applyProjectID, "", "Project ID (required for LlmProxy and Mcp kinds)")
	utils.AddStringFlag(applyCmd, utils.FlagEnvFile, &applyEnvFile, "", "Path to an env file resolving ENV_CLI_* placeholders (defaults to .env in the project root)")
	utils.AddStringFlag(applyCmd, utils.FlagName, &applyName, "", "AI workspace display name")
	utils.AddStringFlag(applyCmd, utils.FlagPlatform, &applyPlatform, "", "Platform name")
	utils.AddStringFlag(applyCmd, utils.FlagOutput, &applyOutput, "", "Output format: \"json\" prints the full server response (default: summary)")
	applyCmd.Flags().BoolVar(&applyInsecure, "insecure", false, "Skip TLS certificate verification")
}

func runApplyCommand() error {
	projectRoot, wsConfig, err := resolveProjectAIWorkspace(applyProjectDir)
	if err != nil {
		return err
	}

	artifact, err := loadAIWorkspaceArtifact(projectRoot, wsConfig)
	if err != nil {
		return fmt.Errorf("AI workspace validation failed for %q: %w", wsConfig.Name, err)
	}

	body, projectID, err := marshalAIWorkspacePayload(artifact, applyProjectID)
	if err != nil {
		return err
	}

	// Resolve ENV_CLI_* placeholders carried from metadata.yaml/runtime.yaml
	// into the generated payload before it is sent.
	body, err = resolveEnvPlaceholders(body, projectRoot, applyEnvFile)
	if err != nil {
		return err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	aiWorkspace, _, err := aiworkspace.ResolveAIWorkspace(cfg, applyName, applyPlatform)
	if err != nil {
		return err
	}

	client := aiworkspace.NewClientWithOptions(aiWorkspace, applyInsecure)

	resp, action, err := applyAIWorkspaceArtifact(client, artifact, body)
	if err != nil {
		return err
	}

	return aiworkspace.PrintApplyResult(resp, applyOutput, artifact.BaseKind, action, artifact.ResourceName, projectID)
}

// applyAIWorkspaceArtifact creates or updates the artifact on the AI workspace,
// choosing the verb the way `ap gateway apply` does: it probes for an existing
// artifact by metadata.name (the resource id) and PUTs an update when one is
// found, otherwise POSTs a create. It returns the server response and the
// past-tense action ("updated" or "applied") for the result summary.
func applyAIWorkspaceArtifact(client *aiworkspace.Client, artifact *aiWorkspaceArtifact, body []byte) (*http.Response, string, error) {
	updatePath := aiWorkspaceUpdatePath(artifact.BaseKind, artifact.ResourceName)

	exists, err := client.Exists(updatePath)
	if err != nil {
		return nil, "", aiworkspace.WrapRequestError(fmt.Sprintf("check %s existence", artifact.BaseKind), err, applyInsecure)
	}

	if exists {
		resp, err := client.PutJSON(updatePath, body)
		if err != nil {
			return nil, "", aiworkspace.WrapRequestError(fmt.Sprintf("update %s", artifact.BaseKind), err, applyInsecure)
		}
		return resp, "updated", nil
	}

	resp, err := client.PostJSON(aiWorkspaceCreatePath(artifact.BaseKind), body)
	if err != nil {
		return nil, "", aiworkspace.WrapRequestError(fmt.Sprintf("apply %s", artifact.BaseKind), err, applyInsecure)
	}
	return resp, "applied", nil
}
