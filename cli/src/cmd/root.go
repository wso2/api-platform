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

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/cmd/gateway"
	"github.com/wso2/api-platform/cli/utils"
)

var rootCmd = &cobra.Command{
	Use:   "ap",
	Short: "ap is a CLI tool to interact with the WSO2 API Platform",
	Long: `ap - WSO2 API Platform CLI

A command-line tool for managing and interacting with the WSO2 API Platform.

USAGE:
  ap [command] [subcommand] [flags]


EXAMPLES:
  # Add a gateway
  ap gateway add -n dev -s http://localhost:9090

  # Generate MCP configuration  
  ap gateway mcp generate -s http://localhost:3001/mcp -o target

  # Show version
  ap version

For detailed documentation, see: src/HELP.md`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of ap",
	Long:  "Print the version and build information of ap",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ap version v%s (built at %s)\n", utils.Version, utils.BuildTime)
	},
}

func init() {
	rootCmd.AddCommand(gateway.GatewayCmd)
	rootCmd.AddCommand(versionCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Oops. An error occurred while executing %s: %v\n", utils.CliName, err)
		os.Exit(1)
	}
}
