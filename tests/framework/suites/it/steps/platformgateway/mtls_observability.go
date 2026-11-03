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
	"crypto/x509"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// Scenario-scoped observability state, reset by the scenario hook.
const (
	keyLogWatch       = "mtlsLogWatch"
	keyLogMarks       = "mtlsLogMarks"
	keyLogCandidate   = "mtlsLogCandidate"
	keyNotedCounters  = "mtlsNotedCounters"
	sinceLatestPhrase = "since the latest request"
)

// gatewayStack is the component whose services' logs the log steps read.
const gatewayStack = "platform-gateway"

func (g *Gateway) registerMTLSObservabilitySteps(sc *godog.ScenarioContext) {
	sc.Before(beginObservabilityScenario)
	sc.StepContext().Before(g.markLogsBeforeStep)
	sc.StepContext().After(commitLogMarksAfterRequest)

	sc.Step(`^the "([^"]*)" access log `+sinceLatestPhrase+` should show a line for "([^"]*)" with:$`, g.accessLogShowsLine)
	sc.Step(`^the "([^"]*)" log `+sinceLatestPhrase+` should contain "(.*)"$`, g.logSinceLatestRequestContains)
	sc.Step(`^the latest analytics event for path "([^"]*)" should have the user id of fixture "([^"]*)"$`, g.analyticsUserIDOfFixture)
	sc.Step(`^the latest analytics event for path "([^"]*)" should not have metadata field "([^"]*)"$`, g.analyticsMetadataFieldAbsent)
	sc.Step(`^the response should contain metric "([^"]*)" with labels (\{.*\}) and value (\S+)$`, g.metricSampleHasValue)
	sc.Step(`^the response should (contain|not contain) metric "([^"]*)" with labels (\{.*\})$`, g.metricSeriesPresence)
	sc.Step(`^the response should not contain metric "(.*)"$`, g.responseOmitsMetric)
	sc.Step(`^I note the policy engine counter "([^"]*)" for series labelled (\{.*\})$`, g.notePolicyEngineCounter)
	sc.Step(`^the policy engine counter "([^"]*)" for series labelled (\{.*\}) should have grown by at least (\d+)$`, g.policyEngineCounterGrew)
}

// beginObservabilityScenario resets the scenario's log marks and counters, and watches the
// logs only when a step of the scenario reads them since the latest request.
func beginObservabilityScenario(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
	tcontext.Remove(ctx, keyLogMarks)
	tcontext.Remove(ctx, keyLogCandidate)
	tcontext.Remove(ctx, keyNotedCounters)
	watch := false
	if sc != nil {
		for _, step := range sc.Steps {
			if step != nil && strings.Contains(step.Text, sinceLatestPhrase) {
				watch = true
			}
		}
	}
	return ctx, tcontext.Set(ctx, keyLogWatch, watch)
}

// ── Logs since the latest request ──────────────────────────────────────────────

// logCandidate is each service's log length before a step, and the response published then.
type logCandidate struct {
	lengths   map[string]int
	published *httpx.Response
}

// markLogsBeforeStep records how long each gateway service's log is before every step of a
// watching scenario. A step that publishes a new response sent a request, and the lengths
// recorded before it become the marks a later log step reads past. Container logs are append
// only, so a length is a position that needs no clock.
func (g *Gateway) markLogsBeforeStep(ctx context.Context, _ *godog.Step) (context.Context, error) {
	if v, _ := tcontext.Get(ctx, keyLogWatch); v != true {
		return ctx, nil
	}
	stack, _, err := g.topo.ServiceControl(gatewayStack)
	if err != nil {
		return ctx, err
	}
	published, _ := httpx.Published(ctx)
	candidate := logCandidate{lengths: map[string]int{}, published: published}
	logs := stack.Logs(ctx)
	for _, svc := range stack.Services() {
		section, ok := serviceLogSection(logs, svc)
		if !ok {
			return ctx, fmt.Errorf("reading the %q log before a step: %s", svc, truncateLog(logs))
		}
		candidate.lengths[svc] = len(section)
	}
	return ctx, tcontext.Set(ctx, keyLogCandidate, candidate)
}

