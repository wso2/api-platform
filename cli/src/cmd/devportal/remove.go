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

package devportal

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	RemoveCmdLiteral = "remove"
	RemoveCmdExample = `# Remove a devportal
ap devportal remove --display-name my-portal`
)

var (
	removeName     string
	removePlatform string
)

var removeCmd = &cobra.Command{
	Use:     RemoveCmdLiteral,
	Short:   "Remove a devportal",
	Long:    "Remove a devportal configuration from the ap config file.",
	Example: RemoveCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runRemoveCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(removeCmd, utils.FlagName, &removeName, "", "Name of the devportal to remove (required)")
	utils.AddStringFlag(removeCmd, utils.FlagPlatform, &removePlatform, "", "Platform name")
	removeCmd.MarkFlagRequired(utils.FlagName)
}

func runRemoveCommand() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	resolvedPlatform := cfg.ResolvePlatform(removePlatform)
	if err := cfg.RemoveDevPortalFromPlatform(resolvedPlatform, removeName); err != nil {
		return err
	}

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("DevPortal removed successfully.")

	return nil
}
