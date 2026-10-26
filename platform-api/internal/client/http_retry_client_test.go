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

package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/httpkit/netguard"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// fakePortal answers every https request with a redirect to location, but only for the first
// hop when redirectOnce is set; any non-https request is recorded and answered 200.
type fakePortal struct {
	location          string
	redirectOnce      bool
	redirected        bool
	httpHits          int
	httpAuthorization string
}

func (f *fakePortal) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme == "https" && !(f.redirectOnce && f.redirected) {
		f.redirected = true
		return &http.Response{
			StatusCode: http.StatusTemporaryRedirect,
			Header:     http.Header{"Location": {f.location}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	}
	if r.URL.Scheme != "https" {
		f.httpHits++
		f.httpAuthorization = r.Header.Get("Authorization")
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
}

// newClientOverFakePortal builds the retryable client over a shared client that permits http
// redirects (the shipped allowed_schemes default), then swaps in the fake portal transport.
func newClientOverFakePortal(t *testing.T, portal *fakePortal) *RetryableHTTPClient {
	t.Helper()
	policy := netguard.PermitPrivateBlockMetadata()
	policy.AllowedSchemes = []string{"http", "https"}
	utils.InitSharedHTTPClient(&http.Client{CheckRedirect: netguard.CheckRedirect(policy, 5)}, 0)
	t.Cleanup(func() { utils.InitSharedHTTPClient(nil, 0) })

	c, err := NewRetryableHTTPClient(RetryConfig{AttemptTimeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewRetryableHTTPClient: %v", err)
	}
	c.client.Transport = portal
	return c
}

func publishWithAuth(c *RetryableHTTPClient) (*http.Response, error) {
	req, _ := http.NewRequest(http.MethodPost, "https://portal.example/publish", strings.NewReader("x"))
	req.Header.Set("Authorization", "Bearer secret")
	return c.Do(req)
}

// flakyPortal answers 503 to the first failures requests and 200 after that, recording each request body.
type flakyPortal struct {
	failures int
	bodies   []string
}

func (f *flakyPortal) RoundTrip(r *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(r.Body)
	f.bodies = append(f.bodies, string(body))
	status := http.StatusOK
	if len(f.bodies) <= f.failures {
		status = http.StatusServiceUnavailable
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
}

func TestRetryableClientResendsBodyOnRetry(t *testing.T) {
	c := newClientOverFakePortal(t, &fakePortal{})
	c.cfg.MaxRetries = 1
	portal := &flakyPortal{failures: 1}
	c.client.Transport = portal

	resp, err := publishWithAuth(c)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 after the retry, got %d", resp.StatusCode)
	}
	if len(portal.bodies) != 2 || portal.bodies[0] != "x" || portal.bodies[1] != "x" {
		t.Fatalf("want the full body on both attempts, got %q", portal.bodies)
	}
}

type failingTransport struct{ calls int }

func (f *failingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	f.calls++
	return nil, errors.New("connection reset")
}

func TestRetryableClientDoesNotRetryUnrewindableBody(t *testing.T) {
	newReq := func(t *testing.T, body io.Reader) *http.Request {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, "https://portal.example/apis", io.NopCloser(body))
		if err != nil {
			t.Fatal(err)
		}
		return req
	}

	t.Run("5xx response", func(t *testing.T) {
		c := newClientOverFakePortal(t, &fakePortal{})
		c.cfg.MaxRetries = 2
		portal := &flakyPortal{failures: 5}
		c.client.Transport = portal

		resp, err := c.Do(newReq(t, strings.NewReader("x")))
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusServiceUnavailable || len(portal.bodies) != 1 {
			t.Fatalf("want the first 503 returned after one attempt, got %d after %d attempts", resp.StatusCode, len(portal.bodies))
		}
	})

	t.Run("transport error", func(t *testing.T) {
		c := newClientOverFakePortal(t, &fakePortal{})
		c.cfg.MaxRetries = 2
		portal := &failingTransport{}
		c.client.Transport = portal

		if _, err := c.Do(newReq(t, strings.NewReader("x"))); err == nil {
			t.Fatal("want the transport error")
		}
		if portal.calls != 1 {
			t.Fatalf("want one attempt, got %d", portal.calls)
		}
	})

	t.Run("bodyless request still retries", func(t *testing.T) {
		c := newClientOverFakePortal(t, &fakePortal{})
		c.cfg.MaxRetries = 2
		portal := &flakyPortal{failures: 1}
		c.client.Transport = portal

		req, err := http.NewRequest(http.MethodGet, "https://portal.example/apis", http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := c.Do(req)
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK || len(portal.bodies) != 2 {
			t.Fatalf("want 200 on the second attempt, got %d after %d attempts", resp.StatusCode, len(portal.bodies))
		}
	})
}

// A slow answer within the attempt timeout is not cut short by the shared client's
// response-header or overall limits.
func TestRetryableClientWaitsTheWholeAttemptForAnAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	utils.InitSharedHTTPClient(&http.Client{
		Timeout:   100 * time.Millisecond,
		Transport: &http.Transport{ResponseHeaderTimeout: 100 * time.Millisecond},
	}, 0)
	t.Cleanup(func() { utils.InitSharedHTTPClient(nil, 0) })

	c, err := NewRetryableHTTPClient(RetryConfig{AttemptTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewRetryableHTTPClient: %v", err)
	}
	req, _ := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("x"))
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("want the slow answer, got %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d", resp.StatusCode)
	}
}

func TestRetryableClientRefusesRedirectToHTTP(t *testing.T) {
	portal := &fakePortal{location: "http://portal.example/publish"}
	c := newClientOverFakePortal(t, portal)

	_, err := publishWithAuth(c)
	if !errors.Is(err, errInsecureRedirect) {
		t.Fatalf("want errInsecureRedirect, got %v", err)
	}
	if portal.httpHits != 0 || portal.httpAuthorization != "" {
		t.Fatalf("request reached the http hop (hits=%d, Authorization=%q)", portal.httpHits, portal.httpAuthorization)
	}
}

func TestRetryableClientFollowsSameHostHTTPSRedirect(t *testing.T) {
	portal := &fakePortal{location: "https://portal.example/publish/", redirectOnce: true}
	c := newClientOverFakePortal(t, portal)

	resp, err := publishWithAuth(c)
	if err != nil {
		t.Fatalf("https redirect should be followed, got %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 after following the redirect, got %d", resp.StatusCode)
	}
}

var retryTestConfig = RetryConfig{MaxRetries: 1, AttemptTimeout: 50 * time.Millisecond, Backoff: time.Millisecond}

func newRetryClientWith(t *testing.T, cfg RetryConfig, transport http.RoundTripper) *RetryableHTTPClient {
	t.Helper()
	c := newClientOverFakePortal(t, &fakePortal{})
	c.cfg = cfg
	c.client.Transport = transport
	return c
}

func getRequest(t *testing.T, ctx context.Context) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://portal.example/apis", nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func okResponse(r *http.Request, body io.ReadCloser) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: body, Request: r}
}

func TestNewRetryableHTTPClientRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  RetryConfig
	}{
		{"negative retries", RetryConfig{MaxRetries: -1, AttemptTimeout: time.Second}},
		{"zero attempt timeout", RetryConfig{}},
		{"negative attempt timeout", RetryConfig{AttemptTimeout: -time.Second}},
		{"negative backoff", RetryConfig{AttemptTimeout: time.Second, Backoff: -time.Second}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewRetryableHTTPClient(tc.cfg); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

func TestRetryableClientCutsOffASlowAttemptAndRetries(t *testing.T) {
	calls := 0
	c := newRetryClientWith(t, retryTestConfig, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			<-r.Context().Done()
			return nil, r.Context().Err()
		}
		return okResponse(r, io.NopCloser(strings.NewReader(""))), nil
	}))

	resp, err := c.Do(getRequest(t, context.Background()))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if calls != 2 {
		t.Fatalf("want the cut-off attempt and one retry, got %d attempts", calls)
	}
}

func TestRetryableClientGivesUpWhenEveryAttemptIsCutOff(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 2, AttemptTimeout: 20 * time.Millisecond, Backoff: time.Millisecond}
	calls := 0
	c := newRetryClientWith(t, cfg, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		<-r.Context().Done()
		return nil, r.Context().Err()
	}))

	started := time.Now()
	_, err := c.Do(getRequest(t, context.Background()))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want a deadline error, got %v", err)
	}
	if calls != cfg.MaxRetries+1 {
		t.Fatalf("want %d attempts, got %d", cfg.MaxRetries+1, calls)
	}
	if took, floor := time.Since(started), time.Duration(cfg.MaxRetries+1)*cfg.AttemptTimeout; took < floor {
		t.Fatalf("every attempt must run to its limit: took %s, want at least %s", took, floor)
	}
}

