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
	UpdateCmdExample = `# Update a subscription plan's throttle limit
ap gateway subscription-plan update --id gold-plan --throttle-limit-count 2000 --throttle-limit-unit Hour

# Deactivate a subscription plan
ap gateway subscription-plan update --id gold-plan --status INACTIVE`
)

var (
	updatePlanID             string
	updatePlanName           string
	updateBillingPlan        string
	updateStopOnQuotaReach   bool
	updateThrottleLimitCount int
	updateThrottleLimitUnit  string
	updateExpiryTime         string
	updateStatus             string
)

var updateCmd = &cobra.Command{
	Use:     UpdateCmdLiteral,
	Short:   "Update a subscription plan on the gateway",
	Long:    "Updates an existing subscription plan. Only the flags you provide are sent to the gateway.",
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
	utils.AddStringFlag(updateCmd, utils.FlagID, &updatePlanID, "", "Subscription plan ID (required)")
	utils.AddStringFlag(updateCmd, utils.FlagPlanName, &updatePlanName, "", "Name of the subscription plan")
	utils.AddStringFlag(updateCmd, utils.FlagBillingPlan, &updateBillingPlan, "", "Billing plan (e.g. FREE, COMMERCIAL)")
	utils.AddBoolFlag(updateCmd, utils.FlagStopOnQuotaReach, &updateStopOnQuotaReach, false, "Stop serving requests once the throttle quota is reached")
	utils.AddIntFlag(updateCmd, utils.FlagThrottleLimitCount, &updateThrottleLimitCount, 0, "Number of allowed requests per throttle limit unit")
	utils.AddStringFlag(updateCmd, utils.FlagThrottleLimitUnit, &updateThrottleLimitUnit, "", "Throttle limit time unit (e.g. Min, Hour, Day)")
	utils.AddStringFlag(updateCmd, utils.FlagExpiryTime, &updateExpiryTime, "", "Plan expiry time (ISO-8601, e.g. 2026-12-31T23:59:59Z)")
	utils.AddStringFlag(updateCmd, utils.FlagStatus, &updateStatus, "", "Plan status (e.g. ACTIVE, INACTIVE)")

	updateCmd.MarkFlagRequired(utils.FlagID)
}

func runUpdateCommand(cmd *cobra.Command) error {
	if strings.TrimSpace(updatePlanID) == "" {
		return fmt.Errorf("--%s is required", utils.FlagID)
	}

	payload := buildPlanPayload(cmd, updatePlanName, updateBillingPlan, updateStopOnQuotaReach,
		updateThrottleLimitCount, updateThrottleLimitUnit, updateExpiryTime, updateStatus)

	if len(payload) == 0 {
		return fmt.Errorf("no fields to update: provide at least one field flag")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to build subscription plan payload: %w", err)
	}

	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf(utils.GatewaySubscriptionPlanByIDPath, url.PathEscape(updatePlanID))
	resp, err := client.Put(endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to update subscription plan: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		resp.Body.Close()
		return fmt.Errorf("failed to update subscription plan: received status code %d", resp.StatusCode)
	}

	fmt.Println("Subscription plan updated successfully.")
	return gateway.PrintJSONResponse(resp)
}

// buildPlanPayload assembles the request body, including only fields that were
// explicitly set on the command line so partial PUT updates don't send
// unintended zero values.
func buildPlanPayload(cmd *cobra.Command, planName, billingPlan string, stopOnQuotaReach bool,
	throttleLimitCount int, throttleLimitUnit, expiryTime, status string) map[string]interface{} {
	payload := map[string]interface{}{}

	if cmd.Flags().Changed(utils.FlagPlanName) {
		payload["planName"] = strings.TrimSpace(planName)
	}
	if cmd.Flags().Changed(utils.FlagBillingPlan) {
		payload["billingPlan"] = strings.TrimSpace(billingPlan)
	}
	if cmd.Flags().Changed(utils.FlagStopOnQuotaReach) {
		payload["stopOnQuotaReach"] = stopOnQuotaReach
	}
	if cmd.Flags().Changed(utils.FlagThrottleLimitCount) {
		payload["throttleLimitCount"] = throttleLimitCount
	}
	if cmd.Flags().Changed(utils.FlagThrottleLimitUnit) {
		payload["throttleLimitUnit"] = strings.TrimSpace(throttleLimitUnit)
	}
	if cmd.Flags().Changed(utils.FlagExpiryTime) {
		payload["expiryTime"] = strings.TrimSpace(expiryTime)
	}
	if cmd.Flags().Changed(utils.FlagStatus) {
		payload["status"] = strings.TrimSpace(status)
	}

	return payload
}
