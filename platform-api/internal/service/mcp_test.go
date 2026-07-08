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
	"errors"
	"log/slog"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// mockMCPProxyRepository is a minimal mock of repository.MCPProxyRepository.
type mockMCPProxyRepository struct {
	repository.MCPProxyRepository
	getByHandleResult *model.MCPProxy
}

func (m *mockMCPProxyRepository) GetByHandle(handle, orgUUID string) (*model.MCPProxy, error) {
	return m.getByHandleResult, nil
}

func validMCPProxyRequest() *api.MCPProxy {
	url := "https://example.com/mcp"
	return &api.MCPProxy{
		Id:          strPointer("mcp-proxy-1"),
		DisplayName: "Test MCP Proxy",
		Version:     "v1.0",
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{Url: &url},
		},
	}
}

func TestMCPProxyServiceCreateRejectsInvalidPolicyVersion(t *testing.T) {
	service := NewMCPProxyService(&mockMCPProxyRepository{}, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	request := validMCPProxyRequest()
	request.Policies = &[]api.Policy{{Name: "api-key-auth", Version: "v1.0.0"}}

	_, err := service.Create("org-1", "alice", request)
	if !errors.Is(err, constants.ErrInvalidPolicyVersion) {
		t.Fatalf("expected ErrInvalidPolicyVersion, got: %v", err)
	}
}

func TestMCPProxyServiceUpdateRejectsInvalidPolicyVersion(t *testing.T) {
	repo := &mockMCPProxyRepository{getByHandleResult: &model.MCPProxy{Handle: "mcp-proxy-1"}}
	service := NewMCPProxyService(repo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	request := validMCPProxyRequest()
	request.Policies = &[]api.Policy{{Name: "api-key-auth", Version: "1"}}

	_, err := service.Update("org-1", "mcp-proxy-1", "alice", request)
	if !errors.Is(err, constants.ErrInvalidPolicyVersion) {
		t.Fatalf("expected ErrInvalidPolicyVersion, got: %v", err)
	}
}
