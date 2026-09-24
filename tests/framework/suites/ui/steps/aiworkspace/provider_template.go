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

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const keyTemplateVersionIDs = "uiLatestTemplateVersionIDs"

// opensLLMProviderTemplates navigates to the organization's LLM Provider Templates list.
func (u *Steps) opensLLMProviderTemplates(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByText("Settings").First().Click(); err != nil {
		return fmt.Errorf("opening Settings: %w", err)
	}
	return nil
}

// createsLLMProviderTemplate creates a new custom LLM provider template with the given
// display name and endpoint URL, registering its first version (v1.0) for cleanup.
func (u *Steps) createsLLMProviderTemplate(ctx context.Context, name, url string) error {
	var err error
	name, err = expandUIValue(ctx, name)
	if err != nil {
		return err
	}
	url, err = expandUIValue(ctx, url)
	if err != nil {
		return err
	}
	if err := u.opensLLMProviderTemplates(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := cyid(page, "add-provider-template-button").Click(); err != nil {
		return fmt.Errorf("starting template creation: %w", err)
	}
	if err := page.Locator(`input[placeholder="Enter template name"]`).Fill(name); err != nil {
		return fmt.Errorf("typing the template name: %w", err)
	}
	if err := page.Locator(`input[placeholder="https://api.openai.com"]`).Fill(url); err != nil {
		return fmt.Errorf("typing the template endpoint URL: %w", err)
	}
	resp, err := page.ExpectResponse(func(r playwright.Response) bool {
		return strings.Contains(r.URL(), "/llm-provider-templates") &&
			!strings.Contains(r.URL(), "/copy") && r.Request().Method() == "POST"
	}, func() error {
		return cyid(page, "create-provider-template-submit").Click()
	})
	if err != nil {
		return fmt.Errorf("submitting the template: %w", err)
	}
	return u.registerCreatedTemplateVersion(ctx, "v1.0", resp)
}

// opensLLMProviderTemplate opens the named template's own overview page from the LLM
// Provider Templates list.
func (u *Steps) opensLLMProviderTemplate(ctx context.Context, name string) error {
	var err error
	name, err = expandUIValue(ctx, name)
	if err != nil {
		return err
	}
	if err := u.opensLLMProviderTemplates(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByText(name).First().Click(); err != nil {
		return fmt.Errorf("opening the LLM provider template %q: %w", name, err)
	}
	return nil
}

// createsLLMProviderTemplateVersion creates a new version of the template currently open,
// starting from the version selector's current entry, and registers the new version for
// cleanup.
func (u *Steps) createsLLMProviderTemplateVersion(ctx context.Context, fromVersion, toVersion, url string) error {
	var err error
	fromVersion, err = expandUIValue(ctx, fromVersion)
	if err != nil {
		return err
	}
	toVersion, err = expandUIValue(ctx, toVersion)
	if err != nil {
		return err
	}
	url, err = expandUIValue(ctx, url)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: fromVersion, Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("opening the version selector: %w", err)
	}
	if err := page.GetByText("Create new version").Click(); err != nil {
		return fmt.Errorf("starting a new template version: %w", err)
	}
	if err := page.Locator(`input[placeholder="v2.0"]`).Fill(toVersion); err != nil {
		return fmt.Errorf("typing the new version: %w", err)
	}
	if err := page.Locator(`input[placeholder="https://api.openai.com"]`).Fill(url); err != nil {
		return fmt.Errorf("typing the version endpoint URL: %w", err)
	}
	resp, err := page.ExpectResponse(func(r playwright.Response) bool {
		return strings.Contains(r.URL(), "/llm-provider-templates/copy") && r.Request().Method() == "POST"
	}, func() error {
		return cyid(page, "create-provider-template-version-submit").Click()
	})
	if err != nil {
		return fmt.Errorf("submitting the new template version: %w", err)
	}
	return u.registerCreatedTemplateVersion(ctx, toVersion, resp)
}

// registerCreatedTemplateVersion records a template version create response's id, keyed by
// its version string, both for later deregistration lookup and for scenario cleanup.
func (u *Steps) registerCreatedTemplateVersion(ctx context.Context, version string, resp playwright.Response) error {
	if resp.Status() < 200 || resp.Status() >= 300 {
		return nil
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := resp.JSON(&body); err != nil {
		return fmt.Errorf("reading the created template version's id: %w", err)
	}
	if body.ID == "" {
		return fmt.Errorf("the template version create response carried no id")
	}
	ids, err := u.templateVersionIDs(ctx)
	if err != nil {
		return err
	}
	ids[version] = body.ID
	if err := tcontext.Set(ctx, keyTemplateVersionIDs, ids); err != nil {
		return err
	}
	return cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindLLMProviderTemplate, ID: body.ID, Actor: u.topo.Admin.Username,
	})
}

