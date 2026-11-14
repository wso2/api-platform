/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package platformgateway

import (
	"context"
	"fmt"
	"strconv"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
)

// attemptCountHeader is the header Envoy sets when a virtual host enables
// include_attempt_count_in_response.
const attemptCountHeader = "x-envoy-attempt-count"

// responseReportsOneUpstreamAttempt asserts the request reached an upstream exactly once -- that
// no failover occurred -- WITHOUT requiring the attempt-count header to be absent.
//
// Absence is not assertable here, and that is a property of Envoy rather than of any policy.
// include_attempt_count_in_response is set per VIRTUAL HOST, so the header appears on every
// response in that vhost as soon as ANY route in it fails over. A step demanding the header be
// missing therefore asserts which OTHER APIs happen to be deployed at that moment, not anything
// about the operation under test: it passes when its feature runs alone and fails as soon as a
// sibling runner deploys a failover-bearing API, with no defect in either. That is not
// hypothetical -- it is how the equivalent assertion behaved in the suite this was ported from,
// and the new framework runs the gateway block at parallel: 12, so sibling runners share a
// gateway by design.
//
// One attempt is the real invariant, and an absent header reads as one.
func (g *Gateway) responseReportsOneUpstreamAttempt(ctx context.Context) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	attempts, err := upstreamAttempts(resp)
	if err != nil {
		return err
	}
	if attempts != 1 {
		return fmt.Errorf("the request was attempted %d times, want exactly 1 -- a failover "+
			"occurred where the policy was expected to be inert (%s)", attempts, resp.Describe())
	}
	return nil
}

// upstreamAttempts reads the attempt count off a response, treating an absent header as one
// attempt: the header only exists once the vhost has enabled it, and a response without it
// cannot have been retried.
func upstreamAttempts(resp *httpx.Response) (int, error) {
	raw := resp.Headers.Get(attemptCountHeader)
	if raw == "" {
		return 1, nil
	}
	attempts, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("header %s is %q, which is not a number (%s)",
			attemptCountHeader, raw, resp.Describe())
	}
	return attempts, nil
}
