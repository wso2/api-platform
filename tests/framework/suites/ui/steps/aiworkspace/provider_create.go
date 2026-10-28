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
	"net/url"
	"regexp"
	"strings"

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const keyLastProviderCreateStatus = "uiLastProviderCreateStatus"

// secretStatusDeprecated is the status platform-api gives a deleted secret.
const secretStatusDeprecated = "DEPRECATED"

// validProviderID is the shape the platform documents for a provider handle: lowercase
// alphanumeric words joined by single hyphens.
var validProviderID = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// providerFormFields maps the field names features use to the create form's test hooks.
var providerFormFields = map[string]string{
	"Name":     "provider-name-input",
	"Endpoint": "provider-upstream-url-input",
}

// providerListName is how the provider list renders a display name on its card: the
// list truncates it to 30 characters (providerTemplateDisplay.ts).
func providerListName(name string) string {
	const maxLength = 30
	name = strings.TrimSpace(name)
	if len([]rune(name)) <= maxLength {
		return name
	}
	return strings.TrimSpace(string([]rune(name)[:maxLength])) + "…"
}

// registerFormSecret registers the secret the create form made for its API key. The form
// creates it with its own generated handle before submitting the provider, so the handle
// comes from the recorded /secrets response.
func (u *Steps) registerFormSecret(ctx context.Context) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	if _, err := t.awaitSecretCall(ctx); err != nil {
		return fmt.Errorf("the form never created a secret for the API key: %w", err)
	}
	handle, ok := t.lastSecretHandle()
	if !ok {
		return fmt.Errorf("the form's /secrets response carried no secret id")
	}
	return cleanup.Register(ctx, cleanup.Resource{Kind: cleanup.KindSecret, ID: handle, Actor: u.topo.Admin.Username})
}

// latestProviderID is the id of the provider this scenario's form most recently created.
func latestProviderID(ctx context.Context) (string, error) {
	v, ok := tcontext.Get(ctx, keyLatestProviderID)
	id, _ := v.(string)
	if !ok || id == "" {
		return "", fmt.Errorf("no provider has been created in this scenario")
	}
	return id, nil
}

// providerIsListed searches the provider list for name and expects the card of the provider
// this scenario created, labelled with that name.
func (u *Steps) providerIsListed(ctx context.Context, name string) error {
	name, err := expandUIValue(ctx, name)
	if err != nil {
		return err
	}
	id, err := latestProviderID(ctx)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := cyid(page, "nav-service-provider").Click(); err != nil {
		return fmt.Errorf("opening Service Provider: %w", err)
	}
	// The list is fetched when the page mounts; reload so it reflects the provider just made.
	if _, err := page.Reload(); err != nil {
		return fmt.Errorf("reloading the provider list: %w", err)
	}
	if err := fillCyidInput(page, "provider-search-input", name); err != nil {
		return err
	}
	card := cyid(page, "provider-card-"+id)
	if err := u.expect.Locator(card).ToBeVisible(); err != nil {
		return fmt.Errorf("provider %q (%s) is not in the provider list: %w", name, id, err)
	}
	if err := u.expect.Locator(card).ToContainText(providerListName(name)); err != nil {
		return fmt.Errorf("provider %q's card does not show its name: %w", name, err)
	}
	return nil
}

// triesToCreateProvider fills and submits the form like createProvider, but records the
// outcome instead of failing on a rejection, for the steps that assert how it was refused.
func (u *Steps) triesToCreateProvider(ctx context.Context, name, apiKey string) error {
	name, err := expandUIValue(ctx, name)
	if err != nil {
		return err
	}
	upstream, err := u.mockLLMURL()
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.watchSecretAndProviderCalls(ctx); err != nil {
		return err
	}
	if err := fillCyidInput(page, "provider-name-input", name); err != nil {
		return err
	}
	if err := fillCyidInput(page, "provider-upstream-url-input", upstream); err != nil {
		return err
	}
	if err := fillCyidInput(page, "provider-api-key-input", apiKey); err != nil {
		return err
	}
	resp, err := page.ExpectResponse(func(r playwright.Response) bool {
		return strings.Contains(r.URL(), "/llm-providers") && r.Request().Method() == "POST"
	}, func() error {
		return cyid(page, "add-provider-button").Click()
	})
	if err != nil {
		return fmt.Errorf("submitting the provider: %w", err)
	}
	if err := tcontext.Set(ctx, keyLastProviderCreateStatus, resp.Status()); err != nil {
		return err
	}
	if resp.Status() >= 200 && resp.Status() < 300 {
		// Accepted when it should not have been: still owned by this scenario, so clean it up.
		var body struct {
			ID string `json:"id"`
		}
		if err := resp.JSON(&body); err == nil && body.ID != "" {
			if err := cleanup.Register(ctx, cleanup.Resource{
				Kind: cleanup.KindLLMProvider, ID: body.ID, Actor: u.topo.Admin.Username,
			}); err != nil {
				return err
			}
		}
		return u.registerFormSecret(ctx)
	}
	return nil
}

