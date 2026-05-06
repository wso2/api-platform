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
	"encoding/json"
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
	GenerateCmdLiteral = "generate"
	GenerateCmdExample = `# Generate a platform API key
ap devportal sub-api-key generate --org org_1 --api-id api_1 --key-name mobile-app-key

# Generate an expiring platform API key
ap devportal sub-api-key generate --org org_1 --api-id api_1 --key-name mobile-app-key --expires-at 2026-12-31T23:59:59Z`
)

var (
	generateOrgID     string
	generateAPIID     string
	generateKeyName   string
	generateExpiresAt string
	generateName      string
	generatePlatform  string
	generateInsecure  bool
)

type platformAPIKeyRequest struct {
	APIID     string `json:"apiId"`
	Name      string `json:"name"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

var generateCmd = &cobra.Command{
	Use:     GenerateCmdLiteral,
	Short:   "Generate a DevPortal platform API key",
	Long:    "Generates a platform API key in the selected DevPortal.",
	Example: GenerateCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGenerateCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(generateCmd, utils.FlagOrgID, &generateOrgID, "", "Organization ID")
	utils.AddStringFlag(generateCmd, utils.FlagAPIID, &generateAPIID, "", "API ID")
	utils.AddStringFlag(generateCmd, utils.FlagKeyName, &generateKeyName, "", "Platform API key name")
	utils.AddStringFlag(generateCmd, utils.FlagExpiresAt, &generateExpiresAt, "", "Optional ISO-8601 expiration time (YYYY-MM-DDTHH:MM:SSZ)")
	utils.AddStringFlag(generateCmd, utils.FlagName, &generateName, "", "DevPortal display name")
	utils.AddStringFlag(generateCmd, utils.FlagPlatform, &generatePlatform, "", "Platform name")
	utils.AddBoolFlag(generateCmd, utils.FlagInsecure, &generateInsecure, false, "Skip TLS certificate verification")
	_ = generateCmd.MarkFlagRequired(utils.FlagOrgID)
	_ = generateCmd.MarkFlagRequired(utils.FlagAPIID)
	_ = generateCmd.MarkFlagRequired(utils.FlagKeyName)
}

func runGenerateCommand() error {
	payload, err := buildPlatformAPIKeyPayload(generateAPIID, generateKeyName, generateExpiresAt)
	if err != nil {
		return err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, generateName, generatePlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, generateInsecure)
	path := fmt.Sprintf("/devportal/organizations/%s/platform-api-keys/generate", url.PathEscape(strings.TrimSpace(generateOrgID)))
	resp, err := client.PostJSON(path, payload)
	if err != nil {
		return internaldevportal.WrapRequestError("generate platform API key", err, generateInsecure)
	}
	if !isHTTPSuccess(resp.StatusCode) {
		return utils.FormatHTTPError("generate platform API key", resp, "DevPortal")
	}

	fmt.Printf("Platform API key generated using devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}

func buildPlatformAPIKeyPayload(apiID, keyName, expiresAt string) ([]byte, error) {
	request := platformAPIKeyRequest{
		APIID: strings.TrimSpace(apiID),
		Name:  strings.TrimSpace(keyName),
	}
	if request.APIID == "" {
		return nil, fmt.Errorf("api ID is required")
	}
	if request.Name == "" {
		return nil, fmt.Errorf("key name is required")
	}

	expires := strings.TrimSpace(expiresAt)
	if expires != "" {
		request.ExpiresAt = expires
	}

	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to build platform API key payload: %w", err)
	}
	return data, nil
}

func isHTTPSuccess(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}
