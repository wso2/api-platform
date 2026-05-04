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

package restapi

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
	EditCmdLiteral = "edit"
	EditCmdExample = `# Edit an API artifact using the default artifact in the current directory
ap devportal rest-api edit --org org_1 --api-id api_1

# Edit an API artifact using a specific zip file
ap devportal rest-api edit -f fooapi/build/devportal.zip --org org_1 --api-id api_1

# Edit an API artifact using a specific devportal
ap devportal rest-api edit -f fooapi/build/devportal.zip --org org_1 --api-id api_1 --display-name my-portal --platform eu`
)

var (
	editFilePath string
	editOrgID    string
	editAPIID    string
	editName     string
	editPlatform string
	editInsecure bool
)

var editCmd = &cobra.Command{
	Use:     EditCmdLiteral,
	Short:   "Edit an API artifact in a DevPortal organization",
	Long:    "Updates a specific API artifact by uploading a DevPortal zip file.",
	Example: EditCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runEditCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(editCmd, utils.FlagFile, &editFilePath, "", "Path to the API artifact file")
	utils.AddStringFlag(editCmd, utils.FlagOrgID, &editOrgID, "", "Organization ID")
	utils.AddStringFlag(editCmd, utils.FlagAPIID, &editAPIID, "", "API ID")
	utils.AddStringFlag(editCmd, utils.FlagName, &editName, "", "DevPortal display name")
	utils.AddStringFlag(editCmd, utils.FlagPlatform, &editPlatform, "", "Platform name")
	editCmd.Flags().BoolVar(&editInsecure, "insecure", false, "Skip TLS certificate verification")
	_ = editCmd.MarkFlagRequired(utils.FlagOrgID)
	_ = editCmd.MarkFlagRequired(utils.FlagAPIID)
}

func runEditCommand() error {
	artifactPath, err := internaldevportal.ResolveArtifactPath(editFilePath)
	if err != nil {
		return err
	}

	orgID := strings.TrimSpace(editOrgID)
	if orgID == "" {
		return fmt.Errorf("organization ID is required")
	}

	apiID := strings.TrimSpace(editAPIID)
	if apiID == "" {
		return fmt.Errorf("api ID is required")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, editName, editPlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, editInsecure)
	path := fmt.Sprintf("/devportal/organizations/%s/apis/%s", url.PathEscape(orgID), url.PathEscape(apiID))
	resp, err := client.PutMultipartFile(path, "artifact", artifactPath)
	if err != nil {
		return internaldevportal.WrapRequestError("edit api artifact", err, editInsecure)
	}

	fmt.Printf("API artifact updated in devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}
