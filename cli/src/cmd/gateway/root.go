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

package gateway

import (
	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/cmd/gateway/mcp"
)

const (
	GatewayCmdLiteral = "gateway"
	GatewayCmdExample = `# Add a new gateway
apipctl gateway add --name dev --server http://localhost:9090

# Generate MCP configuration
apipctl gateway mcp generate --server http://localhost:3001/mcp --output target`
)

// GatewayCmd represents the gateway command
var GatewayCmd = &cobra.Command{
	Use:     GatewayCmdLiteral,
	Short:   "Execute API Platform Gateway operations",
	Long:    "This command allows you to execute various operations related to the WSO2 API Platform Gateway.",
	Example: GatewayCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	// Register subcommands
	GatewayCmd.AddCommand(addCmd)
	GatewayCmd.AddCommand(mcp.McpCmd)
}
