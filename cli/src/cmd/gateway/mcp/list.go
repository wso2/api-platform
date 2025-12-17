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
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
)

const (
	ListCmdLiteral = "list"
	ListCmdExample = `# List all MCPs
apipctl gateway mcp list`
)

var listCmd = &cobra.Command{
	Use:     ListCmdLiteral,
	Short:   "List all MCPs on the gateway",
	Long:    "Retrieves and displays all MCP proxies deployed on the currently active gateway.",
	Example: ListCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runListCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// MCPListItem represents a single MCP in the list response
type MCPListItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Context   string `json:"context"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// MCPListResponse represents the response from GET /mcp-proxies
type MCPListResponse struct {
	Status     string        `json:"status"`
	Count      int           `json:"count"`
	MCPProxies []MCPListItem `json:"mcp_proxies"`
}

func runListCommand() error {
	// Create a client for the active gateway
	client, err := gateway.NewClientForActive()
	if err != nil {
		return err
	}

	// Call the /mcp-proxies endpoint
	resp, err := client.Get("/mcp-proxies")
	if err != nil {
		return fmt.Errorf("failed to call /mcp-proxies endpoint: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check if the response is successful
	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to list MCPs (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var listResp MCPListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Display the MCPs
	if listResp.Count == 0 {
		fmt.Println("No MCPs found on the gateway.")
		return nil
	}

	for i, mcp := range listResp.MCPProxies {
		fmt.Printf("MCP %d:\n", i+1)
		fmt.Printf("  ID: %s\n", mcp.ID)
		fmt.Printf("  Name: %s\n", mcp.Name)
		fmt.Printf("  Version: %s\n", mcp.Version)
		fmt.Printf("  Context: %s\n", mcp.Context)
		fmt.Printf("  Status: %s\n", mcp.Status)
		fmt.Printf("  Created At: %s\n", mcp.CreatedAt)
		if i < len(listResp.MCPProxies)-1 {
			fmt.Println()
		}
	}

	return nil
}
