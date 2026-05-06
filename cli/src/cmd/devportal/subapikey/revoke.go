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

package subapikey

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
	RevokeCmdLiteral = "revoke"
	RevokeCmdExample = `# Revoke a platform API key
ap devportal sub-api-key revoke --org org_1 --api-key-id key_1 --api-id api_1

# Revoke using a specific devportal
ap devportal sub-api-key revoke --org org_1 --api-key-id key_1 --api-id api_1 --display-name my-portal --platform eu`
)

var (
	revokeOrgID    string
	revokeAPIKeyID string
	revokeAPIID    string
	revokeName     string
	revokePlatform string
	revokeInsecure bool
)

var revokeCmd = &cobra.Command{
	Use:     RevokeCmdLiteral,
	Short:   "Revoke a DevPortal platform API key",
	Long:    "Revokes a platform API key in the selected DevPortal.",
	Example: RevokeCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runRevokeCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(revokeCmd, utils.FlagOrgID, &revokeOrgID, "", "Organization ID")
	utils.AddStringFlag(revokeCmd, utils.FlagAPIKeyID, &revokeAPIKeyID, "", "Platform API key ID")
	utils.AddStringFlag(revokeCmd, utils.FlagAPIID, &revokeAPIID, "", "API ID")
	utils.AddStringFlag(revokeCmd, utils.FlagName, &revokeName, "", "DevPortal display name")
	utils.AddStringFlag(revokeCmd, utils.FlagPlatform, &revokePlatform, "", "Platform name")
	utils.AddBoolFlag(revokeCmd, utils.FlagInsecure, &revokeInsecure, false, "Skip TLS certificate verification")
	_ = revokeCmd.MarkFlagRequired(utils.FlagOrgID)
	_ = revokeCmd.MarkFlagRequired(utils.FlagAPIKeyID)
	_ = revokeCmd.MarkFlagRequired(utils.FlagAPIID)
}

func runRevokeCommand() error {
	orgID := strings.TrimSpace(revokeOrgID)
	if orgID == "" {
		return fmt.Errorf("organization ID is required")
	}

	apiKeyID := strings.TrimSpace(revokeAPIKeyID)
	if apiKeyID == "" {
		return fmt.Errorf("api key ID is required")
	}

	apiID := strings.TrimSpace(revokeAPIID)
	if apiID == "" {
		return fmt.Errorf("api ID is required")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, revokeName, revokePlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, revokeInsecure)
	baseURL := strings.TrimSuffix(devPortal.URL, "/")
	path := fmt.Sprintf("/devportal/organizations/%s/platform-api-keys/%s/revoke?apiId=%s", url.PathEscape(orgID), url.PathEscape(apiKeyID), url.QueryEscape(apiID))
	req, err := http.NewRequest(http.MethodPost, baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return internaldevportal.WrapRequestError("revoke platform API key", err, revokeInsecure)
	}
	if !isHTTPSuccess(resp.StatusCode) {
		return utils.FormatHTTPError("revoke platform API key", resp, "DevPortal")
	}

	fmt.Printf("Platform API key revoked using devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}
