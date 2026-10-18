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
ap gateway subscription update --id sub-1 --status REVOKED

# Move a subscription to another plan
ap gateway subscription update --id sub-1 --subscription-plan-id gold-plan-id

# Remove the plan from a subscription
ap gateway subscription update --id sub-1 --clear-subscription-plan

# Rotate the subscription token (prompts for the new token with hidden input)
ap gateway subscription update --id sub-1 --rotate-token`
)

var (
	updateSubscriptionID    string
	updateStatus            string
	updatePlanID            string
	updateClearPlan         bool
	updateSubscriptionToken string
	updateRotateToken       bool
)

var updateCmd = &cobra.Command{
	Use:   UpdateCmdLiteral,
	Short: "Update a subscription on the gateway",
	Long: "Updates an existing subscription in place. Only the fields you pass are changed: " +
		"the status, the subscription plan, and the subscription token. Rotating the token stops the previous " +
		"token from working as soon as the update succeeds.",
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
	utils.AddStringFlag(updateCmd, utils.FlagStatus, &updateStatus, "", "New subscription status (ACTIVE, INACTIVE, REVOKED)")
	utils.AddStringFlag(updateCmd, utils.FlagSubscriptionPlanID, &updatePlanID, "", "ID of the subscription plan to move the subscription to")
	utils.AddBoolFlag(updateCmd, utils.FlagClearSubscriptionPlan, &updateClearPlan, false, "Remove the subscription plan from the subscription")
	utils.AddBoolFlag(updateCmd, utils.FlagRotateToken, &updateRotateToken, false, "Rotate the subscription token; prompts for the new token with hidden input")
	utils.AddStringFlag(updateCmd, utils.FlagSubscriptionToken, &updateSubscriptionToken, "", "New subscription token (not recommended, use --rotate-token)")

	updateCmd.MarkFlagRequired(utils.FlagID)
}

// subscriptionUpdateOptions holds the requested changes for a subscription update. statusSet and
// planSet record whether the --status and --subscription-plan-id flags were supplied at all.
type subscriptionUpdateOptions struct {
	status    string
	statusSet bool
	planID    string
	planSet   bool
	clearPlan bool
	token     string
}

// buildUpdatePayload builds the partial-update body. Only requested fields are included, because the
// gateway leaves omitted fields unchanged; a cleared plan is sent as an explicit null.
func buildUpdatePayload(opts subscriptionUpdateOptions) (map[string]interface{}, error) {
	status := strings.TrimSpace(opts.status)
	planID := strings.TrimSpace(opts.planID)
	token := strings.TrimSpace(opts.token)

	if opts.clearPlan && opts.planSet {
		return nil, fmt.Errorf("--%s and --%s cannot be used together", utils.FlagSubscriptionPlanID, utils.FlagClearSubscriptionPlan)
	}
	if opts.statusSet && status == "" {
		return nil, fmt.Errorf("--%s must not be empty", utils.FlagStatus)
	}
	if opts.planSet && planID == "" {
		return nil, fmt.Errorf("--%s must not be empty", utils.FlagSubscriptionPlanID)
	}

	payload := map[string]interface{}{}
	if status != "" {
		payload["status"] = status
	}
	if planID != "" {
		payload["subscriptionPlanId"] = planID
	}
	if opts.clearPlan {
		payload["subscriptionPlanId"] = nil
	}
	if token != "" {
		payload["subscriptionToken"] = token
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("nothing to update: pass at least one of --%s, --%s, --%s or --%s",
			utils.FlagStatus, utils.FlagSubscriptionPlanID, utils.FlagClearSubscriptionPlan, utils.FlagRotateToken)
	}
	return payload, nil
}

func runUpdateCommand(cmd *cobra.Command) error {
	if strings.TrimSpace(updateSubscriptionID) == "" {
		return fmt.Errorf("--%s is required", utils.FlagID)
	}
	if updateRotateToken && strings.TrimSpace(updateSubscriptionToken) != "" {
		return fmt.Errorf("--%s and --%s cannot be used together", utils.FlagRotateToken, utils.FlagSubscriptionToken)
	}
	if cmd.Flags().Changed(utils.FlagSubscriptionToken) && strings.TrimSpace(updateSubscriptionToken) == "" {
		return fmt.Errorf("--%s must not be empty", utils.FlagSubscriptionToken)
	}

	token := updateSubscriptionToken
	if updateRotateToken {
		var err error
		token, err = utils.PromptPassword("Enter the new subscription token: ")
		if err != nil {
			return err
		}
		if token == "" {
			return fmt.Errorf("the new subscription token must not be empty")
		}
	}

	payload, err := buildUpdatePayload(subscriptionUpdateOptions{
		status:    updateStatus,
		statusSet: cmd.Flags().Changed(utils.FlagStatus),
		planID:    updatePlanID,
		planSet:   cmd.Flags().Changed(utils.FlagSubscriptionPlanID),
		clearPlan: updateClearPlan,
		token:     token,
	})
	if err != nil {
		return err
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
