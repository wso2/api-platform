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
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/testpki"
)

// scoped returns a context containing test scope values.
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

type testServer struct {
	server   *http.Server
	listener net.Listener
	URL      string
}

func (s *testServer) Close() {
	_ = s.server.Close()
	_ = s.listener.Close()
}

func newTestServer(t *testing.T, handler http.Handler) *testServer {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp4", "127.0.0.1:0")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "operation not permitted") {
			t.Skipf("loopback listeners are unavailable: %v", err)
		}
		require.NoError(t, err)
	}
	server := &http.Server{Handler: handler}
	go func() { _ = server.Serve(listener) }()
	return &testServer{server: server, listener: listener, URL: "http://" + listener.Addr().String()}
}

func TestFunnelPublishesTheResponse(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"api-1"}`))
	}))
	defer srv.Close()

	ctx := scoped()
	f := newTestFunnel(0)

	resp, err := f.Post(ctx, srv.URL, nil, []byte(`{}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	published, err := Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, published.StatusCode)
	require.JSONEq(t, `{"id":"api-1"}`, published.Text())
}

func TestStaleResponseTrap(t *testing.T) {
	ctx := scoped()
	f := newTestFunnel(0)

	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
		_, err := f.Get(ctx, "http://127.0.0.1:1/never", nil)
		require.Error(t, err)

		_, err = Published(ctx)
		require.ErrorContains(t, err, "no response has been published")
	})
}

func TestClearHappensBeforeTheCall(t *testing.T) {
	ctx := scoped()
	f := newTestFunnel(0)

	var duringRequest struct {
		checked  atomic.Bool
		hadStale atomic.Bool
	}

	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !duringRequest.checked.Load() {
			duringRequest.checked.Store(true)
			_, err := Published(ctx)
			duringRequest.hadStale.Store(err == nil)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := f.Get(ctx, srv.URL, nil)
	require.NoError(t, err)
	require.True(t, duringRequest.checked.Load())

	duringRequest.checked.Store(false)
	_, err = f.Get(ctx, srv.URL, nil)
	require.NoError(t, err)

	require.False(t, duringRequest.hadStale.Load(),
		"the previous response must already be cleared when the next request is issued")
}

func TestTransientRetry(t *testing.T) {
	t.Run("a recognised transient response is retried transparently", func(t *testing.T) {
		var calls atomic.Int32
		srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
		var calls atomic.Int32
		srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
		srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
		srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":900967}`))
		}))
		defer srv.Close()

		resp, err := newTestFunnel(2).Get(scoped(), srv.URL, nil)
		require.NoError(t, err, "exhaustion is not a transport error")
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}

func TestNegativeRetryCountsStillIssueOneRequest(t *testing.T) {
	var calls atomic.Int32
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(Options{Timeout: 5 * time.Second})
	resp, err := client.Do(context.Background(), Request{URL: srv.URL}, -1, 0)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.EqualValues(t, 1, calls.Load())
}

func TestClientRejectsOversizedResponseBodies(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte{'x'}, int(maxResponseBodyBytes)+1))
	}))
	defer srv.Close()

	response, err := NewClient(Options{Timeout: 5 * time.Second}).Do(
		context.Background(), Request{URL: srv.URL}, 0, 0)
	require.Nil(t, response)
	require.ErrorContains(t, err, "response body")
	require.ErrorContains(t, err, "exceeds the 10485760-byte limit")
}

func TestNewFunnelClampsNegativeRetries(t *testing.T) {
	funnel := NewFunnel(NewClient(Options{}), -1, time.Second)
	require.Equal(t, 0, funnel.maxRetries)
}

func TestTLSVerificationIsSecureByDefaultAndCanBeOptedOutLocally(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := NewClient(Options{Timeout: 5 * time.Second}).Do(
		context.Background(), Request{URL: server.URL}, 0, 0)
	require.Error(t, err)

	response, err := NewClient(Options{
		Timeout:            5 * time.Second,
		InsecureSkipVerify: true,
	}).Do(context.Background(), Request{URL: server.URL}, 0, 0)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
}

