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
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// Scenario-scoped state of the hostname steps, reset by the scenario hook.
const (
	keyHTTPSServerName     = "mtlsHTTPSServerName"
	keyKeptAliveConnection = "mtlsKeptAliveConnection"
)

const (
	// notAskedHold is how long a hostname must stay unasked once it first is not. It covers one
	// listener drain.
	notAskedHold = 3 * time.Second
	// notAskedInterval is the gap between handshakes while a hostname must stay unasked.
	notAskedInterval = 500 * time.Millisecond
	// keptAliveTimeout bounds one request on a kept-alive connection.
	keptAliveTimeout = 30 * time.Second
	// maxKeptAliveBody bounds the body read from a kept-alive connection.
	maxKeptAliveBody = 1 << 20
)

// serverNameChoice is the server name HTTPS requests send. The zero value keeps the default,
// gatewaySNI.
type serverNameChoice struct {
	name string
	none bool
}

// apply sets the choice on the client side of a handshake.
func (c serverNameChoice) apply(opts *httpx.ClientTLS) {
	switch {
	case c.none:
		opts.ServerName, opts.OmitServerName = "", true
	case c.name != "":
		opts.ServerName = c.name
	}
}

// label names the choice in a wait's description.
func (c serverNameChoice) label() string {
	if c.none {
		return "a connection with no server name"
	}
	return fmt.Sprintf("a connection with server name %q", c.name)
}

// serverNameChoiceOf reads the server name a step names, in upper case when asked to.
func serverNameChoiceOf(ctx context.Context, name, upper string) (serverNameChoice, error) {
	expanded, err := stepscommon.Expand(ctx, name)
	if err != nil {
		return serverNameChoice{}, err
	}
	expanded = strings.TrimSpace(expanded)
	if expanded == "" {
		return serverNameChoice{}, fmt.Errorf("server name %q expands to nothing; use the no server name step to send none", name)
	}
	if upper != "" {
		expanded = strings.ToUpper(expanded)
	}
	return serverNameChoice{name: expanded}, nil
}

func (g *Gateway) registerMTLSHostnameSteps(sc *godog.ScenarioContext) {
	sc.Before(resetHostnameState)
	sc.After(closeKeptAliveConnection)
	sc.Step(`^HTTPS requests send server name "([^"]*)"( in upper case)?$`, sendServerName)
	sc.Step(`^HTTPS requests send no server name$`, sendNoServerName)
	sc.Step(`^the HTTPS connection should (not )?have been asked for a client certificate$`, connectionAsked)
	sc.Step(`^the gateway should have resumed the TLS session$`, sessionResumed)
	sc.Step(`^a TLS connection (with server name "([^"]*)"( in upper case)?|with no server name) should (not )?be asked for a client certificate$`,
		g.handshakeAsked)
	sc.Step(`^the certificate fixture "([^"]*)" is pooled as "([^"]*)" as a relay$`, g.poolRelayFixture)
	sc.Step(`^I open a kept-alive HTTPS connection for "([^"]*)" and send a GET request to "([^"]*)"$`, g.openKeptAliveConnection)
	sc.Step(`^I send a GET request to "([^"]*)" on the kept-alive HTTPS connection$`, g.sendOnKeptAliveConnection)
	sc.Step(`^the kept-alive HTTPS connection should have carried (\d+) responses and not be closing$`, g.keptAliveConnectionStillOpen)
}

func resetHostnameState(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
	tcontext.Remove(ctx, keyHTTPSServerName)
	return ctx, nil
}

func sendServerName(ctx context.Context, name, upper string) error {
	choice, err := serverNameChoiceOf(ctx, name, upper)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, keyHTTPSServerName, choice)
}

func sendNoServerName(ctx context.Context) error {
	return tcontext.Set(ctx, keyHTTPSServerName, serverNameChoice{none: true})
}

// applyServerNameChoice sets the scenario's chosen server name on an HTTPS request's TLS.
func (g *Gateway) applyServerNameChoice(ctx context.Context, opts *httpx.ClientTLS) {
	if v, ok := tcontext.Get(ctx, keyHTTPSServerName); ok {
		if choice, isChoice := v.(serverNameChoice); isChoice {
			choice.apply(opts)
		}
	}
}

// ── What the last HTTPS request's handshake did ──────────────────────────────────

func publishedHandshake(ctx context.Context) (*httpx.TLSState, error) {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return nil, err
	}
	if resp.TLS == nil {
		return nil, fmt.Errorf("the last response did not arrive over an HTTPS request step: %s", resp.Describe())
	}
	return resp.TLS, nil
}

