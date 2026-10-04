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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
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
	_, _, token, err := u.platformAPI(ctx)
	if err != nil {
		return err
	}
	base, err := u.topo.URL("platform-api", "https")
	if err != nil {
		return err
	}
	client, err := newPlatformAPIReadinessClient()
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{}, func(context.Context) (bool, error) {
		resp, err := client.Do(ctx, httpx.Request{
			Method: http.MethodGet,
			URL:    base + "/api/v0.9/llm-providers",
			Headers: map[string]string{
				"Authorization": "Bearer " + token,
			},
		}, 0, 0)
		if err != nil {
			return false, retry.Transient(fmt.Errorf("reading providers: %w", err))
		}
		if resp.StatusCode >= 500 {
			return false, retry.Transient(fmt.Errorf("provider list returned HTTP %d", resp.StatusCode))
		}
		if resp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("provider list returned HTTP %d", resp.StatusCode)
		}
		var body providerListResponse
		if err := json.Unmarshal(resp.Body, &body); err != nil {
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

func newPlatformAPIReadinessClient() (*httpx.Client, error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil || rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	if ok := rootCAs.AppendCertsFromPEM(shared.ControlPlaneCrypto()["certs/cert.pem"]); !ok {
		return nil, fmt.Errorf("loading the generated Platform API CA certificate")
	}
	return httpx.NewClient(httpx.Options{
		TLSClientConfig: &tls.Config{RootCAs: rootCAs, ServerName: "platform-api"},
	}), nil
}
