/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package service

import (
	"fmt"
	"strings"
	"time"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
)

// isSQLiteUniqueConstraint reports whether err is a SQLite unique-constraint violation.
func isSQLiteUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// ensureOriginMutable returns ErrArtifactReadOnly when the artifact originated from
// a data-plane gateway. DP artifacts are read-only in the control plane.
func ensureOriginMutable(origin string) error {
	if origin == constants.OriginDP {
		return constants.ErrArtifactReadOnly
	}
	return nil
}

// ensureArtifactMutableByUUID looks the artifact up by UUID and returns
// ErrArtifactReadOnly when it is data-plane-originated.
func ensureArtifactMutableByUUID(repo repository.ArtifactRepository, artifactUUID, orgID string) error {
	if repo == nil {
		return nil
	}
	artifact, err := repo.GetByUUID(artifactUUID, orgID)
	if err != nil {
		return fmt.Errorf("failed to look up artifact origin: %w", err)
	}
	if artifact == nil {
		return nil
	}
	return ensureOriginMutable(artifact.Origin)
}

// toAPIDeploymentResponse converts deployment fields into an api.DeploymentResponse.
func toAPIDeploymentResponse(
	gatewayRepo repository.GatewayRepository,
	deploymentID string,
	name string,
	gatewayID string,
	status model.DeploymentStatus,
	baseDeploymentID *string,
	metadata map[string]interface{},
	createdAt time.Time,
	updatedAt *time.Time,
	statusReason *string,
) (*api.DeploymentResponse, error) {
	deploymentUUID := utils.ParseOpenAPIUUIDOrZero(deploymentID)
	baseUUID := utils.ParseOptionalOpenAPIUUID(baseDeploymentID)

	gateway, err := gatewayRepo.GetByUUID(gatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve gateway handle: %w", err)
	}
	gatewayHandle := gatewayID
	if gateway != nil {
		gatewayHandle = gateway.Handle
	}

	resp := &api.DeploymentResponse{
		BaseDeploymentId: baseUUID,
		CreatedAt:        createdAt,
		DeploymentId:     deploymentUUID,
		GatewayId:        gatewayHandle,
		Metadata:         utils.MapPtrIfNotEmpty(metadata),
		Name:             name,
		Status:           api.DeploymentResponseStatus(status),
		StatusReason:     statusReason,
		UpdatedAt:        updatedAt,
	}
	return resp, nil
}

// mapUpstreamAPIToModel converts an API Upstream to its model representation.
func mapUpstreamAPIToModel(in api.Upstream) *model.UpstreamConfig {
	out := &model.UpstreamConfig{}
	out.Main = &model.UpstreamEndpoint{
		URL: utils.ValueOrEmpty(in.Main.Url),
		Ref: utils.ValueOrEmpty(in.Main.Ref),
	}
	if in.Main.Auth != nil {
		out.Main.Auth = mapUpstreamAuthAPIToModel(in.Main.Auth)
	}
	if in.Sandbox != nil {
		out.Sandbox = &model.UpstreamEndpoint{
			URL: utils.ValueOrEmpty(in.Sandbox.Url),
			Ref: utils.ValueOrEmpty(in.Sandbox.Ref),
		}
		if in.Sandbox.Auth != nil {
			out.Sandbox.Auth = mapUpstreamAuthAPIToModel(in.Sandbox.Auth)
		}
	}
	return out
}

// mapUpstreamModelToAPI converts a model UpstreamConfig to its API representation.
func mapUpstreamModelToAPI(in *model.UpstreamConfig) api.Upstream {
	main := api.UpstreamDefinition{}
	if in != nil && in.Main != nil {
		if strings.TrimSpace(in.Main.URL) != "" {
			u := in.Main.URL
			main.Url = &u
		}
		if strings.TrimSpace(in.Main.Ref) != "" {
			r := in.Main.Ref
			main.Ref = &r
		}
		if in.Main.Auth != nil {
			main.Auth = mapUpstreamAuthModelToAPI(in.Main.Auth)
		}
	}
	var sandbox *api.UpstreamDefinition
	if in != nil && in.Sandbox != nil {
		s := api.UpstreamDefinition{}
		if strings.TrimSpace(in.Sandbox.URL) != "" {
			u := in.Sandbox.URL
			s.Url = &u
		}
		if strings.TrimSpace(in.Sandbox.Ref) != "" {
			r := in.Sandbox.Ref
			s.Ref = &r
		}
		if in.Sandbox.Auth != nil {
			s.Auth = mapUpstreamAuthModelToAPI(in.Sandbox.Auth)
		}
		sandbox = &s
	}
	return api.Upstream{Main: main, Sandbox: sandbox}
}

func mapUpstreamAuthAPIToModel(in *api.UpstreamAuth) *model.UpstreamAuth {
	if in == nil {
		return nil
	}
	authType := ""
	if in.Type != nil {
		authType = normalizeUpstreamAuthType(string(*in.Type))
	}
	return &model.UpstreamAuth{
		Type:   authType,
		Header: utils.ValueOrEmpty(in.Header),
		Value:  utils.ValueOrEmpty(in.Value),
	}
}

func mapUpstreamAuthModelToAPI(in *model.UpstreamAuth) *api.UpstreamAuth {
	if in == nil {
		return nil
	}
	var authType *api.UpstreamAuthType
	if normalized := normalizeUpstreamAuthType(in.Type); normalized != "" {
		t := api.UpstreamAuthType(normalized)
		authType = &t
	}
	return &api.UpstreamAuth{
		Type:   authType,
		Header: utils.StringPtrIfNotEmpty(in.Header),
		Value:  utils.StringPtrIfNotEmpty(in.Value),
	}
}

func normalizeUpstreamAuthType(authType string) string {
	normalized := strings.TrimSpace(authType)
	if normalized == "" {
		return ""
	}
	canonical := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(normalized, "-", ""), "_", ""))
	switch canonical {
	case "apikey":
		return string(api.ApiKey)
	case "basic":
		return string(api.Basic)
	case "bearer":
		return string(api.Bearer)
	default:
		return normalized
	}
}