func connectionAsked(ctx context.Context, negate string) error {
	state, err := publishedHandshake(ctx)
	if err != nil {
		return err
	}
	switch {
	case negate == "" && !state.ClientCertificateRequested:
		return fmt.Errorf("the gateway did not ask the connection for a client certificate (server name %q)", state.ServerName)
	case negate != "" && state.ClientCertificateRequested:
		return fmt.Errorf("the gateway asked the connection for a client certificate (server name %q)", state.ServerName)
	}
	return nil
}

func sessionResumed(ctx context.Context) error {
	state, err := publishedHandshake(ctx)
	if err != nil {
		return err
	}
	if !state.DidResume {
		return fmt.Errorf("the gateway ran a full TLS handshake instead of resuming the cached TLS session")
	}
	return nil
}

// ── Which hostnames the listener asks ───────────────────────────────────────────

// handshakeAsked waits for the listener to ask, or to stop asking, connections that send the
// server name the phrase gives.
func (g *Gateway) handshakeAsked(ctx context.Context, phrase, name, upper, negate string) error {
	choice := serverNameChoice{none: strings.HasPrefix(phrase, "with no")}
	if !choice.none {
		var err error
		if choice, err = serverNameChoiceOf(ctx, name, upper); err != nil {
			return err
		}
	}
	if negate == "" {
		return g.awaitAsked(ctx, choice)
	}
	return g.awaitNotAsked(ctx, choice, notAskedHold)
}

// observeAsked opens one connection that sends the chosen server name and reports whether the
// listener sent a CertificateRequest. A listener that does not answer, as while Envoy swaps
// listeners, is tolerated.
func (g *Gateway) observeAsked(ctx context.Context, choice serverNameChoice) (bool, error) {
	base, err := g.topo.URL("platform-gateway", "https")
	if err != nil {
		return false, err
	}
	opts := &httpx.ClientTLS{InsecureSkipVerify: true}
	choice.apply(opts)
	resp, err := g.funnel.Client().Do(ctx, httpx.Request{Method: http.MethodGet, URL: base + "/", Host: opts.ServerName, TLS: opts}, 0, 0)
	if err != nil {
		return false, tolerated("the HTTPS listener did not complete a handshake for %s: %v", choice.label(), err)
	}
	if resp.TLS == nil {
		return false, fmt.Errorf("the handshake for %s reported no TLS state: %s", choice.label(), resp.Describe())
	}
	return resp.TLS.ClientCertificateRequested, nil
}

// awaitAsked waits until the listener asks connections of the chosen server name.
func (g *Gateway) awaitAsked(ctx context.Context, choice serverNameChoice) error {
	return awaitReadState(ctx, "waiting for the listener to ask "+choice.label()+" for a client certificate",
		func(ctx context.Context) error {
			asked, err := g.observeAsked(ctx, choice)
			if err != nil {
				return err
			}
			if !asked {
				return tolerated("the listener does not ask %s yet", choice.label())
			}
			return nil
		})
}

// awaitNotAsked waits until the listener does not ask connections of the chosen server name,
// then requires it to stay that way for the hold window, so a listener that flaps between
// states does not pass on one lucky sample.
func (g *Gateway) awaitNotAsked(ctx context.Context, choice serverNameChoice, hold time.Duration) error {
	err := awaitReadState(ctx, "waiting for the listener to stop asking "+choice.label()+" for a client certificate",
		func(ctx context.Context) error {
			asked, err := g.observeAsked(ctx, choice)
			if err != nil {
				return err
			}
			if asked {
				return tolerated("the listener still asks %s", choice.label())
			}
			return nil
		})
	if err != nil {
		return err
	}
	_, err = retry.Never(ctx, retry.Options{Fast: true, Interval: notAskedInterval}, hold,
		func(ctx context.Context) (bool, error) { return g.observeAsked(ctx, choice) },
		func(asked bool) bool { return asked })
	if err != nil {
		return fmt.Errorf("checking that the listener does not ask %s for a client certificate: %w", choice.label(), err)
	}
	return nil
}

// ── Relay entries in the pool ───────────────────────────────────────────────────

// poolRelayFixture uploads a fixture as a relay entry under a generated name and requires the
// gateway to have created it.
func (g *Gateway) poolRelayFixture(ctx context.Context, fixture, nameExpr string) error {
	resp, err := g.uploadFixtures(ctx, fixture, nameExpr, certificateUpload{usage: "downstream", role: "relay"})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("expected relay entry %q to be pooled with status 201, got %s", fixture, resp.Describe())
	}
	return nil
}

// ── A kept-alive connection that outlives a listener change ──────────────────────

// keptAliveConnection is one TLS connection, opened without a client certificate, that
// carries several requests in turn.
type keptAliveConnection struct {
	conn   net.Conn
	reader *bufio.Reader
	host   string
	base   string
	// requests counts the responses read on the connection; closing is whether the latest
	// one announced that the gateway closes the connection after it.
	requests int
	closing  bool
}

