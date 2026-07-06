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
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/aiworkspace"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	GetCmdLiteral = "get"
	GetCmdExample = `# List all LLM providers
ap ai-workspace llm-provider get

# Get a single LLM provider by ID
ap ai-workspace llm-provider get --id wso2-claude

# List using a specific AI workspace with pagination
ap ai-workspace llm-provider get --limit 50 --offset 0 --display-name my-workspace --platform eu`
)

var (
	getID       string
	getLimit    string
	getOffset   string
	getName     string
	getPlatform string
	getInsecure bool
)

var getCmd = &cobra.Command{
	Use:     GetCmdLiteral,
	Short:   "Get one or all LLM providers from the AI workspace",
	Long:    "Retrieve LLM providers from the WSO2 API Platform AI workspace. With --id a single provider is fetched; without it all providers in the organization are listed.",
	Example: GetCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGetCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(getCmd, utils.FlagID, &getID, "", "LLM provider ID (omit to list all)")
	utils.AddStringFlag(getCmd, utils.FlagLimit, &getLimit, "", "Maximum number of providers to return when listing")
	utils.AddStringFlag(getCmd, utils.FlagOffset, &getOffset, "", "Number of providers to skip when listing")
	utils.AddStringFlag(getCmd, utils.FlagName, &getName, "", "AI workspace display name")
	utils.AddStringFlag(getCmd, utils.FlagPlatform, &getPlatform, "", "Platform name")
	getCmd.Flags().BoolVar(&getInsecure, "insecure", false, "Skip TLS certificate verification")
}

func runGetCommand() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	aiWorkspace, _, err := aiworkspace.ResolveAIWorkspace(cfg, getName, getPlatform)
	if err != nil {
		return err
	}

	client := aiworkspace.NewClientWithOptions(aiWorkspace, getInsecure)

	var path, action string
	if id := strings.TrimSpace(getID); id != "" {
		path = aiworkspace.ProviderByIDPath(id)
		action = "get llm provider"
	} else {
		path = aiworkspace.ProviderListPath(aiworkspace.ListQuery{Limit: getLimit, Offset: getOffset})
		action = "list llm providers"
	}

	resp, err := client.Get(path)
	if err != nil {
		return aiworkspace.WrapRequestError(action, err, getInsecure)
	}

	return aiworkspace.PrintJSONResponse(resp)
}
