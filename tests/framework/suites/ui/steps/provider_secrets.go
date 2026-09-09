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
	"sync"

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// secretPlaceholder builds the reference string a credential is replaced with once its
// value has been stored as a secret.
func secretPlaceholder(handle string) string {
	return fmt.Sprintf(`{{ secret "%s" }}`, handle)
}

// recordedCall is one finished request to an endpoint this file tracks.
type recordedCall struct {
	method   string
	body     string
	status   int
	response string
}

// callTracker groups the /secrets, /llm-providers, /llm-proxies, /mcp-proxies, and
// /mcp-proxies/fetch-server-info requests made during a scenario action, so later steps
// can assert on them without their own network listener.
type callTracker struct {
	mu              sync.Mutex
	secrets         []recordedCall
	providers       []recordedCall
	proxies         []recordedCall
	mcpProxies      []recordedCall
	fetchServerInfo []recordedCall
}

// record files req into secrets, providers, proxies, mcpProxies, or fetchServerInfo if it
// matches one, ignoring every other request the page makes.
func (t *callTracker) record(req playwright.Request) {
	url := req.URL()
	method := req.Method()
	isSecret := strings.Contains(url, "/secrets") && method == "POST"
	isProvider := strings.Contains(url, "/llm-providers") && (method == "POST" || method == "PUT")
	isProxy := strings.Contains(url, "/llm-proxies") && (method == "POST" || method == "PUT")
	isFetchServerInfo := strings.Contains(url, "/fetch-server-info") && method == "POST"
	// /mcp-proxies/fetch-server-info is a validation sub-endpoint under the same collection
	// path, not a create/update call, and must never be counted as one.
	isMCPProxy := strings.Contains(url, "/mcp-proxies") && !isFetchServerInfo &&
		(method == "POST" || method == "PUT")
	if !isSecret && !isProvider && !isProxy && !isMCPProxy && !isFetchServerInfo {
		return
	}
	body, _ := req.PostData()
	var status int
	var response string
	if resp, err := req.Response(); err == nil && resp != nil {
		status = resp.Status()
		response, _ = resp.Text()
	}
	call := recordedCall{method: method, body: body, status: status, response: response}
	t.mu.Lock()
	defer t.mu.Unlock()
	switch {
	case isSecret:
		t.secrets = append(t.secrets, call)
	case isProvider:
		t.providers = append(t.providers, call)
	case isProxy:
		t.proxies = append(t.proxies, call)
	case isMCPProxy:
		t.mcpProxies = append(t.mcpProxies, call)
	case isFetchServerInfo:
		t.fetchServerInfo = append(t.fetchServerInfo, call)
	}
}

func (t *callTracker) secretCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.secrets)
}

// lastSecretHandle returns the id of the most recently created secret, if any.
func (t *callTracker) lastSecretHandle() (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.secrets) == 0 {
		return "", false
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(t.secrets[len(t.secrets)-1].response), &body); err != nil || body.ID == "" {
		return "", false
	}
	return body.ID, true
}

func countOfMethod(calls []recordedCall, method string) int {
	n := 0
	for _, c := range calls {
		if c.method == method {
			n++
		}
	}
	return n
}

// lastCallOfMethod returns the most recent call of the given method, if any.
func lastCallOfMethod(calls []recordedCall, method string) (recordedCall, bool) {
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i].method == method {
			return calls[i], true
		}
	}
	return recordedCall{}, false
}

func (t *callTracker) providerCount(method string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return countOfMethod(t.providers, method)
}

func (t *callTracker) lastProviderCall(method string) (recordedCall, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return lastCallOfMethod(t.providers, method)
}

func (t *callTracker) proxyCount(method string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return countOfMethod(t.proxies, method)
}

func (t *callTracker) lastProxyCall(method string) (recordedCall, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return lastCallOfMethod(t.proxies, method)
}

func (t *callTracker) mcpProxyCount(method string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return countOfMethod(t.mcpProxies, method)
}

func (t *callTracker) lastMCPProxyCall(method string) (recordedCall, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return lastCallOfMethod(t.mcpProxies, method)
}

// lastFetchServerInfoCall returns the most recent fetch-server-info request, if any. There
// is only ever one caller per scenario action, so no method filter is needed.
func (t *callTracker) lastFetchServerInfoCall() (recordedCall, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.fetchServerInfo) == 0 {
		return recordedCall{}, false
	}
	return t.fetchServerInfo[len(t.fetchServerInfo)-1], true
}

