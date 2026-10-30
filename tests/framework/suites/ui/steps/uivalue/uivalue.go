/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.  You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package uivalue

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

type expansionKey struct{}

type expansionState struct {
	mu     sync.Mutex
	values map[string]string
}

// configureContextValues installs the scenario-value lookup behind ${CTX:name} once.
var configureContextValues sync.Once

var uniquePlaceholder = regexp.MustCompile(`\$\{UNIQUE:([A-Za-z0-9_.-]+)\}`)

// WithExpansionState starts a scenario-local cache. The cache is deliberately context
// scoped rather than runner scoped: the same readable placeholder is reused across steps in
// one scenario, while concurrent scenarios in one runner still receive different resources.
func WithExpansionState(ctx context.Context) context.Context {
	return context.WithValue(ctx, expansionKey{}, &expansionState{values: make(map[string]string)})
}

// Expand resolves framework placeholders at the UI step boundary. Godog passes
// captured text verbatim; resolving here keeps the feature readable while ensuring every
// browser/API operation uses the same scenario-owned value.
func Expand(ctx context.Context, value string) (string, error) {
	if strings.Contains(value, unique.ContextPlaceholder) {
		configureContextValues.Do(func() {
			unique.ContextValue = func(ctx context.Context, name string) (string, error) {
				return tcontext.ResolveString(ctx, name)
			}
		})
		expanded, err := unique.Expand(ctx, value)
		if err != nil {
			return "", fmt.Errorf("expanding UI value %q: %w", value, err)
		}
		value = expanded
	}
	if !strings.Contains(value, unique.Placeholder) {
		return value, nil
	}
	state, _ := ctx.Value(expansionKey{}).(*expansionState)
	if state == nil {
		// Keep the helper useful in focused unit tests and defensive callers that do not run
		// through Godog's scenario hook. The suite hook always supplies the stable cache.
		expanded, err := unique.Expand(ctx, value)
		if err != nil {
			return "", fmt.Errorf("expanding UI value %q: %w", value, err)
		}
		return expanded, nil
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	matches := uniquePlaceholder.FindAllStringSubmatchIndex(value, -1)
	if strings.Contains(value, unique.Placeholder) && len(matches) == 0 {
		return "", fmt.Errorf("expanding UI value %q: malformed UNIQUE placeholder", value)
	}
	var out strings.Builder
	last := 0
	for _, match := range matches {
		out.WriteString(value[last:match[0]])
		base := value[match[2]:match[3]]
		resolved, ok := state.values[base]
		if !ok {
			var err error
			resolved, err = unique.Unique(ctx, base)
			if err != nil {
				return "", fmt.Errorf("expanding UI value %q: %w", value, err)
			}
			state.values[base] = resolved
		}
		out.WriteString(resolved)
		last = match[1]
	}
	out.WriteString(value[last:])
	expanded := out.String()
	if strings.Contains(expanded, unique.Placeholder) {
		return "", fmt.Errorf("expanding UI value %q: malformed UNIQUE placeholder", value)
	}
	return expanded, nil
}
