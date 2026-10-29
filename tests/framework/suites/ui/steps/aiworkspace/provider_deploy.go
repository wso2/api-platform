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
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// keyLatestInvokeURL keeps the provider's invoke URL once the overview has shown it: the
// overview stops rendering it when the provider is no longer deployed, but a journey still
// calls it then to see the call fail.
const keyLatestInvokeURL = "uiLatestInvokeURL"

// keyedMockPath is the testbench route that accepts only its valid upstream key.
const keyedMockPath = "/openai-keyed/v1"

// mockEndpoint resolves a named testbench upstream on the block's network: the plain
// OpenAI-dialect mock, its keyed variant, or its Mistral-dialect route.
func (u *Steps) mockEndpoint(name string) (string, error) {
	plain, err := u.mockLLMURL()
	if err != nil {
		return "", err
	}
	base := strings.TrimSuffix(plain, "/openai/v1")
	switch name {
	case "mock LLM":
		return plain, nil
	case "keyed mock LLM":
		return base + keyedMockPath, nil
	case "Mistral mock LLM":
		return base + "/mistral/v1", nil
	}
	return "", fmt.Errorf("unknown mock endpoint %q", name)
}

// createsProviderAt is createProvider for a named mock upstream.
func (u *Steps) createsProviderAt(ctx context.Context, name, endpoint, apiKey string) error {
	upstream, err := u.mockEndpoint(endpoint)
	if err != nil {
		return err
	}
	return u.submitProviderFormForTemplate(ctx, name, &upstream, apiKey, "")
}

// changesProviderEndpoint replaces the Connection tab's endpoint with a named mock upstream
// and leaves the field, which stages the change for the overview's Save.
func (u *Steps) changesProviderEndpoint(ctx context.Context, endpoint string) error {
	url, err := u.mockEndpoint(endpoint)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	field := page.Locator("label").Filter(playwright.LocatorFilterOptions{HasText: regexp.MustCompile(`^Provider Endpoint`)}).
		Locator("xpath=..").Locator("input")
	if err := field.Fill(url); err != nil {
		return fmt.Errorf("changing the endpoint: %w", err)
	}
	return field.Press("Tab")
}

// seesDeploymentStatus checks the deploy screen's "Deployment Status" row, exactly: the
// gateway's own connection chip also reads "Active", and "Active" must not match "Not Active".
func (u *Steps) seesDeploymentStatus(ctx context.Context, status string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	row := page.GetByText("Deployment Status").First().Locator("xpath=..")
	label := row.GetByText(status, playwright.LocatorGetByTextOptions{Exact: playwright.Bool(true)})
	if err := u.expect.Locator(label).ToBeVisible(); err != nil {
		return fmt.Errorf("the deployment is not shown as %q: %w", status, err)
	}
	return nil
}

// opensDeployScreen opens the provider's deploy screen from its overview without deploying.
func (u *Steps) opensDeployScreen(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator("button, a").Filter(playwright.LocatorFilterOptions{
		HasText: regexp.MustCompile(`Deploy to Gateway`),
	}).First().Click(); err != nil {
		return fmt.Errorf("opening the deploy screen: %w", err)
	}
	return nil
}

// stopsDeployment presses Stop on the deploy screen's current deployment.
func (u *Steps) stopsDeployment(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Stop", Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("stopping the deployment: %w", err)
	}
	return nil
}

// redeploysStoppedDeployment presses Redeploy on the deploy screen's stopped deployment.
func (u *Steps) redeploysStoppedDeployment(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Redeploy", Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("redeploying the stopped deployment: %w", err)
	}
	return nil
}

// invokeURLField is the overview's read-only invoke URL: the input whose box carries the
// "Copy URL" button. Matching any input that holds a URL would also match the Connection
// tab's endpoint field and call the upstream directly, bypassing the gateway.
func invokeURLField(page playwright.Page) playwright.Locator {
	return page.Locator(".MuiInputBase-root").Filter(playwright.LocatorFilterOptions{
		Has: page.Locator(`button[aria-label="Copy URL"]`),
	}).Locator("input").First()
}

