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
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/cucumber/godog"

	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/testpki"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// tagMTLS marks scenarios that change gateway-wide mutual TLS state. They run only in
// blocks whose runners take turns, so they may assert on that state as a whole.
const tagMTLS = "@mtls"

// Scenario-scoped mutual TLS state, reset by the scenario hook.
const (
	keyMTLSScenario   = "mtlsScenario"
	keyPolicyBaseline = "mtlsPolicyBaseline"
	keyTLSSession     = "mtlsTLSSession"
)

// gatewaySNI is the server name HTTPS requests send, as a client addressing the gateway
// by name does. Go sends no SNI when dialling the IP the topology resolves to.
const gatewaySNI = "localhost"

// fixtureThumbprintKey names the context value holding a fixture's SHA-256 thumbprint.
func fixtureThumbprintKey(fixture string) string { return "fixture." + fixture + ".thumbprint" }

// tlsSession is a client that caches its TLS session across requests.
type tlsSession struct {
	cache       tls.ClientSessionCache
	certificate *tls.Certificate
}

// presentation is what an HTTPS request presents to the gateway.
type presentation struct {
	fixture      string
	withChain    bool
	resumable    bool
	reuseSession bool
	relayed      relayedCertificate
	bearer       string
}

var presentationPattern = regexp.MustCompile(`^(?:(with no client certificate)|with client certificate "([^"]+)"( and its chain)?( on a resumable TLS session)?|(on a new connection from the same TLS session cache))(?: and header "([^"]+)" carrying certificate "([^"]+)"(?: encoded as "([^"]+)")?)?(?: and bearer token "([^"]+)")?$`)

// parsePresentation reads the phrase that ends an HTTPS request step.
func parsePresentation(phrase string) (presentation, error) {
	m := presentationPattern.FindStringSubmatch(strings.TrimSpace(phrase))
	if m == nil {
		return presentation{}, fmt.Errorf("unknown HTTPS request phrasing %q: expected "+
			`with no client certificate, with client certificate "<fixture>" [and its chain] `+
			`[on a resumable TLS session], or on a new connection from the same TLS session cache; `+
			`each optionally followed by and header "<name>" carrying certificate "<fixture>" [encoded as "<encoding>"], `+
			`then by and bearer token "<token>"`, phrase)
	}
	p := presentation{
		fixture:      m[2],
		withChain:    m[3] != "",
		resumable:    m[4] != "",
		reuseSession: m[5] != "",
		relayed:      relayedCertificate{header: m[6], fixture: m[7], encoding: m[8]},
		bearer:       m[9],
	}
	return p, nil
}

func (g *Gateway) registerMTLSSteps(sc *godog.ScenarioContext) {
	sc.Before(g.beginMTLSScenario)
	sc.Step(`^the certificate fixtures? "([^"]*)" (?:is|are) pooled as "([^"]*)" with usage "([^"]*)"$`,
		g.poolCertificateFixtures)
	sc.Step(`^I upload the certificate fixtures? "([^"]*)" as "([^"]*)" with usage "([^"]*)"$`,
		g.uploadCertificateFixtures)
	sc.Step(`^the gateway has applied the client authority pool$`, g.awaitGatewayApplied)
	sc.Step(`^I wait for policy snapshot sync$`, g.awaitPolicySnapshotSync)
	sc.Step(`^I send a "([^"]*)" request over HTTPS to "([^"]*)" (.+)$`, g.sendHTTPS)
	sc.Step(`^the gateway should have run a full TLS handshake$`, g.fullTLSHandshake)
}

