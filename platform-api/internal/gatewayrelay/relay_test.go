/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package gatewayrelay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/internal/gatewayrelay/egress"
)

func gatewayRelay(t *testing.T, mutate func(*RelayOptions)) *Relay {
	t.Helper()
	// These tests dial httptest servers on loopback.
	policy, err := egress.Parse(egress.Spec{AllowHosts: []string{"*"}})
	require.NoError(t, err)
	opts := RelayOptions{
		RequestTimeout:   5 * time.Second,
		MaxRequestBytes:  1 << 20,
		MaxResponseBytes: 1 << 20,
		MaxConcurrent:    4,
		MaxPending:       4,
		Egress:           policy,
	}
	if mutate != nil {
		mutate(&opts)
	}
	r, err := NewRelay(opts)
	require.NoError(t, err)
	return r
}

func targetFor(t *testing.T, rawURL string) Target {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return Target{URL: u, GatewayID: "gw-test"}
}

// The gateway is tenant-controlled: it must receive the test request's own
// headers and nothing else, least of all the caller's token.
func TestRelayForwardsOnlyTheSanitizedHeaders(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	relay := gatewayRelay(t, nil)
	_, err := relay.Do(context.Background(), targetFor(t, srv.URL), Request{
		Method:  "GET",
		Path:    "/order",
		Headers: []Header{{Name: "X-API-Key", Value: "test-key"}},
	})
	require.NoError(t, err)

	assert.Equal(t, "test-key", got.Get("X-API-Key"))
	assert.Empty(t, got.Get("Authorization"), "the session's bearer token must never reach a gateway")
	assert.Empty(t, got.Get("Cookie"), "the session cookie must never reach a gateway")
	assert.Empty(t, got.Get("X-Forwarded-For"))
	assert.Empty(t, got.Get("X-Requested-By"))

	// Only the request's header, plus what Go's transport adds to any request.
	for name := range got {
		switch http.CanonicalHeaderKey(name) {
		case "X-Api-Key", "Host", "User-Agent", "Content-Length", "Accept-Encoding":
		default:
			t.Errorf("unexpected header forwarded to the gateway: %s", name)
		}
	}
}

// A deployment on a vhost of its own is routed by Host, so the relay sends the
// vhost while still connecting to the gateway's endpoint.
func TestRelaySendsTheDeploymentVhostAsHost(t *testing.T) {
	var gotHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
	}))
	defer srv.Close()

	target := targetFor(t, srv.URL+"/pizza")
	target.Host = "pizza.example.com"
	_, err := gatewayRelay(t, nil).Do(context.Background(), target, Request{Method: "GET", Path: "/"})

	require.NoError(t, err)
	assert.Equal(t, "pizza.example.com", gotHost)
}

func TestRelaySendsTheEndpointHostWithoutAVhost(t *testing.T) {
	var gotHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
	}))
	defer srv.Close()

	_, err := gatewayRelay(t, nil).Do(context.Background(), targetFor(t, srv.URL), Request{Method: "GET", Path: "/"})

	require.NoError(t, err)
	assert.Equal(t, strings.TrimPrefix(srv.URL, "http://"), gotHost)
}

func TestRelayReturnsGatewayErrorStatusAsData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream down"}`))
	}))
	defer srv.Close()

	relayed, err := gatewayRelay(t, nil).Do(context.Background(), targetFor(t, srv.URL), Request{Method: "GET", Path: "/"})

	// The relay succeeded; the gateway failed. The console must tell the two apart.
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, relayed.Status)
	assert.Equal(t, `{"error":"upstream down"}`, relayed.Body)
	assert.Equal(t, EncodingUTF8, relayed.BodyEncoding)
}

