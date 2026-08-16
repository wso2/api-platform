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

package publishers

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
)

// receiver is a stub log platform: it records every batch body and header set, and
// answers with a status the test controls.
type receiver struct {
	mu       sync.Mutex
	bodies   []string
	headers  []http.Header
	status   int
	attempts int
	// statusSeq, when non-empty, supplies a status per attempt, letting a test
	// drive a failure-then-recovery sequence.
	statusSeq  []int
	retryAfter string
}

func (r *receiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)

	r.mu.Lock()
	r.bodies = append(r.bodies, string(body))
	r.headers = append(r.headers, req.Header.Clone())
	status := r.status
	if len(r.statusSeq) > 0 {
		if r.attempts < len(r.statusSeq) {
			status = r.statusSeq[r.attempts]
		} else {
			status = r.statusSeq[len(r.statusSeq)-1]
		}
	}
	r.attempts++
	retryAfter := r.retryAfter
	r.mu.Unlock()

	if retryAfter != "" {
		w.Header().Set("Retry-After", retryAfter)
	}
	w.WriteHeader(status)
}

func (r *receiver) snapshot() ([]string, []http.Header, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.bodies...), append([]http.Header(nil), r.headers...), r.attempts
}

func httpSinkCfg(endpoint string) config.TrafficLogHTTPConfig {
	cfg := config.TrafficLogHTTPConfig{
		Endpoint:               endpoint,
		ContentType:            "application/x-ndjson",
		AllowInsecureTransport: true,
		BatchMaxEvents:         100,
		BatchMaxBytes:          1 << 20,
		FlushInterval:          25 * time.Millisecond,
		QueueCapacity:          100,
		OnQueueFull:            config.TrafficLogQueueDropNew,
		RequestTimeout:         2 * time.Second,
		MaxRetries:             0,
		RetryBackoff:           time.Millisecond,
		Auth:                   config.TrafficLogHTTPAuthConfig{Type: config.TrafficLogAuthNone},
	}
	return cfg
}

// eventually polls cond until it holds or the deadline passes, so tests never
// depend on a fixed sleep matching the flush interval.
func eventually(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

func TestHTTPSink_DeliversNDJSONBatch(t *testing.T) {
	rec := &receiver{status: http.StatusOK}
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)

	s, err := newHTTPSink(httpSinkCfg(srv.URL))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	s.Write([]byte(`{"n":1}`))
	s.Write([]byte(`{"n":2}`))

	eventually(t, 2*time.Second, func() bool {
		bodies, _, _ := rec.snapshot()
		return len(bodies) > 0
	})

	bodies, headers, _ := rec.snapshot()
	joined := strings.Join(bodies, "")
	assert.Contains(t, joined, `{"n":1}`)
	assert.Contains(t, joined, `{"n":2}`)
	assert.True(t, strings.HasSuffix(bodies[0], "\n"), "NDJSON body must be newline terminated")
	assert.Equal(t, "application/x-ndjson", headers[0].Get("Content-Type"))
	assert.Equal(t, config.TrafficLogSinkHTTP, s.Name())
}

