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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	internaldevportal "github.com/wso2/api-platform/cli/internal/devportal"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	CreateCmdLiteral = "create"
	CreateCmdExample = `# Create a platform subscription
ap devportal subscription create --api-id api_1 --subscription-plan gold

# Create using a specific devportal
ap devportal subscription create --api-id api_1 --subscription-plan gold --display-name my-portal --platform eu`
)

var (
	createAPIID            string
	createSubscriptionPlan string
	createName             string
	createPlatform         string
	createInsecure         bool
)

var createCmd = &cobra.Command{
	Use:     CreateCmdLiteral,
	Short:   "Create a DevPortal platform subscription",
	Long:    "Creates a platform subscription in the selected DevPortal using request flags or a JSON request body from a file.",
	Example: CreateCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runCreateCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(createCmd, utils.FlagAPIID, &createAPIID, "", "API ID")
	utils.AddStringFlag(createCmd, utils.FlagSubscriptionPlan, &createSubscriptionPlan, "", "Subscription plan name")
	utils.AddStringFlag(createCmd, utils.FlagName, &createName, "", "DevPortal display name")
	utils.AddStringFlag(createCmd, utils.FlagPlatform, &createPlatform, "", "Platform name")
	createCmd.Flags().BoolVar(&createInsecure, utils.FlagInsecure, false, "Skip TLS certificate verification")
	_ = createCmd.MarkFlagRequired(utils.FlagAPIID)
}

func runCreateCommand() error {
	payload, err := buildCreatePayload()
	if err != nil {
		return err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, createName, createPlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, createInsecure)
	resp, err := client.PostJSON(internaldevportal.ResourcePath("subscriptions"), payload)
	if err != nil {
		return internaldevportal.WrapRequestError("create platform subscription", err, createInsecure)
	}
	if resp.StatusCode != http.StatusCreated {
		return utils.FormatHTTPError("create platform subscription", resp, "DevPortal")
	}

	fmt.Printf("Platform subscription created using devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}

func buildCreatePayload() ([]byte, error) {
	apiID := strings.TrimSpace(createAPIID)
	subscriptionPlan := strings.TrimSpace(createSubscriptionPlan)

	if apiID == "" {
		return nil, fmt.Errorf("api ID is required")
	}

	payload := map[string]string{
		"apiId": apiID,
	}
	if subscriptionPlan != "" {
		payload["subscriptionPlanId"] = subscriptionPlan
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to build subscription payload: %w", err)
	}

	return data, nil
}