func TestRetryableClientAttemptLimitOutlivesDoUntilBodyClosed(t *testing.T) {
	var attemptCtx context.Context
	c := newRetryClientWith(t, retryTestConfig, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		attemptCtx = r.Context()
		body := readFailsWhenCancelled{r: strings.NewReader("payload"), ctx: r.Context()}
		return okResponse(r, io.NopCloser(&body)), nil
	}))

	resp, err := c.Do(getRequest(t, context.Background()))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if attemptCtx.Err() != nil {
		t.Fatalf("the attempt was cancelled before the body was read: %v", attemptCtx.Err())
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil || string(got) != "payload" {
		t.Fatalf("want the body readable after Do returns, got %q, %v", got, err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if attemptCtx.Err() == nil {
		t.Fatal("closing the body must release the attempt's context")
	}
}

type readFailsWhenCancelled struct {
	r   io.Reader
	ctx context.Context
}

func (b *readFailsWhenCancelled) Read(p []byte) (int, error) {
	if err := b.ctx.Err(); err != nil {
		return 0, err
	}
	return b.r.Read(p)
}

func TestRetryableClientCutsOffASlowBody(t *testing.T) {
	c := newRetryClientWith(t, RetryConfig{AttemptTimeout: 20 * time.Millisecond}, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return okResponse(r, io.NopCloser(blockingBody{ctx: r.Context()})), nil
	}))

	resp, err := c.Do(getRequest(t, context.Background()))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if _, err := io.ReadAll(resp.Body); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want the attempt limit to cut off the body read, got %v", err)
	}
}

type blockingBody struct{ ctx context.Context }

func (b blockingBody) Read([]byte) (int, error) {
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}

func TestRetryableClientRetryRules(t *testing.T) {
	tests := []struct {
		name      string
		retries   int
		status    int
		wantCalls int
	}{
		{"5xx is retried up to the limit", 2, http.StatusServiceUnavailable, 3},
		{"no retries sends once", 0, http.StatusServiceUnavailable, 1},
		{"4xx is final", 2, http.StatusConflict, 1},
		{"401 is final", 2, http.StatusUnauthorized, 1},
		{"404 is final", 2, http.StatusNotFound, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			cfg := RetryConfig{MaxRetries: tc.retries, AttemptTimeout: time.Second, Backoff: time.Millisecond}
			c := newRetryClientWith(t, cfg, roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
			}))

			resp, err := c.Do(getRequest(t, context.Background()))
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.status || calls != tc.wantCalls {
				t.Fatalf("want status %d after %d calls, got %d after %d", tc.status, tc.wantCalls, resp.StatusCode, calls)
			}
		})
	}
}

func TestRetryableClientWaitsTheBackoffBetweenAttempts(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 1, AttemptTimeout: time.Second, Backoff: 40 * time.Millisecond}
	var starts []time.Time
	c := newRetryClientWith(t, cfg, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		starts = append(starts, time.Now())
		return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	}))

	resp, err := c.Do(getRequest(t, context.Background()))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()
	if len(starts) != 2 || starts[1].Sub(starts[0]) < cfg.Backoff {
		t.Fatalf("want two attempts at least %s apart, got %v", cfg.Backoff, starts)
	}
}

func TestRetryableClientDoesNotRetryWithoutTimeForTheBackoff(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 3, AttemptTimeout: time.Second, Backoff: time.Hour}
	calls := 0
	c := newRetryClientWith(t, cfg, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("connection reset")
	}))

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	started := time.Now()
	if _, err := c.Do(getRequest(t, ctx)); err == nil {
		t.Fatal("want the transport error")
	}
	if calls != 1 || time.Since(started) > time.Minute {
		t.Fatalf("want one attempt and no wait for the backoff, got %d attempts after %s", calls, time.Since(started))
	}
}

func TestRetryableClientStopsWhenTheRequestIsCancelledDuringBackoff(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 3, AttemptTimeout: time.Second, Backoff: time.Hour}
	ctx, cancel := context.WithCancel(context.Background())
	c := newRetryClientWith(t, cfg, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		time.AfterFunc(10*time.Millisecond, cancel)
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	}))

	_, err := c.Do(getRequest(t, ctx))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestRetryableClientAttemptNeverOutlastsTheRequestDeadline(t *testing.T) {
	cfg := RetryConfig{AttemptTimeout: time.Hour}
	var attemptDeadline, requestDeadline time.Time
	c := newRetryClientWith(t, cfg, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		attemptDeadline, _ = r.Context().Deadline()
		return okResponse(r, io.NopCloser(strings.NewReader(""))), nil
	}))

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	requestDeadline, _ = ctx.Deadline()
	resp, err := c.Do(getRequest(t, ctx))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()
	if attemptDeadline.After(requestDeadline) {
		t.Fatalf("attempt deadline %s is after the request deadline %s", attemptDeadline, requestDeadline)
	}
}
