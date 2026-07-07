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
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	AddCmdLiteral = "add"
	AddCmdExample = `# Add a new AI workspace fully interactively
ap ai-workspace add

# Add an AI workspace with basic auth
ap ai-workspace add --display-name my-workspace --server https://ai-workspace.example.com --auth basic

# Add an AI workspace with OAuth auth
ap ai-workspace add --display-name my-workspace --server https://ai-workspace.example.com --auth oauth

# Add an AI workspace with API key auth
ap ai-workspace add --display-name my-workspace --server https://ai-workspace.example.com --auth api-key

# Add an AI workspace without interactive prompts using flags
ap ai-workspace add --display-name my-workspace --server https://ai-workspace.example.com --auth basic --no-interactive --username admin --password admin
ap ai-workspace add --display-name my-workspace --server https://ai-workspace.example.com --auth oauth --no-interactive --token your_token_here
ap ai-workspace add --display-name my-workspace --server https://ai-workspace.example.com --auth api-key --no-interactive --api-key your_api_key_here`
)

var (
	addName          string
	addPlatform      string
	addServer        string
	addAuth          string
	addUsername      string
	addPassword      string
	addToken         string
	addAPIKey        string
	addNoInteractive bool
)

var addCmd = &cobra.Command{
	Use:     AddCmdLiteral,
	Short:   "Add a new AI workspace",
	Long:    "Add a new AI workspace configuration to the ap config file.",
	Example: AddCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runAddCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(addCmd, utils.FlagName, &addName, "", "Display name of the AI workspace")
	utils.AddStringFlag(addCmd, utils.FlagPlatform, &addPlatform, "", "Platform name for the AI workspace")
	utils.AddStringFlag(addCmd, utils.FlagServer, &addServer, "", "Server URL of the AI workspace")
	utils.AddStringFlag(addCmd, utils.FlagAuth, &addAuth, "", "Authentication type for the AI workspace. Supported values: basic, oauth, api-key")
	utils.AddStringFlag(addCmd, utils.FlagUsername, &addUsername, "", "Username for AI workspace basic auth (not recommended, use interactive mode)")
	utils.AddStringFlag(addCmd, utils.FlagPassword, &addPassword, "", "Password for AI workspace basic auth (not recommended, use interactive mode)")
	utils.AddStringFlag(addCmd, utils.FlagToken, &addToken, "", "Token for AI workspace OAuth auth (not recommended, use interactive mode)")
	utils.AddStringFlag(addCmd, utils.FlagAPIKey, &addAPIKey, "", "API key for AI workspace API key auth (not recommended, use interactive mode)")
	utils.AddBoolFlag(addCmd, utils.FlagNoInteractive, &addNoInteractive, false, "Skip interactive prompts")
}