func TestRelaySendsMethodPathQueryAndBody(t *testing.T) {
	var (
		gotMethod string
		gotURI    string
		gotBody   string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotURI = r.URL.RequestURI()
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	_, err := gatewayRelay(t, nil).Do(context.Background(), targetFor(t, srv.URL+"/pizza/v1"), Request{
		Method: "post",
		Path:   "/order",
		Query:  []Header{{Name: "dryRun", Value: "true"}},
		Body:   `{"size":"L"}`,
	})
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, gotMethod, "the method is normalized before it is sent")
	assert.Equal(t, "/pizza/v1/order?dryRun=true", gotURI)
	assert.Equal(t, `{"size":"L"}`, gotBody)
}

func TestRelayTruncatesRatherThanFailsAnOversizedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", 5000)))
	}))
	defer srv.Close()

	relay := gatewayRelay(t, func(o *RelayOptions) { o.MaxResponseBytes = 1000 })
	relayed, err := relay.Do(context.Background(), targetFor(t, srv.URL), Request{Method: "GET", Path: "/"})

	require.NoError(t, err, "someone testing an API learns more from a truncated body than from an error")
	assert.True(t, relayed.Truncated)
	assert.Len(t, relayed.Body, 1000)
}

func TestRelayKeepsATruncatedTextBodyReadable(t *testing.T) {
	// The limit lands mid-rune; the body must stay readable text, not turn base64.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("é", 100))) // two bytes per rune
	}))
	defer srv.Close()

	relay := gatewayRelay(t, func(o *RelayOptions) { o.MaxResponseBytes = 9 })
	relayed, err := relay.Do(context.Background(), targetFor(t, srv.URL), Request{Method: "GET", Path: "/"})

	require.NoError(t, err)
	assert.True(t, relayed.Truncated)
	assert.Equal(t, EncodingUTF8, relayed.BodyEncoding)
	assert.Equal(t, strings.Repeat("é", 4), relayed.Body)
}

func TestRelayRefusesAnOversizedRequestBody(t *testing.T) {
	relay := gatewayRelay(t, func(o *RelayOptions) { o.MaxRequestBytes = 10 })
	_, err := relay.Do(context.Background(), targetFor(t, "http://127.0.0.1:1"), Request{
		Method: "POST",
		Path:   "/",
		Body:   strings.Repeat("a", 11),
	})
	assert.ErrorIs(t, err, ErrBodyTooLarge)
}

func TestRelayDoesNotFollowRedirects(t *testing.T) {
	var reachedSecret bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/secret" {
			reachedSecret = true
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/secret", http.StatusFound)
	}))
	defer srv.Close()

	relayed, err := gatewayRelay(t, nil).Do(context.Background(), targetFor(t, srv.URL), Request{
		Method:  "GET",
		Path:    "/",
		Headers: []Header{{Name: "X-API-Key", Value: "test-key"}},
	})
	require.NoError(t, err)

	// Following it would resend the test key to a location the gateway chose.
	assert.Equal(t, http.StatusFound, relayed.Status)
	assert.False(t, reachedSecret)
	var location string
	for _, h := range relayed.Headers {
		if http.CanonicalHeaderKey(h.Name) == "Location" {
			location = h.Value
		}
	}
	assert.Equal(t, "/secret", location)
}

func TestRelaySurfacesEveryResponseHeaderIncludingRepeats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Set-Cookie", "a=1")
		w.Header().Add("Set-Cookie", "b=2")
		w.Header().Set("X-Ratelimit-Remaining", "9")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	relayed, err := gatewayRelay(t, nil).Do(context.Background(), targetFor(t, srv.URL), Request{Method: "GET", Path: "/"})
	require.NoError(t, err)

	var cookies []string
	for _, h := range relayed.Headers {
		if http.CanonicalHeaderKey(h.Name) == "Set-Cookie" {
			cookies = append(cookies, h.Value)
		}
	}
	assert.Equal(t, []string{"a=1", "b=2"}, cookies, "a repeated header must not collapse to one value")
}

