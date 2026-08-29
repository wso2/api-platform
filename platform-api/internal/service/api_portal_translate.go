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
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// APIPortal DTO <-> model translation, shared between the HTTP handler and the
// pdk-facing wrappers on APIPortalService.

// derefAPIPortalMetadata converts the generated Metadata type (a map alias)
// into a plain map[string]interface{} the service works in. Nil in → nil out.
func derefAPIPortalMetadata(m *api.ApiPortalMetadata) map[string]interface{} {
	if m == nil {
		return nil
	}
	return map[string]interface{}(*m)
}

// authConfigStructToMap flattens the generated ApiPortalAuthConfig struct into
// the map shape service-layer validation and encryption operate on. Nil pointer
// fields are dropped so validation sees "missing" rather than "present but
// empty".
func authConfigStructToMap(c *api.ApiPortalAuthConfig) map[string]interface{} {
	if c == nil {
		return nil
	}
	out := map[string]interface{}{}
	if c.StsTokenUrl != nil {
		out[constants.APIPortalAuthConfigKeySTSTokenURL] = *c.StsTokenUrl
	}
	if c.ClientId != nil {
		out[constants.APIPortalAuthConfigKeyClientID] = *c.ClientId
	}
	if c.ClientSecret != nil {
		out[constants.APIPortalAuthConfigKeyClientSecret] = *c.ClientSecret
	}
	return out
}

// stripSensitiveAuthConfig removes keys that carry secret material. Called
// before authConfig leaves the server, alongside the OAS `writeOnly: true`
// marker on ClientSecret — even if the storage-encrypt step is ever skipped,
// the response strip guarantees secrets never appear on the wire.
func stripSensitiveAuthConfig(cfg map[string]interface{}) map[string]interface{} {
	if cfg == nil {
		return nil
	}
	out := make(map[string]interface{}, len(cfg))
	for k, v := range cfg {
		out[k] = v
	}
	for _, key := range constants.APIPortalAuthConfigSensitiveKeys {
		delete(out, key)
	}
	return out
}

// mapToAuthConfigStruct rebuilds the generated struct from the stored map for
// response serialization. Sensitive keys are stripped first, so the generated
// ClientSecret pointer stays nil (and — since it's marked omitempty — won't
// appear in the JSON output).
func mapToAuthConfigStruct(m map[string]interface{}) *api.ApiPortalAuthConfig {
	stripped := stripSensitiveAuthConfig(m)
	if stripped == nil {
		return nil
	}
	c := &api.ApiPortalAuthConfig{}
	if v, ok := stripped[constants.APIPortalAuthConfigKeySTSTokenURL].(string); ok && v != "" {
		s := v
		c.StsTokenUrl = &s
	}
	if v, ok := stripped[constants.APIPortalAuthConfigKeyClientID].(string); ok && v != "" {
		s := v
		c.ClientId = &s
	}
	// ClientSecret is intentionally never populated on the response side.
	return c
}

// ModelToAPIPortalResponse converts an internal model.APIPortal into the
// api-generated ApiPortalResponse. Exported so the HTTP handler can serialize
// what the service returns.
func ModelToAPIPortalResponse(p *model.APIPortal) *api.ApiPortalResponse {
	if p == nil {
		return nil
	}
	id := p.Handle
	handle := p.Handle
	createdAt := p.CreatedAt
	updatedAt := p.UpdatedAt

	resp := &api.ApiPortalResponse{
		Id:        &id,
		Handle:    &handle,
		Name:      p.Name,
		Url:       p.URL,
		AuthType:  api.ApiPortalResponseAuthType(p.AuthType),
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}
	if p.Description != "" {
		desc := p.Description
		resp.Description = &desc
	}
	if p.AuthConfig != nil {
		resp.AuthConfig = mapToAuthConfigStruct(p.AuthConfig)
	}
	if p.Metadata != nil {
		m := api.ApiPortalMetadata(p.Metadata)
		resp.Metadata = &m
	}
	return resp
}

// modelToAPIPortalListItem projects a model.APIPortal onto the list-response
// item type (excludes authConfig and metadata by design).
func modelToAPIPortalListItem(p *model.APIPortal) api.ApiPortalListItem {
	item := api.ApiPortalListItem{
		Id:        p.Handle,
		Handle:    p.Handle,
		Name:      p.Name,
		Url:       p.URL,
		AuthType:  api.ApiPortalListItemAuthType(p.AuthType),
		CreatedAt: p.CreatedAt,
	}
	if p.Description != "" {
		desc := p.Description
		item.Description = &desc
	}
	return item
}

// buildAPIPortalListResponse wraps the raw list + pagination info in the
// api-generated ApiPortalListResponse envelope.
func buildAPIPortalListResponse(list []*model.APIPortal, pag PaginationInfo) *api.ApiPortalListResponse {
	out := &api.ApiPortalListResponse{
		Count: len(list),
		List:  make([]api.ApiPortalListItem, 0, len(list)),
		Pagination: api.Pagination{
			Total:  pag.Total,
			Offset: pag.Offset,
			Limit:  pag.Limit,
		},
	}
	for _, p := range list {
		out.List = append(out.List, modelToAPIPortalListItem(p))
	}
	return out
}
