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
	regenerateAPIID   string
	regenerateKeyName string
)

var regenerateCmd = &cobra.Command{
	Use:   "regenerate",
	Short: fmt.Sprintf("Regenerate an API key for a %s", apiKeyConfig.KindLabel),
	Long:  "Creates a new API key value replacing the previous one. The new plaintext key is returned once in the response.",
	Example: fmt.Sprintf("# Regenerate an API key, replacing its previous value\nap gateway %s api-key regenerate --id %s --key-name my-production-key",
		apiKeyConfig.KindPathSegment, apiKeyConfig.ExampleAPIID),
	Run: func(cmd *cobra.Command, args []string) {
		if err := apikeycmd.RunRegenerate(cmd, apiKeyConfig, regenerateAPIID, regenerateKeyName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	gateway.AddSelectionFlags(regenerateCmd)
	utils.AddStringFlag(regenerateCmd, utils.FlagID, &regenerateAPIID, "", fmt.Sprintf("%s ID (required)", apiKeyConfig.KindLabel))
	utils.AddStringFlag(regenerateCmd, utils.FlagKeyName, &regenerateKeyName, "", "Name of the API key to regenerate (required)")
	regenerateCmd.MarkFlagRequired(utils.FlagID)
	regenerateCmd.MarkFlagRequired(utils.FlagKeyName)
}
