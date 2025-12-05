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
	"encoding/json"
	"fmt"
	"os"

	"github.com/wso2/api-platform/cli/utils"

	"github.com/spf13/cobra"
)

const (
	GenMcpCmdLiteral = "mcp"
	GenMcpCmdExample = `# Generate MCP configuration` + "\n" + CliName + ` ` + GenCmdLiteral +
		` ` + GenMcpCmdLiteral + ` -s http://localhost:3031/mcp`

	MethodInitialized   = "notifications/initialized"
	MethodToolsList     = "tools/list"
	MethodPromptsList   = "prompts/list"
	MethodResourcesList = "resources/list"
)

var mcpServerURL string
var outputDirectory string

var genMcpCmd = &cobra.Command{
	Use:     GenMcpCmdLiteral,
	Short:   "Generate MCP configuration",
	Long:    "Generate an MCP configuration which can be deployed in the WSO2 API Platform Gateway.",
	Example: GenMcpCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		generateMCPConfiguration(mcpServerURL, outputDirectory)
	},
}

func init() {
	genCmd.AddCommand(genMcpCmd)
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	genMcpCmd.Flags().StringVarP(&mcpServerURL, "server", "s", "http://localhost:3001/mcp", "MCP server URL")
	genMcpCmd.Flags().StringVarP(&outputDirectory, "output", "o", cwd, "Output directory for generated configuration")
}

// generateMCPConfiguration handles the MCP configuration generation process
func generateMCPConfiguration(url string, outputDir string) {
	fmt.Printf("Generating MCP configuration for server: %s\n", url)

	// Step 1: initialize
	fmt.Println("→ Sending initialize...")
	sessionID, err := utils.InitializeMCPServer(url)
	if err != nil {
		fmt.Println("ERROR: Failed to initialize MCP server:", err)
		return
	}
	fmt.Println("---------------------------------------------------")

	// Step 2: notifications/initialized
	fmt.Println("→ Sending notifications/initialized...")
	notifyReq := utils.JsonRPCRequest{
		JSONRPC: utils.JsonRpcVersion,
		Method:  MethodInitialized,
	}
	_, err = utils.PostJSONRPCWithSession(url, notifyReq, sessionID)
	if err != nil {
		fmt.Println("ERROR: Failed to send notification:", err)
		return
	}
	fmt.Println("---------------------------------------------------")

	// Step 3: tools/list
	fmt.Println("→ Sending tools/list...")
	toolsReq := utils.JsonRPCRequest{
		JSONRPC: utils.JsonRpcVersion,
		ID:      2,
		Method:  MethodToolsList,
	}
	toolsResp, err := utils.PostJSONRPCWithSession(url, toolsReq, sessionID)
	if err != nil {
		fmt.Println("ERROR: Failed to get tools:", err)
		return
	}
	var toolsResult utils.ToolsResult
	if err := json.Unmarshal(toolsResp, &toolsResult); err != nil {
		fmt.Println("ERROR: Failed to parse tools response:", err)
		return
	}
	if toolsResult.Error != nil {
		fmt.Println("ERROR: tools/list request returned an error:")
		fmt.Println(toolsResult.Error.Message)
		return
	}
	fmt.Printf("→ Available Tools: %d\n", len(toolsResult.Result.Tools))
	fmt.Println("---------------------------------------------------")

	// Step 4: prompts/list
	fmt.Println("→ Sending prompts/list...")
	promptsReq := utils.JsonRPCRequest{
		JSONRPC: utils.JsonRpcVersion,
		ID:      3,
		Method:  MethodPromptsList,
	}
	promptsResp, err := utils.PostJSONRPCWithSession(url, promptsReq, sessionID)
	if err != nil {
		fmt.Println("ERROR: Failed to get prompts:", err)
		return
	}
	var promptsResult utils.PromptsResult
	if err := json.Unmarshal(promptsResp, &promptsResult); err != nil {
		fmt.Println("ERROR: Failed to parse prompts response:", err)
		return
	}
	if promptsResult.Error != nil {
		fmt.Println("ERROR: prompts/list request returned an error:")
		fmt.Println(promptsResult.Error.Message)
		return
	}
	fmt.Printf("→ Available Prompts: %d\n", len(promptsResult.Result.Prompts))
	fmt.Println("---------------------------------------------------")

	// Step 5: resources/list
	fmt.Println("→ Sending resources/list...")
	resourcesReq := utils.JsonRPCRequest{
		JSONRPC: utils.JsonRpcVersion,
		ID:      4,
		Method:  MethodResourcesList,
	}
	resourcesResp, err := utils.PostJSONRPCWithSession(url, resourcesReq, sessionID)
	if err != nil {
		fmt.Println("ERROR: Failed to get resources:", err)
		return
	}
	var resourcesResult utils.ResourcesResult
	if err := json.Unmarshal(resourcesResp, &resourcesResult); err != nil {
		fmt.Println("ERROR: Failed to parse resources response:", err)
		return
	}
	if resourcesResult.Error != nil {
		fmt.Println("ERROR: resources/list request returned an error:")
		fmt.Println(resourcesResult.Error.Message)
		return
	}
	fmt.Printf("→ Available Resources: %d\n", len(resourcesResult.Result.Resources))
	fmt.Println("---------------------------------------------------")

	// Generate MCP configuration file
	err = utils.GenerateMCPConfigFile(url, toolsResult, resourcesResult, promptsResult, outputDir)
	if err != nil {
		fmt.Println("ERROR: Failed to generate MCP configuration file:", err)
		return
	}
	fmt.Println("→ Generated MCP configuration YAML file: generated-mcp.yaml")
}
