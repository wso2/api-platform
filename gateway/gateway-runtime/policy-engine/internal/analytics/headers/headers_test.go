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

package headers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlatten(t *testing.T) {
	want := map[string]string{"x-a": "1", "set-cookie": "a=1, b=2"}
	tests := []struct {
		name string
		in   any
		want map[string]string
	}{
		{"map[string]string is returned as is", map[string]string{"x-a": "1", "set-cookie": "a=1, b=2"}, want},
		{"map[string][]string keeps every value", map[string][]string{"x-a": {"1"}, "set-cookie": {"a=1", "b=2"}}, want},
		{"map[string]interface{} with strings and lists", map[string]interface{}{"x-a": "1", "set-cookie": []interface{}{"a=1", "b=2"}}, want},
		{"JSON object of strings", `{"x-a":"1","set-cookie":"a=1, b=2"}`, want},
		{"JSON object of lists keeps every value", `{"x-a":["1"],"set-cookie":["a=1","b=2"]}`, want},
		{"JSON object mixing both shapes", `{"x-a":"1","set-cookie":["a=1","b=2"]}`, want},
		{"nil", nil, nil},
		{"empty string", "", nil},
		{"empty map", map[string]string{}, nil},
		{"invalid JSON", "{not json", nil},
		{"JSON that is not an object", `["a","b"]`, nil},
		{"unsupported type", 42, nil},
		{"non-string values are dropped", map[string]interface{}{"x-a": "1", "x-n": 7}, map[string]string{"x-a": "1"}},
		{"only non-string values", map[string]interface{}{"x-n": 7}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Flatten(tt.in))
		})
	}
}

func TestValues(t *testing.T) {
	want := map[string][]string{"x-a": {"1"}, "set-cookie": {"a=1", "b=2"}}
	tests := []struct {
		name string
		in   any
		want map[string][]string
	}{
		{"map[string][]string is returned as is", map[string][]string{"x-a": {"1"}, "set-cookie": {"a=1", "b=2"}}, want},
		{"map[string]interface{} with strings and lists", map[string]interface{}{"x-a": "1", "set-cookie": []interface{}{"a=1", "b=2"}}, want},
		{"JSON object mixing both shapes", `{"x-a":"1","set-cookie":["a=1","b=2"]}`, want},
		{"map[string]string: one item each, joined values stay one item", map[string]string{"x-a": "1", "set-cookie": "a=1, b=2"},
			map[string][]string{"x-a": {"1"}, "set-cookie": {"a=1, b=2"}}},
		{"nil", nil, nil},
		{"empty string", "", nil},
		{"invalid JSON", "{not json", nil},
		{"unsupported type", 42, nil},
		{"only non-string values", map[string]interface{}{"x-n": 7}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Values(tt.in))
		})
	}
}
