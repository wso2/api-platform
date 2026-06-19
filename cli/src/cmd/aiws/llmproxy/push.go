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

package llmproxy

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
	PushCmdExample = `# Push an LLM proxy artifact using the active AI workspace
ap ai-ws llm-proxy push -f build/wso2-openai-proxy.json --org <org-id> --project-id <project-id>

# Push using a specific AI workspace
ap ai-ws llm-proxy push -f build/wso2-openai-proxy.json --org <org-id> --project-id <project-id> --display-name my-workspace --platform eu`
)

var (
	pushFilePath  string
	pushOrgID     string
	pushProjectID string
	pushName      string
	pushPlatform  string
	pushInsecure  bool
	pushOutput    string
)

var pushCmd = &cobra.Command{
	Use:     PushCmdLiteral,
	Short:   "Push an LLM proxy artifact to the AI workspace",
	Long:    "Push a generated LLM proxy creation payload (JSON) to the WSO2 API Platform AI workspace. The supplied project ID is injected into the payload before it is sent.",
	Example: PushCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPushCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(pushCmd, utils.FlagFile, &pushFilePath, "", "Path to the LLM proxy payload JSON file (required)")
	utils.AddStringFlag(pushCmd, utils.FlagOrgID, &pushOrgID, "", "Organization ID (required)")
	utils.AddStringFlag(pushCmd, utils.FlagProjectID, &pushProjectID, "", "Project ID to set on the payload (required)")
	utils.AddStringFlag(pushCmd, utils.FlagName, &pushName, "", "AI workspace display name")
	utils.AddStringFlag(pushCmd, utils.FlagPlatform, &pushPlatform, "", "Platform name")
	utils.AddStringFlag(pushCmd, utils.FlagOutput, &pushOutput, "", "Output format: \"json\" prints the full server response (default: summary)")
	pushCmd.Flags().BoolVar(&pushInsecure, "insecure", false, "Skip TLS certificate verification")
	_ = pushCmd.MarkFlagRequired(utils.FlagFile)
	_ = pushCmd.MarkFlagRequired(utils.FlagOrgID)
	_ = pushCmd.MarkFlagRequired(utils.FlagProjectID)
}

func runPushCommand() error {
	orgID := strings.TrimSpace(pushOrgID)
	if orgID == "" {
		return fmt.Errorf("organization ID is required")
	}
	projectID := strings.TrimSpace(pushProjectID)
	if projectID == "" {
		return fmt.Errorf("project ID is required")
	}

	raw, err := aiworkspace.ReadJSONFile(pushFilePath)
	if err != nil {
		return err
	}

	// Decode into a map so the project ID can be injected without dropping any
	// fields the build emitted.
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	proxyID, _ := payload["id"].(string)
	proxyID = strings.TrimSpace(proxyID)
	if proxyID == "" {
		return fmt.Errorf("payload is missing the required \"id\" field")
	}

	payload["projectId"] = projectID

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode payload: %w", err)
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
	resp, err := client.PostJSON(aiworkspace.ProxyPath(orgID), body)
	if err != nil {
		return aiworkspace.WrapRequestError("push llm proxy", err, pushInsecure)
	}

	return aiworkspace.PrintArtifactResult(resp, pushOutput, proxyID,
		fmt.Sprintf("LLM proxy pushed to ai-workspace %s (platform: %s)", aiWorkspace.Name, resolvedPlatform))
}
