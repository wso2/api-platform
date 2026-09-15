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

package main

import (
	"crypto/tls"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

// buildRESTAPITLSConfig translates a config.ServerTLSConfig into a tls.Config:
// bounded protocol version range, an optional cipher-suite restriction
// (TLS 1.2 and below only — TLS 1.3 suite selection isn't configurable in
// Go's crypto/tls), and the ECDH/group preference list, PQC hybrid group
// included when the operator has opted in. Config.Validate already rejects a
// bad version/cipher/curve value before this ever runs in production, so an
// error here can only come from a caller that bypassed validation.
func buildRESTAPITLSConfig(cfg *config.ServerTLSConfig) (*tls.Config, error) {
	if err := config.ValidateServerTLSVersions(cfg.MinimumProtocolVersion, cfg.MaximumProtocolVersion); err != nil {
		return nil, err
	}
	minVersion, _ := config.ParseServerTLSVersion(cfg.MinimumProtocolVersion)
	maxVersion, _ := config.ParseServerTLSVersion(cfg.MaximumProtocolVersion)

	cipherSuites, err := config.ParseServerCiphers(cfg.Ciphers)
	if err != nil {
		return nil, err
	}

	curves, err := config.ParseServerEcdhCurves(cfg.EcdhCurves)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		MinVersion:       minVersion,
		MaxVersion:       maxVersion,
		CipherSuites:     cipherSuites, // nil == Go's own secure default set/order
		CurvePreferences: curves,
	}, nil
}
