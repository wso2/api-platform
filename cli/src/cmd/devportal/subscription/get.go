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
	GetCmdLiteral = "get"
	GetCmdExample = `# Get all platform subscriptions in an organization
ap devportal subscription get --org org_1

# Get a specific platform subscription
ap devportal subscription get --org org_1 --sub-id sub_1`
)

var (
	getOrgID        string
	getSubscription string
	getName         string
	getPlatform     string
	getInsecure     bool
)

var getCmd = &cobra.Command{
	Use:     GetCmdLiteral,
	Short:   "Get DevPortal platform subscriptions",
	Long:    "Retrieves all platform subscriptions for an organization, or a specific platform subscription when --sub-id is provided.",
	Example: GetCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGetCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(getCmd, utils.FlagOrgID, &getOrgID, "", "Organization ID")
	utils.AddStringFlag(getCmd, utils.FlagSubID, &getSubscription, "", "Subscription ID")
	utils.AddStringFlag(getCmd, utils.FlagName, &getName, "", "DevPortal display name")
	utils.AddStringFlag(getCmd, utils.FlagPlatform, &getPlatform, "", "Platform name")
	getCmd.Flags().BoolVar(&getInsecure, utils.FlagInsecure, false, "Skip TLS certificate verification")
	_ = getCmd.MarkFlagRequired(utils.FlagOrgID)
}

func runGetCommand() error {
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
	path := fmt.Sprintf("/devportal/organizations/%s/subscriptions", url.PathEscape(orgID))
	if subscriptionID := strings.TrimSpace(getSubscription); subscriptionID != "" {
		path = fmt.Sprintf("%s/%s", path, url.PathEscape(subscriptionID))
	}

	resp, err := client.Get(path)
	if err != nil {
		if strings.TrimSpace(getSubscription) == "" {
			return internaldevportal.WrapRequestError("get platform subscriptions", err, getInsecure)
		}
		return internaldevportal.WrapRequestError("get platform subscription", err, getInsecure)
	}

	if strings.TrimSpace(getSubscription) == "" {
		fmt.Printf("Platform subscriptions from devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	} else {
		fmt.Printf("Platform subscription details from devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	}
	if resp.StatusCode != http.StatusOK {
		return utils.FormatHTTPError("get platform subscription", resp, "DevPortal")
	}
	return internaldevportal.PrintJSONResponse(resp)
}