func (u *Steps) providerCreationRejectedWith(ctx context.Context, want int) error {
	v, ok := tcontext.Get(ctx, keyLastProviderCreateStatus)
	got, _ := v.(int)
	if !ok {
		return fmt.Errorf("no provider creation was attempted in this scenario")
	}
	if got != want {
		return fmt.Errorf("the provider creation returned HTTP %d, want %d", got, want)
	}
	return nil
}

func (u *Steps) stillOnAddProviderForm(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/service-provider/create$`)); err != nil {
		return fmt.Errorf("the user was taken off the add-provider form: %w", err)
	}
	return u.expect.Locator(cyid(page, "add-provider-button")).ToBeVisible()
}

// exactlyOneProviderNamed asserts, through platform-api, that one provider carries the
// generated display name. The list is newest-first, so the first page holds it.
func (u *Steps) exactlyOneProviderNamed(ctx context.Context, name string) error {
	name, err := expandUIValue(ctx, name)
	if err != nil {
		return err
	}
	page, base, token, err := u.platformAPI(ctx)
	if err != nil {
		return err
	}
	resp, err := page.Context().Request().Get(base+"/api/v0.9/llm-providers?limit=100",
		playwright.APIRequestContextGetOptions{
			Headers:           map[string]string{"Authorization": "Bearer " + token},
			IgnoreHttpsErrors: playwright.Bool(true),
		})
	if err != nil {
		return fmt.Errorf("listing providers: %w", err)
	}
	if resp.Status() != 200 {
		body, _ := resp.Text()
		return fmt.Errorf("listing providers returned %d: %s", resp.Status(), body)
	}
	var list struct {
		List []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"list"`
	}
	if err := resp.JSON(&list); err != nil {
		return fmt.Errorf("reading the provider list: %w", err)
	}
	var ids []string
	for _, p := range list.List {
		if p.DisplayName == name {
			ids = append(ids, p.ID)
		}
	}
	if len(ids) != 1 {
		return fmt.Errorf("want exactly one provider named %q, found %d: %v", name, len(ids), ids)
	}
	return nil
}

// rejectedAttemptSecretRemoved asserts the secret the form created for a rejected attempt
// was deleted: the AI Workspace backend deletes it asynchronously once the create fails.
// platform-api deletes a secret softly — it stays readable with status DEPRECATED — so a
// deprecated secret counts as removed, as does one that no longer answers at all.
func (u *Steps) rejectedAttemptSecretRemoved(ctx context.Context) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	if _, err := t.awaitSecretCall(ctx); err != nil {
		return fmt.Errorf("the rejected attempt never created a secret: %w", err)
	}
	handle, ok := t.lastSecretHandle()
	if !ok {
		return fmt.Errorf("the rejected attempt's /secrets response carried no secret id")
	}
	page, base, token, err := u.platformAPI(ctx)
	if err != nil {
		return err
	}
	_, err = retry.Until(ctx, retry.Options{}, func(context.Context) (int, error) {
		resp, err := page.Context().Request().Get(base+"/api/v0.9/secrets/"+url.PathEscape(handle),
			playwright.APIRequestContextGetOptions{
				Headers:           map[string]string{"Authorization": "Bearer " + token},
				IgnoreHttpsErrors: playwright.Bool(true),
			})
		if err != nil {
			return 0, retry.Transient(err)
		}
		if resp.Status() == 404 {
			return resp.Status(), nil
		}
		var secret struct {
			Status string `json:"status"`
		}
		if resp.Status() == 200 && resp.JSON(&secret) == nil && secret.Status == secretStatusDeprecated {
			return resp.Status(), nil
		}
		return 0, retry.Transient(fmt.Errorf("secret %q answers %d with status %q", handle, resp.Status(), secret.Status))
	}, func(int) bool { return true })
	if err != nil {
		// Left behind: make sure this run does not leak it, then report the defect.
		if regErr := cleanup.Register(ctx, cleanup.Resource{Kind: cleanup.KindSecret, ID: handle, Actor: u.topo.Admin.Username}); regErr != nil {
			return fmt.Errorf("the rejected attempt left its secret %q behind (%v); registering it for cleanup failed: %w", handle, err, regErr)
		}
		return fmt.Errorf("the rejected attempt left its secret %q behind: %w", handle, err)
	}
	return nil
}

