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

// Assertions about the gateway enforcing a configured timeout.

package steps

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
)

// elapsedTolerance allows an elapsed-time assertion to land up to 5% under the configured
// timeout. Proportional rather than absolute, so it scales with the value being asserted.
const elapsedTolerance = 0.05

func (g *Gateway) registerTimeoutSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the gateway should have timed out after "([^"]*)" seconds with status (\d+)$`,
		g.timedOutAfter)
	sc.Step(`^the gateway should have responded within "([^"]*)" seconds$`, g.respondedWithin)
}

// timedOutAfter asserts the gateway waited the configured timeout before giving up.
//
// A LOWER bound on the duration is the whole assertion: an upstream that refuses instantly
// produces the same status as one that timed out correctly, so without it the test proves
// nothing about the timeout. No upper bound, because a loaded runner can add arbitrary delay
// and failing on that would be a flake reported as a product bug.
func (g *Gateway) timedOutAfter(ctx context.Context, wantSeconds string, wantStatus int) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	want, err := parseSeconds(wantSeconds)
	if err != nil {
		return err
	}

	floor := time.Duration(want * (1 - elapsedTolerance) * float64(time.Second))
	if resp.Elapsed < floor {
		return fmt.Errorf(
			"expected the gateway to take at least %ss (tolerance %.0f%%, so >= %s) before "+
				"timing out, but %s returned after %s",
			wantSeconds, elapsedTolerance*100, floor.Round(time.Millisecond),
			resp.Describe(), resp.Elapsed.Round(time.Millisecond))
	}
	if resp.StatusCode != wantStatus {
		return fmt.Errorf("expected the timeout to surface as status %d, got %s",
			wantStatus, resp.Describe())
	}
	return nil
}

// respondedWithin asserts an UPPER bound, for a scenario proving the shorter of two configured
// timeouts is the one applied.
func (g *Gateway) respondedWithin(ctx context.Context, wantSeconds string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	want, err := parseSeconds(wantSeconds)
	if err != nil {
		return err
	}

	ceiling := time.Duration(want * (1 + elapsedTolerance) * float64(time.Second))
	if resp.Elapsed > ceiling {
		return fmt.Errorf(
			"expected a response within %ss (tolerance %.0f%%, so <= %s), but %s took %s",
			wantSeconds, elapsedTolerance*100, ceiling.Round(time.Millisecond),
			resp.Describe(), resp.Elapsed.Round(time.Millisecond))
	}
	return nil
}

func parseSeconds(value string) (float64, error) {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, fmt.Errorf("parsing expected seconds %q: %w", value, err)
	}
	return seconds, nil
}
