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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/testpki"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
	"github.com/wso2/api-platform/tests/framework/testbench/services/tlsbackend"
)

// roleDefault marks the one gateway identity presented to backends that name none.
const roleDefault = "default"

func (g *Gateway) registerDefaultIdentitySteps(sc *godog.ScenarioContext) {
	g.registerEchoBackendSteps(sc)
	sc.Step(`^the gateway identity "([^"]*)" is uploaded from fixture "([^"]*)"( with role default)?$`, g.storeGatewayIdentityWithRole)
	sc.Step(`^I upload the gateway identity "([^"]*)" from fixture "([^"]*)"( with role default)?$`, g.uploadGatewayIdentity)
	sc.Step(`^I rotate the gateway identity "([^"]*)" from fixture "([^"]*)"$`, g.rotateGatewayIdentity)
	sc.Step(`^I remove the gateway identity "([^"]*)"$`, g.deleteGatewayIdentity)
	sc.Step(`^the gateway identity listing should show role default only on "([^"]*)"$`, g.listingShowsDefaultOnly)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" until the backend sees (.+)$`, g.sendUntilBackendSees)
	sc.Step(`^I send (\d+) "([^"]*)" requests to "([^"]*)" and the backend sees (.+) in every response$`, g.sendHoldingBackendView)
}

// ── Gateway identities ─────────────────────────────────────────────────────────

// fixtureUpload describes one upload of a fixture to the certificate pool.
type fixtureUpload struct {
	fixture string
	name    string
	usage   string
	role    string
	withKey bool
}

func (g *Gateway) storeGatewayIdentityWithRole(ctx context.Context, name, fixture, role string) error {
	resp, err := g.uploadCertificate(ctx, fixtureUpload{
		fixture: fixture, name: name, usage: "identity", role: roleFromPhrase(role), withKey: true,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("expected gateway identity %q to be uploaded with status 201, got %s", name, resp.Describe())
	}
	return nil
}

func (g *Gateway) uploadGatewayIdentity(ctx context.Context, name, fixture, role string) error {
	_, err := g.uploadCertificate(ctx, fixtureUpload{
		fixture: fixture, name: name, usage: "identity", role: roleFromPhrase(role), withKey: true,
	})
	return err
}

func roleFromPhrase(phrase string) string {
	if strings.TrimSpace(phrase) == "" {
		return ""
	}
	return roleDefault
}

// fixtureChainPEM returns a fixture's certificate followed by its issuing intermediates.
func fixtureChainPEM(fixture *testpki.Fixture) string {
	chain := string(fixture.CertPEM)
	if len(fixture.ChainPEM) > 0 {
		chain += "\n" + string(fixture.ChainPEM)
	}
	return chain
}

// uploadCertificate uploads a fixture under a generated name through postCertificate.
func (g *Gateway) uploadCertificate(ctx context.Context, u fixtureUpload) (*httpx.Response, error) {
	name, err := g.generatedCertificateName(ctx, u.name)
	if err != nil {
		return nil, err
	}
	fixtures, err := testpki.Default()
	if err != nil {
		return nil, err
	}
	fixture, err := fixtures.Get(u.fixture)
	if err != nil {
		return nil, err
	}
	body := map[string]string{"name": name, "usage": u.usage, "certificate": fixtureChainPEM(fixture)}
	if u.role != "" {
		body["role"] = u.role
	}
	if u.withKey {
		body["privateKey"] = string(fixture.KeyPEM)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encoding the certificate upload: %w", err)
	}
	return g.postCertificate(ctx, name, payload)
}

// identityEntry is one gateway identity in the certificate listing.
type identityEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

func (g *Gateway) gatewayIdentities(ctx context.Context) ([]identityEntry, error) {
	var listing struct {
		Certificates []identityEntry `json:"certificates"`
	}
	if err := g.readJSON(ctx, "gateway-controller", "/certificates?usage=identity", &listing); err != nil {
		return nil, err
	}
	return listing.Certificates, nil
}

// gatewayIdentityID finds the id of the named gateway identity.
func (g *Gateway) gatewayIdentityID(ctx context.Context, nameExpr string) (string, error) {
	name, err := stepscommon.Expand(ctx, nameExpr)
	if err != nil {
		return "", err
	}
	entries, err := g.gatewayIdentities(ctx)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.Name == name {
			return e.ID, nil
		}
	}
	return "", fmt.Errorf("the gateway holds no identity named %q", name)
}

