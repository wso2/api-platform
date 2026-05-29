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

import "github.com/spf13/cobra"

const (
	APIKeyCmdLiteral = "api-key"
	APIKeyCmdExample = `# Generate an API key
ap devportal api-key gen --org org_1 --api-id api_1 --name weather_prod_key

# Generate an API key interactively
ap devportal api-key gen

# List API keys for an API
ap devportal api-key get --org org_1 --api-id api_1

# Regenerate an API key
ap devportal api-key regenerate --org org_1 --api-key-id key_1

# Revoke an API key
ap devportal api-key revoke --org org_1 --api-key-id key_1`
)

// APIKeyCmd represents the DevPortal API key command group.
// It surfaces the "API Keys" tag of the DevPortal REST API
// (docs/devportal-openapi-spec-v1.yaml).
var APIKeyCmd = &cobra.Command{
	Use:     APIKeyCmdLiteral,
	Short:   "Manage DevPortal API keys",
	Long:    "This command allows you to generate, list, regenerate, and revoke DevPortal API keys.",
	Example: APIKeyCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	APIKeyCmd.AddCommand(generateCmd)
	APIKeyCmd.AddCommand(getCmd)
	APIKeyCmd.AddCommand(regenerateCmd)
	APIKeyCmd.AddCommand(revokeCmd)
}
