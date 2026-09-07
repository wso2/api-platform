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
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// TestCanManageAPIKey pins the ownership rule shared by every API key CRUD path:
// the creator always passes, ap:api_key:all:manage (keyAdmin) passes for any key,
// and an unidentified caller never passes — including against a key with no
// recorded creator, which must not be treated as "matching" an empty caller id.
func TestCanManageAPIKey(t *testing.T) {
	tests := []struct {
		name      string
		createdBy string
		caller    string
		keyAdmin  bool
		want      bool
	}{
		{name: "creator manages own key", createdBy: "alice", caller: "alice", want: true},
		{name: "non-creator without scope is denied", createdBy: "alice", caller: "bob", want: false},
		{name: "non-creator with scope is allowed", createdBy: "alice", caller: "bob", keyAdmin: true, want: true},
		{name: "creator with scope is allowed", createdBy: "alice", caller: "alice", keyAdmin: true, want: true},
		{name: "empty caller is denied", createdBy: "alice", caller: "", want: false},
		{name: "empty caller against empty creator is denied", createdBy: "", caller: "", want: false},
		{name: "empty caller with scope is allowed", createdBy: "alice", caller: "", keyAdmin: true, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := canManageAPIKey(tc.createdBy, tc.caller, tc.keyAdmin); got != tc.want {
				t.Errorf("canManageAPIKey(%q, %q, %v) = %v, want %v",
					tc.createdBy, tc.caller, tc.keyAdmin, got, tc.want)
			}
		})
	}
}

// noCollisionAPIKeyRepo reports no name collision, so resolveUniqueKeyName's retry
// loop is never exercised — only the validation gate is under test.
type noCollisionAPIKeyRepo struct {
	repository.APIKeyRepository
}

func (noCollisionAPIKeyRepo) GetByArtifactAndName(string, string) (*model.APIKey, error) {
	return nil, nil
}

// TestResolveUniqueKeyName_RejectsShortId pins issue #3163 for the REST API key path:
// a caller-supplied id under 3 characters must be rejected, while falling back to
// displayName-derived generation is unaffected.
func TestResolveUniqueKeyName_RejectsShortId(t *testing.T) {
	svc := &APIKeyService{apiKeyRepo: noCollisionAPIKeyRepo{}}

	shortID := "ab"
	_, err := svc.resolveUniqueKeyName("artifact-1", &api.CreateAPIKeyRequest{Id: &shortID}, "my-api")
	if err == nil {
		t.Fatal("resolveUniqueKeyName() = nil error, want rejection for a 2-char id")
	}
	assertBadRequest(t, err)

	got, err := svc.resolveUniqueKeyName("artifact-1", &api.CreateAPIKeyRequest{DisplayName: "My Key"}, "my-api")
	if err != nil {
		t.Fatalf("resolveUniqueKeyName() = %v, want success", err)
	}
	if got != "my-key" {
		t.Fatalf("resolveUniqueKeyName() = %q, want %q", got, "my-key")
	}
}
