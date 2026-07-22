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

package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// TLSClientOptions configures how the upstream (Platform API) certificate is
// trusted. The two are mutually exclusive in effect: when SkipVerify is true no
// verification happens and CAFile is ignored.
type TLSClientOptions struct {
	// CAFile is a PEM bundle appended to the system roots so a private or
	// self-signed upstream cert can be trusted with verification still on.
	CAFile string
	// SkipVerify disables certificate verification entirely (dev/demo only).
	SkipVerify bool
}

// NewTransport builds an *http.Transport for upstream calls with explicit
// timeouts and connection pooling. TLS applies only when the upstream URL is
// https:// — this transport is scheme-agnostic and does nothing for http://.
func NewTransport(opts TLSClientOptions) (*http.Transport, error) {
	tlsConf := &tls.Config{
		MinVersion: tls.VersionTLS12,
		// #nosec G402 — SkipVerify is an explicit, demo-gated escape hatch
		// (validated in config); the secure default is false.
		InsecureSkipVerify: opts.SkipVerify,
	}
	if !opts.SkipVerify && opts.CAFile != "" {
		pool, err := caPool(opts.CAFile)
		if err != nil {
			return nil, err
		}
		tlsConf.RootCAs = pool
	}
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       tlsConf,
	}, nil
}

// caPool returns the system root pool with the PEM bundle at path appended, so
// public CAs keep working alongside a private/self-signed upstream cert.
func caPool(path string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read control_plane_ca_file %q: %w", path, err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("no valid certificates in control_plane_ca_file %q", path)
	}
	return pool, nil
}