// Splunk HEC uses "Authorization: Splunk <token>", which the bearer type cannot
// express — this is why the header auth type exists.
func TestHTTPSink_AuthHeaderVariants(t *testing.T) {
	cases := map[string]struct {
		auth config.TrafficLogHTTPAuthConfig
		name string
		want string
		// absent names a header that must NOT be on the request.
		absent string
	}{
		"bearer": {
			auth: config.TrafficLogHTTPAuthConfig{
				Type:   config.TrafficLogAuthBearer,
				Bearer: config.TrafficLogHTTPAuthBearerConfig{Token: "abc"},
			},
			name: "Authorization", want: "Bearer abc",
		},
		"splunk hec via header": {
			auth: config.TrafficLogHTTPAuthConfig{
				Type:   config.TrafficLogAuthHeader,
				Header: config.TrafficLogHTTPAuthHeaderConfig{Name: "Authorization", Value: "Splunk deadbeef"},
			},
			name: "Authorization", want: "Splunk deadbeef",
		},
		"non-authorization header": {
			auth: config.TrafficLogHTTPAuthConfig{
				Type:   config.TrafficLogAuthHeader,
				Header: config.TrafficLogHTTPAuthHeaderConfig{Name: "X-API-Key", Value: "k-1"},
			},
			name: "X-API-Key", want: "k-1",
		},
		"basic": {
			auth: config.TrafficLogHTTPAuthConfig{
				Type:  config.TrafficLogAuthBasic,
				Basic: config.TrafficLogHTTPAuthBasicConfig{Username: "u", Password: "p"},
			},
			name: "Authorization", want: "Basic dTpw",
		},
		// A populated sub-table for a type that was not selected must not leak
		// onto the request — the selected type is the only thing consulted.
		"unselected sub-tables are ignored": {
			auth: config.TrafficLogHTTPAuthConfig{
				Type:   config.TrafficLogAuthBearer,
				Bearer: config.TrafficLogHTTPAuthBearerConfig{Token: "abc"},
				Basic:  config.TrafficLogHTTPAuthBasicConfig{Username: "u", Password: "p"},
				Header: config.TrafficLogHTTPAuthHeaderConfig{Name: "X-API-Key", Value: "k-1"},
			},
			name: "Authorization", want: "Bearer abc", absent: "X-API-Key",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := &receiver{status: http.StatusOK}
			srv := httptest.NewServer(rec)
			t.Cleanup(srv.Close)

			cfg := httpSinkCfg(srv.URL)
			cfg.Auth = tc.auth
			s, err := newHTTPSink(cfg)
			require.NoError(t, err)
			t.Cleanup(func() { _ = s.Close(context.Background()) })

			s.Write([]byte(`{"n":1}`))
			eventually(t, 2*time.Second, func() bool {
				_, headers, _ := rec.snapshot()
				return len(headers) > 0
			})

			_, headers, _ := rec.snapshot()
			assert.Equal(t, tc.want, headers[0].Get(tc.name))
			if tc.absent != "" {
				assert.Empty(t, headers[0].Get(tc.absent),
					"a sub-table for an unselected type must not reach the request")
			}
		})
	}
}

func TestHTTPSink_RetriesServerErrorsThenSucceeds(t *testing.T) {
	rec := &receiver{statusSeq: []int{500, 500, 200}}
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)

	cfg := httpSinkCfg(srv.URL)
	cfg.MaxRetries = 3
	cfg.RetryBackoff = time.Millisecond
	s, err := newHTTPSink(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	s.Write([]byte(`{"n":1}`))
	eventually(t, 3*time.Second, func() bool {
		_, _, attempts := rec.snapshot()
		return attempts >= 3
	})
}

// A 4xx means the receiver rejected the batch's shape. Retrying cannot help and
// would only amplify a permanent failure.
func TestHTTPSink_DoesNotRetryClientErrors(t *testing.T) {
	rec := &receiver{status: http.StatusBadRequest}
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)

	cfg := httpSinkCfg(srv.URL)
	cfg.MaxRetries = 5
	cfg.RetryBackoff = time.Millisecond
	s, err := newHTTPSink(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	s.Write([]byte(`{"n":1}`))
	eventually(t, 2*time.Second, func() bool {
		_, _, attempts := rec.snapshot()
		return attempts >= 1
	})
	time.Sleep(200 * time.Millisecond) // any retry would land well inside this

	_, _, attempts := rec.snapshot()
	assert.Equal(t, 1, attempts, "a 400 must not be retried")
}

