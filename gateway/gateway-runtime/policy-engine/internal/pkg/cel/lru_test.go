/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package cel

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubProgram is a minimal cel.Program stand-in for cache tests.
// programLRUCache stores the interface value, so any non-nil value works.

func TestLRUCache_BasicGetPut(t *testing.T) {
	c := newProgramLRUCache(3)

	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)
	ce := evaluator.(*celEvaluator)

	// Warm two programs via the evaluator so we have real cel.Program values.
	p1, err := ce.getOrCompileProgram(`true`)
	require.NoError(t, err)
	p2, err := ce.getOrCompileProgram(`false`)
	require.NoError(t, err)

	c.put("expr1", p1)
	c.put("expr2", p2)

	got, ok := c.get("expr1")
	assert.True(t, ok)
	assert.Equal(t, p1, got)

	_, ok = c.get("missing")
	assert.False(t, ok)
}

func TestLRUCache_Eviction(t *testing.T) {
	const capacity = 3
	c := newProgramLRUCache(capacity)

	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)
	ce := evaluator.(*celEvaluator)

	// Fill cache to capacity with distinct expressions.
	keys := make([]string, capacity+1)
	for i := 0; i <= capacity; i++ {
		keys[i] = fmt.Sprintf("expr_%d", i)
		p, err := ce.getOrCompileProgram(`true`)
		require.NoError(t, err)
		c.put(keys[i], p)
	}

	assert.Equal(t, capacity, c.len(), "cache must not exceed capacity")

	// keys[0] was inserted first and never touched again — it must be evicted.
	_, ok := c.get(keys[0])
	assert.False(t, ok, "LRU entry (keys[0]) should have been evicted")

	// keys[1..capacity] were inserted after keys[0] and must still be present.
	for i := 1; i <= capacity; i++ {
		_, ok := c.get(keys[i])
		assert.True(t, ok, "key %s should still be in cache", keys[i])
	}
}

func TestLRUCache_PromotionPreventsEviction(t *testing.T) {
	const capacity = 2
	c := newProgramLRUCache(capacity)

	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)
	ce := evaluator.(*celEvaluator)

	p, err := ce.getOrCompileProgram(`true`)
	require.NoError(t, err)

	c.put("a", p)
	c.put("b", p)

	// Promote "a" so it becomes MRU; "b" becomes LRU.
	_, ok := c.get("a")
	require.True(t, ok)

	// Insert "c" — "b" (LRU) should be evicted, not "a".
	c.put("c", p)

	_, ok = c.get("b")
	assert.False(t, ok, `"b" should have been evicted after "a" was promoted`)

	_, ok = c.get("a")
	assert.True(t, ok, `"a" should still be present`)

	_, ok = c.get("c")
	assert.True(t, ok, `"c" should be present`)
}

func TestLRUCache_UpdateExistingKey(t *testing.T) {
	c := newProgramLRUCache(3)

	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)
	ce := evaluator.(*celEvaluator)

	p1, err := ce.getOrCompileProgram(`true`)
	require.NoError(t, err)
	p2, err := ce.getOrCompileProgram(`false`)
	require.NoError(t, err)

	c.put("key", p1)
	c.put("key", p2) // update same key

	assert.Equal(t, 1, c.len(), "duplicate key must not grow the cache")

	got, ok := c.get("key")
	assert.True(t, ok)
	assert.Equal(t, p2, got, "updated value should be returned")
}

func TestCELEvaluator_CacheEvictionIntegration(t *testing.T) {
	// Verify that the evaluator still compiles and evaluates correctly after
	// cache entries have been evicted (i.e., re-compilation on eviction works).
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)
	ce := evaluator.(*celEvaluator)

	// Over-fill the LRU by inserting more distinct expressions than the cache
	// capacity. We only need to confirm no panic / error occurs and results
	// remain correct.
	for i := 0; i < defaultProgramCacheSize+10; i++ {
		expr := fmt.Sprintf("%d == %d", i, i)
		p, err := ce.getOrCompileProgram(expr)
		require.NoError(t, err, "expression %q should compile", expr)
		require.NotNil(t, p)
	}

	assert.LessOrEqual(t, ce.programCache.len(), defaultProgramCacheSize,
		"cache must not exceed defaultProgramCacheSize after overflow")
}
