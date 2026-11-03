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

package tlsbackend

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/util/testpki"
)

func fixtures(t *testing.T) *testpki.Set {
	t.Helper()
	set, err := testpki.Generate(time.Now())
	require.NoError(t, err)
	return set
}

func backend(t *testing.T, set *testpki.Set, name, server, authority string) Backend {
	t.Helper()
	s, err := set.Get(server)
	require.NoError(t, err)
	a, err := set.Get(authority)
	require.NoError(t, err)
	return Backend{Name: name, Port: PortA, Certificate: string(s.CertPEM), PrivateKey: string(s.KeyPEM), ClientCAs: string(a.CertPEM)}
}

// start serves svc over TLS and returns its URL.
func start(t *testing.T, svc *Service) string {
	t.Helper()
	server := httptest.NewUnstartedServer(svc.Handler())
	server.TLS = svc.TLSConfig().Clone()
	server.StartTLS()
	t.Cleanup(server.Close)
	return server.URL
}

type reply struct {
	status  int
	subject string
	body    answer
}

func call(t *testing.T, set *testpki.Set, url, fixture string, withChain bool) reply {
	t.Helper()
	backendCA, err := set.Get("backend-ca")
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(backendCA.Certificate)
	config := &tls.Config{RootCAs: roots, ServerName: testpki.TLSBackendHost, MinVersion: tls.VersionTLS12}
	if fixture != "" {
		f, err := set.Get(fixture)
		require.NoError(t, err)
		cert, err := f.TLSCertificate(withChain)
		require.NoError(t, err)
		config.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) { return &cert, nil }
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: config, DisableKeepAlives: true}}
	resp, err := client.Get(url + "/anything")
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var body answer
	require.NoError(t, json.Unmarshal(raw, &body), string(raw))
	return reply{status: resp.StatusCode, subject: resp.Header.Get(HeaderClientSubject), body: body}
}

func TestBackendReportsTheSubjectOfAnAcceptedCertificate(t *testing.T) {
	set := fixtures(t)
	svc, err := New(backend(t, set, "a", "backend-server-a", "ca-a"))
	require.NoError(t, err)
	url := start(t, svc)

	got := call(t, set, url, "gw-identity-a", false)
	require.Equal(t, http.StatusOK, got.status)
	require.Equal(t, "CN=gateway-a", got.subject)
	require.Equal(t, answer{Backend: "a", Client: "CN=gateway-a"}, got.body)

	got = call(t, set, url, "gw-identity-via-intermediate", true)
	require.Equal(t, http.StatusOK, got.status)
	require.Equal(t, "CN=gateway-via-intermediate", got.subject)

	got = call(t, set, url, "gw-identity-no-eku", false)
	require.Equal(t, http.StatusOK, got.status, "a certificate without extended key usage is accepted")
}

func TestBackendRefusesWithoutASubjectWhenNoTrustedCertificateArrives(t *testing.T) {
	set := fixtures(t)
	svc, err := New(backend(t, set, "a", "backend-server-a", "ca-a"))
	require.NoError(t, err)
	url := start(t, svc)

	for _, tc := range []struct {
		fixture   string
		withChain bool
	}{
		{fixture: ""},
		{fixture: "gw-identity-b"},
		{fixture: "gw-identity-serverauth"},
		{fixture: "gw-identity-via-intermediate"},
		{fixture: "client-expired"},
		{fixture: "client-chain-depth-5", withChain: true},
	} {
		got := call(t, set, url, tc.fixture, tc.withChain)
		require.Equal(t, http.StatusBadRequest, got.status, tc.fixture)
		require.Empty(t, got.subject, tc.fixture)
		require.Equal(t, "a", got.body.Backend, tc.fixture)
		require.Empty(t, got.body.Client, tc.fixture)
		require.NotEmpty(t, got.body.Error, tc.fixture)
	}
}

func TestBackendServesConcurrentRequests(t *testing.T) {
	set := fixtures(t)
	svc, err := New(backend(t, set, "a", "backend-server-a", "ca-a"))
	require.NoError(t, err)
	url := start(t, svc)
	var wg sync.WaitGroup
	statuses := make([]int, 8)
	for i := range statuses {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			fixture := "gw-identity-a"
			if i%2 == 1 {
				fixture = ""
			}
			statuses[i] = call(t, set, url, fixture, false).status
		}(i)
	}
	wg.Wait()
	for i, status := range statuses {
		want := http.StatusOK
		if i%2 == 1 {
			want = http.StatusBadRequest
		}
		require.Equal(t, want, status, "request %d", i)
	}
}

func TestNewRejectsAnIncompleteBackend(t *testing.T) {
	set := fixtures(t)
	valid := backend(t, set, "a", "backend-server-a", "ca-a")
	for name, mutate := range map[string]func(*Backend){
		"blank name":       func(b *Backend) { b.Name = " " },
		"zero port":        func(b *Backend) { b.Port = 0 },
		"port above 65535": func(b *Backend) { b.Port = 65536 },
		"no certificate":   func(b *Backend) { b.Certificate = "" },
		"mismatched key": func(b *Backend) {
			other, _ := set.Get("key-mismatch")
			b.PrivateKey = string(other.KeyPEM)
		},
		"no authority": func(b *Backend) { b.ClientCAs = "not a certificate" },
	} {
		b := valid
		mutate(&b)
		_, err := New(b)
		require.Error(t, err, name)
	}
	svc, err := New(valid)
	require.NoError(t, err)
	require.Equal(t, "tls-backend-a", svc.Name())
	require.Equal(t, PortA, svc.Port())
	require.False(t, svc.Stateful())
	require.Equal(t, tls.RequestClientCert, svc.TLSConfig().ClientAuth)
	require.Equal(t, tls.X25519MLKEM768, svc.TLSConfig().CurvePreferences[0])
}

