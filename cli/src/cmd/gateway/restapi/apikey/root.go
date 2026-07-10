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
	APIKeyCmdExample = `# List API keys for a REST API
ap gateway rest-api api-key list --id reading-list-api-v1.0

# Generate a new API key from a CR file
ap gateway rest-api api-key create --file api-key.yaml`
)

// APIKeyCmd represents the gateway REST API api-key command group. API keys are
// scoped to a REST API via the /rest-apis/{id}/api-keys management endpoints.
var APIKeyCmd = &cobra.Command{
	Use:     APIKeyCmdLiteral,
	Short:   "Manage API keys for a REST API on the gateway",
	Long:    "This command allows you to create, list, regenerate, update, and revoke API keys for a REST API on the WSO2 API Platform Gateway.",
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
