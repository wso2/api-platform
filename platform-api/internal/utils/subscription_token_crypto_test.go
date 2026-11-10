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

package utils

import (
	"bytes"
	"encoding/base64"
	"testing"
)

const subscriptionTestKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func subscriptionTestKey(t *testing.T) []byte {
	t.Helper()
	key, err := DeriveEncryptionKey(subscriptionTestKeyHex)
	if err != nil {
		t.Fatalf("DeriveEncryptionKey: %v", err)
	}
	return key
}

func TestSubscriptionToken_RoundTripsAndUsesAFreshNonce(t *testing.T) {
	key := subscriptionTestKey(t)
	first, err := EncryptSubscriptionToken(key, "sub-token")
	if err != nil {
		t.Fatalf("EncryptSubscriptionToken: %v", err)
	}
	second, err := EncryptSubscriptionToken(key, "sub-token")
	if err != nil {
		t.Fatalf("EncryptSubscriptionToken: %v", err)
	}
	if first == second {
		t.Error("two encryptions of the same token produced the same ciphertext")
	}
	for _, ciphertext := range []string{first, second} {
		got, err := DecryptSubscriptionToken(key, ciphertext)
		if err != nil || got != "sub-token" {
			t.Fatalf("DecryptSubscriptionToken = (%q, %v), want sub-token", got, err)
		}
	}
}

func TestSubscriptionToken_RejectsInvalidInput(t *testing.T) {
	key := subscriptionTestKey(t)
	if _, err := EncryptSubscriptionToken(key[:16], "sub-token"); err == nil {
		t.Error("encrypting with a 16-byte key succeeded")
	}
	if _, err := EncryptSubscriptionToken(key, ""); err == nil {
		t.Error("encrypting an empty token succeeded")
	}

	valid, err := EncryptSubscriptionToken(key, "sub-token")
	if err != nil {
		t.Fatalf("EncryptSubscriptionToken: %v", err)
	}
	otherKey := bytes.Repeat([]byte{7}, 32)
	for name, tc := range map[string]struct {
		key        []byte
		ciphertext string
	}{
		"short key":            {key[:16], valid},
		"empty ciphertext":     {key, ""},
		"not base64":           {key, "%%%"},
		"shorter than a nonce": {key, base64.StdEncoding.EncodeToString([]byte("short"))},
		"another key":          {otherKey, valid},
	} {
		if got, err := DecryptSubscriptionToken(tc.key, tc.ciphertext); err == nil {
			t.Errorf("%s: DecryptSubscriptionToken = %q, want an error", name, got)
		}
	}
}

func TestDeriveEncryptionKey_AcceptsOnlyA32ByteKey(t *testing.T) {
	fromHex := subscriptionTestKey(t)
	fromBase64, err := DeriveEncryptionKey(base64.StdEncoding.EncodeToString(fromHex))
	if err != nil || !bytes.Equal(fromHex, fromBase64) {
		t.Fatalf("base64 form = (%x, %v), want %x", fromBase64, err, fromHex)
	}
	for _, invalid := range []string{
		"",
		subscriptionTestKeyHex[:62],
		base64.StdEncoding.EncodeToString(make([]byte, 16)),
		"not a key",
	} {
		if _, err := DeriveEncryptionKey(invalid); err == nil {
			t.Errorf("DeriveEncryptionKey(%q) succeeded", invalid)
		}
	}
}
