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
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	RevokeCmdLiteral = "revoke"
	RevokeCmdExample = `# Revoke an API key
ap gateway rest-api api-key revoke --id reading-list-api-v1.0 --key-name my-production-key`
)

var (
	revokeAPIID   string
	revokeKeyName string
)

var revokeCmd = &cobra.Command{
	Use:     RevokeCmdLiteral,
	Short:   "Revoke an API key for a REST API",
	Long:    "Invalidates an API key so it can no longer be used for authentication.",
	Example: RevokeCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runRevokeCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	gateway.AddSelectionFlags(revokeCmd)
	utils.AddStringFlag(revokeCmd, utils.FlagID, &revokeAPIID, "", "REST API ID (required)")
	utils.AddStringFlag(revokeCmd, utils.FlagKeyName, &revokeKeyName, "", "Name of the API key to revoke (required)")
	revokeCmd.MarkFlagRequired(utils.FlagID)
	revokeCmd.MarkFlagRequired(utils.FlagKeyName)
}

func runRevokeCommand(cmd *cobra.Command) error {
	if strings.TrimSpace(revokeAPIID) == "" {
		return fmt.Errorf("--%s is required", utils.FlagID)
	}
	if strings.TrimSpace(revokeKeyName) == "" {
		return fmt.Errorf("--%s is required", utils.FlagKeyName)
	}

	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf(utils.GatewayAPIKeyByNamePath, url.PathEscape(revokeAPIID), url.PathEscape(revokeKeyName))
	resp, err := client.Delete(endpoint)
	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to revoke API key: recieved status code %d", resp.StatusCode)
	}

	fmt.Println("API key revoked successfully.")
	return nil
}
