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

package config

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/wso2/api-platform/httpkit/tlsconfig"
)

// ParseAdminEcdhCurves parses a comma-separated EcdhCurves preference list
// (e.g. "X25519MLKEM768,X25519,P-256") into the tls.CurveID slice consumed by
// tls.Config.CurvePreferences. Used both to fail config validation closed on
// an unrecognized curve name and to build the admin TLS listener's config.
//
// The name-to-tls.CurveID vocabulary is sourced from httpkit/tlsconfig (the
// shared, direction-neutral implementation); this function keeps its own
// wrapper for the "empty string is an error" behavior, which differs from
// tlsconfig.ParseCurvePreferences's "empty means use Go's defaults" stance —
// an explicit, always-on TLS listener config has no notion of "unset".
func ParseAdminEcdhCurves(raw string) ([]tls.CurveID, error) {
	parts := strings.Split(raw, ",")
	curves := make([]tls.CurveID, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		curve, ok := tlsconfig.CurvesByName[name]
		if !ok {
			return nil, fmt.Errorf("unsupported ecdh curve %q (supported: X25519, P-256, P-384, P-521, X25519MLKEM768)", name)
		}
		curves = append(curves, curve)
	}
	if len(curves) == 0 {
		return nil, fmt.Errorf("must specify at least one ecdh curve")
	}
	return curves, nil
}

// ValidateAdminTLSVersions checks that min and max are both recognized
// version names and that min does not come after max. Unlike
// tlsconfig.ValidateVersionRange (which treats both-empty as "use Go's
// defaults"), this listener's config always requires both fields to name a
// real version — there is no "unset" state for an always-on TLS listener.
// tls.VersionTLSxx constants are monotonically increasing, so comparing the
// parsed values directly replaces a separate ordering table.
func ValidateAdminTLSVersions(minVersion, maxVersion string) error {
	minV, ok := tlsconfig.ParseVersion(minVersion)
	if !ok {
		return fmt.Errorf("minimum_protocol_version must be one of TLS1_0, TLS1_1, TLS1_2, TLS1_3, got: %q", minVersion)
	}
	if minV < tls.VersionTLS12 {
		return fmt.Errorf("minimum_protocol_version must be TLS1_2 or TLS1_3 (TLS1_0 and TLS1_1 are not permitted), got: %q", minVersion)
	}
	maxV, ok := tlsconfig.ParseVersion(maxVersion)
	if !ok {
		return fmt.Errorf("maximum_protocol_version must be one of TLS1_0, TLS1_1, TLS1_2, TLS1_3, got: %q", maxVersion)
	}
	if minV > maxV {
		return fmt.Errorf("minimum_protocol_version (%s) cannot be greater than maximum_protocol_version (%s)", minVersion, maxVersion)
	}
	return nil
}

// ParseAdminTLSVersion converts a validated version name to its crypto/tls
// identifier. Callers should run ValidateAdminTLSVersions first; an
// unrecognized name here returns ok=false rather than panicking.
func ParseAdminTLSVersion(name string) (version uint16, ok bool) {
	return tlsconfig.ParseVersion(name)
}

// ParseAdminCiphers parses a comma-separated list of Go crypto/tls cipher
// suite names (e.g. "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256") into the
// []uint16 consumed by tls.Config.CipherSuites. An empty string is valid and
// returns (nil, nil) — Go's own default suite set/order applies. Only
// affects TLS 1.2 (and below) connections; TLS 1.3 ignores this field.
func ParseAdminCiphers(raw string) ([]uint16, error) {
	return tlsconfig.ParseCipherSuites(raw)
}