// awaitCall retries getLast until it finds a match. The browser's own page state (a URL
// change, a visible page element) can settle before this suite's network listener has
// finished processing the request behind it, so callers needing a recorded call wait here
// rather than reading the tracker immediately.
func awaitCall(ctx context.Context, what string, getLast func() (recordedCall, bool)) (recordedCall, error) {
	return retry.Until(ctx, retry.Options{},
		func(context.Context) (recordedCall, error) {
			if call, ok := getLast(); ok {
				return call, nil
			}
			return recordedCall{}, retry.Transient(fmt.Errorf("no %s recorded yet", what))
		},
		func(recordedCall) bool { return true },
	)
}

func (t *callTracker) awaitProviderCall(ctx context.Context, method string) (recordedCall, error) {
	return awaitCall(ctx, fmt.Sprintf("%s request to /llm-providers", method),
		func() (recordedCall, bool) { return t.lastProviderCall(method) })
}

func (t *callTracker) awaitProxyCall(ctx context.Context, method string) (recordedCall, error) {
	return awaitCall(ctx, fmt.Sprintf("%s request to /llm-proxies", method),
		func() (recordedCall, bool) { return t.lastProxyCall(method) })
}

func (t *callTracker) awaitMCPProxyCall(ctx context.Context, method string) (recordedCall, error) {
	return awaitCall(ctx, fmt.Sprintf("%s request to /mcp-proxies", method),
		func() (recordedCall, bool) { return t.lastMCPProxyCall(method) })
}

func (t *callTracker) awaitFetchServerInfoCall(ctx context.Context) (recordedCall, error) {
	return awaitCall(ctx, "POST request to /mcp-proxies/fetch-server-info",
		func() (recordedCall, bool) { return t.lastFetchServerInfoCall() })
}

const keyCallTracker = "uiCallTracker"

type callTrackerState struct {
	tracker *callTracker
	handler func(playwright.Request)
}

// watchSecretAndProviderCalls starts recording every /secrets, /llm-providers, and
// /llm-proxies request for the rest of the scenario, replacing any tracker from an
// earlier action.
func (u *UI) watchSecretAndProviderCalls(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if v, ok := tcontext.Get(ctx, keyCallTracker); ok {
		if previous, ok := v.(*callTrackerState); ok {
			page.RemoveListener("requestfinished", previous.handler)
		}
	}
	tracker := &callTracker{}
	handler := tracker.record
	page.OnRequestFinished(handler)
	return tcontext.Set(ctx, keyCallTracker, &callTrackerState{tracker: tracker, handler: handler})
}

func (u *UI) tracker(ctx context.Context) (*callTracker, error) {
	v, ok := tcontext.Get(ctx, keyCallTracker)
	if !ok {
		return nil, fmt.Errorf("no network calls have been recorded in this scenario")
	}
	state, ok := v.(*callTrackerState)
	if !ok {
		return nil, fmt.Errorf("recorded calls are stored as %T", v)
	}
	return state.tracker, nil
}

// --- provider create/update actions ---

// submitsProviderWithCredential fills and submits the OpenAI template's provider form with
// an explicit credential, recording the /secrets and /llm-providers calls it makes.
func (u *UI) submitsProviderWithCredential(ctx context.Context, name, credential string) error {
	if credential != "" {
		if err := markSensitiveArtifacts(ctx); err != nil {
			return err
		}
	}
	if err := u.watchSecretAndProviderCalls(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := fillCyidInput(page, "provider-name-input", name); err != nil {
		return err
	}
	if err := fillCyidTextarea(page, "provider-description-input",
		"UI suite provider for credential secrecy checks."); err != nil {
		return err
	}
	if err := fillCyidInput(page, "provider-api-key-input", credential); err != nil {
		return err
	}
	if err := cyid(page, "add-provider-button").Click(); err != nil {
		return fmt.Errorf("submitting the provider: %w", err)
	}
	// Registered optimistically: a submission the backend rejects registers a provider that
	// never existed, and the deleter treats that as already-gone rather than an error.
	return cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindLLMProvider, ID: toProviderID(name), Actor: u.topo.Admin.Username,
	})
}

// submitsProviderWithCredentialPlaceholder is submitsProviderWithCredential, with the
// credential built as a placeholder referencing an existing secret handle.
func (u *UI) submitsProviderWithCredentialPlaceholder(ctx context.Context, name, handle string) error {
	return u.submitsProviderWithCredential(ctx, name, secretPlaceholder(handle))
}

// credentialField locates the Connection tab's credential input by its label, the same way
// the label is not otherwise associated with the field in the DOM.
func credentialField(page playwright.Page) playwright.Locator {
	return page.Locator("label").Filter(playwright.LocatorFilterOptions{HasText: "Credentials"}).
		Locator("xpath=..").Locator("input")
}

