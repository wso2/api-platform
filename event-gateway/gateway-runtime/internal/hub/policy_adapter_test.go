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

package hub

import (
	"testing"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
)

func TestSubscribeToRequestHeaderContext_PopulatesAPIKeyMetadata(t *testing.T) {
	msg := &connectors.Message{
		Headers: map[string][]string{
			"X-API-Key": {"secret"},
		},
		Metadata: map[string]interface{}{
			"request_path":   "/repos/v1/hub?api_key=secret",
			"request_method": "POST",
		},
	}
	binding := &ChannelBinding{
		APIID:   "api-123",
		Name:    "repo-watcher",
		Mode:    "websub",
		Context: "/repos",
		Version: "v1",
		Vhost:   "example.com",
	}

	ctx := SubscribeToRequestHeaderContext(msg, binding)

	if ctx.SharedContext.APIId != "api-123" {
		t.Fatalf("expected APIId api-123, got %q", ctx.SharedContext.APIId)
	}
	if ctx.SharedContext.APIName != "repo-watcher" {
		t.Fatalf("expected APIName repo-watcher, got %q", ctx.SharedContext.APIName)
	}
	if ctx.SharedContext.APIVersion != "v1" {
		t.Fatalf("expected APIVersion v1, got %q", ctx.SharedContext.APIVersion)
	}
	if ctx.SharedContext.OperationPath != "/repos/v1/hub?api_key=secret" {
		t.Fatalf("expected OperationPath to use request metadata, got %q", ctx.SharedContext.OperationPath)
	}
	if ctx.Path != "/repos/v1/hub?api_key=secret" {
		t.Fatalf("expected request path from metadata, got %q", ctx.Path)
	}
	if ctx.Method != "POST" {
		t.Fatalf("expected request method POST, got %q", ctx.Method)
	}
	if got := ctx.Headers.Get("x-api-key"); len(got) != 1 || got[0] != "secret" {
		t.Fatalf("expected x-api-key header to be preserved, got %#v", got)
	}
}

func TestMessageToRequestHeaderContext_UsesWebhookRequestMetadata(t *testing.T) {
	msg := &connectors.Message{
		Metadata: map[string]interface{}{
			"request_path":   "/repos/v1/webhook-receiver?topic=issues&api_key=secret",
			"request_method": "POST",
		},
	}
	binding := &ChannelBinding{
		APIID:   "api-456",
		Name:    "repo-watcher",
		Mode:    "websub",
		Context: "/repos",
		Version: "v1",
	}

	ctx := MessageToRequestHeaderContext(msg, binding)

	if ctx.SharedContext.APIId != "api-456" {
		t.Fatalf("expected APIId api-456, got %q", ctx.SharedContext.APIId)
	}
	if ctx.Path != "/repos/v1/webhook-receiver?topic=issues&api_key=secret" {
		t.Fatalf("expected webhook request path, got %q", ctx.Path)
	}
	if ctx.SharedContext.OperationPath != ctx.Path {
		t.Fatalf("expected operation path to match request path, got %q", ctx.SharedContext.OperationPath)
	}
	if ctx.Method != "POST" {
		t.Fatalf("expected request method POST, got %q", ctx.Method)
	}
}
