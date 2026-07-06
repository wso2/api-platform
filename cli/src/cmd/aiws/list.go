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

package aiws

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	ListCmdLiteral = "list"
	ListCmdExample = `# List all AI workspaces
ap ai-workspace list`
)

var listCmd = &cobra.Command{
	Use:     ListCmdLiteral,
	Short:   "List all AI workspaces",
	Long:    "List all AI workspace configurations from the ap config file.",
	Example: ListCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runListCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var listPlatform string

func init() {
	utils.AddStringFlag(listCmd, utils.FlagPlatform, &listPlatform, "", "Platform name")
}

func runListCommand() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	resolvedPlatform := cfg.ResolvePlatform(listPlatform)
	platform := cfg.Platforms[resolvedPlatform]
	if platform == nil || len(platform.AIWorkspaces) == 0 {
		fmt.Printf("No ai-workspace configured for platform %s\n", resolvedPlatform)
		return nil
	}

	headers := []string{"PLATFORM", "NAME", "URL", "AUTH", "CURRENT"}
	rows := make([][]string, 0, len(platform.AIWorkspaces))
	for name, ws := range platform.AIWorkspaces {
		current := ""
		if name == platform.ActiveAIWorkspace {
			current = "*"
		}
		rows = append(rows, []string{resolvedPlatform, name, ws.URL, ws.Auth.Type, current})
	}
	utils.PrintTable(headers, rows)

	return nil
}