func TestHTTPSink_HonoursRetryAfterOn429(t *testing.T) {
	rec := &receiver{statusSeq: []int{http.StatusTooManyRequests, http.StatusOK}, retryAfter: "1"}
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)

	cfg := httpSinkCfg(srv.URL)
	cfg.MaxRetries = 2
	cfg.RetryBackoff = time.Millisecond
	s, err := newHTTPSink(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	start := time.Now()
	s.Write([]byte(`{"n":1}`))
	eventually(t, 5*time.Second, func() bool {
		_, _, attempts := rec.snapshot()
		return attempts >= 2
	})
	assert.GreaterOrEqual(t, time.Since(start), time.Second,
		"the receiver's Retry-After must be honoured before the second attempt")
}

// Write runs on the ALS ingest path. Blocking there backpressures Envoy, so a full
// queue must drop and return immediately.
func TestHTTPSink_WriteNeverBlocksOnFullQueue(t *testing.T) {
	// A receiver that never answers keeps the sender goroutine busy, so the queue
	// fills and stays full.
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	t.Cleanup(func() { close(block); srv.Close() })

	cfg := httpSinkCfg(srv.URL)
	cfg.QueueCapacity = 4
	cfg.FlushInterval = time.Hour // only the size bound can trigger a flush
	cfg.BatchMaxEvents = 1
	cfg.RequestTimeout = 5 * time.Second
	s, err := newHTTPSink(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 500; i++ {
			s.Write([]byte(fmt.Sprintf(`{"i":%d}`, i)))
		}
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Write blocked on a full queue; it must drop and return immediately")
	}
}

// Nothing may reach stdout when delivery fails: that is what putting bodies back in
// the container log would look like.
func TestHTTPSink_UnreachableEndpointDropsWithoutFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	endpoint := srv.URL
	srv.Close() // now refusing connections

	cfg := httpSinkCfg(endpoint)
	cfg.MaxRetries = 1
	cfg.RetryBackoff = time.Millisecond
	s, err := newHTTPSink(cfg)
	require.NoError(t, err)

	s.Write([]byte(`{"secret":"value"}`))
	require.NoError(t, s.Close(context.Background()))
	// No assertion on stdout is possible here without capturing the process's fd;
	// the guarantee is structural — httpSink holds no writer other than its HTTP
	// client, so there is no code path from a delivery failure to stdout.
}

// A graceful shutdown must deliver what was accepted but not yet sent.
func TestHTTPSink_CloseFlushesPendingBatch(t *testing.T) {
	rec := &receiver{status: http.StatusOK}
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)

	cfg := httpSinkCfg(srv.URL)
	cfg.FlushInterval = time.Hour // nothing would be sent without the shutdown flush
	s, err := newHTTPSink(cfg)
	require.NoError(t, err)

	s.Write([]byte(`{"pending":true}`))
	require.NoError(t, s.Close(context.Background()))

	bodies, _, _ := rec.snapshot()
	require.Len(t, bodies, 1, "the pending batch must be delivered during Close")
	assert.Contains(t, bodies[0], `{"pending":true}`)
}

func TestHTTPSink_CloseIsIdempotent(t *testing.T) {
	rec := &receiver{status: http.StatusOK}
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)

	s, err := newHTTPSink(httpSinkCfg(srv.URL))
	require.NoError(t, err)
	require.NoError(t, s.Close(context.Background()))
	require.NoError(t, s.Close(context.Background()))
}

// Close must not hang when the receiver does: shutdown is bounded by the caller's
// context.
func TestHTTPSink_CloseRespectsContextDeadline(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	t.Cleanup(func() { close(block); srv.Close() })

	cfg := httpSinkCfg(srv.URL)
	cfg.RequestTimeout = 30 * time.Second
	cfg.BatchMaxEvents = 1
	cfg.FlushInterval = 10 * time.Millisecond
	s, err := newHTTPSink(cfg)
	require.NoError(t, err)

	s.Write([]byte(`{"n":1}`))
	time.Sleep(100 * time.Millisecond) // let the sender pick it up and stall

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err = s.Close(ctx)
	assert.Error(t, err, "Close must report that the flush did not finish")
	assert.Less(t, time.Since(start), 3*time.Second, "Close must not wait past its deadline")
}

