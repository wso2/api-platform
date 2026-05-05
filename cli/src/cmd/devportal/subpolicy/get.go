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
	GetCmdLiteral = "get"
	GetCmdExample = `# Get a subscription policy from the devportal
ap devportal sub-policy get --policy-id gold-policy-id`
)

var (
	getDevportalName string
	getPlatform      string
	getOrgID         string
	getInsecure      bool
	getPolicyID      string
)

var getCmd = &cobra.Command{
	Use:     GetCmdLiteral,
	Short:   "Get a subscription policy from the devportal",
	Long:    "This command allows you to get a subscription policy from the WSO2 API Platform DevPortal.",
	Example: GetCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGetCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(getCmd, utils.FlagPolicyId, &getPolicyID, "", "ID of the policy to get")
	utils.AddStringFlag(getCmd, utils.FlagName, &getDevportalName, "", "Name of the devportal")
	utils.AddStringFlag(getCmd, utils.FlagPlatform, &getPlatform, "", "Platform of the devportal")
	utils.AddStringFlag(getCmd, utils.FlagOrgID, &getOrgID, "", "Organization ID")
	utils.AddBoolFlag(getCmd, utils.FlagInsecure, &getInsecure, false, "Allow insecure connections")
}

func runGetCommand() error {
	// Validate the policy ID
	if getPolicyID == "" {
		return fmt.Errorf("policy ID is required")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, getDevportalName, getPlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, getInsecure)

	path := fmt.Sprintf("/devportal/organizations/%s/subscription-policies/%s", url.PathEscape(getOrgID), url.PathEscape(getPolicyID))
	resp, err := client.Get(path)
	if err != nil {
		return internaldevportal.WrapRequestError("get subscription policy", err, getInsecure)
	}
	if resp.StatusCode != http.StatusOK {
		return utils.FormatHTTPError("get subscription policy", resp, "DevPortal")
	}

	fmt.Printf("Subscription policy retrieved successfully using devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)

	return internaldevportal.PrintJSONResponse(resp)
}
