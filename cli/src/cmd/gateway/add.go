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
	AddCmdExample = `# Add a new gateway (Basic Auth is used by default)
ap gateway add --name dev --server http://localhost:9090

# For Basic Auth, set environment variables before running gateway controller commands:
#   export ` + utils.EnvGatewayUsername + `=admin
#   export ` + utils.EnvGatewayPassword + `=admin
ap gateway add --name dev --server http://localhost:9090`
)

var (
	addName   string
	addServer string
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

	addCmd.MarkFlagRequired(utils.FlagName)
	addCmd.MarkFlagRequired(utils.FlagServer)
}

func runAddCommand() error {
	// Load existing config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// No token is stored anymore; gateways use Basic Auth by default.

	// Create new gateway
	gateway := config.Gateway{
		Name:   addName,
		Server: addServer,
		Token:  "",
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

	// Print success message and guidance about Basic Auth
	fmt.Printf("Gateway in %s added as %s\n", addServer, addName)
	fmt.Printf("Configuration saved to: %s\n", configPath)
	fmt.Printf("Note: gateways use Basic Auth. Ensure %s and %s are exported in your environment for controller commands.\n", utils.EnvGatewayUsername, utils.EnvGatewayPassword)

	return nil
}
