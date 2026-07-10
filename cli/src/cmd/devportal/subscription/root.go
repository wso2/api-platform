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

package subscription

import "github.com/spf13/cobra"

const (
	SubscriptionCmdLiteral = "subscription"
	SubscriptionCmdExample = `# Create a platform subscription
ap devportal subscription create --api-id api_1 --subscription-plan gold --application-id app_1

# Get a platform subscription
ap devportal subscription get --sub-id sub_1`
)

// SubscriptionCmd represents the platform subscription command group.
var SubscriptionCmd = &cobra.Command{
	Use:     SubscriptionCmdLiteral,
	Short:   "Manage DevPortal platform subscriptions",
	Long:    "This command allows you to manage DevPortal platform subscriptions.",
	Example: SubscriptionCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	SubscriptionCmd.AddCommand(createCmd)
	SubscriptionCmd.AddCommand(editCmd)
	SubscriptionCmd.AddCommand(getCmd)
	SubscriptionCmd.AddCommand(deleteCmd)
}
