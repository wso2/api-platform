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

package platformgateway

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const keyAuthHeader = "authHeader"

// BasicAuthHeader returns the Authorization header value carrying the supplied
// credentials as HTTP Basic authentication.
func BasicAuthHeader(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

func scenarioLabel(ctx context.Context) string {
	if local, ok := tcontext.LocalOf(ctx); ok {
		return local.Runner()
	}
	return "unknown runner"
}

func gatewayOrdinal(word string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(word)) {
	case "first":
		return 0, nil
	case "second":
		return 1, nil
	default:
		return 0, fmt.Errorf("unsupported gateway ordinal %q", word)
	}
}

func traverseJSON(doc any, path string) (any, bool) {
	current := doc
	for _, segment := range strings.Split(path, ".") {
		if segment == "" {
			continue
		}
		name, indices := splitIndices(segment)
		if name != "" {
			asMap, ok := current.(map[string]any)
			if !ok {
				return nil, false
			}
			if current, ok = asMap[name]; !ok {
				return nil, false
			}
		}
		for _, index := range indices {
			asSlice, ok := current.([]any)
			if !ok || index < 0 || index >= len(asSlice) {
				return nil, false
			}
			current = asSlice[index]
		}
	}
	return current, true
}

func splitIndices(segment string) (string, []int) {
	if n, err := strconv.Atoi(segment); err == nil {
		return "", []int{n}
	}
	open := strings.Index(segment, "[")
	if open < 0 {
		return segment, nil
	}
	name := segment[:open]
	var indices []int
	rest := segment[open:]
	for strings.HasPrefix(rest, "[") {
		end := strings.Index(rest, "]")
		if end < 0 {
			return segment, nil
		}
		index, err := strconv.Atoi(rest[1:end])
		if err != nil || index < 0 {
			return segment, nil
		}
		indices = append(indices, index)
		rest = rest[end+1:]
	}
	if rest == "" {
		return name, indices
	}
	return segment, nil
}