// beginMTLSScenario resets the scenario's mutual TLS state. For an @mtls scenario it also
// publishes every fixture's thumbprint and requires the gateway to start clean, so a leak
// from the previous scenario fails here, naming the leak.
func (g *Gateway) beginMTLSScenario(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
	tcontext.Remove(ctx, keyPolicyBaseline)
	tcontext.Remove(ctx, keyTLSSession)
	isMTLS := sc != nil && hasTag(scenarioTags(sc), tagMTLS)
	if err := tcontext.Set(ctx, keyMTLSScenario, isMTLS); err != nil {
		return ctx, err
	}
	if !isMTLS {
		return ctx, nil
	}
	fixtures, err := testpki.Default()
	if err != nil {
		return ctx, err
	}
	for _, name := range fixtures.Names() {
		fixture, _ := fixtures.Get(name)
		if err := tcontext.Set(ctx, fixtureThumbprintKey(name), fixture.Thumbprint); err != nil {
			return ctx, err
		}
	}
	if err := g.requireCleanGateway(ctx); err != nil {
		return ctx, err
	}
	markGatewayChanged(ctx)
	return ctx, nil
}

func scenarioTags(sc *godog.Scenario) []string {
	names := make([]string, 0, len(sc.Tags))
	for _, t := range sc.Tags {
		if t != nil {
			names = append(names, t.Name)
		}
	}
	return names
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func isMTLSScenario(ctx context.Context) bool {
	v, ok := tcontext.Get(ctx, keyMTLSScenario)
	flag, _ := v.(bool)
	return ok && flag
}

// requireCleanGateway fails when a client authority survives from an earlier scenario, then
// waits for the gateway to apply the empty pool.
func (g *Gateway) requireCleanGateway(ctx context.Context) error {
	authorities, err := g.controllerClientAuthorities(ctx)
	if err != nil {
		return fmt.Errorf("checking that the gateway starts without client authorities: %w", err)
	}
	if len(authorities) > 0 {
		return fmt.Errorf("the gateway still holds client authorities %v from an earlier scenario", sortedKeys(authorities))
	}
	return g.awaitGatewayApplied(ctx)
}

// ── Certificate fixtures in the pool ────────────────────────────────────────────

// certificateUpload carries the optional fields of a certificate upload. Empty fields are
// omitted, so an upload keeps the product defaults: usage upstream, role client, no match, no key.
type certificateUpload struct {
	usage      string
	role       string
	dnsSANs    []string
	privateKey string
}

func (g *Gateway) poolCertificateFixtures(ctx context.Context, fixtureList, name, usage string) error {
	resp, err := g.uploadFixtures(ctx, fixtureList, name, certificateUpload{usage: usage})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("expected certificate fixtures %q to be pooled with status 201, got %s", fixtureList, resp.Describe())
	}
	return nil
}

func (g *Gateway) uploadCertificateFixtures(ctx context.Context, fixtureList, name, usage string) error {
	_, err := g.uploadFixtures(ctx, fixtureList, name, certificateUpload{usage: usage})
	return err
}

// uploadFixtures uploads the named fixtures as one PEM under a generated name, publishes the
// response, and registers an accepted certificate for cleanup at once.
func (g *Gateway) uploadFixtures(ctx context.Context, fixtureList, nameExpr string, opts certificateUpload) (*httpx.Response, error) {
	name, err := g.generatedCertificateName(ctx, nameExpr)
	if err != nil {
		return nil, err
	}
	pemBundle, err := fixturePEMBundle(fixtureList)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"name": name, "certificate": pemBundle}
	if usage := strings.TrimSpace(opts.usage); usage != "" {
		body["usage"] = usage
	}
	if role := strings.TrimSpace(opts.role); role != "" {
		body["role"] = role
	}
	if len(opts.dnsSANs) > 0 {
		body["match"] = map[string]any{"dnsSANs": opts.dnsSANs}
	}
	if opts.privateKey != "" {
		body["privateKey"] = opts.privateKey
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encoding the certificate upload: %w", err)
	}
	return g.postCertificate(ctx, name, payload)
}

