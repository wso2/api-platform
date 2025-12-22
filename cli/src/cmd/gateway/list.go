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
	ListCmdLiteral = "list"
	ListCmdExample = `# List all gateways
ap gateway list`
)

var listCmd = &cobra.Command{
	Use:     ListCmdLiteral,
	Short:   "List all gateways",
	Long:    "List all gateway configurations from the ap config file.",
	Example: ListCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runListCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func runListCommand() error {
	// Load existing config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if there are any gateways
	if len(cfg.Gateways) == 0 {
		fmt.Println("No gateways configured")
		return nil
	}

	// Display as table
	headers := []string{"NAME", "SERVER"}
	rows := make([][]string, 0, len(cfg.Gateways))
	for _, gw := range cfg.Gateways {
		rows = append(rows, []string{gw.Name, gw.Server})
	}
	utils.PrintTable(headers, rows)

	return nil
}