func commitLogMarksAfterRequest(ctx context.Context, _ *godog.Step, _ godog.StepResultStatus, _ error) (context.Context, error) {
	v, ok := tcontext.Get(ctx, keyLogCandidate)
	if !ok {
		return ctx, nil
	}
	tcontext.Remove(ctx, keyLogCandidate)
	candidate, _ := v.(logCandidate)
	published, _ := httpx.Published(ctx)
	if published == nil || published == candidate.published {
		return ctx, nil
	}
	return ctx, tcontext.Set(ctx, keyLogMarks, candidate.lengths)
}

// serviceLogSection returns one service's part of a stack's labelled, concatenated logs, and
// whether the logs hold a part for it.
func serviceLogSection(logs, service string) (string, bool) {
	header := "───── " + service + " ─────\n"
	start := strings.Index(logs, header)
	if start < 0 {
		return "", false
	}
	section := logs[start+len(header):]
	if end := strings.Index(section, "\n───── "); end >= 0 {
		section = section[:end+1]
	}
	return section, true
}

// logSinceLatestRequest returns what a service logged after the scenario's latest request.
func (g *Gateway) logSinceLatestRequest(ctx context.Context, service string) (string, error) {
	v, ok := tcontext.Get(ctx, keyLogMarks)
	marks, _ := v.(map[string]int)
	if !ok || marks == nil {
		return "", fmt.Errorf("no request was sent in this scenario before reading the %q log since the latest request", service)
	}
	stack, resolved, err := g.topo.ServiceControl(service)
	if err != nil {
		return "", err
	}
	mark, ok := marks[resolved]
	if !ok {
		return "", fmt.Errorf("no log mark for service %q", resolved)
	}
	logs := stack.Logs(ctx)
	section, read := serviceLogSection(logs, resolved)
	if !read {
		return "", tolerated("the %q log could not be read: %s", resolved, truncateLog(logs))
	}
	if len(section) < mark {
		return "", fmt.Errorf("the %q log is shorter than when the latest request was sent: the container restarted", resolved)
	}
	return section[mark:], nil
}

// logSinceLatestRequestContains waits until the service logs the text after the latest request.
// Escaped quotes in the text stand for quotes.
func (g *Gateway) logSinceLatestRequestContains(ctx context.Context, service, text string) error {
	want, err := stepscommon.Expand(ctx, strings.ReplaceAll(text, `\"`, `"`))
	if err != nil {
		return err
	}
	return awaitReadState(ctx, fmt.Sprintf("waiting for the %q log to contain %q after the latest request", service, want),
		func(ctx context.Context) error {
			logs, err := g.logSinceLatestRequest(ctx, service)
			if err != nil {
				return err
			}
			if !strings.Contains(logs, want) {
				return tolerated("the %q log has not shown %q since the latest request", service, want)
			}
			return nil
		})
}

// logExpectation is one thing an access log line must show.
type logExpectation struct {
	describe string
	shownIn  func(line string) bool
}

var thumbprintRow = regexp.MustCompile(`^thumbprint of "([^"]+)"$`)

// accessLogExpectations reads the one-column table of an access log step. A row is a literal
// substring, or `thumbprint of "<fixture>"`, matched case-insensitively because Envoy's
// fingerprint case is not part of its contract.
func accessLogExpectations(ctx context.Context, table *godog.Table) ([]logExpectation, error) {
	if table == nil || len(table.Rows) == 0 {
		return nil, fmt.Errorf("an access log step needs a table of what the line shows")
	}
	out := make([]logExpectation, 0, len(table.Rows))
	for _, row := range table.Rows {
		if len(row.Cells) != 1 {
			return nil, fmt.Errorf("each access log row must have exactly one cell, got %d", len(row.Cells))
		}
		cell, err := stepscommon.Expand(ctx, strings.TrimSpace(row.Cells[0].Value))
		if err != nil {
			return nil, err
		}
		if m := thumbprintRow.FindStringSubmatch(cell); m != nil {
			fixture, err := mtlsFixtureNamed(m[1])
			if err != nil {
				return nil, err
			}
			thumbprint := strings.ToLower(fixture.Thumbprint)
			out = append(out, logExpectation{
				describe: fmt.Sprintf("the thumbprint of %q (%s)", m[1], fixture.Thumbprint),
				shownIn:  func(line string) bool { return strings.Contains(strings.ToLower(line), thumbprint) },
			})
			continue
		}
		literal := cell
		out = append(out, logExpectation{
			describe: fmt.Sprintf("%q", literal),
			shownIn:  func(line string) bool { return strings.Contains(line, literal) },
		})
	}
	return out, nil
}

