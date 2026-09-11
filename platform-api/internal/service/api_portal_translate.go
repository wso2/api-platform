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

// APIPortal DTO <-> model translation.

// derefAPIPortalMetadata converts the generated Metadata alias into a plain map; nil in -> nil out.
func derefAPIPortalMetadata(m *api.ApiPortalMetadata) map[string]interface{} {
	if m == nil {
		return nil
	}
	return map[string]interface{}(*m)
}

// ModelToAPIPortalResponse converts a model.APIPortal into the wire response; the shared key is never surfaced.
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

// modelToAPIPortalListItem projects a model.APIPortal onto the list-response item (metadata and shared key are excluded).
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

// buildAPIPortalListResponse wraps the page + pagination info in the wire envelope.
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
