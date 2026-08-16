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

package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// scoped returns a context with block and runner scope, as the block engine provides.
func scoped() context.Context {
	return tcontext.WithLocal(
		tcontext.WithShared(context.Background(), tcontext.NewShared("b")),
		tcontext.NewLocal("r"),
	)
}

func newTestFunnel(retries int) *Funnel {
	return NewFunnel(NewClient(Options{
		Timeout:    5 * time.Second,
		MaxRetries: retries,
		RetryOn:    []TransientMatcher{TransientByCodeInBody(`"code":900967`)},
	}), retries, 10*time.Millisecond)
}

func TestFunnelPublishesTheResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"api-1"}`))
	}))
	defer srv.Close()

	ctx := scoped()
	f := newTestFunnel(0)

	resp, err := f.Post(ctx, srv.URL, nil, []byte(`{}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// An assertion step reads it back rather than receiving it directly.
	published, err := Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, published.StatusCode)
	require.JSONEq(t, `{"id":"api-1"}`, published.Text())
}

func TestStaleResponseTrap(t *testing.T) {
	// THE reason this layer exists. A step that fails after an earlier step published a
	// response must not leave that response behind for the next assertion to pass against.
	ctx := scoped()
	f := newTestFunnel(0)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"first":true}`))
	}))
	defer srv.Close()

	t.Run("a successful call publishes", func(t *testing.T) {
		_, err := f.Get(ctx, srv.URL, nil)
		require.NoError(t, err)
		published, err := Published(ctx)
		require.NoError(t, err)
		require.Contains(t, published.Text(), "first")
	})

	t.Run("a failing call leaves the response ABSENT, not stale", func(t *testing.T) {
		// Unroutable address: the request cannot happen at all.
		_, err := f.Get(ctx, "http://127.0.0.1:1/never", nil)
		require.Error(t, err)

		// Without clear-before-call, the first response would still be here and the next
		// assertion would pass against a call that never happened.
		_, err = Published(ctx)
		require.ErrorContains(t, err, "no response has been published")
	})
}

func TestClearHappensBeforeTheCall(t *testing.T) {
	// Proven by observing the context DURING the request: if clearing happened after the
	// call, the previous response would still be visible at this point.
	ctx := scoped()
	f := newTestFunnel(0)

	var duringRequest struct {
		checked  bool
		hadStale bool
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !duringRequest.checked {
			duringRequest.checked = true
			_, err := Published(ctx)
			duringRequest.hadStale = err == nil
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Publish something first.
	_, err := f.Get(ctx, srv.URL, nil)
	require.NoError(t, err)
	require.True(t, duringRequest.checked)

	// Second call: the handler checks whether the FIRST response is still published while
	// the second request is in flight.
	duringRequest.checked = false
	_, err = f.Get(ctx, srv.URL, nil)
	require.NoError(t, err)

	require.False(t, duringRequest.hadStale,
		"the previous response must already be cleared when the next request is issued")
}

func TestTransientRetry(t *testing.T) {
	t.Run("a recognised transient response is retried transparently", func(t *testing.T) {
		// Absorbed here, once, so a step issues a SINGLE call and asserts. A step that
		// hand-rolled a retry would eventually retry a real failure too.
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if calls.Add(1) < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"code":900967,"message":"General Error"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		defer srv.Close()

		resp, err := newTestFunnel(3).Get(scoped(), srv.URL, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.EqualValues(t, 3, calls.Load())
	})

	t.Run("an unrecognised 5xx is returned first try, not retried", func(t *testing.T) {
		// Matching on both a status class AND a body marker keeps a real server failure
		// fast and clear instead of slow and confusing.
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":500,"message":"real failure"}`))
		}))
		defer srv.Close()

		resp, err := newTestFunnel(3).Get(scoped(), srv.URL, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		require.EqualValues(t, 1, calls.Load(), "a real 5xx must not be retried")
	})

	t.Run("a 4xx is never retried", func(t *testing.T) {
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()

		resp, err := newTestFunnel(3).Get(scoped(), srv.URL, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		require.EqualValues(t, 1, calls.Load())
	})

	t.Run("exhausted retries return the last response for the step to assert", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":900967}`))
		}))
		defer srv.Close()

		resp, err := newTestFunnel(2).Get(scoped(), srv.URL, nil)
		require.NoError(t, err, "exhaustion is not a transport error")
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	// Several features assert on a 301/302 and its Location header. Following redirects
	// silently would make those assertions impossible to write.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/from" {
			http.Redirect(w, r, "/to", http.StatusMovedPermanently)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("arrived"))
	}))
	defer srv.Close()

	resp, err := newTestFunnel(0).Get(scoped(), srv.URL+"/from", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusMovedPermanently, resp.StatusCode)
	require.Equal(t, "/to", resp.Headers.Get("Location"))
	require.NotContains(t, resp.Text(), "arrived")
}

