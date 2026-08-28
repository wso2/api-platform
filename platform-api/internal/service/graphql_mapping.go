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
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// mapGraphQLAPIModelToAPI converts a model.GraphQLAPI to api.GraphQLAPI,
// including the full SDL (used for Get/Create/Update responses, never for
// list responses — see mapGraphQLAPIModelToListItem). Upstream/policy
// conversion reuses the same generic helpers LLM/MCP already share
// (mapUpstreamAPIToModel/mapUpstreamModelToAPI in llm.go,
// mapMCPPoliciesAPIToModel/mapMCPPoliciesModelToAPI in mcp.go) since
// GraphQL reuses model.UpstreamConfig/model.Policy unmodified.
func mapGraphQLAPIModelToAPI(m *model.GraphQLAPI) *api.GraphQLAPI {
	if m == nil {
		return nil
	}

	desc := m.Description
	createdBy := m.CreatedBy
	kind := constants.GraphQLApi
	sdl := m.Configuration.SDL

	var introspectionMode *api.GraphQLIntrospectionMode
	if m.Configuration.IntrospectionMode != "" {
		im := api.GraphQLIntrospectionMode(m.Configuration.IntrospectionMode)
		introspectionMode = &im
	}

	var subscriptionPlans *[]string
	if len(m.Configuration.SubscriptionPlans) > 0 {
		subscriptionPlans = &m.Configuration.SubscriptionPlans
	}

	upstream := mapUpstreamModelToAPI(&m.Configuration.Upstream)

	return &api.GraphQLAPI{
		Id:                utils.StringPtrIfNotEmpty(m.Handle),
		DisplayName:       m.Name,
		Version:           m.Version,
		Context:           utils.ValueOrEmpty(m.Configuration.Context),
		ProjectId:         m.ProjectID,
		Description:       &desc,
		CreatedBy:         &createdBy,
		Kind:              &kind,
		Sdl:               &sdl,
		IntrospectionMode: introspectionMode,
		Upstream:          upstream,
		Policies:          mapMCPPoliciesModelToAPI(m.Configuration.Policies),
		SubscriptionPlans: subscriptionPlans,
		ReadOnly:          utils.BoolPtr(m.Origin == constants.OriginDP),
		CreatedAt:         utils.TimePtr(m.CreatedAt),
		UpdatedAt:         utils.TimePtr(m.UpdatedAt),
		UpdatedBy:         utils.StringPtrIfNotEmpty(m.UpdatedBy),
	}
}

// mapGraphQLAPIModelToDetail converts a model.GraphQLAPI to
// api.GraphQLAPIDetail — the shape returned by GET
// /graphql-apis/{graphqlApiId}, identical to mapGraphQLAPIModelToAPI's output
// except sdl is omitted (fetch it via GET /graphql-apis/{graphqlApiId}/sdl
// instead).
func mapGraphQLAPIModelToDetail(m *model.GraphQLAPI) *api.GraphQLAPIDetail {
	if m == nil {
		return nil
	}

	desc := m.Description
	createdBy := m.CreatedBy
	kind := constants.GraphQLApi

	var introspectionMode *api.GraphQLIntrospectionMode
	if m.Configuration.IntrospectionMode != "" {
		im := api.GraphQLIntrospectionMode(m.Configuration.IntrospectionMode)
		introspectionMode = &im
	}

	var subscriptionPlans *[]string
	if len(m.Configuration.SubscriptionPlans) > 0 {
		subscriptionPlans = &m.Configuration.SubscriptionPlans
	}

	upstream := mapUpstreamModelToAPI(&m.Configuration.Upstream)

	return &api.GraphQLAPIDetail{
		Id:                utils.StringPtrIfNotEmpty(m.Handle),
		DisplayName:       m.Name,
		Version:           m.Version,
		Context:           utils.ValueOrEmpty(m.Configuration.Context),
		ProjectId:         m.ProjectID,
		Description:       &desc,
		CreatedBy:         &createdBy,
		Kind:              &kind,
		IntrospectionMode: introspectionMode,
		Upstream:          upstream,
		Policies:          mapMCPPoliciesModelToAPI(m.Configuration.Policies),
		SubscriptionPlans: subscriptionPlans,
		ReadOnly:          utils.BoolPtr(m.Origin == constants.OriginDP),
		CreatedAt:         utils.TimePtr(m.CreatedAt),
		UpdatedAt:         utils.TimePtr(m.UpdatedAt),
		UpdatedBy:         utils.StringPtrIfNotEmpty(m.UpdatedBy),
	}
}

// mapGraphQLAPIModelToListItem converts a model.GraphQLAPI to
// api.GraphQLAPIListItem. sdl is deliberately omitted (see
// GraphQLAPIListResponse's schema description in resources/openapi.yaml).
func mapGraphQLAPIModelToListItem(m *model.GraphQLAPI) *api.GraphQLAPIListItem {
	if m == nil {
		return nil
	}

	var introspectionMode *api.GraphQLIntrospectionMode
	if m.Configuration.IntrospectionMode != "" {
		im := api.GraphQLIntrospectionMode(m.Configuration.IntrospectionMode)
		introspectionMode = &im
	}

	upstream := mapUpstreamModelToAPI(&m.Configuration.Upstream)

	return &api.GraphQLAPIListItem{
		Id:                utils.StringPtrIfNotEmpty(m.Handle),
		DisplayName:       m.Name,
		Version:           m.Version,
		Context:           utils.ValueOrEmpty(m.Configuration.Context),
		ProjectId:         m.ProjectID,
		Description:       utils.StringPtrIfNotEmpty(m.Description),
		IntrospectionMode: introspectionMode,
		Upstream:          &upstream,
		ReadOnly:          utils.BoolPtr(m.Origin == constants.OriginDP),
		CreatedBy:         utils.StringPtrIfNotEmpty(m.CreatedBy),
		CreatedAt:         utils.TimePtr(m.CreatedAt),
		UpdatedAt:         utils.TimePtr(m.UpdatedAt),
	}
}
