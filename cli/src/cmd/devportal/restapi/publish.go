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
	PublishCmdLiteral = "publish"
	PublishCmdExample = `# Publish a REST API to the DevPortal

# Publish a REST API from the current directory using the default artifact name (devportal.zip) and active devportal
ap devportal rest-api publish --org org_1	

# Publish a REST API specifing the API artifact	using active devportal & active
ap devportal rest-api publish -f petstore-api.zip --org org_1

# Publish using a specific devportal without relying on the active devportal
ap devportal rest-api publish -f petstore-api.zip --org org_1 --display-name my-portal --platform eu`
)

var (
	publishFilePath string
	publishOrgID    string
	publishName     string
	publishPlatform string
	publishInsecure bool
)

var publishCmd = &cobra.Command{
	Use:     PublishCmdLiteral,
	Short:   "Publish a REST API to the DevPortal",
	Long:    "Publish a REST API to the WSO2 API Platform DevPortal using an API artifact ZIP file.",
	Example: PublishCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPublishCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(publishCmd, utils.FlagFile, &publishFilePath, "", "Path to the API artifact file")
	utils.AddStringFlag(publishCmd, utils.FlagName, &publishName, "", "DevPortal display name")
	utils.AddStringFlag(publishCmd, utils.FlagPlatform, &publishPlatform, "", "Platform name")
	utils.AddStringFlag(publishCmd, utils.FlagOrgID, &publishOrgID, "", "Organization ID")
	publishCmd.Flags().BoolVar(&publishInsecure, "insecure", false, "Skip TLS certificate verification")
	_ = publishCmd.MarkFlagRequired(utils.FlagOrgID)
}

func runPublishCommand() error {
	artifactPath, err := internaldevportal.ResolveArtifactPath(publishFilePath)
	if err != nil {
		return err
	}

	orgID := strings.TrimSpace(publishOrgID)
	if orgID == "" {
		return fmt.Errorf("organization ID is required")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, publishName, publishPlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, publishInsecure)
	path := fmt.Sprintf("/devportal/organizations/%s/apis", url.PathEscape(orgID))
	resp, err := client.PostMultipartFile(path, "artifact", artifactPath)
	if err != nil {
		return internaldevportal.WrapRequestError("publish api artifact", err, publishInsecure)
	}

	fmt.Printf("API artifact published to devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}
