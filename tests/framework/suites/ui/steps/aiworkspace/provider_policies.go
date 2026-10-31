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

package aiworkspace

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"
)

// The Policy Hub catalogue the AI Workspace reads guardrails from is an external service.
// These fixtures are copied from its real responses — the list trimmed to two guardrails
// the gateway also ships, and Word Count Guardrail's definition verbatim — so guardrail
// scenarios do not depend on that service being reachable or its catalogue staying put.
var (
	//go:embed testdata/policyhub/policies.json
	policyHubPolicies []byte
	//go:embed testdata/policyhub/word-count-guardrail.yaml
	policyHubWordCountDefinition []byte
)

// policyHubRequest matches the browser's calls to the Policy Hub, wherever it is hosted.
var policyHubRequest = regexp.MustCompile(`/policy-hub-[^/]+/[^/]+/v[0-9.]+/policies`)

// policyHubServesFixedCatalogue answers this page's Policy Hub calls from the fixtures.
// A call the fixtures do not cover gets a 404, so a scenario cannot silently fall back to
// the live service.
func (u *Steps) policyHubServesFixedCatalogue(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return page.Route(policyHubRequest, func(route playwright.Route) {
		path := route.Request().URL()
		if i := strings.IndexByte(path, '?'); i >= 0 {
			path = path[:i]
		}
		switch {
		case strings.HasSuffix(path, "/policies"):
			_ = route.Fulfill(playwright.RouteFulfillOptions{
				Status: playwright.Int(200), ContentType: playwright.String("application/json"), Body: policyHubPolicies,
			})
		case strings.Contains(path, "/policies/word-count-guardrail/versions/") && strings.HasSuffix(path, "/definition"):
			_ = route.Fulfill(playwright.RouteFulfillOptions{
				Status: playwright.Int(200), ContentType: playwright.String("text/yaml"), Body: policyHubWordCountDefinition,
			})
		default:
			_ = route.Fulfill(playwright.RouteFulfillOptions{Status: playwright.Int(404)})
		}
	})
}

// attachesGuardrail adds a guardrail at the provider's global scope from the Guardrails &
// Policies tab, filling its parameters from the table (parameter path, value). Every
// parameter section is collapsed at first; the step expands the ones a path needs.
func (u *Steps) attachesGuardrail(ctx context.Context, name string, table *godog.Table) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	add := page.Locator(`xpath=//*[normalize-space(text())="Global Guardrails & Policies"]` +
		`/ancestor::*[.//button[normalize-space()="Add"]][1]//button[normalize-space()="Add"]`).First()
	if err := add.Click(); err != nil {
		return fmt.Errorf("opening the guardrail drawer: %w", err)
	}
	drawer := page.Locator(".MuiDrawer-paper")
	if err := drawer.GetByPlaceholder("Search guardrails").Fill(name); err != nil {
		return fmt.Errorf("searching for the %q guardrail: %w", name, err)
	}
	if err := drawer.GetByText(name, playwright.LocatorGetByTextOptions{Exact: playwright.Bool(true)}).First().Click(); err != nil {
		return fmt.Errorf("picking the %q guardrail: %w", name, err)
	}
	expanded := map[string]bool{}
	for _, row := range table.Rows {
		path, value := row.Cells[0].Value, row.Cells[1].Value
		if section, _, nested := strings.Cut(path, "."); nested && !expanded[section] {
			// Only a section's chevron expands it: label → label row → content → node row.
			label := drawer.GetByText(section, playwright.LocatorGetByTextOptions{Exact: playwright.Bool(true)}).First()
			if err := label.Locator("xpath=../../..").Locator("svg").First().Click(); err != nil {
				return fmt.Errorf("expanding the %q parameters: %w", section, err)
			}
			expanded[section] = true
		}
		field := drawer.Locator(`[data-testid="field-` + path + `"]`)
		if err := field.Fill(value); err != nil {
			return fmt.Errorf("setting the %q parameter: %w", path, err)
		}
	}
	if err := drawer.Locator(`[data-testid="policy-param-submit"]`).Click(); err != nil {
		return fmt.Errorf("adding the %q guardrail: %w", name, err)
	}
	return u.expect.Locator(drawer).ToBeHidden()
}

// providerGuardrailsAre expects the Guardrails & Policies tab to show exactly these
// guardrails, by display name.
func (u *Steps) providerGuardrailsAre(ctx context.Context, table *godog.Table) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	pills := page.Locator(`button[aria-label="Remove guardrail"]`)
	for _, row := range table.Rows {
		name := row.Cells[0].Value
		if err := u.expect.Locator(page.GetByText(name, playwright.PageGetByTextOptions{Exact: playwright.Bool(true)}).First()).ToBeVisible(); err != nil {
			return fmt.Errorf("the guardrail %q is not shown: %w", name, err)
		}
	}
	if err := u.expect.Locator(pills).ToHaveCount(len(table.Rows)); err != nil {
		return fmt.Errorf("the tab shows guardrails other than the expected %d: %w", len(table.Rows), err)
	}
	return nil
}

