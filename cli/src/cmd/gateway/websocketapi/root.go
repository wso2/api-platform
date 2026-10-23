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

package websocketapi

import (
	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/cmd/gateway/websocketapi/apikey"
)

const (
	APICmdLiteral = "websocket-api"
	APICmdExample = `# List all WebSocket APIs
ap gateway websocket-api list`
)

// APICmd represents the websocket-api command
var APICmd = &cobra.Command{
	Use:     APICmdLiteral,
	Short:   "Manage WebSocket APIs on the gateway",
	Long:    "This command allows you to manage WebSocket APIs on the WSO2 API Platform Gateway.",
	Example: APICmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	// Register subcommands
	APICmd.AddCommand(listCmd)
	APICmd.AddCommand(getCmd)
	APICmd.AddCommand(deleteCmd)
	APICmd.AddCommand(apikey.APIKeyCmd)
}
