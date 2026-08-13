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
)

// adminEcdhCurvesByName maps the names accepted in AdminTLSConfig.EcdhCurves
// to Go's crypto/tls group identifiers. X25519MLKEM768 is the FIPS 203
// ML-KEM-768 + X25519 hybrid group, implemented natively by Go 1.23+.
var adminEcdhCurvesByName = map[string]tls.CurveID{
	"X25519":         tls.X25519,
	"P-256":          tls.CurveP256,
	"P-384":          tls.CurveP384,
	"P-521":          tls.CurveP521,
	"X25519MLKEM768": tls.X25519MLKEM768,
}

// ParseAdminEcdhCurves parses a comma-separated EcdhCurves preference list
// (e.g. "X25519MLKEM768,X25519,P-256") into the tls.CurveID slice consumed by
// tls.Config.CurvePreferences. Used both to fail config validation closed on
// an unrecognized curve name and to build the admin TLS listener's config.
func ParseAdminEcdhCurves(raw string) ([]tls.CurveID, error) {
	parts := strings.Split(raw, ",")
	curves := make([]tls.CurveID, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		curve, ok := adminEcdhCurvesByName[name]
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

// adminTLSVersionByName maps the version strings accepted in
// AdminTLSConfig.MinimumProtocolVersion/MaximumProtocolVersion to Go's
// crypto/tls version identifiers. Same vocabulary ("TLS1_2", etc.) as the
// router's downstream_tls/upstream_tls for consistency within this shared
// config file, even though the two are enforced by different TLS stacks
// (Envoy/BoringSSL for the router, Go's own crypto/tls here).
var adminTLSVersionByName = map[string]uint16{
	"TLS1_0": tls.VersionTLS10,
	"TLS1_1": tls.VersionTLS11,
	"TLS1_2": tls.VersionTLS12,
	"TLS1_3": tls.VersionTLS13,
}

// adminTLSVersionOrder ranks the version names above so a min > max
// combination can be rejected at validation time.
var adminTLSVersionOrder = map[string]int{
	"TLS1_0": 0,
	"TLS1_1": 1,
	"TLS1_2": 2,
	"TLS1_3": 3,
}

// ValidateAdminTLSVersions checks that min and max are both recognized
// version names and that min does not come after max.
func ValidateAdminTLSVersions(minVersion, maxVersion string) error {
	if _, ok := adminTLSVersionByName[minVersion]; !ok {
		return fmt.Errorf("minimum_protocol_version must be one of TLS1_0, TLS1_1, TLS1_2, TLS1_3, got: %q", minVersion)
	}
	if _, ok := adminTLSVersionByName[maxVersion]; !ok {
		return fmt.Errorf("maximum_protocol_version must be one of TLS1_0, TLS1_1, TLS1_2, TLS1_3, got: %q", maxVersion)
	}
	if adminTLSVersionOrder[minVersion] > adminTLSVersionOrder[maxVersion] {
		return fmt.Errorf("minimum_protocol_version (%s) cannot be greater than maximum_protocol_version (%s)", minVersion, maxVersion)
	}
	return nil
}

// ParseAdminTLSVersion converts a validated version name to its crypto/tls
// identifier. Callers should run ValidateAdminTLSVersions first; an
// unrecognized name here returns ok=false rather than panicking.
func ParseAdminTLSVersion(name string) (version uint16, ok bool) {
	version, ok = adminTLSVersionByName[name]
	return version, ok
}

// adminCipherSuiteByName is built from Go's own list of secure cipher suites
// (tls.CipherSuites — deliberately excludes tls.InsecureCipherSuites) so an
// operator can only ever restrict to suites Go itself considers safe, never
// re-enable a weak one. Includes the three TLS 1.3 suite names for
// completeness, though Go does not apply CipherSuites to TLS 1.3 — TLS 1.3
// suite selection is not configurable and always uses Go's own safe set.
var adminCipherSuiteByName = func() map[string]uint16 {
	m := make(map[string]uint16)
	for _, cs := range tls.CipherSuites() {
		m[cs.Name] = cs.ID
	}
	return m
}()

// ParseAdminCiphers parses a comma-separated list of Go crypto/tls cipher
// suite names (e.g. "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256") into the
// []uint16 consumed by tls.Config.CipherSuites. An empty string is valid and
// returns (nil, nil) — Go's own default suite set/order applies. Only
// affects TLS 1.2 (and below) connections; TLS 1.3 ignores this field.
func ParseAdminCiphers(raw string) ([]uint16, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	suites := make([]uint16, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		id, ok := adminCipherSuiteByName[name]
		if !ok {
			return nil, fmt.Errorf("unsupported or insecure cipher suite %q (see crypto/tls.CipherSuites for the supported list)", name)
		}
		suites = append(suites, id)
	}
	if len(suites) == 0 {
		return nil, fmt.Errorf("must specify at least one cipher suite, or omit ciphers entirely to use Go's default set")
	}
	return suites, nil
}
