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
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
)

// kindSubscription is the CR kind accepted by the create command.
const kindSubscription = "Subscription"

const (
	CreateCmdLiteral = "create"
	CreateCmdExample = `# Create a subscription from a CR file
ap gateway subscription create --file subscription.yaml
ap gateway subscription create -f subscription.json

# The file is a Subscription custom resource, e.g.:
#   apiVersion: gateway.api-platform.wso2.com/v1alpha1
#   kind: Subscription
#   metadata:
#     name: petstore-acme-bronze
#   spec:
#     apiId: petstore-api-v1.0
#     subscriptionPlanId: bronze-1k-per-min
#     status: ACTIVE
#     subscriptionToken: a-strong-token-of-at-least-36-characters`
)

var createFilePath string

var createCmd = &cobra.Command{
	Use:   CreateCmdLiteral,
	Short: "Create a subscription on the gateway",
	Long: "Creates a new subscription from a Subscription custom resource file (YAML or JSON). The resource spec is sent to the gateway management API. " +
		"spec.subscriptionToken must be a plain string; secret references (valueFrom) are an operator-only feature and are not resolved by the CLI.",
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
	utils.AddStringFlag(createCmd, utils.FlagFile, &createFilePath, "", "Path to the Subscription CR file (YAML or JSON)")
	createCmd.MarkFlagRequired(utils.FlagFile)
}

func runCreateCommand(cmd *cobra.Command) error {
	if strings.TrimSpace(createFilePath) == "" {
		return fmt.Errorf("--%s is required", utils.FlagFile)
	}

	cr, err := gateway.ParseResourceCR(createFilePath, kindSubscription)
	if err != nil {
		return err
	}

	if apiID, ok := cr.Spec["apiId"].(string); !ok || strings.TrimSpace(apiID) == "" {
		return fmt.Errorf("invalid %s: spec.apiId is required", kindSubscription)
	}
	if token, present := cr.Spec["subscriptionToken"]; present {
		if _, ok := token.(string); !ok {
			return fmt.Errorf("invalid %s: spec.subscriptionToken must be a plain string; secret references (valueFrom) are only supported by the gateway operator", kindSubscription)
		}
	}

	data, err := json.Marshal(cr.Spec)
	if err != nil {
		return fmt.Errorf("failed to build subscription payload: %w", err)
	}

	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}

	resp, err := client.Post(utils.GatewaySubscriptionsPath, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("failed to create subscription: received status code %d", resp.StatusCode)
	}

	fmt.Printf("Subscription %q created successfully.\n", cr.Metadata.Name)
	return gateway.PrintJSONResponse(resp)
}
