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

package apikey

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
	GetCmdExample = `# List API keys for an API
ap devportal api-key get --api-id api_1

# List API keys from a specific devportal
ap devportal api-key get --api-id api_1 --display-name my-portal --platform eu`
)

var (
	getAPIID       string
	getDisplayName string
	getPlatform    string
	getInsecure    bool
)

var getCmd = &cobra.Command{
	Use:     GetCmdLiteral,
	Short:   "List DevPortal API keys",
	Long:    "Lists API keys for an API. The --api-id flag is required and is resolved to the control-plane API reference.",
	Example: GetCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGetCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(getCmd, utils.FlagAPIID, &getAPIID, "", "Developer Portal API ID")
	utils.AddStringFlag(getCmd, utils.FlagName, &getDisplayName, "", "DevPortal display name")
	utils.AddStringFlag(getCmd, utils.FlagPlatform, &getPlatform, "", "Platform name")
	utils.AddBoolFlag(getCmd, utils.FlagInsecure, &getInsecure, false, "Skip TLS certificate verification")
	_ = getCmd.MarkFlagRequired(utils.FlagAPIID)
}

func runGetCommand() error {
	apiID := strings.TrimSpace(getAPIID)
	if apiID == "" {
		return fmt.Errorf("api ID is required")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, getDisplayName, getPlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, getInsecure)
	path := internaldevportal.ResourcePath("api-keys?apiId=" + url.QueryEscape(apiID))
	resp, err := client.Get(path)
	if err != nil {
		return internaldevportal.WrapRequestError("get API keys", err, getInsecure)
	}
	if resp.StatusCode != http.StatusOK {
		return utils.FormatHTTPError("get API keys", resp, "DevPortal")
	}

	fmt.Printf("API keys from devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}