func runAddCommand() error {
	var err error

	if !addNoInteractive {
		if strings.TrimSpace(addName) == "" {
			addName, err = utils.PromptInput("Enter AI workspace display name: ")
			if err != nil {
				return fmt.Errorf("failed to read display name: %w", err)
			}
		}
		if strings.TrimSpace(addPlatform) == "" {
			addPlatform, err = utils.PromptInput(fmt.Sprintf("Enter platform (default: %s): ", config.DefaultPlatform))
			if err != nil {
				return fmt.Errorf("failed to read platform: %w", err)
			}
		}
		if strings.TrimSpace(addServer) == "" {
			addServer, err = utils.PromptInput("Enter AI workspace server URL: ")
			if err != nil {
				return fmt.Errorf("failed to read server URL: %w", err)
			}
		}
		if strings.TrimSpace(addAuth) == "" {
			addAuth, err = utils.PromptInput("Enter auth type (basic, oauth, api-key): ")
			if err != nil {
				return fmt.Errorf("failed to read auth type: %w", err)
			}
		}
	}

	addName = strings.TrimSpace(addName)
	addPlatform = strings.TrimSpace(addPlatform)
	addServer = strings.TrimSpace(addServer)
	addAuth = strings.ToLower(strings.TrimSpace(addAuth))
	// Normalize credential flags too: trailing whitespace from flags would
	// otherwise flow into the auth checks and be stored/sent verbatim. (The
	// interactive prompts already trim via PromptInput/PromptPassword.)
	addUsername = strings.TrimSpace(addUsername)
	addPassword = strings.TrimSpace(addPassword)
	addToken = strings.TrimSpace(addToken)
	addAPIKey = strings.TrimSpace(addAPIKey)

	if addName == "" {
		return fmt.Errorf("missing required flag --%s (or provide it in interactive mode)", utils.FlagName)
	}
	if addServer == "" {
		return fmt.Errorf("missing required flag --%s (or provide it in interactive mode)", utils.FlagServer)
	}
	if addAuth == "" {
		return fmt.Errorf("missing required flag --%s (or provide it in interactive mode)", utils.FlagAuth)
	}
	if addAuth != utils.AuthTypeBasic && addAuth != utils.AuthTypeOAuth && addAuth != utils.AuthTypeAPIKey {
		return fmt.Errorf("invalid auth type '%s'. AI workspace supports only: %s, %s, %s", addAuth, utils.AuthTypeBasic, utils.AuthTypeOAuth, utils.AuthTypeAPIKey)
	}

	switch addAuth {
	case utils.AuthTypeBasic:
		if addToken != "" || addAPIKey != "" {
			return fmt.Errorf("--token and --api-key cannot be used with auth type '%s'. Use --username and --password instead", addAuth)
		}
	case utils.AuthTypeOAuth:
		if addUsername != "" || addPassword != "" || addAPIKey != "" {
			return fmt.Errorf("--username, --password, and --api-key cannot be used with auth type '%s'. Use --token instead", addAuth)
		}
	case utils.AuthTypeAPIKey:
		if addUsername != "" || addPassword != "" || addToken != "" {
			return fmt.Errorf("--username, --password, and --token cannot be used with auth type '%s'. Use --api-key instead", addAuth)
		}
	}

	if addUsername != "" || addPassword != "" || addToken != "" || addAPIKey != "" {
		fmt.Fprintln(os.Stderr, "Warning: Passing credentials via command-line flags is not recommended for security reasons.")
		fmt.Fprintln(os.Stderr, "Consider using interactive mode or environment variables instead.")
		fmt.Fprintln(os.Stderr, "")
	}

	username := addUsername
	password := addPassword
	token := addToken
	apiKey := addAPIKey

	if !addNoInteractive && username == "" && password == "" && token == "" && apiKey == "" {
		username, password, token, apiKey, err = utils.PromptAIWorkspaceCredentials(addAuth)
		if err != nil {
			return fmt.Errorf("failed to read credentials: %w", err)
		}
	}

	if addAuth == utils.AuthTypeBasic {
		if (username != "" && password == "") || (username == "" && password != "") {
			return fmt.Errorf("for basic auth, both username and password must be provided, or leave both empty to use environment variables (%s and %s)",
				utils.EnvAIWorkspaceUsername, utils.EnvAIWorkspacePassword)
		}
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	resolvedPlatform := cfg.ResolvePlatform(addPlatform)

	aiWorkspace := config.AIWorkspace{
		Name: addName,
		URL:  addServer,
		Auth: config.AuthConfig{
			Type:     addAuth,
			Username: username,
			Password: password,
			Token:    token,
			APIKey:   apiKey,
		},
	}

	if username != "" || password != "" || token != "" || apiKey != "" {
		fmt.Fprintln(os.Stderr, "Note: Credentials will be stored in plaintext in the configuration file (mode 0600).")
		fmt.Fprintln(os.Stderr, "To avoid storing secrets on disk, omit credentials and use environment variables instead.")
		fmt.Fprintln(os.Stderr, "")
	}

	if err := cfg.AddAIWorkspaceToPlatform(resolvedPlatform, aiWorkspace); err != nil {
		return fmt.Errorf("failed to add AI workspace: %w", err)
	}

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	configPath, err := config.GetConfigPath()
	if err != nil {
		utils.LogWarning("could not determine config path", err)
		configPath = "(unknown location)"
	}

	fmt.Printf("AI workspace %s added (platform: %s, server: %s, auth: %s)\n", addName, resolvedPlatform, addServer, addAuth)
	fmt.Printf("Configuration saved to: %s\n", configPath)

	if username != "" || password != "" || token != "" || apiKey != "" {
		switch addAuth {
		case utils.AuthTypeBasic:
			fmt.Printf("Note: Credentials were stored in the configuration; exporting %s and %s will override them at runtime.\n", utils.EnvAIWorkspaceUsername, utils.EnvAIWorkspacePassword)
		case utils.AuthTypeOAuth:
			fmt.Printf("Note: Credentials were stored in the configuration; exporting %s will override them at runtime.\n", utils.EnvAIWorkspaceToken)
		case utils.AuthTypeAPIKey:
			fmt.Printf("Note: Credentials were stored in the configuration; exporting %s will override them at runtime.\n", utils.EnvAIWorkspaceAPIKey)
		}
	} else {
		fmt.Println()
		lines := []string{
			"No credentials stored for this AI workspace.",
			"Set the following environment variable(s) before making AI workspace API calls:",
		}
		switch addAuth {
		case utils.AuthTypeBasic:
			lines = append(lines, "  "+utils.EnvAIWorkspaceUsername, "  "+utils.EnvAIWorkspacePassword)
		case utils.AuthTypeOAuth:
			lines = append(lines, "  "+utils.EnvAIWorkspaceToken)
		case utils.AuthTypeAPIKey:
			lines = append(lines, "  "+utils.EnvAIWorkspaceAPIKey)
		}
		utils.PrintBoxedMessage(lines)
	}

	return nil
}
