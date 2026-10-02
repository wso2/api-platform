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

var listAPIID string

var listCmd = &cobra.Command{
	Use:   "list",
	Short: fmt.Sprintf("List API keys for a %s", apiKeyConfig.KindLabel),
	Long:  fmt.Sprintf("Retrieves and displays all API keys for a %s on the currently active gateway.", apiKeyConfig.KindLabel),
	Example: fmt.Sprintf("# List all API keys for a %s\nap gateway %s api-key list --id %s",
		apiKeyConfig.KindLabel, apiKeyConfig.KindPathSegment, apiKeyConfig.ExampleAPIID),
	Run: func(cmd *cobra.Command, args []string) {
		if err := apikeycmd.RunList(cmd, apiKeyConfig, listAPIID); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	gateway.AddSelectionFlags(listCmd)
	utils.AddStringFlag(listCmd, utils.FlagID, &listAPIID, "", fmt.Sprintf("%s ID (required)", apiKeyConfig.KindLabel))
	listCmd.MarkFlagRequired(utils.FlagID)
}
