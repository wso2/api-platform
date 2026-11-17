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

package apikey

import (
	"testing"
	"time"
)

func TestOperationsAllow(t *testing.T) {
	cases := []struct {
		name       string
		operations string
		operation  string
		method     string
		want       bool
	}{
		{"all", "*", "/anything", "GET", true},
		{"all in an array", `["*"]`, "/anything", "GET", true},
		{"unscoped legacy key", "", "/anything", "GET", true},
		{"a named operation", `["claude-haiku","GET /models"]`, "claude-haiku", "POST", true},
		{"a method and path", `["claude-haiku","GET /models"]`, "/models", "GET", true},
		{"method is case-insensitive", `["get /models"]`, "/models", "GET", true},
		{"another operation", `["claude-haiku","GET /models"]`, "wso2-auto", "POST", false},
		{"the dispatch error chain", `["claude-haiku","GET /models"]`, "__dispatch__", "POST", false},
		{"another method", `["GET /models"]`, "/models", "POST", false},
		{"a path prefix is not a match", `["GET /models"]`, "/models/x", "GET", false},
		{"an empty array reaches nothing", `[]`, "/models", "GET", false},
		{"unparseable fails closed", `claude-haiku`, "claude-haiku", "POST", false},
		{"no operation", `["claude-haiku"]`, "", "POST", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := OperationsAllow(tc.operations, tc.operation, tc.method); got != tc.want {
				t.Fatalf("OperationsAllow(%q, %q, %q) = %v, want %v", tc.operations, tc.operation, tc.method, got, tc.want)
			}
		})
	}
}

// A key scoped to some operations validates on them and on nothing else, with the
// same answer — no key — whether the operation is another Model or none at all.
func TestResolveValidatedAPIKey_EnforcesOperations(t *testing.T) {
	store := NewAPIkeyStore()
	key := &APIKey{
		ID: "k", Name: "haiku-app", APIKey: ComputeAPIKeyHash("model-key"), APIId: "endpoint-1",
		Operations: `["claude-haiku","GET /models"]`, Status: Active, CreatedAt: time.Now(),
	}
	if err := store.StoreAPIKey("endpoint-1", key); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		operation, method string
		want              bool
	}{
		{"claude-haiku", "POST", true},
		{"/models", "GET", true},
		{"wso2-auto", "POST", false},
		{"__dispatch__", "POST", false},
	} {
		got, err := store.ResolveValidatedAPIKey("endpoint-1", tc.operation, tc.method, "model-key")
		if err != nil {
			t.Fatalf("%s: %v", tc.operation, err)
		}
		if (got != nil) != tc.want {
			t.Errorf("%s %s: resolved=%v, want %v", tc.method, tc.operation, got != nil, tc.want)
		}
	}
}
