/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package mcp

import (
	"github.com/spf13/cobra"
)

const (
	McpCmdLiteral = "mcp"
	McpCmdExample = `# Generate MCP configuration
apipctl gateway mcp generate --server http://localhost:3001/mcp --output target`
)

// McpCmd represents the mcp command
var McpCmd = &cobra.Command{
	Use:     McpCmdLiteral,
	Short:   "MCP related operations",
	Long:    "Execute MCP (Model Context Protocol) related operations for the WSO2 API Platform Gateway.",
	Example: McpCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	McpCmd.AddCommand(generateCmd)
}
