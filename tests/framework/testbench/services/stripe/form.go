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

package stripe

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// parseParams reads Stripe's form encoding from the query string and the body into nested values:
// "a[b]=1" is {"a": {"b": "1"}}, and "list[0]=x&list[1]=y" is {"list": ["x", "y"]}. A key given in
// both places takes the body's value.
func parseParams(rawQuery string, body []byte) (map[string]any, error) {
	root := map[string]any{}
	for _, source := range []string{rawQuery, string(body)} {
		values, err := url.ParseQuery(source)
		if err != nil {
			return nil, fmt.Errorf("the parameters are not form-encoded: %w", err)
		}
		for key, list := range values {
			path, err := splitKey(key)
			if err != nil {
				return nil, err
			}
			if err := setPath(root, path, list[len(list)-1]); err != nil {
				return nil, err
			}
		}
	}
	return toLists(root).(map[string]any), nil
}

// splitKey splits "a[b][0]" into ["a", "b", "0"].
func splitKey(key string) ([]string, error) {
	head, rest, nested := strings.Cut(key, "[")
	if head == "" {
		return nil, fmt.Errorf("the parameter name %q is not valid", key)
	}
	path := []string{head}
	for nested {
		inner, after, closed := strings.Cut(rest, "]")
		if !closed || inner == "" {
			return nil, fmt.Errorf("the parameter name %q is not valid", key)
		}
		path = append(path, inner)
		if after == "" {
			break
		}
		if !strings.HasPrefix(after, "[") {
			return nil, fmt.Errorf("the parameter name %q is not valid", key)
		}
		rest = after[1:]
	}
	return path, nil
}

func setPath(node map[string]any, path []string, value string) error {
	for _, part := range path[:len(path)-1] {
		child, exists := node[part]
		if !exists {
			next := map[string]any{}
			node[part] = next
			node = next
			continue
		}
		next, ok := child.(map[string]any)
		if !ok {
			return fmt.Errorf("the parameter %q is given both as a value and as an object", part)
		}
		node = next
	}
	last := path[len(path)-1]
	if _, isObject := node[last].(map[string]any); isObject {
		return fmt.Errorf("the parameter %q is given both as a value and as an object", last)
	}
	node[last] = value
	return nil
}

// toLists turns objects whose keys are all indexes into lists, in index order.
func toLists(value any) any {
	node, ok := value.(map[string]any)
	if !ok {
		return value
	}
	indexes := make([]int, 0, len(node))
	for key, child := range node {
		node[key] = toLists(child)
		if i, err := strconv.Atoi(key); err == nil && i >= 0 && strconv.Itoa(i) == key {
			indexes = append(indexes, i)
		}
	}
	if len(node) == 0 || len(indexes) != len(node) {
		return node
	}
	sort.Ints(indexes)
	list := make([]any, 0, len(indexes))
	for _, i := range indexes {
		list = append(list, node[strconv.Itoa(i)])
	}
	return list
}

func stringParam(form map[string]any, key string) string {
	value, _ := form[key].(string)
	return value
}

func objectParam(form map[string]any, key string) map[string]any {
	value, _ := form[key].(map[string]any)
	return value
}

func listParam(form map[string]any, key string) []any {
	value, _ := form[key].([]any)
	return value
}

// stringMap reads a metadata object; non-text values are left out.
func stringMap(value map[string]any) map[string]string {
	out := map[string]string{}
	for key, item := range value {
		if text, ok := item.(string); ok {
			out[key] = text
		}
	}
	return out
}

func boolParam(form map[string]any, key string) (value, given bool) {
	text, ok := form[key].(string)
	if !ok {
		return false, false
	}
	return text == "true", true
}

func intParam(form map[string]any, key string, fallback, max int) int {
	text, ok := form[key].(string)
	if !ok {
		return fallback
	}
	n, err := strconv.Atoi(text)
	if err != nil || n < 1 {
		return fallback
	}
	if n > max {
		return max
	}
	return n
}
