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

package deploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

// TryUndeployAPI asks to stop the synthetic REST API's deployment on the active gateway, naming
// the given environment or else the gateway's own, and leaves the answer for the scenario.
func (s *Steps) TryUndeployAPI(ctx context.Context, environment string) error {
	expanded, err := cloudbase.ExpandCloudValue(ctx, environment)
	if err != nil {
		return err
	}
	_, err = s.SendUndeploy(ctx, cloudbase.PrimaryAPI, strings.TrimSpace(expanded))
	return err
}

// KeepReturningFor sends a data-plane request for the whole window and fails on the first answer
// that is not the expected status, proving the gateway keeps answering it that way over time.
func (s *Steps) KeepReturningFor(ctx context.Context, method, target string, want, seconds int) error {
	if seconds < 1 {
		return fmt.Errorf("a window of %d seconds is too short to observe", seconds)
	}
	request, err := s.PrepareDataPlaneRequest(ctx, httpx.Request{Method: method, URL: target})
	if err != nil {
		return err
	}
	// A probe that gets no answer is not the expected status, so it fails the window too.
	noRetry := retry.Options{Retryable: func(error) bool { return false }}
	last, err := retry.Never(ctx, noRetry, time.Duration(seconds)*time.Second,
		func(ctx context.Context) (*httpx.Response, error) {
			return s.Funnel.Send(ctx, request)
		},
		func(response *httpx.Response) bool { return response.StatusCode != want })
	if err != nil {
		if last != nil {
			return fmt.Errorf("%s %s did not keep returning %d: %w (last %s)", request.Method, request.URL, want, err, last.Describe())
		}
		return fmt.Errorf("%s %s did not keep returning %d: %w", request.Method, request.URL, want, err)
	}
	return nil
}
