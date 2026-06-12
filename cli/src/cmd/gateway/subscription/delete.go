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
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	DeleteCmdLiteral = "delete"
	DeleteCmdExample = `# Delete a subscription by ID
ap gateway subscription delete --id sub-1`
)

var deleteSubscriptionID string

var deleteCmd = &cobra.Command{
	Use:     DeleteCmdLiteral,
	Short:   "Delete a subscription from the gateway",
	Long:    "Deletes a specific subscription by ID.",
	Example: DeleteCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDeleteCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	gateway.AddSelectionFlags(deleteCmd)
	utils.AddStringFlag(deleteCmd, utils.FlagID, &deleteSubscriptionID, "", "Subscription ID to delete (required)")
	deleteCmd.MarkFlagRequired(utils.FlagID)
}

func runDeleteCommand(cmd *cobra.Command) error {
	if strings.TrimSpace(deleteSubscriptionID) == "" {
		return fmt.Errorf("--%s is required", utils.FlagID)
	}

	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	resp, err := client.Delete(fmt.Sprintf(utils.GatewaySubscriptionByIDPath, url.PathEscape(deleteSubscriptionID)))
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("failed to delete subscription: received status code %d", resp.StatusCode)
	}

	fmt.Println("Subscription deleted successfully.")
	return nil
}
