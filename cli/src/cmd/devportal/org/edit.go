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

package org

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
	EditCmdLiteral = "edit"
	EditCmdExample = `# Update an organization from a JSON file
ap devportal org edit --org org_1 -f organization.json

# Update an organization using a specific devportal
ap devportal org edit --org org_1 -f organization.json --display-name my-portal --platform eu`
)

var (
	editOrgID    string
	editFilePath string
	editName     string
	editPlatform string
	editInsecure bool
)

var editCmd = &cobra.Command{
	Use:     EditCmdLiteral,
	Short:   "Update a DevPortal organization",
	Long:    "Updates an organization in the selected DevPortal using a JSON request body from a file.",
	Example: EditCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runEditCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(editCmd, utils.FlagOrgID, &editOrgID, "", "Organization ID")
	utils.AddStringFlag(editCmd, utils.FlagFile, &editFilePath, "", "Path to the organization JSON file")
	utils.AddStringFlag(editCmd, utils.FlagName, &editName, "", "DevPortal display name")
	utils.AddStringFlag(editCmd, utils.FlagPlatform, &editPlatform, "", "Platform name")
	editCmd.Flags().BoolVar(&editInsecure, "insecure", false, "Skip TLS certificate verification")
	_ = editCmd.MarkFlagRequired(utils.FlagOrgID)
	_ = editCmd.MarkFlagRequired(utils.FlagFile)
}

func runEditCommand() error {
	orgID := strings.TrimSpace(editOrgID)
	if orgID == "" {
		return fmt.Errorf("organization ID is required")
	}

	payload, err := internaldevportal.ReadJSONFile(editFilePath)
	if err != nil {
		return err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, editName, editPlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, editInsecure)
	path := fmt.Sprintf("/organizations/%s", url.PathEscape(orgID))
	resp, err := client.PutJSON(path, payload)
	if err != nil {
		return internaldevportal.WrapRequestError("update organization", err, editInsecure)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return utils.FormatHTTPError("update organization", resp, "DevPortal")
	}

	fmt.Printf("Organization updated using devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}
