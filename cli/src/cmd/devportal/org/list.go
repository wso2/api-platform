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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	internaldevportal "github.com/wso2/api-platform/cli/internal/devportal"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	ListCmdLiteral = "list"
	ListCmdExample = `# List all organizations using the active devportal
ap devportal org list

# List all organizations using a specific devportal
ap devportal org list --display-name my-portal --platform eu`
)

var (
	listName     string
	listPlatform string
	listInsecure bool
)

type organizationListRow struct {
	OrgID                  string `json:"orgId"`
	OrgName                string `json:"orgName"`
	BusinessOwner          string `json:"businessOwner"`
	OrganizationIdentifier string `json:"organizationIdentifier"`
}

var listCmd = &cobra.Command{
	Use:     ListCmdLiteral,
	Short:   "List DevPortal organizations",
	Long:    "Retrieves all organizations from the selected DevPortal.",
	Example: ListCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runListCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(listCmd, utils.FlagName, &listName, "", "DevPortal display name")
	utils.AddStringFlag(listCmd, utils.FlagPlatform, &listPlatform, "", "Platform name")
	listCmd.Flags().BoolVar(&listInsecure, "insecure", false, "Skip TLS certificate verification")
}

func runListCommand() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, listName, listPlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, listInsecure)
	resp, err := client.Get("/devportal/organizations")
	if err != nil {
		return internaldevportal.WrapRequestError("list organizations", err, listInsecure)
	}

	fmt.Printf("Organizations from devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return printOrganizationListResponse(resp)
}

func printOrganizationListResponse(resp *http.Response) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	orgRows, err := extractOrganizationListRows(body)
	if err != nil {
		return fmt.Errorf("failed to parse organization list response: %w", err)
	}

	if len(orgRows) == 0 {
		fmt.Println("No organizations found.")
		return nil
	}

	headers := []string{"ORG_ID", "ORG_NAME", "BUSINESS_OWNER", "ORGANIZATION_IDENTIFIER"}
	rows := make([][]string, 0, len(orgRows))
	for _, org := range orgRows {
		rows = append(rows, []string{
			org.OrgID,
			org.OrgName,
			org.BusinessOwner,
			org.OrganizationIdentifier,
		})
	}

	utils.PrintTable(headers, rows)
	return nil
}

func extractOrganizationListRows(body []byte) ([]organizationListRow, error) {
	var directRows []organizationListRow
	if err := json.Unmarshal(body, &directRows); err == nil {
		return directRows, nil
	}

	var wrapped struct {
		Organizations []organizationListRow `json:"organizations"`
		Items         []organizationListRow `json:"items"`
		Data          []organizationListRow `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return nil, err
	}

	switch {
	case len(wrapped.Organizations) > 0:
		return wrapped.Organizations, nil
	case len(wrapped.Items) > 0:
		return wrapped.Items, nil
	case len(wrapped.Data) > 0:
		return wrapped.Data, nil
	}

	// No supported list field carried entries. Distinguish a recognized-but-empty
	// response (e.g. `{}` or `{"organizations": []}`) from a non-empty object whose
	// shape we don't understand: the latter should surface as a parse error
	// rather than be silently reported as "No organizations found.".
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	recognized := false
	for _, key := range []string{"organizations", "items", "data"} {
		if _, ok := raw[key]; ok {
			recognized = true
			break
		}
	}
	if len(raw) > 0 && !recognized {
		return nil, fmt.Errorf("unsupported response shape: %s", string(body))
	}

	return []organizationListRow{}, nil
}
