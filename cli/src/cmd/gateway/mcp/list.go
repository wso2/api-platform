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
	"github.com/wso2/api-platform/cli/utils"
)

const (
	ListCmdLiteral = "list"
	ListCmdExample = `# List all MCPs
ap gateway mcp list`
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

// MCPListItem is a list-view projection of an MCPProxy. The management API list
// response returns each item as a full k8s-shaped resource body — we flatten
// the fields we care about out of `metadata`, `spec` and `status` here.
type MCPListItem struct {
	Metadata map[string]interface{} `json:"metadata"`
	Spec     map[string]interface{} `json:"spec"`
	Status   map[string]interface{} `json:"status"`
}

// ID returns the server-assigned id (status.id) falling back to metadata.name.
func (i MCPListItem) ID() string {
	if v, ok := i.Status["id"].(string); ok && v != "" {
		return v
	}
	if v, ok := i.Metadata["name"].(string); ok {
		return v
	}
	return ""
}

// DisplayName returns spec.displayName.
func (i MCPListItem) DisplayName() string {
	if v, ok := i.Spec["displayName"].(string); ok {
		return v
	}
	return ""
}

// Version returns spec.version.
func (i MCPListItem) Version() string {
	if v, ok := i.Spec["version"].(string); ok {
		return v
	}
	return ""
}

// Context returns spec.context.
func (i MCPListItem) Context() string {
	if v, ok := i.Spec["context"].(string); ok {
		return v
	}
	return ""
}

// State returns status.state.
func (i MCPListItem) State() string {
	if v, ok := i.Status["state"].(string); ok {
		return v
	}
	return ""
}

// CreatedAt returns status.createdAt as a string.
func (i MCPListItem) CreatedAt() string {
	if v, ok := i.Status["createdAt"].(string); ok {
		return v
	}
	return ""
}

// MCPListResponse represents the response from GET ${managementBase}/mcp-proxies
type MCPListResponse struct {
	Status     string        `json:"status"`
	Count      int           `json:"count"`
	MCPProxies []MCPListItem `json:"mcpProxies"`
}

func runListCommand() error {
	// Create a client for the active gateway
	client, err := gateway.NewClientForActive()
	if err != nil {
		return err
	}

	// Call the management MCP proxies endpoint
	resp, err := client.Get(utils.GatewayMCPProxiesPath)
	if err != nil {
		return fmt.Errorf("failed to call %s endpoint: %w", utils.GatewayMCPProxiesPath, err)
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

	// Display the MCPs as a table when present
	if listResp.Count == 0 {
		fmt.Println("No MCPs found on the gateway.")
		return nil
	}

	headers := []string{"ID", "DISPLAY_NAME", "VERSION", "CONTEXT", "STATE", "CREATED_AT"}
	rows := make([][]string, 0, len(listResp.MCPProxies))
	for _, mcp := range listResp.MCPProxies {
		rows = append(rows, []string{mcp.ID(), mcp.DisplayName(), mcp.Version(), mcp.Context(), mcp.State(), mcp.CreatedAt()})
	}
	utils.PrintTable(headers, rows)

	return nil
}