func TestRelayRejectsWhenPoolAndQueueAreBothFull(t *testing.T) {
	block := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(block) })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-block
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	defer unblock()

	relay := gatewayRelay(t, func(o *RelayOptions) {
		o.MaxConcurrent = 1
		o.MaxPending = 1
	})
	target := targetFor(t, srv.URL)

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = relay.Do(context.Background(), target, Request{Method: "GET", Path: "/"})
		}()
	}
	// Let both goroutines take their slots: one running, one queued.
	require.Eventually(t, func() bool { return relay.inFlight.Load() == 2 }, time.Second, 5*time.Millisecond)

	_, err := relay.Do(context.Background(), target, Request{Method: "GET", Path: "/"})
	assert.ErrorIs(t, err, ErrBusy, "a bounded pool with an unbounded queue is only delayed memory growth")

	unblock()
	wg.Wait()
}

// request_timeout covers the whole call, the wait for a slot included; a call
// that never got a slot is busy, since the gateway was never asked.
func TestRelayCountsTheWaitForASlotTowardTheTimeout(t *testing.T) {
	var reached atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	const timeout = 200 * time.Millisecond
	relay := gatewayRelay(t, func(o *RelayOptions) {
		o.MaxConcurrent = 1
		o.MaxPending = 1
		o.RequestTimeout = timeout
	})
	// Hold the only slot, as a long-running call would.
	release, err := relay.acquire(context.Background())
	require.NoError(t, err)
	defer release()

	started := time.Now()
	done := make(chan error, 1)
	go func() {
		_, err := relay.Do(context.Background(), targetFor(t, srv.URL), Request{Method: "GET", Path: "/"})
		done <- err
	}()
	select {
	case err := <-done:
		assert.ErrorIs(t, err, ErrBusy)
		assert.GreaterOrEqual(t, time.Since(started), timeout, "the call waited in the queue, not refused at once")
		assert.False(t, reached.Load(), "a busy call must not reach the gateway")
	case <-time.After(timeout + 800*time.Millisecond):
		t.Fatal("a queued call waited past request_timeout")
	}
}

// A free slot and an expired deadline are both ready, and select picks either;
// both must end as busy without calling the gateway.
func TestRelayDoesNotCallTheGatewayWithASlotTakenAfterTheDeadline(t *testing.T) {
	var reached atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	relay := gatewayRelay(t, nil)
	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	for i := 0; i < 32; i++ {
		_, err := relay.Do(expired, targetFor(t, srv.URL), Request{Method: "GET", Path: "/"})
		require.ErrorIs(t, err, ErrBusy)
	}
	assert.False(t, reached.Load())
	assert.Zero(t, relay.inFlight.Load(), "every slot was given back")
}

func TestRelayRefusesLinkLocalMetadataEvenIfResolutionYieldsIt(t *testing.T) {
	// Even when "*" is allowed, the dial-time guard refuses the metadata endpoint.
	relay := gatewayRelay(t, func(o *RelayOptions) { o.RequestTimeout = 2 * time.Second })
	_, err := relay.Do(context.Background(), targetFor(t, "http://169.254.169.254"), Request{Method: "GET", Path: "/latest/meta-data/"})
	require.Error(t, err)
	// The guard returns one opaque error for every refusal; the reason goes to the log.
	assert.Contains(t, err.Error(), "not allowed")
}

func TestNewRelayRejectsUnboundedOptions(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts RelayOptions
	}{
		{"no timeout", RelayOptions{MaxResponseBytes: 1, MaxConcurrent: 1}},
		{"no response ceiling", RelayOptions{RequestTimeout: time.Second, MaxConcurrent: 1}},
		{"no concurrency ceiling", RelayOptions{RequestTimeout: time.Second, MaxResponseBytes: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewRelay(tc.opts)
			assert.Error(t, err, "a zero bound is worse than the feature being off")
		})
	}
}

func TestNewRelayRequiresAnEgressPolicy(t *testing.T) {
	// A nil policy is a wiring mistake, never "no restrictions".
	_, err := NewRelay(RelayOptions{
		RequestTimeout:   time.Second,
		MaxRequestBytes:  1,
		MaxResponseBytes: 1,
		MaxConcurrent:    1,
	})
	assert.Error(t, err)
}
