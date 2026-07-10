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

package restapi

import (
	"github.com/spf13/cobra"
)

const (
	APICmdLiteral = "rest-api"
	APICmdExample = `# List all REST APIs
ap devportal rest-api list
ap devportal rest-api publish -f petstore-api.zip`
)

// APICmd represents the api command
var APICmd = &cobra.Command{
	Use:     APICmdLiteral,
	Short:   "Manage REST APIs on the devportal",
	Long:    "This command allows you to manage REST APIs on the WSO2 API Platform DevPortal.",
	Example: APICmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	// Register subcommands
	APICmd.AddCommand(listCmd)
	APICmd.AddCommand(getCmd)
	APICmd.AddCommand(editCmd)
	APICmd.AddCommand(deleteCmd)
}