// openKeptAliveConnection connects to the HTTPS listener with the host as server name,
// sends a GET request on it with the host as Host, and keeps the connection for later
// requests. It replaces a connection an earlier step kept.
func (g *Gateway) openKeptAliveConnection(ctx context.Context, hostExpr, path string) error {
	host, err := stepscommon.Expand(ctx, hostExpr)
	if err != nil {
		return err
	}
	base, err := g.topo.URL("platform-gateway", "https")
	if err != nil {
		return err
	}
	endpoint, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("parse gateway endpoint %q: %w", base, err)
	}
	closeKept(ctx)
	httpx.ClearPublished(ctx)
	dialer := tls.Dialer{Config: &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: true, //nolint:gosec // the scenarios assert on client certificates, not on the listener's
		MinVersion:         tls.VersionTLS12,
	}}
	conn, err := dialer.DialContext(ctx, "tcp", endpoint.Host)
	if err != nil {
		return fmt.Errorf("opening a kept-alive HTTPS connection to %s with server name %q: %w", endpoint.Host, host, err)
	}
	kept := &keptAliveConnection{conn: conn, reader: bufio.NewReader(conn), host: host, base: base}
	// Stored before the request so the scenario hook closes it even when the request fails.
	if err := tcontext.Set(ctx, keyKeptAliveConnection, kept); err != nil {
		_ = conn.Close()
		return err
	}
	return g.sendKeptAlive(ctx, kept, path)
}

func (g *Gateway) sendOnKeptAliveConnection(ctx context.Context, path string) error {
	v, ok := tcontext.Get(ctx, keyKeptAliveConnection)
	kept, _ := v.(*keptAliveConnection)
	if !ok || kept == nil {
		return fmt.Errorf("no kept-alive HTTPS connection: open one first")
	}
	httpx.ClearPublished(ctx)
	return g.sendKeptAlive(ctx, kept, path)
}

// sendKeptAlive writes one GET request on the connection, reads its whole response and
// publishes it. A connection the gateway closed in between fails here, naming that.
func (g *Gateway) sendKeptAlive(ctx context.Context, kept *keptAliveConnection, path string) error {
	resolved, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(resolved, "/") {
		resolved = "/" + resolved
	}
	_ = kept.conn.SetDeadline(time.Now().Add(keptAliveTimeout))
	started := time.Now()
	if _, err := fmt.Fprintf(kept.conn, "GET %s HTTP/1.1\r\nHost: %s\r\nAccept: */*\r\n\r\n", resolved, kept.host); err != nil {
		return fmt.Errorf("the kept-alive HTTPS connection could not carry another request: %w", err)
	}
	resp, err := http.ReadResponse(kept.reader, nil)
	if err != nil {
		return fmt.Errorf("the kept-alive HTTPS connection did not answer, so the gateway closed it: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxKeptAliveBody+1))
	if err != nil {
		return fmt.Errorf("reading the response on the kept-alive HTTPS connection: %w", err)
	}
	if len(body) > maxKeptAliveBody {
		return fmt.Errorf("the response on the kept-alive HTTPS connection exceeds %d bytes", maxKeptAliveBody)
	}
	kept.requests++
	kept.closing = resp.Close
	return g.funnel.Publish(ctx, &httpx.Response{
		StatusCode: resp.StatusCode, Body: body, Headers: resp.Header.Clone(),
		Method: http.MethodGet, URL: kept.base + resolved, Elapsed: time.Since(started),
	})
}

// keptAliveConnectionStillOpen requires the connection to have carried exactly the given
// number of responses on one connection, the latest not announcing a close. A drained
// connection still answers its last request, but with Connection: close.
func (g *Gateway) keptAliveConnectionStillOpen(ctx context.Context, requests int) error {
	v, ok := tcontext.Get(ctx, keyKeptAliveConnection)
	kept, _ := v.(*keptAliveConnection)
	if !ok || kept == nil {
		return fmt.Errorf("no kept-alive HTTPS connection: open one first")
	}
	if kept.requests != requests {
		return fmt.Errorf("expected the kept-alive HTTPS connection to have carried %d responses, it carried %d", requests, kept.requests)
	}
	if kept.closing {
		return fmt.Errorf("the latest response on the kept-alive HTTPS connection announced that the gateway closes it (Connection: close)")
	}
	return nil
}

func closeKept(ctx context.Context) {
	if v, ok := tcontext.Get(ctx, keyKeptAliveConnection); ok {
		if kept, isKept := v.(*keptAliveConnection); isKept && kept != nil {
			_ = kept.conn.Close()
		}
	}
	tcontext.Remove(ctx, keyKeptAliveConnection)
}

func closeKeptAliveConnection(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
	closeKept(ctx)
	return ctx, nil
}
