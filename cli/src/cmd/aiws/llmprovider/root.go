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

package llmprovider

import (
	"github.com/spf13/cobra"
)

const (
	LLMProviderCmdLiteral = "llm-provider"
	LLMProviderCmdExample = `# Push an LLM provider artifact to the AI workspace
ap ai-ws llm-provider push -f build/wso2-claude.json --org <org-id>`
)

// LLMProviderCmd is the parent command for LLM provider operations.
var LLMProviderCmd = &cobra.Command{
	Use:     LLMProviderCmdLiteral,
	Short:   "Manage LLM providers on the AI workspace",
	Long:    "This command allows you to manage LLM providers on the WSO2 API Platform AI workspace.",
	Example: LLMProviderCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	LLMProviderCmd.AddCommand(pushCmd)
}
