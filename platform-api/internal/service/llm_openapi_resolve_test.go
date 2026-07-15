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

package service

import (
	"context"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

// The successful URL-fetch path (URL taking precedence when reachable) is covered by
// utils.TestFetchOpenAPISpecFromURL_*; here we verify the resolver's decision logic:
// precedence ordering, fallback on fetch failure, and the "leave empty" case.
func TestResolveTemplateOpenAPISpec(t *testing.T) {
	// A loopback URL is refused by the SSRF guard, so it stands in for an unreachable /
	// disallowed spec URL — the resolver must then fall back to the inline spec.
	const blockedURL = "http://127.0.0.1:9/openapi.yaml"
	const inline = "openapi: 3.0.3\ninfo:\n  title: Inline\n  version: v1.0\npaths: {}\n"

	t.Run("nil template returns empty", func(t *testing.T) {
		if got := resolveTemplateOpenAPISpec(context.Background(), nil, 0, nil); got != "" {
			t.Fatalf("want empty, got %q", got)
		}
	})

	t.Run("both unset returns empty", func(t *testing.T) {
		tpl := &model.LLMProviderTemplate{ID: "t1"}
		if got := resolveTemplateOpenAPISpec(context.Background(), tpl, 0, nil); got != "" {
			t.Fatalf("want empty, got %q", got)
		}
	})

	t.Run("inline only returns inline", func(t *testing.T) {
		tpl := &model.LLMProviderTemplate{ID: "t2", OpenAPISpec: inline}
		if got := resolveTemplateOpenAPISpec(context.Background(), tpl, 0, nil); got != inline {
			t.Fatalf("want inline spec, got %q", got)
		}
	})

	t.Run("url fetch failure falls back to inline", func(t *testing.T) {
		tpl := &model.LLMProviderTemplate{
			ID:          "t3",
			OpenAPISpec: inline,
			Metadata:    &model.LLMProviderTemplateMetadata{OpenapiSpecURL: blockedURL},
		}
		if got := resolveTemplateOpenAPISpec(context.Background(), tpl, 0, nil); got != inline {
			t.Fatalf("want inline fallback, got %q", got)
		}
	})

	t.Run("url fetch failure with no inline returns empty", func(t *testing.T) {
		tpl := &model.LLMProviderTemplate{
			ID:       "t4",
			Metadata: &model.LLMProviderTemplateMetadata{OpenapiSpecURL: blockedURL},
		}
		if got := resolveTemplateOpenAPISpec(context.Background(), tpl, 0, nil); got != "" {
			t.Fatalf("want empty, got %q", got)
		}
	})
}