// observeAccessLogLine classifies the log since the latest request. The first JSON access log
// line for the path is that request's line: it satisfies the step when it shows every
// expectation and contradicts it otherwise. No line for the path yet is the only tolerated
// state.
func observeAccessLogLine(logs, path string, expectations []logExpectation) error {
	marker := `"path":"` + path + `"`
	for _, line := range strings.Split(logs, "\n") {
		if !strings.Contains(line, marker) {
			continue
		}
		var missing []string
		for _, e := range expectations {
			if !e.shownIn(line) {
				missing = append(missing, e.describe)
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("the access log line for %s does not show %s: %s", path, strings.Join(missing, " and "), line)
		}
		return nil
	}
	return tolerated("no access log line for %s since the latest request yet", path)
}

func (g *Gateway) accessLogShowsLine(ctx context.Context, service, path string, table *godog.Table) error {
	resolvedPath, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	expectations, err := accessLogExpectations(ctx, table)
	if err != nil {
		return err
	}
	return awaitReadState(ctx, fmt.Sprintf("waiting for the %q access log line for %s", service, resolvedPath),
		func(ctx context.Context) error {
			logs, err := g.logSinceLatestRequest(ctx, service)
			if err != nil {
				return err
			}
			return observeAccessLogLine(logs, resolvedPath, expectations)
		})
}

// ── Analytics attribution ──────────────────────────────────────────────────────

// certificateSubjectIdentity is the identity mtls-auth reports for a certificate: its first
// URI SAN, else its first DNS SAN, else its Subject DN.
func certificateSubjectIdentity(cert *x509.Certificate) string {
	if len(cert.URIs) > 0 {
		return cert.URIs[0].String()
	}
	if len(cert.DNSNames) > 0 {
		return cert.DNSNames[0]
	}
	return cert.Subject.String()
}

func (g *Gateway) analyticsUserIDOfFixture(ctx context.Context, path, fixtureName string) error {
	fixture, err := mtlsFixtureNamed(fixtureName)
	if err != nil {
		return err
	}
	event, err := g.analyticsEventForPath(ctx, path)
	if err != nil {
		return err
	}
	if want := certificateSubjectIdentity(fixture.Certificate); event.UserID != want {
		return fmt.Errorf("analytics event for %q has user id %q, want the identity of fixture %q (%q)",
			path, event.UserID, fixtureName, want)
	}
	return nil
}

func (g *Gateway) analyticsMetadataFieldAbsent(ctx context.Context, path, field string) error {
	event, err := g.analyticsEventForPath(ctx, path)
	if err != nil {
		return err
	}
	if value, present := event.Metadata[field]; present {
		return fmt.Errorf("analytics event for %q has metadata field %q (%v), want none", path, field, value)
	}
	return nil
}

// ── Prometheus samples ─────────────────────────────────────────────────────────

// metricSample is one sample line of a Prometheus exposition.
type metricSample struct {
	name   string
	labels map[string]string
	value  string
}

var (
	metricLabel    = regexp.MustCompile(`(\w+)="((?:[^"\\]|\\.)*)"`)
	metricLabelSet = regexp.MustCompile(`^\{\s*(?:\w+="(?:[^"\\]|\\.)*"(?:\s*,\s*\w+="(?:[^"\\]|\\.)*")*)?\s*\}$`)
)

// parseLabels reads a {name="value",...} label set.
func parseLabels(set string) (map[string]string, error) {
	set = strings.TrimSpace(set)
	if !metricLabelSet.MatchString(set) {
		return nil, fmt.Errorf("label set %q must be written as {name=\"value\",...}", set)
	}
	labels := map[string]string{}
	for _, p := range metricLabel.FindAllStringSubmatch(set, -1) {
		if _, dup := labels[p[1]]; dup {
			return nil, fmt.Errorf("label %q appears twice in %q", p[1], set)
		}
		labels[p[1]] = p[2]
	}
	return labels, nil
}

// parseSamples reads the sample lines of a Prometheus text exposition.
func parseSamples(body string) []metricSample {
	var out []metricSample
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		s := metricSample{labels: map[string]string{}}
		rest := line
		if open := strings.IndexByte(line, '{'); open >= 0 {
			closing := strings.LastIndexByte(line, '}')
			if closing < open {
				continue
			}
			s.name = line[:open]
			for _, p := range metricLabel.FindAllStringSubmatch(line[open+1:closing], -1) {
				s.labels[p[1]] = p[2]
			}
			rest = line[closing+1:]
		} else {
			fields := strings.Fields(line)
			s.name, rest = fields[0], strings.Join(fields[1:], " ")
		}
		if fields := strings.Fields(rest); len(fields) > 0 {
			s.value = fields[0]
		}
		out = append(out, s)
	}
	return out
}

