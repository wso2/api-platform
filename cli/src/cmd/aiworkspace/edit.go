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
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/aiworkspace"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	EditCmdLiteral = "edit"
	EditCmdExample = `# Generate and update the AI workspace artifact from the current project
ap ai-workspace edit

# Update a proxy or MCP artifact (--project-id is required for those kinds)
ap ai-workspace edit --project-id <project-id>

# Update from a specific project directory using a specific AI workspace
ap ai-workspace edit -f /path/to/project --project-id <project-id> --display-name my-workspace --platform eu`
)

var (
	editProjectDir string
	editProjectID  string
	editEnvFile    string
	editName       string
	editPlatform   string
	editInsecure   bool
	editOutput     string
)

var editCmd = &cobra.Command{
	Use:   EditCmdLiteral,
	Short: "Generate and update an existing AI workspace artifact",
	Long: "Generate the payload from the project's metadata.yaml, runtime.yaml and definition.yaml, then " +
		"update the existing artifact on the WSO2 API Platform AI workspace with a PUT request. The target " +
		"endpoint is selected by the artifact kind declared in the project (LlmProvider, LlmProxy, Mcp) and the " +
		"artifact is identified by metadata.name. For the LlmProxy and Mcp kinds --project-id is required.",
	Example: EditCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runEditCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(editCmd, utils.FlagFile, &editProjectDir, "", "Path to the project directory (defaults to current directory)")
	utils.AddStringFlag(editCmd, utils.FlagProjectID, &editProjectID, "", "Project ID (required for LlmProxy and Mcp kinds)")
	utils.AddStringFlag(editCmd, utils.FlagEnvFile, &editEnvFile, "", "Path to an env file resolving ENV_CLI_* placeholders (defaults to .env in the project root)")
	utils.AddStringFlag(editCmd, utils.FlagName, &editName, "", "AI workspace display name")
	utils.AddStringFlag(editCmd, utils.FlagPlatform, &editPlatform, "", "Platform name")
	utils.AddStringFlag(editCmd, utils.FlagOutput, &editOutput, "", "Output format: \"json\" prints the full server response (default: summary)")
	editCmd.Flags().BoolVar(&editInsecure, "insecure", false, "Skip TLS certificate verification")
}

func runEditCommand() error {
	projectRoot, wsConfig, err := resolveProjectAIWorkspace(editProjectDir)
	if err != nil {
		return err
	}

	artifact, err := loadAIWorkspaceArtifact(projectRoot, wsConfig)
	if err != nil {
		return fmt.Errorf("AI workspace validation failed for %q: %w", wsConfig.Name, err)
	}

	body, projectID, err := marshalAIWorkspacePayload(artifact, editProjectID)
	if err != nil {
		return err
	}

	// Resolve ENV_CLI_* placeholders carried from metadata.yaml/runtime.yaml
	// into the generated payload before it is sent.
	body, err = resolveEnvPlaceholders(body, projectRoot, editEnvFile)
	if err != nil {
		return err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	aiWorkspace, _, err := aiworkspace.ResolveAIWorkspace(cfg, editName, editPlatform)
	if err != nil {
		return err
	}

	client := aiworkspace.NewClientWithOptions(aiWorkspace, editInsecure)
	resp, err := client.PutJSON(aiWorkspaceUpdatePath(artifact.BaseKind, artifact.ResourceName), body)
	if err != nil {
		return aiworkspace.WrapRequestError(fmt.Sprintf("update %s", artifact.BaseKind), err, editInsecure)
	}

	return aiworkspace.PrintApplyResult(resp, editOutput, artifact.BaseKind, "updated", artifact.ResourceName, projectID)
}