func TestNewClientUsesPQCFirstTLSCurveDefaults(t *testing.T) {
	client := NewClient(Options{})
	transport, ok := client.http.Transport.(*http.Transport)
	require.True(t, ok)
	require.Equal(t,
		[]tls.CurveID{
			tls.X25519MLKEM768,
			secP256r1MLKEM768,
			secP384r1MLKEM1024,
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
			tls.CurveP521,
		},
		transport.TLSClientConfig.CurvePreferences)
	require.False(t, transport.TLSClientConfig.InsecureSkipVerify)

	insecureClient := NewClient(Options{InsecureSkipVerify: true})
	insecureTransport, ok := insecureClient.http.Transport.(*http.Transport)
	require.True(t, ok)
	require.True(t, insecureTransport.TLSClientConfig.InsecureSkipVerify)
}

func TestNewClientClonesAndNormalizesSuppliedTLSConfig(t *testing.T) {
	configured := &tls.Config{
		ServerName:       "example.test",
		CurvePreferences: []tls.CurveID{tls.CurveP521, tls.CurveP256},
	}
	originalCurves := append([]tls.CurveID(nil), configured.CurvePreferences...)

	client := NewClient(Options{TLSClientConfig: configured})
	transport, ok := client.http.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotSame(t, configured, transport.TLSClientConfig)
	require.Equal(t, "example.test", transport.TLSClientConfig.ServerName)
	require.Equal(t,
		[]tls.CurveID{
			tls.X25519MLKEM768,
			secP256r1MLKEM768,
			secP384r1MLKEM1024,
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
			tls.CurveP521,
		},
		transport.TLSClientConfig.CurvePreferences)
	require.Equal(t, originalCurves, configured.CurvePreferences)
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	read, err := f.Client().Do(ctx, Request{Method: http.MethodGet, URL: srv.URL}, 0, 0)
	require.NoError(t, err)
	require.Contains(t, read.Text(), "original")
	require.False(t, tcontext.Contains(ctx, ResponseKey),
		"an intermediate read must not publish")

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

func TestFunnelPublishRejectsNilAndPublishesProtocolResponse(t *testing.T) {
	funnel := newTestFunnel(0)
	ctx := scoped()
	require.Error(t, funnel.Publish(ctx, nil))

	want := &Response{StatusCode: 408, Method: "GET", URL: "http://127.0.0.1/slow"}
	require.NoError(t, funnel.Publish(ctx, want))
	got, err := Published(ctx)
	require.NoError(t, err)
	require.Same(t, want, got)
}

func TestFunnelRequiresScope(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := newTestFunnel(0).Get(context.Background(), srv.URL, nil)
	require.ErrorContains(t, err, "no local scope in context")
}

// Every client must negotiate the hybrid post-quantum ordering, including one built from a
// caller-supplied TLS config that only sets roots and a server name.
func TestSuppliedTLSConfigStillGetsPostQuantumCurveOrdering(t *testing.T) {
	supplied := &tls.Config{RootCAs: x509.NewCertPool(), ServerName: "platform-api"}
	c := NewClient(Options{TLSClientConfig: supplied})

	got := c.http.Transport.(*http.Transport).TLSClientConfig
	require.Equal(t, defaultCurvePreferences(), got.CurvePreferences)
	require.Equal(t, tls.X25519MLKEM768, got.CurvePreferences[0])

	// The caller's own settings survive, and their config is not mutated.
	require.Equal(t, "platform-api", got.ServerName)
	require.NotNil(t, got.RootCAs)
	require.Empty(t, supplied.CurvePreferences)
}

func TestNilTLSConfigKeepsPostQuantumCurveOrdering(t *testing.T) {
	c := NewClient(Options{})
	got := c.http.Transport.(*http.Transport).TLSClientConfig
	require.Equal(t, defaultCurvePreferences(), got.CurvePreferences)
}

// An explicit extra curve is appended after the defaults, never replacing them.
func TestExplicitCurvesAreAppendedAfterTheDefaults(t *testing.T) {
	c := NewClient(Options{TLSClientConfig: &tls.Config{CurvePreferences: []tls.CurveID{tls.CurveP521}}})
	got := c.http.Transport.(*http.Transport).TLSClientConfig
	require.Equal(t, tls.X25519MLKEM768, got.CurvePreferences[0])
	require.Subset(t, got.CurvePreferences, defaultCurvePreferences())
}

// newClientAuthServer serves the subject of the client certificate that arrived, or "none",
// and records the SNI of every handshake.
func newClientAuthServer(t *testing.T, auth tls.ClientAuthType, resumable bool) (*httptest.Server, *atomic.Value) {
	t.Helper()
	var sni atomic.Value
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subject := "none"
		if len(r.TLS.PeerCertificates) > 0 {
			subject = r.TLS.PeerCertificates[0].Subject.CommonName
		}
		_, _ = w.Write([]byte(subject + " " + r.Host))
	}))
	server.TLS = &tls.Config{
		ClientAuth:             auth,
		SessionTicketsDisabled: !resumable,
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			sni.Store(hello.ServerName)
			return nil, nil
		},
	}
	server.StartTLS()
	t.Cleanup(server.Close)
	return server, &sni
}

