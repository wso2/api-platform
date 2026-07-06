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
	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/cmd/aiws/llmprovider"
	"github.com/wso2/api-platform/cli/cmd/aiws/llmproxy"
	"github.com/wso2/api-platform/cli/cmd/aiws/mcpproxy"
)

const (
	AiWSCmdLiteral = "ai-workspace"
	AiWSCmdExample = `# Add a new AI-Workspace
ap ai-workspace add --display-name my-portal --platform eu --server https://ai-workspace.example.com --auth api-key`
)

var AiWSCmd = &cobra.Command{
	Use:     AiWSCmdLiteral,
	Short:   "Execute AI-Workspace operations",
	Long:    "This command allows you to execute various operations related to AI-Workspaces.",
	Example: AiWSCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	AiWSCmd.AddCommand(addCmd)
	AiWSCmd.AddCommand(listCmd)
	AiWSCmd.AddCommand(removeCmd)
	AiWSCmd.AddCommand(useCmd)
	AiWSCmd.AddCommand(currentCmd)
	AiWSCmd.AddCommand(buildCmd)
	AiWSCmd.AddCommand(llmprovider.LLMProviderCmd)
	AiWSCmd.AddCommand(llmproxy.LLMProxyCmd)
	AiWSCmd.AddCommand(mcpproxy.MCPProxyCmd)
}
