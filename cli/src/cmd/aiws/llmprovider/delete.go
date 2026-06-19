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

package llmprovider

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/aiworkspace"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	DeleteCmdLiteral = "delete"
	DeleteCmdExample = `# Delete an LLM provider by ID using the active AI workspace
ap ai-ws llm-provider delete --id wso2-claude

# Delete using a specific AI workspace
ap ai-ws llm-provider delete --id wso2-claude --display-name my-workspace --platform eu`
)

var (
	deleteID       string
	deleteName     string
	deletePlatform string
	deleteInsecure bool
)

var deleteCmd = &cobra.Command{
	Use:     DeleteCmdLiteral,
	Short:   "Delete an LLM provider from the AI workspace",
	Long:    "Delete an LLM provider from the WSO2 API Platform AI workspace by its identifier.",
	Example: DeleteCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDeleteCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(deleteCmd, utils.FlagID, &deleteID, "", "LLM provider ID (required)")
	utils.AddStringFlag(deleteCmd, utils.FlagName, &deleteName, "", "AI workspace display name")
	utils.AddStringFlag(deleteCmd, utils.FlagPlatform, &deletePlatform, "", "Platform name")
	deleteCmd.Flags().BoolVar(&deleteInsecure, "insecure", false, "Skip TLS certificate verification")
	_ = deleteCmd.MarkFlagRequired(utils.FlagID)
}

func runDeleteCommand() error {
	id := strings.TrimSpace(deleteID)
	if id == "" {
		return fmt.Errorf("LLM provider ID is required")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	aiWorkspace, resolvedPlatform, err := aiworkspace.ResolveAIWorkspace(cfg, deleteName, deletePlatform)
	if err != nil {
		return err
	}

	client := aiworkspace.NewClientWithOptions(aiWorkspace, deleteInsecure)
	resp, err := client.Delete(aiworkspace.ProviderByIDPath(id))
	if err != nil {
		return aiworkspace.WrapRequestError("delete llm provider", err, deleteInsecure)
	}
	resp.Body.Close()

	fmt.Printf("LLM provider %q deleted from ai-workspace %s (platform: %s)\n", id, aiWorkspace.Name, resolvedPlatform)
	return nil
}
