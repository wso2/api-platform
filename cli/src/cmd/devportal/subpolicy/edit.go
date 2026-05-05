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
	EditCmdLiteral = "edit"
	EditCmdExample = `# Edit a subscription policy from a json file
ap devportal sub-policy edit -f gold-policy.yaml`
)

var (
	editFilePath     string
	editDevportalName string
	editPlatform      string
	editOrgID         string
	editInsecure      bool
)

var editCmd = &cobra.Command{
	Use:     EditCmdLiteral,
	Short:   "Edit a subscription policy to the devportal",
	Long:    "This command allows you to edit a subscription policy to the WSO2 API Platform DevPortal using a JSON or YAML file.",
	Example: EditCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runEditCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(editCmd, utils.FlagFile, &editFilePath, "", "Path to the policy file")
	utils.AddStringFlag(editCmd, utils.FlagName, &editDevportalName, "", "Name of the devportal")
	utils.AddStringFlag(editCmd, utils.FlagPlatform, &editPlatform, "", "Platform of the devportal")
	utils.AddStringFlag(editCmd, utils.FlagOrgID, &editOrgID, "", "Organization ID")
	utils.AddBoolFlag(editCmd, utils.FlagInsecure, &editInsecure, false, "Allow insecure connections")
}

func runEditCommand() error {
	// Validate the file path
	if editFilePath == "" {
		return fmt.Errorf("file path is required")
	}

	payload, err := internaldevportal.ReadJSONFile(editFilePath)
	if err != nil {
		return fmt.Errorf("failed to read policy file: %v", err)
	}

	if editOrgID == "" {
		return fmt.Errorf("organization ID is required")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, editDevportalName, editPlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, editInsecure)

	path := fmt.Sprintf("/devportal/organizations/%s/subscription-policies", url.PathEscape(editOrgID))
	resp, err := client.PutJSON(path, payload)
	if err != nil {
		return internaldevportal.WrapRequestError("apply subscription policy", err, editInsecure)
	}
	if resp.StatusCode != http.StatusOK {
		return utils.FormatHTTPError("apply subscription policy", resp, "DevPortal")
	}

	fmt.Printf("Subscription policy applied successfully using devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)

	return internaldevportal.PrintJSONResponse(resp)
}
