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
	"context"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// legacyNameArtifactRepo resolves any handle/kind/org to a fixed artifact UUID, so
// UpdateAPIKey/RevokeAPIKey can proceed straight to the API-key lookup.
type legacyNameArtifactRepo struct {
	repository.ArtifactRepository
	artifactUUID string
}

func (r legacyNameArtifactRepo) GetAPIMetadataByHandleAndKind(_, _, _ string) (*model.APIMetadata, error) {
	return &model.APIMetadata{ID: r.artifactUUID}, nil
}

// legacyNameAPIKeyRepo serves a single pre-existing key (whose Name may contain a
// legacy underscore, accepted before validateAPIKeyName was tightened to hyphen-only)
// and records whether Update/Revoke were reached without error.
type legacyNameAPIKeyRepo struct {
	repository.APIKeyRepository
	key       *model.APIKey
	updated   bool
	revoked   bool
	revokedBy string
}

func (r *legacyNameAPIKeyRepo) GetByArtifactAndName(_, name string) (*model.APIKey, error) {
	if r.key != nil && r.key.Name == name {
		return r.key, nil
	}
	return nil, nil
}

func (r *legacyNameAPIKeyRepo) Update(_ *model.APIKey) error {
	r.updated = true
	return nil
}

func (r *legacyNameAPIKeyRepo) Revoke(_, _, updatedBy string) error {
	r.revoked = true
	r.revokedBy = updatedBy
	return nil
}

// legacyNameAPIRepo reports no gateway deployments, which is enough for UpdateAPIKey
// to reach the point of persisting the update (RevokeAPIKey requires at least one
// deployment, so a single stub gateway is returned for that path).
type legacyNameAPIRepo struct {
	repository.APIRepository
	gateways []*model.APIGatewayWithDetails
}

func (r legacyNameAPIRepo) GetAPIGatewaysWithDetails(_, _ string) ([]*model.APIGatewayWithDetails, error) {
	return r.gateways, nil
}

// TestLegacyUnderscoreNamedKey_SurvivesUpdateAndRevoke proves that tightening
// validateAPIKeyName to hyphen-only (issue #3163 follow-up, CodeRabbit review on PR
// #3215) never re-validates an ALREADY-PERSISTED key's Name. A key named
// "legacy_key_v1" — accepted under the old, underscore-permissive regex — must keep
// updating and revoking indefinitely; only a caller's choice of a *new* id is subject
// to the tightened rule.
func TestLegacyUnderscoreNamedKey_SurvivesUpdateAndRevoke(t *testing.T) {
	const legacyName = "legacy_key_v1"
	existing := &model.APIKey{
		UUID:         "key-uuid-1",
		ArtifactUUID: "artifact-1",
		Name:         legacyName,
		CreatedBy:    "alice",
		Status:       constants.APIKeyStatusActive,
	}

	newSvc := func(keyRepo *legacyNameAPIKeyRepo, gateways []*model.APIGatewayWithDetails) *APIKeyService {
		hub := &capturingEventHub{}
		return &APIKeyService{
			artifactRepo:         legacyNameArtifactRepo{artifactUUID: "artifact-1"},
			apiRepo:              legacyNameAPIRepo{gateways: gateways},
			apiKeyRepo:           keyRepo,
			gatewayEventsService: NewGatewayEventsService(hub, newTestIdentityService(), newTestLogger()),
			hashingAlgorithms:    []string{defaultHashingAlgorithm},
			slogger:              newTestLogger(),
		}
	}

	t.Run("update succeeds without re-validating the legacy name", func(t *testing.T) {
		keyRepo := &legacyNameAPIKeyRepo{key: existing}
		svc := newSvc(keyRepo, []*model.APIGatewayWithDetails{{ID: "gw-1", Name: "gw-1"}})

		err := svc.UpdateAPIKey(context.Background(), "my-api", "RestApi", "org-1", legacyName, "alice",
			false, false, &api.UpdateAPIKeyRequest{ApiKey: "new-plain-key-value"})
		if err != nil {
			t.Fatalf("UpdateAPIKey() = %v, want success for a pre-existing underscore-named key", err)
		}
		if !keyRepo.updated {
			t.Fatal("UpdateAPIKey() did not reach the repository Update call")
		}
	})

	t.Run("revoke succeeds without re-validating the legacy name", func(t *testing.T) {
		keyRepo := &legacyNameAPIKeyRepo{key: existing}
		svc := newSvc(keyRepo, []*model.APIGatewayWithDetails{{ID: "gw-1", Name: "gw-1"}})

		err := svc.RevokeAPIKey(context.Background(), "my-api", "RestApi", "org-1", legacyName, "alice", false, false)
		if err != nil {
			t.Fatalf("RevokeAPIKey() = %v, want success for a pre-existing underscore-named key", err)
		}
		if !keyRepo.revoked {
			t.Fatal("RevokeAPIKey() did not reach the repository Revoke call")
		}
	})
}

// TestLegacyUnderscoreNamedKey_SurvivesBackfill proves BackfillAPIKeysToGateway (the
// deploy-time resync path) never validates an existing key's Name either — a legacy
// underscore-containing key is broadcast to a newly-associated gateway exactly like
// any other active key.
func TestLegacyUnderscoreNamedKey_SurvivesBackfill(t *testing.T) {
	const legacyName = "legacy_key_v1"
	repo := &stubBackfillAPIKeyRepo{keys: []*model.APIKey{
		{UUID: "key-uuid-1", ArtifactUUID: "artifact-1", Name: legacyName, Status: constants.APIKeyStatusActive},
	}}
	hub := &capturingEventHub{}
	events := NewGatewayEventsService(hub, newTestIdentityService(), newTestLogger())

	BackfillAPIKeysToGateway(repo, nil, events, newTestLogger(), "artifact-1", "gw-X", "")

	if len(hub.published) != 1 {
		t.Fatalf("expected 1 broadcast event for the legacy underscore-named key, got %d", len(hub.published))
	}
	if got := decodeKeyName(t, hub.published[0]); got != legacyName {
		t.Fatalf("backfilled key name = %q, want %q", got, legacyName)
	}
}
