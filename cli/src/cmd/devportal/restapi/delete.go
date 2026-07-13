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
	DeleteCmdLiteral = "delete"
	DeleteCmdExample = `# Delete an API artifact using the active devportal
ap devportal rest-api delete --api-id api_1

# Delete an API artifact using a specific devportal
ap devportal rest-api delete --api-id api_1 --display-name my-portal --platform eu`
)

var (
	deleteAPIID    string
	deleteName     string
	deletePlatform string
	deleteInsecure bool
)

var deleteCmd = &cobra.Command{
	Use:     DeleteCmdLiteral,
	Short:   "Delete an API artifact from a DevPortal organization",
	Long:    "Deletes a specific API artifact by ID from a DevPortal organization.",
	Example: DeleteCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDeleteCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(deleteCmd, utils.FlagAPIID, &deleteAPIID, "", "API ID")
	utils.AddStringFlag(deleteCmd, utils.FlagName, &deleteName, "", "DevPortal display name")
	utils.AddStringFlag(deleteCmd, utils.FlagPlatform, &deletePlatform, "", "Platform name")
	deleteCmd.Flags().BoolVar(&deleteInsecure, "insecure", false, "Skip TLS certificate verification")
	_ = deleteCmd.MarkFlagRequired(utils.FlagAPIID)
}

func runDeleteCommand() error {
	apiID := strings.TrimSpace(deleteAPIID)
	if apiID == "" {
		return fmt.Errorf("api ID is required")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, _, err := internaldevportal.ResolveDevPortal(cfg, deleteName, deletePlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, deleteInsecure)
	path := internaldevportal.ResourcePath("apis/" + url.PathEscape(apiID))
	resp, err := client.Delete(path)
	if err != nil {
		return internaldevportal.WrapRequestError("delete api artifact", err, deleteInsecure)
	}

	return internaldevportal.PrintJSONResponse(resp)
}