// generatedCertificateName expands a certificate name and requires it to be generated for
// this runner, since certificate names are unique across the gateway.
func (g *Gateway) generatedCertificateName(ctx context.Context, nameExpr string) (string, error) {
	name, err := stepscommon.Expand(ctx, nameExpr)
	if err != nil {
		return "", err
	}
	generator, err := unique.Of(ctx)
	if err != nil {
		return "", err
	}
	if !strings.Contains(name, generator.Suffix()) {
		return "", fmt.Errorf("certificate name %q is not generated; use a stored unique value", name)
	}
	return name, nil
}

// fixturePEMBundle joins the certificates of comma-separated fixtures, in order.
func fixturePEMBundle(fixtureList string) (string, error) {
	fixtures, err := testpki.Default()
	if err != nil {
		return "", err
	}
	var bundle bytes.Buffer
	for i, name := range strings.Split(fixtureList, ",") {
		fixture, err := fixtures.Get(name)
		if err != nil {
			return "", err
		}
		if i > 0 {
			bundle.WriteString("\n")
		}
		bundle.Write(fixture.CertPEM)
	}
	return bundle.String(), nil
}

// ── HTTPS requests ─────────────────────────────────────────────────────────────

// sendHTTPS sends one request to the gateway's HTTPS listener on its own connection and
// publishes the response. In an @mtls scenario it first waits for the gateway to apply the
// client authority pool, so the request meets the configuration the scenario set up.
func (g *Gateway) sendHTTPS(ctx context.Context, method, path, phrase string) error {
	p, err := parsePresentation(phrase)
	if err != nil {
		return err
	}
	clientTLS, err := g.clientTLS(ctx, p)
	if err != nil {
		return err
	}
	resolved, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	base, err := g.topo.URL("platform-gateway", "https")
	if err != nil {
		return err
	}
	if resolved != "" && !strings.HasPrefix(resolved, "/") {
		resolved = "/" + resolved
	}
	headers := g.scenarioHeaders(ctx)
	if p.bearer != "" {
		token, expandErr := stepscommon.Expand(ctx, p.bearer)
		if expandErr != nil {
			return expandErr
		}
		headers["Authorization"] = "Bearer " + token
	}
	if err := p.relayed.apply(headers); err != nil {
		return err
	}
	if isMTLSScenario(ctx) {
		if err := g.settleAfterChange(ctx); err != nil {
			return err
		}
	}
	url := base + resolved
	if _, err := g.funnel.Send(ctx, httpx.Request{
		Method: strings.ToUpper(method), URL: url, Headers: headers, Host: g.requestHost(ctx), TLS: clientTLS,
	}); err != nil {
		return fmt.Errorf("invoking %s %s over HTTPS: %w", strings.ToUpper(method), url, err)
	}
	return nil
}

// clientTLS builds the client side of the handshake a presentation describes.
func (g *Gateway) clientTLS(ctx context.Context, p presentation) (*httpx.ClientTLS, error) {
	// The listener serves one self-signed certificate whatever server name arrives, so no
	// name verifies against it; the scenarios assert on client certificates, not on the
	// server's.
	opts := &httpx.ClientTLS{ServerName: gatewaySNI, InsecureSkipVerify: true}
	g.applyServerNameChoice(ctx, opts)
	if p.reuseSession {
		v, ok := tcontext.Get(ctx, keyTLSSession)
		session, _ := v.(*tlsSession)
		if !ok || session == nil {
			return nil, fmt.Errorf("no TLS session cache: send a request on a resumable TLS session first")
		}
		opts.Certificate, opts.Sessions = session.certificate, session.cache
		return opts, nil
	}
	if p.fixture != "" {
		fixtures, err := testpki.Default()
		if err != nil {
			return nil, err
		}
		fixture, err := fixtures.Get(p.fixture)
		if err != nil {
			return nil, err
		}
		cert, err := fixture.TLSCertificate(p.withChain)
		if err != nil {
			return nil, err
		}
		opts.Certificate = &cert
	}
	if p.resumable {
		session := &tlsSession{cache: tls.NewLRUClientSessionCache(1), certificate: opts.Certificate}
		if err := tcontext.Set(ctx, keyTLSSession, session); err != nil {
			return nil, err
		}
		opts.Sessions = session.cache
	}
	return opts, nil
}

