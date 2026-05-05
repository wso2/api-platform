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
package subpolicy

import (
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	internaldevportal "github.com/wso2/api-platform/cli/internal/devportal"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	DeleteCmdLiteral = "delete"
	DeleteCmdExample = `# Delete a subscription policy from the devportal
ap devportal sub-policy delete --policy-id gold-policy-id

# Delete a subscription policy by defining the devportal and platform
ap devportal sub-policy delete --policy-id gold-policy-id --devportal my-devportal --platform my-platform`
)

var (
	deleteDevportalName string
	deletePlatform      string
	deleteOrgID         string
	deleteInsecure      bool
	deletePolicyID      string
)

var deleteCmd = &cobra.Command{
	Use:     DeleteCmdLiteral,
	Short:   "Delete a subscription policy from the devportal",
	Long:    "This command allows you to delete a subscription policy from the WSO2 API Platform DevPortal.",
	Example: DeleteCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDeleteCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(deleteCmd, utils.FlagPolicyId, &deletePolicyID, "", "ID of the policy to delete")
	utils.AddStringFlag(deleteCmd, utils.FlagName, &deleteDevportalName, "", "Name of the devportal")
	utils.AddStringFlag(deleteCmd, utils.FlagPlatform, &deletePlatform, "", "Platform of the devportal")
	utils.AddStringFlag(deleteCmd, utils.FlagOrgID, &deleteOrgID, "", "Organization ID")
	utils.AddBoolFlag(deleteCmd, utils.FlagInsecure, &deleteInsecure, false, "Allow insecure connections")
}

func runDeleteCommand() error {
	// Validate the policy ID
	if deletePolicyID == "" {
		return fmt.Errorf("policy ID is required")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, deleteDevportalName, deletePlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, deleteInsecure)

	path := fmt.Sprintf("/devportal/organizations/%s/subscription-policies/%s", url.PathEscape(deleteOrgID), url.PathEscape(deletePolicyID))
	resp, err := client.Delete(path)
	if err != nil {
		return internaldevportal.WrapRequestError("delete subscription policy", err, deleteInsecure)
	}
	if resp.StatusCode != http.StatusOK {
		return utils.FormatHTTPError("delete subscription policy", resp, "DevPortal")
	}

	fmt.Printf("Subscription policy deleted successfully using devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)

	return internaldevportal.PrintJSONResponse(resp)
}
