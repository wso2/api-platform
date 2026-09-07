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

package unique

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

func runnerCtx(runner string) context.Context {
	return tcontext.WithLocal(
		tcontext.WithShared(context.Background(), tcontext.NewShared("b")),
		tcontext.NewLocal(runner),
	)
}

func TestUniqueNames(t *testing.T) {
	g, err := NewGenerator()
	require.NoError(t, err)

	t.Run("repeated calls never collide", func(t *testing.T) {
		// A scenario creating three APIs needs three distinct names.
		seen := map[string]bool{}
		for range 1000 {
			n := g.Unique("TestAPI")
			require.False(t, seen[n], "duplicate name %q", n)
			seen[n] = true
		}
	})

	t.Run("the base stays readable in the name", func(t *testing.T) {
		// So a failing test's output still says what the resource was.
		require.True(t, strings.HasPrefix(g.Unique("TestAPI"), "TestAPI_"))
	})

	t.Run("all names from one generator share its suffix", func(t *testing.T) {
		// Attribution: a leftover resource is traceable to the runner that leaked it.
		a, b := g.Unique("x"), g.Unique("y")
		require.Contains(t, a, g.Suffix())
		require.Contains(t, b, g.Suffix())
	})

	t.Run("illegal characters are replaced rather than passed through", func(t *testing.T) {
		n := g.Unique("My API/v1 (test)")
		require.NotContains(t, n, "/")
		require.NotContains(t, n, " ")
		require.NotContains(t, n, "(")
	})

	t.Run("an empty or unusable base still yields a legal name", func(t *testing.T) {
		require.True(t, strings.HasPrefix(g.Unique(""), "res_"))
		require.True(t, strings.HasPrefix(g.Unique("///"), "res_"))
	})
}

func TestTwoGeneratorsNeverShareASuffix(t *testing.T) {
	// This is what keeps two concurrent runners apart. If suffixes collided, two runners
	// creating "TestAPI" would collide exactly as hardcoded names do.
	seen := map[string]bool{}
	for range 500 {
		g, err := NewGenerator()
		require.NoError(t, err)
		require.False(t, seen[g.Suffix()], "suffix %q generated twice", g.Suffix())
		seen[g.Suffix()] = true
	}
}

func TestUniqueContextIsURLSafe(t *testing.T) {
	g, err := NewGenerator()
	require.NoError(t, err)

	ctxPath := g.UniqueContext("My Test API")
	t.Run("starts with a slash and is lowercase", func(t *testing.T) {
		// A context becomes part of a URL, so it has different legality rules from a display
		// name — which is why it is a separate method.
		require.True(t, strings.HasPrefix(ctxPath, "/"))
		require.Equal(t, strings.ToLower(ctxPath), ctxPath)
	})

	t.Run("contains nothing needing URL escaping", func(t *testing.T) {
		for _, bad := range []string{" ", "(", ")", "_", "?", "#", "%"} {
			require.NotContains(t, strings.TrimPrefix(ctxPath, "/"), bad)
		}
	})

	t.Run("repeated contexts differ", func(t *testing.T) {
		require.NotEqual(t, ctxPath, g.UniqueContext("My Test API"))
	})
}

func TestContextIntegration(t *testing.T) {
	t.Run("a generator is created lazily when none was installed", func(t *testing.T) {
		// A step must not fail merely because wiring was forgotten.
		ctx := runnerCtx("r")
		n, err := Unique(ctx, "TestAPI")
		require.NoError(t, err)
		require.Contains(t, n, "TestAPI_")
	})

	t.Run("the same runner reuses one generator, so the suffix is stable", func(t *testing.T) {
		ctx := runnerCtx("r")
		first, err := Unique(ctx, "a")
		require.NoError(t, err)
		second, err := Unique(ctx, "b")
		require.NoError(t, err)

		g, err := Of(ctx)
		require.NoError(t, err)
		require.Contains(t, first, g.Suffix())
		require.Contains(t, second, g.Suffix())
	})

	t.Run("two runners get different suffixes", func(t *testing.T) {
		a, b := runnerCtx("runner-a"), runnerCtx("runner-b")
		ga, err := Of(a)
		require.NoError(t, err)
		gb, err := Of(b)
		require.NoError(t, err)
		require.NotEqual(t, ga.Suffix(), gb.Suffix())
	})

	t.Run("an explicitly installed generator is used", func(t *testing.T) {
		ctx := runnerCtx("r")
		g, err := NewGenerator()
		require.NoError(t, err)
		require.NoError(t, Install(ctx, g))

		n, err := Unique(ctx, "x")
		require.NoError(t, err)
		require.Contains(t, n, g.Suffix())
	})

	t.Run("a wrongly-typed value under the key is reported", func(t *testing.T) {
		ctx := runnerCtx("r")
		require.NoError(t, tcontext.Set(ctx, suffixKey, "not a generator"))
		_, err := Of(ctx)
		require.ErrorContains(t, err, "not a *Generator")
	})
}

