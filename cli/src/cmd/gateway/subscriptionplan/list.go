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
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	ListCmdLiteral = "list"
	ListCmdExample = `# List all subscription plans
ap gateway subscription-plan list`
)

var listCmd = &cobra.Command{
	Use:     ListCmdLiteral,
	Short:   "List all subscription plans on the gateway",
	Long:    "Retrieves and displays all subscription plans available on the currently active gateway.",
	Example: ListCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runListCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// SubscriptionPlan is a list-view projection of a subscription plan.
type SubscriptionPlan struct {
	ID                 string `json:"id"`
	PlanName           string `json:"planName"`
	BillingPlan        string `json:"billingPlan"`
	ThrottleLimitCount int    `json:"throttleLimitCount"`
	ThrottleLimitUnit  string `json:"throttleLimitUnit"`
	Status             string `json:"status"`
	CreatedAt          string `json:"createdAt"`
}

// SubscriptionPlanListResponse represents the response from GET /subscription-plans.
type SubscriptionPlanListResponse struct {
	SubscriptionPlans []SubscriptionPlan `json:"subscriptionPlans"`
	Count             int                `json:"count"`
}

func init() {
	gateway.AddSelectionFlags(listCmd)
}

func runListCommand(cmd *cobra.Command) error {
	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	resp, err := client.Get(utils.GatewaySubscriptionPlansPath)
	if err != nil {
		return fmt.Errorf("failed to call %s endpoint: %w", utils.GatewaySubscriptionPlansPath, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		fmt.Println("No subscription plans found on the gateway.")
		return nil
	}

	var listResp SubscriptionPlanListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(listResp.SubscriptionPlans) == 0 {
		fmt.Println("No subscription plans found on the gateway.")
		return nil
	}

	headers := []string{"ID", "PLAN_NAME", "BILLING_PLAN", "THROTTLE_LIMIT", "STATUS", "CREATED_AT"}
	rows := make([][]string, 0, len(listResp.SubscriptionPlans))
	for _, p := range listResp.SubscriptionPlans {
		throttle := ""
		if p.ThrottleLimitCount > 0 || p.ThrottleLimitUnit != "" {
			throttle = fmt.Sprintf("%d/%s", p.ThrottleLimitCount, p.ThrottleLimitUnit)
		}
		rows = append(rows, []string{p.ID, p.PlanName, p.BillingPlan, throttle, p.Status, p.CreatedAt})
	}
	utils.PrintTable(headers, rows)

	return nil
}
