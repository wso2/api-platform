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
	"errors"
	"testing"
	"time"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/wso2/api-platform/common/apikey"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestAPIKeyStateHandlerHandleResources_ReplacesStore(t *testing.T) {
	store := apikey.NewAPIkeyStore()
	handler := NewAPIKeyStateHandler(store)

	resources := []*discoveryv3.Resource{
		{
			Resource: mustBuildAPIKeyStateAny(t, APIKeyStateResource{
				Version: 1,
				APIKeys: []APIKeyData{
					{
						ID:              "key-1",
						Name:            "test-key",
						APIKey:          apikey.ComputeAPIKeyHash("plain-test-key"),
						APIId:           "api-1",
						ApplicationID:   "app-1",
						ApplicationName: "Test App",
						Operations:      "*",
						Status:          "active",
						CreatedAt:       time.Now(),
						CreatedBy:       "tester",
						UpdatedAt:       time.Now(),
						Source:          "external",
					},
				},
			}),
		},
	}

	if err := handler.HandleResources(context.Background(), resources, "42"); err != nil {
		t.Fatalf("HandleResources returned error: %v", err)
	}

	resolved, err := store.ResolveValidatedAPIKey("api-1", "/repos/v1/hub", "POST", "plain-test-key", "")
	if err != nil {
		t.Fatalf("expected stored key to resolve, got error: %v", err)
	}
	if resolved == nil {
		t.Fatal("expected resolved API key, got nil")
	}
	if resolved.ApplicationID != "app-1" {
		t.Fatalf("expected ApplicationID app-1, got %q", resolved.ApplicationID)
	}
	if resolved.ApplicationName != "Test App" {
		t.Fatalf("expected ApplicationName Test App, got %q", resolved.ApplicationName)
	}
}

func TestAPIKeyStateHandlerHandleResources_EmptySnapshotClearsStore(t *testing.T) {
	store := apikey.NewAPIkeyStore()
	if err := store.StoreAPIKey("api-1", &apikey.APIKey{
		ID:         "key-1",
		Name:       "test-key",
		APIKey:     apikey.ComputeAPIKeyHash("plain-test-key"),
		APIId:      "api-1",
		Operations: "*",
		Status:     apikey.Active,
		CreatedAt:  time.Now(),
		CreatedBy:  "tester",
		UpdatedAt:  time.Now(),
		Source:     "external",
	}); err != nil {
		t.Fatalf("failed to seed API key store: %v", err)
	}

	handler := NewAPIKeyStateHandler(store)
	if err := handler.HandleResources(context.Background(), nil, "43"); err != nil {
		t.Fatalf("HandleResources returned error: %v", err)
	}

	resolved, err := store.ResolveValidatedAPIKey("api-1", "/repos/v1/hub", "POST", "plain-test-key", "")
	if err == nil || resolved != nil {
		t.Fatalf("expected API key store to be cleared, got resolved=%v err=%v", resolved, err)
	}
}

func TestAPIKeyStateHandlerHandleResources_KeepsExistingStoreOnInvalidSnapshot(t *testing.T) {
	store := apikey.NewAPIkeyStore()
	if err := store.StoreAPIKey("api-1", &apikey.APIKey{
		ID:         "existing-key",
		Name:       "existing-key",
		APIKey:     apikey.ComputeAPIKeyHash("plain-test-key"),
		APIId:      "api-1",
		Operations: "*",
		Status:     apikey.Active,
		CreatedAt:  time.Now(),
		CreatedBy:  "tester",
		UpdatedAt:  time.Now(),
		Source:     "external",
	}); err != nil {
		t.Fatalf("failed to seed API key store: %v", err)
	}

	handler := NewAPIKeyStateHandler(store)
	resources := []*discoveryv3.Resource{
		{
			Resource: mustBuildAPIKeyStateAny(t, APIKeyStateResource{
				Version: 2,
				APIKeys: []APIKeyData{
					{
						ID:         "invalid-key",
						Name:       "invalid-key",
						APIKey:     "",
						APIId:      "api-1",
						Operations: "*",
						Status:     "active",
						CreatedAt:  time.Now(),
						CreatedBy:  "tester",
						UpdatedAt:  time.Now(),
						Source:     "external",
					},
				},
			}),
		},
	}

	err := handler.HandleResources(context.Background(), resources, "44")
	if !errors.Is(err, apikey.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	resolved, err := store.ResolveValidatedAPIKey("api-1", "/repos/v1/hub", "POST", "plain-test-key", "")
	if err != nil {
		t.Fatalf("expected existing key to remain after failed update, got error: %v", err)
	}
	if resolved == nil || resolved.ID != "existing-key" {
		t.Fatalf("expected existing key to remain, got %#v", resolved)
	}
}

func mustBuildAPIKeyStateAny(t *testing.T, state APIKeyStateResource) *anypb.Any {
	t.Helper()

	jsonBytes, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal API key state: %v", err)
	}

	payload := &structpb.Struct{}
	if err := payload.UnmarshalJSON(jsonBytes); err != nil {
		t.Fatalf("failed to unmarshal API key JSON into struct: %v", err)
	}

	structBytes, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal struct: %v", err)
	}

	innerAny := &anypb.Any{
		TypeUrl: "type.googleapis.com/google.protobuf.Struct",
		Value:   structBytes,
	}

	innerBytes, err := proto.Marshal(innerAny)
	if err != nil {
		t.Fatalf("failed to marshal inner Any: %v", err)
	}

	return &anypb.Any{
		TypeUrl: APIKeyStateTypeURL,
		Value:   innerBytes,
	}
}
