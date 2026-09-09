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
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// APIPortal DTO <-> model translation, shared between the HTTP handler and the
// pdk-facing wrappers on APIPortalService.

// derefAPIPortalMetadata converts the generated Metadata type (a map alias)
// into a plain map[string]interface{} the service works in. Nil in -> nil out.
func derefAPIPortalMetadata(m *api.ApiPortalMetadata) map[string]interface{} {
	if m == nil {
		return nil
	}
	return map[string]interface{}(*m)
}

// ModelToAPIPortalResponse converts an internal model.APIPortal into the
// api-generated ApiPortalResponse. Exported so the HTTP handler can serialize
// what the service returns. The InternalAuthKey field is NEVER surfaced,
// the only path for a client to see the shared key is the write-only field
// on Create/Update requests, and that value is not stored in a form that can
// be re-read.
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
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}
	if p.Description != "" {
		desc := p.Description
		resp.Description = &desc
	}
	if p.Metadata != nil {
		m := api.ApiPortalMetadata(p.Metadata)
		resp.Metadata = &m
	}
	return resp
}

// modelToAPIPortalListItem projects a model.APIPortal onto the list-response
// item type (excludes metadata by design, and never carries the shared key).
func modelToAPIPortalListItem(p *model.APIPortal) api.ApiPortalListItem {
	item := api.ApiPortalListItem{
		Id:        p.Handle,
		Handle:    p.Handle,
		Name:      p.Name,
		Url:       p.URL,
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