// invokeURL is the provider's invoke URL as the overview shows it, or the one it showed
// last when it no longer does.
func (u *Steps) invokeURL(ctx context.Context) (string, error) {
	page, err := u.page(ctx)
	if err != nil {
		return "", err
	}
	field := invokeURLField(page)
	if visible, _ := field.IsVisible(); visible {
		url, err := field.InputValue()
		if err != nil {
			return "", fmt.Errorf("reading the invoke URL: %w", err)
		}
		return url, tcontext.Set(ctx, keyLatestInvokeURL, url)
	}
	v, _ := tcontext.Get(ctx, keyLatestInvokeURL)
	if url, ok := v.(string); ok && url != "" {
		return url, nil
	}
	return "", fmt.Errorf("the overview shows no invoke URL and none was seen earlier")
}

// callsProvider sends a chat completion with the user's message and the generated key, and
// retries until the gateway answers with want: a deployment change reaches the gateway a
// moment after the control plane reports it.
func (u *Steps) callsProvider(ctx context.Context, streaming, message string, want int) error {
	return u.callProviderUntil(ctx, streaming != "", message, fmt.Sprintf("HTTP %d", want),
		func(inv invocation) bool { return inv.status == want })
}

// callsProviderUntilAnswering is callsProvider retried until the response carries text,
// whatever its status: an upstream's refusal is an answer too.
func (u *Steps) callsProviderUntilAnswering(ctx context.Context, message, text string) error {
	return u.callProviderUntil(ctx, false, message, fmt.Sprintf("an answer containing %q", text),
		func(inv invocation) bool { return strings.Contains(inv.body, text) })
}

func (u *Steps) callReturned(ctx context.Context, want int) error {
	inv, err := latestInvocation(ctx)
	if err != nil {
		return err
	}
	if inv.status != want {
		return fmt.Errorf("the call returned HTTP %d, want %d: %s", inv.status, want, inv.body)
	}
	return nil
}

func (u *Steps) callProviderUntil(ctx context.Context, streaming bool, message, want string, done func(invocation) bool) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	key, err := u.latestAPIKey(ctx)
	if err != nil {
		return err
	}
	header, err := u.latestAPIKeyHeader(ctx)
	if err != nil {
		return err
	}
	url, err := u.invokeURL(ctx)
	if err != nil {
		return err
	}
	payload := map[string]any{"messages": []map[string]string{{"role": "user", "content": message}}}
	if streaming {
		payload["stream"] = true
	}
	var last invocation
	inv, err := retry.Until(ctx, retry.Options{}, func(context.Context) (invocation, error) {
		resp, err := page.Context().Request().Post(url+"/chat/completions",
			playwright.APIRequestContextPostOptions{Data: payload, Headers: map[string]string{header: key}})
		if err != nil {
			return invocation{}, retry.Transient(err)
		}
		body, _ := resp.Body()
		last = invocation{status: resp.Status(), body: string(body)}
		return last, nil
	}, done)
	if err == nil && !done(inv) {
		// Until returns a nil error when no attempt met the condition.
		err = errors.New("the condition never held")
	}
	if err != nil {
		return fmt.Errorf("calling %s never gave %s (last: HTTP %d %s): %w", url, want, last.status, last.body, err)
	}
	return tcontext.Set(ctx, keyInvocation, inv)
}

// callsProviderUntilLimited keeps calling the provider until a call is refused with 429. A
// limit set on a redeployed provider applies once the new deployment reaches the gateway,
// so the calls continue, within the polling deadline, until it does.
func (u *Steps) callsProviderUntilLimited(ctx context.Context) error {
	return u.callProviderUntil(ctx, false, "Hello", "HTTP 429",
		func(inv invocation) bool { return inv.status == 429 })
}

func latestInvocation(ctx context.Context) (invocation, error) {
	v, ok := tcontext.Get(ctx, keyInvocation)
	inv, isInv := v.(invocation)
	if !ok || !isInv {
		return invocation{}, fmt.Errorf("no call to the provider was made in this scenario")
	}
	return inv, nil
}

