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
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	UpdateCmdLiteral = "update"
	UpdateCmdExample = `# Replace an API key's value with a custom, externally generated one
ap gateway graphql-api api-key update --id countries-graphql-api --key-name my-production-key --api-key <36+ character value>`
)

var (
	updateAPIID     string
	updateKeyName   string
	updateNewAPIKey string
)

var updateCmd = &cobra.Command{
	Use:     UpdateCmdLiteral,
	Short:   "Update an API key for a GraphQL API",
	Long:    "Replaces an existing API key's value with a custom plain-text value instead of an auto-generated one. The key must be at least 36 characters. It is hashed before storage; the plaintext is not echoed back.",
	Example: UpdateCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUpdateCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	gateway.AddSelectionFlags(updateCmd)
	utils.AddStringFlag(updateCmd, utils.FlagID, &updateAPIID, "", "GraphQL API ID (required)")
	utils.AddStringFlag(updateCmd, utils.FlagKeyName, &updateKeyName, "", "Name of the API key to update (required)")
	utils.AddStringFlag(updateCmd, utils.FlagAPIKey, &updateNewAPIKey, "", "New plain-text API key value, minimum 36 characters (required)")
	updateCmd.MarkFlagRequired(utils.FlagID)
	updateCmd.MarkFlagRequired(utils.FlagKeyName)
	updateCmd.MarkFlagRequired(utils.FlagAPIKey)
}

func runUpdateCommand(cmd *cobra.Command) error {
	if strings.TrimSpace(updateAPIID) == "" {
		return fmt.Errorf("--%s is required", utils.FlagID)
	}
	if strings.TrimSpace(updateKeyName) == "" {
		return fmt.Errorf("--%s is required", utils.FlagKeyName)
	}
	if strings.TrimSpace(updateNewAPIKey) == "" {
		return fmt.Errorf("--%s is required", utils.FlagAPIKey)
	}

	// The server persists this as the key's hash; the request body field is
	// "apiKey" per APIKeyCreationRequest — never "name" (renaming a key is not
	// what this endpoint does).
	payload := map[string]string{"apiKey": strings.TrimSpace(updateNewAPIKey)}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to build API key payload: %w", err)
	}

	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	// Client.Put already treats any non-2xx status as an error and returns a
	// nil *http.Response in that case, so err == nil here always means success.
	endpoint := fmt.Sprintf(utils.GatewayGraphQLAPIKeyByNamePath, url.PathEscape(updateAPIID), url.PathEscape(updateKeyName))
	resp, err := client.Put(endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to update API key: %w", err)
	}

	fmt.Println("API key updated successfully.")
	return gateway.PrintJSONResponse(resp)
}
