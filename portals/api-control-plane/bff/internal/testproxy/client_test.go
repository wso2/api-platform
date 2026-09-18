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

package testproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRelay(t *testing.T, mutate func(*RelayOptions)) *Relay {
	t.Helper()
	opts := RelayOptions{
		RequestTimeout:   5 * time.Second,
		MaxRequestBytes:  1 << 20,
		MaxResponseBytes: 1 << 20,
		MaxConcurrent:    4,
		MaxPending:       4,
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

// TestRelayForwardsOnlyTheSanitizedHeaders is the single most important test in
// this package.
//
// The BFF holds a Platform API bearer token for every logged-in caller, and the
// sibling reverse proxy in internal/proxy injects it on everything it forwards.
// A test-console target is a URL a tenant admin controls, so the same injection
// here would hand that token to whoever runs the gateway. This asserts the
// outbound request carries the envelope's headers and nothing else at all.
func TestRelayForwardsOnlyTheSanitizedHeaders(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	relay := testRelay(t, nil)
	_, err := relay.Do(context.Background(), targetFor(t, srv.URL), Envelope{
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

	// Only the caller's own header plus whatever Go's own transport must add
	// for a valid HTTP/1.1 request. Anything else would mean state from this
	// process leaked into a tenant-controlled destination.
	for name := range got {
		switch http.CanonicalHeaderKey(name) {
		case "X-Api-Key", "Host", "User-Agent", "Content-Length", "Accept-Encoding":
		default:
			t.Errorf("unexpected header forwarded to the gateway: %s", name)
		}
	}
}

func TestRelayReturnsGatewayErrorStatusAsData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream down"}`))
	}))
	defer srv.Close()

	relayed, err := testRelay(t, nil).Do(context.Background(), targetFor(t, srv.URL), Envelope{Method: "GET", Path: "/"})

	// The relay succeeded; the gateway is what failed. Collapsing these two
	// into one error would leave the console unable to tell them apart.
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

	_, err := testRelay(t, nil).Do(context.Background(), targetFor(t, srv.URL+"/pizza/v1"), Envelope{
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

	relay := testRelay(t, func(o *RelayOptions) { o.MaxResponseBytes = 1000 })
	relayed, err := relay.Do(context.Background(), targetFor(t, srv.URL), Envelope{Method: "GET", Path: "/"})

	require.NoError(t, err, "someone testing an API learns more from a truncated body than from an error")
	assert.True(t, relayed.Truncated)
	assert.Len(t, relayed.Body, 1000)
}

func TestRelayKeepsATruncatedTextBodyReadable(t *testing.T) {
	// The ceiling lands mid-rune here. Left as-is the body would be invalid
	// UTF-8 and would come back base64-encoded, which is unreadable in the
	// console — the opposite of why truncating is preferred to failing.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("é", 100))) // two bytes per rune
	}))
	defer srv.Close()

	relay := testRelay(t, func(o *RelayOptions) { o.MaxResponseBytes = 9 })
	relayed, err := relay.Do(context.Background(), targetFor(t, srv.URL), Envelope{Method: "GET", Path: "/"})

	require.NoError(t, err)
	assert.True(t, relayed.Truncated)
	assert.Equal(t, EncodingUTF8, relayed.BodyEncoding)
	assert.Equal(t, strings.Repeat("é", 4), relayed.Body)
}

func TestRelayRefusesAnOversizedRequestBody(t *testing.T) {
	relay := testRelay(t, func(o *RelayOptions) { o.MaxRequestBytes = 10 })
	_, err := relay.Do(context.Background(), targetFor(t, "http://127.0.0.1:1"), Envelope{
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

	relayed, err := testRelay(t, nil).Do(context.Background(), targetFor(t, srv.URL), Envelope{
		Method:  "GET",
		Path:    "/",
		Headers: []Header{{Name: "X-API-Key", Value: "test-key"}},
	})
	require.NoError(t, err)

	// The 3xx is a result the tester wants to see. Chasing it would also resend
	// the test key to a location the gateway chose rather than the one resolved.
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
		// A browser could not read either of these from a cross-origin call
		// without an explicit Access-Control-Expose-Headers, so relaying is
		// strictly more informative than a direct fetch would have been.
		w.Header().Add("Set-Cookie", "a=1")
		w.Header().Add("Set-Cookie", "b=2")
		w.Header().Set("X-Ratelimit-Remaining", "9")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	relayed, err := testRelay(t, nil).Do(context.Background(), targetFor(t, srv.URL), Envelope{Method: "GET", Path: "/"})
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

	relay := testRelay(t, func(o *RelayOptions) {
		o.MaxConcurrent = 1
		o.MaxPending = 1
	})
	target := targetFor(t, srv.URL)

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = relay.Do(context.Background(), target, Envelope{Method: "GET", Path: "/"})
		}()
	}
	// Let both goroutines take their slots: one running, one queued.
	require.Eventually(t, func() bool { return relay.inFlight.Load() == 2 }, time.Second, 5*time.Millisecond)

	_, err := relay.Do(context.Background(), target, Envelope{Method: "GET", Path: "/"})
	assert.ErrorIs(t, err, ErrBusy, "a bounded pool with an unbounded queue is only delayed memory growth")

	unblock()
	wg.Wait()
}

func TestRelayRefusesLinkLocalMetadataEvenIfResolutionYieldsIt(t *testing.T) {
	// Defense in depth behind the resolver: if a gateway record ever named the
	// cloud metadata endpoint, the dial-time guard still refuses it.
	relay := testRelay(t, func(o *RelayOptions) { o.RequestTimeout = 2 * time.Second })
	_, err := relay.Do(context.Background(), targetFor(t, "http://169.254.169.254"), Envelope{Method: "GET", Path: "/latest/meta-data/"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disallowed address")
	// This error names the resolved host, which is exactly why the handler logs
	// it rather than returning it — see TestInvokeDoesNotLeakTheResolvedTarget.
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

func TestNewRelayRejectsMalformedCIDRs(t *testing.T) {
	_, err := NewRelay(RelayOptions{
		RequestTimeout:   time.Second,
		MaxResponseBytes: 1,
		MaxConcurrent:    1,
		DenyCIDRs:        []string{"not-a-cidr"},
	})
	assert.Error(t, err)
}
