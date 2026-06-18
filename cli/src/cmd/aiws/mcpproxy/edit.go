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

package mcpproxy

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
	EditCmdExample = `# Update an existing MCP proxy using the active AI workspace
ap ai-ws mcp-proxy edit -f build/bijira-mcp-everything.json --org <org-id> --project-id <project-id>

# Update using a specific AI workspace
ap ai-ws mcp-proxy edit -f build/bijira-mcp-everything.json --org <org-id> --project-id <project-id> --display-name my-workspace --platform eu`
)

var (
	editFilePath  string
	editOrgID     string
	editProjectID string
	editName      string
	editPlatform  string
	editInsecure  bool
)

var editCmd = &cobra.Command{
	Use:     EditCmdLiteral,
	Short:   "Update an existing MCP proxy on the AI workspace",
	Long:    "Update an existing MCP proxy on the WSO2 API Platform AI workspace by sending its JSON payload with a PUT request. The proxy is identified by the \"id\" field in the payload and the supplied project ID is injected into the payload before it is sent.",
	Example: EditCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runEditCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(editCmd, utils.FlagFile, &editFilePath, "", "Path to the MCP proxy payload JSON file (required)")
	utils.AddStringFlag(editCmd, utils.FlagOrgID, &editOrgID, "", "Organization ID (required)")
	utils.AddStringFlag(editCmd, utils.FlagProjectID, &editProjectID, "", "Project ID to set on the payload (required)")
	utils.AddStringFlag(editCmd, utils.FlagName, &editName, "", "AI workspace display name")
	utils.AddStringFlag(editCmd, utils.FlagPlatform, &editPlatform, "", "Platform name")
	editCmd.Flags().BoolVar(&editInsecure, "insecure", false, "Skip TLS certificate verification")
	_ = editCmd.MarkFlagRequired(utils.FlagFile)
	_ = editCmd.MarkFlagRequired(utils.FlagOrgID)
	_ = editCmd.MarkFlagRequired(utils.FlagProjectID)
}

func runEditCommand() error {
	orgID := strings.TrimSpace(editOrgID)
	if orgID == "" {
		return fmt.Errorf("organization ID is required")
	}
	projectID := strings.TrimSpace(editProjectID)
	if projectID == "" {
		return fmt.Errorf("project ID is required")
	}

	raw, err := aiworkspace.ReadJSONFile(editFilePath)
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

	aiWorkspace, resolvedPlatform, err := aiworkspace.ResolveAIWorkspace(cfg, editName, editPlatform)
	if err != nil {
		return err
	}

	client := aiworkspace.NewClientWithOptions(aiWorkspace, editInsecure)
	resp, err := client.PutJSON(aiworkspace.MCPProxyResourcePath(orgID, proxyID), body)
	if err != nil {
		return aiworkspace.WrapRequestError("update mcp proxy", err, editInsecure)
	}

	fmt.Printf("MCP proxy %q updated on ai-workspace %s (platform: %s)\n", proxyID, aiWorkspace.Name, resolvedPlatform)
	return aiworkspace.PrintJSONResponse(resp)
}