func TestFromEnvRoundTripsAndRejectsMalformedValues(t *testing.T) {
	set := fixtures(t)
	a := backend(t, set, "a", "backend-server-a", "ca-a")
	b := backend(t, set, "b", "backend-server-b", "ca-b")
	b.Port = PortB
	value, err := Encode([]Backend{a, b})
	require.NoError(t, err)
	services, err := FromEnv(value)
	require.NoError(t, err)
	require.Len(t, services, 2)
	require.Equal(t, "tls-backend-a", services[0].Name())
	require.Equal(t, PortB, services[1].Port())

	_, err = Encode(nil)
	require.Error(t, err)
	for _, bad := range []string{"", "  ", "{", "[]", `[{"name":"a","port":8443}]`} {
		_, err := FromEnv(bad)
		require.Error(t, err, bad)
	}
}

func TestOptionalBackendReportsAnyPresentedCertificateAndWhetherItVerified(t *testing.T) {
	set := fixtures(t)
	b := backend(t, set, "optional", "backend-server-a", "ca-a")
	b.Port, b.Optional = PortOptional, true
	svc, err := New(b)
	require.NoError(t, err)
	url := start(t, svc)

	for _, tc := range []struct {
		fixture, subject, verified string
	}{
		{"gw-identity-a", "CN=gateway-a", "true"},
		{"gw-identity-b", "CN=gateway-b", "false"},
		{"client-expired", "CN=client-expired", "false"},
		{"", "", "false"},
	} {
		got := callOptional(t, set, url, tc.fixture)
		require.Equal(t, http.StatusOK, got.status, tc.fixture)
		require.Equal(t, tc.subject, got.subject, tc.fixture)
		require.Equal(t, tc.verified, got.verifiedHeader, tc.fixture)
		require.Equal(t, optionalAnswer{Backend: "optional", Client: tc.subject, Verified: tc.verified == "true"}, got.body, tc.fixture)
	}
}

type optionalReply struct {
	status         int
	subject        string
	verifiedHeader string
	body           optionalAnswer
}

func callOptional(t *testing.T, set *testpki.Set, url, fixture string) optionalReply {
	t.Helper()
	backendCA, err := set.Get("backend-ca")
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(backendCA.Certificate)
	config := &tls.Config{RootCAs: roots, ServerName: testpki.TLSBackendHost, MinVersion: tls.VersionTLS12}
	if fixture != "" {
		f, err := set.Get(fixture)
		require.NoError(t, err)
		cert, err := f.TLSCertificate(false)
		require.NoError(t, err)
		config.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) { return &cert, nil }
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: config, DisableKeepAlives: true}}
	resp, err := client.Get(url + "/anything")
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var body optionalAnswer
	require.NoError(t, json.Unmarshal(raw, &body), string(raw))
	return optionalReply{status: resp.StatusCode, subject: resp.Header.Get(HeaderClientSubject),
		verifiedHeader: resp.Header.Get(HeaderClientVerified), body: body}
}

func TestOptionalBackendSendsTheHeaderAndBodyFieldWhenEmpty(t *testing.T) {
	set := fixtures(t)
	b := backend(t, set, "optional", "backend-server-a", "ca-a")
	b.Optional = true
	svc, err := New(b)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	svc.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	values, present := rec.Header()[HeaderClientSubject]
	require.True(t, present)
	require.Equal(t, []string{""}, values)
	require.Equal(t, []string{"false"}, rec.Header()[HeaderClientVerified])
	require.JSONEq(t, `{"backend":"optional","client":"","verified":false}`, rec.Body.String())
}

func TestBackendClosesTheConnectionOnlyWhenAsked(t *testing.T) {
	set := fixtures(t)
	for _, optional := range []bool{false, true} {
		b := backend(t, set, "closing", "backend-server-a", "ca-a")
		b.Optional = optional
		svc, err := New(b)
		require.NoError(t, err)

		plain := httptest.NewRecorder()
		svc.Handler().ServeHTTP(plain, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Empty(t, plain.Header().Get("Connection"), "optional=%t", optional)

		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.Header.Set(HeaderCloseConnection, "true")
		closing := httptest.NewRecorder()
		svc.Handler().ServeHTTP(closing, request)
		require.Equal(t, "close", closing.Header().Get("Connection"), "optional=%t", optional)
	}
}

func TestOptionalFlagRoundTripsThroughTheEnvironment(t *testing.T) {
	set := fixtures(t)
	b := backend(t, set, "optional", "backend-server-a", "ca-a")
	b.Port, b.Optional = PortOptional, true
	value, err := Encode([]Backend{b})
	require.NoError(t, err)
	require.Contains(t, value, `"optional":true`)
	services, err := FromEnv(value)
	require.NoError(t, err)
	require.Equal(t, PortOptional, services[0].Port())
}
