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

package streamfault

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServiceIdentity(t *testing.T) {
	s := New()
	require.Equal(t, "stream-fault", s.Name())
	require.Equal(t, Port, s.Port())
	require.False(t, s.Stateful())
}

// get issues one request against a live server, returning the response, the body bytes that
// arrived, and the error the body read ended with.
func get(t *testing.T, srv *httptest.Server, mode, seconds string) (*http.Response, string, error) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/upstream/path", nil)
	require.NoError(t, err)
	if mode != "" {
		req.Header.Set(ModeHeader, mode)
	}
	if seconds != "" {
		req.Header.Set(DurationHeader, seconds)
	}
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	body, readErr := io.ReadAll(resp.Body)
	return resp, string(body), readErr
}

func TestWholeIsTheDefaultAndCompletes(t *testing.T) {
	srv := httptest.NewServer(New().Handler())
	defer srv.Close()

	for _, mode := range []string{"", ModeWhole, "not-a-mode"} {
		resp, body, readErr := get(t, srv, mode, "")
		require.NoError(t, readErr, "mode %q", mode)
		require.Equal(t, http.StatusOK, resp.StatusCode, "mode %q", mode)
		require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"), "mode %q", mode)
		require.Contains(t, body, "/upstream/path", "mode %q", mode)
		require.Contains(t, body, "[DONE]", "mode %q", mode)
	}
}

func TestFailRefusesBeforeAnyBody(t *testing.T) {
	srv := httptest.NewServer(New().Handler())
	defer srv.Close()

	resp, body, readErr := get(t, srv, ModeFail, "")
	require.NoError(t, readErr)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.Contains(t, body, "refused")
	require.NotContains(t, body, "[DONE]")
}

// TestTruncateCommitsThenCutsOff is the service's reason to exist: the status and the first
// chunk arrive, and the body then ends without the terminating chunk, so the read fails.
func TestTruncateCommitsThenCutsOff(t *testing.T) {
	srv := httptest.NewServer(New().Handler())
	defer srv.Close()

	resp, body, readErr := get(t, srv, ModeTruncate, "")
	require.Equal(t, http.StatusOK, resp.StatusCode, "the 200 must be delivered before the cut-off")
	require.Contains(t, body, "truncated", "the chunk that did arrive must be kept")
	require.Error(t, readErr, "a body ending without its terminating chunk must not read cleanly")
	require.True(t,
		strings.Contains(readErr.Error(), "unexpected EOF") ||
			strings.Contains(readErr.Error(), "connection reset"),
		"unexpected read error: %v", readErr)
}

func TestDripStreamsForItsDurationAndCompletes(t *testing.T) {
	srv := httptest.NewServer(New().Handler())
	defer srv.Close()

	started := time.Now()
	resp, body, readErr := get(t, srv, ModeDrip, "1")
	require.NoError(t, readErr)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, body, "dripping")
	require.Contains(t, body, "[DONE]")
	require.GreaterOrEqual(t, time.Since(started), time.Second,
		"drip must still be sending after its duration, or a route timeout cannot land mid-body")
}

func TestDripBoundsItsDuration(t *testing.T) {
	srv := httptest.NewServer(New().Handler())
	defer srv.Close()

	// An unparseable, zero or negative duration falls back to the default rather than
	// streaming forever or not at all, and an over-long one is clamped. Each is read under a
	// deadline shorter than any of those durations, so the assertion is that the stream is
	// still open when the deadline lands - a handler that resolved the duration to zero
	// would have finished.
	for _, seconds := range []string{"not-a-number", "0", "-3", "600"} {
		ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/x", nil)
		require.NoError(t, err)
		req.Header.Set(ModeHeader, ModeDrip)
		req.Header.Set(DurationHeader, seconds)

		resp, err := srv.Client().Do(req)
		require.NoError(t, err, "duration %q: the headers must arrive at once", seconds)
		_, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()
		require.Error(t, readErr, "duration %q: the stream must still be open at the deadline", seconds)
	}
	require.LessOrEqual(t, maxDripSeconds, 30, "drip must stay inside the suite's own timeouts")
}
