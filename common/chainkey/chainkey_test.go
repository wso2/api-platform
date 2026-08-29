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

package chainkey

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForIsStableAndSeparated(t *testing.T) {
	key := For("api-1", "api.example.com", "message/send")

	assert.Equal(t, "api-1\x1fapi.example.com\x1fmessage/send", key)
	assert.Equal(t, 3, len(strings.Split(key, Separator)),
		"a key must split back into exactly its three components")
	assert.Equal(t, key, For("api-1", "api.example.com", "message/send"),
		"composition must be deterministic — the whole contract rests on it")
}

// The convergence property this package exists for: two transports of one logical
// operation compose one key, because the key is a function of the operation and not
// of the route that carried it.
func TestBothTransportsComposeTheSameKey(t *testing.T) {
	fromJSONRPC := For("api-1", "api.example.com", "message/send")
	fromHTTPJSON := For("api-1", "api.example.com", "message/send")

	assert.Equal(t, fromJSONRPC, fromHTTPJSON)
}

func TestDistinctInputsDoNotCollide(t *testing.T) {
	keys := map[string]string{
		"base":           For("api-1", "api.example.com", "message/send"),
		"other-op":       For("api-1", "api.example.com", "message/stream"),
		"other-vhost":    For("api-1", "sandbox.example.com", "message/send"),
		"other-api":      For("api-2", "api.example.com", "message/send"),
		"shifted-fields": For("api-1", "api.example.com/message", "send"),
	}

	seen := make(map[string]string, len(keys))
	for name, key := range keys {
		if prior, dup := seen[key]; dup {
			t.Fatalf("%s collides with %s", name, prior)
		}
		seen[key] = name
	}
}

// A printable separator would make "shifted-fields" above collide with "base". This
// pins the reason 0x1f was chosen, so a future change to a "/" or ":" separator has
// to fail a test rather than silently merge two operations' policies.
func TestSeparatorIsNotHTTPSafe(t *testing.T) {
	assert.Equal(t, "\x1f", Separator)
	assert.False(t, ValidComponent("tools/call"+Separator+"forged"))
}

func TestValidComponent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"plain", "message/send", true},
		{"with dots and dashes", "a2a.v1-send", true},
		{"empty", "", false},
		{"embedded separator", "tools/call\x1fother", false},
		{"leading separator", "\x1ftools/call", false},
		{"only separator", "\x1f", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.valid, ValidComponent(tc.input))
		})
	}
}

func TestSplitRoundTrips(t *testing.T) {
	apiID, vhost, operation, ok := Split(For("api-1", "api.example.com", "SendMessage"))
	require.True(t, ok)
	assert.Equal(t, "api-1", apiID)
	assert.Equal(t, "api.example.com", vhost)
	assert.Equal(t, "SendMessage", operation)

	// The default vhost is empty, and that is a valid partition rather than malformed.
	apiID, vhost, operation, ok = Split(For("api-1", "", "GetTask"))
	require.True(t, ok)
	assert.Equal(t, "api-1", apiID)
	assert.Empty(t, vhost)
	assert.Equal(t, "GetTask", operation)
}

func TestSplitRejectsNonKeys(t *testing.T) {
	for name, key := range map[string]string{
		"route key":       "GET|/pets|api.example.com",
		"two components":  "api-1" + Separator + "SendMessage",
		"four components": "api-1" + Separator + "h" + Separator + "a" + Separator + "b",
		"empty api id":    Separator + "h" + Separator + "SendMessage",
		"empty operation": "api-1" + Separator + "h" + Separator,
		"empty string":    "",
	} {
		t.Run(name, func(t *testing.T) {
			_, _, _, ok := Split(key)
			assert.False(t, ok)
		})
	}
}

// IsComposed has to answer "claims to be one of ours" rather than "is a valid one",
// so a caller can report a malformed key instead of silently treating it as a route key.
func TestIsComposed(t *testing.T) {
	assert.True(t, IsComposed(For("api-1", "h", "SendMessage")))
	assert.True(t, IsComposed("api-1"+Separator+"malformed"))
	assert.False(t, IsComposed("GET|/pets|h"))
}
