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
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	internaldevportal "github.com/wso2/api-platform/cli/internal/devportal"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	ListCmdLiteral = "list"
	ListCmdExample = `# List all subscription plans in an organization using the active devportal
ap devportal sub-plan list

# List all subscription plans using a specific devportal
ap devportal sub-plan list --display-name my-portal --platform eu`
)

var (
	listName     string
	listPlatform string
	listInsecure bool
)

var listCmd = &cobra.Command{
	Use:     ListCmdLiteral,
	Short:   "List all DevPortal subscription plans",
	Long:    "Retrieves all subscription plans for a given DevPortal organization.",
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
	utils.AddBoolFlag(listCmd, utils.FlagInsecure, &listInsecure, false, "Skip TLS certificate verification")
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
	path := internaldevportal.ResourcePath("subscription-plans")
	resp, err := client.Get(path)
	if err != nil {
		return internaldevportal.WrapRequestError("list subscription plans", err, listInsecure)
	}
	if resp.StatusCode != http.StatusOK {
		return utils.FormatHTTPError("list subscription plans", resp, "DevPortal")
	}

	fmt.Printf("Subscription plans from devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}
