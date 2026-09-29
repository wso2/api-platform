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
	"encoding/json"
	"fmt"
	"strings"

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// opensProxyProviderTab switches the proxy overview to its Providers tab and opens the
// first provider's settings.
//
// A proxy now serves a list of providers, so the tab is a list of rows rather than one
// provider's fields — the credential lives in the settings panel a row opens. Opening it
// here keeps every caller on the same footing as before: the step leaves the page with the
// credential field on screen and ready to type into.
func (u *Steps) opensProxyProviderTab(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("tab", playwright.PageGetByRoleOptions{
		Name: "Providers", Exact: playwright.Bool(true),
	}).Click(); err != nil {
		return fmt.Errorf("opening the Providers tab: %w", err)
	}
	if err := cyid(page, "provider-row-0-edit").Click(); err != nil {
		return fmt.Errorf("opening the provider's settings: %w", err)
	}
	return u.expect.Locator(cyid(page, "provider-settings-api-key")).ToBeVisible()
}

// changesProxyCredential types a new value into the unmasked API key field and clicks the
// page's Save button — editing stages the change locally; Save is what persists it,
// recording the calls the save makes.
func (u *Steps) changesProxyCredential(ctx context.Context, newValue string) error {
	if err := u.markSensitive(ctx); err != nil {
		return err
	}
	if err := u.watchSecretAndProviderCalls(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := cyid(page, "provider-settings-api-key").Locator("input:visible").Fill(newValue); err != nil {
		return fmt.Errorf("typing the new credential: %w", err)
	}
	// Two saves, because there are two decisions. The panel's save applies the change to
	// the provider it is editing; the page's save is what sends the whole list. Between
	// them the proxy on the server is untouched, which is what lets a user back out.
	if err := cyid(page, "provider-settings-save").Click(); err != nil {
		return fmt.Errorf("saving the provider's settings: %w", err)
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Save", Exact: playwright.Bool(true),
	}).Click(); err != nil {
		return fmt.Errorf("clicking Save: %w", err)
	}
	return nil
}

// changesProxyCredentialToPlaceholder is changesProxyCredential, with the new value built
// as a placeholder referencing an existing secret handle.
func (u *Steps) changesProxyCredentialToPlaceholder(ctx context.Context, handle string) error {
	return u.changesProxyCredential(ctx, secretPlaceholder(handle))
}

// proxyAuthValue decodes the credential value a proxy create/update request body carries,
// out from under whatever JSON string-escaping the raw body uses.
//
// Two shapes describe a proxy's providers and a request carries one of them: the list,
// where the credential belongs to the entry marked primary, or the single `provider` a
// client written before the list still sends. A reader that knows only one shape reports a
// proxy with no credential at all rather than saying it could not find one.
//
// Which shape is present decides where to look, and the other is not consulted. A request
// that sends the list has said where its credential lives; reading a legacy `provider`
// alongside it would let a list with no credential on its primary pass on the strength of
// a field the application no longer sends — the assertion would hold while the thing it
// asserts had stopped being true.
func proxyAuthValue(call recordedCall) (string, error) {
	type auth struct {
		Value string `json:"value"`
	}
	var body struct {
		Providers []struct {
			IsPrimary bool  `json:"isPrimary"`
			Auth      *auth `json:"auth"`
		} `json:"providers"`
		Provider *struct {
			Auth *auth `json:"auth"`
		} `json:"provider"`
	}
	if err := json.Unmarshal([]byte(call.body), &body); err != nil {
		return "", fmt.Errorf("parsing the request body: %w", err)
	}
	if len(body.Providers) > 0 {
		for _, entry := range body.Providers {
			if entry.IsPrimary {
				if entry.Auth == nil {
					return "", fmt.Errorf("the provider list carries no credential on its primary entry")
				}
				return entry.Auth.Value, nil
			}
		}
		return "", fmt.Errorf("the provider list names no primary entry")
	}
	if body.Provider != nil && body.Provider.Auth != nil {
		return body.Provider.Auth.Value, nil
	}
	return "", nil
}

// proxyCallCarriesAPlaceholder asserts the most recent request of the given method carries
// a secret placeholder and never the plaintext credential.
func (u *Steps) proxyCallCarriesAPlaceholder(ctx context.Context, method, plaintext string) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	call, err := t.awaitProxyCall(ctx, method)
	if err != nil {
		return fmt.Errorf("no %s request to /llm-proxies was recorded: %w", method, err)
	}
	authValue, err := proxyAuthValue(call)
	if err != nil {
		return err
	}
	if !strings.Contains(authValue, `{{ secret "`) {
		return fmt.Errorf("the credential carries no secret placeholder")
	}
	if strings.Contains(authValue, plaintext) {
		return fmt.Errorf("the credential still carries the plaintext value")
	}
	return nil
}

