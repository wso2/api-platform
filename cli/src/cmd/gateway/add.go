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

package gateway

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
	AddCmdExample = `# Add a new gateway with no authentication
ap gateway add --display-name dev --server http://localhost:9090

# Add a gateway with basic authentication (interactive prompts)
ap gateway add --display-name dev --server http://localhost:9090 --auth basic

# Add a gateway with bearer token authentication (interactive prompts)
ap gateway add --display-name prod --server https://api.example.com --auth bearer

# Add a gateway without interactive prompts (credentials must be in environment variables)
ap gateway add --display-name dev --server http://localhost:9090 --auth basic --no-interactive

# For Basic Auth, set environment variables before running gateway commands:
#   export ` + utils.EnvGatewayUsername + `=admin
#   export ` + utils.EnvGatewayPassword + `=admin

# For Bearer Auth, set environment variable before running gateway commands:
#   export ` + utils.EnvGatewayToken + `=your_token_here`
)

var (
	addName          string
	addServer        string
	addAuth          string
	addUsername      string
	addPassword      string
	addToken         string
	addNoInteractive bool
)

var addCmd = &cobra.Command{
	Use:     AddCmdLiteral,
	Short:   "Add a new gateway",
	Long:    "Add a new gateway configuration to the ap config file.",
	Example: AddCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runAddCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(addCmd, utils.FlagName, &addName, "", "Name of the gateway (required)")
	utils.AddStringFlag(addCmd, utils.FlagServer, &addServer, "", "Server URL of the gateway (required)")
	utils.AddStringFlag(addCmd, utils.FlagAuth, &addAuth, utils.AuthTypeNone, "Authentication type: none, basic, or bearer (default: none)")
	utils.AddStringFlag(addCmd, utils.FlagUsername, &addUsername, "", "Username for basic auth (not recommended, use interactive mode)")
	utils.AddStringFlag(addCmd, utils.FlagPassword, &addPassword, "", "Password for basic auth (not recommended, use interactive mode)")
	utils.AddStringFlag(addCmd, utils.FlagToken, &addToken, "", "Token for bearer auth (not recommended, use interactive mode)")
	utils.AddBoolFlag(addCmd, utils.FlagNoInteractive, &addNoInteractive, false, "Skip interactive prompts for credentials")

	addCmd.MarkFlagRequired(utils.FlagName)
	addCmd.MarkFlagRequired(utils.FlagServer)
}

func runAddCommand() error {
	// Validate auth type
	addAuth = strings.ToLower(addAuth)
	if addAuth != utils.AuthTypeNone && addAuth != utils.AuthTypeBasic && addAuth != utils.AuthTypeBearer {
		return fmt.Errorf("invalid auth type '%s'. Must be one of: none, basic, bearer", addAuth)
	}

	// Validate credential flags match the auth type
	switch addAuth {
	case utils.AuthTypeNone:
		if addUsername != "" || addPassword != "" || addToken != "" {
			return fmt.Errorf("credential flags (--username, --password, --token) cannot be used with auth type 'none'")
		}
	case utils.AuthTypeBasic:
		if addToken != "" {
			return fmt.Errorf("--token flag cannot be used with auth type 'basic'. Use --username and --password instead")
		}
	case utils.AuthTypeBearer:
		if addUsername != "" || addPassword != "" {
			return fmt.Errorf("--username and --password flags cannot be used with auth type 'bearer'. Use --token instead")
		}
	}

	// Check if credential flags were used and warn
	if addUsername != "" || addPassword != "" || addToken != "" {
		fmt.Println("Warning: Passing credentials via command-line flags is not recommended for security reasons.")
		fmt.Println("Consider using interactive mode or environment variables instead.")
		fmt.Println()
	}

	var username, password, token string
	var err error

	// Handle credentials based on auth type and interactive mode
	if addAuth != utils.AuthTypeNone && !addNoInteractive {
		// Interactive mode - prompt for credentials
		if addUsername == "" && addPassword == "" && addToken == "" {
			// No flags provided, use interactive prompts
			username, password, token, err = utils.PromptCredentials(addAuth)
			if err != nil {
				return fmt.Errorf("failed to read credentials: %w", err)
			}
		} else {
			// Flags were provided, use them (warning already shown above)
			username = addUsername
			password = addPassword
			token = addToken
		}
	} else if addAuth != utils.AuthTypeNone && addNoInteractive {
		// No-interactive mode with auth - use flags if provided, otherwise skip credentials
		username = addUsername
		password = addPassword
		token = addToken
	}

	// Validate credential completeness after collection
	switch addAuth {
	case utils.AuthTypeBasic:
		// For Basic auth: either both username+password are set, or neither (use env vars)
		if (username != "" && password == "") || (username == "" && password != "") {
			return fmt.Errorf("for basic auth, both username and password must be provided, or leave both empty to use environment variables (%s and %s)",
				utils.EnvGatewayUsername, utils.EnvGatewayPassword)
		}
	case utils.AuthTypeBearer:
		// For Bearer auth: token is either set or empty (use env var)
		// No additional validation needed as token is a single field
	}

	// Load existing config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create new gateway
	gateway := config.Gateway{
		Name:   addName,
		Server: addServer,
		Auth:   addAuth,
	}

	// Only store credentials if they are not empty
	if username != "" {
		gateway.Username = username
	}
	if password != "" {
		gateway.Password = password
	}
	if token != "" {
		gateway.Token = token
	}

	// Add gateway to config
	if err := cfg.AddGateway(gateway); err != nil {
		return fmt.Errorf("failed to add gateway: %w", err)
	}

	// Save config
	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Get config path for display
	configPath, err := config.GetConfigPath()
	if err != nil {
		utils.LogWarning("could not determine config path", err)
		configPath = "(unknown location)"
	}

	// Print success message
	fmt.Printf("Gateway %s added as %s with auth type: %s\n", addServer, addName, addAuth)
	fmt.Printf("Configuration saved to: %s\n", configPath)

	// Show info message based on whether credentials were stored
	if addAuth != utils.AuthTypeNone {
		hasStoredCreds := utils.HasCredentials(addAuth, username, password, token)
		if !hasStoredCreds {
			// Show boxed message about environment variables
			fmt.Println()
			var envVars []string
			switch addAuth {
			case utils.AuthTypeBasic:
				envVars = []string{utils.EnvGatewayUsername, utils.EnvGatewayPassword}
			case utils.AuthTypeBearer:
				envVars = []string{utils.EnvGatewayToken}
			}

			lines := []string{
				"No credentials stored for this gateway.",
				"Set the following environment variable(s) before making API calls:",
			}
			for _, envVar := range envVars {
				lines = append(lines, "  "+envVar)
			}
			utils.PrintBoxedMessage(lines)
		}
	}

	return nil
}
