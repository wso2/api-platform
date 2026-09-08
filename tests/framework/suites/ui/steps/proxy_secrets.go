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

package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// opensProxyProviderTab switches the proxy overview to its Provider tab.
func (u *UI) opensProxyProviderTab(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("tab", playwright.PageGetByRoleOptions{Name: "Provider"}).Click(); err != nil {
		return fmt.Errorf("opening the Provider tab: %w", err)
	}
	return u.expect.Locator(page.Locator(`input[placeholder="Enter API key"]`)).ToBeVisible()
}

// changesProxyCredential types a new value into the unmasked API key field and clicks the
// page's Save button — editing stages the change locally; Save is what persists it,
// recording the calls the save makes.
func (u *UI) changesProxyCredential(ctx context.Context, newValue string) error {
	if err := u.watchSecretAndProviderCalls(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator(`input[placeholder="Enter API key"]`).Fill(newValue); err != nil {
		return fmt.Errorf("typing the new credential: %w", err)
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Save"}).Click(); err != nil {
		return fmt.Errorf("clicking Save: %w", err)
	}
	return nil
}

// changesProxyCredentialToPlaceholder is changesProxyCredential, with the new value built
// as a placeholder referencing an existing secret handle.
func (u *UI) changesProxyCredentialToPlaceholder(ctx context.Context, handle string) error {
	return u.changesProxyCredential(ctx, secretPlaceholder(handle))
}

// proxyAuthValue decodes the credential value a proxy create/update request body carries,
// out from under whatever JSON string-escaping the raw body uses.
func proxyAuthValue(call recordedCall) (string, error) {
	var body struct {
		Provider struct {
			Auth struct {
				Value string `json:"value"`
			} `json:"auth"`
		} `json:"provider"`
	}
	if err := json.Unmarshal([]byte(call.body), &body); err != nil {
		return "", fmt.Errorf("parsing the request body: %w", err)
	}
	return body.Provider.Auth.Value, nil
}

// proxyCallCarriesAPlaceholder asserts the most recent request of the given method carries
// a secret placeholder and never the plaintext credential.
func (u *UI) proxyCallCarriesAPlaceholder(ctx context.Context, method, plaintext string) error {
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
		return fmt.Errorf("the credential carries no secret placeholder: %s", authValue)
	}
	if strings.Contains(authValue, plaintext) {
		return fmt.Errorf("the credential still carries the plaintext value")
	}
	return nil
}

// proxyCallCarriesPlaceholderFor asserts the most recent request of the given method
// carries a placeholder referencing the given secret handle.
func (u *UI) proxyCallCarriesPlaceholderFor(ctx context.Context, method, handle string) error {
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
		return fmt.Errorf("the credential does not reference secret %q: %s", handle, authValue)
	}
	return nil
}

func (u *UI) theProxyWasCreatedWithAPlaceholder(ctx context.Context, plaintext string) error {
	return u.proxyCallCarriesAPlaceholder(ctx, "POST", plaintext)
}

func (u *UI) theProxyWasCreatedWithThePlaceholderReferencing(ctx context.Context, handle string) error {
	return u.proxyCallCarriesPlaceholderFor(ctx, "POST", handle)
}

func (u *UI) theProxyWasUpdatedWithAPlaceholder(ctx context.Context, plaintext string) error {
	return u.proxyCallCarriesAPlaceholder(ctx, "PUT", plaintext)
}

func (u *UI) theProxyWasUpdatedWithThePlaceholderReferencing(ctx context.Context, handle string) error {
	return u.proxyCallCarriesPlaceholderFor(ctx, "PUT", handle)
}

func (u *UI) theProxyWasNotCreated(ctx context.Context) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	if n := t.proxyCount("POST"); n != 0 {
		return fmt.Errorf("expected no proxy creation request, got %d", n)
	}
	return nil
}

func (u *UI) theProxyWasNotUpdated(ctx context.Context) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	if n := t.proxyCount("PUT"); n != 0 {
		return fmt.Errorf("expected no proxy update request, got %d", n)
	}
	return nil
}

func (u *UI) theProxyWasUpdated(ctx context.Context) error {
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
func (u *UI) theCurrentSecretIsRememberedAsTheOriginal(ctx context.Context) error {
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
func (u *UI) theOriginalSecretIsNowDeprecated(ctx context.Context) error {
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
