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
)

const (
	CurrentCmdLiteral = "current"
	CurrentCmdExample = `# Show the current active gateway
ap gateway current`
)

var currentCmd = &cobra.Command{
	Use:     CurrentCmdLiteral,
	Short:   "Show the current active gateway",
	Long:    "Display the current active gateway configuration.",
	Example: CurrentCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runCurrentCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func runCurrentCommand() error {
	// Load existing config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get active gateway
	gateway, err := cfg.GetActiveGateway()
	if err != nil {
		return err
	}

	// Display gateway info
	securityStatus := "secure"
	if gateway.Insecure {
		securityStatus = "insecure"
	}
	fmt.Printf("Current gateway: %s - %s (%s)\n", gateway.Name, gateway.Server, securityStatus)

	return nil
}