func clientCertificate(t *testing.T, name string) *tls.Certificate {
	t.Helper()
	set, err := testpki.Default()
	require.NoError(t, err)
	fixture, err := set.Get(name)
	require.NoError(t, err)
	cert, err := fixture.TLSCertificate(false)
	require.NoError(t, err)
	return &cert
}

func TestClientTLSPresentsTheCertificateEvenWhenTheServerNamesNoIssuer(t *testing.T) {
	server, _ := newClientAuthServer(t, tls.RequestClientCert, false)
	client := NewClient(Options{Timeout: 5 * time.Second})

	resp, err := client.Do(context.Background(), Request{URL: server.URL, TLS: &ClientTLS{
		Certificate: clientCertificate(t, "client-wrong-ca"), InsecureSkipVerify: true,
	}}, 0, 0)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(resp.Text(), "client-wrong-ca "), resp.Text())
	require.NotNil(t, resp.TLS)
	require.True(t, resp.TLS.ClientCertificateRequested)
	require.False(t, resp.TLS.DidResume)
	require.NotEmpty(t, resp.TLS.PeerCertificates)
}

func TestClientTLSWithoutACertificateAnswersTheRequestEmpty(t *testing.T) {
	server, _ := newClientAuthServer(t, tls.RequestClientCert, false)
	resp, err := NewClient(Options{Timeout: 5 * time.Second}).Do(context.Background(),
		Request{URL: server.URL, TLS: &ClientTLS{InsecureSkipVerify: true}}, 0, 0)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(resp.Text(), "none "), resp.Text())
	require.True(t, resp.TLS.ClientCertificateRequested)
}

func TestClientTLSReportsAServerThatDoesNotAskForACertificate(t *testing.T) {
	server, _ := newClientAuthServer(t, tls.NoClientCert, false)
	resp, err := NewClient(Options{Timeout: 5 * time.Second}).Do(context.Background(), Request{
		URL: server.URL, TLS: &ClientTLS{Certificate: clientCertificate(t, "client-valid"), InsecureSkipVerify: true},
	}, 0, 0)
	require.NoError(t, err)
	require.False(t, resp.TLS.ClientCertificateRequested)
	require.True(t, strings.HasPrefix(resp.Text(), "none "), resp.Text())
}

func TestClientTLSSendsTheChosenServerNameAndHost(t *testing.T) {
	server, sni := newClientAuthServer(t, tls.NoClientCert, false)
	resp, err := NewClient(Options{Timeout: 5 * time.Second}).Do(context.Background(), Request{
		URL: server.URL, Host: "api.example.test",
		TLS: &ClientTLS{ServerName: "localhost", InsecureSkipVerify: true},
	}, 0, 0)
	require.NoError(t, err)
	require.Equal(t, "localhost", sni.Load())
	require.Equal(t, "localhost", resp.TLS.ServerName)
	require.True(t, strings.HasSuffix(resp.Text(), " api.example.test"), resp.Text())
}

