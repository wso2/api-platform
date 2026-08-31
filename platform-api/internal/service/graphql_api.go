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
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// GraphQLAPIService handles business logic for GraphQL API operations.
// GraphQL is a core artifact kind (like RestApi/LlmProvider/LlmProxy/Mcp)
type GraphQLAPIService struct {
	repo                 repository.GraphQLAPIRepository
	projectRepo          repository.ProjectRepository
	auditRepo            repository.AuditRepository
	deploymentRepo       repository.DeploymentRepository
	gatewayRepo          repository.GatewayRepository
	orgRepo              repository.OrganizationRepository
	gatewayEventsService *GatewayEventsService
	identity             *IdentityService
	secretService        *SecretService
	slogger              *slog.Logger
	maxSDLFetchBytes     int64
}

// NewGraphQLAPIService creates a new GraphQLAPIService instance.
func NewGraphQLAPIService(
	repo repository.GraphQLAPIRepository,
	projectRepo repository.ProjectRepository,
	auditRepo repository.AuditRepository,
	deploymentRepo repository.DeploymentRepository,
	gatewayRepo repository.GatewayRepository,
	orgRepo repository.OrganizationRepository,
	gatewayEventsService *GatewayEventsService,
	identity *IdentityService,
	slogger *slog.Logger,
) *GraphQLAPIService {
	return &GraphQLAPIService{
		repo:                 repo,
		projectRepo:          projectRepo,
		auditRepo:            auditRepo,
		deploymentRepo:       deploymentRepo,
		gatewayRepo:          gatewayRepo,
		orgRepo:              orgRepo,
		gatewayEventsService: gatewayEventsService,
		identity:             identity,
		slogger:              slogger,
	}
}

// SetSecretService injects the SecretService used to validate
// {{ secret "..." }} placeholders on Create/Update — GraphQL's
// upstream.auth/policy params can embed the same placeholders REST's can,
// so this is wired the same way APIService.SetSecretService is. Called
// after both services are constructed to avoid a circular dependency.
func (s *GraphQLAPIService) SetSecretService(ss *SecretService) {
	s.secretService = ss
}

// SetMaxSDLFetchBytes sets the byte ceiling applied when fetching an SDL
// document from sdlUrl — reuses cfg.Server.OpenAPISpecMaxFetchBytes, the same
// generic external-document-fetch limit already used for LLM provider
// templates' openapiSpecUrl, rather than introducing a GraphQL-only config
// key for what is the same kind of bounded fetch. Zero/unset falls back to
// FetchOpenAPISpecFromURL's own built-in default.
func (s *GraphQLAPIService) SetMaxSDLFetchBytes(n int64) {
	s.maxSDLFetchBytes = n
}

// toGraphQLAPI converts m via mapGraphQLAPIModelToAPI, resolves its stored
// project UUID back to the project's handle for the response's projectId
// field (mirrors internal/service/api.go's modelToRESTAPI), and resolves its
// createdBy/updatedBy UUIDs to their raw external identity.
func (s *GraphQLAPIService) toGraphQLAPI(m *model.GraphQLAPI) (*api.GraphQLAPI, error) {
	resp := mapGraphQLAPIModelToAPI(m)
	if resp == nil {
		return nil, nil
	}
	if s.projectRepo != nil {
		project, err := s.projectRepo.GetProjectByUUID(resp.ProjectId)
		if err != nil {
			return nil, err
		}
		if project != nil {
			resp.ProjectId = project.Handle
		}
	}
	if err := s.identity.ResolveIdentityField(&resp.CreatedBy); err != nil {
		return nil, err
	}
	if err := s.identity.ResolveIdentityField(&resp.UpdatedBy); err != nil {
		return nil, err
	}
	return resp, nil
}

// toGraphQLAPIDetail is toGraphQLAPI's counterpart for the sdl-less detail
// response (GET /graphql-apis/{graphqlApiId}) — same project-handle and
// identity resolution, built from mapGraphQLAPIModelToDetail instead.
func (s *GraphQLAPIService) toGraphQLAPIDetail(m *model.GraphQLAPI) (*api.GraphQLAPIDetail, error) {
	resp := mapGraphQLAPIModelToDetail(m)
	if resp == nil {
		return nil, nil
	}
	if s.projectRepo != nil {
		project, err := s.projectRepo.GetProjectByUUID(resp.ProjectId)
		if err != nil {
			return nil, err
		}
		if project != nil {
			resp.ProjectId = project.Handle
		}
	}
	if err := s.identity.ResolveIdentityField(&resp.CreatedBy); err != nil {
		return nil, err
	}
	if err := s.identity.ResolveIdentityField(&resp.UpdatedBy); err != nil {
		return nil, err
	}
	return resp, nil
}