func (g *Gateway) fullTLSHandshake(ctx context.Context) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if resp.TLS == nil {
		return fmt.Errorf("the last response did not arrive over an HTTPS request step: %s", resp.Describe())
	}
	if resp.TLS.DidResume {
		return fmt.Errorf("the gateway resumed the cached TLS session instead of running a full handshake")
	}
	return nil
}

// ── Waiting for the gateway to apply what the controller holds ─────────────────

// Names the controller and Envoy use for the client authority pool.
const (
	clientAuthorityResourceType = "ClientCertificateAuthority"
	downstreamClientCASecret    = "downstream_client_ca"
	mtlsAuthPolicyName          = "mtls-auth"
)

// clientAuthority is one pool entry as one component holds it.
type clientAuthority struct {
	role        string
	count       int
	thumbprints []string
}

// awaitGatewayApplied waits until the policy engine and Envoy hold what the controller holds:
// the same client authorities and an HTTPS listener that asks for a client certificate exactly
// when the controller's configuration requires it, with no listener or cluster still warming.
// Policy chain versions are left to awaitPolicySnapshotSync: the controller publishes an empty
// policy snapshot when its last API is removed, and the policy engine is never sent it.
func (g *Gateway) awaitGatewayApplied(ctx context.Context) error {
	if err := awaitReadState(ctx, "waiting for the gateway to apply the client authority pool", g.observeGatewayApplied); err != nil {
		return err
	}
	clearGatewayChange(ctx)
	return nil
}

// observeGatewayApplied returns nil once the runtime holds what the controller holds, a
// tolerated error while a change is still propagating, and a plain error for a state no
// amount of propagation produces.
func (g *Gateway) observeGatewayApplied(ctx context.Context) error {
	want, err := g.controllerClientAuthorities(ctx)
	if err != nil {
		return err
	}
	attached, err := g.controllerAttachesMTLSAuth(ctx, managementControllerAdmin)
	if err != nil {
		return err
	}
	if err := g.requireRuntimeControllerAgrees(ctx, attached); err != nil {
		return err
	}
	held, err := g.policyEngineClientAuthorities(ctx)
	if err != nil {
		return err
	}
	if err := compareClientAuthorities(want, held); err != nil {
		return err
	}
	listeners, err := g.envoyListeners(ctx)
	if err != nil {
		return err
	}
	if len(listeners.warming) > 0 {
		return tolerated("Envoy listeners %v are still warming", listeners.warming)
	}
	requests := attached && len(want) > 0
	if listeners.namesClientCA != requests {
		return tolerated("the HTTPS listener requests a client certificate: %t, the controller's configuration requires %t",
			listeners.namesClientCA, requests)
	}
	if requests {
		served, present, err := g.envoyClientCAThumbprints(ctx)
		if err != nil {
			return err
		}
		if !present {
			return tolerated("Envoy serves no %s secret yet", downstreamClientCASecret)
		}
		if expected := poolThumbprints(held); !sameSet(expected, served) {
			return tolerated("Envoy's %s holds %v, the pool holds %v", downstreamClientCASecret, sortedKeys(served), sortedKeys(expected))
		}
	}
	warming, err := g.envoyWarmingClusters(ctx)
	if err != nil {
		return err
	}
	if len(warming) > 0 {
		return tolerated("Envoy clusters %v are still warming", warming)
	}
	return nil
}

