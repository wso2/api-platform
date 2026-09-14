/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package xdsclient

import (
	"fmt"
	"time"
)

// Config holds the xDS client configuration
type Config struct {
	// Server is the xDS server address (e.g., "localhost:18000")
	ServerAddress string

	// ConnectTimeout is the timeout for establishing initial connection
	ConnectTimeout time.Duration

	// RequestTimeout is the timeout for individual xDS requests
	RequestTimeout time.Duration

	// InitialReconnectDelay is the initial delay before reconnecting after connection failure
	InitialReconnectDelay time.Duration

	// MaxReconnectDelay is the maximum delay between reconnection attempts
	MaxReconnectDelay time.Duration

	// TLSEnabled indicates whether to use TLS for the gRPC connection
	TLSEnabled bool

	// TLSCertPath is the path to the TLS certificate file (if TLSEnabled)
	TLSCertPath string

	// TLSKeyPath is the path to the TLS private key file (if TLSEnabled)
	TLSKeyPath string

	// TLSCAPath is the path to the CA certificate for server verification (if TLSEnabled)
	TLSCAPath string

	// TLSCiphers is a comma-separated list of Go crypto/tls cipher suite
	// names restricting which suites this client offers. Empty means Go's
	// own secure default set/order applies. Only affects TLS 1.2 and below.
	TLSCiphers string

	// TLSEcdhCurves is a comma-separated list of TLS 1.3 key-exchange groups
	// this client offers, most preferred first (e.g. "X25519MLKEM768,X25519,P-256").
	// Required whenever TLSEnabled -- see config.ParseAdminEcdhCurves.
	TLSEcdhCurves string
}

// Validate validates the xDS client configuration
func (c *Config) Validate() error {
	if c.ServerAddress == "" {
		return fmt.Errorf("server address is required")
	}

	if c.ConnectTimeout <= 0 {
		return fmt.Errorf("connect timeout must be positive")
	}

	if c.RequestTimeout <= 0 {
		return fmt.Errorf("request timeout must be positive")
	}

	if c.InitialReconnectDelay <= 0 {
		return fmt.Errorf("initial reconnect delay must be positive")
	}

	if c.MaxReconnectDelay <= 0 {
		return fmt.Errorf("max reconnect delay must be positive")
	}

	if c.TLSEnabled {
		if c.TLSCertPath == "" {
			return fmt.Errorf("TLS cert path is required when TLS is enabled")
		}
		if c.TLSKeyPath == "" {
			return fmt.Errorf("TLS key path is required when TLS is enabled")
		}
		if c.TLSCAPath == "" {
			return fmt.Errorf("TLS CA path is required when TLS is enabled")
		}
		if c.TLSEcdhCurves == "" {
			return fmt.Errorf("TLS ECDH curves are required when TLS is enabled")
		}
	}

	return nil
}

// NewDefaultConfig creates a Config with sensible defaults
func NewDefaultConfig(serverAddress string) *Config {
	return &Config{
		ServerAddress:         serverAddress,
		ConnectTimeout:        DefaultConnectTimeout,
		RequestTimeout:        DefaultRequestTimeout,
		InitialReconnectDelay: DefaultInitialReconnectDelay,
		MaxReconnectDelay:     DefaultMaxReconnectDelay,
		TLSEnabled:            false,
	}
}
