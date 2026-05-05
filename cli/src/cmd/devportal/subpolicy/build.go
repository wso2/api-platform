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

package subpolicy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	BuildCmdLiteral = "build"
	BuildCmdExample = `# Generate a request-count policy template
ap devportal sub-policy build --display-name gold --type requestcount --pricing-model FREE

# Generate an event-count policy template in the current directory
ap devportal sub-policy build --display-name monetized-events --type eventcount --pricing-model VOLUME_TIERS`
)

const (
	policyTypeRequestCount = "requestcount"
	policyTypeEventCount   = "eventcount"

	pricingModelFree           = "FREE"
	pricingModelVolumeTiers    = "VOLUME_TIERS"
	pricingModelGraduatedTiers = "GRADUATED_TIERS"

	defaultBillingPlanFree       = "FREE"
	defaultBillingPlanCommercial = "COMMERCIAL"
	defaultCurrency              = "USD"
	defaultBillingPeriod         = "month"
	defaultPolicyOutputName      = "subscription-policy.json"
)

var (
	policyDisplayName string
	policyType        string
	requestCount      string
	eventCount        string
	pricingModel      string
	flatAmount        string
	unitAmount        string
	billingPeriod     string
	currency          string
	buildNoInteractive bool
)

type subscriptionPolicyTemplate struct {
	PolicyName    string                   `json:"policyName"`
	DisplayName   string                   `json:"displayName"`
	BillingPlan   string                   `json:"billingPlan"`
	Description   string                   `json:"description"`
	Type          string                   `json:"type"`
	RequestCount  *int                     `json:"requestCount,omitempty"`
	EventCount    *int                     `json:"eventCount,omitempty"`
	PricingModel  string                   `json:"pricingModel"`
	Currency      string                   `json:"currency,omitempty"`
	BillingPeriod string                   `json:"billingPeriod,omitempty"`
	FlatAmount	*int                     `json:"flatAmount,omitempty"`
	UnitAmount	*int                     `json:"unitAmount,omitempty"`
	ExternalProductID string                   `json:"externalProductId,omitempty"`
	ExternalPriceID   string                   `json:"externalPriceId,omitempty"`
	PricingTiers  []subscriptionPolicyTier `json:"pricingTiers,omitempty"`
	BillingMeterData []billingMeterDataEntry	   `json:"billingMeterData,omitempty"`
}

type subscriptionPolicyTier struct {
	TierIndex int  `json:"tierIndex"`
	StartUnit int  `json:"startUnit"`
	EndUnit   *int `json:"endUnit"`
	UnitPrice *int `json:"unitPrice"`
	FlatPrice *int `json:"flatPrice"`
}

type billingMeterDataEntry struct {
	APIID  string `json:"apiId"`
	MeterID string `json:"meterId"`
}

var buildCmd = &cobra.Command{
	Use:     BuildCmdLiteral,
	Short:   "Generate a subscription policy request template",
	Long:    "Generates a subscription policy JSON template based on the selected policy type and pricing model.",
	Example: BuildCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := buildPolicyTemplateFile(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(buildCmd, utils.FlagName, &policyDisplayName, "", "Display name for the subscription policy")
	utils.AddStringFlag(buildCmd, utils.FlagType, &policyType, "", "Policy type: requestcount or eventcount")
	utils.AddStringFlag(buildCmd, utils.FlagPricingModel, &pricingModel, "", "Pricing model: FREE, VOLUME_TIERS, or GRADUATED_TIERS")
	utils.AddStringFlag(buildCmd, utils.FlagRequestCount, &requestCount, "", "Optional request count value to prefill in the generated template")
	utils.AddStringFlag(buildCmd, utils.FlagEventCount, &eventCount, "", "Optional event count value to prefill in the generated template")
	utils.AddStringFlag(buildCmd, utils.FlagFlatAmount, &flatAmount, "", "Optional flat amount value to prefill in the generated template")
	utils.AddStringFlag(buildCmd, utils.FlagUnitAmount, &unitAmount, "", "Optional unit amount value to prefill in the generated template")
	utils.AddStringFlag(buildCmd, utils.FlagBillingPeriod, &billingPeriod, "", "Optional billing period value to prefill in the generated template")
	utils.AddStringFlag(buildCmd, utils.FlagCurrency, &currency, "", "Optional currency value to prefill in the generated template")
	utils.AddBoolFlag(buildCmd, utils.FlagNoInteractive, &buildNoInteractive, false, "Skip interactive prompts")
}

func buildPolicyTemplateFile() error {
	if err := promptForBuildInputs(); err != nil {
		return err
	}

	normalizedType, err := normalizePolicyType(policyType)
	if err != nil {
		return err
	}

	normalizedPricingModel, err := normalizePricingModel(pricingModel)
	if err != nil {
		return err
	}

	template, err := generatePolicyTemplate(strings.TrimSpace(policyDisplayName), normalizedType, normalizedPricingModel)
	if err != nil {
		return err
	}

	outputPath, err := resolvePolicyOutputPath(template.PolicyName)
	if err != nil {
		return err
	}

	templateData, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal policy template: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := os.WriteFile(outputPath, append(templateData, '\n'), 0644); err != nil {
		return fmt.Errorf("failed to write policy template: %w", err)
	}

	fmt.Printf("Subscription policy template generated at %s\n", outputPath)
	return nil
}

func promptForBuildInputs() error {
	var err error

	if buildNoInteractive {
		return nil
	}

	if strings.TrimSpace(policyDisplayName) == "" {
		policyDisplayName, err = utils.PromptInput("Enter policy display name: ")
		if err != nil {
			return fmt.Errorf("failed to read policy display name: %w", err)
		}
	}
	if strings.TrimSpace(policyType) == "" {
		policyType, err = utils.PromptInput("Enter policy type (requestcount, eventcount): ")
		if err != nil {
			return fmt.Errorf("failed to read policy type: %w", err)
		}
	}
	if strings.TrimSpace(pricingModel) == "" {
		pricingModel, err = utils.PromptInput("Enter pricing model (FREE, VOLUME_TIERS, GRADUATED_TIERS): ")
		if err != nil {
			return fmt.Errorf("failed to read pricing model: %w", err)
		}
	}

	return nil
}

func normalizePolicyType(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case policyTypeRequestCount:
		return policyTypeRequestCount, nil
	case policyTypeEventCount:
		return policyTypeEventCount, nil
	default:
		return "", fmt.Errorf("invalid policy type %q. Supported values: %s, %s", value, policyTypeRequestCount, policyTypeEventCount)
	}
}

