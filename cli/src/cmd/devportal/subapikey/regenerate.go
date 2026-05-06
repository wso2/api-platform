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
	RegenerateCmdExample = `# Regenerate a platform API key
ap devportal sub-api-key regenerate --org org_1 --api-key-id key_1 --api-id api_1 --key-name mobile-app-key

# Regenerate with a new expiration
ap devportal sub-api-key regenerate --org org_1 --api-key-id key_1 --api-id api_1 --key-name mobile-app-key --expires-at 2026-12-31T23:59:59Z`
)

var (
	regenerateOrgID     string
	regenerateAPIKeyID  string
	regenerateAPIID     string
	regenerateKeyName   string
	regenerateExpiresAt string
	regenerateName      string
	regeneratePlatform  string
	regenerateInsecure  bool
)

var regenerateCmd = &cobra.Command{
	Use:     RegenerateCmdLiteral,
	Short:   "Regenerate a DevPortal platform API key",
	Long:    "Regenerates a platform API key in the selected DevPortal.",
	Example: RegenerateCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runRegenerateCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(regenerateCmd, utils.FlagOrgID, &regenerateOrgID, "", "Organization ID")
	utils.AddStringFlag(regenerateCmd, utils.FlagAPIKeyID, &regenerateAPIKeyID, "", "Platform API key ID")
	utils.AddStringFlag(regenerateCmd, utils.FlagAPIID, &regenerateAPIID, "", "API ID")
	utils.AddStringFlag(regenerateCmd, utils.FlagKeyName, &regenerateKeyName, "", "Platform API key name")
	utils.AddStringFlag(regenerateCmd, utils.FlagExpiresAt, &regenerateExpiresAt, "", "Optional ISO-8601 expiration time (YYYY-MM-DDTHH:MM:SSZ)")
	utils.AddStringFlag(regenerateCmd, utils.FlagName, &regenerateName, "", "DevPortal display name")
	utils.AddStringFlag(regenerateCmd, utils.FlagPlatform, &regeneratePlatform, "", "Platform name")
	utils.AddBoolFlag(regenerateCmd, utils.FlagInsecure, &regenerateInsecure, false, "Skip TLS certificate verification")
	_ = regenerateCmd.MarkFlagRequired(utils.FlagOrgID)
	_ = regenerateCmd.MarkFlagRequired(utils.FlagAPIKeyID)
	_ = regenerateCmd.MarkFlagRequired(utils.FlagAPIID)
	_ = regenerateCmd.MarkFlagRequired(utils.FlagKeyName)
}

func runRegenerateCommand() error {
	apiKeyID := strings.TrimSpace(regenerateAPIKeyID)
	if apiKeyID == "" {
		return fmt.Errorf("api key ID is required")
	}

	payload, err := buildPlatformAPIKeyPayload(regenerateAPIID, regenerateKeyName, regenerateExpiresAt)
	if err != nil {
		return err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, regenerateName, regeneratePlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, regenerateInsecure)
	path := fmt.Sprintf("/devportal/organizations/%s/platform-api-keys/%s/regenerate", url.PathEscape(strings.TrimSpace(regenerateOrgID)), url.PathEscape(apiKeyID))
	resp, err := client.PostJSON(path, payload)
	if err != nil {
		return internaldevportal.WrapRequestError("regenerate platform API key", err, regenerateInsecure)
	}
	if !isHTTPSuccess(resp.StatusCode) {
		return utils.FormatHTTPError("regenerate platform API key", resp, "DevPortal")
	}

	fmt.Printf("Platform API key regenerated using devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}