func hasLabels(s metricSample, want map[string]string) bool {
	for k, v := range want {
		if got, ok := s.labels[k]; !ok || got != v {
			return false
		}
	}
	return true
}

func sameLabels(s metricSample, want map[string]string) bool {
	return len(s.labels) == len(want) && hasLabels(s, want)
}

func publishedMetricsAndLabels(ctx context.Context, set string) (*httpx.Response, map[string]string, error) {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return nil, nil, err
	}
	expanded, err := stepscommon.Expand(ctx, set)
	if err != nil {
		return nil, nil, err
	}
	labels, err := parseLabels(expanded)
	return resp, labels, err
}

// metricSampleHasValue asserts the published exposition has a sample of the metric with
// exactly these labels and this value.
func (g *Gateway) metricSampleHasValue(ctx context.Context, name, set, value string) error {
	resp, labels, err := publishedMetricsAndLabels(ctx, set)
	if err != nil {
		return err
	}
	want, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("metric value %q is not a number", value)
	}
	for _, s := range parseSamples(resp.Text()) {
		if s.name != name || !sameLabels(s, labels) {
			continue
		}
		got, parseErr := strconv.ParseFloat(s.value, 64)
		if parseErr != nil || got != want {
			return fmt.Errorf("metric %s%s has value %q, want %s", name, set, s.value, value)
		}
		return nil
	}
	return fmt.Errorf("no sample of metric %s with labels %s: %s", name, set, resp.Describe())
}

// responseOmitsMetric fails when the published exposition contains the text. The text is a
// substring, so it can name a family prefix rather than one sample.
func (g *Gateway) responseOmitsMetric(ctx context.Context, text string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	want, err := stepscommon.Expand(ctx, strings.ReplaceAll(text, `\"`, `"`))
	if err != nil {
		return err
	}
	if name, found := metricNamedWithPrefix(resp.Text(), want); found {
		return fmt.Errorf("the response has the metric %q, which starts with %q: %s", name, want, resp.Describe())
	}
	return nil
}

// metricNamedWithPrefix returns the first metric of a Prometheus exposition whose name starts
// with prefix. Only names count: a label value such as an API name may contain the same text.
func metricNamedWithPrefix(exposition, prefix string) (string, bool) {
	for _, line := range strings.Split(exposition, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if name == "#" {
			if len(fields) < 3 || (fields[1] != "HELP" && fields[1] != "TYPE") {
				continue
			}
			name = fields[2]
		}
		name, _, _ = strings.Cut(name, "{")
		if strings.HasPrefix(name, prefix) {
			return name, true
		}
	}
	return "", false
}

