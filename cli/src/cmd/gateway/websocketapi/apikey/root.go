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

package apikey

import (
	"github.com/spf13/cobra"
)

const (
	APIKeyCmdLiteral = "api-key"
	APIKeyCmdExample = `# List API keys for a WebSocket API
ap gateway websocket-api api-key list --id chat-websocket-api

# Generate a new API key with an auto-generated name
ap gateway websocket-api api-key create --id chat-websocket-api`
)

// APIKeyCmd represents the gateway WebSocket API api-key command group. API
// keys are scoped to a WebSocket API via the /websocket-apis/{id}/api-keys
// management endpoints.
var APIKeyCmd = &cobra.Command{
	Use:     APIKeyCmdLiteral,
	Short:   "Manage API keys for a WebSocket API on the gateway",
	Long:    "This command allows you to create, list, regenerate, update, and revoke API keys for a WebSocket API on the WSO2 API Platform Gateway.",
	Example: APIKeyCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	APIKeyCmd.AddCommand(createCmd)
	APIKeyCmd.AddCommand(listCmd)
	APIKeyCmd.AddCommand(regenerateCmd)
	APIKeyCmd.AddCommand(updateCmd)
	APIKeyCmd.AddCommand(revokeCmd)
}
