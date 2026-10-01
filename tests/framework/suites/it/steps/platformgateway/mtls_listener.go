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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// listenerHoldWindow is how long, after the gateway has applied the client authority pool, the
// HTTPS listener must keep not asking for a client certificate. It covers one listener drain.
const listenerHoldWindow = 3 * time.Second

func (g *Gateway) registerMTLSListenerSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the certificate fixtures? "([^"]*)" (?:is|are) pooled as "([^"]*)"$`, g.poolCertificateFixturesWithoutUsage)
	sc.Step(`^the HTTPS listener should request a client certificate$`, g.listenerRequestsClientCertificate)
	sc.Step(`^the HTTPS listener should stop requesting a client certificate$`, g.listenerStopsRequestingClientCertificate)
	sc.Step(`^the HTTPS listener should not request a client certificate$`, g.listenerDoesNotRequestClientCertificate)
	sc.Step(`^the HTTPS listener should present the certificate in "([^"]*)"$`, g.listenerPresentsCertificateFile)
	sc.Step(`^the response should include a warning with code "([^"]*)" for field "([^"]*)"$`, responseWarnsForField)
	sc.Step(`^the response should include a warning with code "([^"]*)"$`,
		func(ctx context.Context, code string) error { return responseWarnsForField(ctx, code, "") })
	sc.Step(`^the response should include no warnings$`, responseHasNoWarnings)
}

// ── Pooling certificates without a usage, or with a role ───────────────────────

// poolCertificateFixturesWithoutUsage uploads fixtures with no usage, which the gateway keeps
// as backend trust, and requires 201.
func (g *Gateway) poolCertificateFixturesWithoutUsage(ctx context.Context, fixtureList, name string) error {
	return g.poolCertificateFixtures(ctx, fixtureList, name, "")
}

// ── Whether the HTTPS listener asks for a client certificate ─────────────────────

// listenerAddress returns the host:port of the gateway's HTTPS listener.
func (g *Gateway) listenerAddress() (string, error) {
	base, err := g.topo.URL("platform-gateway", "https")
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("the HTTPS endpoint %q has no host", base)
	}
	return parsed.Host, nil
}

// probeListener completes one handshake with the HTTPS listener, presenting no certificate.
// The listener validates optionally, so the handshake succeeds whether or not it asks. A
// listener that does not answer is tolerated, as while Envoy replaces it.
func (g *Gateway) probeListener(ctx context.Context) (*httpx.TLSState, error) {
	address, err := g.listenerAddress()
	if err != nil {
		return nil, err
	}
	// The listener serves a self-signed certificate; the probe observes the handshake only.
	state, err := g.funnel.Client().Handshake(ctx, address, &httpx.ClientTLS{ServerName: gatewaySNI, InsecureSkipVerify: true})
	if err != nil {
		return nil, tolerated("the HTTPS listener did not complete a handshake: %v", err)
	}
	return state, nil
}

// awaitListenerRequests waits until the listener asks for a client certificate exactly when
// want says; the other answer is the in-between state.
func (g *Gateway) awaitListenerRequests(ctx context.Context, want bool) error {
	return awaitReadState(ctx, fmt.Sprintf("waiting for the HTTPS listener to request a client certificate: %t", want),
		func(ctx context.Context) error {
			state, err := g.probeListener(ctx)
			if err != nil {
				return err
			}
			if state.ClientCertificateRequested != want {
				return tolerated("the HTTPS listener requests a client certificate: %t", state.ClientCertificateRequested)
			}
			return nil
		})
}

func (g *Gateway) listenerRequestsClientCertificate(ctx context.Context) error {
	return g.awaitListenerRequests(ctx, true)
}

func (g *Gateway) listenerStopsRequestingClientCertificate(ctx context.Context) error {
	return g.awaitListenerRequests(ctx, false)
}

// listenerDoesNotRequestClientCertificate settles an @mtls scenario's pending changes, then
// requires the listener not to ask for a client certificate for the whole hold window.
func (g *Gateway) listenerDoesNotRequestClientCertificate(ctx context.Context) error {
	if isMTLSScenario(ctx) {
		if err := g.awaitGatewayApplied(ctx); err != nil {
			return err
		}
	}
	_, err := retry.Never(ctx, retry.Options{Fast: true}, listenerHoldWindow, g.probeListener,
		func(state *httpx.TLSState) bool { return state.ClientCertificateRequested })
	if err != nil {
		return fmt.Errorf("the HTTPS listener was expected to not request a client certificate for %s: %w", listenerHoldWindow, err)
	}
	return nil
}

