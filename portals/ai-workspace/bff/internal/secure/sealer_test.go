/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package secure

import (
	"bytes"
	"errors"
	"testing"
)

func newTestSealer(t *testing.T, material, label string) *Sealer {
	t.Helper()
	s, err := NewSealer(DeriveKey(material, label))
	if err != nil {
		t.Fatalf("NewSealer: %v", err)
	}
	return s
}

// The property the whole multi-replica design rests on: two processes that never
// talked to each other, holding only the same configured secret, agree on the key.
func TestSeparateSealersFromSameMaterialInterchange(t *testing.T) {
	replicaA := newTestSealer(t, "shared-client-secret", "label/v1")
	replicaB := newTestSealer(t, "shared-client-secret", "label/v1")

	token, err := replicaA.Seal([]byte("session state"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	got, err := replicaB.Open(token)
	if err != nil {
		t.Fatalf("replica B could not open replica A's record: %v", err)
	}
	if !bytes.Equal(got, []byte("session state")) {
		t.Fatalf("round trip = %q, want %q", got, "session state")
	}
}

func TestSealIsNotDeterministic(t *testing.T) {
	s := newTestSealer(t, "m", "label/v1")
	a, _ := s.Seal([]byte("same plaintext"))
	b, _ := s.Seal([]byte("same plaintext"))
	if a == b {
		t.Fatal("two seals of the same plaintext are identical — the nonce is not fresh per call")
	}
}

func TestOpenRejectsTamperedAndForeignRecords(t *testing.T) {
	s := newTestSealer(t, "material-one", "label/v1")
	token, _ := s.Seal([]byte("session state"))

	// Flipping any single character must fail the GCM tag rather than decode to
	// something the caller would act on.
	tampered := []byte(token)
	tampered[len(tampered)-1] ^= 'A' ^ 'B'

	for name, bad := range map[string]string{
		"tampered":   string(tampered),
		"truncated":  token[:len(token)/2],
		"not base64": "!!!not-base64!!!",
		"empty":      "",
		"too short":  "AAAA",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := s.Open(bad); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Open(%s) error = %v, want ErrInvalid", name, err)
			}
		})
	}

	t.Run("other deployment's key", func(t *testing.T) {
		other := newTestSealer(t, "material-two", "label/v1")
		if _, err := other.Open(token); !errors.Is(err, ErrInvalid) {
			t.Fatalf("a record from another deployment opened: %v", err)
		}
	})

	// Label separation is what keeps the session-state key from opening a login
	// transaction, even though both derive from the same configured secret.
	t.Run("same material, other label", func(t *testing.T) {
		other := newTestSealer(t, "material-one", "label/v2")
		if _, err := other.Open(token); !errors.Is(err, ErrInvalid) {
			t.Fatalf("a differently-labelled key opened the record: %v", err)
		}
	})
}

func TestNewSealerRejectsWrongKeyLength(t *testing.T) {
	if _, err := NewSealer(make([]byte, KeySize-1)); err == nil {
		t.Fatal("NewSealer accepted a short key")
	}
}
