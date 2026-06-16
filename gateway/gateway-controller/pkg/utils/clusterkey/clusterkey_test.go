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

// hexShape24 matches exactly 24 lowercase hex characters - the cluster-key
// fragment shape produced by APILevel.
var hexShape24 = regexp.MustCompile("^[a-f0-9]{24}$")

// TestAPILevel validates the API-level cluster-key fragment: deterministic,
// distinct per apiID, and pinned to SHA-256[:12] (24 hex chars).
func TestAPILevel(t *testing.T) {
	t.Run("deterministic for identical input", func(t *testing.T) {
		a := APILevel("api-1")
		b := APILevel("api-1")
		assert.Equal(t, a, b, "same input must produce same hash")
		assert.Regexp(t, hexShape24, a, "hash must be exactly 24 lowercase hex characters")
	})

	t.Run("different apiID produces different hash", func(t *testing.T) {
		a := APILevel("api-1")
		b := APILevel("api-2")
		assert.NotEqual(t, a, b)
	})

	// Known-answer vectors pin the algorithm to SHA-256[:12]. Without these, any
	// deterministic 24-hex function would satisfy the shape checks above.
	t.Run("known-answer vectors", func(t *testing.T) {
		assert.Equal(t, "f9811b73ac5d1a8db842634f", APILevel("api-1"))
		assert.Equal(t, "2a28373e2cacc6ea903d8c7e", APILevel("test-api"))
		// A realistic UUIDv7-shaped apiID, the form used in production.
		assert.Equal(t, "54a9b3e5ce2b6ccb97168e59", APILevel("0190b3e2-7b1c-7c2a-9b3d-1a2b3c4d5e6f"))
	})

	// Empty input is deterministic (the SHA-256 of the empty string), documenting
	// that APILevel itself does not reject empty apiIDs; non-emptiness is enforced
	// upstream at deploy time.
	t.Run("empty input is deterministic", func(t *testing.T) {
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8", APILevel(""))
	})
}

// TestAPILevelName validates the full cluster-name contract: the env prefix
// joined to the APILevel fragment. Both xDS builders go through this helper, so
// the two paths cannot drift.
func TestAPILevelName(t *testing.T) {
	t.Run("joins env prefix to fragment", func(t *testing.T) {
		assert.Equal(t, "main_"+APILevel("api-1"), APILevelName("main", "api-1"))
		assert.Equal(t, "sandbox_"+APILevel("api-1"), APILevelName("sandbox", "api-1"))
	})

	t.Run("main and sandbox share the fragment, differ by prefix", func(t *testing.T) {
		main := APILevelName("main", "api-1")
		sandbox := APILevelName("sandbox", "api-1")
		assert.NotEqual(t, main, sandbox)
		assert.Equal(t, "main_f9811b73ac5d1a8db842634f", main)
		assert.Equal(t, "sandbox_f9811b73ac5d1a8db842634f", sandbox)
	})
}
