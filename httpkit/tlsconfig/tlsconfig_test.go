package tlsconfig

import (
	"crypto/tls"
	"testing"
)

func TestParseCurvePreferences(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []tls.CurveID
		wantErr bool
	}{
		{name: "empty uses default", raw: "", want: nil},
		{name: "single curve", raw: "X25519", want: []tls.CurveID{tls.X25519}},
		{
			name: "hybrid first then classical fallback, order preserved",
			raw:  "X25519MLKEM768,X25519,P-256",
			want: []tls.CurveID{tls.X25519MLKEM768, tls.X25519, tls.CurveP256},
		},
		{name: "trims whitespace", raw: " X25519 , P-256 ", want: []tls.CurveID{tls.X25519, tls.CurveP256}},
		{name: "unknown curve rejected", raw: "Curve25519", wantErr: true},
		{name: "only commas is empty after trim", raw: ",,", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCurvePreferences(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseCurvePreferences(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ParseCurvePreferences(%q) = %v, want %v", tt.raw, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("ParseCurvePreferences(%q)[%d] = %v, want %v", tt.raw, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseCipherSuites(t *testing.T) {
	if suites, err := ParseCipherSuites(""); err != nil || suites != nil {
		t.Fatalf("ParseCipherSuites(\"\") = %v, %v, want nil, nil", suites, err)
	}

	// Pick a real secure suite name from Go's own list rather than hardcoding
	// one, so this test doesn't rot if Go's secure-suite set ever changes.
	secureSuites := tls.CipherSuites()
	if len(secureSuites) == 0 {
		t.Fatal("tls.CipherSuites() returned no suites")
	}
	name := secureSuites[0].Name
	got, err := ParseCipherSuites(name)
	if err != nil {
		t.Fatalf("ParseCipherSuites(%q) unexpected error: %v", name, err)
	}
	if len(got) != 1 || got[0] != secureSuites[0].ID {
		t.Fatalf("ParseCipherSuites(%q) = %v, want [%v]", name, got, secureSuites[0].ID)
	}

	if _, err := ParseCipherSuites("TLS_NOT_A_REAL_SUITE"); err == nil {
		t.Fatal("ParseCipherSuites accepted an unknown suite name")
	}

	// Every suite returned by tls.InsecureCipherSuites must be rejected —
	// this package must never let a caller re-enable a weak suite.
	for _, cs := range tls.InsecureCipherSuites() {
		if _, err := ParseCipherSuites(cs.Name); err == nil {
			t.Fatalf("ParseCipherSuites accepted insecure suite %q", cs.Name)
		}
	}
}

func TestValidateVersionRange(t *testing.T) {
	tests := []struct {
		name     string
		min, max string
		wantErr  bool
	}{
		{name: "both empty is valid (use Go defaults)", min: "", max: ""},
		{name: "valid range", min: "TLS1_2", max: "TLS1_3"},
		{name: "equal min and max is valid", min: "TLS1_3", max: "TLS1_3"},
		{name: "min only is invalid", min: "TLS1_2", max: "", wantErr: true},
		{name: "max only is invalid", min: "", max: "TLS1_3", wantErr: true},
		{name: "unknown min rejected", min: "TLS9_9", max: "TLS1_3", wantErr: true},
		{name: "unknown max rejected", min: "TLS1_2", max: "TLS9_9", wantErr: true},
		{name: "min greater than max rejected", min: "TLS1_3", max: "TLS1_2", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVersionRange(tt.min, tt.max)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateVersionRange(%q, %q) error = %v, wantErr %v", tt.min, tt.max, err, tt.wantErr)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	v, ok := ParseVersion("TLS1_3")
	if !ok || v != tls.VersionTLS13 {
		t.Fatalf("ParseVersion(TLS1_3) = %v, %v, want %v, true", v, ok, tls.VersionTLS13)
	}
	if _, ok := ParseVersion("bogus"); ok {
		t.Fatal("ParseVersion accepted an unknown version name")
	}
}

func TestNegotiatedCurveName(t *testing.T) {
	cs := tls.ConnectionState{CurveID: tls.X25519MLKEM768}
	if got := NegotiatedCurveName(cs); got != "X25519MLKEM768" {
		t.Fatalf("NegotiatedCurveName = %q, want %q", got, "X25519MLKEM768")
	}

	cs = tls.ConnectionState{CurveID: tls.X25519}
	if got := NegotiatedCurveName(cs); got != "X25519" {
		t.Fatalf("NegotiatedCurveName = %q, want %q", got, "X25519")
	}
}
