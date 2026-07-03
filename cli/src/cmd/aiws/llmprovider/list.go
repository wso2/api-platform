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
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/aiworkspace"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	ListCmdLiteral = "list"
	ListCmdExample = `# List all LLM providers
ap ai-ws llm-provider list

# List with pagination
ap ai-ws llm-provider list --limit 50 --offset 0

# List using a specific AI workspace
ap ai-ws llm-provider list --display-name my-workspace --platform eu`
)

var (
	listLimit    string
	listOffset   string
	listName     string
	listPlatform string
	listInsecure bool
)

var listCmd = &cobra.Command{
	Use:     ListCmdLiteral,
	Short:   "List all LLM providers in the AI workspace",
	Long:    "Retrieves all LLM providers for a given organization from the WSO2 API Platform AI workspace.",
	Example: ListCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runListCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(listCmd, utils.FlagLimit, &listLimit, "", "Maximum number of providers to return")
	utils.AddStringFlag(listCmd, utils.FlagOffset, &listOffset, "", "Number of providers to skip")
	utils.AddStringFlag(listCmd, utils.FlagName, &listName, "", "AI workspace display name")
	utils.AddStringFlag(listCmd, utils.FlagPlatform, &listPlatform, "", "Platform name")
	listCmd.Flags().BoolVar(&listInsecure, "insecure", false, "Skip TLS certificate verification")
}

func runListCommand() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	aiWorkspace, _, err := aiworkspace.ResolveAIWorkspace(cfg, listName, listPlatform)
	if err != nil {
		return err
	}

	client := aiworkspace.NewClientWithOptions(aiWorkspace, listInsecure)
	path := aiworkspace.ProviderListPath(aiworkspace.ListQuery{Limit: listLimit, Offset: listOffset})

	resp, err := client.Get(path)
	if err != nil {
		return aiworkspace.WrapRequestError("list llm providers", err, listInsecure)
	}

	return aiworkspace.PrintJSONResponse(resp)
}
