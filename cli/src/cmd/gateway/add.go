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
apipctl gateway add --name dev --server http://localhost:9090

# Add a gateway with authentication token
apipctl gateway add --name prod --server https://api.example.com --token <TOKEN>

# Add a gateway with insecure connection (skip TLS verification)
apipctl gateway add --name local --server https://localhost:9090 --insecure`
)

var (
	addName     string
	addServer   string
	addToken    string
	addEnvToken string
	addInsecure bool
)

var addCmd = &cobra.Command{
	Use:     AddCmdLiteral,
	Short:   "Add a new gateway",
	Long:    "Add a new gateway configuration to the apipctl config file.",
	Example: AddCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runAddCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	addCmd.Flags().StringVarP(&addName, "name", "n", "", "Name of the gateway (required)")
	addCmd.Flags().StringVarP(&addServer, "server", "s", "", "Server URL of the gateway (required)")
	addCmd.Flags().StringVarP(&addToken, "token", "t", "", "Authentication token for the gateway")
	addCmd.Flags().StringVarP(&addEnvToken, "env-token", "e", "", "Environment variable name to read token from")
	addCmd.Flags().BoolVarP(&addInsecure, "insecure", "i", false, "Allow insecure server connections")

	addCmd.MarkFlagRequired("name")
	addCmd.MarkFlagRequired("server")
}

func runAddCommand() error {
	// Load existing config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Handle token input: --token, --env-token, or interactive prompt
	var token string

	// Check if both --token and --env-token are provided
	if addToken != "" && addEnvToken != "" {
		return fmt.Errorf("cannot specify both --token and --env-token flags")
	}

	if addToken != "" {
		// Warn if token was provided via flag (security risk)
		fmt.Println("\n⚠️  Warning: Providing tokens via --token flag is not recommended")
		fmt.Println("   Tokens in command-line arguments may be visible in:")
		fmt.Println("   • Shell history")
		fmt.Println("   • Process lists")
		fmt.Println("   • Log files")
		fmt.Println("   Next time, use --env-token or omit both flags for interactive prompt.")
		token = addToken
	} else if addEnvToken != "" {
		// Store environment variable reference
		token = fmt.Sprintf("${%s}", addEnvToken)
	} else if !addInsecure {
		// Interactive prompt
		fmt.Println("\nAuthentication token (leave empty for insecure connection):")
		fmt.Println("  • If provided: connection will use Bearer token authentication")
		fmt.Println("  • If empty: connection will be marked as insecure (--insecure=true)")
		fmt.Print("Token: ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read token: %w", err)
		}
		token = strings.TrimSpace(input)
	}

	// Determine insecure flag: default to true if no token provided
	insecure := addInsecure
	if token == "" && !addInsecure {
		insecure = true
	}

	// Create new gateway
	gateway := config.Gateway{
		Name:     addName,
		Server:   addServer,
		Token:    token,
		Insecure: insecure,
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
