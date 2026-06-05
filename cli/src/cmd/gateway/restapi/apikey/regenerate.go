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
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	RegenerateCmdLiteral = "regenerate"
	RegenerateCmdExample = `# Regenerate an API key, replacing its previous value
ap gateway rest-api api-key regenerate --id reading-list-api-v1.0 --key-name my-production-key`
)

var (
	regenerateAPIID   string
	regenerateKeyName string
)

var regenerateCmd = &cobra.Command{
	Use:     RegenerateCmdLiteral,
	Short:   "Regenerate an API key for a REST API",
	Long:    "Creates a new API key value replacing the previous one. The new plaintext key is returned once in the response.",
	Example: RegenerateCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runRegenerateCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	gateway.AddSelectionFlags(regenerateCmd)
	utils.AddStringFlag(regenerateCmd, utils.FlagID, &regenerateAPIID, "", "REST API ID (required)")
	utils.AddStringFlag(regenerateCmd, utils.FlagKeyName, &regenerateKeyName, "", "Name of the API key to regenerate (required)")
	regenerateCmd.MarkFlagRequired(utils.FlagID)
	regenerateCmd.MarkFlagRequired(utils.FlagKeyName)
}

func runRegenerateCommand(cmd *cobra.Command) error {
	if strings.TrimSpace(regenerateAPIID) == "" {
		return fmt.Errorf("--%s is required", utils.FlagID)
	}
	if strings.TrimSpace(regenerateKeyName) == "" {
		return fmt.Errorf("--%s is required", utils.FlagKeyName)
	}

	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf(utils.GatewayAPIKeyRegeneratePath, url.PathEscape(regenerateAPIID), url.PathEscape(regenerateKeyName))
	resp, err := client.Post(endpoint, bytes.NewReader([]byte("{}")))
	if err != nil {
		return fmt.Errorf("failed to regenerate API key: %w", err)
	}

	fmt.Println("API key regenerated successfully.")
	return gateway.PrintJSONResponse(resp)
}
