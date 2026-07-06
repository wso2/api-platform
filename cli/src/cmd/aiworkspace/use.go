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
	UseCmdLiteral = "use"
	UseCmdExample = `# Set my-workspace as the active AI workspace
ap ai-workspace use --display-name my-workspace`
)

var (
	useName     string
	usePlatform string
)

var useCmd = &cobra.Command{
	Use:     UseCmdLiteral,
	Short:   "Set the active AI workspace",
	Long:    "Set the active AI workspace that will be used by default for operations.",
	Example: UseCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUseCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(useCmd, utils.FlagName, &useName, "", "Name of the AI workspace to use (required)")
	utils.AddStringFlag(useCmd, utils.FlagPlatform, &usePlatform, "", "Platform name")
	useCmd.MarkFlagRequired(utils.FlagName)
}

func runUseCommand() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	resolvedPlatform := cfg.ResolvePlatform(usePlatform)
	aiWorkspace, err := cfg.GetAIWorkspaceFromPlatform(resolvedPlatform, useName)
	if err != nil {
		return err
	}

	if err := cfg.SetActiveAIWorkspaceForPlatform(resolvedPlatform, useName); err != nil {
		return err
	}

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("AI workspace set to %s (platform: %s, auth: %s).\n", useName, resolvedPlatform, aiWorkspace.Auth.Type)

	hasEnvCreds := false
	hasConfigCreds := false
	message := ""

	switch aiWorkspace.Auth.Type {
	case utils.AuthTypeBasic:
		hasEnvCreds = os.Getenv(utils.EnvAIWorkspaceUsername) != "" && os.Getenv(utils.EnvAIWorkspacePassword) != ""
		hasConfigCreds = aiWorkspace.Auth.Username != "" && aiWorkspace.Auth.Password != ""
		message = fmt.Sprintf("\nBasic authentication requires the following environment variables:\n  %s\n  %s\n", utils.EnvAIWorkspaceUsername, utils.EnvAIWorkspacePassword)
	case utils.AuthTypeOAuth:
		hasEnvCreds = os.Getenv(utils.EnvAIWorkspaceToken) != ""
		hasConfigCreds = aiWorkspace.Auth.Token != ""
		message = fmt.Sprintf("\nOAuth authentication requires the following environment variable:\n  %s\n", utils.EnvAIWorkspaceToken)
	case utils.AuthTypeAPIKey:
		hasEnvCreds = os.Getenv(utils.EnvAIWorkspaceAPIKey) != ""
		hasConfigCreds = aiWorkspace.Auth.APIKey != ""
		message = fmt.Sprintf("\nAPI key authentication requires the following environment variable:\n  %s\n", utils.EnvAIWorkspaceAPIKey)
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
