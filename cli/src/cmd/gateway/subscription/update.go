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
	"bytes"
	"encoding/json"
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
	UpdateCmdLiteral = "update"
	UpdateCmdExample = `# Update a subscription's status
ap gateway subscription update --id sub-1 --status REVOKED`
)

var (
	updateSubscriptionID string
	updateStatus         string
)

var updateCmd = &cobra.Command{
	Use:     UpdateCmdLiteral,
	Short:   "Update a subscription on the gateway",
	Long:    "Updates an existing subscription's status.",
	Example: UpdateCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUpdateCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	gateway.AddSelectionFlags(updateCmd)
	utils.AddStringFlag(updateCmd, utils.FlagID, &updateSubscriptionID, "", "Subscription ID (required)")
	utils.AddStringFlag(updateCmd, utils.FlagStatus, &updateStatus, "", "Subscription status (ACTIVE, INACTIVE, REVOKED) (required)")

	updateCmd.MarkFlagRequired(utils.FlagID)
	updateCmd.MarkFlagRequired(utils.FlagStatus)
}

func runUpdateCommand(cmd *cobra.Command) error {
	if strings.TrimSpace(updateSubscriptionID) == "" {
		return fmt.Errorf("--%s is required", utils.FlagID)
	}
	if strings.TrimSpace(updateStatus) == "" {
		return fmt.Errorf("--%s is required", utils.FlagStatus)
	}

	payload := map[string]interface{}{
		"status": strings.TrimSpace(updateStatus),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to build subscription payload: %w", err)
	}

	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf(utils.GatewaySubscriptionByIDPath, url.PathEscape(updateSubscriptionID))
	resp, err := client.Put(endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to update subscription: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		resp.Body.Close()
		return fmt.Errorf("failed to update subscription: received status code %d", resp.StatusCode)
	}

	fmt.Println("Subscription updated successfully.")
	return gateway.PrintJSONResponse(resp)
}
