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

package restapi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

const (
	redeployProbeInterval  = retry.BaseInterval
	redeployStatusInterval = 2 * time.Second
	redeployTolerableDrops = 1
)

// KeepServingUntilDeployed probes a data-plane URL for as long as the redeployment is in flight
// and fails on the first response that is not the expected status, or when more than
// redeployTolerableDrops requests in a row get no answer at all.
func (s *Steps) KeepServingUntilDeployed(ctx context.Context, method, target string, want int) error {
	method = strings.ToUpper(strings.TrimSpace(method))
	target, err := cloudbase.ExpandCloudValue(ctx, target)
	if err != nil {
		return err
	}
	target, err = s.ResolveDataPlaneURL(ctx, target)
	if err != nil {
		return err
	}
	var (
		status     string
		polledAt   time.Time
		unanswered int
	)
	last, err := retry.Until(ctx, retry.Options{Timeout: 3 * time.Minute, Interval: redeployProbeInterval},
		func(ctx context.Context) (string, error) {
			probe, probeErr := s.Client.Do(ctx, httpx.Request{Method: method, URL: target}, 0, 0)
			switch {
			case probeErr != nil:
				unanswered++
				if unanswered > redeployTolerableDrops {
					return "", fmt.Errorf("traffic was interrupted during the redeployment: %d requests in a row got no answer: %w",
						unanswered, probeErr)
				}
			case probe.StatusCode != want:
				return "", fmt.Errorf("traffic was interrupted during the redeployment: expected %d, got %s",
					want, probe.Describe())
			default:
				unanswered = 0
			}
			if status != "" && time.Since(polledAt) < redeployStatusInterval {
				return status, nil
			}
			current, statusErr := s.DeploymentStatus(ctx, cloudbase.PrimaryAPI)
			if statusErr != nil {
				return "", statusErr
			}
			status, polledAt = current, time.Now()
			return status, nil
		},
		func(status string) bool { return status == "DEPLOYED" })
	if err != nil {
		return err
	}
	if last != "DEPLOYED" {
		return fmt.Errorf("the redeployment never reached DEPLOYED; last status %q", last)
	}
	return nil
}

// CreateAPIKey issues an API key for the synthetic REST API and keeps its value in scenario state
// only; the response is not published so the key never reaches a step's response slot.
func (s *Steps) CreateAPIKey(ctx context.Context) error {
	apiID, err := cloudbase.RequiredValue(ctx, cloudbase.PrimaryAPI.ID)
	if err != nil {
		return err
	}
	resp, err := s.DoAuthenticatedRaw(ctx, http.MethodPost, "/rest-apis/"+url.PathEscape(apiID)+"/api-keys",
		map[string]string{"displayName": "Synthetic key for " + apiID})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return cloudbase.ResponseError(resp)
	}
	key, err := cloudbase.StringField(resp, "apiKey")
	if err != nil {
		return fmt.Errorf("creating an API key for %s: %w", apiID, err)
	}
	return tcontext.Set(ctx, cloudbase.KeyAPIKey, key)
}
