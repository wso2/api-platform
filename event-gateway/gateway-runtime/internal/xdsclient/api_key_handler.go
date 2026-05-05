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
	"fmt"
	"log/slog"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	commonapikey "github.com/wso2/api-platform/common/apikey"
	pkgapikey "github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/pkg/apikey"
)

// APIKeyStateResource is an alias for the shared type from policy-engine/pkg/apikey.
type APIKeyStateResource = pkgapikey.APIKeyStateResource

// APIKeyData is an alias for the shared type from policy-engine/pkg/apikey.
type APIKeyData = pkgapikey.APIKeyData

// APIKeyStateHandler keeps the shared API key validation store in sync with xDS snapshots.
// All proto decoding is delegated to policy-engine/pkg/apikey so logic is defined once.
type APIKeyStateHandler struct {
	store *commonapikey.APIkeyStore
}

// NewAPIKeyStateHandler creates a new API key state handler backed by store.
func NewAPIKeyStateHandler(store *commonapikey.APIkeyStore) *APIKeyStateHandler {
	return &APIKeyStateHandler{store: store}
}

// HandleResources processes an APIKeyState xDS update and replaces the store contents.
func (h *APIKeyStateHandler) HandleResources(ctx context.Context, resources []*discoveryv3.Resource, version string) error {
	var allKeys []pkgapikey.APIKeyData

	for _, res := range resources {
		if res == nil || res.Resource == nil {
			continue
		}

		state, err := pkgapikey.DecodeAPIKeyStateResource(res.Resource)
		if err != nil {
			return fmt.Errorf("failed to decode API key state resource: %w", err)
		}

		allKeys = append(allKeys, state.APIKeys...)
	}

	combined := &pkgapikey.APIKeyStateResource{APIKeys: allKeys}
	if err := pkgapikey.ApplyToStore(ctx, combined, h.store); err != nil {
		return fmt.Errorf("failed to apply API key state to store: %w", err)
	}

	slog.InfoContext(ctx, "Updated API key store from xDS",
		"version", version,
		"resources", len(resources),
		"api_keys", len(allKeys))

	return nil
}
