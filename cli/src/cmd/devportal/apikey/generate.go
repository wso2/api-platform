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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	internaldevportal "github.com/wso2/api-platform/cli/internal/devportal"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	GenerateCmdLiteral = "generate"
	GenerateCmdExample = `# Generate an API key
ap devportal api-key generate --org org_1 --api-id api_1 --name weather_prod_key

# Generate an API key with an expiry
ap devportal api-key generate --org org_1 --api-id api_1 --name weather_prod_key --expires-at 2026-12-31T23:59:59Z

# Generate an API key interactively (prompts for any missing value)
ap devportal api-key generate`
)

// keyNamePattern mirrors the ApiKeyRequest.name constraint in
// docs/devportal-openapi-spec-v1.yaml: lowercase, may contain numbers,
// underscores, and hyphens.
var keyNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,127}$`)

var (
	generateOrgID         string
	generateAPIID         string
	generateName          string
	generateExpiresAt     string
	generateDisplayName   string
	generatePlatform      string
	generateInsecure      bool
	generateNoInteractive bool
)

// apiKeyRequest is the ApiKeyBody request payload defined in
// docs/devportal-openapi-spec-v1.yaml.
type apiKeyRequest struct {
	APIID     string `json:"apiId"`
	Name      string `json:"name"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

var generateCmd = &cobra.Command{
	Use:     GenerateCmdLiteral,
	Short:   "Generate a DevPortal API key",
	Long: "Generates an API key for an API in the selected DevPortal. The plaintext secret is " +
		"returned once in the response and never persisted. Run without the required flags to be " +
		"prompted interactively.",
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
	utils.AddStringFlag(generateCmd, utils.FlagAPIID, &generateAPIID, "", "Developer Portal API ID")
	utils.AddStringFlag(generateCmd, utils.FlagPropertyName, &generateName, "", "API key name (lowercase letters, numbers, '_' and '-'; matches ^[a-z0-9][a-z0-9_-]{0,127}$)")
	utils.AddStringFlag(generateCmd, utils.FlagExpiresAt, &generateExpiresAt, "", "Optional expiry: ISO-8601 datetime with timezone (e.g. 2026-12-31T23:59:59Z), epoch seconds, or epoch milliseconds")
	utils.AddStringFlag(generateCmd, utils.FlagName, &generateDisplayName, "", "DevPortal display name")
	utils.AddStringFlag(generateCmd, utils.FlagPlatform, &generatePlatform, "", "Platform name")
	utils.AddBoolFlag(generateCmd, utils.FlagInsecure, &generateInsecure, false, "Skip TLS certificate verification")
	utils.AddBoolFlag(generateCmd, utils.FlagNoInteractive, &generateNoInteractive, false, "Skip interactive prompts; fail if a required flag is missing")
}

func runGenerateCommand() error {
	if !generateNoInteractive {
		if err := promptForGenerateInputs(); err != nil {
			return err
		}
	}

	payload, err := buildAPIKeyPayload(generateAPIID, generateName, generateExpiresAt)
	if err != nil {
		return err
	}

	orgID := strings.TrimSpace(generateOrgID)
	if orgID == "" {
		return fmt.Errorf("organization ID is required (provide --%s or use interactive mode)", utils.FlagOrgID)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, generateDisplayName, generatePlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, generateInsecure)
	path := internaldevportal.OrgScopedPath(orgID, "api-keys/generate")
	resp, err := client.PostJSON(path, payload)
	if err != nil {
		return internaldevportal.WrapRequestError("generate API key", err, generateInsecure)
	}
	if !isHTTPSuccess(resp.StatusCode) {
		return utils.FormatHTTPError("generate API key", resp, "DevPortal")
	}

	fmt.Printf("API key generated using devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}

// promptForGenerateInputs interactively fills any required value that was not
// supplied through a flag.
func promptForGenerateInputs() error {
	var err error
	if strings.TrimSpace(generateOrgID) == "" {
		if generateOrgID, err = utils.PromptInput("Enter organization ID: "); err != nil {
			return err
		}
	}
	if strings.TrimSpace(generateAPIID) == "" {
		if generateAPIID, err = utils.PromptInput("Enter Developer Portal API ID: "); err != nil {
			return err
		}
	}
	if strings.TrimSpace(generateName) == "" {
		if generateName, err = utils.PromptInput("Enter API key name (e.g. weather_prod_key): "); err != nil {
			return err
		}
	}
	if strings.TrimSpace(generateExpiresAt) == "" {
		if generateExpiresAt, err = utils.PromptInput("Enter expiry (optional, e.g. 2026-12-31T23:59:59Z; leave empty for none): "); err != nil {
			return err
		}
	}
	return nil
}

func buildAPIKeyPayload(apiID, name, expiresAt string) ([]byte, error) {
	request := apiKeyRequest{
		APIID: strings.TrimSpace(apiID),
		Name:  strings.TrimSpace(name),
	}
	if request.APIID == "" {
		return nil, fmt.Errorf("api ID is required")
	}
	if request.Name == "" {
		return nil, fmt.Errorf("api key name is required")
	}
	if !keyNamePattern.MatchString(request.Name) {
		return nil, fmt.Errorf("api key name %q is invalid: must match ^[a-z0-9][a-z0-9_-]{0,127}$ (lowercase letters, numbers, '_' and '-')", request.Name)
	}

	if expires := strings.TrimSpace(expiresAt); expires != "" {
		request.ExpiresAt = expires
	}

	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to build API key payload: %w", err)
	}
	return data, nil
}

func isHTTPSuccess(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}
