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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// DPToCPSteps provides step definitions for the data-plane -> control-plane
// artifact push flow. The gateway-controller pushes every gateway-originated
// artifact (LLM provider template/provider/proxy, MCP proxy, REST API) to the
// control plane on create/update, undeploys them on delete, and re-pushes any
// pending/failed ones on (re)connect. In the integration test the control plane
// is stood in for by mock-platform-api, which records what it received; these
// steps drive and assert against that recorder plus the gateway's own DB
// bookkeeping (cp_sync_status / cp_artifact_id).
type DPToCPSteps struct {
	state *TestState
}

// NewDPToCPSteps creates a new DPToCPSteps instance.
func NewDPToCPSteps(state *TestState) *DPToCPSteps {
	return &DPToCPSteps{state: state}
}

const (
	// cpPushAssertTimeout bounds how long we wait for an asynchronous push to
	// arrive at (or be recorded by) the control plane. The create-path push for
	// deployable kinds waits for the deployment to land first, and a rejected
	// push is retried with backoff (~15s over 5 attempts), so this is generous.
	cpPushAssertTimeout = 35 * time.Second

	// cpPushPollInterval is how often the assertions re-poll the recorder / DB.
	cpPushPollInterval = 500 * time.Millisecond

	// cpNoPushGrace is how long a negative ("should not receive") assertion waits
	// before concluding the artifact was genuinely not pushed.
	cpNoPushGrace = 6 * time.Second
)

// cpRecordedArtifact mirrors the mock-platform-api recorder's per-artifact view
// (see tests/mock-servers/mock-platform-api/main.go recordedArtifact).
type cpRecordedArtifact struct {
	DPID          string                 `json:"dpid"`
	CPID          string                 `json:"cpId"`
	Kind          string                 `json:"kind"`
	Handle        string                 `json:"handle"`
	Status        string                 `json:"status"`
	Origin        string                 `json:"origin"`
	DeployedAt    *time.Time             `json:"deployedAt,omitempty"`
	CreatedAt     time.Time              `json:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt"`
	Configuration map[string]interface{} `json:"configuration"`
	PushCount     int                    `json:"pushCount"`
}

// RegisterDPToCPSteps registers all DP->CP Gherkin steps.
func RegisterDPToCPSteps(ctx *godog.ScenarioContext, state *TestState) {
	d := NewDPToCPSteps(state)

	// Recorder control.
	ctx.Step(`^I reset the control plane recorder$`, d.resetRecorder)
	ctx.Step(`^I make the control plane reject artifact imports$`, d.rejectImports)
	ctx.Step(`^I make the control plane accept artifact imports$`, d.acceptImports)

	// Push assertions (against the mock control plane recorder).
	ctx.Step(`^the control plane should receive the "([^"]*)" artifact "([^"]*)"$`, d.shouldReceive)
	ctx.Step(`^the control plane should receive the "([^"]*)" artifact "([^"]*)" with status "([^"]*)"$`, d.shouldReceiveWithStatus)
	ctx.Step(`^the control plane should not receive the "([^"]*)" artifact "([^"]*)"$`, d.shouldNotReceive)
	ctx.Step(`^the control plane copy of the "([^"]*)" artifact "([^"]*)" should have been pushed at least (\d+) times$`, d.pushedAtLeast)
	ctx.Step(`^the control plane copy of the "([^"]*)" artifact "([^"]*)" configuration should contain "([^"]*)"$`, d.configShouldContain)
	ctx.Step(`^the control plane copy of the "([^"]*)" artifact "([^"]*)" should reference (provider|template) "([^"]*)"$`, d.shouldReference)
	ctx.Step(`^the control plane copy of the "([^"]*)" artifact "([^"]*)" should carry a deployed timestamp$`, d.shouldCarryDeployedAt)
	ctx.Step(`^the control plane should have undeployed the "([^"]*)" artifact "([^"]*)"$`, d.shouldBeUndeployed)

	// Gateway-side bookkeeping assertions (against the controller DB via db-reader).
	ctx.Step(`^the gateway should record cp_sync_status "([^"]*)" for the "([^"]*)" artifact "([^"]*)"$`, d.shouldRecordSyncStatus)
	ctx.Step(`^the gateway should record a cp_artifact_id for the "([^"]*)" artifact "([^"]*)"$`, d.shouldRecordCPArtifactID)
}

// --- recorder control -------------------------------------------------------

func (d *DPToCPSteps) resetRecorder() error {
	return d.postJSON("/_test/reset", nil)
}

