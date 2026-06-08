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

package platform

import "github.com/spf13/cobra"

const (
	PlatformCmdLiteral = "platform"
	PlatformCmdExample = `# Add a platform
ap platform add --display-name dev

# Switch the current platform
ap platform use --display-name dev

# Show the current platform
ap platform current

# List all platforms
ap platform list`
)

var PlatformCmd = &cobra.Command{
	Use:     PlatformCmdLiteral,
	Short:   "Execute platform operations",
	Long:    "This command allows you to manage CLI platforms.",
	Example: PlatformCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	PlatformCmd.AddCommand(addCmd)
	PlatformCmd.AddCommand(useCmd)
	PlatformCmd.AddCommand(currentCmd)
	PlatformCmd.AddCommand(listCmd)
}