func TestClientTLSCanSendNoServerNameEvenToAHostnameURL(t *testing.T) {
	server, sni := newClientAuthServer(t, tls.NoClientCert, false)
	url := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	client := NewClient(Options{Timeout: 5 * time.Second})

	resp, err := client.Do(context.Background(), Request{
		URL: url, TLS: &ClientTLS{ServerName: "ignored.example", OmitServerName: true, InsecureSkipVerify: true},
	}, 0, 0)
	require.NoError(t, err)
	require.Equal(t, "", sni.Load())
	require.Equal(t, "", resp.TLS.ServerName)
	require.False(t, resp.TLS.ClientCertificateRequested)

	resp, err = client.Do(context.Background(), Request{URL: url, TLS: &ClientTLS{InsecureSkipVerify: true}}, 0, 0)
	require.NoError(t, err)
	require.Equal(t, "localhost", sni.Load())
	require.Equal(t, "localhost", resp.TLS.ServerName)
}

func TestClientTLSVerifiesTheServerUnlessToldNotTo(t *testing.T) {
	server, _ := newClientAuthServer(t, tls.NoClientCert, false)
	_, err := NewClient(Options{Timeout: 5 * time.Second}).Do(context.Background(),
		Request{URL: server.URL, TLS: &ClientTLS{}}, 0, 0)
	require.Error(t, err)
}

func TestClientTLSResumesOnlyThroughASharedCacheTheServerHonours(t *testing.T) {
	for _, tc := range []struct {
		name      string
		resumable bool
		want      bool
	}{
		{name: "server issues tickets", resumable: true, want: true},
		{name: "server refuses resumption", resumable: false, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, _ := newClientAuthServer(t, tls.NoClientCert, tc.resumable)
			client := NewClient(Options{Timeout: 5 * time.Second})
			opts := &ClientTLS{InsecureSkipVerify: true, Sessions: tls.NewLRUClientSessionCache(1)}
			first, err := client.Do(context.Background(), Request{URL: server.URL, TLS: opts}, 0, 0)
			require.NoError(t, err)
			require.False(t, first.TLS.DidResume)
			second, err := client.Do(context.Background(), Request{URL: server.URL, TLS: opts}, 0, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, second.TLS.DidResume)
		})
	}
}

func TestClientTLSKeepsTheSharedCurveOrderingAndLeavesTheSharedConfigAlone(t *testing.T) {
	client := NewClient(Options{Timeout: 5 * time.Second, TLSClientConfig: &tls.Config{ServerName: "platform-api"}})
	exchange := client.newClientTLSExchange(&ClientTLS{ServerName: "localhost", InsecureSkipVerify: true})
	config := exchange.client.Transport.(*http.Transport).TLSClientConfig
	require.Equal(t, defaultCurvePreferences(), config.CurvePreferences)
	require.Equal(t, "localhost", config.ServerName)
	require.True(t, exchange.client.Transport.(*http.Transport).DisableKeepAlives)
	require.Equal(t, "platform-api", client.tlsConfig.ServerName)
	require.False(t, client.tlsConfig.InsecureSkipVerify)
	require.Nil(t, client.tlsConfig.GetClientCertificate)
}

func TestPlainRequestsCarryNoTLSState(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer server.Close()
	resp, err := NewClient(Options{Timeout: 5 * time.Second, InsecureSkipVerify: true}).Do(
		context.Background(), Request{URL: server.URL}, 0, 0)
	require.NoError(t, err)
	require.Nil(t, resp.TLS)
	require.Nil(t, (&clientTLSExchange{}).state(nil))
}

func TestFunnelPublishesAClientTLSResponse(t *testing.T) {
	server, _ := newClientAuthServer(t, tls.RequestClientCert, false)
	ctx := scoped()
	funnel := NewFunnel(NewClient(Options{Timeout: 5 * time.Second}), 0, time.Millisecond)
	_, err := funnel.Send(ctx, Request{URL: server.URL, TLS: &ClientTLS{
		Certificate: clientCertificate(t, "client-valid"), InsecureSkipVerify: true,
	}})
	require.NoError(t, err)
	published, err := Published(ctx)
	require.NoError(t, err)
	require.True(t, published.TLS.ClientCertificateRequested)
	require.True(t, strings.HasPrefix(published.Text(), "client-valid "), published.Text())
}