// Create creates a new GraphQL API. Supply either req.Sdl directly or
// req.Upstream.Main.Url — exactly one schema-resolution path runs.
func (s *GraphQLAPIService) Create(orgUUID, createdBy string, req *api.CreateGraphQLAPIRequest) (*api.GraphQLAPI, error) {
	if req == nil {
		return nil, apperror.ValidationFailed.New("A request body is required.")
	}
	if req.DisplayName == "" || req.Version == "" || req.Context == "" {
		return nil, apperror.ValidationFailed.New("The displayName, context and version fields are required.")
	}
	if req.ProjectId == "" {
		return nil, apperror.ValidationFailed.New("The projectId field is required.")
	}

	// Validate {{ secret "..." }} placeholders anywhere in the request — the
	// gateway-controller's template engine resolves placeholders generically
	// across the whole artifact (upstream auth and policies alike), so
	// validation must cover the same surface as REST's CreateAPI does.
	if s.secretService != nil {
		configJSON, err := marshalUpstreamForValidation(req)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request for secret validation: %w", err)
		}
		if err := s.secretService.ValidateSecretRefs(orgUUID, configJSON); err != nil {
			return nil, err
		}
	}

	// Resolve the project by handle (req.ProjectId is actually the project's
	// user-facing handle, e.g. "default-project", not its internal UUID —
	// mirrors internal/service/api.go's CreateAPI). GO-AUTH-005: org scoping
	// is enforced here, never trusted from the request.
	projectUUID := req.ProjectId
	if s.projectRepo != nil {
		project, err := s.projectRepo.GetProjectByHandleAndOrgID(req.ProjectId, orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate project: %w", err)
		}
		if project == nil || project.OrganizationID != orgUUID {
			return nil, apperror.ProjectRefNotFound.New()
		}
		projectUUID = project.ID
	}

	// Handle (user-facing identifier): use the supplied one, or generate from
	// displayName with collision detection (mirrors internal/service/api.go's
	// CreateAPI).
	var handle string
	if req.Id != nil && *req.Id != "" {
		handle = *req.Id
	} else {
		generated, err := utils.GenerateHandle(req.DisplayName, s.handleExistsCheck(orgUUID))
		if err != nil {
			s.slogger.Error("Failed to generate GraphQL API handle", "apiName", req.DisplayName, "error", err)
			return nil, err
		}
		handle = generated
	}

	exists, err := s.repo.Exists(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to check GraphQL API exists: %w", err)
	}
	if exists {
		return nil, apperror.GraphQLAPIExists.New()
	}

	upstream := mapUpstreamAPIToModel(req.Upstream)
	sdl, introspectionMode, err := s.resolveSchema(utils.ValueOrEmpty(req.Sdl), utils.ValueOrEmpty(req.SdlUrl), upstream)
	if err != nil {
		return nil, err
	}

	var subscriptionPlans []string
	if req.SubscriptionPlans != nil {
		subscriptionPlans = *req.SubscriptionPlans
	}

	context := req.Context
	m := &model.GraphQLAPI{
		Handle:         handle,
		OrganizationID: orgUUID,
		ProjectID:      projectUUID,
		Name:           req.DisplayName,
		Description:    utils.ValueOrEmpty(req.Description),
		CreatedBy:      createdBy,
		UpdatedBy:      createdBy,
		Version:        req.Version,
		Configuration: model.GraphQLAPIConfig{
			Name:              req.DisplayName,
			Version:           req.Version,
			Context:           &context,
			SDL:               sdl,
			IntrospectionMode: introspectionMode,
			Upstream:          *upstream,
			Policies:          mapMCPPoliciesAPIToModel(req.Policies),
			SubscriptionPlans: subscriptionPlans,
		},
		Origin: constants.OriginCP,
	}

	if err := s.repo.Create(m); err != nil {
		if isSQLiteUniqueConstraint(err) {
			return nil, apperror.GraphQLAPIExists.Wrap(err)
		}
		return nil, fmt.Errorf("failed to create GraphQL API: %w", err)
	}

	if s.auditRepo != nil {
		_ = s.auditRepo.Record("CREATE", m.ID, "graphql_api", orgUUID, createdBy)
	}
	return s.Get(orgUUID, handle)
}

