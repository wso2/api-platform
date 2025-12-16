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

package mcp

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	mcpgen "github.com/wso2/api-platform/cli/internal/mcp"
)

const (
	GenerateCmdLiteral = "generate"
	GenerateCmdExample = `# Generate MCP configuration
apipctl gateway mcp generate --server http://localhost:3001/mcp --output target

# Generate MCP configuration with default output directory (current directory)
apipctl gateway mcp generate --server http://localhost:3001/mcp`
)

var (
	generateServer string
	generateOutput string
)

var generateCmd = &cobra.Command{
	Use:     GenerateCmdLiteral,
	Short:   "Generate MCP configuration",
	Long:    "Generate an MCP configuration which can be deployed in the WSO2 API Platform Gateway.",
	Example: GenerateCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGenerateCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	generateCmd.Flags().StringVarP(&generateServer, "server", "s", "", "MCP server URL (required)")
	generateCmd.Flags().StringVarP(&generateOutput, "output", "o", cwd, "Output directory for generated configuration")

	generateCmd.MarkFlagRequired("server")
}

func runGenerateCommand() error {
	return mcpgen.Generate(generateServer, generateOutput)
}
