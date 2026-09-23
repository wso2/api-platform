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
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/cmd/gateway/apikeycmd"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
)

var (
	revokeAPIID   string
	revokeKeyName string
)

var revokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: fmt.Sprintf("Revoke an API key for a %s", apiKeyConfig.KindLabel),
	Long:  "Invalidates an API key so it can no longer be used for authentication.",
	Example: fmt.Sprintf("# Revoke an API key\nap gateway %s api-key revoke --id %s --key-name my-production-key",
		apiKeyConfig.KindPathSegment, apiKeyConfig.ExampleAPIID),
	Run: func(cmd *cobra.Command, args []string) {
		if err := apikeycmd.RunRevoke(cmd, apiKeyConfig, revokeAPIID, revokeKeyName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	gateway.AddSelectionFlags(revokeCmd)
	utils.AddStringFlag(revokeCmd, utils.FlagID, &revokeAPIID, "", fmt.Sprintf("%s ID (required)", apiKeyConfig.KindLabel))
	utils.AddStringFlag(revokeCmd, utils.FlagKeyName, &revokeKeyName, "", "Name of the API key to revoke (required)")
	revokeCmd.MarkFlagRequired(utils.FlagID)
	revokeCmd.MarkFlagRequired(utils.FlagKeyName)
}
