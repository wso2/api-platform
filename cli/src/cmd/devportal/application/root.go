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

package application

import "github.com/spf13/cobra"

const (
	ApplicationCmdLiteral = "application"
	ApplicationCmdExample = `# Create an application
ap devportal application create --name "Weather App" --type WEB

# List applications
ap devportal application get

# Get a single application
ap devportal application get --app-id app_1

# Update an application
ap devportal application update --app-id app_1 --name "Weather App" --type WEB

# Delete an application
ap devportal application delete --app-id app_1`
)

// ApplicationCmd represents the DevPortal application command group.
// It maps to the "Applications" tag of the DevPortal REST API
// (docs/devportal-openapi-spec-v1.yaml).
var ApplicationCmd = &cobra.Command{
	Use:     ApplicationCmdLiteral,
	Short:   "Manage DevPortal applications",
	Long:    "This command allows you to create, update, get, and delete applications on the WSO2 API Platform DevPortal.",
	Example: ApplicationCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	ApplicationCmd.AddCommand(createCmd)
	ApplicationCmd.AddCommand(updateCmd)
	ApplicationCmd.AddCommand(getCmd)
	ApplicationCmd.AddCommand(deleteCmd)
}
