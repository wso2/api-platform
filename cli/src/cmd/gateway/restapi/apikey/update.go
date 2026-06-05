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
	UpdateCmdExample = `# Rename an API key
ap gateway rest-api api-key update --id reading-list-api-v1.0 --key-name old-key-name --name new-key-name`
)

var (
	updateAPIID   string
	updateKeyName string
	updateNewName string
)

var updateCmd = &cobra.Command{
	Use:     UpdateCmdLiteral,
	Short:   "Update an API key for a REST API",
	Long:    "Updates an existing API key with a new name.",
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
	utils.AddStringFlag(updateCmd, utils.FlagID, &updateAPIID, "", "REST API ID (required)")
	utils.AddStringFlag(updateCmd, utils.FlagKeyName, &updateKeyName, "", "Current name of the API key to update (required)")
	utils.AddStringFlag(updateCmd, utils.FlagPropertyName, &updateNewName, "", "New name for the API key (required)")
	updateCmd.MarkFlagRequired(utils.FlagID)
	updateCmd.MarkFlagRequired(utils.FlagKeyName)
	updateCmd.MarkFlagRequired(utils.FlagPropertyName)
}

func runUpdateCommand(cmd *cobra.Command) error {
	if strings.TrimSpace(updateAPIID) == "" {
		return fmt.Errorf("--%s is required", utils.FlagID)
	}
	if strings.TrimSpace(updateKeyName) == "" {
		return fmt.Errorf("--%s is required", utils.FlagKeyName)
	}
	if strings.TrimSpace(updateNewName) == "" {
		return fmt.Errorf("--%s is required", utils.FlagPropertyName)
	}

	payload := map[string]string{"name": strings.TrimSpace(updateNewName)}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to build API key payload: %w", err)
	}

	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf(utils.GatewayAPIKeyByNamePath, url.PathEscape(updateAPIID), url.PathEscape(updateKeyName))
	resp, err := client.Put(endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to update API key: %w", err)
	}

	fmt.Println("API key updated successfully.")
	return gateway.PrintJSONResponse(resp)
}
