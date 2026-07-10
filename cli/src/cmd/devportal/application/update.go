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
	UpdateCmdLiteral = "update"
	UpdateCmdExample = `# Update an application
ap devportal application update --app-id app_1 --name "Weather App" --type WEB

# Update with a description
ap devportal application update --app-id app_1 --name "Weather App" --type WEB --description "Calls the Weather APIs"

# Update using a specific devportal
ap devportal application update --app-id app_1 --name "Weather App" --type WEB --display-name my-portal --platform eu`
)

var (
	updateAppID       string
	updateAppName     string
	updateType        string
	updateDescription string
	updateName        string
	updatePlatform    string
	updateInsecure    bool
)

var updateCmd = &cobra.Command{
	Use:     UpdateCmdLiteral,
	Aliases: []string{"edit"},
	Short:   "Update a DevPortal application",
	Long:    "Updates an existing application in the selected DevPortal. Only --name and --type are required in the body.",
	Example: UpdateCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUpdateCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(updateCmd, utils.FlagAppID, &updateAppID, "", "Application ID")
	utils.AddStringFlag(updateCmd, utils.FlagPropertyName, &updateAppName, "", "Application name")
	utils.AddStringFlag(updateCmd, utils.FlagType, &updateType, "", "Application type (e.g. WEB)")
	utils.AddStringFlag(updateCmd, utils.FlagDescription, &updateDescription, "", "Application description (optional)")
	utils.AddStringFlag(updateCmd, utils.FlagName, &updateName, "", "DevPortal display name")
	utils.AddStringFlag(updateCmd, utils.FlagPlatform, &updatePlatform, "", "Platform name")
	utils.AddBoolFlag(updateCmd, utils.FlagInsecure, &updateInsecure, false, "Skip TLS certificate verification")
	_ = updateCmd.MarkFlagRequired(utils.FlagAppID)
	_ = updateCmd.MarkFlagRequired(utils.FlagPropertyName)
	_ = updateCmd.MarkFlagRequired(utils.FlagType)
}

func runUpdateCommand() error {
	appID := strings.TrimSpace(updateAppID)
	if appID == "" {
		return fmt.Errorf("application ID is required")
	}

	payload, err := buildApplicationPayload(updateAppName, updateType, updateDescription)
	if err != nil {
		return err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, updateName, updatePlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, updateInsecure)
	path := internaldevportal.ResourcePath("applications/" + url.PathEscape(appID))
	resp, err := client.PutJSON(path, payload)
	if err != nil {
		return internaldevportal.WrapRequestError("update application", err, updateInsecure)
	}
	if resp.StatusCode != http.StatusOK {
		return utils.FormatHTTPError("update application", resp, "DevPortal")
	}

	fmt.Printf("Application updated using devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}
