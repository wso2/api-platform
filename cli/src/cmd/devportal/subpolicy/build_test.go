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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wso2/api-platform/cli/test/testutil"
)

func TestBuildPolicyTemplateFile_RequestCountFree(t *testing.T) {
	workDir := t.TempDir()
	testutil.WithWorkingDir(t, workDir)
	resetPolicyBuildFlags()

	policyDisplayName = "Gold Plan"
	policyType = "requestcount"
	pricingModel = "FREE"
	buildNoInteractive = true

	if err := buildPolicyTemplateFile(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outputPath := filepath.Join(workDir, "gold-plan.json")
	template := readPolicyTemplateFile(t, outputPath)

	if template.PolicyName != "gold-plan" {
		t.Fatalf("expected sanitized policy name gold-plan, got %q", template.PolicyName)
	}
	if template.DisplayName != "Gold Plan" {
		t.Fatalf("expected display name to be preserved, got %q", template.DisplayName)
	}
	if template.BillingPlan != defaultBillingPlanFree {
		t.Fatalf("expected FREE billing plan, got %q", template.BillingPlan)
	}
	if template.RequestCount == nil || *template.RequestCount != 0 {
		t.Fatalf("expected requestCount placeholder, got %+v", template.RequestCount)
	}
	if template.EventCount != nil {
		t.Fatalf("did not expect eventCount for requestcount policy")
	}
	if template.Currency != "" || template.BillingPeriod != "" {
		t.Fatalf("did not expect pricing fields for FREE model: %+v", template)
	}
	if len(template.PricingTiers) != 0 {
		t.Fatalf("did not expect pricing tiers for FREE model: %+v", template.PricingTiers)
	}
}

func TestBuildPolicyTemplateFile_EventCountTieredTemplate(t *testing.T) {
	workDir := t.TempDir()
	testutil.WithWorkingDir(t, workDir)
	resetPolicyBuildFlags()

	policyDisplayName = "Monetized Events"
	policyType = "eventcount"
	pricingModel = "VOLUME_TIERS"
	buildNoInteractive = true

	if err := buildPolicyTemplateFile(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outputPath := filepath.Join(workDir, "monetized-events.json")
	template := readPolicyTemplateFile(t, outputPath)

	if template.BillingPlan != defaultBillingPlanCommercial {
		t.Fatalf("expected COMMERCIAL billing plan, got %q", template.BillingPlan)
	}
	if template.EventCount == nil || *template.EventCount != 0 {
		t.Fatalf("expected eventCount placeholder, got %+v", template.EventCount)
	}
	if template.RequestCount != nil {
		t.Fatalf("did not expect requestCount for eventcount policy")
	}
	if template.Currency != defaultCurrency {
		t.Fatalf("expected default currency %q, got %q", defaultCurrency, template.Currency)
	}
	if template.BillingPeriod != defaultBillingPeriod {
		t.Fatalf("expected default billing period %q, got %q", defaultBillingPeriod, template.BillingPeriod)
	}
	if len(template.PricingTiers) != 1 {
		t.Fatalf("expected one pricing tier placeholder, got %+v", template.PricingTiers)
	}
}

func TestBuildPolicyTemplateFile_UsesProvidedTieredPricingOverrides(t *testing.T) {
	workDir := t.TempDir()
	testutil.WithWorkingDir(t, workDir)
	resetPolicyBuildFlags()

	policyDisplayName = "Tiered Policy"
	policyType = "requestcount"
	pricingModel = "GRADUATED_TIERS"
	flatAmount = "150"
	unitAmount = "25"
	currency = "EUR"
	billingPeriod = "year"
	buildNoInteractive = true

	if err := buildPolicyTemplateFile(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	template := readPolicyTemplateFile(t, filepath.Join(workDir, "tiered-policy.json"))
	if template.Currency != "EUR" {
		t.Fatalf("expected EUR currency, got %q", template.Currency)
	}
	if template.BillingPeriod != "year" {
		t.Fatalf("expected year billing period, got %q", template.BillingPeriod)
	}
	if template.FlatAmount == nil || *template.FlatAmount != 150 {
		t.Fatalf("expected flat amount override, got %+v", template.FlatAmount)
	}
	if template.UnitAmount == nil || *template.UnitAmount != 25 {
		t.Fatalf("expected unit amount override, got %+v", template.UnitAmount)
	}
	if len(template.PricingTiers) != 1 {
		t.Fatalf("expected pricing tier placeholder, got %+v", template.PricingTiers)
	}
}

func TestBuildPolicyTemplateFile_InvalidInputs(t *testing.T) {
	resetPolicyBuildFlags()
	policyDisplayName = "Broken"
	policyType = "invalid"
	pricingModel = "FREE"
	buildNoInteractive = true

	err := buildPolicyTemplateFile()
	if err == nil || !strings.Contains(err.Error(), "invalid policy type") {
		t.Fatalf("expected invalid policy type error, got %v", err)
	}

	resetPolicyBuildFlags()
	policyDisplayName = "Broken"
	policyType = "requestcount"
	pricingModel = "invalid"
	buildNoInteractive = true

	err = buildPolicyTemplateFile()
	if err == nil || !strings.Contains(err.Error(), "invalid pricing model") {
		t.Fatalf("expected invalid pricing model error, got %v", err)
	}
}

func readPolicyTemplateFile(t *testing.T, path string) subscriptionPolicyTemplate {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read generated template %s: %v", path, err)
	}

	var template subscriptionPolicyTemplate
	if err := json.Unmarshal(data, &template); err != nil {
		t.Fatalf("failed to parse generated template %s: %v", path, err)
	}

	return template
}

func resetPolicyBuildFlags() {
	policyDisplayName = ""
	policyType = ""
	requestCount = ""
	eventCount = ""
	pricingModel = ""
	flatAmount = ""
	unitAmount = ""
	billingPeriod = ""
	currency = ""
	buildNoInteractive = false
}