func (g *Gateway) rotateGatewayIdentity(ctx context.Context, nameExpr, fixtureName string) error {
	id, err := g.gatewayIdentityID(ctx, nameExpr)
	if err != nil {
		return err
	}
	fixtures, err := testpki.Default()
	if err != nil {
		return err
	}
	fixture, err := fixtures.Get(fixtureName)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{
		"certificate": fixtureChainPEM(fixture), "privateKey": string(fixture.KeyPEM),
	})
	if err != nil {
		return fmt.Errorf("encoding the identity rotation: %w", err)
	}
	endpoint, err := g.serviceURL(ctx, "gateway-controller", "/certificates/"+id)
	if err != nil {
		return err
	}
	markGatewayChanged(ctx)
	_, err = g.funnel.Put(ctx, endpoint, g.headerWith(ctx, "Content-Type", "application/json"), payload)
	return err
}

func (g *Gateway) deleteGatewayIdentity(ctx context.Context, nameExpr string) error {
	id, err := g.gatewayIdentityID(ctx, nameExpr)
	if err != nil {
		return err
	}
	endpoint, err := g.serviceURL(ctx, "gateway-controller", "/certificates/"+id)
	if err != nil {
		return err
	}
	markGatewayChanged(ctx)
	_, err = g.funnel.Delete(ctx, endpoint, g.scenarioHeaders(ctx))
	return err
}

// listingShowsDefaultOnly requires the named identity to carry role default and every other
// identity to carry none.
func (g *Gateway) listingShowsDefaultOnly(ctx context.Context, nameExpr string) error {
	name, err := stepscommon.Expand(ctx, nameExpr)
	if err != nil {
		return err
	}
	entries, err := g.gatewayIdentities(ctx)
	if err != nil {
		return err
	}
	found := false
	for _, e := range entries {
		switch {
		case e.Name == name && e.Role != roleDefault:
			return fmt.Errorf("gateway identity %q has role %q, expected %q", name, e.Role, roleDefault)
		case e.Name == name:
			found = true
		case e.Role != "":
			return fmt.Errorf("gateway identity %q has role %q, only %q should have one", e.Name, e.Role, name)
		}
	}
	if !found {
		return fmt.Errorf("the gateway holds no identity named %q", name)
	}
	return nil
}

// ── What a TLS backend sees ────────────────────────────────────────────────────

type viewKind int

const (
	viewNone viewKind = iota
	viewRefused
	viewSubject
)

// backendView is what a TLS echo backend reports about the client certificate of a request:
// a verified certificate's subject, none, or a refusal for the lack of a verified one.
type backendView struct {
	kind    viewKind
	subject string
}

func (v backendView) String() string {
	switch v.kind {
	case viewNone:
		return "no client certificate"
	case viewRefused:
		return "a refusal"
	default:
		return fmt.Sprintf("the client certificate %q", v.subject)
	}
}

var backendViewPattern = regexp.MustCompile(
	`^(?:(no client certificate)|(a refusal)|(the gateway listener certificate)|the client certificate of "([^"]+)")$`)

// parseBackendView reads a phrase naming what a backend sees. listenerSubject supplies the
// subject of the HTTPS listener certificate.
func parseBackendView(phrase string, listenerSubject func() (string, error)) (backendView, error) {
	m := backendViewPattern.FindStringSubmatch(strings.TrimSpace(phrase))
	switch {
	case m == nil:
		return backendView{}, fmt.Errorf("unknown backend view %q: expected no client certificate, a refusal, "+
			`the gateway listener certificate, or the client certificate of "<fixture>"`, phrase)
	case m[1] != "":
		return backendView{kind: viewNone}, nil
	case m[2] != "":
		return backendView{kind: viewRefused}, nil
	case m[3] != "":
		subject, err := listenerSubject()
		if err != nil {
			return backendView{}, err
		}
		return backendView{kind: viewSubject, subject: subject}, nil
	}
	fixtures, err := testpki.Default()
	if err != nil {
		return backendView{}, err
	}
	fixture, err := fixtures.Get(m[4])
	if err != nil {
		return backendView{}, err
	}
	return backendView{kind: viewSubject, subject: fixture.Certificate.Subject.String()}, nil
}

// listenerSubject reads the subject of the certificate the HTTPS listener serves.
func (g *Gateway) listenerSubject(ctx context.Context) (string, error) {
	base, err := g.topo.URL("platform-gateway", "https")
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parsing the HTTPS listener address %q: %w", base, err)
	}
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 10 * time.Second},
		// Only the certificate the listener serves is read, nothing is sent.
		Config: &tls.Config{ServerName: gatewaySNI, InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}, //nolint:gosec
	}
	conn, err := dialer.DialContext(ctx, "tcp", parsed.Host)
	if err != nil {
		return "", fmt.Errorf("reading the HTTPS listener certificate from %s: %w", parsed.Host, err)
	}
	defer conn.Close()
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return "", fmt.Errorf("the connection to %s is not a TLS connection", parsed.Host)
	}
	certificates := tlsConn.ConnectionState().PeerCertificates
	if len(certificates) == 0 {
		return "", fmt.Errorf("the HTTPS listener at %s served no certificate", parsed.Host)
	}
	return certificates[0].Subject.String(), nil
}

