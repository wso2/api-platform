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

package subplan

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
	DeleteCmdExample = `# Delete a subscription plan by policy ID
ap devportal sub-plan delete --policy-id plan_1

# Delete using a specific devportal
ap devportal sub-plan delete --policy-id plan_1 --display-name my-portal --platform eu`
)

var (
	deletePolicyID string
	deleteName     string
	deletePlatform string
	deleteInsecure bool
)

var deleteCmd = &cobra.Command{
	Use:     DeleteCmdLiteral,
	Short:   "Delete a DevPortal subscription plan",
	Long:    "Deletes a subscription plan from the WSO2 API Platform DevPortal by its policy ID.",
	Example: DeleteCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDeleteCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(deleteCmd, utils.FlagPolicyId, &deletePolicyID, "", "Subscription plan policy ID")
	utils.AddStringFlag(deleteCmd, utils.FlagName, &deleteName, "", "DevPortal display name")
	utils.AddStringFlag(deleteCmd, utils.FlagPlatform, &deletePlatform, "", "Platform name")
	utils.AddBoolFlag(deleteCmd, utils.FlagInsecure, &deleteInsecure, false, "Skip TLS certificate verification")
	_ = deleteCmd.MarkFlagRequired(utils.FlagPolicyId)
}

func runDeleteCommand() error {
	policyID := strings.TrimSpace(deletePolicyID)
	if policyID == "" {
		return fmt.Errorf("policy ID is required")
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
	path := internaldevportal.ResourcePath("subscription-plans/" + url.PathEscape(policyID))
	resp, err := client.Delete(path)
	if err != nil {
		return internaldevportal.WrapRequestError("delete subscription plan", err, deleteInsecure)
	}
	// The DevPortal returns 204 No Content on a successful delete.
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return utils.FormatHTTPError("delete subscription plan", resp, "DevPortal")
	}

	fmt.Printf("Subscription plan deleted from devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}
