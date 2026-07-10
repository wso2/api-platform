/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package webhook

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// hybridEncrypt mirrors the producer side: AES-256-GCM body + RSA-OAEP(SHA-256)-wrapped content key.
func hybridEncrypt(t *testing.T, pub *rsa.PublicKey, plaintext string) *EncryptedKey {
	t.Helper()
	contentKey := make([]byte, 32)
	if _, err := rand.Read(contentKey); err != nil {
		t.Fatalf("content key: %v", err)
	}
	block, err := aes.NewCipher(contentKey)
	if err != nil {
		t.Fatalf("aes: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("gcm: %v", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		t.Fatalf("nonce: %v", err)
	}
	sealed := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	ciphertext := sealed[:len(sealed)-16]
	tag := sealed[len(sealed)-16:]

	wrapped, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, contentKey, nil)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	return &EncryptedKey{
		WrappedKey: base64.StdEncoding.EncodeToString(wrapped),
		IV:         base64.StdEncoding.EncodeToString(nonce),
		Tag:        base64.StdEncoding.EncodeToString(tag),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}
}

func writeKeyPEM(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	der := x509.MarshalPKCS1PrivateKey(key)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
	path := filepath.Join(t.TempDir(), "wh.pem")
	if err := os.WriteFile(path, pemBytes, 0600); err != nil {
		t.Fatalf("write pem: %v", err)
	}
	return path
}

func TestDecryptor_RoundTrip(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	path := writeKeyPEM(t, key)

	d, err := NewDecryptor(path)
	if err != nil {
		t.Fatalf("NewDecryptor: %v", err)
	}

	const secret = "ap-key-9f8b7c6d5e4f3a2b1c0d"
	enc := hybridEncrypt(t, &key.PublicKey, secret)

	got, err := d.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != secret {
		t.Fatalf("plaintext mismatch: got %q want %q", got, secret)
	}
}

func TestDecryptor_TamperedTagFails(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	d, err := NewDecryptor(writeKeyPEM(t, key))
	if err != nil {
		t.Fatalf("NewDecryptor: %v", err)
	}
	enc := hybridEncrypt(t, &key.PublicKey, "secret-value")
	enc.Tag = base64.StdEncoding.EncodeToString(make([]byte, 16)) // zeroed tag
	if _, err := d.Decrypt(enc); err == nil {
		t.Fatal("expected decryption to fail with a tampered tag")
	}
}

func TestDecryptor_NilWhenNoKeyPath(t *testing.T) {
	d, err := NewDecryptor("")
	if err != nil {
		t.Fatalf("NewDecryptor(\"\"): %v", err)
	}
	if _, err := d.Decrypt(&EncryptedKey{WrappedKey: "x", Ciphertext: "y"}); err != ErrDecryptorUnavailable {
		t.Fatalf("expected ErrDecryptorUnavailable, got %v", err)
	}
}

// sign reproduces the producer's HMAC scheme for tests.
func sign(secret string, ts int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10) + "." + string(body)))
	return "t=" + strconv.FormatInt(ts, 10) + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

func TestLooksLikeUniqueViolation(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errorString("UNIQUE constraint failed: api_keys.artifact_uuid, api_keys.name"), true},
		{errorString("ERROR: duplicate key value violates unique constraint \"api_keys_pkey\""), true},
		{errorString("Violation of UNIQUE KEY constraint 'UQ_api_keys'."), true},
		{errorString("connection refused"), false},
	}
	for _, c := range cases {
		if got := looksLikeUniqueViolation(c.err); got != c.want {
			t.Errorf("looksLikeUniqueViolation(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

type errorString string

func (e errorString) Error() string { return string(e) }

func TestVerifier_Verify(t *testing.T) {
	const secret = "shared-secret"
	v := NewVerifier(secret, 5*time.Minute)
	now := time.Unix(1_700_000_000, 0)
	body := []byte(`{"event_id":"e1"}`)

	t.Run("valid", func(t *testing.T) {
		if err := v.Verify(sign(secret, now.Unix(), body), body, now); err != nil {
			t.Fatalf("expected valid signature, got %v", err)
		}
	})
	t.Run("missing header", func(t *testing.T) {
		if err := v.Verify("", body, now); err != ErrSignatureMissing {
			t.Fatalf("want ErrSignatureMissing, got %v", err)
		}
	})
	t.Run("tampered body", func(t *testing.T) {
		if err := v.Verify(sign(secret, now.Unix(), body), []byte(`{"event_id":"e2"}`), now); err != ErrSignatureInvalid {
			t.Fatalf("want ErrSignatureInvalid, got %v", err)
		}
	})
	t.Run("stale timestamp", func(t *testing.T) {
		old := now.Add(-10 * time.Minute)
		if err := v.Verify(sign(secret, old.Unix(), body), body, now); err != ErrTimestampOutOfTolerance {
			t.Fatalf("want ErrTimestampOutOfTolerance, got %v", err)
		}
	})
}
