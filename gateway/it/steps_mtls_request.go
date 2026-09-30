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

package it

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// httpsListenerAddr is the HTTPS listener's address, dialed directly at the
// TLS level to observe whether the server asks for a client certificate.
const httpsListenerAddr = "localhost:8443"

// mtlsListenerProbeTimeout and mtlsListenerProbeInterval absorb xDS
// propagation lag in the polling listener probes. The single-shot
// assertions probe once, after any pending propagation has settled.
// tlsDialTimeout bounds each TLS handshake a probe makes.
const (
	mtlsListenerProbeTimeout  = 15 * time.Second
	mtlsListenerProbeInterval = 500 * time.Millisecond
	tlsDialTimeout            = 10 * time.Second
)

// probeClientCertRequested reports whether the HTTPS listener sent a
// CertificateRequest, detected by the GetClientCertificate callback running.
// It answers with no certificate; the listener validates optionally, so the
// handshake succeeds either way.
func (m *mtlsSteps) probeClientCertRequested() (bool, error) {
	invoked := false
	conf := &tls.Config{
		InsecureSkipVerify: true, // the listener uses a self-signed default cert
		MinVersion:         tls.VersionTLS12,
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			invoked = true
			return &tls.Certificate{}, nil
		},
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: tlsDialTimeout}, "tcp", httpsListenerAddr, conf)
	if err != nil {
		return false, fmt.Errorf("failed to complete a TLS handshake against %s: %w", httpsListenerAddr, err)
	}
	defer conn.Close()
	return invoked, nil
}

func (m *mtlsSteps) httpsListenerShouldRequestClientCertificate() error {
	var lastErr error
	deadline := time.Now().Add(mtlsListenerProbeTimeout)
	for attempt := 0; attempt == 0 || time.Now().Before(deadline); attempt++ {
		if attempt > 0 {
			time.Sleep(mtlsListenerProbeInterval)
		}
		invoked, err := m.probeClientCertRequested()
		if err != nil {
			lastErr = err
			continue
		}
		if invoked {
			return nil
		}
		lastErr = fmt.Errorf("HTTPS listener at %s did not request a client certificate", httpsListenerAddr)
	}
	return lastErr
}

// httpsListenerShouldStopRequestingClientCertificate polls until the HTTPS
// listener no longer requests a client certificate.
func (m *mtlsSteps) httpsListenerShouldStopRequestingClientCertificate() error {
	var lastErr error
	deadline := time.Now().Add(mtlsListenerProbeTimeout)
	for attempt := 0; attempt == 0 || time.Now().Before(deadline); attempt++ {
		if attempt > 0 {
			time.Sleep(mtlsListenerProbeInterval)
		}
		invoked, err := m.probeClientCertRequested()
		if err != nil {
			lastErr = err
			continue
		}
		if !invoked {
			return nil
		}
		lastErr = fmt.Errorf("HTTPS listener at %s still requests a client certificate", httpsListenerAddr)
	}
	return lastErr
}

func (m *mtlsSteps) httpsListenerShouldNotRequestClientCertificate() error {
	if err := settlePendingPropagation(m.state); err != nil {
		return err
	}
	invoked, err := m.probeClientCertRequested()
	if err != nil {
		return err
	}
	if invoked {
		return fmt.Errorf("HTTPS listener at %s requested a client certificate, expected none", httpsListenerAddr)
	}
	return nil
}

// tlsClientWithCertificate builds a one-off client that presents the named
// fixture, optionally with its chain. Keep-alives are off because the
// certificate is negotiated per connection. The certificate is presented
// unconditionally, as curl does: Go's default selection would withhold one
// whose issuer the server did not name, hiding untrusted-certificate cases.
func (m *mtlsSteps) tlsClientWithCertificate(name string, includeChain bool) (*http.Client, error) {
	certPEM, err := m.readFixtureCert(name)
	if err != nil {
		return nil, err
	}
	if includeChain {
		chainPEM, err := m.readFixtureFile(name, ".chain.crt")
		if err != nil {
			return nil, err
		}
		combined := make([]byte, 0, len(certPEM)+len(chainPEM))
		combined = append(combined, certPEM...)
		combined = append(combined, chainPEM...)
		certPEM = combined
	}
	keyPEM, err := m.readFixtureFile(name, ".key")
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate fixture %q: %w", name, err)
	}
	return &http.Client{
		Timeout: m.state.Config.HTTPTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // the listener uses a self-signed default cert
				MinVersion:         tls.VersionTLS12,
				GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
					return &cert, nil
				},
			},
			DisableKeepAlives: true,
		},
	}, nil
}

