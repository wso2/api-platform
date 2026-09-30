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

package gatewayidentity

import (
	"crypto/elliptic"
	"encoding/pem"
	"errors"
	"testing"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/testutil/pki"
)

func requireFieldError(t *testing.T, err error, field, message string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %q on %s, got no error", message, field)
	}
	var fe *FieldError
	if !errors.As(err, &fe) {
		t.Fatalf("expected a *FieldError, got %T: %v", err, err)
	}
	if fe.Field != field || fe.Message != message {
		t.Fatalf("got %s: %q, want %s: %q", fe.Field, fe.Message, field, message)
	}
}

// ============ Private key blocks ============

func TestInspect_OpenSSLECKeyWithLeadingParametersBlock_Accepted(t *testing.T) {
	root := pki.NewRootCA(t, "EC Params Root CA")
	leaf := pki.NewLeaf(t, root, "ec-params-identity")

	bundle, err := Inspect(leaf.PEM(), leaf.ECKeyPEMWithParameters(), time.Now())
	if err != nil {
		t.Fatalf("expected an EC PARAMETERS block ahead of the key to be skipped, got: %v", err)
	}
	if bundle.KeyAlgorithm != "ECDSA" {
		t.Errorf("KeyAlgorithm = %q, want ECDSA", bundle.KeyAlgorithm)
	}
}

func TestInspectPrivateKey_NoKeyBlock_Refused(t *testing.T) {
	leaf := pki.NewLeaf(t, pki.NewRootCA(t, "No Key Root CA"), "no-key")
	onlyParameters := leaf.ECKeyPEMWithParameters()
	block, _ := pem.Decode(onlyParameters)

	_, _, err := InspectPrivateKey(pem.EncodeToMemory(block))
	requireFieldError(t, err, fieldPrivateKey, msgNotPEMKey)

	_, _, err = InspectPrivateKey(leaf.PEM())
	requireFieldError(t, err, fieldPrivateKey, msgNotPEMKey)
}

func TestInspectPrivateKey_TwoKeyBlocks_Refused(t *testing.T) {
	root := pki.NewRootCA(t, "Two Keys Root CA")
	first := pki.NewLeaf(t, root, "first")
	second := pki.NewLeaf(t, root, "second")

	_, _, err := InspectPrivateKey(append(first.KeyPEM(), second.KeyPEM()...))
	requireFieldError(t, err, fieldPrivateKey, msgSeveralKeys)
}

// ============ Key strength ============

func TestInspectPrivateKey_KeyStrength(t *testing.T) {
	for name, tc := range map[string]struct {
		key    func(t *testing.T) *pki.Entity
		refuse bool
	}{
		"RSA 1024": {key: func(t *testing.T) *pki.Entity {
			return pki.NewLeafWithKey(t, pki.NewRootCA(t, "RSA Root CA"), "rsa-1024", pki.NewRSAKey(t, 1024))
		}, refuse: true},
		"RSA 2048": {key: func(t *testing.T) *pki.Entity {
			return pki.NewLeafWithKey(t, pki.NewRootCA(t, "RSA Root CA"), "rsa-2048", pki.NewRSAKey(t, 2048))
		}},
		"ECDSA P-224": {key: func(t *testing.T) *pki.Entity {
			return pki.NewLeafWithKey(t, pki.NewRootCA(t, "EC Root CA"), "p224", pki.NewECKey(t, elliptic.P224()))
		}, refuse: true},
		"ECDSA P-384": {key: func(t *testing.T) *pki.Entity {
			return pki.NewLeafWithKey(t, pki.NewRootCA(t, "EC Root CA"), "p384", pki.NewECKey(t, elliptic.P384()))
		}},
		"ECDSA P-521": {key: func(t *testing.T) *pki.Entity {
			return pki.NewLeafWithKey(t, pki.NewRootCA(t, "EC Root CA"), "p521", pki.NewECKey(t, elliptic.P521()))
		}},
	} {
		t.Run(name, func(t *testing.T) {
			leaf := tc.key(t)
			_, err := Inspect(leaf.PEM(), leaf.KeyPEM(), time.Now())
			if tc.refuse {
				requireFieldError(t, err, fieldPrivateKey, msgWeakKey)
				return
			}
			if err != nil {
				t.Fatalf("expected the key to be accepted, got: %v", err)
			}
		})
	}
}

func TestInspectPrivateKey_WeakSEC1Key_Refused(t *testing.T) {
	leaf := pki.NewLeafWithKey(t, pki.NewRootCA(t, "EC Root CA"), "p224-sec1", pki.NewECKey(t, elliptic.P224()))

	_, _, err := InspectPrivateKey(leaf.ECKeyPEMWithParameters())
	requireFieldError(t, err, fieldPrivateKey, msgWeakKey)
}

// ============ Chain order and validity ============

func chainPEM(entities ...*pki.Entity) []byte {
	var out []byte
	for _, e := range entities {
		out = append(out, e.PEM()...)
	}
	return out
}

func TestInspectCertificateChain_OrderedChain_Accepted(t *testing.T) {
	root := pki.NewRootCA(t, "Ordered Root CA")
	intermediate := pki.NewIntermediate(t, root, "Ordered Intermediate CA")
	leaf := pki.NewLeaf(t, intermediate, "ordered-identity")

	chain, err := InspectCertificateChain(chainPEM(leaf, intermediate, root), time.Now())
	if err != nil {
		t.Fatalf("expected a leaf-first chain to be accepted, got: %v", err)
	}
	if len(chain) != 3 {
		t.Errorf("chain length = %d, want 3", len(chain))
	}
}

func TestInspectCertificateChain_OutOfOrder_Refused(t *testing.T) {
	root := pki.NewRootCA(t, "Unordered Root CA")
	intermediate := pki.NewIntermediate(t, root, "Unordered Intermediate CA")
	leaf := pki.NewLeaf(t, intermediate, "unordered-identity")
	unrelated := pki.NewRootCA(t, "Unrelated Root CA")

	for name, pemData := range map[string][]byte{
		"root before intermediate": chainPEM(leaf, root, intermediate),
		"unrelated issuer":         chainPEM(leaf, unrelated),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := InspectCertificateChain(pemData, time.Now())
			requireFieldError(t, err, fieldCertificate, msgChainOutOfOrder)
		})
	}
}

func TestInspectCertificateChain_ExpiredIntermediate_Refused(t *testing.T) {
	root := pki.NewRootCA(t, "Expired Intermediate Root CA")
	past := time.Now().Add(-48 * time.Hour)
	intermediate := pki.NewIntermediate(t, root, "Expired Intermediate CA", pki.WithValidity(past, time.Now().Add(-24*time.Hour)))
	leaf := pki.NewLeaf(t, intermediate, "under-expired-intermediate")

	_, err := InspectCertificateChain(chainPEM(leaf, intermediate, root), time.Now())
	requireFieldError(t, err, fieldCertificate,
		"an issuing certificate in the chain expired on "+intermediate.Cert.NotAfter.Format(time.RFC3339))
}
