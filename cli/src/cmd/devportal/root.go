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

package devportal

import (
	"github.com/spf13/cobra"
	devportalapikey "github.com/wso2/api-platform/cli/cmd/devportal/apikey"
	devportalapplication "github.com/wso2/api-platform/cli/cmd/devportal/application"
	"github.com/wso2/api-platform/cli/cmd/devportal/restapi"
	devportalsubplan "github.com/wso2/api-platform/cli/cmd/devportal/subplan"
	devportalsubscription "github.com/wso2/api-platform/cli/cmd/devportal/subscription"
)

const (
	DevPortalCmdLiteral = "devportal"
	DevPortalCmdExample = `# Add a new DevPortal
ap devportal add --display-name my-portal --platform eu --server https://devportal.example.com --auth api-key`
)

// DevPortalCmd represents the DevPortal command.
var DevPortalCmd = &cobra.Command{
	Use:     DevPortalCmdLiteral,
	Short:   "Execute DevPortal operations",
	Long:    "This command allows you to execute various operations related to DevPortals.",
	Example: DevPortalCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	DevPortalCmd.AddCommand(addCmd)
	DevPortalCmd.AddCommand(listCmd)
	DevPortalCmd.AddCommand(removeCmd)
	DevPortalCmd.AddCommand(useCmd)
	DevPortalCmd.AddCommand(currentCmd)
	DevPortalCmd.AddCommand(healthCmd)
	DevPortalCmd.AddCommand(buildCmd)
	DevPortalCmd.AddCommand(genCmd)
	DevPortalCmd.AddCommand(applyCmd)
	DevPortalCmd.AddCommand(devportalapikey.APIKeyCmd)
	DevPortalCmd.AddCommand(devportalapplication.ApplicationCmd)
	DevPortalCmd.AddCommand(devportalsubscription.SubscriptionCmd)
	DevPortalCmd.AddCommand(devportalsubplan.SubPlanCmd)
	DevPortalCmd.AddCommand(restapi.APICmd)
}
