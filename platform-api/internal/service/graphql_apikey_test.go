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

// GraphQL API keys reuse the existing generic APIKeyService (see
// internal/handler/graphql_apikey.go's doc comment for why a dedicated
// GraphQLAPIKeyService was NOT introduced) — these tests pin that the shared
// service works correctly end-to-end when called with constants.GraphQLApi,
// the same way the eventgateway plugin already calls it with
// constants.WebSubApi/constants.WebBrokerApi.

import (
	"context"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// gqlKeyArtifactRepo is a minimal ArtifactRepository resolving one GraphQL API
// handle to a fixed UUID via GetAPIMetadataByHandleAndKind — the only method
// APIKeyService.CreateAPIKey/RevokeAPIKey actually call on it. The interface
// is embedded (mirroring guardStubArtifactRepo's approach in
// deployment_undeploy_guard_test.go) so every other method panics if
// accidentally invoked, rather than silently returning a zero value.
type gqlKeyArtifactRepo struct {
	repository.ArtifactRepository
	metadata *model.APIMetadata
}

func (g *gqlKeyArtifactRepo) GetAPIMetadataByHandleAndKind(handle, kind, orgUUID string) (*model.APIMetadata, error) {
	if handle == g.metadata.Handle && kind == constants.GraphQLApi {
		return g.metadata, nil
	}
	return nil, nil
}

// TestGraphQLAPIKey_CreateAndRevoke_Success exercises the shared APIKeyService
// with kind=constants.GraphQLApi end-to-end: create persists and broadcasts to
// every associated gateway, then revoke looks the key back up (ownership
// check passes since the same caller created it) and broadcasts a revocation.
func TestGraphQLAPIKey_CreateAndRevoke_Success(t *testing.T) {
	apiUUID := "gql-uuid-1"
	artifactRepo := &gqlKeyArtifactRepo{metadata: &model.APIMetadata{ID: apiUUID, Handle: "countries-graphql-api"}}
	apiRepo := dpKeyAPIRepo{} // GetAPIGatewaysWithDetails returns one gateway — see artifact_dp_apikey_test.go
	keyRepo := &dpCapturingAPIKeyRepo{}
	events := NewGatewayEventsService(dpNoopEventHub{}, newTestIdentityService(), newTestLogger())

	svc := NewAPIKeyService(apiRepo, artifactRepo, keyRepo, events, &noopAuditRepo{}, nil, newTestLogger())

	createReq := &api.CreateAPIKeyRequest{
		ApiKey:      "test-plaintext-key",
		DisplayName: "My GraphQL Key",
	}
	if err := svc.CreateAPIKey(context.Background(), "countries-graphql-api", constants.GraphQLApi, "org-1", "creator-uuid", createReq); err != nil {
		t.Fatalf("CreateAPIKey for GraphQL API = %v, want success", err)
	}
	if keyRepo.created == nil {
		t.Fatal("expected the API key to be persisted")
	}
	if keyRepo.created.ArtifactUUID != apiUUID {
		t.Errorf("persisted key ArtifactUUID = %q, want %q", keyRepo.created.ArtifactUUID, apiUUID)
	}
	keyName := keyRepo.created.Name

	if err := svc.RevokeAPIKey(context.Background(), "countries-graphql-api", constants.GraphQLApi, "org-1", keyName, "creator-uuid", false, false); err != nil {
		t.Fatalf("RevokeAPIKey for GraphQL API = %v, want success", err)
	}
}

// TestGraphQLAPIKey_Revoke_NotCreator_Forbidden verifies the shared ownership
// predicate (canManageAPIKey) is enforced for GraphQL API keys exactly as it
// is for REST/WebSub/WebBroker: a caller who isn't the key's creator, and
// doesn't hold ap:api_key:all:manage, is denied.
func TestGraphQLAPIKey_Revoke_NotCreator_Forbidden(t *testing.T) {
	apiUUID := "gql-uuid-1"
	artifactRepo := &gqlKeyArtifactRepo{metadata: &model.APIMetadata{ID: apiUUID, Handle: "countries-graphql-api"}}
	apiRepo := dpKeyAPIRepo{}
	keyRepo := &dpCapturingAPIKeyRepo{}
	events := NewGatewayEventsService(dpNoopEventHub{}, newTestIdentityService(), newTestLogger())

	svc := NewAPIKeyService(apiRepo, artifactRepo, keyRepo, events, &noopAuditRepo{}, nil, newTestLogger())

	createReq := &api.CreateAPIKeyRequest{ApiKey: "test-plaintext-key", DisplayName: "My GraphQL Key"}
	if err := svc.CreateAPIKey(context.Background(), "countries-graphql-api", constants.GraphQLApi, "org-1", "creator-uuid", createReq); err != nil {
		t.Fatalf("CreateAPIKey for GraphQL API = %v, want success", err)
	}
	keyName := keyRepo.created.Name

	err := svc.RevokeAPIKey(context.Background(), "countries-graphql-api", constants.GraphQLApi, "org-1", keyName, "someone-else", false, false)
	if err == nil {
		t.Fatal("expected an error revoking another user's GraphQL API key without ap:api_key:all:manage")
	}
	if code := graphQLCatalogCode(t, err); code != "REST_API_API_KEY_FORBIDDEN" {
		t.Errorf("expected REST_API_API_KEY_FORBIDDEN (the shared ownership-forbidden code every kind currently returns), got %s", code)
	}
}
