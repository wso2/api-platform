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
	DeleteCmdLiteral = "delete"
	DeleteCmdExample = `# Delete a platform subscription
ap devportal subscription delete --org org_1 --sub-id sub_1

# Delete using a specific devportal
ap devportal subscription delete --org org_1 --sub-id sub_1 --display-name my-portal --platform eu`
)

var (
	deleteOrgID        string
	deleteSubscription string
	deleteName         string
	deletePlatform     string
	deleteInsecure     bool
)

var deleteCmd = &cobra.Command{
	Use:     DeleteCmdLiteral,
	Short:   "Delete a DevPortal platform subscription",
	Long:    "Deletes a platform subscription from the selected DevPortal.",
	Example: DeleteCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDeleteCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(deleteCmd, utils.FlagOrgID, &deleteOrgID, "", "Organization ID")
	utils.AddStringFlag(deleteCmd, utils.FlagSubID, &deleteSubscription, "", "Subscription ID")
	utils.AddStringFlag(deleteCmd, utils.FlagName, &deleteName, "", "DevPortal display name")
	utils.AddStringFlag(deleteCmd, utils.FlagPlatform, &deletePlatform, "", "Platform name")
	deleteCmd.Flags().BoolVar(&deleteInsecure, utils.FlagInsecure, false, "Skip TLS certificate verification")
	_ = deleteCmd.MarkFlagRequired(utils.FlagOrgID)
	_ = deleteCmd.MarkFlagRequired(utils.FlagSubID)
}

func runDeleteCommand() error {
	orgID := strings.TrimSpace(deleteOrgID)
	if orgID == "" {
		return fmt.Errorf("organization ID is required")
	}

	subscriptionID := strings.TrimSpace(deleteSubscription)
	if subscriptionID == "" {
		return fmt.Errorf("subscription ID is required")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, deleteName, deletePlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, deleteInsecure)
	path := internaldevportal.OrgScopedPath(orgID, "subscriptions/"+url.PathEscape(subscriptionID))
	resp, err := client.Delete(path)
	if err != nil {
		return internaldevportal.WrapRequestError("delete platform subscription", err, deleteInsecure)
	}
	if resp.StatusCode != http.StatusOK {
		return utils.FormatHTTPError("delete platform subscription", resp, "DevPortal")
	}
	fmt.Printf("Platform subscription deleted from devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}

