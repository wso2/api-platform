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

package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
)

func TestDecodeJSONBody(t *testing.T) {
	const maxBytes = 64
	padding := strings.Repeat(" ", 2*maxBytes)

	cases := []struct {
		name string
		body string
		want *apperror.Def
	}{
		{name: "single object", body: `{"a":1}`},
		{name: "trailing whitespace", body: "{\"a\":1}\n \t"},
		{name: "trailing garbage", body: `{"a":1} garbage`, want: &apperror.APIPublicationValidationFailed},
		{name: "two objects", body: `{"a":1}{"a":2}`, want: &apperror.APIPublicationValidationFailed},
		{name: "malformed", body: `{"a":`, want: &apperror.APIPublicationValidationFailed},
		{name: "empty", body: ``, want: &apperror.APIPublicationValidationFailed},
		{name: "valid object then padding past the cap", body: `{"a":1}` + padding, want: &apperror.PayloadTooLarge},
		{name: "oversized object", body: `{"a":"` + strings.Repeat("x", maxBytes) + `"}`, want: &apperror.PayloadTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(tc.body))
			var dst map[string]any
			err := decodeJSONBody(httptest.NewRecorder(), r, maxBytes, &dst)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("want no error, got %v", err)
				}
				return
			}
			if err == nil || !tc.want.Is(err) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}
