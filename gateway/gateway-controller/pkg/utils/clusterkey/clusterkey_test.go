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

package clusterkey

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// hexShape64 matches exactly 64 lowercase hex characters - the cluster-key
// fragment shape produced by Hash.
var hexShape64 = regexp.MustCompile("^[a-f0-9]{64}$")

// TestHash validates the Hash helper: deterministic, distinct, and full SHA-256.
func TestHash(t *testing.T) {
	t.Run("deterministic for identical input", func(t *testing.T) {
		a := Hash("api-1")
		b := Hash("api-1")
		assert.Equal(t, a, b, "same input must produce same hash")
		assert.Regexp(t, hexShape64, a, "hash must be exactly 64 lowercase hex characters")
	})

	t.Run("different input produces different hash", func(t *testing.T) {
		a := Hash("api-1")
		b := Hash("api-2")
		assert.NotEqual(t, a, b)
	})

	// Known-answer vectors pin the algorithm to full SHA-256.
	t.Run("known-answer vectors", func(t *testing.T) {
		assert.Equal(t, "f9811b73ac5d1a8db842634fc0f871e03207ae44105fc9c2b7f1985e70f90d5a", Hash("api-1"))
		assert.Equal(t, "2a28373e2cacc6ea903d8c7e52dd3c49f8a87f95ec65ba1156de7e6564ca9524", Hash("test-api"))
		assert.Equal(t, "54a9b3e5ce2b6ccb97168e5948a66f48e084213b38eb8c7dc01c6f624a63c2f2", Hash("0190b3e2-7b1c-7c2a-9b3d-1a2b3c4d5e6f"))
	})

	t.Run("empty input is deterministic", func(t *testing.T) {
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", Hash(""))
	})
}

// TestHashedName validates the full hashed name contract.
func TestHashedName(t *testing.T) {
	t.Run("joins prefix to fragment", func(t *testing.T) {
		assert.Equal(t, "main_"+Hash("api-1"), HashedName("main", "api-1"))
		assert.Equal(t, "sandbox_"+Hash("api-1"), HashedName("sandbox", "api-1"))
	})

	t.Run("main and sandbox share the fragment, differ by prefix", func(t *testing.T) {
		main := HashedName("main", "api-1")
		sandbox := HashedName("sandbox", "api-1")
		assert.NotEqual(t, main, sandbox)
		assert.Equal(t, "main_f9811b73ac5d1a8db842634fc0f871e03207ae44105fc9c2b7f1985e70f90d5a", main)
		assert.Equal(t, "sandbox_f9811b73ac5d1a8db842634fc0f871e03207ae44105fc9c2b7f1985e70f90d5a", sandbox)
	})
}

// TestDefinitionName validates the upstream-definition cluster-name contract: the
// "upstream_" prefix, kind and API ID scoping, and dot/colon sanitization. The RDC
// transformer and the xDS translator both go through this helper, so per-op
// definition cluster names cannot drift.
func TestDefinitionName(t *testing.T) {
	t.Run("format and scoping", func(t *testing.T) {
		assert.Equal(t, "upstream_RestApi_api-1_my-upstream", DefinitionName("RestApi", "api-1", "my-upstream"))
	})

	t.Run("sanitizes dots and colons", func(t *testing.T) {
		tests := []struct {
			defName  string
			expected string
		}{
			{"my.upstream", "upstream_RestApi_api-1_my_upstream"},
			{"my:upstream", "upstream_RestApi_api-1_my_upstream"},
			{"host.example.com:8080", "upstream_RestApi_api-1_host_example_com_8080"},
			{"a.b.c:d", "upstream_RestApi_api-1_a_b_c_d"},
		}
		for _, tt := range tests {
			t.Run(tt.defName, func(t *testing.T) {
				assert.Equal(t, tt.expected, DefinitionName("RestApi", "api-1", tt.defName))
			})
		}
	})
}
