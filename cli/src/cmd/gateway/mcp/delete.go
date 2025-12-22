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
	"net/url"
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	DeleteCmdLiteral = "delete"
	DeleteCmdExample = `# Delete an MCP by ID
ap gateway mcp delete --id sample-1`
)

var (
	deleteMCPID string
)

var deleteCmd = &cobra.Command{
	Use:     DeleteCmdLiteral,
	Short:   "Delete an MCP from the gateway",
	Long:    "Deletes a specific MCP from the gateway by ID.",
	Example: DeleteCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDeleteCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(deleteCmd, utils.FlagID, &deleteMCPID, "", "MCP ID (handle) to delete")
	deleteCmd.MarkFlagRequired(utils.FlagID)
}

func runDeleteCommand() error {
	// Proceed with deletion (no confirm flag required)

	// Create a client for the active gateway
	client, err := gateway.NewClientForActive()
	if err != nil {
		return err
	}

	// Call the DELETE endpoint
	resp, err := client.Delete(fmt.Sprintf(utils.GatewayMCPProxyByIDPath, url.PathEscape(deleteMCPID)))
	if err != nil {
		return fmt.Errorf("failed to delete MCP: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Handle different status codes
	if resp.StatusCode == 404 {
		return fmt.Errorf("MCP with ID '%s' not found", deleteMCPID)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		// Try to parse error message from response
		var errorResp map[string]interface{}
		if json.Unmarshal(body, &errorResp) == nil {
			if msg, ok := errorResp["message"].(string); ok {
				return fmt.Errorf("failed to delete MCP (status %d): %s", resp.StatusCode, msg)
			}
		}
		return fmt.Errorf("failed to delete MCP (status %d): %s", resp.StatusCode, string(body))
	}

	// Success
	fmt.Println("MCP deleted successfully.")
	return nil
}