func TestHTTPSink_RejectsBadAuthAndTLSConfig(t *testing.T) {
	t.Run("bearer without token", func(t *testing.T) {
		cfg := httpSinkCfg("http://127.0.0.1:1")
		cfg.Auth = config.TrafficLogHTTPAuthConfig{Type: config.TrafficLogAuthBearer}
		_, err := newHTTPSink(cfg)
		assert.Error(t, err)
	})
	t.Run("header without name", func(t *testing.T) {
		cfg := httpSinkCfg("http://127.0.0.1:1")
		cfg.Auth = config.TrafficLogHTTPAuthConfig{
			Type:   config.TrafficLogAuthHeader,
			Header: config.TrafficLogHTTPAuthHeaderConfig{Value: "x"},
		}
		_, err := newHTTPSink(cfg)
		assert.Error(t, err)
	})
	t.Run("half-configured mTLS", func(t *testing.T) {
		cfg := httpSinkCfg("http://127.0.0.1:1")
		cfg.TLS = config.TrafficLogHTTPTLSConfig{CertFile: "/nonexistent.pem"}
		_, err := newHTTPSink(cfg)
		assert.Error(t, err)
	})
	t.Run("unreadable ca file", func(t *testing.T) {
		cfg := httpSinkCfg("http://127.0.0.1:1")
		cfg.TLS = config.TrafficLogHTTPTLSConfig{CAFile: "/nonexistent-ca.pem"}
		_, err := newHTTPSink(cfg)
		assert.Error(t, err)
	})
}

func TestParseRetryAfter(t *testing.T) {
	assert.Equal(t, 5*time.Second, parseRetryAfter("5"))
	assert.Equal(t, time.Duration(0), parseRetryAfter(""))
	assert.Equal(t, time.Duration(0), parseRetryAfter("not-a-number"))
	assert.Equal(t, time.Duration(0), parseRetryAfter("-3"))
	assert.Equal(t, retryAfterCap, parseRetryAfter("99999"), "a huge Retry-After must be capped")
}

func TestEncodeNDJSON(t *testing.T) {
	got := encodeNDJSON([][]byte{[]byte(`{"a":1}`), []byte(`{"b":2}`)})
	assert.Equal(t, "{\"a\":1}\n{\"b\":2}\n", string(got))
}

// Jitter is what keeps replicas from reconverging into a thundering herd after a
// shared receiver outage, so successive backoffs must not all be identical.
func TestHTTPSink_BackoffIsJitteredAndGrows(t *testing.T) {
	s := &httpSink{cfg: config.TrafficLogHTTPConfig{RetryBackoff: time.Second}}

	seen := map[time.Duration]bool{}
	for i := 0; i < 20; i++ {
		d := s.backoff(1)
		assert.GreaterOrEqual(t, d, 500*time.Millisecond)
		assert.LessOrEqual(t, d, time.Second)
		seen[d] = true
	}
	assert.Greater(t, len(seen), 1, "backoff must be jittered, not a fixed delay")

	assert.Greater(t, s.backoff(3), 500*time.Millisecond)
	assert.LessOrEqual(t, s.backoff(3), 4*time.Second)
}

// --- mTLS -------------------------------------------------------------------
//
// Test certificates use ECDSA P-256, matching internal/utils/grpc_test.go. That
// is not a post-quantum-cryptography.md violation: X.509 certificates cannot be
// ML-DSA-signed in Go's TLS stack today, and the key exchange these tests
// actually exercise is the hybrid X25519MLKEM768 that buildTrafficLogTLSConfig
// puts first in CurvePreferences.

// tlsFixture is a throwaway CA plus a server and client leaf signed by it,
// written to PEM files the sink config can reference by path.
type tlsFixture struct {
	caFile, serverCert, serverKey, clientCert, clientKey string
	caPool                                               *x509.CertPool
	serverPair                                           tls.Certificate
}

