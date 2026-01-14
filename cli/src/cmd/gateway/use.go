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
	UseCmdLiteral = "use"
	UseCmdExample = `# Set dev as the active gateway
ap gateway use --name dev`
)

var useName string

var useCmd = &cobra.Command{
	Use:     UseCmdLiteral,
	Short:   "Set the active gateway",
	Long:    "Set the active gateway that will be used by default for operations.",
	Example: UseCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUseCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(useCmd, utils.FlagName, &useName, "", "Name of the gateway to use (required)")
	useCmd.MarkFlagRequired(utils.FlagName)
}

func runUseCommand() error {
	// Load existing config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if gateway exists and get it
	gateway, err := cfg.GetGateway(useName)
	if err != nil {
		return err
	}

	// Set as active
	if err := cfg.SetActiveGateway(useName); err != nil {
		return err
	}

	// Save config
	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Gateway set to %s (auth: %s).\n", useName, gateway.Auth)

	// Validate credentials availability for the gateway's auth type
	// Only warn if BOTH env vars AND config credentials are missing
	if gateway.Auth != utils.AuthTypeNone {
		hasEnvCreds := utils.HasEnvCredentials(gateway.Auth)
		hasConfigCreds := utils.HasConfigCredentials(gateway.Auth, gateway.Username, gateway.Password, gateway.Token)

		if !hasEnvCreds && !hasConfigCreds {
			// Neither env vars nor config credentials are available
			missing, _, err := utils.ValidateAuthEnvVars(gateway.Auth)
			if err != nil {
				fmt.Printf("\nWarning: failed to validate auth env vars: %v\n", err)
			} else {
				fmt.Println("\n" + utils.FormatMissingEnvVarsWarning(gateway.Auth, missing))
			}
		} else if hasConfigCreds && !hasEnvCreds {
			// Config has credentials, env vars not set - this is fine, just inform
			fmt.Println("Using credentials from configuration.")
		} else if hasEnvCreds {
			// Env vars are set - they will be used
			fmt.Println("Using credentials from environment variables.")
		}
	}

	return nil
}