// resolveSchema implements the onboarding paths: a directly supplied SDL
// (pasted inline, uploaded as a file, or fetched from sdlUrl — the caller has
// already collapsed all three into suppliedSDL/sdlURL by the time this runs)
// is parsed/validated as-is; when neither is given, upstream.main.url is
// required and the schema is derived via introspection. Exactly one of
// sdl/mode is returned on success; on failure the error is always the sterile
// GraphQLAPISchemaResolveFailed catalog entry (422) — the specific
// parser/fetch/introspection failure reason is never surfaced to the client
// (error-handling.md / ssrf-prevention.md).
func (s *GraphQLAPIService) resolveSchema(suppliedSDL, sdlURL string, upstream *model.UpstreamConfig) (sdl string, introspectionMode string, err error) {
	suppliedSDL = strings.TrimSpace(suppliedSDL)
	sdlURL = strings.TrimSpace(sdlURL)

	if suppliedSDL != "" && sdlURL != "" {
		return "", "", apperror.ValidationFailed.New("The sdl and sdlUrl fields are mutually exclusive — provide only one.")
	}

	if sdlURL != "" {
		fetched, err := utils.FetchOpenAPISpecFromURL(context.Background(), sdlURL, s.maxSDLFetchBytes)
		if err != nil {
			s.slogger.Warn("Failed to fetch GraphQL SDL from sdlUrl", "error", err)
			return "", "", apperror.GraphQLAPISchemaResolveFailed.Wrap(err)
		}
		suppliedSDL = strings.TrimSpace(fetched)
	}

	if suppliedSDL != "" {
		if err := validateGraphQLSDL(suppliedSDL); err != nil {
			s.slogger.Warn("Supplied GraphQL SDL failed validation", "error", err)
			return "", "", apperror.GraphQLAPISchemaResolveFailed.Wrap(err)
		}
		return suppliedSDL, "SDL", nil
	}

	if upstream == nil || upstream.Main == nil || strings.TrimSpace(upstream.Main.URL) == "" {
		return "", "", apperror.ValidationFailed.New("One of sdl, sdlUrl, or upstream.main.url must be provided.")
	}

	derived, err := fetchAndConvertGraphQLSchema(upstream.Main.URL)
	if err != nil {
		s.slogger.Warn("GraphQL introspection failed", "error", err)
		return "", "", apperror.GraphQLAPISchemaResolveFailed.Wrap(err)
	}
	return derived, "ENDPOINT", nil
}

// handleExistsCheck returns a function that checks if a GraphQL API handle
// exists in the organization, for use with utils.GenerateHandle.
func (s *GraphQLAPIService) handleExistsCheck(orgUUID string) func(string) bool {
	return func(handle string) bool {
		exists, err := s.repo.Exists(handle, orgUUID)
		if err != nil {
			// On error, assume it exists to be safe (triggers a retry with a
			// different suffix rather than risking a collision).
			return true
		}
		return exists
	}
}

// Get retrieves a GraphQL API by its handle.
func (s *GraphQLAPIService) Get(orgUUID, handle string) (*api.GraphQLAPI, error) {
	if handle == "" {
		return nil, apperror.ValidationFailed.New("The GraphQL API id is required.")
	}

	m, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get GraphQL API: %w", err)
	}
	if m == nil {
		return nil, apperror.GraphQLAPINotFound.New()
	}

	return s.toGraphQLAPI(m)
}

// GetDetail is Get's counterpart for GET /graphql-apis/{graphqlApiId}, which
// deliberately omits sdl from its response — see GetSDL to fetch it
// separately.
func (s *GraphQLAPIService) GetDetail(orgUUID, handle string) (*api.GraphQLAPIDetail, error) {
	if handle == "" {
		return nil, apperror.ValidationFailed.New("The GraphQL API id is required.")
	}

	m, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get GraphQL API: %w", err)
	}
	if m == nil {
		return nil, apperror.GraphQLAPINotFound.New()
	}

	return s.toGraphQLAPIDetail(m)
}

