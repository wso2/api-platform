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

package restapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	internaldevportal "github.com/wso2/api-platform/cli/internal/devportal"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	ListCmdLiteral = "list"
	ListCmdExample = `# List all APIs in an organization using the active devportal
ap devportal rest-api list --org org_1

# List all APIs using a specific devportal
ap devportal rest-api list --org org_1 --display-name my-portal --platform eu`
)

var (
	listOrgID    string
	listName     string
	listPlatform string
	listInsecure bool
)

type apiListRow struct {
	APIID     string `json:"apiID"`
	APIHandle string `json:"apiHandle"`
	APIInfo   struct {
		APIName    string `json:"apiName"`
		APIVersion string `json:"apiVersion"`
	} `json:"apiInfo"`
}

var listCmd = &cobra.Command{
	Use:     ListCmdLiteral,
	Short:   "List all APIs in a DevPortal organization",
	Long:    "Retrieves all API artifacts for a given DevPortal organization.",
	Example: ListCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runListCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(listCmd, utils.FlagOrgID, &listOrgID, "", "Organization ID")
	utils.AddStringFlag(listCmd, utils.FlagName, &listName, "", "DevPortal display name")
	utils.AddStringFlag(listCmd, utils.FlagPlatform, &listPlatform, "", "Platform name")
	listCmd.Flags().BoolVar(&listInsecure, "insecure", false, "Skip TLS certificate verification")
	_ = listCmd.MarkFlagRequired(utils.FlagOrgID)
}

func runListCommand() error {
	orgID := strings.TrimSpace(listOrgID)
	if orgID == "" {
		return fmt.Errorf("organization ID is required")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, _, err := internaldevportal.ResolveDevPortal(cfg, listName, listPlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, listInsecure)
	path := internaldevportal.OrgScopedPath(orgID, "apis?tags=default")
	resp, err := client.Get(path)
	if err != nil {
		return internaldevportal.WrapRequestError("list api artifacts", err, listInsecure)
	}

	return printAPIListResponse(resp)
}

func printAPIListResponse(resp *http.Response) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	apiRows, err := extractAPIListRows(body)
	if err != nil {
		return fmt.Errorf("failed to parse API list response: %w", err)
	}

	if len(apiRows) == 0 {
		fmt.Println("No APIs found in the organization.")
		return nil
	}

	headers := []string{"API_ID", "API_HANDLE", "API_NAME", "API_VERSION"}
	rows := make([][]string, 0, len(apiRows))
	for _, api := range apiRows {
		rows = append(rows, []string{
			api.APIID,
			api.APIHandle,
			api.APIInfo.APIName,
			api.APIInfo.APIVersion,
		})
	}

	utils.PrintTable(headers, rows)
	return nil
}

func extractAPIListRows(body []byte) ([]apiListRow, error) {
	var directRows []apiListRow
	if err := json.Unmarshal(body, &directRows); err == nil {
		return directRows, nil
	}

	var wrapped struct {
		APIs  []apiListRow `json:"apis"`
		Items []apiListRow `json:"items"`
		Data  []apiListRow `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return nil, err
	}

	switch {
	case len(wrapped.APIs) > 0:
		return wrapped.APIs, nil
	case len(wrapped.Items) > 0:
		return wrapped.Items, nil
	case len(wrapped.Data) > 0:
		return wrapped.Data, nil
	default:
		return []apiListRow{}, nil
	}
}