func normalizePricingModel(value string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")

	switch normalized {
	case pricingModelFree:
		return pricingModelFree, nil
	case pricingModelVolumeTiers:
		return pricingModelVolumeTiers, nil
	case pricingModelGraduatedTiers:
		return pricingModelGraduatedTiers, nil
	default:
		return "", fmt.Errorf("invalid pricing model %q. Supported values: %s, %s, %s", value, pricingModelFree, pricingModelVolumeTiers, pricingModelGraduatedTiers)
	}
}

func generatePolicyTemplate(displayName, normalizedType, normalizedPricingModel string) (*subscriptionPolicyTemplate, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return nil, fmt.Errorf("missing required flag --%s (or provide it in interactive mode)", utils.FlagName)
	}

	template := &subscriptionPolicyTemplate{
		PolicyName:   sanitizePolicyName(displayName),
		DisplayName:  displayName,
		Description:  "",
		Type:         normalizedType,
		PricingModel: normalizedPricingModel,
	}

	switch normalizedPricingModel {
	case pricingModelFree:
		template.BillingPlan = defaultBillingPlanFree
	case pricingModelVolumeTiers, pricingModelGraduatedTiers:
		template.BillingPlan = defaultBillingPlanCommercial
		template.Currency = firstNonEmpty(strings.TrimSpace(currency), defaultCurrency)
		template.BillingPeriod = firstNonEmpty(strings.TrimSpace(billingPeriod), defaultBillingPeriod)
		flatAmountValue, err := parseOptionalInt(flatAmount)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", utils.FlagFlatAmount, err)
		}
		unitAmountValue, err := parseOptionalInt(unitAmount)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", utils.FlagUnitAmount, err)
		}
		if flatAmountValue == nil {
			flatAmountValue = intPtr(0)
		}
		if unitAmountValue == nil {
			unitAmountValue = intPtr(0)
		}
		template.FlatAmount = flatAmountValue
		template.UnitAmount = unitAmountValue
		template.ExternalProductID = ""
		template.ExternalPriceID = ""
		defaultBillingMeterData := billingMeterDataEntry{
			APIID:  "",
			MeterID: "",
		}
		template.BillingMeterData = []billingMeterDataEntry{defaultBillingMeterData}
	default:
		return nil, fmt.Errorf("unsupported pricing model %q", normalizedPricingModel)
	}

	switch normalizedType {
	case policyTypeRequestCount:
		value, err := parseOptionalInt(requestCount)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", utils.FlagRequestCount, err)
		}
		if value == nil {
			value = intPtr(0)
		}
		template.RequestCount = value
	case policyTypeEventCount:
		value, err := parseOptionalInt(eventCount)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", utils.FlagEventCount, err)
		}
		if value == nil {
			value = intPtr(0)
		}
		template.EventCount = value
	default:
		return nil, fmt.Errorf("unsupported policy type %q", normalizedType)
	}

	switch normalizedPricingModel {
	case pricingModelVolumeTiers, pricingModelGraduatedTiers:
		template.PricingTiers = []subscriptionPolicyTier{
			{
				TierIndex: 1,
				StartUnit: 0,
				EndUnit:   intPtr(0),
				UnitPrice: intPtr(0),
				FlatPrice: intPtr(0),
			},
		}
	}

	return template, nil
}

func resolvePolicyOutputPath(policyName string) (string, error) {
	fileName := defaultPolicyOutputName
	if policyName != "" {
		fileName = fmt.Sprintf("%s.json", policyName)
	}

	outputPath, err := filepath.Abs(filepath.Join(".", fileName))
	if err != nil {
		return "", fmt.Errorf("failed to resolve output path: %w", err)
	}

	return outputPath, nil
}

func parseOptionalInt(value string) (*int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}

	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return nil, fmt.Errorf("expected integer value")
	}

	return &parsed, nil
}

func sanitizePolicyName(displayName string) string {
	displayName = strings.ToLower(strings.TrimSpace(displayName))
	if displayName == "" {
		return "subscription-policy"
	}

	replacer := strings.NewReplacer(
		" ", "-",
		"/", "-",
		"\\", "-",
		"_", "-",
	)
	displayName = replacer.Replace(displayName)

	var builder strings.Builder
	for _, char := range displayName {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '.' {
			builder.WriteRune(char)
		}
	}

	sanitized := strings.Trim(builder.String(), "-.")
	if sanitized == "" {
		return "subscription-policy"
	}

	return sanitized
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func intPtr(value int) *int {
	return &value
}
