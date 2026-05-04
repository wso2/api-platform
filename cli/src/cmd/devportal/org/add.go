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
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	internaldevportal "github.com/wso2/api-platform/cli/internal/devportal"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	AddCmdLiteral = "add"
	AddCmdExample = `# Create an organization from a JSON file
ap devportal org add -f organization.json

# Create an organization using a specific devportal
ap devportal org add -f organization.json --display-name my-portal --platform eu`
)

var (
	addFilePath string
	addName     string
	addPlatform string
	addInsecure bool
)

var addCmd = &cobra.Command{
	Use:     AddCmdLiteral,
	Short:   "Create a DevPortal organization",
	Long:    "Creates an organization in the selected DevPortal using a JSON request body from a file.",
	Example: AddCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runAddCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(addCmd, utils.FlagFile, &addFilePath, "", "Path to the organization JSON file")
	utils.AddStringFlag(addCmd, utils.FlagName, &addName, "", "DevPortal display name")
	utils.AddStringFlag(addCmd, utils.FlagPlatform, &addPlatform, "", "Platform name")
	addCmd.Flags().BoolVar(&addInsecure, "insecure", false, "Skip TLS certificate verification")
	_ = addCmd.MarkFlagRequired(utils.FlagFile)
}

func runAddCommand() error {
	payload, err := internaldevportal.ReadJSONFile(addFilePath)
	if err != nil {
		return err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, addName, addPlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, addInsecure)
	resp, err := client.PostJSON("/devportal/organizations", payload)
	if err != nil {
		return internaldevportal.WrapRequestError("create organization", err, addInsecure)
	}

	fmt.Printf("Organization created using devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}