// providerCreatedWithIDStarting checks the generated id of the provider this scenario just
// created: it starts with the expected slug and is a well-formed handle of 3 to 40
// characters, the limits the platform's API documents.
func (u *Steps) providerCreatedWithIDStarting(ctx context.Context, prefix string) error {
	id, err := latestProviderID(ctx)
	if err != nil {
		return err
	}
	switch {
	case !strings.HasPrefix(id, prefix):
		return fmt.Errorf("the generated provider id %q does not start with %q", id, prefix)
	case !validProviderID.MatchString(id):
		return fmt.Errorf("the generated provider id %q is not lowercase words joined by single hyphens", id)
	case len(id) < 3 || len(id) > 40:
		return fmt.Errorf("the generated provider id %q is %d characters, outside 3..40", id, len(id))
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/service-provider/` + regexp.QuoteMeta(id) + `$`))
}

// triesToSubmitEmptyProviderForm does what a user does with an incomplete form: visits each
// required field, leaves it empty, and presses Add Provider. The press is forced because
// the form disables the button while required fields are empty; a disabled button
// ignores it, which is the behaviour the sheet's expectation is checked against.
func (u *Steps) triesToSubmitEmptyProviderForm(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.watchSecretAndProviderCalls(ctx); err != nil {
		return err
	}
	for _, field := range []string{"Name", "Endpoint"} {
		input := cyid(page, providerFormFields[field]).Locator("input:visible")
		if err := u.expect.Locator(input).ToHaveValue(""); err != nil {
			return fmt.Errorf("the %s field is not empty: %w", field, err)
		}
		if err := input.Focus(); err != nil {
			return fmt.Errorf("focusing the %s field: %w", field, err)
		}
		if err := input.Blur(); err != nil {
			return fmt.Errorf("leaving the %s field: %w", field, err)
		}
	}
	return cyid(page, "add-provider-button").Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
}

func (u *Steps) entersProviderEndpoint(ctx context.Context, endpoint string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := fillCyidInput(page, "provider-upstream-url-input", endpoint); err != nil {
		return err
	}
	// Field validation runs when the field loses focus, as it does for a user tabbing on.
	return cyid(page, "provider-upstream-url-input").Locator("input:visible").Press("Tab")
}

func (u *Steps) entersProviderName(ctx context.Context, name string) error {
	name, err := expandUIValue(ctx, name)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return fillCyidInput(page, "provider-name-input", name)
}

// providerFormShowsFieldError expects the field to be marked invalid with a visible message.
func (u *Steps) providerFormShowsFieldError(ctx context.Context, field string) error {
	hook, ok := providerFormFields[field]
	if !ok {
		return fmt.Errorf("unknown provider form field %q", field)
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	wrapper := cyid(page, hook)
	if err := u.expect.Locator(wrapper.Locator("input:visible")).ToHaveAttribute("aria-invalid", "true"); err != nil {
		return fmt.Errorf("the %s field is not marked invalid: %w", field, err)
	}
	if err := u.expect.Locator(wrapper.Locator(".Mui-error").Last()).ToBeVisible(); err != nil {
		return fmt.Errorf("the %s field shows no error message: %w", field, err)
	}
	return nil
}
