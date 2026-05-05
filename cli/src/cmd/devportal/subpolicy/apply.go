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
	ApplyCmdLiteral = "apply"
	ApplyCmdExample = `# Apply a subscription policy from a json file
ap devportal sub-policy apply -f gold-policy.yaml`
)

var (
	applyFilePath string
	addName       string
	addPlatform   string
	orgID         string
	addInsecure   bool
)

var applyCmd = &cobra.Command{
	Use:     ApplyCmdLiteral,
	Short:   "Apply a subscription policy to the devportal",
	Long:    "This command allows you to apply a subscription policy to the WSO2 API Platform DevPortal using a JSON or YAML file.",
	Example: ApplyCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runApplyCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(applyCmd, utils.FlagFile, &applyFilePath, "", "Path to the policy file")
	utils.AddStringFlag(applyCmd, utils.FlagName, &addName, "", "Name of the devportal")
	utils.AddStringFlag(applyCmd, utils.FlagPlatform, &addPlatform, "", "Platform of the devportal")
	utils.AddStringFlag(applyCmd, utils.FlagOrgID, &orgID, "", "Organization ID")
	utils.AddBoolFlag(applyCmd, utils.FlagInsecure, &addInsecure, false, "Allow insecure connections")
}

func runApplyCommand() error {
	// Validate the file path
	if applyFilePath == "" {
		return fmt.Errorf("file path is required")
	}

	payload, err := internaldevportal.ReadJSONFile(applyFilePath)
	if err != nil {
		return fmt.Errorf("failed to read policy file: %v", err)
	}

	if orgID == "" {
		return fmt.Errorf("organization ID is required")
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

	path := fmt.Sprintf("/devportal/organizations/%s/subscription-policies", url.PathEscape(orgID))
	resp, err := client.PostJSON(path, payload)
	if err != nil {
		return internaldevportal.WrapRequestError("apply subscription policy", err, addInsecure)
	}
	if resp.StatusCode != http.StatusCreated {
		return utils.FormatHTTPError("apply subscription policy", resp, "DevPortal")
	}

	fmt.Printf("Subscription policy applied successfully using devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)

	return internaldevportal.PrintJSONResponse(resp)
}
