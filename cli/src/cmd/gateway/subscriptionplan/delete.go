/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package subscriptionplan

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	DeleteCmdLiteral = "delete"
	DeleteCmdExample = `# Delete a subscription plan by ID
ap gateway subscription-plan delete --id gold-plan`
)

var deletePlanID string

var deleteCmd = &cobra.Command{
	Use:     DeleteCmdLiteral,
	Short:   "Delete a subscription plan from the gateway",
	Long:    "Deletes a specific subscription plan by ID.",
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
	utils.AddStringFlag(deleteCmd, utils.FlagID, &deletePlanID, "", "Subscription plan ID to delete (required)")
	deleteCmd.MarkFlagRequired(utils.FlagID)
}

func runDeleteCommand(cmd *cobra.Command) error {
	if strings.TrimSpace(deletePlanID) == "" {
		return fmt.Errorf("--%s is required", utils.FlagID)
	}

	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	resp, err := client.Delete(fmt.Sprintf(utils.GatewaySubscriptionPlanByIDPath, url.PathEscape(deletePlanID)))
	if err != nil {
		return fmt.Errorf("failed to delete subscription plan: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == 404 {
		return fmt.Errorf("subscription plan with ID '%s' not found", deletePlanID)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		// Try to parse error message from response
		var errorResp map[string]interface{}
		if json.Unmarshal(body, &errorResp) == nil {
			if msg, ok := errorResp["message"].(string); ok {
				return fmt.Errorf("failed to delete subscription plan (status %d): %s", resp.StatusCode, msg)
			}
		}
		return fmt.Errorf("failed to delete subscription plan (status %d): %s", resp.StatusCode, string(body))
	}

	fmt.Println("Subscription plan deleted successfully.")
	return nil
}
