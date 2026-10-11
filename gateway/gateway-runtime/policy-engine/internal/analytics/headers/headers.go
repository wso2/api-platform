/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

// Package headers converts captured request/response headers into the single
// shape every analytics consumer uses: one string per header name.
//
// Captured headers reach the analytics pipeline in several shapes, depending on
// who produced them and which path carried them:
//
//   - map[string]string: the analytics system policy's own capture (repeated
//     values already joined with ", "), and the correlation store's payload.
//   - map[string][]string: the analytics-header-filter path (finalizeAnalyticsHeaders).
//   - map[string]interface{}: a decoded JSON object, or a third-party policy.
//   - string: any of the above JSON-encoded, as carried in Envoy dynamic metadata.
//
// Flatten is the one place that turns these into map[string]string, so the
// correlation store, the traffic log and every publisher agree on the result.
// Values is its counterpart for consumers that model a header as a list of values.
package headers

import (
	"encoding/json"
	"strings"
)

// valueSeparator joins the values of a repeated header (RFC 9110 section 5.3),
// matching how the analytics system policy flattens its own capture.
const valueSeparator = ", "

// Flatten returns v as one string per header name. A header with several values
// keeps all of them, joined with ", ". It returns nil for nil, an empty value,
// an unrecognised shape or a string that is not a JSON object of headers.
func Flatten(v any) map[string]string {
	switch h := v.(type) {
	case nil:
		return nil
	case map[string]string:
		if len(h) == 0 {
			return nil
		}
		return h
	case map[string][]string:
		if len(h) == 0 {
			return nil
		}
		out := make(map[string]string, len(h))
		for name, values := range h {
			out[name] = strings.Join(values, valueSeparator)
		}
		return out
	case map[string]interface{}:
		if len(h) == 0 {
			return nil
		}
		out := make(map[string]string, len(h))
		for name, value := range h {
			if s, ok := joinValue(value); ok {
				out[name] = s
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case string:
		if h == "" {
			return nil
		}
		var decoded map[string]interface{}
		if err := json.Unmarshal([]byte(h), &decoded); err != nil {
			return nil
		}
		return Flatten(decoded)
	default:
		return nil
	}
}

// joinValue renders one decoded header value as a string: a string as is, a
// list of strings joined with ", ". Any other type is not a header value.
func joinValue(v interface{}) (string, bool) {
	switch value := v.(type) {
	case string:
		return value, true
	case []string:
		return strings.Join(value, valueSeparator), true
	case []interface{}:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			s, ok := item.(string)
			if !ok {
				return "", false
			}
			parts = append(parts, s)
		}
		return strings.Join(parts, valueSeparator), true
	default:
		return "", false
	}
}

// Values returns v with each header's values kept as separate items, for
// consumers that model a header as a list (e.g. OTel attributes, which are string
// arrays). A single string value becomes a one-item list; a value that was
// already joined with ", " stays one item. It accepts the same shapes as Flatten
// and returns nil in the same cases.
func Values(v any) map[string][]string {
	switch h := v.(type) {
	case nil:
		return nil
	case map[string]string:
		if len(h) == 0 {
			return nil
		}
		out := make(map[string][]string, len(h))
		for name, value := range h {
			out[name] = []string{value}
		}
		return out
	case map[string][]string:
		if len(h) == 0 {
			return nil
		}
		return h
	case map[string]interface{}:
		if len(h) == 0 {
			return nil
		}
		out := make(map[string][]string, len(h))
		for name, value := range h {
			if values, ok := listValue(value); ok {
				out[name] = values
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case string:
		if h == "" {
			return nil
		}
		var decoded map[string]interface{}
		if err := json.Unmarshal([]byte(h), &decoded); err != nil {
			return nil
		}
		return Values(decoded)
	default:
		return nil
	}
}

// listValue renders one decoded header value as a list: a string as one item, a
// list of strings as is. Any other type is not a header value.
func listValue(v interface{}) ([]string, bool) {
	switch value := v.(type) {
	case string:
		return []string{value}, true
	case []string:
		return value, true
	case []interface{}:
		out := make([]string, 0, len(value))
		for _, item := range value {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}

// Count returns how many headers v holds, for any shape Flatten accepts. The two
// typed map shapes the correlation store carries are counted without decoding.
func Count(v any) int {
	switch h := v.(type) {
	case map[string]string:
		return len(h)
	case map[string][]string:
		return len(h)
	}
	return len(Flatten(v))
}
