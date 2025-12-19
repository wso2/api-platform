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
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	AddCmdLiteral = "add"
	AddCmdExample = `# Add a new gateway
ap gateway add --name dev --server http://localhost:9090

# Add a gateway with authentication token
ap gateway add --name prod --server https://api.example.com --token <TOKEN>

# Add a gateway with insecure connection (skip TLS verification)
ap gateway add --name local --server https://localhost:9090 --insecure`
)

var (
	addName        string
	addServer      string
	addToken       string
	addEnvToken    string
	addUsername    string
	addPassword    string
	addPasswordEnv string
	addInsecure    bool
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
	utils.AddStringFlag(addCmd, utils.FlagToken, &addToken, "", "Bearer token for authentication")
	utils.AddStringFlag(addCmd, utils.FlagEnvToken, &addEnvToken, "", "Environment variable name to read bearer token from")
	utils.AddStringFlag(addCmd, utils.FlagUsername, &addUsername, "", "Username for basic authentication")
	utils.AddStringFlag(addCmd, utils.FlagPassword, &addPassword, "", "Password for basic authentication")
	utils.AddStringFlag(addCmd, utils.FlagPasswordEnv, &addPasswordEnv, "", "Environment variable name to read password from")
	utils.AddBoolFlag(addCmd, utils.FlagInsecure, &addInsecure, false, "Allow insecure server connections (no authentication)")

	addCmd.MarkFlagRequired(utils.FlagName)
	addCmd.MarkFlagRequired(utils.FlagServer)
}

func runAddCommand() error {
	// Load existing config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Validate authentication flags
	hasToken := addToken != "" || addEnvToken != ""
	hasBasicAuth := addUsername != "" || addPassword != "" || addPasswordEnv != ""

	if hasToken && hasBasicAuth {
		return fmt.Errorf("cannot specify both bearer token and basic auth flags")
	}

	if addToken != "" && addEnvToken != "" {
		return fmt.Errorf("cannot specify both --%s and --%s flags", utils.FlagToken, utils.FlagEnvToken)
	}

	if addPassword != "" && addPasswordEnv != "" {
		return fmt.Errorf("cannot specify both --%s and --%s flags", utils.FlagPassword, utils.FlagPasswordEnv)
	}

	if hasBasicAuth && addUsername == "" {
		return fmt.Errorf("--%s is required when using basic authentication", utils.FlagUsername)
	}

	var token string
	var basicAuth *config.BasicAuth
	var insecure bool

	// Handle bearer token authentication
	if addToken != "" {
		fmt.Printf("\n⚠️  Warning: Providing tokens via --%s flag is not recommended\n", utils.FlagToken)
		fmt.Println("   Tokens in command-line arguments may be visible in:")
		fmt.Println("   • Shell history")
		fmt.Println("   • Process lists")
		fmt.Println("   • Log files")
		fmt.Printf("   Next time, use --%s or omit for interactive prompt.\n", utils.FlagEnvToken)
		token = addToken
	} else if addEnvToken != "" {
		token = fmt.Sprintf("${%s}", addEnvToken)
	} else if hasBasicAuth {
		// Handle basic auth
		password := addPassword
		if addPasswordEnv != "" {
			password = fmt.Sprintf("${%s}", addPasswordEnv)
		} else if addPassword != "" {
			fmt.Printf("\n⚠️  Warning: Providing passwords via --%s flag is not recommended\n", utils.FlagPassword)
			fmt.Println("   Passwords in command-line arguments may be visible in:")
			fmt.Println("   • Shell history")
			fmt.Println("   • Process lists")
			fmt.Println("   • Log files")
			fmt.Printf("   Next time, use --%s or omit for interactive prompt.\n", utils.FlagPasswordEnv)
		} else {
			// Interactive password prompt
			fmt.Print("Password: ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}
			password = strings.TrimSpace(input)
		}

		basicAuth = &config.BasicAuth{
			Username: addUsername,
			Password: password,
		}
	} else if addInsecure {
		insecure = true
	} else {
		// Interactive prompt for token
		fmt.Println("\nAuthentication token (leave empty for insecure connection):")
		fmt.Println("  • If provided: connection will use Bearer token authentication")
		fmt.Println("  • If empty: connection will be marked as insecure")
		fmt.Print("Token: ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read token: %w", err)
		}
		token = strings.TrimSpace(input)
		if token == "" {
			insecure = true
		}
	}

	// Create new gateway
	gateway := config.Gateway{
		Name:      addName,
		Server:    addServer,
		Token:     token,
		BasicAuth: basicAuth,
		Insecure:  insecure,
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
	fmt.Printf("Gateway in %s added as %s\n", addServer, addName)
	fmt.Printf("Configuration saved to: %s\n", configPath)

	return nil
}