// listenerPresentsCertificateFile requires the listener's leaf certificate to be the
// certificate in a PEM file, named relative to the repository root.
func (g *Gateway) listenerPresentsCertificateFile(ctx context.Context, relative string) error {
	path, err := repositoryFile(relative)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading the expected listener certificate %q: %w", relative, err)
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return fmt.Errorf("%q does not start with a PEM certificate", relative)
	}
	want := sha256.Sum256(block.Bytes)
	if isMTLSScenario(ctx) {
		if err := g.awaitGatewayApplied(ctx); err != nil {
			return err
		}
	}
	state, err := g.probeListener(ctx)
	if err != nil {
		return err
	}
	if len(state.PeerCertificates) == 0 {
		return fmt.Errorf("the HTTPS listener presented no certificate")
	}
	got := sha256.Sum256(state.PeerCertificates[0].Raw)
	if got != want {
		return fmt.Errorf("the HTTPS listener presented %q (sha256 %s), expected the certificate in %q (sha256 %s)",
			state.PeerCertificates[0].Subject.String(), hex.EncodeToString(got[:]), relative, hex.EncodeToString(want[:]))
	}
	return nil
}

// repositoryFile resolves a repository-relative path and refuses one that leaves the checkout.
func repositoryFile(relative string) (string, error) {
	root, ok := shared.RepoRootFromCallerFile()
	if !ok {
		return "", fmt.Errorf("the repository root could not be located")
	}
	if relative == "" || filepath.IsAbs(relative) || strings.ContainsRune(relative, 0) {
		return "", fmt.Errorf("%q is not a repository-relative path", relative)
	}
	path := filepath.Clean(filepath.Join(root, filepath.FromSlash(relative)))
	if !strings.HasPrefix(path, filepath.Clean(root)+string(filepath.Separator)) {
		return "", fmt.Errorf("%q leaves the repository", relative)
	}
	return path, nil
}

// ── Deploy-response validation errors and warnings ────────────────────────────

// fieldMessage is one validation error or warning of a management API response.
type fieldMessage struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// responseWarnings reads the warnings of the published response. A status.warnings list,
// including an empty one, is that list. Top-level warnings are used only when status.warnings
// is absent or null, or when status is not an object (a certificate response carries the
// string "success" there).
func responseWarnings(ctx context.Context) ([]fieldMessage, error) {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return nil, err
	}
	var body struct {
		Warnings json.RawMessage `json:"warnings"`
		Status   json.RawMessage `json:"status"`
	}
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, fmt.Errorf("the response is not JSON while reading its warnings: %w (%s)", err, resp.Describe())
	}
	var status struct {
		Warnings json.RawMessage `json:"warnings"`
	}
	if len(body.Status) > 0 && body.Status[0] == '{' {
		if err := json.Unmarshal(body.Status, &status); err != nil {
			return nil, fmt.Errorf("the response status is malformed: %w (%s)", err, resp.Describe())
		}
	}
	if status.Warnings != nil && string(status.Warnings) != "null" {
		var warnings []fieldMessage
		if err := json.Unmarshal(status.Warnings, &warnings); err != nil {
			return nil, fmt.Errorf("status.warnings is not a list: %w (%s)", err, resp.Describe())
		}
		return warnings, nil
	}
	if body.Warnings != nil && string(body.Warnings) != "null" {
		var warnings []fieldMessage
		if err := json.Unmarshal(body.Warnings, &warnings); err != nil {
			return nil, fmt.Errorf("the response warnings are not a list: %w (%s)", err, resp.Describe())
		}
		return warnings, nil
	}
	return nil, nil
}

// responseWarnsForField requires a warning with code, for field unless field is empty.
func responseWarnsForField(ctx context.Context, code, field string) error {
	warnings, err := responseWarnings(ctx)
	if err != nil {
		return err
	}
	wantField, err := stepscommon.Expand(ctx, field)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		if w.Code == code && (wantField == "" || w.Field == wantField) {
			return nil
		}
	}
	if wantField == "" {
		return fmt.Errorf("no warning with code %q in %+v", code, warnings)
	}
	return fmt.Errorf("no warning with code %q for field %q in %+v", code, wantField, warnings)
}

func responseHasNoWarnings(ctx context.Context) error {
	warnings, err := responseWarnings(ctx)
	if err != nil {
		return err
	}
	if len(warnings) != 0 {
		return fmt.Errorf("expected no warnings, got %+v", warnings)
	}
	return nil
}
