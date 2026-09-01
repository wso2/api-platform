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

package runtime

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/components"
)

// Phase is one stage of bringing a component up. The phases are ordered and each is a
// strictly stronger claim than the last.
type Phase int

const (
	// PhaseStarted means the container is running and its declared ports are listening.
	PhaseStarted Phase = iota

	// PhaseHealthy means the component returned its declared ready status.
	PhaseHealthy

	// PhaseSchemaApplied means the component's storage and schema are ready.
	PhaseSchemaApplied
)

// String renders the phase for diagnostics.
func (p Phase) String() string {
	switch p {
	case PhaseStarted:
		return "started"
	case PhaseHealthy:
		return "healthy"
	case PhaseSchemaApplied:
		return "schema-applied"
	default:
		return fmt.Sprintf("phase(%d)", int(p))
	}
}

// Prober performs one health attempt. Injectable so the gate's timing and
// classification logic can be tested without containers.
type Prober interface {
	// Probe returns the observed status, or an error if the attempt could not be made
	// at all (connection refused during warm-up, for instance).
	Probe(ctx context.Context, url string) (int, error)
}

// HTTPProber probes over HTTP, tolerating self-signed certificates.
//
// TLS verification is deliberately skipped: components under test present certificates
// generated per block, and asserting the test harness's trust of them proves nothing
// about the product. Certificate behaviour is a product assertion, made by a test.
type HTTPProber struct {
	client *http.Client
}

// NewHTTPProber returns a prober with a per-attempt timeout.
//
// The timeout must be shorter than the health gate's overall budget, or one hung
// attempt consumes the entire window and the failure reports as "never became ready"
// rather than "stopped responding".
func NewHTTPProber(perAttempt time.Duration) *HTTPProber {
	if perAttempt <= 0 {
		perAttempt = 5 * time.Second
	}
	return &HTTPProber{client: &http.Client{
		Timeout: perAttempt,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // see doc comment
		},
	}}
}

// Probe implements Prober.
func (p *HTTPProber) Probe(ctx context.Context, url string) (int, error) {
	if p == nil || p.client == nil {
		return 0, fmt.Errorf("readiness: HTTP prober is not initialized")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("building health request for %s: %w", url, err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, nil
}

// AwaitHealthy polls a component's declared health endpoint until it answers with the
// declared status, and fails otherwise.
//
// A component with no health check declared is treated as ready at PhaseStarted, which
// is appropriate for a passive dependency with nothing meaningful to ask.
//
// The returned error is deliberately verbose. A readiness timeout is one of the most
// expensive failures to diagnose from CI output alone, so it reports the URL probed,
// how long was spent, how many attempts were made, and the last thing observed —
// distinguishing "answered the wrong status" from "never answered at all".
func AwaitHealthy(ctx context.Context, inst *components.Instance, prober Prober) error {
	if ctx == nil {
		return fmt.Errorf("readiness: context is required")
	}
	if inst == nil {
		return fmt.Errorf("readiness: instance is required")
	}
	def := inst.Definition()
	if def == nil {
		return fmt.Errorf("readiness: instance definition is required")
	}
	hc := def.Health
	if hc == nil {
		return nil // nothing to assert; PhaseStarted is this component's readiness
	}
	if hc.Timeout <= 0 || hc.Interval <= 0 {
		return fmt.Errorf("readiness: %s has invalid health timing", inst.Label())
	}

	base, err := inst.URL(hc.Endpoint)
	if err != nil {
		return fmt.Errorf("readiness: %s: %w", inst.Label(), err)
	}
	url := base + hc.Path

	if prober == nil {
		// Keep per-attempt well inside the overall budget so a single hung attempt
		// cannot swallow the window.
		perAttempt := hc.Timeout / 4
		if perAttempt > 10*time.Second {
			perAttempt = 10 * time.Second
		}
		prober = NewHTTPProber(perAttempt)
	}

	start := time.Now()
	deadline := start.Add(hc.Timeout)

	var (
		attempts  int
		lastCode  int
		lastErr   error
		sawAnswer bool
	)

	for {
		attempts++
		code, err := prober.Probe(ctx, url)
		switch {
		case err == nil && code == hc.ExpectStatus:
			return nil
		case err == nil:
			// Answering the wrong status is a different fault from not answering:
			// the component is up but not ready, or the path is wrong.
			sawAnswer = true
			lastCode = code
			lastErr = nil
		default:
			lastErr = err
		}

		// Honour caller cancellation before sleeping, so a cancelled run stops
		// promptly instead of burning the rest of the window.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("readiness: %s cancelled after %s and %d attempt(s) probing %s: %w",
				inst.Label(), time.Since(start).Round(time.Millisecond), attempts, url, ctxErr)
		}

		if remaining := time.Until(deadline); remaining <= 0 {
			break
		} else if remaining < hc.Interval {
			if !waitContext(ctx, remaining) {
				return fmt.Errorf("readiness: %s cancelled after %s and %d attempt(s) probing %s: %w",
					inst.Label(), time.Since(start).Round(time.Millisecond), attempts, url, ctx.Err())
			}
		} else {
			if !waitContext(ctx, hc.Interval) {
				return fmt.Errorf("readiness: %s cancelled after %s and %d attempt(s) probing %s: %w",
					inst.Label(), time.Since(start).Round(time.Millisecond), attempts, url, ctx.Err())
			}
		}

		if time.Now().After(deadline) {
			break
		}
	}

	observed := "no response at all"
	if sawAnswer {
		observed = fmt.Sprintf("last status %d (wanted %d)", lastCode, hc.ExpectStatus)
	} else if lastErr != nil {
		observed = fmt.Sprintf("last error: %v", lastErr)
	}

	return fmt.Errorf("readiness: %s did not become healthy within %s: probed %s %d time(s), %s. "+
		"A container that listens but never serves %d is a partial boot, not a ready component",
		inst.Label(), hc.Timeout, url, attempts, observed, hc.ExpectStatus)
}

func waitContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