// GetSDL retrieves a GraphQL API's resolved SDL text for
// GET /graphql-apis/{graphqlApiId}/sdl — the counterpart to GetDetail
// omitting it.
func (s *GraphQLAPIService) GetSDL(orgUUID, handle string) (string, error) {
	if handle == "" {
		return "", apperror.ValidationFailed.New("The GraphQL API id is required.")
	}

	m, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return "", fmt.Errorf("failed to get GraphQL API: %w", err)
	}
	if m == nil {
		return "", apperror.GraphQLAPINotFound.New()
	}

	return m.Configuration.SDL, nil
}

// List retrieves GraphQL APIs for an organization, filtered by project.
func (s *GraphQLAPIService) List(orgUUID, projectHandle string, limit, offset int) (*api.GraphQLAPIListResponse, error) {
	projectUUID := ""
	// If a project handle is provided, resolve it and validate that it belongs
	// to the organization (mirrors internal/service/api.go's
	// GetAPIsByOrganization) — projectHandle is the caller-facing slug (e.g.
	// "default-project"), never the internal UUID rows are actually keyed on.
	if projectHandle != "" && s.projectRepo != nil {
		project, err := s.projectRepo.GetProjectByHandleAndOrgID(projectHandle, orgUUID)
		if err != nil {
			return nil, err
		}
		if project == nil {
			return nil, apperror.ProjectRefNotFound.New()
		}
		projectUUID = project.ID
	}

	apis, err := s.repo.List(orgUUID, projectUUID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list GraphQL APIs: %w", err)
	}

	var totalCount int
	if projectUUID != "" {
		totalCount, err = s.repo.CountByProject(orgUUID, projectUUID)
	} else {
		totalCount, err = s.repo.Count(orgUUID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to count GraphQL APIs: %w", err)
	}

	resp := &api.GraphQLAPIListResponse{
		Count: len(apis),
		Pagination: api.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  totalCount,
		},
	}

	// Resolve each item's stored project UUID back to its handle for display
	// (mirrors REST's modelToRESTAPIUnresolved), caching per unique project
	// UUID since a filtered list page typically shares one project.
	projectHandles := map[string]string{}
	if projectHandle != "" {
		projectHandles[projectUUID] = projectHandle
	}
	resolveProjectHandle := func(uuid string) (string, error) {
		if handle, ok := projectHandles[uuid]; ok {
			return handle, nil
		}
		if s.projectRepo == nil {
			return uuid, nil
		}
		project, err := s.projectRepo.GetProjectByUUID(uuid)
		if err != nil {
			return "", err
		}
		handle := uuid
		if project != nil {
			handle = project.Handle
		}
		projectHandles[uuid] = handle
		return handle, nil
	}

	resp.List = make([]api.GraphQLAPIListItem, 0, len(apis))
	createdByFields := make([]**string, 0, len(apis))
	for _, a := range apis {
		item := mapGraphQLAPIModelToListItem(a)
		if item == nil {
			continue
		}
		if handle, err := resolveProjectHandle(item.ProjectId); err == nil {
			item.ProjectId = handle
		} else {
			return nil, err
		}
		resp.List = append(resp.List, *item)
		createdByFields = append(createdByFields, &resp.List[len(resp.List)-1].CreatedBy)
	}
	if err := s.identity.ResolveIdentityFields(createdByFields); err != nil {
		return nil, err
	}

	return resp, nil
}

