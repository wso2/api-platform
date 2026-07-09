/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/pagination"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// Pagination bounds mirror the limit-Q / offset-Q parameter contract in the
// OpenAPI spec (resources/openapi.yaml): limit is 1..100 with a default of 20,
// offset is >= 0 with a default of 0.
const (
	defaultPageLimit  = 20
	maxPageLimit      = 100
	minPageLimit      = 1
	defaultPageOffset = 0
)

// parsePagination extracts and normalizes the `limit` and `offset` query
// parameters according to the spec contract. A missing or malformed value falls
// back to the documented default; a well-formed but out-of-range value is
// clamped into the documented window on either side. Neither errors, so a client
// can never push the API outside its safe window.
func parsePagination(r *http.Request) (limit, offset int) {
	limit = defaultPageLimit
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			limit = min(max(parsed, minPageLimit), maxPageLimit)
		}
	}

	offset = defaultPageOffset
	if v := strings.TrimSpace(r.URL.Query().Get("offset")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	return limit, offset
}

// parseListOptions extends parsePagination with the standard sorting and search
// query parameters (sortBy, sortOrder, query) documented for collection GETs.
//
// Only sortOrder is normalized here (to "asc"/"desc", defaulting to "desc");
// sortBy is passed through verbatim and resolved against a per-table allowlist at
// the repository layer, so an unrecognized field silently falls back to the
// default sort rather than erroring — matching how out-of-range limit/offset are
// clamped rather than rejected. `query` is a trimmed substring matched
// case-insensitively against the resource handle (id).
func parseListOptions(r *http.Request) repository.ListOptions {
	limit, offset := parsePagination(r)

	sortOrder := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sortOrder")))
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	return repository.ListOptions{
		Limit:     limit,
		Offset:    offset,
		SortBy:    strings.TrimSpace(r.URL.Query().Get("sortBy")),
		SortOrder: sortOrder,
		Search:    strings.TrimSpace(r.URL.Query().Get("query")),
	}
}

// pageWindow windows a fully-materialized, bounded collection in the handler
// before responding.
func pageWindow[T any](items []T, limit, offset int) []T {
	return pagination.Window(items, limit, offset)
}

// paginateDeploymentList windows a fully-materialized deployment list response
// and stamps its pagination metadata. Deployments are a small, bounded set per
// artifact, so the total reflects the full list and the window is applied in
// memory.
func paginateDeploymentList(resp *api.DeploymentListResponse, limit, offset int) {
	if resp == nil {
		return
	}
	total := len(resp.List)
	resp.List = pageWindow(resp.List, limit, offset)
	resp.Count = len(resp.List)
	resp.Pagination = api.Pagination{Total: total, Offset: offset, Limit: limit}
}
