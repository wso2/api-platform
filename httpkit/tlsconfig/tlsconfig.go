// Package tlsconfig parses operator-facing cipher suite, ECDH/curve
// preference, and TLS version names into the crypto/tls identifiers needed
// to build a *tls.Config. The parsing here is direction-neutral: the same
// names and logic apply whether the resulting config is used for an inbound
// (server) or outbound (client) TLS connection.
package tlsconfig

import (
	"crypto/tls"
	"fmt"
	"strings"
)

// CurvesByName maps the curve/group names accepted in a CurvePreferences
// configuration string to Go's crypto/tls group identifiers.
//
// X25519MLKEM768 is the FIPS 203 ML-KEM-768 + X25519 hybrid group,
// implemented natively by Go 1.23+. It is never selected unless a caller
// names it explicitly — this package draws no distinction between "PQC" and
// "classical" curves beyond the name-to-ID mapping itself; whether hybrid
// curves are the default is a decision for the caller's own configuration,
// not this package.
var CurvesByName = map[string]tls.CurveID{
	"X25519":         tls.X25519,
	"P-256":          tls.CurveP256,
	"P-384":          tls.CurveP384,
	"P-521":          tls.CurveP521,
	"X25519MLKEM768": tls.X25519MLKEM768,
}

// ParseCurvePreferences parses a comma-separated curve/group preference list
// (e.g. "X25519MLKEM768,X25519,P-256") into the []tls.CurveID slice consumed
// by tls.Config.CurvePreferences, preserving order — order is the caller's
// preference ranking and is significant for interop (a hybrid group must be
// listed before, not instead of, a classical fallback for a peer that
// doesn't support it yet). An empty string returns (nil, nil): Go's own
// default preference list applies.
func ParseCurvePreferences(raw string) ([]tls.CurveID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	curves := make([]tls.CurveID, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		curve, ok := CurvesByName[name]
		if !ok {
			return nil, fmt.Errorf("unsupported curve/group %q (supported: X25519, P-256, P-384, P-521, X25519MLKEM768)", name)
		}
		curves = append(curves, curve)
	}
	if len(curves) == 0 {
		return nil, fmt.Errorf("must specify at least one curve, or omit entirely to use the default preference list")
	}
	return curves, nil
}

// versionByName maps the version strings accepted in a MinVersion/MaxVersion
// configuration to Go's crypto/tls version identifiers.
var versionByName = map[string]uint16{
	"TLS1_0": tls.VersionTLS10,
	"TLS1_1": tls.VersionTLS11,
	"TLS1_2": tls.VersionTLS12,
	"TLS1_3": tls.VersionTLS13,
}

// versionOrder ranks the version names above so a min > max combination can
// be rejected by ValidateVersionRange.
var versionOrder = map[string]int{
	"TLS1_0": 0,
	"TLS1_1": 1,
	"TLS1_2": 2,
	"TLS1_3": 3,
}

// ParseVersion converts a version name to its crypto/tls identifier. Callers
// that need to validate a min/max pair together should call
// ValidateVersionRange first; ok is false for an unrecognized name rather
// than panicking.
func ParseVersion(name string) (version uint16, ok bool) {
	version, ok = versionByName[name]
	return version, ok
}

// ValidateVersionRange checks that minVersion and maxVersion are both
// recognized version names and that minVersion does not come after
// maxVersion. Both empty is treated as "unset" (Go's own defaults apply);
// exactly one empty is rejected, since that combination cannot express a
// coherent bound.
func ValidateVersionRange(minVersion, maxVersion string) error {
	if minVersion == "" && maxVersion == "" {
		return nil
	}
	if minVersion == "" || maxVersion == "" {
		return fmt.Errorf("min_version and max_version must both be set, or both left empty to use Go's defaults")
	}
	if _, ok := versionByName[minVersion]; !ok {
		return fmt.Errorf("min_version must be one of TLS1_0, TLS1_1, TLS1_2, TLS1_3, got: %q", minVersion)
	}
	if _, ok := versionByName[maxVersion]; !ok {
		return fmt.Errorf("max_version must be one of TLS1_0, TLS1_1, TLS1_2, TLS1_3, got: %q", maxVersion)
	}
	if versionOrder[minVersion] > versionOrder[maxVersion] {
		return fmt.Errorf("min_version (%s) cannot be greater than max_version (%s)", minVersion, maxVersion)
	}
	return nil
}

// cipherSuiteByName is built from Go's own list of secure cipher suites
// (tls.CipherSuites — deliberately excludes tls.InsecureCipherSuites) so a
// caller can only ever restrict to suites Go itself considers safe, never
// re-enable a weak one.
var cipherSuiteByName = func() map[string]uint16 {
	m := make(map[string]uint16)
	for _, cs := range tls.CipherSuites() {
		m[cs.Name] = cs.ID
	}
	return m
}()

// ParseCipherSuites parses a comma-separated list of Go crypto/tls cipher
// suite names (e.g. "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256") into the
// []uint16 consumed by tls.Config.CipherSuites. An empty string is valid and
// returns (nil, nil) — Go's own default suite set/order applies. This only
// affects TLS 1.2 and below; TLS 1.3 suite selection is not configurable in
// Go and always uses its own safe set.
func ParseCipherSuites(raw string) ([]uint16, error) {
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
		id, ok := cipherSuiteByName[name]
		if !ok {
			return nil, fmt.Errorf("unsupported or insecure cipher suite %q (see crypto/tls.CipherSuites for the supported list)", name)
		}
		suites = append(suites, id)
	}
	if len(suites) == 0 {
		return nil, fmt.Errorf("must specify at least one cipher suite, or omit entirely to use Go's default set")
	}
	return suites, nil
}

// NegotiatedCurveName returns the human-readable name of the curve/group
// negotiated on a completed TLS connection (e.g. "X25519MLKEM768", "X25519"),
// or the numeric fallback Go's own CurveID.String() produces for a group
// this package doesn't otherwise name. Intended for logging/status
// reporting so operators can confirm whether a given connection actually
// negotiated a post-quantum hybrid group or fell back to a classical one, as
// required whenever hybrid PQC support is enabled.
func NegotiatedCurveName(cs tls.ConnectionState) string {
	return cs.CurveID.String()
}
