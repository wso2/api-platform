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
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"
)

// modelRemoveButtons are the remove controls of the model pills on the Models tab, one per
// model the selected model provider lists. The model provider row's own remove control,
// "Remove model provider <name>", shares the prefix and is excluded.
func modelRemoveButtons(page playwright.Page) playwright.Locator {
	return page.Locator(`button[aria-label^="Remove model "]:not([aria-label^="Remove model provider "])`)
}

func (u *Steps) reloadsPage(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if _, err := page.Reload(); err != nil {
		return fmt.Errorf("reloading the page: %w", err)
	}
	return nil
}

// opensProviderTab switches the provider overview to the named tab.
func (u *Steps) opensProviderTab(ctx context.Context, tab string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("tab", playwright.PageGetByRoleOptions{Name: tab, Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("opening the %s tab: %w", tab, err)
	}
	return nil
}

// addsModel types a model id into the Models tab and presses Enter, as the tab asks.
func (u *Steps) addsModel(ctx context.Context, model string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	input := page.GetByPlaceholder("e.g., gpt-4.1-mini")
	if err := input.Fill(model); err != nil {
		return fmt.Errorf("typing the model %q: %w", model, err)
	}
	return input.Press("Enter")
}

func (u *Steps) removesModel(ctx context.Context, model string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator(`button[aria-label="Remove model ` + model + `"]`).Click(); err != nil {
		return fmt.Errorf("removing the model %q: %w", model, err)
	}
	return nil
}

// savesProviderChanges presses the overview's Save button, which persists every change
// staged on its tabs, and expects the provider update it sends to succeed.
func (u *Steps) savesProviderChanges(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	resp, err := page.ExpectResponse(func(r playwright.Response) bool {
		return strings.Contains(r.URL(), "/llm-providers/") && r.Request().Method() == "PUT"
	}, func() error {
		return page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Save", Exact: playwright.Bool(true)}).Click()
	})
	if err != nil {
		return fmt.Errorf("saving the provider changes: %w", err)
	}
	if resp.Status() != 200 {
		body, _ := resp.Text()
		return fmt.Errorf("saving the provider changes returned HTTP %d: %s", resp.Status(), body)
	}
	return nil
}

// providerModelsAre expects the Models tab to list exactly the models in the table.
func (u *Steps) providerModelsAre(ctx context.Context, table *godog.Table) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	for _, row := range table.Rows {
		model := row.Cells[0].Value
		if err := u.expect.Locator(page.Locator(`button[aria-label="Remove model ` + model + `"]`)).ToBeVisible(); err != nil {
			return fmt.Errorf("the Models tab does not list %q: %w", model, err)
		}
	}
	if err := u.expect.Locator(modelRemoveButtons(page)).ToHaveCount(len(table.Rows)); err != nil {
		return fmt.Errorf("the Models tab lists models other than the expected %d: %w", len(table.Rows), err)
	}
	return nil
}

// clearsProviderCredentialAndSaves empties the Connection tab's credential the way a user
// does — focusing the masked field clears it, then typing and deleting leaves it empty —
// and presses Save, recording the provider calls the attempt makes. Save is pressed even
// when the form keeps it disabled; a disabled button ignores the press, which is what the
// sheet's expectation is then checked against.
func (u *Steps) clearsProviderCredentialAndSaves(ctx context.Context) error {
	if err := u.watchSecretAndProviderCalls(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	field := credentialField(page)
	if err := field.Click(); err != nil {
		return fmt.Errorf("focusing the credential: %w", err)
	}
	if err := field.PressSequentially("x"); err != nil {
		return fmt.Errorf("typing into the credential: %w", err)
	}
	if err := field.Press("Backspace"); err != nil {
		return fmt.Errorf("clearing the credential: %w", err)
	}
	if err := u.expect.Locator(field).ToHaveValue(""); err != nil {
		return fmt.Errorf("the credential field is not empty: %w", err)
	}
	return page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Save", Exact: playwright.Bool(true)}).
		Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
}

// credentialValidationErrorShown expects the empty credential to be called out: the field
// marked invalid with a message.
func (u *Steps) credentialValidationErrorShown(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	field := credentialField(page)
	if err := u.expect.Locator(field).ToHaveAttribute("aria-invalid", "true"); err != nil {
		return fmt.Errorf("the credential field is not marked invalid: %w", err)
	}
	return nil
}
