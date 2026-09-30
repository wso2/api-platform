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

package it

import (
	"context"
	"crypto/x509"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// mtlsObservabilitySteps holds the mTLS container-log and analytics
// attribution steps, reusing the mTLS and analytics step instances.
type mtlsObservabilitySteps struct {
	composeManager *ComposeManager
	mtls           *mtlsSteps
	analytics      *AnalyticsSteps

	// scenarioStart bounds log polling when the scenario has sent no request
	// yet.
	scenarioStart time.Time

	// containerIDs caches each service's container id for the scenario.
	containerIDs map[string]string
}

// logThumbprintRow matches an access-log table row naming a fixture's
// thumbprint rather than a literal substring.
var logThumbprintRow = regexp.MustCompile(`^thumbprint of "([^"]+)"$`)

// logExpectation is one thing a container log line must show.
type logExpectation struct {
	describe string
	shownIn  func(line string) bool
}

// RegisterMTLSObservabilitySteps registers the mTLS observability steps.
func RegisterMTLSObservabilitySteps(ctx *godog.ScenarioContext, composeManager *ComposeManager, mtls *mtlsSteps, analytics *AnalyticsSteps) {
	o := &mtlsObservabilitySteps{composeManager: composeManager, mtls: mtls, analytics: analytics}

	ctx.Before(func(c context.Context, sc *godog.Scenario) (context.Context, error) {
		o.scenarioStart = time.Now()
		o.containerIDs = map[string]string{}
		return c, nil
	})

	ctx.Step(`^the "([^"]*)" container log should contain the thumbprint of fixture "([^"]*)" within (\d+) seconds$`,
		o.containerLogShouldContainThumbprintOfFixtureWithin)
	// (.*) because the expected substring can carry escaped quotes (e.g.
	// `\"peerSubj\":\"CN=client-valid\"`), which [^"]* would stop at.
	ctx.Step(`^the "([^"]*)" container log should contain "(.*)" within (\d+) seconds$`,
		o.containerLogShouldContainWithin)
	ctx.Step(`^the "([^"]*)" access log should show within (\d+) seconds:$`,
		o.accessLogShouldShowWithin)

	ctx.Step(`^the latest analytics event should have the user id of fixture "([^"]*)"$`,
		o.latestAnalyticsEventShouldHaveUserIDOfFixture)
	ctx.Step(`^the latest analytics event should have no metadata field "([^"]*)"$`,
		o.latestAnalyticsEventShouldHaveNoMetadataField)
}

// containerLogShouldContainWithin polls the service's log since the latest
// request until a line contains substr, matched exactly and case-sensitively.
func (o *mtlsObservabilitySteps) containerLogShouldContainWithin(service, substr string, seconds int) error {
	return o.pollLogs(service, seconds, []logExpectation{logContains(unescapeGherkinQuotes(substr))})
}

// containerLogShouldContainThumbprintOfFixtureWithin polls the service's log
// for the fixture's thumbprint.
func (o *mtlsObservabilitySteps) containerLogShouldContainThumbprintOfFixtureWithin(service, fixture string, seconds int) error {
	expectation, err := o.logContainsThumbprintOf(fixture)
	if err != nil {
		return err
	}
	return o.pollLogs(service, seconds, []logExpectation{expectation})
}

// accessLogShouldShowWithin polls the service's log until one line shows
// every row of the one-column table: a literal substring, matched exactly
// and case-sensitively, or `thumbprint of "fixture"`.
func (o *mtlsObservabilitySteps) accessLogShouldShowWithin(service string, seconds int, table *godog.Table) error {
	expectations := make([]logExpectation, 0, len(table.Rows))
	for _, row := range table.Rows {
		if len(row.Cells) != 1 {
			return fmt.Errorf("each expected log row must have exactly one cell, got %d", len(row.Cells))
		}
		cell := strings.TrimSpace(row.Cells[0].Value)
		if m := logThumbprintRow.FindStringSubmatch(cell); m != nil {
			expectation, err := o.logContainsThumbprintOf(m[1])
			if err != nil {
				return err
			}
			expectations = append(expectations, expectation)
			continue
		}
		expectations = append(expectations, logContains(cell))
	}
	return o.pollLogs(service, seconds, expectations)
}

func logContains(substr string) logExpectation {
	return logExpectation{
		describe: fmt.Sprintf("%q", substr),
		shownIn:  func(line string) bool { return strings.Contains(line, substr) },
	}
}

// logContainsThumbprintOf matches the fixture's thumbprint
// case-insensitively, because Envoy's fingerprint rendering case is not
// guaranteed.
func (o *mtlsObservabilitySteps) logContainsThumbprintOf(fixture string) (logExpectation, error) {
	thumbprint, err := o.mtls.thumbprintOf(fixture)
	if err != nil {
		return logExpectation{}, err
	}
	lowerThumbprint := strings.ToLower(thumbprint)
	return logExpectation{
		describe: fmt.Sprintf("the thumbprint of fixture %q (%s)", fixture, thumbprint),
		shownIn:  func(line string) bool { return strings.Contains(strings.ToLower(line), lowerThumbprint) },
	}, nil
}

// pollLogs fetches, every 500ms, the service's log since the scenario's
// latest request was sent, until one log line shows every expectation or the
// timeout elapses.
func (o *mtlsObservabilitySteps) pollLogs(service string, seconds int, expectations []logExpectation) error {
	containerID, err := o.containerID(service)
	if err != nil {
		return err
	}
	since := o.scenarioStart
	if at, ok := lastRequestAt(o.mtls.state); ok && at.After(since) {
		since = at
	}

	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	var lastErr error
	for {
		read := false
		logs, err := ContainerLogs(containerID, since)
		if err != nil {
			lastErr = err
		} else {
			read = true
			for _, line := range strings.Split(logs, "\n") {
				if lineShowsAll(line, expectations) {
					return nil
				}
			}
		}

		if time.Now().After(deadline) {
			if !read {
				return fmt.Errorf("%q container log could not be read within %ds: %v", service, seconds, lastErr)
			}
			described := make([]string, 0, len(expectations))
			for _, e := range expectations {
				described = append(described, e.describe)
			}
			return fmt.Errorf("no %q container log line since the latest request showed %s within %ds", service, strings.Join(described, " and "), seconds)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// lineShowsAll reports whether one log line shows every expectation.
func lineShowsAll(line string, expectations []logExpectation) bool {
	for _, e := range expectations {
		if !e.shownIn(line) {
			return false
		}
	}
	return true
}

// containerID resolves the service's container once per scenario.
func (o *mtlsObservabilitySteps) containerID(service string) (string, error) {
	if id, ok := o.containerIDs[service]; ok {
		return id, nil
	}
	if o.composeManager == nil {
		return "", fmt.Errorf("compose manager is not initialized")
	}
	id, err := o.composeManager.ServiceContainerID(service)
	if err != nil {
		return "", err
	}
	o.containerIDs[service] = id
	return id, nil
}

// mtlsAuthSubjectIdentity mirrors the mtls-auth policy's subject: the first
// URI SAN, else the first DNS SAN, else the Subject DN. It becomes the
// analytics event's user_id.
func mtlsAuthSubjectIdentity(cert *x509.Certificate) string {
	if len(cert.URIs) > 0 {
		return cert.URIs[0].String()
	}
	if len(cert.DNSNames) > 0 {
		return cert.DNSNames[0]
	}
	return cert.Subject.String()
}

// latestAnalyticsEventShouldHaveUserIDOfFixture verifies the latest
// analytics event's user_id is the fixture certificate's subject identity.
func (o *mtlsObservabilitySteps) latestAnalyticsEventShouldHaveUserIDOfFixture(fixture string) error {
	event, err := o.analytics.latestEventOrFetch()
	if err != nil {
		return err
	}

	cert, err := o.mtls.parseFixtureCert(fixture)
	if err != nil {
		return err
	}
	expected := mtlsAuthSubjectIdentity(cert)

	if event.UserID != expected {
		return fmt.Errorf("expected latest analytics event user_id to be the subject of fixture %q (%q), got %q",
			fixture, expected, event.UserID)
	}
	return nil
}

// latestAnalyticsEventShouldHaveNoMetadataField verifies fieldName is absent
// from the latest analytics event's metadata.
func (o *mtlsObservabilitySteps) latestAnalyticsEventShouldHaveNoMetadataField(fieldName string) error {
	event, err := o.analytics.latestEventOrFetch()
	if err != nil {
		return err
	}

	if _, present := event.Metadata[fieldName]; present {
		return fmt.Errorf("expected latest analytics event to have no metadata field %q, but it was present", fieldName)
	}
	return nil
}
