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
	"fmt"

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const keyPlatformAPIToken = "uiPlatformAPIToken"

// platformAPIBaseURL is platform-api's own address, resolved on the block's network.
func (u *UI) platformAPIBaseURL() (string, error) {
	inst, err := u.topo.Component("platform-api")
	if err != nil {
		return "", err
	}
	return inst.InternalURL("https")
}

// platformAPIToken returns a bearer token for the fixed admin identity, authenticating
// once per scenario and caching the result.
func (u *UI) platformAPIToken(ctx context.Context) (string, error) {
	if v, ok := tcontext.Get(ctx, keyPlatformAPIToken); ok {
		if token, ok := v.(string); ok && token != "" {
			return token, nil
		}
	}
	page, err := u.page(ctx)
	if err != nil {
		return "", err
	}
	base, err := u.platformAPIBaseURL()
	if err != nil {
		return "", err
	}
	resp, err := page.Context().Request().Post(base+"/api/portal/v0.9/auth/login",
		playwright.APIRequestContextPostOptions{
			Form:              map[string]any{"username": u.topo.Admin.Username, "password": u.topo.Admin.Password},
			IgnoreHttpsErrors: playwright.Bool(true),
		})
	if err != nil {
		return "", fmt.Errorf("authenticating against platform-api: %w", err)
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := resp.JSON(&body); err != nil {
		return "", fmt.Errorf("reading the platform-api login response: %w", err)
	}
	if body.Token == "" {
		return "", fmt.Errorf("platform-api login returned no token")
	}
	if err := tcontext.Set(ctx, keyPlatformAPIToken, body.Token); err != nil {
		return "", err
	}
	return body.Token, nil
}

// createSecretDirectly stores a secret through platform-api's own API, bypassing the UI.
func (u *UI) createSecretDirectly(ctx context.Context, handle, value string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	base, err := u.platformAPIBaseURL()
	if err != nil {
		return err
	}
	token, err := u.platformAPIToken(ctx)
	if err != nil {
		return err
	}
	resp, err := page.Context().Request().Post(base+"/api/v0.9/secrets",
		playwright.APIRequestContextPostOptions{
			Headers:           map[string]string{"Authorization": "Bearer " + token},
			Form:              map[string]any{"id": handle, "displayName": handle, "value": value, "type": "GENERIC"},
			IgnoreHttpsErrors: playwright.Bool(true),
		})
	if err != nil {
		return fmt.Errorf("creating secret %q: %w", handle, err)
	}
	if status := resp.Status(); status != 200 && status != 201 && status != 409 {
		body, _ := resp.Text()
		return fmt.Errorf("creating secret %q returned %d: %s", handle, status, body)
	}
	return cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindSecret, ID: handle, Actor: u.topo.Admin.Username,
	})
}

// deletesProviderDirectly removes a provider through platform-api's own API, bypassing the
// UI — for tearing down a provider mid-scenario without navigating back to it.
func (u *UI) deletesProviderDirectly(ctx context.Context, name string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	base, err := u.platformAPIBaseURL()
	if err != nil {
		return err
	}
	token, err := u.platformAPIToken(ctx)
	if err != nil {
		return err
	}
	id := toProviderID(name)
	resp, err := page.Context().Request().Delete(base+"/api/v0.9/llm-providers/"+id,
		playwright.APIRequestContextDeleteOptions{
			Headers:           map[string]string{"Authorization": "Bearer " + token},
			IgnoreHttpsErrors: playwright.Bool(true),
		})
	if err != nil {
		return fmt.Errorf("deleting provider %q: %w", name, err)
	}
	if status := resp.Status(); status != 200 && status != 204 && status != 404 {
		body, _ := resp.Text()
		return fmt.Errorf("deleting provider %q returned %d: %s", name, status, body)
	}
	reg, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	reg.Deregister(cleanup.KindLLMProvider, id)
	return nil
}

// fetchSecretDirectly reads a secret's stored fields through platform-api's own API and
// returns the raw response body.
func (u *UI) fetchSecretDirectly(ctx context.Context, handle string) (string, error) {
	page, err := u.page(ctx)
	if err != nil {
		return "", err
	}
	base, err := u.platformAPIBaseURL()
	if err != nil {
		return "", err
	}
	token, err := u.platformAPIToken(ctx)
	if err != nil {
		return "", err
	}
	resp, err := page.Context().Request().Get(base+"/api/v0.9/secrets/"+handle,
		playwright.APIRequestContextGetOptions{
			Headers:           map[string]string{"Authorization": "Bearer " + token},
			IgnoreHttpsErrors: playwright.Bool(true),
		})
	if err != nil {
		return "", fmt.Errorf("fetching secret %q: %w", handle, err)
	}
	body, err := resp.Text()
	if err != nil {
		return "", fmt.Errorf("reading the response for secret %q: %w", handle, err)
	}
	if resp.Status() != 200 {
		return "", fmt.Errorf("fetching secret %q returned %d: %s", handle, resp.Status(), body)
	}
	return body, nil
}
