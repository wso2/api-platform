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
	GetCmdLiteral = "get"
	GetCmdExample = `# Get a subscription plan by ID
ap gateway subscription-plan get --id gold-plan`
)

var getPlanID string

var getCmd = &cobra.Command{
	Use:     GetCmdLiteral,
	Short:   "Get a subscription plan from the gateway",
	Long:    "Retrieves a specific subscription plan by ID.",
	Example: GetCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGetCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	gateway.AddSelectionFlags(getCmd)
	utils.AddStringFlag(getCmd, utils.FlagID, &getPlanID, "", "Subscription plan ID (required)")
	getCmd.MarkFlagRequired(utils.FlagID)
}

func runGetCommand(cmd *cobra.Command) error {
	if strings.TrimSpace(getPlanID) == "" {
		return fmt.Errorf("--%s is required", utils.FlagID)
	}

	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	resp, err := client.Get(fmt.Sprintf(utils.GatewaySubscriptionPlanByIDPath, url.PathEscape(getPlanID)))
	if err != nil {
		return fmt.Errorf("failed to get subscription plan: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return fmt.Errorf("subscription plan with ID '%s' not found", getPlanID)
	}

	return gateway.PrintJSONResponse(resp)
}
