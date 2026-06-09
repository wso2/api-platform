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

package subscription

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	ListCmdLiteral = "list"
	ListCmdExample = `# List all subscriptions
ap gateway subscription list

# List subscriptions filtered by API and status
ap gateway subscription list --api-id reading-list-api-v1.0 --status ACTIVE`
)

var (
	listAPIID         string
	listApplicationID string
	listStatus        string
)

var listCmd = &cobra.Command{
	Use:     ListCmdLiteral,
	Short:   "List subscriptions on the gateway",
	Long:    "Retrieves and displays subscriptions on the currently active gateway, with optional filtering.",
	Example: ListCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runListCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	gateway.AddSelectionFlags(listCmd)
	utils.AddStringFlag(listCmd, utils.FlagAPIID, &listAPIID, "", "Filter by REST API ID")
	utils.AddStringFlag(listCmd, utils.FlagApplicationID, &listApplicationID, "", "Filter by application ID")
	utils.AddStringFlag(listCmd, utils.FlagStatus, &listStatus, "", "Filter by status (ACTIVE, INACTIVE, REVOKED)")
}

// Subscription is a list-view projection of a subscription.
type Subscription struct {
	ID                 string `json:"id"`
	APIID              string `json:"apiId"`
	ApplicationID      string `json:"applicationId"`
	SubscriptionPlanID string `json:"subscriptionPlanId"`
	Status             string `json:"status"`
	CreatedAt          string `json:"createdAt"`
}

// SubscriptionListResponse represents the response from GET /subscriptions.
type SubscriptionListResponse struct {
	Subscriptions []Subscription `json:"subscriptions"`
	Count         int            `json:"count"`
}

func runListCommand(cmd *cobra.Command) error {
	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	query := url.Values{}
	if strings.TrimSpace(listAPIID) != "" {
		query.Set("apiId", listAPIID)
	}
	if strings.TrimSpace(listApplicationID) != "" {
		query.Set("applicationId", listApplicationID)
	}
	if strings.TrimSpace(listStatus) != "" {
		query.Set("status", listStatus)
	}

	path := utils.GatewaySubscriptionsPath
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}

	resp, err := client.Get(path)
	if err != nil {
		return fmt.Errorf("failed to call %s endpoint: %w", utils.GatewaySubscriptionsPath, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		fmt.Println("No subscriptions found on the gateway.")
		return nil
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("failed to list subscriptions: received status code %d: %s", resp.StatusCode, string(body))
	}

	var listResp SubscriptionListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(listResp.Subscriptions) == 0 {
		fmt.Println("No subscriptions found on the gateway.")
		return nil
	}

	headers := []string{"ID", "API_ID", "APPLICATION_ID", "PLAN_ID", "STATUS", "CREATED_AT"}
	rows := make([][]string, 0, len(listResp.Subscriptions))
	for _, s := range listResp.Subscriptions {
		rows = append(rows, []string{s.ID, s.APIID, s.ApplicationID, s.SubscriptionPlanID, s.Status, s.CreatedAt})
	}
	utils.PrintTable(headers, rows)

	return nil
}