// Update updates an existing GraphQL API. The project association is
// immutable via this endpoint (req.ProjectId is not applied) — a PUT never
// moves an artifact to a different project.
func (s *GraphQLAPIService) Update(orgUUID, handle, updatedBy string, req *api.GraphQLAPI) (*api.GraphQLAPI, error) {
	if handle == "" || req == nil {
		return nil, apperror.ValidationFailed.New("The GraphQL API id and a request body are required.")
	}
	if req.DisplayName == "" || req.Version == "" || req.Context == "" {
		return nil, apperror.ValidationFailed.New("The displayName, context and version fields are required.")
	}

	// Validate {{ secret "..." }} placeholders anywhere in the request — see
	// Create for why this covers the whole request, not just upstream.
	if s.secretService != nil {
		configJSON, err := marshalUpstreamForValidation(req)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request for secret validation: %w", err)
		}
		if err := s.secretService.ValidateSecretRefs(orgUUID, configJSON); err != nil {
			return nil, err
		}
	}

	existing, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get GraphQL API: %w", err)
	}
	if existing == nil {
		return nil, apperror.GraphQLAPINotFound.New()
	}
	// DP-originated artifacts are read-only in the control plane.
	if err := ensureOriginMutable(existing.Origin); err != nil {
		return nil, err
	}
	if req.Id != nil && *req.Id != "" && *req.Id != handle {
		return nil, apperror.ValidationFailed.New("The id in the request body must match the path parameter.")
	}

	upstream := mapUpstreamAPIToModel(req.Upstream)
	sdl, introspectionMode, err := s.resolveSchema(utils.ValueOrEmpty(req.Sdl), utils.ValueOrEmpty(req.SdlUrl), upstream)
	if err != nil {
		return nil, err
	}

	var subscriptionPlans []string
	if req.SubscriptionPlans != nil {
		subscriptionPlans = *req.SubscriptionPlans
	}

	context := req.Context
	existing.Name = req.DisplayName
	existing.Version = req.Version
	existing.Description = utils.ValueOrEmpty(req.Description)
	existing.UpdatedBy = updatedBy
	existing.Configuration = model.GraphQLAPIConfig{
		Name:              req.DisplayName,
		Version:           req.Version,
		Context:           &context,
		SDL:               sdl,
		IntrospectionMode: introspectionMode,
		Upstream:          *upstream,
		Policies:          mapMCPPoliciesAPIToModel(req.Policies),
		SubscriptionPlans: subscriptionPlans,
	}

	if err := s.repo.Update(existing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.GraphQLAPINotFound.Wrap(err)
		}
		return nil, fmt.Errorf("failed to update GraphQL API: %w", err)
	}

	if s.auditRepo != nil {
		_ = s.auditRepo.Record("UPDATE", existing.ID, "graphql_api", orgUUID, updatedBy)
	}
	return s.Get(orgUUID, handle)
}

// Delete deletes a GraphQL API by its handle.
func (s *GraphQLAPIService) Delete(orgUUID, handle, deletedBy string) error {
	if handle == "" {
		return apperror.ValidationFailed.New("The GraphQL API id is required.")
	}

	existing, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to get GraphQL API: %w", err)
	}
	if existing == nil {
		return apperror.GraphQLAPINotFound.New()
	}
	// DP-originated artifacts may only be deleted once undeployed on all gateways.
	if err := ensureOriginDeletable(s.deploymentRepo, existing.Origin, existing.ID, orgUUID); err != nil {
		return err
	}

	// Get all gateways in the organization to broadcast deletion event.
	// We broadcast to all gateways (not just those with active deployments) because
	// deployment_status rows may have been cascade-deleted when deployments were removed,
	// leaving stale artifacts on gateways that would otherwise never receive the delete event.
	var gateways []*model.Gateway
	if s.gatewayRepo != nil {
		gws, err := s.gatewayRepo.GetByOrganizationID(orgUUID)
		if err != nil {
			s.slogger.Warn("Failed to get gateways for GraphQL API deletion", "error", err, "apiUUID", existing.ID)
		} else {
			gateways = gws
		}
	}

	if err := s.repo.Delete(handle, orgUUID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperror.GraphQLAPINotFound.Wrap(err)
		}
		return fmt.Errorf("failed to delete GraphQL API: %w", err)
	}

	if s.auditRepo != nil {
		_ = s.auditRepo.Record("DELETE", existing.ID, "graphql_api", orgUUID, deletedBy)
	}

	// Send deletion events to all gateways in the organization
	if s.gatewayEventsService != nil && len(gateways) > 0 {
		for _, gateway := range gateways {
			deletionEvent := &model.GraphQLAPIDeletionEvent{
				ApiId: existing.ID,
			}
			if err := s.gatewayEventsService.BroadcastGraphQLAPIDeletionEvent(gateway.ID, deletionEvent); err != nil {
				s.slogger.Warn("Failed to broadcast GraphQL API deletion event", "error", err, "gatewayID", gateway.ID, "apiUUID", existing.ID)
			} else {
				s.slogger.Info("GraphQL API deletion event sent", "gatewayID", gateway.ID, "apiUUID", existing.ID)
			}
		}
	}

	return nil
}

// Count returns the total number of GraphQL APIs for an organization.
func (s *GraphQLAPIService) Count(orgUUID string) (int, error) {
	return s.repo.Count(orgUUID)
}

