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
	PushCmdLiteral = "push"
	PushCmdExample = `# Push an LLM provider artifact using the active AI workspace
ap ai-ws llm-provider push -f build/wso2-claude.json --org <org-id>

# Push using a specific AI workspace
ap ai-ws llm-provider push -f build/wso2-claude.json --org <org-id> --display-name my-workspace --platform eu`
)

var (
	pushFilePath string
	pushOrgID    string
	pushName     string
	pushPlatform string
	pushInsecure bool
	pushOutput   string
)

var pushCmd = &cobra.Command{
	Use:     PushCmdLiteral,
	Short:   "Push an LLM provider artifact to the AI workspace",
	Long:    "Push a generated LLM provider creation payload (JSON) to the WSO2 API Platform AI workspace.",
	Example: PushCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPushCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(pushCmd, utils.FlagFile, &pushFilePath, "", "Path to the LLM provider payload JSON file (required)")
	utils.AddStringFlag(pushCmd, utils.FlagOrgID, &pushOrgID, "", "Organization ID (required)")
	utils.AddStringFlag(pushCmd, utils.FlagName, &pushName, "", "AI workspace display name")
	utils.AddStringFlag(pushCmd, utils.FlagPlatform, &pushPlatform, "", "Platform name")
	utils.AddStringFlag(pushCmd, utils.FlagOutput, &pushOutput, "", "Output format: \"json\" prints the full server response (default: summary)")
	pushCmd.Flags().BoolVar(&pushInsecure, "insecure", false, "Skip TLS certificate verification")
	_ = pushCmd.MarkFlagRequired(utils.FlagFile)
	_ = pushCmd.MarkFlagRequired(utils.FlagOrgID)
}

func runPushCommand() error {
	orgID := strings.TrimSpace(pushOrgID)
	if orgID == "" {
		return fmt.Errorf("organization ID is required")
	}

	payload, err := aiworkspace.ReadJSONFile(pushFilePath)
	if err != nil {
		return err
	}

	var meta struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(payload, &meta); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}
	providerID := strings.TrimSpace(meta.ID)
	if providerID == "" {
		providerName := strings.TrimSpace(meta.Name)
		providerID = aiworkspace.ResourceID(providerName, "llm-provider")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	aiWorkspace, resolvedPlatform, err := aiworkspace.ResolveAIWorkspace(cfg, pushName, pushPlatform)
	if err != nil {
		return err
	}

	client := aiworkspace.NewClientWithOptions(aiWorkspace, pushInsecure)
	resp, err := client.PostJSON(aiworkspace.ProviderPath(orgID), payload)
	if err != nil {
		return aiworkspace.WrapRequestError("push llm provider", err, pushInsecure)
	}

	return aiworkspace.PrintArtifactResult(resp, pushOutput, providerID,
		fmt.Sprintf("LLM provider pushed to ai-workspace %s (platform: %s)", aiWorkspace.Name, resolvedPlatform))
}
