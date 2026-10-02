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
	"errors"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// The HTTP-level contract is covered end to end in
// internal/handler/agent_proxy_apikey_integration_test.go. These cover the
// fail-closed listing branches.

const (
	agentKeyTestOrg   = "org-1"
	agentKeyTestProxy = "weather-agent"
	agentKeyTestUUID  = "agent-uuid-1"
)

type stubAgentKeyProxyRepo struct {
	repository.AgentProxyRepository
}

func (stubAgentKeyProxyRepo) GetByHandle(handle, orgUUID string) (*model.AgentProxy, error) {
	if handle == agentKeyTestProxy && orgUUID == agentKeyTestOrg {
		return &model.AgentProxy{UUID: agentKeyTestUUID, Handle: agentKeyTestProxy}, nil
	}
	return nil, nil
}

type listOnlyAgentKeyRepo struct {
	repository.APIKeyRepository
	keys []*model.APIKey
}

func (r *listOnlyAgentKeyRepo) ListByArtifact(artifactUUID string) ([]*model.APIKey, error) {
	var out []*model.APIKey
	for _, k := range r.keys {
		if k.ArtifactUUID == artifactUUID {
			out = append(out, k)
		}
	}
	return out, nil
}

func agentKeyOwnedBy(name, user string) *model.APIKey {
	return &model.APIKey{
		UUID: "uuid-" + name, ArtifactUUID: agentKeyTestUUID, Name: name, DisplayName: name,
		Status: constants.APIKeyStatusActive, CreatedBy: user,
		APIKeyHashes: `{"sha256": "abc"}`, MaskedAPIKey: "***abcde",
	}
}

// An empty caller identity is never treated as a key's creator — not even for a
// key whose recorded creator is itself empty (GO-AUTH-020).
func TestAgentProxyAPIKeyList_EmptyCallerSeesNothing(t *testing.T) {
	repo := &listOnlyAgentKeyRepo{keys: []*model.APIKey{agentKeyOwnedBy("a-key", "alice"), agentKeyOwnedBy("orphan", "")}}
	svc := NewAgentProxyAPIKeyService(stubAgentKeyProxyRepo{}, repo, newTestIdentityService())

	list, err := svc.List(context.Background(), agentKeyTestOrg, agentKeyTestProxy, "", false, 20, 0)
	if err != nil || len(list.List) != 0 || list.Pagination.Total != 0 {
		t.Fatalf("list by an empty caller = %+v, %v; want nothing", list, err)
	}
}

func TestAgentProxyAPIKeyList_UnknownAgentProxyIsNotFound(t *testing.T) {
	svc := NewAgentProxyAPIKeyService(stubAgentKeyProxyRepo{}, &listOnlyAgentKeyRepo{}, newTestIdentityService())

	for _, org := range []string{agentKeyTestOrg, "another-org"} {
		handle := "no-such-agent"
		if org != agentKeyTestOrg {
			handle = agentKeyTestProxy // right handle, wrong tenant
		}
		_, err := svc.List(context.Background(), org, handle, "alice", true, 20, 0)
		var appErr *apperror.Error
		if !errors.As(err, &appErr) || appErr.Code != apperror.CodeAgentProxyNotFound {
			t.Fatalf("org %s handle %s: err = %v, want AGENT_PROXY_NOT_FOUND", org, handle, err)
		}
	}
}
