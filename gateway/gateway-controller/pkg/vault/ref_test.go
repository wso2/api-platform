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

package vault

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRef_LocalHandles(t *testing.T) {
	// Every handle the management API accepts today must stay a local handle.
	for _, h := range []string{
		"openai-key",
		"amp-5f1c2e-3",
		"my.secret_v2",
		"kv:not-a-ref", // no "//", so still a plain handle
		"weird#name?x=1",
	} {
		ref, err := ParseRef(h)
		require.NoError(t, err, h)
		assert.False(t, ref.IsExternal(), h)
		assert.Equal(t, h, ref.Handle)
		assert.Equal(t, h, ref.String())
	}
}

func TestParseRef_EmptyHandle(t *testing.T) {
	_, err := ParseRef("  ")
	assert.Error(t, err)
}

func TestParseRef_External(t *testing.T) {
	tests := []struct {
		in   string
		want Ref
	}{
		{
			in:   "kv://amp-shared/gateways/gw1/openai#api_key",
			want: Ref{Resolver: "amp-shared", Path: "gateways/gw1/openai", Field: "api_key"},
		},
		{
			in:   "kv://customer/llm/openai",
			want: Ref{Resolver: "customer", Path: "llm/openai", Field: DefaultField},
		},
		{
			in:   "kv://amp-shared/creds/c1?version=3#token",
			want: Ref{Resolver: "amp-shared", Path: "creds/c1", Field: "token", Version: 3},
		},
		{
			in:   "kv://r1/a?version=12",
			want: Ref{Resolver: "r1", Path: "a", Field: DefaultField, Version: 12},
		},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			ref, err := ParseRef(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, ref)
			assert.True(t, ref.IsExternal())
			assert.True(t, IsExternalRef(tt.in))

			// The canonical form parses back to the same reference.
			again, err := ParseRef(ref.String())
			require.NoError(t, err)
			assert.Equal(t, ref, again)
		})
	}
}

func TestParseRef_ExternalInvalid(t *testing.T) {
	for _, in := range []string{
		"kv://",
		"kv://resolver",
		"kv://resolver/",
		"kv:///path",
		"kv://Upper/path",
		"kv://bad_name/path",
		"kv://r/a//b",
		"kv://r/../sys/seal",
		"kv://r/a/./b",
		"kv://r/a%2f..",
		"kv://r/a\\b",
		"kv://r/a#",
		"kv://r/a#f/x",
		"kv://r/a?version=0",
		"kv://r/a?version=-1",
		"kv://r/a?version=abc",
		"kv://r/a?ver=1",
		"kv://r/a?version=1&x=2",
		"kv://r/a\nb",
	} {
		t.Run(in, func(t *testing.T) {
			_, err := ParseRef(in)
			assert.Error(t, err)
		})
	}
}

func TestValidResolverName(t *testing.T) {
	assert.True(t, ValidResolverName("amp-shared"))
	assert.True(t, ValidResolverName("a"))
	assert.False(t, ValidResolverName(""))
	assert.False(t, ValidResolverName("-a"))
	assert.False(t, ValidResolverName("a-"))
	assert.False(t, ValidResolverName("A"))
	assert.False(t, ValidResolverName("a.b"))
}
