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

package subapikey

import "github.com/spf13/cobra"

const (
	SubKeyCmdLiteral = "sub-api-key"
	SubKeyCmdExample = `# Generate a platform API key
ap devportal sub-api-key generate --org org_1 --api-id api_1 --key-name mobile-app-key

# List platform API keys
ap devportal sub-api-key get --org org_1`
)

// SubKeyCmd represents the platform API key command group.
var SubKeyCmd = &cobra.Command{
	Use:     SubKeyCmdLiteral,
	Short:   "Manage DevPortal platform API keys",
	Long:    "This command allows you to manage DevPortal platform API keys.",
	Example: SubKeyCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	SubKeyCmd.AddCommand(generateCmd)
	SubKeyCmd.AddCommand(getCmd)
	SubKeyCmd.AddCommand(regenerateCmd)
	SubKeyCmd.AddCommand(revokeCmd)
}
