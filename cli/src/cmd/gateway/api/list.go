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

package api

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
	ListCmdExample = `# List all APIs
ap gateway api list`
)

var listCmd = &cobra.Command{
	Use:     ListCmdLiteral,
	Short:   "List all APIs on the gateway",
	Long:    "Retrieves and displays all APIs deployed on the currently active gateway.",
	Example: ListCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runListCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// APIListItem represents a single API in the list response
type APIListItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Context   string `json:"context"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// APIListResponse represents the response from GET /apis
type APIListResponse struct {
	Status string        `json:"status"`
	Count  int           `json:"count"`
	APIs   []APIListItem `json:"apis"`
}

func runListCommand() error {
	// Create a client for the active gateway
	client, err := gateway.NewClientForActive()
	if err != nil {
		return err
	}

	// Call the /apis endpoint
	resp, err := client.Get(utils.GatewayAPIsPath)
	if err != nil {
		return fmt.Errorf("failed to call %s endpoint: %w", utils.GatewayAPIsPath, err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check if the response is successful
	if resp.StatusCode != 200 {
		return gateway.FormatHTTPError("List APIs", resp, "Gateway Controller")
	}

	// Parse the response
	var listResp APIListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Display the APIs
	if listResp.Count == 0 {
		fmt.Println("No APIs found on the gateway.")
		return nil
	}

	for i, api := range listResp.APIs {
		fmt.Printf("API %d:\n", i+1)
		fmt.Printf("  ID: %s\n", api.ID)
		fmt.Printf("  Name: %s\n", api.Name)
		fmt.Printf("  Version: %s\n", api.Version)
		fmt.Printf("  Context: %s\n", api.Context)
		fmt.Printf("  Status: %s\n", api.Status)
		fmt.Printf("  Created At: %s\n", api.CreatedAt)
		if i < len(listResp.APIs)-1 {
			fmt.Println()
		}
	}

	return nil
}
