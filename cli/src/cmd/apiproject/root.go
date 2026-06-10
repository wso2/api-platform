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
package apiproject

import "github.com/spf13/cobra"

const (
	ApiProjectCmdLiteral = "apiproject"
	ApiProjectCmdExample = `# Add a new API project
ap apiproject init --display-name foo-api --type rest --version 1.0 --context /foo`
)

var ApiProjectCmd = &cobra.Command{
	Use:     ApiProjectCmdLiteral,
	Short:   "Execute API project operations",
	Long:    "This command allows you to manage API projects.",
	Example: ApiProjectCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	ApiProjectCmd.AddCommand(initCmd)
}