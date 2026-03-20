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

package utils

import "testing"

func TestNormalizeAndValidateLLMResourcePath(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		want      string
		wantValid bool
	}{
		{name: "valid path", in: "/chat/completions", want: "/chat/completions", wantValid: true},
		{name: "valid wildcard", in: "/models/*/responses", want: "/models/*/responses", wantValid: true},
		{name: "valid root", in: "/", want: "/", wantValid: true},
		{name: "trim and valid", in: "  /responses  ", want: "/responses", wantValid: true},
		{name: "missing slash", in: "chat.completions", wantValid: false},
		{name: "empty", in: "   ", wantValid: false},
		{name: "invalid character", in: "/chat/completions?x=1", wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotValid := NormalizeAndValidateLLMResourcePath(tt.in)
			if gotValid != tt.wantValid {
				t.Fatalf("expected valid=%v, got %v", tt.wantValid, gotValid)
			}
			if got != tt.want {
				t.Fatalf("expected resource=%q, got %q", tt.want, got)
			}
		})
	}
}