// templateVersionIDs returns the version-to-id map recorded so far in this scenario.
func (u *Steps) templateVersionIDs(ctx context.Context) (map[string]string, error) {
	v, ok := tcontext.Get(ctx, keyTemplateVersionIDs)
	if !ok {
		return map[string]string{}, nil
	}
	ids, ok := v.(map[string]string)
	if !ok {
		return nil, fmt.Errorf("template version id map in scope has unexpected type %T", v)
	}
	return ids, nil
}

// seesVersionButton asserts the version selector currently shows the given version.
func (u *Steps) seesVersionButton(ctx context.Context, version string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: version, Exact: playwright.Bool(true)})).ToBeVisible()
}

// createsProviderFromTemplateVersion creates a provider from the named template's specific
// version.
func (u *Steps) createsProviderFromTemplateVersion(ctx context.Context, providerName, templateName, version string) error {
	var err error
	providerName, err = expandUIValue(ctx, providerName)
	if err != nil {
		return err
	}
	templateName, err = expandUIValue(ctx, templateName)
	if err != nil {
		return err
	}
	version, err = expandUIValue(ctx, version)
	if err != nil {
		return err
	}
	if err := u.startAddingProviderFromTemplateVersion(ctx, templateName, version); err != nil {
		return err
	}
	ids, err := u.templateVersionIDs(ctx)
	if err != nil {
		return err
	}
	expectedTemplateID, ok := ids[version]
	if !ok || expectedTemplateID == "" {
		return fmt.Errorf("no recorded template id for version %q", version)
	}
	return u.submitProviderFormForTemplate(ctx, providerName, nil, expectedTemplateID)
}

// confirmsTemplateVersionDelete opens the delete confirmation for the template version
// currently open and confirms it, without assuming the deletion succeeds.
func (u *Steps) confirmsTemplateVersionDelete(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := cyid(page, "provider-template-delete-button").Click(); err != nil {
		return fmt.Errorf("opening the delete dialog for the template version: %w", err)
	}
	dialog := page.GetByRole("dialog")
	if err := dialog.GetByRole("button",
		playwright.LocatorGetByRoleOptions{Name: "Delete"}).Click(); err != nil {
		return fmt.Errorf("confirming the delete: %w", err)
	}
	return nil
}

// attemptsToDeleteCurrentTemplateVersion confirms a delete that is expected to be blocked
// because a provider still references the version, so cleanup state is left untouched.
func (u *Steps) attemptsToDeleteCurrentTemplateVersion(ctx context.Context) error {
	return u.confirmsTemplateVersionDelete(ctx)
}

// deletesTemplateVersion confirms a delete that is expected to succeed for the named
// version, and deregisters it from cleanup.
func (u *Steps) deletesTemplateVersion(ctx context.Context, version string) error {
	if err := u.confirmsTemplateVersionDelete(ctx); err != nil {
		return err
	}
	ids, err := u.templateVersionIDs(ctx)
	if err != nil {
		return err
	}
	id, ok := ids[version]
	if !ok {
		return fmt.Errorf("no recorded id for template version %q", version)
	}
	reg, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	reg.Deregister(cleanup.KindLLMProviderTemplate, id)
	return nil
}