func (d *DPToCPSteps) rejectImports() error {
	return d.postJSON("/_test/config", map[string]bool{"rejectImports": true})
}

func (d *DPToCPSteps) acceptImports() error {
	return d.postJSON("/_test/config", map[string]bool{"rejectImports": false})
}

// --- push assertions --------------------------------------------------------

func (d *DPToCPSteps) shouldReceive(kind, handle string) error {
	_, err := d.waitForArtifact(kind, handle, func(cpRecordedArtifact) bool { return true }, cpPushAssertTimeout)
	if err != nil {
		return fmt.Errorf("control plane did not receive %s %q: %w", kind, handle, err)
	}
	return nil
}

func (d *DPToCPSteps) shouldReceiveWithStatus(kind, handle, status string) error {
	_, err := d.waitForArtifact(kind, handle, func(a cpRecordedArtifact) bool {
		return strings.EqualFold(a.Status, status)
	}, cpPushAssertTimeout)
	if err != nil {
		return fmt.Errorf("control plane did not receive %s %q with status %q: %w", kind, handle, status, err)
	}
	return nil
}

func (d *DPToCPSteps) shouldNotReceive(kind, handle string) error {
	// Wait a grace period so an in-flight push has a fair chance to arrive, then
	// assert the artifact is still absent.
	deadline := time.Now().Add(cpNoPushGrace)
	for time.Now().Before(deadline) {
		if _, ok, err := d.findArtifact(kind, handle); err != nil {
			return err
		} else if ok {
			return fmt.Errorf("control plane unexpectedly received %s %q", kind, handle)
		}
		time.Sleep(cpPushPollInterval)
	}
	return nil
}

func (d *DPToCPSteps) pushedAtLeast(kind, handle string, n int) error {
	_, err := d.waitForArtifact(kind, handle, func(a cpRecordedArtifact) bool {
		return a.PushCount >= n
	}, cpPushAssertTimeout)
	if err != nil {
		return fmt.Errorf("%s %q was not pushed at least %d times: %w", kind, handle, n, err)
	}
	return nil
}

func (d *DPToCPSteps) configShouldContain(kind, handle, substr string) error {
	rec, err := d.waitForArtifact(kind, handle, func(cpRecordedArtifact) bool { return true }, cpPushAssertTimeout)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(rec.Configuration)
	if err != nil {
		return fmt.Errorf("failed to marshal recorded configuration for %s %q: %w", kind, handle, err)
	}
	if !strings.Contains(string(raw), substr) {
		return fmt.Errorf("recorded configuration for %s %q does not contain %q\nconfiguration: %s", kind, handle, substr, string(raw))
	}
	return nil
}

func (d *DPToCPSteps) shouldReference(kind, handle, refType, refHandle string) error {
	rec, err := d.waitForArtifact(kind, handle, func(cpRecordedArtifact) bool { return true }, cpPushAssertTimeout)
	if err != nil {
		return err
	}
	spec, _ := rec.Configuration["spec"].(map[string]interface{})
	if spec == nil {
		return fmt.Errorf("recorded %s %q has no spec", kind, handle)
	}
	var got string
	switch refType {
	case "provider":
		provider, _ := spec["provider"].(map[string]interface{})
		got, _ = provider["id"].(string)
	case "template":
		got, _ = spec["template"].(string)
	default:
		return fmt.Errorf("unknown reference type %q", refType)
	}
	if got != refHandle {
		return fmt.Errorf("recorded %s %q should reference %s %q by handle, got %q", kind, handle, refType, refHandle, got)
	}
	return nil
}

// shouldCarryDeployedAt asserts the recorded push carried a non-null deployedAt.
// For LLM provider templates this guards the regression where template pushes sent
// a nil deployedAt watermark (the CP then silently dropped template updates).
func (d *DPToCPSteps) shouldCarryDeployedAt(kind, handle string) error {
	_, err := d.waitForArtifact(kind, handle, func(a cpRecordedArtifact) bool {
		return a.DeployedAt != nil
	}, cpPushAssertTimeout)
	if err != nil {
		return fmt.Errorf("control plane copy of %s %q did not carry a deployed timestamp: %w", kind, handle, err)
	}
	return nil
}

func (d *DPToCPSteps) shouldBeUndeployed(kind, handle string) error {
	_, err := d.waitForArtifact(kind, handle, func(a cpRecordedArtifact) bool {
		return strings.EqualFold(a.Status, "undeployed")
	}, cpPushAssertTimeout)
	if err != nil {
		return fmt.Errorf("control plane did not receive an undeploy for %s %q: %w", kind, handle, err)
	}
	return nil
}

