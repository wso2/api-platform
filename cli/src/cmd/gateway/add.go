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

	// Determine insecure flag: default to true if neither token nor insecure flag is provided
	insecure := addInsecure
	if addToken == "" && !addInsecure {
		insecure = true
	}

	// Create new gateway
	gateway := config.Gateway{
		Name:     addName,
		Server:   addServer,
		Token:    addToken,
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
