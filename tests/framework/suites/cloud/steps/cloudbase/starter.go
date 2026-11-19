/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudbase

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
)

// GatewayReadyTimeout bounds how long the starter waits for a gateway it created, or one that is
// still being built, to become active. The platform builds a gateway through its own control plane,
// which takes minutes; the wait never gets less than five.
const GatewayReadyTimeout = 15 * time.Minute

// GatewayPollInterval is how often the starter checks a gateway that is being built.
const GatewayPollInterval = 10 * time.Second

// GatewayStatusActive is the status of a gateway that is connected and serves traffic.
const GatewayStatusActive = "ACTIVE"

// OrganisationAPI sends authenticated requests to the platform's API for one organisation.
type OrganisationAPI struct {
	Client *httpx.Client
	Base   string
	Token  string
	// pollInterval is how often a gateway that is being built is checked; zero means
	// gatewayPollInterval.
	PollInterval time.Duration
}

// Do sends a request to the organisation's platform API with the run's token.
func (p OrganisationAPI) Do(ctx context.Context, method, path string, body []byte) (*httpx.Response, error) {
	return p.Client.Do(ctx, httpx.Request{
		Method: method,
		URL:    CloudAPIURL(p.Base, path),
		Headers: map[string]string{
			"Authorization": "Bearer " + p.Token,
			"Content-Type":  "application/json",
		},
		Body: body,
	}, 0, 0)
}

// Get reads a path of the organisation's platform API with the run's token.
func (p OrganisationAPI) Get(ctx context.Context, path string) (*httpx.Response, error) {
	return p.Do(ctx, http.MethodGet, path, nil)
}

// AwaitPortalActive waits until the managed API Portal reports active. A portal that reports
// failed stops the wait at once, since it will not recover.
func AwaitPortalActive(ctx context.Context, platform OrganisationAPI, id string) error {
	interval := platform.PollInterval
	if interval <= 0 {
		interval = PortalPollInterval
	}
	return retry.Await(ctx, retry.Options{Timeout: PortalBuildTimeout, Interval: interval},
		func(ctx context.Context) (string, error) {
			resp, err := platform.Get(ctx, ManagedPortalsCollection+"/"+url.PathEscape(id))
			if err != nil {
				return "", retry.Transient(err)
			}
			if resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusNotFound {
				return "", retry.Transient(ResponseError(resp))
			}
			if resp.StatusCode != http.StatusOK {
				return "", ResponseError(resp)
			}
			status, err := StringField(resp, "status")
			if err != nil {
				return "", err
			}
			if status == PortalStatusFailed {
				return "", fmt.Errorf("managed API Portal %q failed to provision", id)
			}
			return status, nil
		},
		func(status string) bool { return status == PortalStatusActive },
		"waiting for managed API Portal "+id+" to become active")
}
