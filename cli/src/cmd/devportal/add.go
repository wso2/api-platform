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
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	AddCmdLiteral = "add"
	AddCmdExample = `# Add a new DevPortal fully interactively
ap devportal add

# Add a DevPortal with basic auth
ap devportal add --display-name my-portal --server https://devportal.example.com --auth basic

# Add a DevPortal with OAuth auth
ap devportal add --display-name my-portal --server https://devportal.example.com --auth oauth

# Add a DevPortal with API key auth
ap devportal add --display-name my-portal --server https://devportal.example.com --auth api-key

# Add a DevPortal without interactive prompts using flags
ap devportal add --display-name my-portal --server https://devportal.example.com --auth basic --no-interactive --username admin --password admin
ap devportal add --display-name my-portal --server https://devportal.example.com --auth oauth --no-interactive --token your_token_here
ap devportal add --display-name my-portal --server https://devportal.example.com --auth api-key --no-interactive --api-key your_api_key_here`
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
	Short:   "Add a new DevPortal",
	Long:    "Add a new DevPortal configuration to the ap config file.",
	Example: AddCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runAddCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(addCmd, utils.FlagName, &addName, "", "Display name of the DevPortal")
	utils.AddStringFlag(addCmd, utils.FlagPlatform, &addPlatform, "", "Platform name for the DevPortal")
	utils.AddStringFlag(addCmd, utils.FlagServer, &addServer, "", "Server URL of the DevPortal")
	utils.AddStringFlag(addCmd, utils.FlagAuth, &addAuth, "", "Authentication type for the DevPortal. Supported values: basic, oauth, api-key")
	utils.AddStringFlag(addCmd, utils.FlagUsername, &addUsername, "", "Username for DevPortal basic auth (not recommended, use interactive mode)")
	utils.AddStringFlag(addCmd, utils.FlagPassword, &addPassword, "", "Password for DevPortal basic auth (not recommended, use interactive mode)")
	utils.AddStringFlag(addCmd, utils.FlagToken, &addToken, "", "Token for DevPortal OAuth auth (not recommended, use interactive mode)")
	utils.AddStringFlag(addCmd, utils.FlagAPIKey, &addAPIKey, "", "API key for DevPortal API key auth (not recommended, use interactive mode)")
	utils.AddBoolFlag(addCmd, utils.FlagNoInteractive, &addNoInteractive, false, "Skip interactive prompts")
}

func runAddCommand() error {
	var err error

	if !addNoInteractive {
		if strings.TrimSpace(addName) == "" {
			addName, err = utils.PromptInput("Enter DevPortal display name: ")
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
			addServer, err = utils.PromptInput("Enter DevPortal server URL: ")
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
		return fmt.Errorf("invalid auth type '%s'. DevPortal supports only: %s, %s, %s", addAuth, utils.AuthTypeBasic, utils.AuthTypeOAuth, utils.AuthTypeAPIKey)
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
		username, password, token, apiKey, err = utils.PromptDevPortalCredentials(addAuth)
		if err != nil {
			return fmt.Errorf("failed to read credentials: %w", err)
		}
	}

	switch addAuth {
	case utils.AuthTypeBasic:
		if (username != "" && password == "") || (username == "" && password != "") {
			return fmt.Errorf("for basic auth, both username and password must be provided, or leave both empty to use environment variables (%s and %s)",
				utils.EnvDevPortalUsername, utils.EnvDevPortalPassword)
		}
	case utils.AuthTypeOAuth:
		// empty token means env var usage
	case utils.AuthTypeAPIKey:
		// empty api key means env var usage
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	resolvedPlatform := cfg.ResolvePlatform(addPlatform)

	devPortal := config.DevPortal{
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

	if err := cfg.AddDevPortalToPlatform(resolvedPlatform, devPortal); err != nil {
		return fmt.Errorf("failed to add DevPortal: %w", err)
	}

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	configPath, err := config.GetConfigPath()
	if err != nil {
		utils.LogWarning("could not determine config path", err)
		configPath = "(unknown location)"
	}

	fmt.Printf("DevPortal %s added (platform: %s, server: %s, auth: %s)\n", addName, resolvedPlatform, addServer, addAuth)
	fmt.Printf("Configuration saved to: %s\n", configPath)

	if username != "" || password != "" || token != "" || apiKey != "" {
		switch addAuth {
		case utils.AuthTypeBasic:
			fmt.Printf("Note: Credentials were stored in the configuration; exporting %s and %s will override them at runtime.\n", utils.EnvDevPortalUsername, utils.EnvDevPortalPassword)
		case utils.AuthTypeOAuth:
			fmt.Printf("Note: Credentials were stored in the configuration; exporting %s will override them at runtime.\n", utils.EnvDevPortalToken)
		case utils.AuthTypeAPIKey:
			fmt.Printf("Note: Credentials were stored in the configuration; exporting %s will override them at runtime.\n", utils.EnvDevPortalAPIKey)
		}
	} else {
		fmt.Println()
		lines := []string{
			"No credentials stored for this DevPortal.",
			"Set the following environment variable(s) before making DevPortal API calls:",
		}
		switch addAuth {
		case utils.AuthTypeBasic:
			lines = append(lines, "  "+utils.EnvDevPortalUsername, "  "+utils.EnvDevPortalPassword)
		case utils.AuthTypeOAuth:
			lines = append(lines, "  "+utils.EnvDevPortalToken)
		case utils.AuthTypeAPIKey:
			lines = append(lines, "  "+utils.EnvDevPortalAPIKey)
		}
		utils.PrintBoxedMessage(lines)
	}

	return nil
}
