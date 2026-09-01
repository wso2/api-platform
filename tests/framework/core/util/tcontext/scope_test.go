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

package tcontext

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// blockCtx builds a context with both scopes attached, as the block engine will.
func blockCtx(block, runner string) (context.Context, *Shared, *Local) {
	shared := NewShared(block)
	local := NewLocal(runner)
	ctx := WithLocal(WithShared(context.Background(), shared), local)
	return ctx, shared, local
}

func TestReadIntents(t *testing.T) {
	ctx, shared, local := blockCtx("gateway-core", "api-deploy")
	shared.Set("baseUrl", "http://127.0.0.1:34199")
	local.Set("createdApiId", "abc-123")

	t.Run("Resolve returns a required value", func(t *testing.T) {
		v, err := Resolve(ctx, "createdApiId")
		require.NoError(t, err)
		require.Equal(t, "abc-123", v)
	})

	t.Run("Resolve on a missing key fails immediately and lists what is present", func(t *testing.T) {
		// The whole point: a mistyped step argument fails here with a legible message
		// rather than surfacing later as a nil dereference somewhere unrelated.
		_, err := Resolve(ctx, "createdApiID")
		require.ErrorContains(t, err, `no value in context for key "createdApiID"`)
		require.ErrorContains(t, err, "createdApiId")
		require.ErrorContains(t, err, "baseUrl")
	})

	t.Run("Resolve strips an angle-bracket reference wrapper", func(t *testing.T) {
		// So a step accepts either a bare key or a Gherkin-style reference.
		v, err := Resolve(ctx, "<createdApiId>")
		require.NoError(t, err)
		require.Equal(t, "abc-123", v)
	})

	t.Run("Get is nullable and reports absence without an error", func(t *testing.T) {
		v, ok := Get(ctx, "httpResponse")
		require.False(t, ok)
		require.Nil(t, v)
	})

	t.Run("Contains reports presence only", func(t *testing.T) {
		require.True(t, Contains(ctx, "baseUrl"))
		require.False(t, Contains(ctx, "nope"))
	})

	t.Run("ResolveString rejects a type mismatch as loudly as a missing key", func(t *testing.T) {
		local.Set("count", 42)
		_, err := ResolveString(ctx, "count")
		require.ErrorContains(t, err, "holds int, not a string")
	})
}

func TestScopePrecedence(t *testing.T) {
	ctx, shared, local := blockCtx("b", "r")

	t.Run("local shadows shared", func(t *testing.T) {
		// This is what lets a runner override a block-wide default without mutating
		// shared state that its siblings also read.
		shared.Set("tier", "block-default")
		local.Set("tier", "runner-override")

		v, err := Resolve(ctx, "tier")
		require.NoError(t, err)
		require.Equal(t, "runner-override", v)
	})

	t.Run("shared is still visible when local has no such key", func(t *testing.T) {
		shared.Set("baseGatewayUrl", "http://gw")
		v, err := Resolve(ctx, "baseGatewayUrl")
		require.NoError(t, err)
		require.Equal(t, "http://gw", v)
	})

	t.Run("a shadowed key is listed once, not twice", func(t *testing.T) {
		shared.Set("dup", 1)
		local.Set("dup", 2)
		count := 0
		for _, k := range Keys(ctx) {
			if k == "dup" {
				count++
			}
		}
		require.Equal(t, 1, count)
	})
}