// opensProviderConnectionTab switches the provider overview to its Connection tab.
func (u *UI) opensProviderConnectionTab(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("tab", playwright.PageGetByRoleOptions{Name: "Connection"}).Click(); err != nil {
		return fmt.Errorf("opening the Connection tab: %w", err)
	}
	return u.expect.Locator(page.GetByText("Credentials").First()).ToBeVisible()
}

// opensProviderFromList navigates to the provider list, reloading so a list already
// fetched earlier in this scenario reflects the provider just created, searches for it so
// it is not lost among others the org has accumulated, and opens its card.
func (u *UI) opensProviderFromList(ctx context.Context, name string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := cyid(page, "nav-service-provider").Click(); err != nil {
		return fmt.Errorf("opening Service Provider: %w", err)
	}
	if _, err := page.Reload(); err != nil {
		return fmt.Errorf("reloading the provider list: %w", err)
	}
	if err := fillCyidInput(page, "provider-search-input", name); err != nil {
		return err
	}
	if err := cyid(page, "provider-card-"+toProviderID(name)).Click(); err != nil {
		return fmt.Errorf("opening the provider card for %q: %w", name, err)
	}
	return nil
}

// changesProviderCredential clears the masked credential field, types a new value, and
// clicks the page's Save button — editing stages the change locally; Save is what
// persists it, recording the calls the save makes.
func (u *UI) changesProviderCredential(ctx context.Context, newValue string) error {
	if err := markSensitiveArtifacts(ctx); err != nil {
		return err
	}
	if err := u.watchSecretAndProviderCalls(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	field := credentialField(page)
	if err := field.Click(); err != nil {
		return fmt.Errorf("focusing the credentials field: %w", err)
	}
	if err := u.expect.Locator(field).ToHaveValue(""); err != nil {
		return fmt.Errorf("the masked credential never cleared: %w", err)
	}
	if err := field.Fill(newValue); err != nil {
		return fmt.Errorf("typing the new credential: %w", err)
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Save"}).Click(); err != nil {
		return fmt.Errorf("clicking Save: %w", err)
	}
	return nil
}

// changesProviderCredentialToPlaceholder is changesProviderCredential, with the new value
// built as a placeholder referencing an existing secret handle.
func (u *UI) changesProviderCredentialToPlaceholder(ctx context.Context, handle string) error {
	return u.changesProviderCredential(ctx, secretPlaceholder(handle))
}

// secretCreationAlwaysFails stubs every secret-creation call for the rest of the scenario
// with a server error, so the caller's own handling of that failure can be exercised.
func (u *UI) secretCreationAlwaysFails(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return page.Route("**/secrets", func(route playwright.Route) {
		_ = route.Fulfill(playwright.RouteFulfillOptions{
			Status:      playwright.Int(500),
			ContentType: playwright.String("application/json"),
			Body:        `{"error":"simulated vault failure"}`,
		})
	})
}

// --- assertions on recorded calls ---

const keyLastSecretHandle = "uiLastSecretHandle"

// aSecretWasCreatedForThatCredential asserts exactly one secret was created, waiting for
// the network listener to catch up the same way awaitProviderCall/awaitProxyCall do, since
// this step is sometimes the first one to check the tracker after a page navigation.
func (u *UI) aSecretWasCreatedForThatCredential(ctx context.Context) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	if _, err := retry.Until(ctx, retry.Options{},
		func(context.Context) (struct{}, error) {
			switch n := t.secretCount(); {
			case n == 1:
				return struct{}{}, nil
			case n > 1:
				return struct{}{}, fmt.Errorf("expected exactly one secret to be created, got %d", n)
			default:
				return struct{}{}, retry.Transient(fmt.Errorf("no secret creation recorded yet"))
			}
		},
		func(struct{}) bool { return true },
	); err != nil {
		return err
	}
	if handle, ok := t.lastSecretHandle(); ok {
		if err := tcontext.Set(ctx, keyLastSecretHandle, handle); err != nil {
			return err
		}
		return cleanup.Register(ctx, cleanup.Resource{
			Kind: cleanup.KindSecret, ID: handle, Actor: u.topo.Admin.Username,
		})
	}
	return nil
}

func (u *UI) noSecretWasCreatedForThatCredential(ctx context.Context) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	if n := t.secretCount(); n != 0 {
		return fmt.Errorf("expected no secret to be created, got %d", n)
	}
	return nil
}

