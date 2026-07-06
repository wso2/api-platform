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
	PushCmdLiteral = "push"
	PushCmdExample = `# Generate and push the AI workspace artifact from the current project
ap ai-workspace push

# Push a proxy or MCP artifact (--project-id is required for those kinds)
ap ai-workspace push --project-id <project-id>

# Push from a specific project directory using a specific AI workspace
ap ai-workspace push -f /path/to/project --project-id <project-id> --display-name my-workspace --platform eu`
)

var (
	pushProjectDir string
	pushProjectID  string
	pushName       string
	pushPlatform   string
	pushInsecure   bool
	pushOutput     string
)

var pushCmd = &cobra.Command{
	Use:   PushCmdLiteral,
	Short: "Generate and push an AI workspace artifact",
	Long: "Generate the creation payload from the project's metadata.yaml, runtime.yaml and definition.yaml, " +
		"then create the artifact on the WSO2 API Platform AI workspace. The target endpoint is selected by the " +
		"artifact kind declared in the project (LlmProvider, LlmProxy, Mcp). For the LlmProxy and Mcp kinds " +
		"--project-id is required.",
	Example: PushCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPushCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(pushCmd, utils.FlagFile, &pushProjectDir, "", "Path to the project directory (defaults to current directory)")
	utils.AddStringFlag(pushCmd, utils.FlagProjectID, &pushProjectID, "", "Project ID (required for LlmProxy and Mcp kinds)")
	utils.AddStringFlag(pushCmd, utils.FlagName, &pushName, "", "AI workspace display name")
	utils.AddStringFlag(pushCmd, utils.FlagPlatform, &pushPlatform, "", "Platform name")
	utils.AddStringFlag(pushCmd, utils.FlagOutput, &pushOutput, "", "Output format: \"json\" prints the full server response (default: summary)")
	pushCmd.Flags().BoolVar(&pushInsecure, "insecure", false, "Skip TLS certificate verification")
}

func runPushCommand() error {
	projectRoot, wsConfig, err := resolveProjectAIWorkspace(pushProjectDir)
	if err != nil {
		return err
	}

	artifact, err := loadAIWorkspaceArtifact(projectRoot, wsConfig)
	if err != nil {
		return fmt.Errorf("AI workspace validation failed for %q: %w", wsConfig.Name, err)
	}

	body, projectID, err := marshalAIWorkspacePayload(artifact, pushProjectID)
	if err != nil {
		return err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	aiWorkspace, _, err := aiworkspace.ResolveAIWorkspace(cfg, pushName, pushPlatform)
	if err != nil {
		return err
	}

	client := aiworkspace.NewClientWithOptions(aiWorkspace, pushInsecure)
	resp, err := client.PostJSON(aiWorkspaceCreatePath(artifact.BaseKind), body)
	if err != nil {
		return aiworkspace.WrapRequestError(fmt.Sprintf("push %s", artifact.BaseKind), err, pushInsecure)
	}

	return aiworkspace.PrintApplyResult(resp, pushOutput, artifact.BaseKind, "applied", artifact.ResourceName, projectID)
}
