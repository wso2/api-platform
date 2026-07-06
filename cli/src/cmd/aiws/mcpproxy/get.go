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
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/aiworkspace"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	GetCmdLiteral = "get"
	GetCmdExample = `# List all MCP proxies in a project
ap ai-workspace mcp-proxy get --project-id <project-id>

# Get a single MCP proxy by ID
ap ai-workspace mcp-proxy get --id bijira-mcp-everything

# List using a specific AI workspace with pagination
ap ai-workspace mcp-proxy get --project-id <project-id> --limit 50 --offset 0 --display-name my-workspace --platform eu`
)

var (
	getID        string
	getProjectID string
	getLimit     string
	getOffset    string
	getName      string
	getPlatform  string
	getInsecure  bool
)

var getCmd = &cobra.Command{
	Use:     GetCmdLiteral,
	Short:   "Get one or all MCP proxies from the AI workspace",
	Long:    "Retrieve MCP proxies from the WSO2 API Platform AI workspace. With --id a single proxy is fetched by its identifier; without it all proxies in the project are listed (--project-id is required for listing).",
	Example: GetCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGetCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(getCmd, utils.FlagID, &getID, "", "MCP proxy ID (omit to list all)")
	utils.AddStringFlag(getCmd, utils.FlagProjectID, &getProjectID, "", "Project ID (required when listing)")
	utils.AddStringFlag(getCmd, utils.FlagLimit, &getLimit, "", "Maximum number of proxies to return when listing")
	utils.AddStringFlag(getCmd, utils.FlagOffset, &getOffset, "", "Number of proxies to skip when listing")
	utils.AddStringFlag(getCmd, utils.FlagName, &getName, "", "AI workspace display name")
	utils.AddStringFlag(getCmd, utils.FlagPlatform, &getPlatform, "", "Platform name")
	getCmd.Flags().BoolVar(&getInsecure, "insecure", false, "Skip TLS certificate verification")
}

func runGetCommand() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	aiWorkspace, _, err := aiworkspace.ResolveAIWorkspace(cfg, getName, getPlatform)
	if err != nil {
		return err
	}

	client := aiworkspace.NewClientWithOptions(aiWorkspace, getInsecure)

	var path, action string
	if id := strings.TrimSpace(getID); id != "" {
		// Fetching a single proxy takes only the id path parameter.
		path = aiworkspace.MCPProxyByIDPath(id)
		action = "get mcp proxy"
	} else {
		projectID := strings.TrimSpace(getProjectID)
		if projectID == "" {
			return fmt.Errorf("project ID is required when listing proxies (use --id to get a single proxy)")
		}
		path = aiworkspace.MCPProxyListPath(projectID, aiworkspace.ListQuery{Limit: getLimit, Offset: getOffset})
		action = "list mcp proxies"
	}

	resp, err := client.Get(path)
	if err != nil {
		return aiworkspace.WrapRequestError(action, err, getInsecure)
	}

	return aiworkspace.PrintJSONResponse(resp)
}
