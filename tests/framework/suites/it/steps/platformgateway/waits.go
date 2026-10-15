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

package platformgateway

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// Fail-fast waits. An observation either satisfies the wait, is one of the wait's listed
// in-between states, or contradicts it. Only the listed states are polled through; anything
// else ends the wait at once with what was observed.

// tolerated reports an in-between state the wait keeps polling through.
func tolerated(format string, args ...any) error {
	return retry.Transient(fmt.Errorf(format, args...))
}

// awaitState polls observe until it returns nil. A tolerated error keeps polling until the
// propagation ceiling and is logged, so in-between states stay countable; any other error
// fails the wait immediately.
func awaitState(ctx context.Context, what string, observe func(context.Context) error) error {
	return awaitStateWith(ctx, retry.Options{}, what, observe)
}

// awaitReadState is awaitState for a wait whose polls only read logs, listings, metrics or the
// listener's handshake, so it polls at retry.FastInterval.
func awaitReadState(ctx context.Context, what string, observe func(context.Context) error) error {
	return awaitStateWith(ctx, retry.Options{Fast: true}, what, observe)
}

func awaitStateWith(ctx context.Context, opts retry.Options, what string, observe func(context.Context) error) error {
	return retry.Await(ctx, opts,
		func(ctx context.Context) (bool, error) {
			if err := observe(ctx); err != nil {
				if retry.IsTransient(err) {
					slog.Info("wait tolerated an in-between state", "wait", what, "observed", err.Error())
				}
				return false, err
			}
			return true, nil
		},
		func(done bool) bool { return done },
		what)
}

// describe renders an observed response for a fail-fast or timeout message.
func describe(resp *httpx.Response) string {
	if resp == nil {
		return "no response"
	}
	return fmt.Sprintf("%s (upstream service time %q)", resp.Describe(), resp.Headers.Get(headerUpstreamServiceTime))
}

// headerUpstreamServiceTime is set by Envoy on every response an upstream produced and never
// on a reply Envoy generated itself, such as a request that matched no route.
const headerUpstreamServiceTime = "X-Envoy-Upstream-Service-Time"

// fromUpstream reports whether a response came from the upstream rather than from Envoy.
func fromUpstream(resp *httpx.Response) bool {
	return resp.Headers.Get(headerUpstreamServiceTime) != ""
}

// noRouteBody is the body of the gateway's own reply to a request that matched no route.
const noRouteBody = `{"error":"Not Found"}`

// policyChainMissingBody is the body of the policy engine's 500 for a route whose policy chain
// it has not received yet.
const policyChainMissingBody = `{"error":"Internal Server Error"}`

// routeInBetween names the replies a data-plane route gives while it is not live yet: the
// gateway's own no-route 404 before the route exists, the policy engine's 500 until it holds
// the route's policy chain (wso2/api-platform#3621) and, for a route that forwards, Envoy's
// own 503 while the upstream cluster warms.
func routeInBetween(resp *httpx.Response, allowWarmingUpstream bool) (string, bool) {
	if fromUpstream(resp) {
		return "", false
	}
	switch {
	case resp.StatusCode == http.StatusNotFound && strings.TrimSpace(resp.Text()) == noRouteBody:
		return "Envoy has no route yet", true
	case resp.StatusCode == http.StatusInternalServerError && strings.TrimSpace(resp.Text()) == policyChainMissingBody:
		return "the policy engine has no policy chain for the route yet", true
	case resp.StatusCode == http.StatusServiceUnavailable && allowWarmingUpstream:
		return "the upstream cluster is still warming", true
	default:
		return "", false
	}
}

// sendUntilRouteAnswers polls a data-plane path until its route is live and answers want.
// Waiting for 200 tolerates Envoy's no-route 404, the policy engine's missing-chain 500 and Envoy's
// 503 while the upstream warms. Waiting for 401, the rejection an authenticating policy gives, or
// for 503, the answer of a route whose upstream cannot be reached, tolerates only the first two,
// so a 200 fails at once. Any other answer fails at once too.
func (g *Gateway) sendUntilRouteAnswers(ctx context.Context, method, path string, want int) error {
	if want != http.StatusOK && want != http.StatusUnauthorized && want != http.StatusServiceUnavailable {
		return fmt.Errorf("waiting for a route supports status 200, 401 or 503, not %d", want)
	}
	resolved, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	url, err := g.gatewayURL(resolved)
	if err != nil {
		return err
	}
	method = strings.ToUpper(method)
	headers := g.scenarioHeaders(ctx)
	return awaitState(ctx, fmt.Sprintf("waiting for %s %s to answer %d", method, url, want),
		func(ctx context.Context) error {
			resp, sendErr := g.funnel.Send(ctx, httpx.Request{Method: method, URL: url, Headers: headers, Host: g.requestHost(ctx)})
			if sendErr != nil {
				return tolerated("the gateway did not answer: %v", sendErr)
			}
			if resp.StatusCode == want {
				return nil
			}
			if state, ok := routeInBetween(resp, want == http.StatusOK); ok {
				return tolerated("%s: %s", state, describe(resp))
			}
			return fmt.Errorf("expected %d once the route is live, got %s", want, describe(resp))
		})
}

func (g *Gateway) registerWaitSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" until the route answers (\d+)$`, g.sendUntilRouteAnswers)
}
