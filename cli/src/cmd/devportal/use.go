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

package devportal

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	UseCmdLiteral = "use"
	UseCmdExample = `# Set my-portal as the active devportal
ap devportal use --display-name my-portal`
)

var (
	useName     string
	usePlatform string
)

var useCmd = &cobra.Command{
	Use:     UseCmdLiteral,
	Short:   "Set the active devportal",
	Long:    "Set the active devportal that will be used by default for operations.",
	Example: UseCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUseCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(useCmd, utils.FlagName, &useName, "", "Name of the devportal to use (required)")
	utils.AddStringFlag(useCmd, utils.FlagPlatform, &usePlatform, "", "Platform name")
	useCmd.MarkFlagRequired(utils.FlagName)
}

func runUseCommand() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	resolvedPlatform := cfg.ResolvePlatform(usePlatform)
	devPortal, err := cfg.GetDevPortalFromPlatform(resolvedPlatform, useName)
	if err != nil {
		return err
	}

	if err := cfg.SetActiveDevPortalForPlatform(resolvedPlatform, useName); err != nil {
		return err
	}

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("DevPortal set to %s (platform: %s, auth: %s).\n", useName, resolvedPlatform, devPortal.Auth.Type)

	hasEnvCreds := false
	hasConfigCreds := false
	message := ""

	switch devPortal.Auth.Type {
	case utils.AuthTypeBasic:
		hasEnvCreds = os.Getenv(utils.EnvDevPortalUsername) != "" && os.Getenv(utils.EnvDevPortalPassword) != ""
		hasConfigCreds = devPortal.Auth.Username != "" && devPortal.Auth.Password != ""
		message = fmt.Sprintf("\nBasic authentication requires the following environment variables:\n  %s\n  %s\n", utils.EnvDevPortalUsername, utils.EnvDevPortalPassword)
	case utils.AuthTypeOAuth:
		hasEnvCreds = os.Getenv(utils.EnvDevPortalToken) != ""
		hasConfigCreds = devPortal.Auth.Token != ""
		message = fmt.Sprintf("\nOAuth authentication requires the following environment variable:\n  %s\n", utils.EnvDevPortalToken)
	case utils.AuthTypeAPIKey:
		hasEnvCreds = os.Getenv(utils.EnvDevPortalAPIKey) != ""
		hasConfigCreds = devPortal.Auth.APIKey != ""
		message = fmt.Sprintf("\nAPI key authentication requires the following environment variable:\n  %s\n", utils.EnvDevPortalAPIKey)
	}

	if !hasEnvCreds && !hasConfigCreds {
		fmt.Print(message)
	} else if hasConfigCreds && !hasEnvCreds {
		fmt.Println("Using credentials from configuration.")
	} else if hasEnvCreds {
		fmt.Println("Using credentials from environment variables.")
	}

	return nil
}
