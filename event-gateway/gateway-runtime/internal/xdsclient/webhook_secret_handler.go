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
	"fmt"
	"log/slog"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	commonwebhooksecret "github.com/wso2/api-platform/common/webhooksecret"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// WebhookSecretStateHandler keeps the shared webhook secret store in sync with xDS snapshots.
// It mirrors the APIKeyStateHandler pattern.
type WebhookSecretStateHandler struct {
	store *commonwebhooksecret.WebhookSecretStore
}

// NewWebhookSecretStateHandler creates a new handler backed by store.
func NewWebhookSecretStateHandler(store *commonwebhooksecret.WebhookSecretStore) *WebhookSecretStateHandler {
	return &WebhookSecretStateHandler{store: store}
}

// HandleResources processes a WebhookSecretState xDS update and atomically replaces the store.
// The resource is encoded as: outer Any → structpb.Struct → JSON map.
func (h *WebhookSecretStateHandler) HandleResources(ctx context.Context, resources []*discoveryv3.Resource, version string) error {
	combined := make(map[string]map[string]string)

	for _, res := range resources {
		if res == nil || res.Resource == nil {
			continue
		}

		s := &structpb.Struct{}
		if err := proto.Unmarshal(res.Resource.Value, s); err != nil {
			return fmt.Errorf("webhooksecret handler: failed to unmarshal Struct: %w", err)
		}

		jsonBytes, err := json.Marshal(s.AsMap())
		if err != nil {
			return fmt.Errorf("webhooksecret handler: failed to marshal struct to JSON: %w", err)
		}

		var payload struct {
			Secrets map[string]map[string]string `json:"secrets"`
		}
		if err := json.Unmarshal(jsonBytes, &payload); err != nil {
			return fmt.Errorf("webhooksecret handler: failed to unmarshal payload: %w", err)
		}

		for apiID, secrets := range payload.Secrets {
			if combined[apiID] == nil {
				combined[apiID] = make(map[string]string)
			}
			for name, plaintext := range secrets {
				combined[apiID][name] = plaintext
			}
		}
	}

	if err := h.store.ReplaceAll(combined); err != nil {
		return fmt.Errorf("webhooksecret handler: failed to apply to store: %w", err)
	}

	slog.InfoContext(ctx, "Updated webhook secret store from xDS",
		"version", version,
		"api_count", len(combined))

	return nil
}