// handshakeListener accepts one TLS connection, records the client certificate it saw, and
// reports whether any bytes followed the handshake.
func handshakeListener(t *testing.T, auth tls.ClientAuthType) (string, *tls.Certificate, <-chan string, <-chan int) {
	t.Helper()
	serverCert := clientCertificate(t, "ca-a")
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{*serverCert},
		ClientAuth:   auth,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	subjects := make(chan string, 1)
	bytesRead := make(chan int, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		tlsConn := conn.(*tls.Conn)
		if handshakeErr := tlsConn.Handshake(); handshakeErr != nil {
			return
		}
		state := tlsConn.ConnectionState()
		subject := "none"
		if len(state.PeerCertificates) > 0 {
			subject = state.PeerCertificates[0].Subject.CommonName
		}
		subjects <- subject
		_ = tlsConn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		n, _ := tlsConn.Read(make([]byte, 8))
		bytesRead <- n
	}()
	return ln.Addr().String(), serverCert, subjects, bytesRead
}

func TestHandshakeReportsTheRequestWithoutAnHTTPRequest(t *testing.T) {
	client := NewClient(Options{Timeout: 5 * time.Second})
	address, serverCert, subjects, bytesRead := handshakeListener(t, tls.RequestClientCert)
	state, err := client.Handshake(context.Background(), address, &ClientTLS{
		Certificate: clientCertificate(t, "client-valid"), ServerName: "localhost", InsecureSkipVerify: true,
	})
	require.NoError(t, err)
	require.True(t, state.ClientCertificateRequested)
	require.Equal(t, "localhost", state.ServerName)
	require.Equal(t, serverCert.Certificate[0], state.PeerCertificates[0].Raw)
	require.Equal(t, "client-valid", <-subjects)
	require.Zero(t, <-bytesRead)

	quietAddress, _, _, quietBytes := handshakeListener(t, tls.NoClientCert)
	state, err = client.Handshake(context.Background(), quietAddress, &ClientTLS{
		InsecureSkipVerify: true, ServerName: "localhost",
	})
	require.NoError(t, err)
	require.False(t, state.ClientCertificateRequested)
	require.NotEmpty(t, state.PeerCertificates)
	require.Zero(t, <-quietBytes)
}

func TestHandshakeRejectsAMissingConfigAndAClosedPort(t *testing.T) {
	_, err := NewClient(Options{}).Handshake(context.Background(), "127.0.0.1:1", nil)
	require.ErrorContains(t, err, "no client TLS")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := ln.Addr().String()
	require.NoError(t, ln.Close())
	_, err = NewClient(Options{Timeout: time.Second}).Handshake(context.Background(), address, &ClientTLS{InsecureSkipVerify: true})
	require.Error(t, err)
}

func TestHandshakeKeepsAShortTimeoutAndCapsALongOne(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	var mu sync.Mutex
	var held []net.Conn
	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			mu.Lock()
			held = append(held, conn)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, conn := range held {
			_ = conn.Close()
		}
	})
	address := ln.Addr().String()

	t.Run("shorter client timeout is kept", func(t *testing.T) {
		started := time.Now()
		_, err := NewClient(Options{Timeout: 200 * time.Millisecond}).Handshake(context.Background(), address,
			&ClientTLS{InsecureSkipVerify: true})
		require.Error(t, err)
		require.Less(t, time.Since(started), time.Second)
	})
	t.Run("longer client timeout is capped", func(t *testing.T) {
		started := time.Now()
		_, err := NewClient(Options{Timeout: 30 * time.Second}).Handshake(context.Background(), address,
			&ClientTLS{InsecureSkipVerify: true})
		elapsed := time.Since(started)
		require.Error(t, err)
		require.Greater(t, elapsed, time.Second)
		require.Less(t, elapsed, defaultHandshakeTimeout+3*time.Second)
	})
}
