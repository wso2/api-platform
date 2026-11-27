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

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// isSQLiteUniqueConstraint reports whether err is a SQLite unique-constraint violation.
func isSQLiteUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// ensureOriginMutable returns an ARTIFACT_READ_ONLY error when the artifact
// originated from a data-plane gateway. DP artifacts are read-only in the
// control plane.
func ensureOriginMutable(origin string) error {
	if origin == constants.OriginDP {
		return apperror.ArtifactReadOnly.New(
			"This artifact is read-only because it originated from a data-plane gateway.")
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

// mapUpstreamAuthModelToAPI maps a stored upstream auth block to its API
// representation. Type and Header identify the scheme and survive the trip;
// Value is declared writeOnly in the OpenAPI contract and is never returned,
// matching mapUpstreamConfigToDTO, which backs the LLM, MCP and GraphQL
// upstream responses.
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
		Value:  nil, // writeOnly: supplied on configure, never returned
	}
}

// preserveWebSubUpstreamAuth backfills the auth value of each endpoint in
// updated from the stored configuration whenever the request leaves it empty.
// Reads redact the value, so a client that round-trips a GET response sends an
// auth block with no value and means "leave it as it is"; without this the
// stored credential would be overwritten with an empty string on every save
// that did not retype it.
//
// It mirrors preserveUpstreamAuthOnAPIUpdate, the REST API counterpart.
// updated is taken by value, but Sandbox and both Auth blocks are pointers
// into the caller's request. Every write below replaces a pointer rather than
// writing through one, so a credential restored from storage never lands back
// on the request object the caller still holds.
func preserveWebSubUpstreamAuth(existing model.UpstreamConfig, updated api.Upstream) api.Upstream {
	updated.Main.Auth = preserveWebSubEndpointAuth(existing.Main, updated.Main.Auth)
	if updated.Sandbox != nil {
		sandbox := *updated.Sandbox
		sandbox.Auth = preserveWebSubEndpointAuth(existing.Sandbox, sandbox.Auth)
		updated.Sandbox = &sandbox
	}
	return updated
}

// preserveWebSubEndpointAuth returns the auth block to store for one endpoint.
// A request that omits the auth block entirely, or sends one with an empty
// value, keeps the stored credential. An explicit auth type change does not:
// the stored secret belongs to the previous scheme and must not be silently
// reattached to a different one.
//
// An omitted type and an omitted header are likewise "leave it as it is":
// mapUpstreamAuthAPIToModel stores an empty string for a nil field, so without
// restoring the stored ones here a save that did not retype them would keep
// the credential but blank the scheme it belongs to and the header it is sent
// in. A header the request does supply always wins.
func preserveWebSubEndpointAuth(existing *model.UpstreamEndpoint, updated *api.UpstreamAuth) *api.UpstreamAuth {
	if existing == nil || existing.Auth == nil {
		return updated
	}
	storedType := normalizeUpstreamAuthType(existing.Auth.Type)

	if updated == nil {
		// The request omits the block entirely, so the whole stored one stays.
		// A stored block with no credential is still configuration -- a scheme
		// (including "none") and a header name -- and is restored rather than
		// dropped.
		restored := &api.UpstreamAuth{
			Header: utils.StringPtrIfNotEmpty(existing.Auth.Header),
			Value:  utils.StringPtrIfNotEmpty(existing.Auth.Value),
		}
		if storedType != "" {
			authType := api.UpstreamAuthType(storedType)
			restored.Type = &authType
		}
		return restored
	}

	// Copy before merging: the block belongs to the caller's request.
	merged := *updated
	typeChanged := merged.Type != nil &&
		normalizeUpstreamAuthType(string(*merged.Type)) != storedType
	if merged.Type == nil && storedType != "" {
		restoredType := api.UpstreamAuthType(storedType)
		merged.Type = &restoredType
	}
	if typeChanged {
		// The stored header and credential belong to the previous scheme and
		// must not be reattached to a different one.
		return &merged
	}
	// Same scheme: an omitted header and an omitted value both mean "leave it
	// as it is". A header supplied by the request always wins.
	if merged.Header == nil {
		merged.Header = utils.StringPtrIfNotEmpty(existing.Auth.Header)
	}
	if merged.Value == nil || *merged.Value == "" {
		merged.Value = utils.StringPtrIfNotEmpty(existing.Auth.Value)
	}
	return &merged
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

// resolveProjectRef resolves a project reference within an organization. The reference is
// the project handle, as elsewhere in the Platform API; the project UUID is still accepted
// for existing clients.
func resolveProjectRef(projectRepo repository.ProjectRepository, ref, orgUUID string) (*model.Project, error) {
	project, err := projectRepo.GetProjectByHandleAndOrgID(ref, orgUUID)
	if err != nil || project != nil {
		return project, err
	}
	return projectRepo.GetProjectByUUIDAndOrgID(ref, orgUUID)
}

// projectHandleFor returns the handle of the project with the given UUID, falling back to
// the UUID when the project cannot be read.
func projectHandleFor(projectRepo repository.ProjectRepository, projectUUID string) string {
	if projectRepo == nil || projectUUID == "" {
		return projectUUID
	}
	project, err := projectRepo.GetProjectByUUID(projectUUID)
	if err != nil || project == nil {
		return projectUUID
	}
	return project.Handle
}