// proxyCallCarriesPlaceholderFor asserts the most recent request of the given method
// carries a placeholder referencing the given secret handle.
func (u *Steps) proxyCallCarriesPlaceholderFor(ctx context.Context, method, handle string) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	call, err := t.awaitProxyCall(ctx, method)
	if err != nil {
		return fmt.Errorf("no %s request to /llm-proxies was recorded: %w", method, err)
	}
	authValue, err := proxyAuthValue(call)
	if err != nil {
		return err
	}
	if !strings.Contains(authValue, secretPlaceholder(handle)) {
		return fmt.Errorf("the credential does not reference secret %q", handle)
	}
	return nil
}

func (u *Steps) theProxyWasCreatedWithAPlaceholder(ctx context.Context, plaintext string) error {
	return u.proxyCallCarriesAPlaceholder(ctx, "POST", plaintext)
}

func (u *Steps) theProxyWasCreatedWithThePlaceholderReferencing(ctx context.Context, handle string) error {
	return u.proxyCallCarriesPlaceholderFor(ctx, "POST", handle)
}

func (u *Steps) theProxyWasUpdatedWithAPlaceholder(ctx context.Context, plaintext string) error {
	return u.proxyCallCarriesAPlaceholder(ctx, "PUT", plaintext)
}

func (u *Steps) theProxyWasUpdatedWithThePlaceholderReferencing(ctx context.Context, handle string) error {
	return u.proxyCallCarriesPlaceholderFor(ctx, "PUT", handle)
}

func (u *Steps) theProxyWasNotCreated(ctx context.Context) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	if n := t.proxyCount("POST"); n != 0 {
		return fmt.Errorf("expected no proxy creation request, got %d", n)
	}
	return nil
}

func (u *Steps) theProxyWasNotUpdated(ctx context.Context) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	if n := t.proxyCount("PUT"); n != 0 {
		return fmt.Errorf("expected no proxy update request, got %d", n)
	}
	return nil
}

func (u *Steps) theProxyWasUpdated(ctx context.Context) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	call, err := t.awaitProxyCall(ctx, "PUT")
	if err != nil {
		return fmt.Errorf("no PUT request to /llm-proxies was recorded: %w", err)
	}
	if call.status != 200 {
		return fmt.Errorf("the proxy update returned %d", call.status)
	}
	return nil
}

const keyOriginalSecretHandle = "uiOriginalSecretHandle"

// theCurrentSecretIsRememberedAsTheOriginal snapshots the most recently created secret's
// handle into a key later actions in the scenario won't overwrite, so a subsequent
// credential rotation can still be checked against it.
func (u *Steps) theCurrentSecretIsRememberedAsTheOriginal(ctx context.Context) error {
	handle, ok := tcontext.Get(ctx, keyLastSecretHandle)
	if !ok {
		return fmt.Errorf("no secret handle has been recorded yet")
	}
	return tcontext.Set(ctx, keyOriginalSecretHandle, handle)
}

// theOriginalSecretIsNowDeprecated asserts the secret remembered by
// theCurrentSecretIsRememberedAsTheOriginal is still resolvable but marked DEPRECATED —
// the soft-delete a credential rotation leaves behind. Cleanup runs server-side after the
// rotation succeeds and is not awaited by the browser, so this polls rather than reading
// the secret once.
func (u *Steps) theOriginalSecretIsNowDeprecated(ctx context.Context) error {
	v, ok := tcontext.Get(ctx, keyOriginalSecretHandle)
	if !ok {
		return fmt.Errorf("no original secret handle was remembered in this scenario")
	}
	handle, _ := v.(string)
	_, err := retry.Until(ctx, retry.Options{},
		func(context.Context) (string, error) {
			body, err := u.fetchSecretDirectly(ctx, handle)
			if err != nil {
				return "", retry.Transient(err)
			}
			var parsed struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal([]byte(body), &parsed); err != nil {
				return "", fmt.Errorf("parsing the secret response: %w", err)
			}
			if parsed.Status != "DEPRECATED" {
				return "", retry.Transient(fmt.Errorf("secret status is %q, not yet DEPRECATED", parsed.Status))
			}
			return parsed.Status, nil
		},
		func(string) bool { return true },
	)
	return err
}