func (u *Steps) callResponseContains(ctx context.Context, negated, text string) error {
	inv, err := latestInvocation(ctx)
	if err != nil {
		return err
	}
	if contains := strings.Contains(inv.body, text); contains == (negated != "") {
		if negated != "" {
			return fmt.Errorf("the response unexpectedly contains %q: %s", text, inv.body)
		}
		return fmt.Errorf("the response does not contain %q: %s", text, inv.body)
	}
	return nil
}

// callAnswers checks the chat completion's answer and the model that gave it.
func (u *Steps) callAnswers(ctx context.Context, table *godog.Table) error {
	inv, err := latestInvocation(ctx)
	if err != nil {
		return err
	}
	var completion struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(inv.body), &completion); err != nil || len(completion.Choices) == 0 {
		return fmt.Errorf("the response is not a chat completion: %s", inv.body)
	}
	got := map[string]string{"answer": completion.Choices[0].Message.Content, "model": completion.Model}
	for _, row := range table.Rows {
		field, want := row.Cells[0].Value, row.Cells[1].Value
		value, known := got[field]
		if !known {
			return fmt.Errorf("unknown chat completion field %q", field)
		}
		if value != want {
			return fmt.Errorf("the completion's %s is %q, want %q", field, value, want)
		}
	}
	return nil
}

// callStreamedInOrder reassembles the streamed chunks' content and checks it reads as want,
// and that the stream ended with the [DONE] sentinel.
func (u *Steps) callStreamedInOrder(ctx context.Context, want string) error {
	inv, err := latestInvocation(ctx)
	if err != nil {
		return err
	}
	var text strings.Builder
	var events []string
	scanner := bufio.NewScanner(strings.NewReader(inv.body))
	for scanner.Scan() {
		data, ok := strings.CutPrefix(scanner.Text(), "data: ")
		if !ok {
			continue
		}
		events = append(events, data)
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if data != "[DONE]" && json.Unmarshal([]byte(data), &chunk) == nil && len(chunk.Choices) > 0 {
			text.WriteString(chunk.Choices[0].Delta.Content)
		}
	}
	if len(events) < 3 {
		return fmt.Errorf("the response is not a chunked stream: %s", inv.body)
	}
	if events[len(events)-1] != "[DONE]" {
		return fmt.Errorf("the stream did not end with [DONE]: last event %q", events[len(events)-1])
	}
	if text.String() != want {
		return fmt.Errorf("the streamed chunks read %q, want %q", text.String(), want)
	}
	return nil
}

// editsGuardrail opens the named guardrail's pill, changes its parameters and updates it.
func (u *Steps) editsGuardrail(ctx context.Context, name string, table *godog.Table) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByText(name, playwright.PageGetByTextOptions{Exact: playwright.Bool(true)}).First().Click(); err != nil {
		return fmt.Errorf("opening the %q guardrail: %w", name, err)
	}
	drawer := page.Locator(".MuiDrawer-paper")
	expanded := map[string]bool{}
	for _, row := range table.Rows {
		path, value := row.Cells[0].Value, row.Cells[1].Value
		field := drawer.Locator(`[data-testid="field-` + path + `"]`)
		if section, _, nested := strings.Cut(path, "."); nested && !expanded[section] {
			if visible, _ := field.IsVisible(); !visible {
				label := drawer.GetByText(section, playwright.LocatorGetByTextOptions{Exact: playwright.Bool(true)}).First()
				if err := label.Locator("xpath=../../..").Locator("svg").First().Click(); err != nil {
					return fmt.Errorf("expanding the %q parameters: %w", section, err)
				}
			}
			expanded[section] = true
		}
		if err := field.Fill(value); err != nil {
			return fmt.Errorf("setting the %q parameter: %w", path, err)
		}
	}
	if err := drawer.Locator(`[data-testid="policy-param-submit"]`).Click(); err != nil {
		return fmt.Errorf("updating the %q guardrail: %w", name, err)
	}
	return u.expect.Locator(drawer).ToBeHidden()
}

func (u *Steps) removesGuardrail(ctx context.Context, name string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	pill := page.Locator(`[role="button"], button`).Filter(playwright.LocatorFilterOptions{HasText: name})
	if err := pill.Locator(`button[aria-label="Remove guardrail"]`).First().Click(); err != nil {
		return fmt.Errorf("removing the %q guardrail: %w", name, err)
	}
	return nil
}
