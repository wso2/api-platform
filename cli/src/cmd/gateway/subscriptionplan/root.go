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

package subscriptionplan

import (
	"github.com/spf13/cobra"
)

const (
	SubscriptionPlanCmdLiteral = "subscription-plan"
	SubscriptionPlanCmdExample = `# List all subscription plans
ap gateway subscription-plan list

# Create a subscription plan from a CR file
ap gateway subscription-plan create --file subscription-plan.yaml`
)

// SubscriptionPlanCmd represents the gateway subscription-plan command group.
// It surfaces the Subscription Plan operations of the gateway management API.
var SubscriptionPlanCmd = &cobra.Command{
	Use:     SubscriptionPlanCmdLiteral,
	Short:   "Manage subscription plans on the gateway",
	Long:    "This command allows you to manage subscription plans on the WSO2 API Platform Gateway.",
	Example: SubscriptionPlanCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	SubscriptionPlanCmd.AddCommand(createCmd)
	SubscriptionPlanCmd.AddCommand(listCmd)
	SubscriptionPlanCmd.AddCommand(getCmd)
	SubscriptionPlanCmd.AddCommand(updateCmd)
	SubscriptionPlanCmd.AddCommand(deleteCmd)
}
