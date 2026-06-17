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
package project

import "github.com/spf13/cobra"

const (
	ProjectCmdLiteral = "project"
	ProjectCmdExample = `# Initialize a new project
ap project init --display-name foo-api --type rest`
)

var ProjectCmd = &cobra.Command{
	Use:     ProjectCmdLiteral,
	Short:   "Execute project operations",
	Long:    "This command allows you to manage projects (REST/SOAP/GraphQL APIs, LLM proxies/providers, MCP proxies).",
	Example: ProjectCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	ProjectCmd.AddCommand(initCmd)
}