// rateLimitRow locates one criteria row (Request Count, Token Count, Cost) inside the named
// section (Backend, Per Consumer) of the Rate Limiting tab. The row's header holds its label
// and Enable switch; the row itself also holds the quota and reset fields.
func rateLimitRow(page playwright.Page, section, criteria string) (header, row playwright.Locator) {
	card := page.Locator(".MuiCardContent-root").Filter(playwright.LocatorFilterOptions{
		Has: page.GetByRole("heading", playwright.PageGetByRoleOptions{Name: section, Exact: playwright.Bool(true)}),
	})
	label := card.GetByText(criteria, playwright.LocatorGetByTextOptions{Exact: playwright.Bool(true)}).First()
	header = label.Locator(`xpath=ancestor::div[.//input[@type="checkbox"]][1]`)
	return header, header.Locator("xpath=..")
}

// setsRateLimit enables a criteria row of the section's provider-wide limits and sets its
// quota and reset window.
func (u *Steps) setsRateLimit(ctx context.Context, section, criteria, quota, duration, unit string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	header, row := rateLimitRow(page, section, criteria)
	toggle := header.Locator(`input[type="checkbox"]`)
	if checked, err := toggle.IsChecked(); err != nil {
		return fmt.Errorf("reading the %s %s switch: %w", section, criteria, err)
	} else if !checked {
		if err := toggle.Click(); err != nil {
			return fmt.Errorf("enabling the %s %s limit: %w", section, criteria, err)
		}
	}
	inputs := row.Locator(`input:not([type="checkbox"]):not([aria-hidden="true"])`)
	if visible, _ := inputs.First().IsVisible(); !visible {
		if err := header.Locator("button").Click(); err != nil {
			return fmt.Errorf("expanding the %s %s limit: %w", section, criteria, err)
		}
	}
	if err := inputs.Nth(0).Fill(quota); err != nil {
		return fmt.Errorf("setting the %s %s quota: %w", section, criteria, err)
	}
	if err := inputs.Nth(1).Fill(duration); err != nil {
		return fmt.Errorf("setting the %s %s reset duration: %w", section, criteria, err)
	}
	if err := row.GetByRole("combobox").Click(); err != nil {
		return fmt.Errorf("opening the %s %s reset unit: %w", section, criteria, err)
	}
	// The menu labels units capitalised ("Hour"); the stored value is lower case.
	if err := page.GetByRole("option", playwright.PageGetByRoleOptions{Name: regexp.MustCompile(`(?i)^` + unit + `$`)}).Click(); err != nil {
		return fmt.Errorf("picking the %s %s reset unit %q: %w", section, criteria, unit, err)
	}
	return nil
}

// rateLimitIs expects a criteria row to be enabled with the given quota and reset window.
func (u *Steps) rateLimitIs(ctx context.Context, section, criteria, quota, duration, unit string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	header, row := rateLimitRow(page, section, criteria)
	if err := u.expect.Locator(header.Locator(`input[type="checkbox"]`)).ToBeChecked(); err != nil {
		return fmt.Errorf("the %s %s limit is not enabled: %w", section, criteria, err)
	}
	inputs := row.Locator(`input:not([type="checkbox"]):not([aria-hidden="true"])`)
	if err := u.expect.Locator(inputs.Nth(0)).ToHaveValue(quota); err != nil {
		return fmt.Errorf("the %s %s quota is not %q: %w", section, criteria, quota, err)
	}
	if err := u.expect.Locator(inputs.Nth(1)).ToHaveValue(duration); err != nil {
		return fmt.Errorf("the %s %s reset duration is not %q: %w", section, criteria, duration, err)
	}
	// A collapsed row hides its fields, and role queries skip hidden elements.
	if err := u.expect.Locator(row.Locator(`[role="combobox"]`)).ToHaveText(regexp.MustCompile(`(?i)^` + unit + `$`)); err != nil {
		return fmt.Errorf("the %s %s reset unit is not %q: %w", section, criteria, unit, err)
	}
	return nil
}

// triesToSaveProviderChanges presses Save and records whether the provider update it may
// send was accepted, without failing, for the steps that assert how a save was refused.
func (u *Steps) triesToSaveProviderChanges(ctx context.Context) error {
	if err := u.watchSecretAndProviderCalls(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Save", Exact: playwright.Bool(true)}).Click()
}

// rateLimitIsNotSet expects a criteria row to be switched off, i.e. no limit was stored.
func (u *Steps) rateLimitIsNotSet(ctx context.Context, section, criteria string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	header, _ := rateLimitRow(page, section, criteria)
	if err := u.expect.Locator(header.Locator(`input[type="checkbox"]`)).Not().ToBeChecked(); err != nil {
		return fmt.Errorf("a %s %s limit is set: %w", section, criteria, err)
	}
	return nil
}

// seesErrorAbout expects an error notification whose text names what was wrong, rather
// than a generic failure.
func (u *Steps) seesErrorAbout(ctx context.Context, subject string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	snackbar := page.Locator(`[data-testid="aiworkspace-snackbar-notification"]`).Filter(playwright.LocatorFilterOptions{HasText: subject})
	if err := u.expect.Locator(snackbar.First()).ToBeVisible(); err != nil {
		return fmt.Errorf("no error notification mentions %q: %w", subject, err)
	}
	return nil
}
