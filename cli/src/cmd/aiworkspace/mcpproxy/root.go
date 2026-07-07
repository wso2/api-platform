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
	"github.com/spf13/cobra"
)

const (
	MCPProxyCmdLiteral = "mcp-proxy"
	MCPProxyCmdExample = `# List MCP proxies in a project on the AI workspace
ap ai-workspace mcp-proxy list --project-id <project-id>

# Create or update an MCP proxy from a project with:
#   ap ai-workspace apply --project-id <project-id>`
)

// MCPProxyCmd is the parent command for MCP proxy operations.
var MCPProxyCmd = &cobra.Command{
	Use:     MCPProxyCmdLiteral,
	Short:   "Manage MCP proxies on the AI workspace",
	Long:    "This command allows you to manage MCP proxies on the WSO2 API Platform AI workspace.",
	Example: MCPProxyCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	MCPProxyCmd.AddCommand(listCmd)
	MCPProxyCmd.AddCommand(getCmd)
	MCPProxyCmd.AddCommand(deleteCmd)
}