// --- gateway DB bookkeeping assertions --------------------------------------

func (d *DPToCPSteps) shouldRecordSyncStatus(status, kind, handle string) error {
	deadline := time.Now().Add(cpPushAssertTimeout)
	var last string
	for time.Now().Before(deadline) {
		got, err := d.queryArtifactColumn("cp_sync_status", kind, handle)
		if err == nil {
			last = got
			if strings.EqualFold(got, status) {
				return nil
			}
		}
		time.Sleep(cpPushPollInterval)
	}
	return fmt.Errorf("gateway did not record cp_sync_status %q for %s %q (last seen: %q)", status, kind, handle, last)
}

func (d *DPToCPSteps) shouldRecordCPArtifactID(kind, handle string) error {
	deadline := time.Now().Add(cpPushAssertTimeout)
	for time.Now().Before(deadline) {
		got, err := d.queryArtifactColumn("cp_artifact_id", kind, handle)
		if err == nil && got != "" {
			return nil
		}
		time.Sleep(cpPushPollInterval)
	}
	return fmt.Errorf("gateway did not record a cp_artifact_id for %s %q", kind, handle)
}

// queryArtifactColumn reads a single column of the gateway artifacts row for the
// given kind/handle via the DB reader sidecar. Returns "" (no error) when the row
// or column value is NULL/absent.
func (d *DPToCPSteps) queryArtifactColumn(column, kind, handle string) (string, error) {
	if !validSQLIdentifier.MatchString(column) {
		return "", fmt.Errorf("invalid column identifier %q", column)
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultDBQueryTimeout)
	defer cancel()
	query := fmt.Sprintf(
		"SELECT %s FROM artifacts WHERE kind = '%s' AND handle = '%s';",
		column, sqlLiteral(kind), sqlLiteral(handle),
	)
	return executeQuery(ctx, query)
}

// --- recorder HTTP helpers --------------------------------------------------

// findArtifact returns the recorded artifact for kind/handle, or ok=false if the
// control plane has not received it yet.
func (d *DPToCPSteps) findArtifact(kind, handle string) (cpRecordedArtifact, bool, error) {
	arts, err := d.getRecordedArtifacts()
	if err != nil {
		return cpRecordedArtifact{}, false, err
	}
	for _, a := range arts {
		if a.Kind == kind && a.Handle == handle {
			return a, true, nil
		}
	}
	return cpRecordedArtifact{}, false, nil
}

// waitForArtifact polls the recorder until an artifact matching kind/handle
// satisfies pred, or the timeout elapses.
func (d *DPToCPSteps) waitForArtifact(kind, handle string, pred func(cpRecordedArtifact) bool, timeout time.Duration) (cpRecordedArtifact, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	found := false
	var last cpRecordedArtifact
	for time.Now().Before(deadline) {
		a, ok, err := d.findArtifact(kind, handle)
		if err != nil {
			lastErr = err
		} else if ok {
			found, last = true, a
			if pred(a) {
				return a, nil
			}
		}
		time.Sleep(cpPushPollInterval)
	}
	if lastErr != nil {
		return cpRecordedArtifact{}, fmt.Errorf("timed out after %s (last query error: %w)", timeout, lastErr)
	}
	if found {
		return cpRecordedArtifact{}, fmt.Errorf("timed out after %s; artifact present but predicate unmet (last status=%q, pushCount=%d)", timeout, last.Status, last.PushCount)
	}
	return cpRecordedArtifact{}, fmt.Errorf("timed out after %s; artifact never received", timeout)
}

func (d *DPToCPSteps) getRecordedArtifacts() ([]cpRecordedArtifact, error) {
	req, err := http.NewRequest(http.MethodGet, d.state.Config.MockPlatformAPIURL+"/_test/artifacts", nil)
	if err != nil {
		return nil, err
	}
	resp, err := d.state.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query control plane recorder: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("control plane recorder returned status %d: %s", resp.StatusCode, string(body))
	}
	var out []cpRecordedArtifact
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse recorder response: %w (body: %s)", err, string(body))
	}
	return out, nil
}

func (d *DPToCPSteps) postJSON(path string, payload interface{}) error {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(http.MethodPost, d.state.Config.MockPlatformAPIURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.state.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call control plane recorder %s: %w", path, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("control plane recorder %s returned status %d: %s", path, resp.StatusCode, string(respBody))
	}
	return nil
}