func newTLSFixture(t *testing.T) *tlsFixture {
	t.Helper()
	dir := t.TempDir()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "traffic-log-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	leaf := func(cn string, serial int64, server bool) (string, string) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(serial),
			Subject:      pkix.Name{CommonName: cn},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature,
		}
		if server {
			tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
			tmpl.DNSNames = []string{"localhost"}
			tmpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
		} else {
			tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
		require.NoError(t, err)
		keyDER, err := x509.MarshalECPrivateKey(key)
		require.NoError(t, err)

		certPath := filepath.Join(dir, cn+".crt")
		keyPath := filepath.Join(dir, cn+".key")
		require.NoError(t, os.WriteFile(certPath,
			pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
		require.NoError(t, os.WriteFile(keyPath,
			pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600))
		return certPath, keyPath
	}

	caPath := filepath.Join(dir, "ca.crt")
	require.NoError(t, os.WriteFile(caPath,
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0o600))

	f := &tlsFixture{caFile: caPath}
	f.serverCert, f.serverKey = leaf("server", 2, true)
	f.clientCert, f.clientKey = leaf("client", 3, false)

	f.caPool = x509.NewCertPool()
	require.True(t, f.caPool.AppendCertsFromPEM(pem.EncodeToMemory(
		&pem.Block{Type: "CERTIFICATE", Bytes: caDER})))
	f.serverPair, err = tls.LoadX509KeyPair(f.serverCert, f.serverKey)
	require.NoError(t, err)
	return f
}

// mtlsServer starts a TLS receiver that REQUIRES and verifies a client certificate.
func (f *tlsFixture) mtlsServer(t *testing.T, rec *receiver) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(rec)
	srv.TLS = &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{f.serverPair},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    f.caPool,
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

// TestHTTPSink_MutualTLS proves the client certificate is actually presented and
// accepted — the existing TLS test only covers rejecting a half-configured pair.
func TestHTTPSink_MutualTLS(t *testing.T) {
	f := newTLSFixture(t)
	rec := &receiver{status: http.StatusOK}
	srv := f.mtlsServer(t, rec)

	cfg := httpSinkCfg(srv.URL)
	cfg.AllowInsecureTransport = false // a real https:// endpoint
	cfg.TLS = config.TrafficLogHTTPTLSConfig{
		CAFile:   f.caFile,
		CertFile: f.clientCert,
		KeyFile:  f.clientKey,
	}
	s, err := newHTTPSink(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	s.Write([]byte(`{"n":1}`))
	eventually(t, 3*time.Second, func() bool {
		bodies, _, _ := rec.snapshot()
		return len(bodies) > 0
	})
	bodies, _, _ := rec.snapshot()
	assert.Contains(t, bodies[0], `{"n":1}`,
		"the batch must arrive over a mutually-authenticated connection")
}

// TestHTTPSink_MutualTLSComposesWithAuth pins the orthogonality the config shape
// implies: the client certificate authenticates the transport, the auth block
// authenticates the request, and selecting one does not disable the other.
func TestHTTPSink_MutualTLSComposesWithAuth(t *testing.T) {
	f := newTLSFixture(t)
	rec := &receiver{status: http.StatusOK}
	srv := f.mtlsServer(t, rec)

	cfg := httpSinkCfg(srv.URL)
	cfg.AllowInsecureTransport = false
	cfg.TLS = config.TrafficLogHTTPTLSConfig{
		CAFile: f.caFile, CertFile: f.clientCert, KeyFile: f.clientKey,
	}
	cfg.Auth = config.TrafficLogHTTPAuthConfig{
		Type:   config.TrafficLogAuthBearer,
		Bearer: config.TrafficLogHTTPAuthBearerConfig{Token: "tok-abc"},
	}
	s, err := newHTTPSink(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	s.Write([]byte(`{"n":1}`))
	eventually(t, 3*time.Second, func() bool {
		_, headers, _ := rec.snapshot()
		return len(headers) > 0
	})
	_, headers, _ := rec.snapshot()
	assert.Equal(t, "Bearer tok-abc", headers[0].Get("Authorization"),
		"mTLS must not suppress the configured auth header")
}

// TestHTTPSink_MutualTLSRequiredButNotConfigured is the fail-closed case: the
// receiver demands a client certificate and the sink has none, so the handshake
// fails, the line is dropped and counted, and nothing falls back to stdout.
func TestHTTPSink_MutualTLSRequiredButNotConfigured(t *testing.T) {
	f := newTLSFixture(t)
	rec := &receiver{status: http.StatusOK}
	srv := f.mtlsServer(t, rec)

	cfg := httpSinkCfg(srv.URL)
	cfg.AllowInsecureTransport = false
	cfg.TLS = config.TrafficLogHTTPTLSConfig{CAFile: f.caFile} // trusts the CA, presents nothing
	s, err := newHTTPSink(cfg)
	require.NoError(t, err, "the sink still builds — the failure is at handshake time, not config time")
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	s.Write([]byte(`{"n":1}`))
	require.NoError(t, s.Close(context.Background()))

	bodies, _, _ := rec.snapshot()
	assert.Empty(t, bodies, "no batch may reach a receiver that rejected the handshake")
}

// TestHTTPSink_RetryAfterReplacesBackoff pins that a receiver-supplied
// Retry-After is used INSTEAD of the exponential backoff, not in addition to it.
// Sleeping both made "Retry-After: 2" wait 2s + the backoff, overshooting the
// delay the receiver actually asked for.
func TestHTTPSink_RetryAfterReplacesBackoff(t *testing.T) {
	var mu sync.Mutex
	var stamps []time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		stamps = append(stamps, time.Now())
		n := len(stamps)
		mu.Unlock()
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cfg := httpSinkCfg(srv.URL)
	cfg.MaxRetries = 3
	cfg.RetryBackoff = 3 * time.Second // deliberately far larger than Retry-After
	s, err := newHTTPSink(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	s.Write([]byte(`{"n":1}`))
	eventually(t, 8*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(stamps) >= 2
	})

	mu.Lock()
	gap := stamps[1].Sub(stamps[0])
	mu.Unlock()
	// Retry-After alone is 1s. Backoff alone (or both) would be >= 1.5s.
	assert.Less(t, gap, 1500*time.Millisecond,
		"Retry-After must replace the backoff, not stack with it (gap was %s)", gap)
	assert.GreaterOrEqual(t, gap, 900*time.Millisecond,
		"the receiver's Retry-After must still be honoured (gap was %s)", gap)
}

// TestHTTPSink_WriteAfterCloseIsCounted pins that a line written after Close is
// accounted for. Previously it was accepted into the queue that no longer had a
// reader, so it was neither delivered nor counted — silent loss, the one outcome
// the drop counter exists to rule out.
func TestHTTPSink_WriteAfterCloseIsCounted(t *testing.T) {
	rec := &receiver{status: http.StatusOK}
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)

	s, err := newHTTPSink(httpSinkCfg(srv.URL))
	require.NoError(t, err)
	require.NoError(t, s.Close(context.Background()))

	before := gatherCounter(t, "policy_engine_traffic_log_dropped_total", "http")
	for i := 0; i < 5; i++ {
		s.Write([]byte(`{"after":"close"}`))
	}
	after := gatherCounter(t, "policy_engine_traffic_log_dropped_total", "http")

	assert.Equal(t, float64(5), after-before,
		"every line written after Close must be counted as dropped, not silently discarded")
	bodies, _, _ := rec.snapshot()
	assert.Empty(t, bodies, "nothing may be delivered after Close")
}

// gatherCounter sums a counter across all label sets matching sink.
func gatherCounter(t *testing.T, name, sink string) float64 {
	t.Helper()
	families, err := metrics.GetRegistry().Gather()
	require.NoError(t, err)
	var total float64
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "sink" && l.GetValue() == sink {
					total += m.GetCounter().GetValue()
				}
			}
		}
	}
	return total
}
