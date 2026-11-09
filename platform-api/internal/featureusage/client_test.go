/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package featureusage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient returns a collectorClient posting to a server that answers
// with the given statuses in turn (the last one repeats), each after delay.
func newTestClient(t *testing.T, tokens TokenSource, maxAttempts int, attemptTimeout, delay time.Duration, statuses ...int) (*collectorClient, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(calls.Add(1))
		time.Sleep(delay)
		w.WriteHeader(statuses[min(n, len(statuses))-1])
	}))
	t.Cleanup(server.Close)
	return &collectorClient{
		httpClient:     server.Client(),
		eventsURL:      server.URL + eventsPath,
		tokens:         tokens,
		maxAttempts:    maxAttempts,
		retryBackoff:   time.Millisecond,
		attemptTimeout: attemptTimeout,
	}, &calls
}

// send reports every request it made — including the extra one a 401
// triggers — while the 401 retry still stays outside the retry budget.
func TestSendCountsEveryRequestButNotThe401RetryAgainstTheBudget(t *testing.T) {
	tests := map[string]struct {
		statuses     []int
		maxAttempts  int
		wantRequests int
		wantOK       bool
	}{
		"401 then success":                        {[]int{401, 200}, 1, 2, true},
		"401, 503, then success within budget 2":  {[]int{401, 503, 200}, 2, 3, true},
		"401 then 503s until the budget is spent": {[]int{401, 503}, 2, 3, false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c, calls := newTestClient(t, &fakeTokens{}, tc.maxAttempts, time.Second, 0, tc.statuses...)
			requests, err := c.send(context.Background(), []byte(`{}`))
			if (err == nil) != tc.wantOK {
				t.Fatalf("err = %v, want success %v", err, tc.wantOK)
			}
			if requests != tc.wantRequests || int(calls.Load()) != tc.wantRequests {
				t.Errorf("reported %d requests, collector saw %d, want %d", requests, calls.Load(), tc.wantRequests)
			}
		})
	}
}

// flakyTokens fails the first `failures` fetches with a retryable error, like
// an IdP that is briefly unavailable, then hands out a token.
type flakyTokens struct {
	failures int
	fetches  atomic.Int32
}

func (f *flakyTokens) Token(context.Context) (string, error) {
	if int(f.fetches.Add(1)) <= f.failures {
		return "", &tokenError{msg: "token endpoint unavailable", retryable: true}
	}
	return "token", nil
}
func (*flakyTokens) Invalidate(string) {}

// An attempt whose token fetch fails uses up the retry budget but sends no
// collector request, so it is not counted as one.
func TestATokenFailureCountsAgainstTheBudgetButNotAsARequest(t *testing.T) {
	t.Run("recovers within the budget", func(t *testing.T) {
		c, calls := newTestClient(t, &flakyTokens{failures: 1}, 2, time.Second, 0, http.StatusOK)
		requests, err := c.send(context.Background(), []byte(`{}`))
		if err != nil || requests != 1 || calls.Load() != 1 {
			t.Fatalf("send = (%d, %v), collector saw %d; want one request and success", requests, err, calls.Load())
		}
	})
	t.Run("budget spent on token failures", func(t *testing.T) {
		tokens := &flakyTokens{failures: 10}
		c, calls := newTestClient(t, tokens, 2, time.Second, 0, http.StatusOK)
		requests, err := c.send(context.Background(), []byte(`{}`))
		if err == nil || requests != 0 || calls.Load() != 0 {
			t.Fatalf("send = (%d, %v), collector saw %d; want no request and a failure", requests, err, calls.Load())
		}
		if n := tokens.fetches.Load(); n != 2 {
			t.Errorf("token fetched %d times, want max_attempts = 2", n)
		}
	})
}

// slowTokens takes delay to hand out a token, like a slow IdP.
type slowTokens struct{ delay time.Duration }

func (s slowTokens) Token(ctx context.Context) (string, error) {
	select {
	case <-time.After(s.delay):
		return "token", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
func (slowTokens) Invalidate(string) {}

// A slow token fetch has its own deadline and does not eat the collector
// request's: each may take most of attemptTimeout, together more than it.
func TestTokenFetchDoesNotConsumeTheRequestBudget(t *testing.T) {
	const attemptTimeout = 300 * time.Millisecond
	c, calls := newTestClient(t, slowTokens{delay: 200 * time.Millisecond}, 1, attemptTimeout, 200*time.Millisecond, http.StatusOK)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := c.send(ctx, []byte(`{}`)); err != nil {
		t.Fatalf("send failed although each step fits its own budget: %v", err)
	}
	if calls.Load() != 1 {
		t.Errorf("collector saw %d requests, want 1", calls.Load())
	}
}

// The overall delivery deadline still bounds the token fetch and the request
// together.
func TestTheDeliveryDeadlineBoundsBothSteps(t *testing.T) {
	c, calls := newTestClient(t, slowTokens{delay: 200 * time.Millisecond}, 1, time.Second, 200*time.Millisecond, http.StatusOK)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := c.send(ctx, []byte(`{}`)); err == nil {
		t.Fatal("send succeeded past the delivery deadline")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Errorf("send ran %s past a 250ms delivery deadline", elapsed)
	}
	_ = calls
}
