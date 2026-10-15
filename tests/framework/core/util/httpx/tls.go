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
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"sync/atomic"
)

// ClientTLS describes the TLS one HTTPS request negotiates. A request carrying it runs on a
// connection of its own, because a client certificate is negotiated per connection.
type ClientTLS struct {
	// Certificate is presented whenever the server asks for one, whether or not the server
	// names its issuer, so an untrusted certificate reaches the server. Nil presents none.
	Certificate *tls.Certificate
	// ServerName is the SNI value sent. Empty sends the URL host, which Go omits for an IP.
	ServerName string
	// OmitServerName sends no SNI whatever the URL host is. It ignores ServerName and needs
	// InsecureSkipVerify, since a certificate cannot be verified against no name.
	OmitServerName bool
	// Sessions is shared by requests that may resume one another's TLS session. Nil
	// disables resumption.
	Sessions tls.ClientSessionCache
	// InsecureSkipVerify disables server certificate and hostname verification, for a local
	// listener that serves a self-signed certificate.
	InsecureSkipVerify bool
}

// TLSState is what the handshake of a ClientTLS request negotiated.
type TLSState struct {
	// Version is the negotiated TLS version.
	Version uint16
	// ServerName is the SNI value the client sent.
	ServerName string
	// DidResume reports that the handshake resumed a cached session.
	DidResume bool
	// ClientCertificateRequested reports that the server sent a CertificateRequest.
	ClientCertificateRequested bool
	// PeerCertificates is the chain the server presented, leaf first.
	PeerCertificates []*x509.Certificate
}

// clientTLSExchange is the one-off client for a ClientTLS request and a record of whether
// its handshake saw a CertificateRequest.
type clientTLSExchange struct {
	client    *http.Client
	requested atomic.Bool
}

// newClientTLSExchange derives a single-connection client from the shared transport's TLS
// settings, keeping its roots and curve ordering.
func (c *Client) newClientTLSExchange(opts *ClientTLS) *clientTLSExchange {
	exchange := &clientTLSExchange{}
	config := c.tlsConfig.Clone()
	config.ServerName = opts.ServerName
	if opts.OmitServerName {
		config.ServerName = ""
	}
	config.InsecureSkipVerify = opts.InsecureSkipVerify //nolint:gosec // explicit opt-in for local self-signed listeners
	config.ClientSessionCache = opts.Sessions
	config.Certificates = nil
	config.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		exchange.requested.Store(true)
		if opts.Certificate == nil {
			return &tls.Certificate{}, nil
		}
		return opts.Certificate, nil
	}
	transport := &http.Transport{TLSClientConfig: config, DisableKeepAlives: true}
	if opts.OmitServerName {
		transport.DialTLSContext = dialWithoutServerName(config)
	}
	exchange.client = &http.Client{
		Timeout:       c.http.Timeout,
		CheckRedirect: c.http.CheckRedirect,
		Transport:     transport,
	}
	return exchange
}

// dialWithoutServerName dials and handshakes itself, because the transport fills an empty
// ServerName in from the URL host and a hostname URL would then send an SNI.
func dialWithoutServerName(config *tls.Config) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		raw, err := (&net.Dialer{}).DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		conn := tls.Client(raw, config)
		if err := conn.HandshakeContext(ctx); err != nil {
			_ = raw.Close()
			return nil, err
		}
		return conn, nil
	}
}

// state describes the handshake of a completed exchange.
func (e *clientTLSExchange) state(conn *tls.ConnectionState) *TLSState {
	if conn == nil {
		return nil
	}
	return &TLSState{
		Version:                    conn.Version,
		ServerName:                 conn.ServerName,
		DidResume:                  conn.DidResume,
		ClientCertificateRequested: e.requested.Load(),
		PeerCertificates:           conn.PeerCertificates,
	}
}
