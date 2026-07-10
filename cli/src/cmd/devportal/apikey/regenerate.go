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
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	internaldevportal "github.com/wso2/api-platform/cli/internal/devportal"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	RegenerateCmdLiteral = "regenerate"
	RegenerateCmdExample = `# Regenerate an API key
ap devportal api-key regenerate --api-key-id key_1

# Regenerate using a specific devportal
ap devportal api-key regenerate --api-key-id key_1 --display-name my-portal --platform eu`
)

var (
	regenerateAPIKeyID    string
	regenerateDisplayName string
	regeneratePlatform    string
	regenerateInsecure    bool
)

var regenerateCmd = &cobra.Command{
	Use:   RegenerateCmdLiteral,
	Short: "Regenerate a DevPortal API key",
	Long: "Regenerates the secret for an existing API key. The old secret is invalidated at connected " +
		"gateways and the new plaintext secret is returned once in the response.",
	Example: RegenerateCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runRegenerateCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(regenerateCmd, utils.FlagAPIKeyID, &regenerateAPIKeyID, "", "API key ID")
	utils.AddStringFlag(regenerateCmd, utils.FlagName, &regenerateDisplayName, "", "DevPortal display name")
	utils.AddStringFlag(regenerateCmd, utils.FlagPlatform, &regeneratePlatform, "", "Platform name")
	utils.AddBoolFlag(regenerateCmd, utils.FlagInsecure, &regenerateInsecure, false, "Skip TLS certificate verification")
	_ = regenerateCmd.MarkFlagRequired(utils.FlagAPIKeyID)
}

func runRegenerateCommand() error {
	apiKeyID := strings.TrimSpace(regenerateAPIKeyID)
	if apiKeyID == "" {
		return fmt.Errorf("api key ID is required")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, regenerateDisplayName, regeneratePlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, regenerateInsecure)
	baseURL := strings.TrimSuffix(devPortal.URL, "/")
	path := internaldevportal.ResourcePath("api-keys/" + url.PathEscape(apiKeyID) + "/regenerate")
	req, err := http.NewRequest(http.MethodPost, baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return internaldevportal.WrapRequestError("regenerate API key", err, regenerateInsecure)
	}
	if !isHTTPSuccess(resp.StatusCode) {
		return utils.FormatHTTPError("regenerate API key", resp, "DevPortal")
	}

	fmt.Printf("API key regenerated using devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}
