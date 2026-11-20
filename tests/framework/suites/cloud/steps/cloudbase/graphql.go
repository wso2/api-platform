/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudbase

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
)

const (
	KindGraphQLAPI         = "GraphQLApi"
	GraphQLAPIsCollection  = "/graphql-apis"
	GraphQLAPILabel        = "synthetic GraphQL API"
	KeyGraphQLDeploymentID = "cloudGraphQLDeploymentID"
	KeyGraphQLDefinition   = "cloudGraphQLDefinition"
	keyGraphQLBuildIDs     = "cloudGraphQLBuildIDs"
	keyGraphQLBuildsOwner  = "cloudGraphQLBuildsOwner"
	// graphQLBackendEnv overrides the backend the synthetic GraphQL APIs call.
	graphQLBackendEnv = "CLOUD_IT_GRAPHQL_BACKEND_URL"
	// defaultGraphQLBackend is the testbench GraphQL backend, which answers with the body it received.
	defaultGraphQLBackend = "https://development-wso2cloud.gateway.dev.cloud.wso2.com/apip-testbench-mocks/graphql-backend"
	GraphQLSchema         = "type Query { hello: String }"
)

// GraphQLCreateKeys are the keys of a GraphQL API creation request a values table may set.
var GraphQLCreateKeys = map[string]bool{"policies": true, "sdl": true}

// NewGraphQLDefinition returns the metadata of the default synthetic GraphQL API: an inline schema
// and the testbench GraphQL backend as its upstream.
func NewGraphQLDefinition(ctx context.Context, id string) (map[string]any, error) {
	projectID, err := RequiredValue(ctx, KeyProjectID)
	if err != nil {
		return nil, err
	}
	backend, err := TestbenchURL(graphQLBackendEnv, defaultGraphQLBackend)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id": id, "displayName": PublicationTitle + id, "description": SyntheticDescription,
		"context": "/" + id + "/v1", "version": "1.0.0", "projectId": projectID,
		"upstream":     map[string]any{"main": map[string]string{"url": backend}},
		"schemaSource": "inline", "sdl": GraphQLSchema,
	}, nil
}

// ApplyGraphQLOverrides sets the fields a values table supplies, accepting only the allowed ones.
func ApplyGraphQLOverrides(ctx context.Context, definition map[string]any, overrides map[string]string, allowed map[string]bool) error {
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		if !allowed[key] {
			return fmt.Errorf("unsupported GraphQL API value %q", key)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value, err := ExpandCloudValue(ctx, overrides[key])
		if err != nil {
			return err
		}
		switch key {
		case "policies":
			var parsed any
			if err := json.Unmarshal([]byte(value), &parsed); err != nil {
				return fmt.Errorf("GraphQL API value %q is not valid JSON: %w", key, err)
			}
			definition[key] = parsed
		case "upstream":
			definition[key] = map[string]any{"main": map[string]string{"url": value}}
		default:
			definition[key] = value
		}
	}
	// The default inline schema goes with the inline source.
	if _, given := overrides["sdl"]; !given && definition["schemaSource"] != "inline" {
		delete(definition, "sdl")
	}
	return nil
}

// GraphQLForm is the multipart form the GraphQL API routes take: the metadata as JSON.
func GraphQLForm(definition map[string]any) ([]byte, string, error) {
	metadata, err := json.Marshal(definition)
	if err != nil {
		return nil, "", err
	}
	return MultipartFields([][2]string{{"metadata", string(metadata)}})
}

// CreateGraphQLAPI creates the default synthetic GraphQL API. The control plane accepts a GraphQL
// API only as a multipart form whose metadata part carries the definition.
func (s *Steps) CreateGraphQLAPI(ctx context.Context) error {
	return s.CreateGraphQLAPIAs(ctx, nil)
}

// CreateGraphQLAPIAs creates the synthetic GraphQL API with the overrides applied and keeps its
// metadata. It is registered for cleanup before the request.
func (s *Steps) CreateGraphQLAPIAs(ctx context.Context, overrides map[string]string) error {
	if err := GenerateResourceAndStore(ctx, "syn-graphql", KeyGraphQLAPIID); err != nil {
		return err
	}
	id, err := RequiredValue(ctx, KeyGraphQLAPIID)
	if err != nil {
		return err
	}
	definition, err := NewGraphQLDefinition(ctx, id)
	if err != nil {
		return err
	}
	if err := ApplyGraphQLOverrides(ctx, definition, overrides, GraphQLCreateKeys); err != nil {
		return err
	}
	resp, err := s.SendGraphQLCreation(ctx, id, definition)
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return ResponseError(resp)
	}
	return StoreDefinition(ctx, GraphQLAPI, definition)
}

// SendGraphQLCreation registers a GraphQL API for cleanup, sends its creation and publishes the
// answer. The API stays registered whatever the answer, since a creation can fail after the API is
// stored; cleanup treats an absent API as deleted.
func (s *Steps) SendGraphQLCreation(ctx context.Context, id string, definition map[string]any) (*httpx.Response, error) {
	body, contentType, err := GraphQLForm(definition)
	if err != nil {
		return nil, err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindGraphQLAPI, ID: id, Actor: ResourceActor, Description: "GraphQL API " + id,
	}); err != nil {
		return nil, fmt.Errorf("registering GraphQL API %q for cleanup: %w", id, err)
	}
	return s.DoAuthenticatedContent(ctx, http.MethodPost, GraphQLAPIsCollection, contentType, body)
}