func TestScopeIsolation(t *testing.T) {
	t.Run("two runners in one block share shared but not local", func(t *testing.T) {
		// The core isolation property. Sibling runners run concurrently on one topology,
		// so a value one writes must not be visible to the other.
		shared := NewShared("gateway-core")
		shared.Set("baseUrl", "http://gw")

		first := WithLocal(WithShared(context.Background(), shared), NewLocal("runner-a"))
		second := WithLocal(WithShared(context.Background(), shared), NewLocal("runner-b"))

		require.NoError(t, Set(first, "apiId", "from-a"))

		v, err := Resolve(second, "baseUrl")
		require.NoError(t, err)
		require.Equal(t, "http://gw", v, "shared must be visible to both")

		require.False(t, Contains(second, "apiId"), "local must NOT leak between runners")
	})

	t.Run("two blocks share nothing at all", func(t *testing.T) {
		a := WithLocal(WithShared(context.Background(), NewShared("block-a")), NewLocal("r"))
		b := WithLocal(WithShared(context.Background(), NewShared("block-b")), NewLocal("r"))

		sharedA, ok := SharedOf(a)
		require.True(t, ok)
		sharedA.Set("baseUrl", "http://a")

		require.False(t, Contains(b, "baseUrl"), "blocks must be fully isolated")
	})

	t.Run("a context with no scopes fails legibly rather than panicking", func(t *testing.T) {
		bare := context.Background()
		require.False(t, Contains(bare, "anything"))
		_, err := Resolve(bare, "anything")
		require.ErrorContains(t, err, "(none)")
		require.ErrorContains(t, Set(bare, "k", "v"), "no local scope in context")
	})
}

func TestRemove(t *testing.T) {
	// Remove exists for the request funnel: clearing a stored response BEFORE a call means
	// a step that throws leaves the key ABSENT rather than holding a previous step's
	// response for the next assertion to pass against.
	ctx, _, _ := blockCtx("b", "r")
	require.NoError(t, Set(ctx, "httpResponse", "old"))
	require.True(t, Contains(ctx, "httpResponse"))

	Remove(ctx, "httpResponse")
	require.False(t, Contains(ctx, "httpResponse"),
		"a cleared response must be absent, not stale")
}

func TestLists(t *testing.T) {
	ctx, _, local := blockCtx("b", "r")

	t.Run("append and read back", func(t *testing.T) {
		local.Append("createdApiIds", "a")
		local.Append("createdApiIds", "b")
		require.Equal(t, []any{"a", "b"}, local.List("createdApiIds"))
	})

	t.Run("List returns a copy, so a concurrent append cannot tear it", func(t *testing.T) {
		got := local.List("createdApiIds")
		local.Append("createdApiIds", "c")
		require.Len(t, got, 2, "the earlier read must not observe the later append")
	})

	t.Run("ListKeys reports only non-empty lists", func(t *testing.T) {
		local.Append("createdAppIds", "x")
		require.Equal(t, []string{"createdApiIds", "createdAppIds"}, local.ListKeys())

		local.ClearList("createdAppIds")
		require.Equal(t, []string{"createdApiIds"}, local.ListKeys())
	})

	t.Run("an unknown list is empty rather than nil-panicking", func(t *testing.T) {
		require.Empty(t, local.List("never-used"))
	})

	_ = ctx
}

func TestConcurrentAccess(t *testing.T) {
	// Scenarios within a runner may execute concurrently, and every runner in a block
	// reads shared state simultaneously. A race here would surface as an unrelated flake
	// rather than as a failure pointing at the write, so it is worth proving directly.
	// Run with -race for this to mean anything.
	shared := NewShared("b")
	shared.Set("baseUrl", "http://gw")
	local := NewLocal("r")
	ctx := WithLocal(WithShared(context.Background(), shared), local)

	const workers = 24
	var wg sync.WaitGroup

	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)

			require.NoError(t, Set(ctx, key, i))
			local.Append("ids", i)

			if _, err := Resolve(ctx, key); err != nil {
				t.Errorf("resolving own key %q: %v", key, err)
			}
			if _, err := Resolve(ctx, "baseUrl"); err != nil {
				t.Errorf("resolving shared key: %v", err)
			}
			_ = Keys(ctx)
			_ = local.List("ids")
		}(i)
	}

	wg.Wait()
	require.Len(t, local.List("ids"), workers, "every append must be recorded exactly once")
}

func TestBlockAndRunnerNamesAreTraceable(t *testing.T) {
	// Diagnostics need to say which block and runner a value came from.
	_, shared, local := blockCtx("gateway-core/postgres", "jwt-auth")
	require.Equal(t, "gateway-core/postgres", shared.Block())
	require.Equal(t, "jwt-auth", local.Runner())
}
