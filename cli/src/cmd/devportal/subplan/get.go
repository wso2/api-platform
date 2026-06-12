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
	GetCmdLiteral = "get"
	GetCmdExample = `# Get a subscription plan by policy ID
ap devportal sub-plan get --policy-id plan_1 --org org_1

# Get using a specific devportal
ap devportal sub-plan get --policy-id plan_1 --org org_1 --display-name my-portal --platform eu`
)

var (
	getPolicyID string
	getOrgID    string
	getName     string
	getPlatform string
	getInsecure bool
)

var getCmd = &cobra.Command{
	Use:     GetCmdLiteral,
	Short:   "Get a DevPortal subscription plan",
	Long:    "Retrieves a subscription plan from the WSO2 API Platform DevPortal by its policy ID.",
	Example: GetCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGetCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(getCmd, utils.FlagPolicyId, &getPolicyID, "", "Subscription plan policy ID")
	utils.AddStringFlag(getCmd, utils.FlagOrgID, &getOrgID, "", "Organization ID")
	utils.AddStringFlag(getCmd, utils.FlagName, &getName, "", "DevPortal display name")
	utils.AddStringFlag(getCmd, utils.FlagPlatform, &getPlatform, "", "Platform name")
	utils.AddBoolFlag(getCmd, utils.FlagInsecure, &getInsecure, false, "Skip TLS certificate verification")
	_ = getCmd.MarkFlagRequired(utils.FlagPolicyId)
	_ = getCmd.MarkFlagRequired(utils.FlagOrgID)
}

func runGetCommand() error {
	policyID := strings.TrimSpace(getPolicyID)
	if policyID == "" {
		return fmt.Errorf("policy ID is required")
	}

	orgID := strings.TrimSpace(getOrgID)
	if orgID == "" {
		return fmt.Errorf("organization ID is required")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, getName, getPlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, getInsecure)
	path := fmt.Sprintf("/devportal/organizations/%s/subscription-policies/%s", url.PathEscape(orgID), url.PathEscape(policyID))
	resp, err := client.Get(path)
	if err != nil {
		return internaldevportal.WrapRequestError("get subscription plan", err, getInsecure)
	}
	if resp.StatusCode != http.StatusOK {
		return utils.FormatHTTPError("get subscription plan", resp, "DevPortal")
	}

	fmt.Printf("Subscription plan retrieved from devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}
