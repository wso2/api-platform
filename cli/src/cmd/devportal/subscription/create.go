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
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	internaldevportal "github.com/wso2/api-platform/cli/internal/devportal"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	CreateCmdLiteral = "create"
	CreateCmdExample = `# Create a platform subscription from a JSON file
ap devportal subscription create --org org_1 -f subscription.json

# Create using a specific devportal
ap devportal subscription create --org org_1 -f subscription.json --display-name my-portal --platform eu`
)

var (
	createOrgID    string
	createFilePath string
	createName     string
	createPlatform string
	createInsecure bool
)

var createCmd = &cobra.Command{
	Use:     CreateCmdLiteral,
	Short:   "Create a DevPortal platform subscription",
	Long:    "Creates a platform subscription in the selected DevPortal using a JSON request body from a file.",
	Example: CreateCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runCreateCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(createCmd, utils.FlagOrgID, &createOrgID, "", "Organization ID")
	utils.AddStringFlag(createCmd, utils.FlagFile, &createFilePath, "", "Path to the subscription JSON file")
	utils.AddStringFlag(createCmd, utils.FlagName, &createName, "", "DevPortal display name")
	utils.AddStringFlag(createCmd, utils.FlagPlatform, &createPlatform, "", "Platform name")
	createCmd.Flags().BoolVar(&createInsecure, utils.FlagInsecure, false, "Skip TLS certificate verification")
	_ = createCmd.MarkFlagRequired(utils.FlagOrgID)
	_ = createCmd.MarkFlagRequired(utils.FlagFile)
}

func runCreateCommand() error {
	payload, err := internaldevportal.ReadJSONFile(createFilePath)
	if err != nil {
		return err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, createName, createPlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, createInsecure)
	resp, err := client.PostJSON(fmt.Sprintf("/devportal/organizations/%s/api-platform-subscriptions", createOrgID), payload)
	if err != nil {
		return internaldevportal.WrapRequestError("create platform subscription", err, createInsecure)
	}
	if resp.StatusCode != http.StatusCreated {
		return utils.FormatHTTPError("create platform subscription", resp, "DevPortal")
	}


	fmt.Printf("Platform subscription created using devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}

