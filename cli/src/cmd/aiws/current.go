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
	CurrentCmdLiteral = "current"
	CurrentCmdExample = `# Show the current active AI workspace
ap ai-ws current`
)

var currentCmd = &cobra.Command{
	Use:     CurrentCmdLiteral,
	Short:   "Show the current active AI workspace",
	Long:    "Display the current active AI workspace configuration.",
	Example: CurrentCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runCurrentCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var currentPlatform string

func init() {
	utils.AddStringFlag(currentCmd, utils.FlagPlatform, &currentPlatform, "", "Platform name")
}

func runCurrentCommand() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	resolvedPlatform := cfg.ResolvePlatform(currentPlatform)
	aiWorkspace, err := cfg.GetActiveAIWorkspaceFromPlatform(resolvedPlatform)
	if err != nil {
		return err
	}

	fmt.Printf("Current ai-workspace: %s - %s (platform: %s, auth: %s)\n", aiWorkspace.Name, aiWorkspace.URL, resolvedPlatform, aiWorkspace.Auth.Type)

	return nil
}
