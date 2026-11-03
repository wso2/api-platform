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
	"fmt"
	"net"
	"net/http"
	"time"
)

// defaultHandshakeTimeout bounds one handshake. A shorter client timeout is kept. A longer
// one is not, so a probe cannot occupy a request-sized deadline.
const defaultHandshakeTimeout = 10 * time.Second

// Handshake completes one TLS handshake with address, a host:port, and closes the connection
// without sending a request. It reports what the handshake negotiated, including whether the
// server asked for a client certificate.
func (c *Client) Handshake(ctx context.Context, address string, opts *ClientTLS) (*TLSState, error) {
	if opts == nil {
		return nil, fmt.Errorf("httpx: handshake with %s: no client TLS", address)
	}
	exchange := c.newClientTLSExchange(opts)
	transport, ok := exchange.client.Transport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("httpx: handshake with %s: unexpected transport %T", address, exchange.client.Transport)
	}
	timeout := c.http.Timeout
	if timeout <= 0 || timeout > defaultHandshakeTimeout {
		timeout = defaultHandshakeTimeout
	}
	dialer := &tls.Dialer{NetDialer: &net.Dialer{Timeout: timeout}, Config: transport.TLSClientConfig}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := dialer.DialContext(dialCtx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("httpx: handshake with %s: %w", address, err)
	}
	defer func() { _ = conn.Close() }()
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return nil, fmt.Errorf("httpx: handshake with %s: unexpected connection %T", address, conn)
	}
	state := tlsConn.ConnectionState()
	return exchange.state(&state), nil
}
