/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.  You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package aiworkspace

import (
	"context"
	"fmt"

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/util/retry"
)

type providerListEntry struct {
	ID       string `json:"id"`
	Template string `json:"template"`
}

type providerListResponse struct {
	List []providerListEntry `json:"list"`
}

// waitForProviderTemplateAssociation waits for the committed provider list to expose the
// exact provider/template relationship returned by creation. A successful POST and a browser
// redirect are not sufficient: the template-delete guard reads this list independently.
func (u *Steps) waitForProviderTemplateAssociation(ctx context.Context, providerID, templateID string) error {
	page, base, token, err := u.platformAPI(ctx)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{}, func(context.Context) (bool, error) {
		resp, err := page.Context().Request().Get(base+"/api/v0.9/llm-providers",
			playwright.APIRequestContextGetOptions{
				Headers:           map[string]string{"Authorization": "Bearer " + token},
				IgnoreHttpsErrors: playwright.Bool(true),
			})
		if err != nil {
			return false, retry.Transient(fmt.Errorf("reading providers: %w", err))
		}
		if resp.Status() >= 500 {
			return false, retry.Transient(fmt.Errorf("provider list returned HTTP %d", resp.Status()))
		}
		if resp.Status() != 200 {
			return false, fmt.Errorf("provider list returned HTTP %d", resp.Status())
		}
		var body providerListResponse
		if err := resp.JSON(&body); err != nil {
			return false, fmt.Errorf("decoding provider list: %w", err)
		}
		for _, provider := range body.List {
			if provider.ID == providerID && provider.Template == templateID {
				return true, nil
			}
		}
		return false, nil
	}, func(found bool) bool { return found },
		fmt.Sprintf("waiting for provider %q to reference template %q", providerID, templateID))
}
