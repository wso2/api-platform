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

package application

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
	CreateCmdLiteral = "create"
	CreateCmdExample = `# Create an application
ap devportal application create --org org_1 --name "Weather App" --type WEB

# Create an application with a description
ap devportal application create --org org_1 --name "Weather App" --type WEB --description "Calls the Weather APIs"

# Create using a specific devportal
ap devportal application create --org org_1 --name "Weather App" --type WEB --display-name my-portal --platform eu`
)

var (
	createOrgID       string
	createAppName     string
	createType        string
	createDescription string
	createName        string
	createPlatform    string
	createInsecure    bool
)

// applicationRequest is the ApplicationRequest payload defined in
// docs/devportal-openapi-spec-v1.yaml.
type applicationRequest struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

var createCmd = &cobra.Command{
	Use:     CreateCmdLiteral,
	Short:   "Create a DevPortal application",
	Long:    "Creates an application in the selected DevPortal. Only --name and --type are required.",
	Example: CreateCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runCreateCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(createCmd, utils.FlagOrgID, &createOrgID, "", "Organization ID")
	utils.AddStringFlag(createCmd, utils.FlagAppName, &createAppName, "", "Application name")
	utils.AddStringFlag(createCmd, utils.FlagType, &createType, "", "Application type (e.g. WEB)")
	utils.AddStringFlag(createCmd, utils.FlagDescription, &createDescription, "", "Application description (optional)")
	utils.AddStringFlag(createCmd, utils.FlagName, &createName, "", "DevPortal display name")
	utils.AddStringFlag(createCmd, utils.FlagPlatform, &createPlatform, "", "Platform name")
	utils.AddBoolFlag(createCmd, utils.FlagInsecure, &createInsecure, false, "Skip TLS certificate verification")
	_ = createCmd.MarkFlagRequired(utils.FlagOrgID)
	_ = createCmd.MarkFlagRequired(utils.FlagAppName)
	_ = createCmd.MarkFlagRequired(utils.FlagType)
}

func runCreateCommand() error {
	orgID := strings.TrimSpace(createOrgID)
	if orgID == "" {
		return fmt.Errorf("organization ID is required")
	}

	payload, err := buildApplicationPayload(createAppName, createType, createDescription)
	if err != nil {
		return err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, createName, createPlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, createInsecure)
	path := fmt.Sprintf("/devportal/organizations/%s/applications", url.PathEscape(orgID))
	resp, err := client.PostJSON(path, payload)
	if err != nil {
		return internaldevportal.WrapRequestError("create application", err, createInsecure)
	}
	if resp.StatusCode != http.StatusCreated {
		return utils.FormatHTTPError("create application", resp, "DevPortal")
	}

	fmt.Printf("Application created using devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}

// buildApplicationPayload builds the ApplicationRequest JSON body. Name and
// type are required; description is optional and omitted when empty.
func buildApplicationPayload(name, appType, description string) ([]byte, error) {
	request := applicationRequest{
		Name: strings.TrimSpace(name),
		Type: strings.TrimSpace(appType),
	}
	if request.Name == "" {
		return nil, fmt.Errorf("application name is required")
	}
	if request.Type == "" {
		return nil, fmt.Errorf("application type is required")
	}
	if desc := strings.TrimSpace(description); desc != "" {
		request.Description = desc
	}

	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to build application payload: %w", err)
	}
	return data, nil
}