// backendViewOf reads what the backend reported in a response it produced.
func backendViewOf(resp *httpx.Response) (backendView, error) {
	switch resp.StatusCode {
	case http.StatusOK:
		if subject := resp.Headers.Get(tlsbackend.HeaderClientSubject); subject != "" {
			return backendView{kind: viewSubject, subject: subject}, nil
		}
		return backendView{kind: viewNone}, nil
	case http.StatusBadRequest:
		return backendView{kind: viewRefused}, nil
	default:
		return backendView{}, fmt.Errorf("the backend answered %d, which is neither a verdict on the client certificate nor a refusal", resp.StatusCode)
	}
}

// judgeBackendResponse classifies one response against what the wait expects. Replies the
// gateway generates while a route or its upstream cluster is not live yet, and the views in
// inBetween, are polled through; anything else ends the wait at once.
func judgeBackendResponse(resp *httpx.Response, want backendView, inBetween []backendView) error {
	if !fromUpstream(resp) {
		if state, ok := routeInBetween(resp, true); ok {
			return tolerated("%s: %s", state, describe(resp))
		}
		return fmt.Errorf("expected the request to reach the backend, got %s", describe(resp))
	}
	seen, err := backendViewOf(resp)
	if err != nil {
		return fmt.Errorf("%w: %s", err, describe(resp))
	}
	if seen == want {
		return nil
	}
	for _, accepted := range inBetween {
		if seen == accepted {
			return tolerated("the backend sees %s, expected %s", seen, want)
		}
	}
	return fmt.Errorf("the backend sees %s, expected %s: %s", seen, want, describe(resp))
}

// sendUntilBackendSees polls a data-plane path until the backend reports what the phrase
// names. The phrase may end in "while tolerating <view> or <view>", the views the backend
// reports while a change is still propagating.
func (g *Gateway) sendUntilBackendSees(ctx context.Context, method, path, phrase string) error {
	listener := func() (string, error) { return g.listenerSubject(ctx) }
	wantPhrase, tolerancePhrase, _ := strings.Cut(phrase, " while tolerating ")
	want, err := parseBackendView(wantPhrase, listener)
	if err != nil {
		return err
	}
	var inBetween []backendView
	if tolerancePhrase != "" {
		for _, p := range strings.Split(tolerancePhrase, " or ") {
			view, err := parseBackendView(p, listener)
			if err != nil {
				return err
			}
			inBetween = append(inBetween, view)
		}
	}
	resolved, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	endpoint, err := g.gatewayURL(resolved)
	if err != nil {
		return err
	}
	method = strings.ToUpper(method)
	// The backend closes each connection after answering, so every poll reaches it on a
	// connection the gateway dials with the certificate it holds at that moment.
	headers := g.scenarioHeaders(ctx)
	headers[tlsbackend.HeaderCloseConnection] = "true"
	return awaitState(ctx, fmt.Sprintf("waiting for %s %s to reach a backend that sees %s", method, endpoint, want),
		func(ctx context.Context) error {
			resp, sendErr := g.funnel.Send(ctx, httpx.Request{Method: method, URL: endpoint, Headers: headers, Host: g.requestHost(ctx)})
			if sendErr != nil {
				return tolerated("the gateway did not answer: %v", sendErr)
			}
			return judgeBackendResponse(resp, want, inBetween)
		})
}

// sendHoldingBackendView sends the request n times and requires every response to come from
// the backend and show the view the phrase names. Nothing is tolerated: the first response
// that differs fails, so a change that appears late is caught.
func (g *Gateway) sendHoldingBackendView(ctx context.Context, n int, method, path, phrase string) error {
	if n <= 0 {
		return fmt.Errorf("the request count must be positive, got %d", n)
	}
	want, err := parseBackendView(phrase, func() (string, error) { return g.listenerSubject(ctx) })
	if err != nil {
		return err
	}
	resolved, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	endpoint, err := g.gatewayURL(resolved)
	if err != nil {
		return err
	}
	method = strings.ToUpper(method)
	headers := g.scenarioHeaders(ctx)
	for i := 1; i <= n; i++ {
		resp, sendErr := g.funnel.Send(ctx, httpx.Request{Method: method, URL: endpoint, Headers: headers, Host: g.requestHost(ctx)})
		if sendErr != nil {
			return fmt.Errorf("request %d of %d to %s: %w", i, n, endpoint, sendErr)
		}
		if err := judgeBackendResponse(resp, want, nil); err != nil {
			return fmt.Errorf("request %d of %d: %s", i, n, err.Error())
		}
	}
	return nil
}