// tlsClientNoCertificate builds a one-off *http.Client whose transport
// presents no client certificate at all.
func (m *mtlsSteps) tlsClientNoCertificate() *http.Client {
	return &http.Client{
		Timeout: m.state.Config.HTTPTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
			},
			DisableKeepAlives: true,
		},
	}
}

func (m *mtlsSteps) getWithClientCertificate(url, name string) error {
	client, err := m.tlsClientWithCertificate(name, false)
	if err != nil {
		return err
	}
	return m.httpSteps.SendRequestWithClient(client, http.MethodGet, url)
}

// getWithClientCertificateOnResumableSession sends a request with the named
// certificate from a client that caches its TLS session. Keep-alives stay off,
// so the next request through the same client opens a new connection and
// offers whatever session the gateway let it cache.
func (m *mtlsSteps) getWithClientCertificateOnResumableSession(url, name string) error {
	client, err := m.tlsClientWithCertificate(name, false)
	if err != nil {
		return err
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		return fmt.Errorf("client certificate transport is %T, expected *http.Transport", client.Transport)
	}
	transport.TLSClientConfig.ClientSessionCache = tls.NewLRUClientSessionCache(1)
	m.resumingClient = client
	return m.httpSteps.SendRequestWithClient(client, http.MethodGet, url)
}

func (m *mtlsSteps) getWithCachedTLSSession(url string) error {
	if m.resumingClient == nil {
		return fmt.Errorf("no TLS session cache - send a request on a resumable TLS session first")
	}
	return m.httpSteps.SendRequestWithClient(m.resumingClient, http.MethodGet, url)
}

func (m *mtlsSteps) gatewayShouldHaveRunFullTLSHandshake() error {
	resp := m.httpSteps.LastResponse()
	if resp == nil || resp.TLS == nil {
		return fmt.Errorf("the last response did not arrive over TLS")
	}
	if resp.TLS.DidResume {
		return fmt.Errorf("the gateway resumed the cached TLS session instead of running a full handshake")
	}
	return nil
}

func (m *mtlsSteps) getWithClientCertificateAndChain(url, name string) error {
	client, err := m.tlsClientWithCertificate(name, true)
	if err != nil {
		return err
	}
	return m.httpSteps.SendRequestWithClient(client, http.MethodGet, url)
}

func (m *mtlsSteps) getWithNoClientCertificate(url string) error {
	return m.httpSteps.SendRequestWithClient(m.tlsClientNoCertificate(), http.MethodGet, url)
}

func (m *mtlsSteps) requireJWTToken() error {
	if m.jwtSteps == nil || m.jwtSteps.currentToken == "" {
		return fmt.Errorf("no JWT token available - call 'I get a JWT token from the mock JWKS server' first")
	}
	return nil
}

// getWithJWTTokenAndClientCertificate sends the bearer JWT and the client
// certificate on the same request.
func (m *mtlsSteps) getWithJWTTokenAndClientCertificate(url, name string) error {
	if err := m.requireJWTToken(); err != nil {
		return err
	}
	restore := m.useHeaderForOneRequest("Authorization", "Bearer "+m.jwtSteps.currentToken)
	defer restore()
	return m.getWithClientCertificate(url, name)
}

// getWithJWTTokenAndNoClientCertificate sends the bearer JWT without a
// client certificate.
func (m *mtlsSteps) getWithJWTTokenAndNoClientCertificate(url string) error {
	if err := m.requireJWTToken(); err != nil {
		return err
	}
	restore := m.useHeaderForOneRequest("Authorization", "Bearer "+m.jwtSteps.currentToken)
	defer restore()
	return m.getWithNoClientCertificate(url)
}

// httpsListenerShouldPresentCertificateFile checks that the HTTPS listener's
// leaf certificate is byte-for-byte the certificate in the given PEM file.
func (m *mtlsSteps) httpsListenerShouldPresentCertificateFile(path string) error {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read expected listener certificate %q: %w", path, err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return fmt.Errorf("%q does not start with a PEM certificate", path)
	}
	expected := sha256.Sum256(block.Bytes)

	if err := settlePendingPropagation(m.state); err != nil {
		return err
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: tlsDialTimeout}, "tcp", httpsListenerAddr, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12})
	if err != nil {
		return fmt.Errorf("failed to complete a TLS handshake against %s: %w", httpsListenerAddr, err)
	}
	defer conn.Close()
	peers := conn.ConnectionState().PeerCertificates
	if len(peers) == 0 {
		return fmt.Errorf("the HTTPS listener presented no certificate")
	}
	got := sha256.Sum256(peers[0].Raw)
	if got != expected {
		return fmt.Errorf("the HTTPS listener presented %q (sha256 %x), expected the certificate in %q (sha256 %x)",
			peers[0].Subject.String(), got, path, expected)
	}
	return nil
}
