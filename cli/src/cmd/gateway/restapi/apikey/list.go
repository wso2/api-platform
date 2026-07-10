/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package apikey

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	ListCmdLiteral = "list"
	ListCmdExample = `# List all API keys for a REST API
ap gateway rest-api api-key list --id reading-list-api-v1.0`
)

var listAPIID string

var listCmd = &cobra.Command{
	Use:     ListCmdLiteral,
	Short:   "List API keys for a REST API",
	Long:    "Retrieves and displays all API keys for a REST API on the currently active gateway.",
	Example: ListCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runListCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	gateway.AddSelectionFlags(listCmd)
	utils.AddStringFlag(listCmd, utils.FlagID, &listAPIID, "", "REST API ID (required)")
	listCmd.MarkFlagRequired(utils.FlagID)
}

// APIKey is a list-view projection of an API key. The plaintext apiKey value is
// only present on create/regenerate responses, so it is intentionally omitted
// from the list table.
type APIKey struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	APIID       string `json:"apiId"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
	ExpiresAt   string `json:"expiresAt"`
}

// APIKeyListResponse represents the response from GET /rest-apis/{id}/api-keys.
type APIKeyListResponse struct {
	APIKeys    []APIKey `json:"apiKeys"`
	TotalCount int      `json:"totalCount"`
	Status     string   `json:"status"`
}

func runListCommand(cmd *cobra.Command) error {
	if strings.TrimSpace(listAPIID) == "" {
		return fmt.Errorf("--%s is required", utils.FlagID)
	}

	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf(utils.GatewayAPIKeysPath, url.PathEscape(listAPIID))
	resp, err := client.Get(endpoint)
	if err != nil {
		return fmt.Errorf("failed to call %s endpoint: %w", endpoint, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("REST API with ID '%s' not found", listAPIID)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to list API keys (status %d): %s", resp.StatusCode, string(body))
	}

	var listResp APIKeyListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(listResp.APIKeys) == 0 {
		fmt.Printf("No API keys found for REST API '%s'.\n", listAPIID)
		return nil
	}

	headers := []string{"NAME", "DISPLAY_NAME", "API_ID", "STATUS", "CREATED_AT", "EXPIRES_AT"}
	rows := make([][]string, 0, len(listResp.APIKeys))
	for _, k := range listResp.APIKeys {
		rows = append(rows, []string{k.Name, k.DisplayName, k.APIID, k.Status, k.CreatedAt, k.ExpiresAt})
	}
	utils.PrintTable(headers, rows)

	return nil
}
