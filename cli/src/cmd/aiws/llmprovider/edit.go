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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/aiworkspace"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	EditCmdLiteral = "edit"
	EditCmdExample = `# Update an existing LLM provider using the active AI workspace
ap ai-workspace llm-provider edit -f build/wso2-claude.json

# Update using a specific AI workspace
ap ai-workspace llm-provider edit -f build/wso2-claude.json --display-name my-workspace --platform eu`
)

var (
	editFilePath string
	editName     string
	editPlatform string
	editInsecure bool
	editOutput   string
)

var editCmd = &cobra.Command{
	Use:     EditCmdLiteral,
	Short:   "Update an existing LLM provider on the AI workspace",
	Long:    "Update an existing LLM provider on the WSO2 API Platform AI workspace by sending its JSON payload with a PUT request. The provider is identified by the \"id\" field in the payload.",
	Example: EditCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runEditCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(editCmd, utils.FlagFile, &editFilePath, "", "Path to the LLM provider payload JSON file (required)")
	utils.AddStringFlag(editCmd, utils.FlagName, &editName, "", "AI workspace display name")
	utils.AddStringFlag(editCmd, utils.FlagPlatform, &editPlatform, "", "Platform name")
	utils.AddStringFlag(editCmd, utils.FlagOutput, &editOutput, "", "Output format: \"json\" prints the full server response (default: summary)")
	editCmd.Flags().BoolVar(&editInsecure, "insecure", false, "Skip TLS certificate verification")
	_ = editCmd.MarkFlagRequired(utils.FlagFile)
}

func runEditCommand() error {
	payload, err := aiworkspace.ReadJSONFile(editFilePath)
	if err != nil {
		return err
	}

	var meta struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(payload, &meta); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}
	providerID := strings.TrimSpace(meta.ID)
	if providerID == "" {
		return fmt.Errorf("payload is missing the required \"id\" field")
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
	resp, err := client.PutJSON(aiworkspace.ProviderByIDPath(providerID), payload)
	if err != nil {
		return aiworkspace.WrapRequestError("update llm provider", err, editInsecure)
	}

	return aiworkspace.PrintApplyResult(resp, editOutput, "LlmProvider", "updated", providerID, "")
}
