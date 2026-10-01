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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/testpki"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// encryptedKeyPassphrase protects the encrypted copy of every fixture key published to an
// @mtls scenario.
const encryptedKeyPassphrase = "test"

// tlsConnectFailureMarker opens the body Envoy answers with when it cannot establish the
// upstream connection.
const tlsConnectFailureMarker = "upstream connect error"

func (g *Gateway) registerOutboundMTLSSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I upload the gateway identity fixture "([^"]*)" with its chain as "([^"]*)"$`, g.uploadIdentityWithChain)
	sc.Step(`^I upload the gateway identity fixture "([^"]*)" as "([^"]*)"$`, g.uploadIdentity)
	sc.Step(`^the gateway identity fixture "([^"]*)" is stored as "([^"]*)"$`, g.storeIdentity)
	sc.Step(`^I upload to the certificates endpoint the identity body:$`, g.uploadIdentityBody)
	sc.Step(`^I delete the gateway identity named "([^"]*)"$`, g.deleteCertificateNamed)
	sc.Step(`^I update the gateway identity "([^"]*)" with the fixture "([^"]*)" and its chain$`, g.rotateIdentity)
	sc.Step(`^I update the certificate "([^"]*)" with the identity fixture "([^"]*)"$`, g.updateCertificateWithIdentity)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" until the backend refuses the request with 400$`, g.sendUntilBackendRefuses)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" until the upstream TLS connection fails with 503$`, g.sendUntilUpstreamTLSFails)
	sc.Step(`^the gateway has applied its configuration$`, g.awaitGatewayApplied)
}

// ── Gateway identities ───────────────────────────────────────────────────────────

// identityMaterial returns the fixture's certificate, followed by its issuing chain when
// withChain is set, and its key.
func identityMaterial(fixture *testpki.Fixture, withChain bool) (certificate, key string, err error) {
	pem := append([]byte{}, fixture.CertPEM...)
	if withChain {
		if len(fixture.ChainPEM) == 0 {
			return "", "", fmt.Errorf("fixture %q has no issuing chain", fixture.Name)
		}
		pem = append(pem, fixture.ChainPEM...)
	}
	return string(pem), string(fixture.KeyPEM), nil
}

func (g *Gateway) uploadIdentity(ctx context.Context, fixtureName, nameExpr string) error {
	_, err := g.uploadIdentityFixture(ctx, fixtureName, nameExpr, false)
	return err
}

func (g *Gateway) uploadIdentityWithChain(ctx context.Context, fixtureName, nameExpr string) error {
	_, err := g.uploadIdentityFixture(ctx, fixtureName, nameExpr, true)
	return err
}

// storeIdentity uploads an identity and requires the gateway to have created it.
func (g *Gateway) storeIdentity(ctx context.Context, fixtureName, nameExpr string) error {
	resp, err := g.uploadIdentityFixture(ctx, fixtureName, nameExpr, false)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("expected gateway identity %q to be stored with status 201, got %s", fixtureName, resp.Describe())
	}
	return nil
}

func (g *Gateway) uploadIdentityFixture(ctx context.Context, fixtureName, nameExpr string, withChain bool) (*httpx.Response, error) {
	name, err := g.generatedCertificateName(ctx, nameExpr)
	if err != nil {
		return nil, err
	}
	fixtures, err := testpki.Default()
	if err != nil {
		return nil, err
	}
	fixture, err := fixtures.Get(fixtureName)
	if err != nil {
		return nil, err
	}
	certificate, key, err := identityMaterial(fixture, withChain)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(map[string]string{
		"name": name, "usage": "identity", "certificate": certificate, "privateKey": key,
	})
	if err != nil {
		return nil, fmt.Errorf("encoding the identity upload: %w", err)
	}
	return g.postCertificate(ctx, name, payload)
}

// uploadIdentityBody posts a feature-written body, with fixture material embedded as context
// values, to the certificates endpoint.
func (g *Gateway) uploadIdentityBody(ctx context.Context, body *godog.DocString) error {
	expanded, err := stepscommon.Expand(ctx, body.Content)
	if err != nil {
		return err
	}
	var named struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal([]byte(expanded), &named) // an unparseable body is refused by the gateway, which the scenario asserts
	_, err = g.postCertificate(ctx, named.Name, []byte(expanded))
	return err
}

// rotateIdentity replaces the named identity's certificate chain and key in place.
func (g *Gateway) rotateIdentity(ctx context.Context, nameExpr, fixtureName string) error {
	return g.putIdentityShape(ctx, nameExpr, "identity", fixtureName, true)
}

// updateCertificateWithIdentity sends an identity-shaped update to a certificate that is not
// an identity.
func (g *Gateway) updateCertificateWithIdentity(ctx context.Context, nameExpr, fixtureName string) error {
	return g.putIdentityShape(ctx, nameExpr, "", fixtureName, false)
}

func (g *Gateway) putIdentityShape(ctx context.Context, nameExpr, usage, fixtureName string, withChain bool) error {
	name, err := stepscommon.Expand(ctx, nameExpr)
	if err != nil {
		return err
	}
	stored, err := g.certificateNamed(ctx, name)
	if err != nil {
		return fmt.Errorf("updating certificate %q: %w", name, err)
	}
	id := stored.ID
	fixtures, err := testpki.Default()
	if err != nil {
		return err
	}
	fixture, err := fixtures.Get(fixtureName)
	if err != nil {
		return err
	}
	certificate, key, err := identityMaterial(fixture, withChain)
	if err != nil {
		return err
	}
	// An update names neither the certificate nor its usage.
	payload, err := json.Marshal(map[string]string{"certificate": certificate, "privateKey": key})
	if err != nil {
		return fmt.Errorf("encoding the identity update: %w", err)
	}
	url, err := g.serviceURL(ctx, "gateway-controller", "/certificates/"+id)
	if err != nil {
		return err
	}
	markGatewayChanged(ctx)
	_, err = g.funnel.Put(ctx, url, g.headerWith(ctx, "Content-Type", "application/json"), payload)
	return err
}

// ── Waiting for a route to a TLS backend ─────────────────────────────────────────

// tlsConnectFailure reports whether Envoy itself answered 503 because it could not establish
// the upstream connection.
func tlsConnectFailure(resp *httpx.Response) bool {
	return resp.StatusCode == http.StatusServiceUnavailable && !fromUpstream(resp) &&
		strings.Contains(resp.Text(), tlsConnectFailureMarker)
}

// judgeBackendRefusal accepts the backend's own 400. The in-between states are the no-route
// 404 and Envoy's 503 while the upstream cluster warms; a failed upstream connection, or any
// other answer, ends the wait.
func judgeBackendRefusal(resp *httpx.Response) error {
	if resp.StatusCode == http.StatusBadRequest && fromUpstream(resp) {
		return nil
	}
	if tlsConnectFailure(resp) {
		return fmt.Errorf("expected the backend's 400 but the upstream TLS connection failed: %s", describe(resp))
	}
	if state, ok := routeInBetween(resp, true); ok {
		return tolerated("%s: %s", state, describe(resp))
	}
	return fmt.Errorf("expected the backend's 400 once the route is live, got %s", describe(resp))
}

// judgeUpstreamTLSFailure accepts Envoy's 503 for a failed upstream connection. The in-between
// states are the no-route 404 and Envoy's 503 while the upstream cluster warms; a reply from
// the backend, which means the connection succeeded, ends the wait.
func judgeUpstreamTLSFailure(resp *httpx.Response) error {
	if tlsConnectFailure(resp) {
		return nil
	}
	if state, ok := routeInBetween(resp, true); ok {
		return tolerated("%s: %s", state, describe(resp))
	}
	return fmt.Errorf("expected Envoy's 503 %q once the route is live, got %s", tlsConnectFailureMarker, describe(resp))
}

func (g *Gateway) sendUntilBackendRefuses(ctx context.Context, method, path string) error {
	return g.sendUntilJudged(ctx, method, path, "be refused by the backend with 400", judgeBackendRefusal)
}

func (g *Gateway) sendUntilUpstreamTLSFails(ctx context.Context, method, path string) error {
	return g.sendUntilJudged(ctx, method, path, "fail its upstream TLS connection with 503", judgeUpstreamTLSFailure)
}

// sendUntilJudged polls a data-plane path until judge accepts the answer, then waits for the
// gateway to hold what the controller holds, so the next request meets a settled gateway.
func (g *Gateway) sendUntilJudged(
	ctx context.Context, method, path, what string, judge func(*httpx.Response) error,
) error {
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
	if err := awaitState(ctx, fmt.Sprintf("waiting for %s %s to %s", method, url, what),
		func(ctx context.Context) error {
			resp, sendErr := g.funnel.Send(ctx, httpx.Request{Method: method, URL: url, Headers: headers, Host: g.requestHost(ctx)})
			if sendErr != nil {
				return tolerated("the gateway did not answer: %v", sendErr)
			}
			return judge(resp)
		}); err != nil {
		return err
	}
	return g.awaitGatewayApplied(ctx)
}
