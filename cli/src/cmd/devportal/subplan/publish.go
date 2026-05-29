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

package subplan

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	internaldevportal "github.com/wso2/api-platform/cli/internal/devportal"
	"github.com/wso2/api-platform/cli/utils"
	"gopkg.in/yaml.v3"
)

const (
	PublishCmdLiteral = "publish"
	PublishCmdExample = `# Publish a single subscription plan (kind: SubscriptionPolicy)
ap devportal sub-plan publish -f sub_plan_gold.yaml --org org_1

# Publish multiple plans in one file (kind: SubscriptionPolicyList)
ap devportal sub-plan publish -f sub_plans.yaml --org org_1

# Publish using a specific devportal without relying on the active devportal
ap devportal sub-plan publish -f sub_plan_gold.yaml --org org_1 --display-name my-portal --platform eu`

	// kindSubscriptionPolicy is the CR kind for a single subscription plan.
	kindSubscriptionPolicy = "SubscriptionPolicy"
	// kindSubscriptionPolicyList is the CR kind for a bulk list of plans.
	kindSubscriptionPolicyList = "SubscriptionPolicyList"
	// multipartFieldName is the multipart field the DevPortal expects the
	// subscription plan YAML in (see SubscriptionPolicyBody in the API spec).
	multipartFieldName = "subscriptionPolicy"
)

var (
	publishFilePath string
	publishOrgID    string
	publishName     string
	publishPlatform string
	publishInsecure bool
)

// subscriptionPlanCR is the minimal CR shape used to validate a subscription
// plan file before it is uploaded. It accepts both a single
// `kind: SubscriptionPolicy` document and a `kind: SubscriptionPolicyList`
// document with an `items` array.
type subscriptionPlanCR struct {
	APIVersion string             `yaml:"apiVersion"`
	Kind       string             `yaml:"kind"`
	Metadata   planMetadata       `yaml:"metadata"`
	Spec       map[string]any     `yaml:"spec"`
	Items      []subscriptionPlan `yaml:"items"`
}

type subscriptionPlan struct {
	Metadata planMetadata   `yaml:"metadata"`
	Spec     map[string]any `yaml:"spec"`
}

type planMetadata struct {
	Name string `yaml:"name"`
}

var publishCmd = &cobra.Command{
	Use:   PublishCmdLiteral,
	Short: "Publish subscription plans to the DevPortal",
	Long: "Publishes one or more subscription plans to the WSO2 API Platform DevPortal from a YAML CR file. " +
		"Use kind: SubscriptionPolicy for a single plan or kind: SubscriptionPolicyList (with an items array) for bulk publishing.",
	Example: PublishCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPublishCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(publishCmd, utils.FlagFile, &publishFilePath, "", "Path to the subscription plan YAML CR file")
	utils.AddStringFlag(publishCmd, utils.FlagOrgID, &publishOrgID, "", "Organization ID")
	utils.AddStringFlag(publishCmd, utils.FlagName, &publishName, "", "DevPortal display name")
	utils.AddStringFlag(publishCmd, utils.FlagPlatform, &publishPlatform, "", "Platform name")
	utils.AddBoolFlag(publishCmd, utils.FlagInsecure, &publishInsecure, false, "Skip TLS certificate verification")
	_ = publishCmd.MarkFlagRequired(utils.FlagFile)
	_ = publishCmd.MarkFlagRequired(utils.FlagOrgID)
}

func runPublishCommand() error {
	filePath := strings.TrimSpace(publishFilePath)
	if filePath == "" {
		return fmt.Errorf("file path is required")
	}

	orgID := strings.TrimSpace(publishOrgID)
	if orgID == "" {
		return fmt.Errorf("organization ID is required")
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("subscription plan file not found: %s", filePath)
		}
		return fmt.Errorf("failed to read subscription plan file: %w", err)
	}

	if err := validateSubscriptionPlanCR(content); err != nil {
		return err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, publishName, publishPlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, publishInsecure)
	path := fmt.Sprintf("/devportal/organizations/%s/subscription-policies", url.PathEscape(orgID))
	resp, err := client.PostMultipartFile(path, multipartFieldName, filePath)
	if err != nil {
		return internaldevportal.WrapRequestError("publish subscription plan", err, publishInsecure)
	}

	fmt.Printf("Subscription plan(s) published to devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}

// validateSubscriptionPlanCR parses the file and verifies it is a supported
// subscription plan CR before it is uploaded, so the user gets a clear error
// locally instead of a server-side 400.
func validateSubscriptionPlanCR(content []byte) error {
	var cr subscriptionPlanCR
	if err := yaml.Unmarshal(content, &cr); err != nil {
		return fmt.Errorf("invalid subscription plan YAML: %w", err)
	}

	switch cr.Kind {
	case kindSubscriptionPolicy:
		if strings.TrimSpace(cr.Metadata.Name) == "" {
			return fmt.Errorf("invalid %s: metadata.name is required", kindSubscriptionPolicy)
		}
	case kindSubscriptionPolicyList:
		if len(cr.Items) == 0 {
			return fmt.Errorf("invalid %s: items must contain at least one plan", kindSubscriptionPolicyList)
		}
		for i, item := range cr.Items {
			if strings.TrimSpace(item.Metadata.Name) == "" {
				return fmt.Errorf("invalid %s: items[%d].metadata.name is required", kindSubscriptionPolicyList, i)
			}
		}
	case "":
		return fmt.Errorf("missing 'kind': expected %s or %s", kindSubscriptionPolicy, kindSubscriptionPolicyList)
	default:
		return fmt.Errorf("unsupported kind %q: expected %s or %s", cr.Kind, kindSubscriptionPolicy, kindSubscriptionPolicyList)
	}

	return nil
}