// compareClientAuthorities compares the controller's pool with the policy engine's. An entry
// the engine has not received yet, or still holds after its removal, is propagation. The same
// name with another role or certificate count is not: names are generated per scenario and
// never reused, so no earlier state explains it.
func compareClientAuthorities(want, held map[string]clientAuthority) error {
	for _, name := range sortedKeys(want) {
		h, ok := held[name]
		if !ok {
			return tolerated("the policy engine does not hold client authority %q yet", name)
		}
		if w := want[name]; w.role != h.role || w.count != h.count {
			return fmt.Errorf("the policy engine holds %q with role %q and %d certificates, the controller with role %q and %d",
				name, h.role, h.count, w.role, w.count)
		}
	}
	for _, name := range sortedKeys(held) {
		if _, ok := want[name]; !ok {
			return tolerated("the policy engine still holds removed client authority %q", name)
		}
	}
	return nil
}

func poolThumbprints(pool map[string]clientAuthority) map[string]bool {
	out := map[string]bool{}
	for _, a := range pool {
		for _, t := range a.thumbprints {
			out[t] = true
		}
	}
	return out
}

// beforeAPIMutation records the controller's policy chain version before an @mtls scenario
// changes an API, so a later policy snapshot wait can require the controller to move past it.
func (g *Gateway) beforeAPIMutation(ctx context.Context) error {
	markGatewayChanged(ctx)
	if !isMTLSScenario(ctx) {
		return nil
	}
	sync, err := g.policySnapshotVersions(ctx)
	if err != nil {
		return fmt.Errorf("reading the policy chain version before an API change: %w", err)
	}
	return tcontext.Set(ctx, keyPolicyBaseline, sync.controller)
}

// awaitPolicySnapshotSync waits until the policy engine holds the controller's policy chain
// version and, after an API change, until the controller has moved past the version it held
// before that change. The controller not having published yet and the engine lagging it are
// tolerated; a controller without a version, or an engine ahead of the controller, fails.
func (g *Gateway) awaitPolicySnapshotSync(ctx context.Context) error {
	baseline := ""
	if v, ok := tcontext.Get(ctx, keyPolicyBaseline); ok {
		baseline, _ = v.(string)
	}
	err := awaitReadState(ctx, fmt.Sprintf("waiting for the policy engine to apply the controller's policy chain past version %q", baseline),
		func(ctx context.Context) error {
			s, err := g.policySnapshotVersions(ctx)
			if err != nil {
				return err
			}
			return s.observe(baseline)
		})
	if err != nil {
		return err
	}
	tcontext.Remove(ctx, keyPolicyBaseline)
	return nil
}

// observe classifies the versions against the pre-change baseline.
func (s policySync) observe(baseline string) error {
	switch {
	case s.controller == "":
		return fmt.Errorf("the controller reports no policy chain version (%s)", s)
	case s.controller == baseline:
		return tolerated("the controller has not published past version %q yet (%s)", baseline, s)
	case s.engine == s.controller:
		return nil
	case versionAhead(s.engine, s.controller):
		return fmt.Errorf("the policy engine is ahead of the controller (%s)", s)
	default:
		return tolerated("the policy engine has not applied the controller's version yet (%s)", s)
	}
}

// versionAhead reports whether version a is numerically greater than b. Versions that are not
// both integers are never ahead.
func versionAhead(a, b string) bool {
	x, errA := strconv.ParseUint(a, 10, 64)
	y, errB := strconv.ParseUint(b, 10, 64)
	return errA == nil && errB == nil && x > y
}

// policySync holds the policy chain versions the controller published and the policy
// engine applied.
type policySync struct{ controller, engine string }

// String renders the versions for a wait's failure message.
func (s policySync) String() string {
	return fmt.Sprintf("controller %q, policy engine %q", s.controller, s.engine)
}

