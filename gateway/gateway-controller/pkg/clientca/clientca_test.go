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

package clientca_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/clientca"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/testutil/pki"
)

// joinPEM concatenates the PEM certificates of several entities, in the
// order given, to build a multi-certificate upload body.
func joinPEM(entities ...*pki.Entity) []byte {
	var buf []byte
	for _, e := range entities {
		buf = append(buf, e.PEM()...)
	}
	return buf
}

// base64BodyOf returns the base64 body of a PEM block, to check an error
// message does not leak it.
func base64BodyOf(pemBytes []byte) string {
	var body []string
	for _, line := range strings.Split(string(pemBytes), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "-----") {
			continue
		}
		body = append(body, line)
	}
	return strings.Join(body, "")
}

func assertFieldError(t *testing.T, err error, field, message string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error, got nil")
	}
	var fe *clientca.FieldError
	if !errors.As(err, &fe) {
		t.Fatalf("expected *clientca.FieldError, got %T: %v", err, err)
	}
	if fe.Field != field {
		t.Fatalf("expected field %q, got %q", field, fe.Field)
	}
	if fe.Message != message {
		t.Fatalf("expected message %q, got %q", message, fe.Message)
	}
}

func TestInspect_IssuingCAWithRoot_EitherOrder(t *testing.T) {
	root := pki.NewRootCA(t, "Order Root CA")
	issuing := pki.NewIntermediate(t, root, "Order Issuing CA")

	cases := []struct {
		name string
		body []byte
	}{
		{"root then issuing", joinPEM(root, issuing)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bundle, err := clientca.Inspect(tc.body, time.Now())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if bundle.Identity == nil || bundle.Identity.Subject.String() != issuing.Cert.Subject.String() {
				t.Fatalf("expected identity to be the issuing CA, got %v", bundle.Identity)
			}
			if len(bundle.Certificates) != 2 {
				t.Fatalf("expected 2 certificates, got %d", len(bundle.Certificates))
			}
			if bundle.IsLeaf {
				t.Fatalf("expected IsLeaf false for a CA identity")
			}
		})
	}
}

func TestInspect_LeafSignedByRoot(t *testing.T) {
	root := pki.NewRootCA(t, "Leaf Parent Root CA")
	leaf := pki.NewLeaf(t, root, "leaf-under-root")

	bundle, err := clientca.Inspect(joinPEM(leaf, root), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bundle.Identity == nil || bundle.Identity.Subject.String() != leaf.Cert.Subject.String() {
		t.Fatalf("expected identity to be the leaf, got %v", bundle.Identity)
	}
	if !bundle.IsLeaf {
		t.Fatalf("expected IsLeaf true for a leaf identity")
	}
	if len(bundle.Certificates) != 2 {
		t.Fatalf("expected 2 certificates, got %d", len(bundle.Certificates))
	}
}

func TestInspect_PrivateKeyPresent(t *testing.T) {
	root := pki.NewRootCA(t, "Key Leak Root CA")
	const wantMessage = "the upload contains a private key; a client-CA entry accepts certificates only"

	cases := []struct {
		name   string
		keyPEM []byte
	}{
		{
			name:   "EC PRIVATE KEY",
			keyPEM: []byte("-----BEGIN EC PRIVATE KEY-----\nMIIBaAIBAQ==\n-----END EC PRIVATE KEY-----\n"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := append(append([]byte{}, root.PEM()...), tc.keyPEM...)

			_, err := clientca.Inspect(body, time.Now())
			assertFieldError(t, err, "certificate", wantMessage)

			if marker := base64BodyOf(tc.keyPEM); marker != "" && strings.Contains(err.Error(), marker) {
				t.Fatalf("error message leaked private key bytes: %s", err.Error())
			}
		})
	}
}

func TestInspect_GarbageDERInCertificateBlock(t *testing.T) {
	// "bm90IHZhbGlkIGRlciBkYXRh" base64-decodes to "not valid der data" — a
	// well-formed PEM block whose payload is not a parseable certificate.
	garbage := "-----BEGIN CERTIFICATE-----\nbm90IHZhbGlkIGRlciBkYXRh\n-----END CERTIFICATE-----\n"

	_, err := clientca.Inspect([]byte(garbage), time.Now())
	assertFieldError(t, err, "certificate", "the value is not a PEM-encoded certificate")
}

func TestInspect_ExpiredIdentity(t *testing.T) {
	now := time.Now()
	expired := pki.NewRootCA(t, "Expired Root CA", pki.WithValidity(now.Add(-48*time.Hour), now.Add(-24*time.Hour)))

	_, err := clientca.Inspect(expired.PEM(), now)
	if err == nil {
		t.Fatalf("expected an error for an expired certificate")
	}
	var fe *clientca.FieldError
	if !errors.As(err, &fe) {
		t.Fatalf("expected *clientca.FieldError, got %T: %v", err, err)
	}

	const prefix = "the certificate expired on "
	if !strings.HasPrefix(fe.Message, prefix) {
		t.Fatalf("expected message to start with %q, got %q", prefix, fe.Message)
	}

	suffix := strings.TrimPrefix(fe.Message, prefix)
	parsed, parseErr := time.Parse(time.RFC3339, suffix)
	if parseErr != nil {
		t.Fatalf("expected notAfter suffix %q to be RFC3339: %v", suffix, parseErr)
	}
	if !parsed.Equal(expired.Cert.NotAfter) {
		t.Fatalf("expected notAfter %v, got %v", expired.Cert.NotAfter, parsed)
	}
}

func TestExpiryWarning(t *testing.T) {
	now := time.Now()

	w := clientca.ExpiryWarning(now.Add(29*24*time.Hour), now)
	if w == nil {
		t.Fatalf("expected a warning for a certificate expiring in 29 days")
	}
	if w.Code != "CERT_EXPIRES_SOON" {
		t.Fatalf("expected warning code CERT_EXPIRES_SOON, got %q", w.Code)
	}
	if w.Field != "notAfter" {
		t.Fatalf("expected warning field %q, got %q", "notAfter", w.Field)
	}

	if w := clientca.ExpiryWarning(now.Add(31*24*time.Hour), now); w != nil {
		t.Fatalf("expected no warning for a certificate expiring in 31 days, got %+v", w)
	}

	if w := clientca.ExpiryWarning(now.Add(-1*time.Hour), now); w == nil {
		t.Fatalf("expected a warning for an already-expired certificate")
	}
}
