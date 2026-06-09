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

package xdsclient

import (
	"context"
	"encoding/json"
	"testing"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/wso2/api-platform/common/webhooksecret"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestWebhookSecretStateHandlerHandleResources_PopulatesStore(t *testing.T) {
	store := webhooksecret.NewWebhookSecretStore()
	handler := NewWebhookSecretStateHandler(store)

	resources := []*discoveryv3.Resource{
		{
			Resource: mustBuildWebhookSecretStateAny(t, map[string]map[string]string{
				"api-1": {"key-a": "secret-value-1", "key-b": "secret-value-2"},
			}),
		},
	}

	if err := handler.HandleResources(context.Background(), resources, "1"); err != nil {
		t.Fatalf("HandleResources returned error: %v", err)
	}

	secrets := store.GetAllByAPI("api-1")
	if len(secrets) != 2 {
		t.Fatalf("expected 2 secrets for api-1, got %d: %v", len(secrets), secrets)
	}

	secretSet := make(map[string]bool, len(secrets))
	for _, s := range secrets {
		secretSet[s] = true
	}
	if !secretSet["secret-value-1"] || !secretSet["secret-value-2"] {
		t.Fatalf("expected both secret values, got: %v", secrets)
	}
}

func TestWebhookSecretStateHandlerHandleResources_MultipleAPIs(t *testing.T) {
	store := webhooksecret.NewWebhookSecretStore()
	handler := NewWebhookSecretStateHandler(store)

	resources := []*discoveryv3.Resource{
		{
			Resource: mustBuildWebhookSecretStateAny(t, map[string]map[string]string{
				"api-1": {"k1": "s1"},
				"api-2": {"k2": "s2"},
			}),
		},
	}

	if err := handler.HandleResources(context.Background(), resources, "1"); err != nil {
		t.Fatalf("HandleResources returned error: %v", err)
	}

	if got := store.GetAllByAPI("api-1"); len(got) != 1 || got[0] != "s1" {
		t.Fatalf("expected api-1 → [s1], got %v", got)
	}
	if got := store.GetAllByAPI("api-2"); len(got) != 1 || got[0] != "s2" {
		t.Fatalf("expected api-2 → [s2], got %v", got)
	}
}

func TestWebhookSecretStateHandlerHandleResources_EmptySnapshotClearsStore(t *testing.T) {
	store := webhooksecret.NewWebhookSecretStore()
	if err := store.Store("api-1", "existing-key", "existing-secret"); err != nil {
		t.Fatalf("failed to seed store: %v", err)
	}

	handler := NewWebhookSecretStateHandler(store)
	if err := handler.HandleResources(context.Background(), nil, "2"); err != nil {
		t.Fatalf("HandleResources returned error: %v", err)
	}

	if got := store.GetAllByAPI("api-1"); len(got) != 0 {
		t.Fatalf("expected store cleared, got %v", got)
	}
}

func TestWebhookSecretStateHandlerHandleResources_NilResourceSkipped(t *testing.T) {
	store := webhooksecret.NewWebhookSecretStore()
	handler := NewWebhookSecretStateHandler(store)

	resources := []*discoveryv3.Resource{
		nil,
		{Resource: nil},
		{
			Resource: mustBuildWebhookSecretStateAny(t, map[string]map[string]string{
				"api-1": {"k": "v"},
			}),
		},
	}

	if err := handler.HandleResources(context.Background(), resources, "3"); err != nil {
		t.Fatalf("HandleResources returned error: %v", err)
	}

	if got := store.GetAllByAPI("api-1"); len(got) != 1 || got[0] != "v" {
		t.Fatalf("expected api-1 → [v], got %v", got)
	}
}

// mustBuildWebhookSecretStateAny encodes secrets in the double-wrapped wire format
// that the xDS server produces:
//
//	outer Any.Value = proto.Marshal(inner Any)
//	inner Any.Value = proto.Marshal(structpb.Struct)
//
// This matches what the event-gateway handler receives from the LinearCache.
func mustBuildWebhookSecretStateAny(t *testing.T, secrets map[string]map[string]string) *anypb.Any {
	t.Helper()

	payload := map[string]any{"secrets": secrets}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal secrets payload: %v", err)
	}

	s := &structpb.Struct{}
	if err := s.UnmarshalJSON(jsonBytes); err != nil {
		t.Fatalf("failed to unmarshal JSON into Struct: %v", err)
	}

	structBytes, err := proto.Marshal(s)
	if err != nil {
		t.Fatalf("failed to marshal Struct: %v", err)
	}

	innerAny := &anypb.Any{
		TypeUrl: WebhookSecretStateTypeURL,
		Value:   structBytes,
	}
	innerAnyBytes, err := proto.Marshal(innerAny)
	if err != nil {
		t.Fatalf("failed to marshal inner Any: %v", err)
	}

	return &anypb.Any{
		TypeUrl: WebhookSecretStateTypeURL,
		Value:   innerAnyBytes,
	}
}
