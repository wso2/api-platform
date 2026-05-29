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

package subscription

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
	EditCmdLiteral = "edit"
	EditCmdExample = `# Update a platform subscription status
ap devportal subscription edit --org org_1 --sub-id sub_1 --status ACTIVE

# Update using a specific devportal
ap devportal subscription edit --org org_1 --sub-id sub_1 --status ACTIVE --display-name my-portal --platform eu`
)

var (
	editOrgID        string
	editSubscription string
	editStatus       string
	editName         string
	editPlatform     string
	editInsecure     bool
)

var editCmd = &cobra.Command{
	Use:     EditCmdLiteral,
	Short:   "Update a DevPortal platform subscription",
	Long:    "Updates a platform subscription in the selected DevPortal using request flags or a JSON request body from a file.",
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
	utils.AddStringFlag(editCmd, utils.FlagSubID, &editSubscription, "", "Subscription ID")
	utils.AddStringFlag(editCmd, utils.FlagStatus, &editStatus, "", "Subscription status")
	utils.AddStringFlag(editCmd, utils.FlagName, &editName, "", "DevPortal display name")
	utils.AddStringFlag(editCmd, utils.FlagPlatform, &editPlatform, "", "Platform name")
	editCmd.Flags().BoolVar(&editInsecure, utils.FlagInsecure, false, "Skip TLS certificate verification")
	_ = editCmd.MarkFlagRequired(utils.FlagOrgID)
	_ = editCmd.MarkFlagRequired(utils.FlagSubID)
	_ = editCmd.MarkFlagRequired(utils.FlagStatus)
}

func runEditCommand() error {
	subscriptionID := strings.TrimSpace(editSubscription)
	if subscriptionID == "" {
		return fmt.Errorf("subscription ID is required")
	}

	payload, err := buildEditPayload()
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
	path := fmt.Sprintf("/devportal/organizations/%s/api-platform-subscriptions/%s", url.PathEscape(editOrgID), url.PathEscape(subscriptionID))
	resp, err := client.PutJSON(path, payload)
	if err != nil {
		return internaldevportal.WrapRequestError("update platform subscription", err, editInsecure)
	}

	if resp.StatusCode != http.StatusOK {
		return utils.FormatHTTPError("update platform subscription", resp, "DevPortal")
	}
	fmt.Printf("Platform subscription updated using devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}

func buildEditPayload() ([]byte, error) {
	status := strings.TrimSpace(editStatus)

	if status == "" {
		return nil, fmt.Errorf("status is required")
	}

	payload := map[string]string{
		"status": status,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to build subscription payload: %w", err)
	}

	return data, nil
}