func TestIntermediateReadsDoNotDisturbTheAssertionTarget(t *testing.T) {
	// The GET-mutate-PUT pattern: the GET is consumed locally and must NOT become the
	// response under test. That is why the raw client is reachable and unpublishing.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"original"}`))
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"name":"updated"}`))
	}))
	defer srv.Close()

	ctx := scoped()
	f := newTestFunnel(0)

	// Intermediate read via the raw client — deliberately not the funnel.
	read, err := f.Client().Do(ctx, Request{Method: http.MethodGet, URL: srv.URL}, 0, 0)
	require.NoError(t, err)
	require.Contains(t, read.Text(), "original")
	require.False(t, tcontext.Contains(ctx, ResponseKey),
		"an intermediate read must not publish")

	// The PUT is the response under test.
	put, err := f.Put(ctx, srv.URL, nil, []byte(`{"name":"updated"}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, put.StatusCode)

	published, err := Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, published.StatusCode,
		"the published response must be the PUT, not the intermediate GET")
}

func TestResponseHelpers(t *testing.T) {
	t.Run("RequireSuccessWithBody names the call and its status", func(t *testing.T) {
		// Guards against parsing a failed or empty response, which yields an opaque JSON
		// error several frames from the real problem.
		r := &Response{StatusCode: 404, Method: "GET", URL: "http://x/apis/1", Body: []byte(`not found`)}
		err := r.RequireSuccessWithBody("fetching the API")
		require.ErrorContains(t, err, "fetching the API")
		require.ErrorContains(t, err, "GET http://x/apis/1 -> 404")
	})

	t.Run("a 2xx with an empty body is still rejected", func(t *testing.T) {
		r := &Response{StatusCode: 200, Method: "GET", URL: "http://x", Body: []byte("   ")}
		require.ErrorContains(t, r.RequireSuccessWithBody("reading"), "empty body")
	})

	t.Run("a nil response is reported rather than dereferenced", func(t *testing.T) {
		var r *Response
		require.ErrorContains(t, r.RequireSuccessWithBody("x"), "no response")
		require.Equal(t, "no response", r.Describe())
		require.False(t, r.Succeeded())
	})

	t.Run("Describe truncates a large body", func(t *testing.T) {
		big := make([]byte, 2000)
		for i := range big {
			big[i] = 'a'
		}
		r := &Response{StatusCode: 200, Method: "GET", URL: "http://x", Body: big}
		require.Contains(t, r.Describe(), "truncated")
		require.Less(t, len(r.Describe()), 700)
	})
}

func TestPublishedErrors(t *testing.T) {
	t.Run("absence is an error, not a nil response", func(t *testing.T) {
		// An assertion step with nothing published means no request was made or the
		// request step failed. Both deserve a clear failure.
		_, err := Published(scoped())
		require.ErrorContains(t, err, "no response has been published")
	})

	t.Run("a wrongly-typed published value is reported", func(t *testing.T) {
		ctx := scoped()
		require.NoError(t, tcontext.Set(ctx, ResponseKey, "not a response"))
		_, err := Published(ctx)
		require.ErrorContains(t, err, "not an *httpx.Response")
	})
}

func TestFunnelRequiresScope(t *testing.T) {
	// Publishing needs runner scope. Without it the framework is misconfigured, and saying
	// so beats a silent no-op that makes every later assertion fail mysteriously.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := newTestFunnel(0).Get(context.Background(), srv.URL, nil)
	require.ErrorContains(t, err, "no local scope in context")
}
