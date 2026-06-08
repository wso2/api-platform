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
	UseCmdLiteral = "use"
	UseCmdExample = `# Set the current platform
ap platform use --display-name dev`
)

var platformName string

var useCmd = &cobra.Command{
	Use:     UseCmdLiteral,
	Short:   "Set the current platform",
	Long:    "Set the current platform used by default for gateway and devportal commands.",
	Example: UseCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUseCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(useCmd, utils.FlagName, &platformName, "", "Name of the platform to use (required)")
	_ = useCmd.MarkFlagRequired(utils.FlagName)
}

func runUseCommand() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	platformName := cfg.SetCurrentPlatform(strings.TrimSpace(platformName))

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Current platform set to %s.\n", platformName)
	return nil
}
