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

package subscriptionplan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
)

// kindSubscriptionPlan is the CR kind accepted by the create command.
const kindSubscriptionPlan = "SubscriptionPlan"

const (
	CreateCmdLiteral = "create"
	CreateCmdExample = `# Create a subscription plan from a CR file
ap gateway subscription-plan create --file subscription-plan.yaml
ap gateway subscription-plan create -f subscription-plan.json

# The file is a SubscriptionPlan custom resource, e.g.:
#   apiVersion: gateway.api-platform.wso2.com/v1
#   kind: SubscriptionPlan
#   metadata:
#     name: bronze-1k-per-min
#   spec:
#     planName: Bronze
#     status: ACTIVE
#     stopOnQuotaReach: true
#     throttleLimitCount: 1000
#     throttleLimitUnit: Min`
)

var createFilePath string

var createCmd = &cobra.Command{
	Use:     CreateCmdLiteral,
	Short:   "Create a subscription plan on the gateway",
	Long:    "Creates a new subscription plan from a SubscriptionPlan custom resource file (YAML or JSON). The resource spec is sent to the gateway management API.",
	Example: CreateCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runCreateCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	gateway.AddSelectionFlags(createCmd)
	utils.AddStringFlag(createCmd, utils.FlagFile, &createFilePath, "", "Path to the SubscriptionPlan CR file (YAML or JSON)")
	createCmd.MarkFlagRequired(utils.FlagFile)
}

func runCreateCommand(cmd *cobra.Command) error {
	if strings.TrimSpace(createFilePath) == "" {
		return fmt.Errorf("--%s is required", utils.FlagFile)
	}

	cr, err := gateway.ParseResourceCR(createFilePath, kindSubscriptionPlan)
	if err != nil {
		return err
	}

	if name, ok := cr.Spec["planName"].(string); !ok || strings.TrimSpace(name) == "" {
		return fmt.Errorf("invalid %s: spec.planName is required", kindSubscriptionPlan)
	}

	data, err := json.Marshal(cr.Spec)
	if err != nil {
		return fmt.Errorf("failed to build subscription plan payload: %w", err)
	}

	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	resp, err := client.Post(utils.GatewaySubscriptionPlansPath, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create subscription plan: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("failed to create subscription plan: received status code %d", resp.StatusCode)
	}

	fmt.Printf("Subscription plan %q created successfully.\n", cr.Metadata.Name)
	return gateway.PrintJSONResponse(resp)
}
