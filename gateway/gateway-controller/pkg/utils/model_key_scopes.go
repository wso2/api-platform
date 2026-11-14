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

package utils

import (
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/apikeyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/modelgateway/inbound"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// ModelKeyScopes resolves where a Model's API keys are sent.
//
// Every chain for a Model lives in its endpoints' runtime configs, so api-key-auth
// on a Model chain validates against the endpoint's id. A Model's key is therefore
// sent once under each endpoint serving it, scoped to that Model's handle — the
// operation the dispatch resolver binds — and to that endpoint's catalog routes,
// where the catalog then lists only that Model. The database keeps the Model as
// the key's owner.
//
// A key whose owner is any other kind is sent as it always was.
func ModelKeyScopes(store *storage.ConfigStore) apikeyxds.ScopeResolver {
	return func(artifactUUID string) ([]apikeyxds.KeyScope, bool) {
		if store == nil {
			return nil, false
		}
		cfg, err := store.Get(artifactUUID)
		if err != nil || cfg == nil || cfg.Kind != models.KindModel {
			return nil, false
		}
		model, ok := cfg.SourceConfiguration.(api.ModelConfiguration)
		if !ok {
			return nil, true
		}
		var scopes []apikeyxds.KeyScope
		for _, handle := range models.ModelEndpointHandles(model.Spec) {
			endpoint, err := store.GetByKindAndHandle(models.KindModelEndpoint, handle)
			if err != nil || endpoint == nil {
				continue
			}
			ep, ok := endpoint.SourceConfiguration.(api.ModelEndpointConfiguration)
			if !ok {
				continue
			}
			profile, ok := inbound.Lookup(models.EffectiveInboundTemplate(ep.Spec))
			if !ok {
				continue
			}
			operations := append([]string{cfg.Handle}, profile.CatalogScopes()...)
			scopes = append(scopes, apikeyxds.KeyScope{
				APIId:      endpoint.UUID,
				Operations: operations,
				Owner:      cfg.Handle,
			})
		}
		return scopes, true
	}
}