// policySnapshotVersions reads the policy chain version of the controller that feeds the
// runtime, since each controller numbers its own snapshots, and the policy engine's.
func (g *Gateway) policySnapshotVersions(ctx context.Context) (policySync, error) {
	var controller, engine struct {
		Version *string `json:"policy_chain_version"`
	}
	service, _, err := g.runtimeControllerAdmin()
	if err != nil {
		return policySync{}, err
	}
	if err := g.readJSON(ctx, service, "/xds_sync_status", &controller); err != nil {
		return policySync{}, err
	}
	if err := g.readJSON(ctx, "policy-engine", "/xds_sync_status", &engine); err != nil {
		return policySync{}, err
	}
	var s policySync
	if controller.Version != nil {
		s.controller = *controller.Version
	}
	if engine.Version != nil {
		s.engine = *engine.Version
	}
	return s, nil
}

// controllerClientAuthorities lists the controller's usage: downstream certificates by name.
func (g *Gateway) controllerClientAuthorities(ctx context.Context) (map[string]clientAuthority, error) {
	var listing struct {
		Certificates []struct {
			Name  string `json:"name"`
			Role  string `json:"role"`
			Count int    `json:"count"`
		} `json:"certificates"`
	}
	if err := g.readJSON(ctx, "gateway-controller", "/certificates?usage=downstream", &listing); err != nil {
		return nil, err
	}
	out := make(map[string]clientAuthority, len(listing.Certificates))
	for _, c := range listing.Certificates {
		out[c.Name] = clientAuthority{role: c.Role, count: c.Count}
	}
	return out, nil
}

