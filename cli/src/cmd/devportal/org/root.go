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

package org

import "github.com/spf13/cobra"

const (
	OrgCmdLiteral = "org"
	OrgCmdExample = `# List all organizations
ap devportal org list

# Get a specific organization
ap devportal org get --org org_1`
)

// OrgCmd represents the organization command group.
var OrgCmd = &cobra.Command{
	Use:     OrgCmdLiteral,
	Short:   "Manage DevPortal organizations",
	Long:    "This command allows you to manage DevPortal organizations.",
	Example: OrgCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	OrgCmd.AddCommand(addCmd)
	OrgCmd.AddCommand(deleteCmd)
	OrgCmd.AddCommand(editCmd)
	OrgCmd.AddCommand(listCmd)
	OrgCmd.AddCommand(getCmd)
}