func TestExpandPlaceholder(t *testing.T) {
	ctx := runnerCtx("r")

	t.Run("a placeholder becomes a generated name", func(t *testing.T) {
		// A feature file never contains a literal resource name.
		out, err := Expand(ctx, `{"name":"${UNIQUE:TestAPI}"}`)
		require.NoError(t, err)
		require.NotContains(t, out, Placeholder)
		require.Contains(t, out, "TestAPI_")
	})

	t.Run("the same base twice in one string resolves to the SAME name", func(t *testing.T) {
		// So a step can reference one resource more than once in a payload.
		out, err := Expand(ctx, `{"name":"${UNIQUE:api}","displayName":"${UNIQUE:api}"}`)
		require.NoError(t, err)

		parts := strings.Split(out, `"`)
		var values []string
		for _, p := range parts {
			if strings.HasPrefix(p, "api_") {
				values = append(values, p)
			}
		}
		require.Len(t, values, 2)
		require.Equal(t, values[0], values[1], "one base must resolve consistently within a string")
	})

	t.Run("different bases resolve differently", func(t *testing.T) {
		out, err := Expand(ctx, `${UNIQUE:one}|${UNIQUE:two}`)
		require.NoError(t, err)
		halves := strings.Split(out, "|")
		require.Len(t, halves, 2)
		require.NotEqual(t, halves[0], halves[1])
	})

	t.Run("a string with no placeholder is untouched", func(t *testing.T) {
		const s = `{"name":"literal"}`
		out, err := Expand(ctx, s)
		require.NoError(t, err)
		require.Equal(t, s, out)
	})

	t.Run("a placeholder with no closing brace at all is an error", func(t *testing.T) {
		// A name that LOOKS generated but is not would collide exactly as a hardcoded one
		// does — and would be far harder to spot.
		_, err := Expand(ctx, `name is ${UNIQUE:oops`)
		require.ErrorContains(t, err, "unterminated")
	})

	t.Run("a placeholder closed by surrounding syntax is caught as a bad base", func(t *testing.T) {
		// The subtle case: inside JSON, ${UNIQUE:oops" is "terminated" by the JSON's own
		// closing brace, so the base becomes `oops"`. Taking that as a name would produce a
		// plausible-looking result from a typo.
		_, err := Expand(ctx, `{"name":"${UNIQUE:oops"}`)
		require.ErrorContains(t, err, "invalid base")
		require.ErrorContains(t, err, "probably unterminated")
	})

	t.Run("an empty base is rejected", func(t *testing.T) {
		_, err := Expand(ctx, `${UNIQUE:}`)
		require.ErrorContains(t, err, "empty base")
	})

	t.Run("dots, dashes and underscores are legal in a base", func(t *testing.T) {
		out, err := Expand(ctx, `${UNIQUE:my-api_v1.0}`)
		require.NoError(t, err)
		require.NotContains(t, out, Placeholder)
	})

	t.Run("surrounding text is preserved exactly", func(t *testing.T) {
		out, err := Expand(ctx, `prefix-${UNIQUE:mid}-suffix`)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(out, "prefix-"))
		require.True(t, strings.HasSuffix(out, "-suffix"))
	})
}

func TestConcurrentGeneration(t *testing.T) {
	// Scenarios within a runner may run concurrently, so name generation must be safe and
	// must not produce duplicates under contention. Run with -race.
	ctx := runnerCtx("r")

	const workers = 32
	const perWorker = 50

	var (
		mu   sync.Mutex
		seen = map[string]bool{}
		wg   sync.WaitGroup
	)

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perWorker {
				n, err := Unique(ctx, "TestAPI")
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				mu.Lock()
				if seen[n] {
					t.Errorf("duplicate name under concurrency: %q", n)
				}
				seen[n] = true
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	require.Len(t, seen, workers*perWorker)

	// And all of them share one suffix, so the runner remains identifiable.
	g, err := Of(ctx)
	require.NoError(t, err)
	for n := range seen {
		require.Contains(t, n, g.Suffix())
	}
}

func TestExpandResolvesContextPlaceholders(t *testing.T) {
	restore := ContextValue
	t.Cleanup(func() { ContextValue = restore })

	t.Run("substitutes an exposed value", func(t *testing.T) {
		ContextValue = func(context.Context, string) (string, error) { return "abc123", nil }
		got, err := Expand(context.Background(), "Bearer ${CTX:jwtToken}")
		require.NoError(t, err)
		require.Equal(t, "Bearer abc123", got)
	})

	t.Run("a resolver error fails the expansion rather than substituting nothing", func(t *testing.T) {
		// An empty substitution would send "Authorization: Bearer " and surface much later as a
		// 401, naming neither the placeholder nor the step that should have set it.
		ContextValue = func(context.Context, string) (string, error) {
			return "", errors.New(`"jwtToken" is not set in this scenario`)
		}
		_, err := Expand(context.Background(), "Bearer ${CTX:jwtToken}")
		require.Error(t, err)
		require.Contains(t, err.Error(), "jwtToken")
	})

	t.Run("errors when the suite exposes nothing", func(t *testing.T) {
		ContextValue = nil
		_, err := Expand(context.Background(), "${CTX:jwtToken}")
		require.Error(t, err)
		require.Contains(t, err.Error(), "exposes no context values")
	})

	t.Run("an unterminated placeholder is an error", func(t *testing.T) {
		ContextValue = func(context.Context, string) (string, error) { return "x", nil }
		_, err := Expand(context.Background(), "Bearer ${CTX:jwtToken")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unterminated")
	})

	t.Run("leaves a string without the placeholder untouched", func(t *testing.T) {
		ContextValue = func(context.Context, string) (string, error) { return "x", nil }
		got, err := Expand(context.Background(), "Bearer literal")
		require.NoError(t, err)
		require.Equal(t, "Bearer literal", got)
	})
}