// controllerAttachesMTLSAuth reports whether any API the controller behind an admin service
// holds attaches mtls-auth, at API level or on an operation.
func (g *Gateway) controllerAttachesMTLSAuth(ctx context.Context, adminService string) (bool, error) {
	type policy struct {
		Name string `json:"name"`
	}
	var dump struct {
		APIs []struct {
			Configuration struct {
				Spec struct {
					Policies   []policy `json:"policies"`
					Operations []struct {
						Policies []policy `json:"policies"`
					} `json:"operations"`
				} `json:"spec"`
			} `json:"configuration"`
		} `json:"apis"`
	}
	if err := g.readJSON(ctx, adminService, "/config_dump", &dump); err != nil {
		return false, err
	}
	for _, api := range dump.APIs {
		lists := [][]policy{api.Configuration.Spec.Policies}
		for _, op := range api.Configuration.Spec.Operations {
			lists = append(lists, op.Policies)
		}
		for _, list := range lists {
			for _, p := range list {
				if p.Name == mtlsAuthPolicyName {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// policyEngineClientAuthorities reads the client authorities the policy engine holds.
func (g *Gateway) policyEngineClientAuthorities(ctx context.Context) (map[string]clientAuthority, error) {
	var dump struct {
		Lazy struct {
			ByType map[string][]struct {
				ID       string `json:"id"`
				Resource struct {
					Certificates []string `json:"certificates"`
					Role         string   `json:"role"`
				} `json:"resource"`
			} `json:"resources_by_type"`
		} `json:"lazy_resources"`
	}
	if err := g.readJSON(ctx, "policy-engine", "/config_dump", &dump); err != nil {
		return nil, err
	}
	resources := dump.Lazy.ByType[clientAuthorityResourceType]
	out := make(map[string]clientAuthority, len(resources))
	for _, r := range resources {
		out[r.ID] = clientAuthority{
			role:        r.Resource.Role,
			count:       len(r.Resource.Certificates),
			thumbprints: pemThumbprints([]byte(strings.Join(r.Resource.Certificates, "\n"))),
		}
	}
	return out, nil
}

// envoyListenerState is what Envoy's dynamic listeners show about the client authority pool.
type envoyListenerState struct {
	namesClientCA bool
	warming       []string
}

func (g *Gateway) envoyListeners(ctx context.Context) (envoyListenerState, error) {
	var dump struct {
		Configs []struct {
			Name         string          `json:"name"`
			ActiveState  json.RawMessage `json:"active_state"`
			WarmingState json.RawMessage `json:"warming_state"`
		} `json:"configs"`
	}
	if err := g.readJSON(ctx, "envoy-admin", "/config_dump?resource=dynamic_listeners", &dump); err != nil {
		return envoyListenerState{}, err
	}
	var state envoyListenerState
	for _, l := range dump.Configs {
		if len(l.WarmingState) > 0 && string(l.WarmingState) != "null" {
			state.warming = append(state.warming, l.Name)
		}
		if bytes.Contains(l.ActiveState, []byte(`"`+downstreamClientCASecret+`"`)) {
			state.namesClientCA = true
		}
	}
	return state, nil
}

// envoyClientCAThumbprints returns the thumbprints Envoy's active client CA secret holds and
// whether it serves one.
func (g *Gateway) envoyClientCAThumbprints(ctx context.Context) (map[string]bool, bool, error) {
	var dump struct {
		Configs []struct {
			Name   string `json:"name"`
			Secret struct {
				ValidationContext struct {
					TrustedCA struct {
						InlineBytes []byte `json:"inline_bytes"`
					} `json:"trusted_ca"`
				} `json:"validation_context"`
			} `json:"secret"`
		} `json:"configs"`
	}
	if err := g.readJSON(ctx, "envoy-admin", "/config_dump?resource=dynamic_active_secrets", &dump); err != nil {
		return nil, false, err
	}
	for _, c := range dump.Configs {
		if c.Name != downstreamClientCASecret {
			continue
		}
		set := map[string]bool{}
		for _, t := range pemThumbprints(c.Secret.ValidationContext.TrustedCA.InlineBytes) {
			set[t] = true
		}
		return set, true, nil
	}
	return nil, false, nil
}

func (g *Gateway) envoyWarmingClusters(ctx context.Context) ([]string, error) {
	var dump struct {
		Configs []struct {
			Cluster struct {
				Name string `json:"name"`
			} `json:"cluster"`
		} `json:"configs"`
	}
	if err := g.readJSON(ctx, "envoy-admin", "/config_dump?resource=dynamic_warming_clusters", &dump); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(dump.Configs))
	for _, c := range dump.Configs {
		names = append(names, c.Cluster.Name)
	}
	return names, nil
}

// readJSON reads a component endpoint as the admin user without publishing the response. A
// component that does not answer or answers 503 is tolerated; any other answer fails at once.
func (g *Gateway) readJSON(ctx context.Context, service, path string, out any) error {
	url, err := g.serviceURL(ctx, service, path)
	if err != nil {
		return err
	}
	headers, err := g.adminHeaders(ctx)
	if err != nil {
		return err
	}
	resp, err := g.funnel.Client().Do(ctx, httpx.Request{Method: http.MethodGet, URL: url, Headers: headers}, 0, 0)
	if err != nil {
		return tolerated("%s did not answer: %v", url, err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusServiceUnavailable:
		return tolerated("%s is not ready: %s", url, resp.Describe())
	default:
		return fmt.Errorf("reading %s: %s", url, resp.Describe())
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		return fmt.Errorf("reading %s: %w (%s)", url, err, resp.Describe())
	}
	return nil
}

func (g *Gateway) adminHeaders(ctx context.Context) (map[string]string, error) {
	user, err := tcontext.ResolveString(ctx, frameworkruntime.KeyAdminUser)
	if err != nil {
		return nil, err
	}
	pass, err := tcontext.ResolveString(ctx, frameworkruntime.KeyAdminPass)
	if err != nil {
		return nil, err
	}
	return map[string]string{"Authorization": BasicAuthHeader(user, pass)}, nil
}

// pemThumbprints returns the lowercase hexadecimal SHA-256 thumbprint of each certificate
// in a PEM bundle, in order.
func pemThumbprints(bundle []byte) []string {
	var out []string
	for {
		var block *pem.Block
		block, bundle = pem.Decode(bundle)
		if block == nil {
			return out
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		sum := sha256.Sum256(block.Bytes)
		out = append(out, hex.EncodeToString(sum[:]))
	}
}

func sameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