// providerAuthValue decodes the credential value a provider create/update request body
// carries, out from under whatever JSON string-escaping the raw body uses.
func providerAuthValue(call recordedCall) (string, error) {
	var body struct {
		Upstream struct {
			Main struct {
				Auth struct {
					Value string `json:"value"`
				} `json:"auth"`
			} `json:"main"`
		} `json:"upstream"`
	}
	if err := json.Unmarshal([]byte(call.body), &body); err != nil {
		return "", fmt.Errorf("parsing the request body: %w", err)
	}
	return body.Upstream.Main.Auth.Value, nil
}

// providerCallCarriesAPlaceholder asserts the most recent request of the given method
// carries a secret placeholder and never the plaintext credential.
func (u *UI) providerCallCarriesAPlaceholder(ctx context.Context, method, plaintext string) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	call, err := t.awaitProviderCall(ctx, method)
	if err != nil {
		return fmt.Errorf("no %s request to /llm-providers was recorded: %w", method, err)
	}
	authValue, err := providerAuthValue(call)
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

// providerCallCarriesPlaceholderFor asserts the most recent request of the given method
// carries a placeholder referencing the given secret handle.
func (u *UI) providerCallCarriesPlaceholderFor(ctx context.Context, method, handle string) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	call, err := t.awaitProviderCall(ctx, method)
	if err != nil {
		return fmt.Errorf("no %s request to /llm-providers was recorded: %w", method, err)
	}
	authValue, err := providerAuthValue(call)
	if err != nil {
		return err
	}
	if !strings.Contains(authValue, secretPlaceholder(handle)) {
		return fmt.Errorf("the credential does not reference the expected secret")
	}
	return nil
}

func (u *UI) theProviderWasCreatedWithAPlaceholder(ctx context.Context, plaintext string) error {
	return u.providerCallCarriesAPlaceholder(ctx, "POST", plaintext)
}

func (u *UI) theProviderWasCreatedWithThePlaceholderReferencing(ctx context.Context, handle string) error {
	return u.providerCallCarriesPlaceholderFor(ctx, "POST", handle)
}

func (u *UI) theProviderWasUpdatedWithAPlaceholder(ctx context.Context, plaintext string) error {
	return u.providerCallCarriesAPlaceholder(ctx, "PUT", plaintext)
}

func (u *UI) theProviderWasUpdatedWithThePlaceholderReferencing(ctx context.Context, handle string) error {
	return u.providerCallCarriesPlaceholderFor(ctx, "PUT", handle)
}

func (u *UI) theProviderWasNotCreated(ctx context.Context) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	if n := t.providerCount("POST"); n != 0 {
		return fmt.Errorf("expected no provider creation request, got %d", n)
	}
	return nil
}

func (u *UI) theProviderWasNotUpdated(ctx context.Context) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	if n := t.providerCount("PUT"); n != 0 {
		return fmt.Errorf("expected no provider update request, got %d", n)
	}
	return nil
}

func (u *UI) theProviderWasUpdated(ctx context.Context) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	call, err := t.awaitProviderCall(ctx, "PUT")
	if err != nil {
		return fmt.Errorf("no PUT request to /llm-providers was recorded: %w", err)
	}
	if call.status != 200 {
		return fmt.Errorf("the provider update returned %d", call.status)
	}
	return nil
}

// pageNeverShows asserts text does not appear anywhere in the page's rendered content.
func (u *UI) pageNeverShows(ctx context.Context, text string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	body, err := page.Locator("body").InnerText()
	if err != nil {
		return fmt.Errorf("reading the page body: %w", err)
	}
	if strings.Contains(body, text) {
		return fmt.Errorf("the page unexpectedly shows %q", text)
	}
	return nil
}

// seesAnErrorNotification asserts a snackbar notification is visible, regardless of the
// exact message it carries.
func (u *UI) seesAnErrorNotification(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(page.Locator(".MuiSnackbarContent-root").First()).ToBeVisible()
}

// --- direct-to-platform-api secret steps ---

func (u *UI) aSecretAlreadyHoldsTheValue(ctx context.Context, handle, value string) error {
	return u.createSecretDirectly(ctx, handle, value)
}

func (u *UI) fetchingTheSecretDirectlyReturnsNoPlaintextValue(ctx context.Context, handle string) error {
	body, err := u.fetchSecretDirectly(ctx, handle)
	if err != nil {
		return err
	}
	if strings.Contains(body, `"value"`) || strings.Contains(body, `"encryptedValue"`) {
		return fmt.Errorf("the secret response unexpectedly carries a value field")
	}
	return nil
}