// AddGatewaysToAPI associates multiple gateways with a GraphQL API identified by
// handle. Mirrors APIService.AddGatewaysToAPIByHandle/AddGatewaysToAPI (api.go):
// the underlying artifact_gateway_mappings table and its CRUD methods are
// kind-agnostic (see GraphQLAPIRepository's doc comment), so this is a thin
// wrapper resolving the handle to a UUID and delegating to the same generic
// association helpers, reusing REST's response DTO
// (api.RESTAPIGatewayListResponse) since the shape carries no REST-specific
// fields — See resources/openapi.yaml's
// /graphql-apis/{graphqlApiId}/gateways path.
func (s *GraphQLAPIService) AddGatewaysToAPI(handle string, gatewayIds []string, orgUUID, createdBy string) (*api.RESTAPIGatewayListResponse, error) {
	apiModel, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get GraphQL API: %w", err)
	}
	if apiModel == nil {
		return nil, apperror.GraphQLAPINotFound.New()
	}

	var validGateways []*model.Gateway
	for _, gatewayId := range gatewayIds {
		gateway, err := s.gatewayRepo.GetByHandleAndOrgID(gatewayId, orgUUID)
		if err != nil {
			return nil, err
		}
		if gateway == nil {
			return nil, apperror.GatewayNotFound.New()
		}
		validGateways = append(validGateways, gateway)
	}

	existingAssociations, err := s.repo.GetAPIAssociations(apiModel.ID, constants.AssociationTypeGateway, orgUUID)
	if err != nil {
		return nil, err
	}
	existingGatewayIds := make(map[string]bool)
	for _, assoc := range existingAssociations {
		existingGatewayIds[assoc.GatewayID] = true
	}
	for _, gateway := range validGateways {
		if existingGatewayIds[gateway.ID] {
			if err := s.repo.UpdateAPIAssociation(apiModel.ID, gateway.ID, constants.AssociationTypeGateway, orgUUID, createdBy); err != nil {
				return nil, err
			}
		} else {
			association := &model.APIAssociation{
				ArtifactID:     apiModel.ID,
				OrganizationID: orgUUID,
				GatewayID:      gateway.ID,
				CreatedBy:      createdBy,
			}
			if err := s.repo.CreateAPIAssociation(association); err != nil {
				return nil, err
			}
			existingGatewayIds[gateway.ID] = true
		}
	}

	return s.getAPIGateways(apiModel.ID, orgUUID)
}

// GetAPIGateways retrieves a page of gateways associated with a GraphQL API
// identified by handle, applying the requested limit/offset window. Mirrors
// APIService.GetAPIGatewaysByHandle.
func (s *GraphQLAPIService) GetAPIGateways(handle, orgUUID string, limit, offset int) (*api.RESTAPIGatewayListResponse, error) {
	apiModel, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get GraphQL API: %w", err)
	}
	if apiModel == nil {
		return nil, apperror.GraphQLAPINotFound.New()
	}

	gatewayDetails, err := s.repo.GetAPIGatewaysWithDetails(apiModel.ID, orgUUID)
	if err != nil {
		return nil, err
	}
	org, err := s.orgRepo.GetOrganizationByUUID(orgUUID)
	if err != nil {
		return nil, err
	}
	orgHandle := ""
	if org != nil {
		orgHandle = org.Handle
	}

	// The gateways associated with a single API are a small, bounded set, so the
	// requested window is applied in memory while the total reflects the full set.
	total := len(gatewayDetails)
	page := paginateSlice(gatewayDetails, limit, offset)

	response, err := apiGatewayDetailsToAPIList(page, orgHandle)
	if err != nil {
		return nil, fmt.Errorf("failed to convert API gateway details: %w", err)
	}
	response.Pagination = api.Pagination{Total: total, Offset: offset, Limit: limit}
	return response, nil
}

// getAPIGateways retrieves all gateways associated with a GraphQL API (by UUID),
// unpaginated — used internally right after a gateway association change so the
// caller sees the full, up-to-date set (mirrors APIService.GetAPIGateways).
func (s *GraphQLAPIService) getAPIGateways(apiUUID, orgUUID string) (*api.RESTAPIGatewayListResponse, error) {
	gatewayDetails, err := s.repo.GetAPIGatewaysWithDetails(apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	org, err := s.orgRepo.GetOrganizationByUUID(orgUUID)
	if err != nil {
		return nil, err
	}
	orgHandle := ""
	if org != nil {
		orgHandle = org.Handle
	}
	response, err := apiGatewayDetailsToAPIList(gatewayDetails, orgHandle)
	if err != nil {
		return nil, fmt.Errorf("failed to convert API gateway details: %w", err)
	}
	return response, nil
}
