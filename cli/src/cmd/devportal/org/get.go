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

package org

import (
	"fmt"
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
	GetCmdExample = `# Get an organization using the active devportal
ap devportal org get --org org_1

# Get an organization using a specific devportal
ap devportal org get --org org_1 --display-name my-portal --platform eu`
)

var (
	getOrgID    string
	getName     string
	getPlatform string
	getInsecure bool
)

var getCmd = &cobra.Command{
	Use:     GetCmdLiteral,
	Short:   "Get a DevPortal organization",
	Long:    "Retrieves a specific organization from the selected DevPortal.",
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
	utils.AddStringFlag(getCmd, utils.FlagName, &getName, "", "DevPortal display name")
	utils.AddStringFlag(getCmd, utils.FlagPlatform, &getPlatform, "", "Platform name")
	getCmd.Flags().BoolVar(&getInsecure, "insecure", false, "Skip TLS certificate verification")
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
	path := fmt.Sprintf("/organizations/%s", url.PathEscape(orgID))
	resp, err := client.Get(path)
	if err != nil {
		return internaldevportal.WrapRequestError("get organization", err, getInsecure)
	}

	fmt.Printf("Organization details from devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}