// metricSeriesPresence asserts whether the published exposition has a series of the metric
// whose labels include these.
func (g *Gateway) metricSeriesPresence(ctx context.Context, mode, name, set string) error {
	resp, labels, err := publishedMetricsAndLabels(ctx, set)
	if err != nil {
		return err
	}
	found := false
	for _, s := range parseSamples(resp.Text()) {
		if s.name == name && hasLabels(s, labels) {
			found = true
			break
		}
	}
	switch {
	case mode == "contain" && !found:
		return fmt.Errorf("no series of metric %s with labels %s: %s", name, set, resp.Describe())
	case mode == "not contain" && found:
		return fmt.Errorf("metric %s still has a series with labels %s", name, set)
	}
	return nil
}

// ── Policy engine counters ─────────────────────────────────────────────────────

// counterTotal sums a counter over the samples whose name is the counter's or ends in
// "_<counter>", and whose labels include the selected ones.
func counterTotal(body, name string, labels map[string]string) (float64, error) {
	total := 0.0
	for _, s := range parseSamples(body) {
		if s.name != name && !strings.HasSuffix(s.name, "_"+name) {
			continue
		}
		if !hasLabels(s, labels) {
			continue
		}
		v, err := strconv.ParseFloat(s.value, 64)
		if err != nil {
			return 0, fmt.Errorf("unreadable value %q on a sample of %s", s.value, s.name)
		}
		total += v
	}
	return total, nil
}

// policyEngineCounter reads a counter's total from the policy engine's metrics. An engine
// that does not answer, or answers 503, is tolerated; any other answer fails.
func (g *Gateway) policyEngineCounter(ctx context.Context, name string, labels map[string]string) (float64, error) {
	url, err := g.serviceURL(ctx, "policy-engine-metrics", "/metrics")
	if err != nil {
		return 0, err
	}
	resp, err := g.funnel.Client().Do(ctx, httpx.Request{Method: http.MethodGet, URL: url}, 0, 0)
	if err != nil {
		return 0, tolerated("%s did not answer: %v", url, err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusServiceUnavailable:
		return 0, tolerated("%s is not ready: %s", url, resp.Describe())
	default:
		return 0, fmt.Errorf("reading %s: %s", url, resp.Describe())
	}
	return counterTotal(resp.Text(), name, labels)
}

func counterKey(name, set string) string { return name + "|" + set }

func (g *Gateway) notePolicyEngineCounter(ctx context.Context, name, set string) error {
	labels, err := parseLabels(set)
	if err != nil {
		return err
	}
	total, err := g.policyEngineCounter(ctx, name, labels)
	if err != nil {
		return fmt.Errorf("noting the policy engine counter %s%s: %w", name, set, err)
	}
	noted := map[string]float64{}
	if v, ok := tcontext.Get(ctx, keyNotedCounters); ok {
		if existing, ok := v.(map[string]float64); ok {
			for k, val := range existing {
				noted[k] = val
			}
		}
	}
	noted[counterKey(name, set)] = total
	return tcontext.Set(ctx, keyNotedCounters, noted)
}

// observeCounterGrowth classifies a counter reading against the noted total: grown enough
// satisfies the wait, not yet grown enough is tolerated, and below the noted total contradicts
// it, since a counter that went down was reset.
func observeCounterGrowth(name, set string, before, now float64, growth int) error {
	switch {
	case now < before:
		return fmt.Errorf("the policy engine counter %s%s fell from %g to %g: the counter was reset", name, set, before, now)
	case now-before >= float64(growth):
		return nil
	default:
		return tolerated("the policy engine counter %s%s grew by %g, waiting for %d", name, set, now-before, growth)
	}
}

func (g *Gateway) policyEngineCounterGrew(ctx context.Context, name, set string, growth int) error {
	labels, err := parseLabels(set)
	if err != nil {
		return err
	}
	v, _ := tcontext.Get(ctx, keyNotedCounters)
	noted, _ := v.(map[string]float64)
	before, ok := noted[counterKey(name, set)]
	if !ok {
		return fmt.Errorf("the policy engine counter %s%s was not noted earlier in this scenario", name, set)
	}
	return awaitReadState(ctx, fmt.Sprintf("waiting for the policy engine counter %s%s to grow by %d", name, set, growth),
		func(ctx context.Context) error {
			now, err := g.policyEngineCounter(ctx, name, labels)
			if err != nil {
				return err
			}
			return observeCounterGrowth(name, set, before, now, growth)
		})
}
