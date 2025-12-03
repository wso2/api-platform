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

package cmd

import (
	"github.com/spf13/cobra"
)

const GenCmdLiteral = "generate"
const GenCmdExample = `# Generate MCP configuration` + "\n" +
	CliName + ` ` + GatewayCmdLiteral + ` ` + GenCmdLiteral + ` mcp`

var genCmd = &cobra.Command{
	Use:     GenCmdLiteral,
	Aliases: []string{"gen"},
	Short:   "Generate configuration",
	Long:    "Generate a configuration which can be deployed in the WSO2 API Platform Gateway",
	Example: GenCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	gatewayCmd.AddCommand(genCmd)
}
