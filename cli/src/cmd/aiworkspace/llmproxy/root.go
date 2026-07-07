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

package llmproxy

import (
	"github.com/spf13/cobra"
)

const (
	LLMProxyCmdLiteral = "app-llm-proxy"
	LLMProxyCmdExample = `# List LLM proxies in a project on the AI workspace
ap ai-workspace app-llm-proxy list --project-id <project-id>

# Create or update a proxy from a project with:
#   ap ai-workspace apply --project-id <project-id>`
)

// LLMProxyCmd is the parent command for LLM proxy operations.
var LLMProxyCmd = &cobra.Command{
	Use:     LLMProxyCmdLiteral,
	Short:   "Manage LLM proxies on the AI workspace",
	Long:    "This command allows you to manage LLM proxies on the WSO2 API Platform AI workspace.",
	Example: LLMProxyCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	LLMProxyCmd.AddCommand(listCmd)
	LLMProxyCmd.AddCommand(getCmd)
	LLMProxyCmd.AddCommand(deleteCmd)
}
