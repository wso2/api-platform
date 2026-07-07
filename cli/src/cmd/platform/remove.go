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

package platform

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	RemoveCmdLiteral = "remove"
	RemoveCmdExample = `# Remove a platform
ap platform remove --display-name dev`
)

var removeName string

var removeCmd = &cobra.Command{
	Use:     RemoveCmdLiteral,
	Short:   "Remove a platform",
	Long:    "Remove a platform and all of its connections (gateways, devportals, AI workspaces) from the CLI configuration.",
	Example: RemoveCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runRemoveCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(removeCmd, utils.FlagName, &removeName, "", "Name of the platform to remove (required)")
	_ = removeCmd.MarkFlagRequired(utils.FlagName)
}

func runRemoveCommand() error {
	name := strings.TrimSpace(removeName)
	if name == "" {
		return fmt.Errorf("platform name is required")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.RemovePlatform(name); err != nil {
		return err
	}

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Platform %s removed.\n", name)
	return nil
}
