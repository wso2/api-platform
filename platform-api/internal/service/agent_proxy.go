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
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// agentProxyAuditResource is the resource label recorded against Agent proxy
// audit entries.
const agentProxyAuditResource = "agent_proxy"

// reservedAgentProxyHandles are handles the Agent proxy collection cannot hand
// out, because a static sibling route under /agent-proxies/ already owns the
// path segment. An Agent proxy called "fetch-agent-card" would be addressed by
// the discovery route instead of its own resource, so the name is refused at
// creation rather than left to shadow itself later.
//
// Generated handles are checked against this set too: deriving one from a
// display name is not a way around the reservation.
var reservedAgentProxyHandles = map[string]struct{}{
	"fetch-agent-card": {},
}

// AgentProxyService implements the Agent proxy CRUD operations.
//
// Public identifiers are handles, always resolved inside the organization the
// access token carries before anything reaches a UUID-keyed repository — so a
// handle from one organization can never address a row in another.
type AgentProxyService struct {
	repo                 repository.AgentProxyRepository
	projectRepo          repository.ProjectRepository
	deploymentRepo       repository.DeploymentRepository
	gatewayRepo          repository.GatewayRepository
	gatewayEventsService *GatewayEventsService
	secretService        *SecretService
	auditRepo            repository.AuditRepository
	identity             *IdentityService
	cfg                  *config.Server
	slogger              *slog.Logger
}

// NewAgentProxyService creates a new AgentProxyService instance.
func NewAgentProxyService(repo repository.AgentProxyRepository, projectRepo repository.ProjectRepository,
	deploymentRepo repository.DeploymentRepository, gatewayRepo repository.GatewayRepository,
	gatewayEventsService *GatewayEventsService, slogger *slog.Logger, auditRepo repository.AuditRepository,
	cfg *config.Server, identity *IdentityService) *AgentProxyService {
	return &AgentProxyService{
		repo:                 repo,
		projectRepo:          projectRepo,
		deploymentRepo:       deploymentRepo,
		gatewayRepo:          gatewayRepo,
		gatewayEventsService: gatewayEventsService,
		auditRepo:            auditRepo,
		identity:             identity,
		cfg:                  cfg,
		slogger:              slogger,
	}
}

// WithSecretService injects the SecretService used to validate
// {{ secret "handle" }} placeholders and to clean up rotated credentials.
func (s *AgentProxyService) WithSecretService(ss *SecretService) *AgentProxyService {
	s.secretService = ss
	return s
}

// Create stores a new Agent proxy and returns it as the caller will read it back.
func (s *AgentProxyService) Create(orgUUID, createdBy string, req *api.A2AAgentProxy) (*api.A2AAgentProxy, error) {
	if req == nil {
		return nil, apperror.ValidationFailed.New("A request body is required.")
	}
	if err := validateAgentProxyAuthoringFields(req); err != nil {
		return nil, err
	}

	projectUUID, err := s.resolveProjectUUID(orgUUID, req.ProjectId)
	if err != nil {
		return nil, err
	}

	handle, err := s.resolveNewHandle(orgUUID, req)
	if err != nil {
		return nil, err
	}
	req.Id = &handle

	configuration := dto.AgentProxyConfigurationFromRequest(req)
	if err := validateEffectiveUpstreamAuth(&configuration.Upstream); err != nil {
		return nil, err
	}
	if err := s.validateSecretRefs(orgUUID, configuration); err != nil {
		return nil, err
	}

	// Associations are resolved up front so they are persisted in the same
	// transaction as the Agent proxy row.
	associatedGateways, err := resolveAssociatedGateways(s.gatewayRepo, orgUUID, req.AssociatedGateways)
	if err != nil {
		return nil, err
	}

	m := &model.AgentProxy{
		Handle:             handle,
		OrganizationUUID:   orgUUID,
		ProjectUUID:        projectUUID,
		Name:               req.DisplayName,
		Description:        utils.ValueOrEmpty(req.Description),
		Protocol:           model.AgentProxyProtocol(req.Protocol),
		Version:            req.Version,
		CreatedBy:          createdBy,
		UpdatedBy:          createdBy,
		Configuration:      configuration,
		Origin:             constants.OriginCP,
		AssociatedGateways: associatedGateways,
	}

	if err := s.repo.Create(m); err != nil {
		return nil, s.mapRepositoryError(err, "failed to create agent proxy")
	}

	_ = s.auditRepo.Record("CREATE", m.UUID, agentProxyAuditResource, orgUUID, createdBy)
	return s.Get(orgUUID, handle)
}

// Get returns one Agent proxy by its public handle.
func (s *AgentProxyService) Get(orgUUID, handle string) (*api.A2AAgentProxy, error) {
	m, err := s.load(orgUUID, handle)
	if err != nil {
		return nil, err
	}
	return s.toAPI(orgUUID, m)
}

// List returns the organization's Agent proxies, optionally restricted to one
// protocol. The filter is applied to the page and to the total alike, so
// pagination.total always counts the same set the page is drawn from.
//
// protocol is nil when the caller omitted the parameter entirely. That is not
// the same as supplying it empty, which is an invalid filter value rather than
// "no filter" — so the two cannot be collapsed into one empty string.
func (s *AgentProxyService) List(orgUUID string, protocol *string, limit, offset int) (*api.AgentProxyListResponse, error) {
	filter, err := parseAgentProxyProtocolFilter(protocol)
	if err != nil {
		return nil, err
	}
	opts := repository.AgentProxyListOptions{Limit: limit, Offset: offset, Protocol: filter}

	proxies, err := s.repo.List(orgUUID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent proxies: %w", err)
	}
	total, err := s.repo.Count(orgUUID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to count agent proxies: %w", err)
	}

	resp := &api.AgentProxyListResponse{
		List:       make([]api.AgentProxyListItem, 0, len(proxies)),
		Pagination: api.Pagination{Limit: limit, Offset: offset, Total: total},
	}

	// One memo for the whole page: a project handle is otherwise re-queried once
	// per item, and a page is commonly a single project's worth of Agent proxies.
	projectHandles := make(map[string]string, len(proxies))
	identityFields := make([]**string, 0, 2*len(proxies))
	for _, p := range proxies {
		projectHandle, err := s.resolveProjectHandle(orgUUID, p.ProjectUUID, projectHandles)
		if err != nil {
			return nil, err
		}
		resp.List = append(resp.List, dto.AgentProxyToListItem(p, projectHandle))
		item := &resp.List[len(resp.List)-1]
		identityFields = append(identityFields, &item.CreatedBy, &item.UpdatedBy)
	}
	if err := s.identity.ResolveIdentityFields(identityFields); err != nil {
		return nil, err
	}
	resp.Count = len(resp.List)
	return resp, nil
}

// Update replaces the writable configuration of an existing Agent proxy.
//
// It is a full replacement and is idempotent: an omitted optional field resets
// to its default or absence, so replaying the same body leaves the same
// resource. The one exception is the write-only upstream credential, which
// responses redact and a round trip therefore cannot carry back — see
// dto.PreserveAgentProxyUpstreamAuth for exactly how narrow that inheritance is.
func (s *AgentProxyService) Update(orgUUID, handle, updatedBy string, req *api.A2AAgentProxy) (*api.A2AAgentProxy, error) {
	if req == nil {
		return nil, apperror.ValidationFailed.New("A request body is required.")
	}
	if err := validateAgentProxyAuthoringFields(req); err != nil {
		return nil, err
	}

	existing, err := s.load(orgUUID, handle)
	if err != nil {
		return nil, err
	}

	// A gateway-originated Agent proxy is owned by its data plane and is read-only
	// here; readOnly in the request body is never consulted for this.
	if err := ensureOriginMutable(existing.Origin); err != nil {
		return nil, err
	}

	// Protocol is fixed at creation. The comparison is against the persisted
	// column, not against anything in the request or the stored document.
	if model.AgentProxyProtocol(req.Protocol) != existing.Protocol {
		return nil, apperror.ValidationFailed.New(
			fmt.Sprintf("The protocol of an Agent proxy cannot be changed. This Agent proxy is %q.", string(existing.Protocol)))
	}

	// project_uuid is not rewritten by an update — an artifact stays in the
	// project it was created in — so a different project is refused rather than
	// accepted and silently ignored.
	projectUUID, err := s.resolveProjectUUID(orgUUID, req.ProjectId)
	if err != nil {
		return nil, err
	}
	if projectUUID != existing.ProjectUUID {
		return nil, apperror.ValidationFailed.New("The projectId of an Agent proxy cannot be changed.")
	}

	existingUpstream := existing.Configuration.Upstream

	configuration := dto.AgentProxyConfigurationFromRequest(req)
	configuration.Upstream = *dto.PreserveAgentProxyUpstreamAuth(&existingUpstream, &configuration.Upstream)

	// The effective configuration is what is checked — after retention, so an
	// unchanged auth block that legitimately arrived without its redacted value
	// passes, while a *changed* one that arrived without a credential does not
	// inherit the old secret and is rejected here rather than persisted empty.
	if err := validateEffectiveUpstreamAuth(&configuration.Upstream); err != nil {
		return nil, err
	}
	if err := s.validateSecretRefs(orgUUID, configuration); err != nil {
		return nil, err
	}

	// Full replacement extends to associations: an omitted list means an empty
	// association set, so the replacement flag is set unconditionally rather than
	// only when the field was present.
	associatedGateways, err := resolveAssociatedGateways(s.gatewayRepo, orgUUID, req.AssociatedGateways)
	if err != nil {
		return nil, err
	}

	existing.Name = req.DisplayName
	existing.Description = utils.ValueOrEmpty(req.Description)
	existing.Version = req.Version
	existing.UpdatedBy = updatedBy
	existing.Configuration = configuration
	existing.AssociatedGateways = associatedGateways
	existing.ReplaceAssociatedGateways = true

	if err := s.repo.Update(existing); err != nil {
		return nil, s.mapRepositoryError(err, "failed to update agent proxy")
	}

	// Best-effort, and only after the new reference is persisted: until then the
	// in-use check would still see this Agent proxy pointing at the old handle.
	if s.secretService != nil {
		s.secretService.cleanupRotatedSecret(
			orgUUID,
			mainUpstreamAuthValue(&existingUpstream),
			mainUpstreamAuthValue(&existing.Configuration.Upstream),
			updatedBy,
			s.slogger,
		)
	}

	_ = s.auditRepo.Record("UPDATE", existing.UUID, agentProxyAuditResource, orgUUID, updatedBy)
	return s.Get(orgUUID, handle)
}

// Delete removes an Agent proxy from the control plane.
//
// The identifiers gateway cleanup needs are read before the row is removed,
// because they are unrecoverable afterwards. Broadcasting the deletion to those
// gateways is Section 10's event implementation; until it lands, a gateway keeps
// serving routes for an Agent proxy this call has already deleted, so deletion
// is not yet a finished operation.
func (s *AgentProxyService) Delete(orgUUID, handle, deletedBy string) error {
	m, err := s.load(orgUUID, handle)
	if err != nil {
		return err
	}

	// A gateway-originated Agent proxy may only be deleted once it is undeployed
	// everywhere.
	if err := ensureOriginDeletable(s.deploymentRepo, m.Origin, m.UUID, orgUUID); err != nil {
		return err
	}

	if err := s.repo.Delete(handle, orgUUID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperror.AgentProxyNotFound.Wrap(err)
		}
		return fmt.Errorf("failed to delete agent proxy: %w", err)
	}

	_ = s.auditRepo.Record("DELETE", m.UUID, agentProxyAuditResource, orgUUID, deletedBy)
	return nil
}

// load fetches one Agent proxy by handle within the organization, mapping a
// missing row onto the catalog's 404.
func (s *AgentProxyService) load(orgUUID, handle string) (*model.AgentProxy, error) {
	if strings.TrimSpace(handle) == "" {
		return nil, apperror.ValidationFailed.New("The Agent proxy id is required.")
	}
	m, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent proxy: %w", err)
	}
	if m == nil {
		return nil, apperror.AgentProxyNotFound.New()
	}
	return m, nil
}

// toAPI renders a stored Agent proxy as its public shape, with the project UUID
// resolved to a handle and the audit UUIDs resolved to external identities.
func (s *AgentProxyService) toAPI(orgUUID string, m *model.AgentProxy) (*api.A2AAgentProxy, error) {
	projectHandle, err := s.resolveProjectHandle(orgUUID, m.ProjectUUID, nil)
	if err != nil {
		return nil, err
	}
	resp := dto.AgentProxyToResponse(m, projectHandle, mapAssociatedGatewaysModelToAPI(m.AssociatedGateways))
	if resp == nil {
		return nil, nil
	}
	if err := s.identity.ResolveIdentityField(&resp.CreatedBy); err != nil {
		return nil, err
	}
	if err := s.identity.ResolveIdentityField(&resp.UpdatedBy); err != nil {
		return nil, err
	}
	return resp, nil
}

// resolveProjectUUID maps the request's project handle to its UUID within the
// caller's organization. A handle naming another organization's project is
// indistinguishable from one that does not exist, and both are a bad reference
// in the request body rather than a missing addressed resource — so both are the
// same 400.
func (s *AgentProxyService) resolveProjectUUID(orgUUID, projectHandle string) (string, error) {
	handle := strings.TrimSpace(projectHandle)
	if handle == "" {
		return "", apperror.ValidationFailed.New("The projectId field is required.")
	}
	if s.projectRepo == nil {
		return "", fmt.Errorf("cannot resolve project handle: project repository unavailable")
	}
	project, err := s.projectRepo.GetProjectByHandleAndOrgID(handle, orgUUID)
	if err != nil {
		return "", fmt.Errorf("failed to validate project: %w", err)
	}
	if project == nil || project.OrganizationID != orgUUID {
		return "", apperror.ProjectRefNotFound.New()
	}
	return project.ID, nil
}

// resolveProjectHandle maps a stored project UUID back to the handle responses
// carry. The lookup is organization-scoped: a stored UUID is not itself proof of
// tenancy. cache is an optional per-response memo; pass nil for a single item.
func (s *AgentProxyService) resolveProjectHandle(orgUUID, projectUUID string, cache map[string]string) (string, error) {
	uuid := strings.TrimSpace(projectUUID)
	if uuid == "" {
		return "", nil
	}
	if handle, ok := cache[uuid]; ok {
		return handle, nil
	}
	if s.projectRepo == nil {
		return "", fmt.Errorf("cannot resolve project handle: project repository unavailable")
	}
	project, err := s.projectRepo.GetProjectByUUIDAndOrgID(uuid, orgUUID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve project: %w", err)
	}
	if project == nil {
		return "", apperror.ProjectNotFound.New()
	}
	if cache != nil {
		cache[uuid] = project.Handle
	}
	return project.Handle, nil
}

// resolveNewHandle settles the public handle of an Agent proxy being created:
// the caller's own, or one derived from the display name. Either way it must be
// syntactically valid, unreserved, and free within the organization.
func (s *AgentProxyService) resolveNewHandle(orgUUID string, req *api.A2AAgentProxy) (string, error) {
	if req.Id != nil && strings.TrimSpace(*req.Id) != "" {
		handle := strings.TrimSpace(*req.Id)
		if err := utils.ValidateHandle(handle); err != nil {
			return "", err
		}
		if err := ensureAgentProxyHandleNotReserved(handle); err != nil {
			return "", err
		}
		exists, err := s.repo.Exists(handle, orgUUID)
		if err != nil {
			return "", fmt.Errorf("failed to check agent proxy exists: %w", err)
		}
		if exists {
			return "", apperror.AgentProxyExists.New()
		}
		return handle, nil
	}

	// A generated handle competes for the same namespace, so a reserved candidate
	// counts as taken and generation moves on to a suffixed one.
	handle, err := utils.GenerateHandle(req.DisplayName, func(candidate string) bool {
		if _, reserved := reservedAgentProxyHandles[candidate]; reserved {
			return true
		}
		exists, _ := s.repo.Exists(candidate, orgUUID)
		return exists
	})
	if err != nil {
		return "", err
	}
	return handle, nil
}

// validateSecretRefs checks every {{ secret "handle" }} placeholder anywhere in
// the configuration resolves in this organization. The whole document is scanned
// rather than upstream.auth alone, because the gateway-controller's template
// engine resolves placeholders generically across the artifact.
func (s *AgentProxyService) validateSecretRefs(orgUUID string, configuration model.AgentProxyConfiguration) error {
	if s.secretService == nil {
		return nil
	}
	configJSON, err := marshalUpstreamForValidation(configuration)
	if err != nil {
		return fmt.Errorf("failed to marshal agent proxy configuration for secret validation: %w", err)
	}
	return s.secretService.ValidateSecretRefs(orgUUID, configJSON)
}

// mapRepositoryError translates the Agent proxy repository's own failures onto
// the catalog. Anything it does not recognize stays wrapped for the mapper to
// log and serve as a generic 500.
func (s *AgentProxyService) mapRepositoryError(err error, logMsg string) error {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return apperror.AgentProxyNotFound.Wrap(err)
	case errors.Is(err, repository.ErrAgentProxyProtocolImmutable):
		return apperror.ValidationFailed.Wrap(err, "The protocol of an Agent proxy cannot be changed.")
	case errors.Is(err, repository.ErrAgentProxyProjectOrgMismatch):
		return apperror.ProjectRefNotFound.Wrap(err)
	case isSQLiteUniqueConstraint(err):
		// A handle that passed the pre-check can still lose a race to a concurrent
		// create; the database's uniqueness is the authority, and it is a conflict
		// rather than an internal failure.
		return apperror.AgentProxyExists.Wrap(err)
	default:
		return fmt.Errorf("%s: %w", logMsg, err)
	}
}

// validateAgentProxyAuthoringFields checks the mandatory authoring fields shared
// by create and replace. The per-rule contract validation (card modes, transport
// and operation names, card size) is the validation section's own work; what is
// here is what the persisted model cannot be built without.
func validateAgentProxyAuthoringFields(req *api.A2AAgentProxy) error {
	if strings.TrimSpace(req.DisplayName) == "" {
		return apperror.ValidationFailed.New("The displayName field is required.")
	}
	if strings.TrimSpace(req.Version) == "" {
		return apperror.ValidationFailed.New("The version field is required.")
	}
	if req.Kind != nil && *req.Kind != api.A2AAgentProxyKindAgentProxy {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The kind field must be %q.", string(api.A2AAgentProxyKindAgentProxy)))
	}
	if !model.IsSupportedAgentProxyProtocol(model.AgentProxyProtocol(req.Protocol)) {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The protocol %q is not supported. Supported protocols: %s.",
				string(req.Protocol), strings.Join(model.SupportedAgentProxyProtocols(), ", ")))
	}
	if upstreamEndpointTarget(req.Upstream.Main) == "" {
		return apperror.ValidationFailed.New("The upstream main url or ref field is required.")
	}
	if strings.TrimSpace(string(req.A2a.ProtocolVersion)) == "" {
		return apperror.ValidationFailed.New("The a2a.protocolVersion field is required.")
	}
	if len(req.A2a.Transports) == 0 {
		return apperror.ValidationFailed.New("At least one a2a.transports entry is required.")
	}
	return nil
}

// validateEffectiveUpstreamAuth rejects an upstream auth block that names a
// credential-bearing type but carries no credential.
//
// It runs on the *effective* configuration — on update that means after
// credential retention, which is what separates the two cases that look alike
// on the wire. Responses redact the credential, so an unchanged auth block
// always arrives without one and inherits the stored value; a changed block
// (different type, or a different header to send the credential in) inherits
// nothing by design, and without this check it would be persisted with an empty
// value, silently erasing the credential while returning 200.
//
// Errors name the endpoint and nothing else: no handle, no stored value, no
// resolved secret.
func validateEffectiveUpstreamAuth(cfg *model.UpstreamConfig) error {
	if cfg == nil {
		return nil
	}
	if err := validateEndpointAuthComplete(cfg.Main, "main"); err != nil {
		return err
	}
	return validateEndpointAuthComplete(cfg.Sandbox, "sandbox")
}

func validateEndpointAuthComplete(endpoint *model.UpstreamEndpoint, name string) error {
	if endpoint == nil || endpoint.Auth == nil {
		return nil
	}
	// "none" is the documented way to remove authentication, so it is the one
	// type that is complete without a credential.
	if endpoint.Auth.Type == string(api.None) {
		return nil
	}
	if strings.TrimSpace(endpoint.Auth.Value) == "" {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The upstream %s auth configuration requires a credential value. "+
				"An omitted value only carries the stored credential forward when the auth "+
				"configuration is otherwise unchanged.", name))
	}
	return nil
}

// upstreamEndpointTarget returns whichever of url/ref the endpoint carries, or
// "" when it carries neither. The schema permits exactly one; rejecting both at
// once belongs to the validation section.
func upstreamEndpointTarget(in api.UpstreamDefinition) string {
	if in.Url != nil && strings.TrimSpace(*in.Url) != "" {
		return strings.TrimSpace(*in.Url)
	}
	if in.Ref != nil && strings.TrimSpace(*in.Ref) != "" {
		return strings.TrimSpace(*in.Ref)
	}
	return ""
}

func ensureAgentProxyHandleNotReserved(handle string) error {
	if _, reserved := reservedAgentProxyHandles[handle]; reserved {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The id %q is reserved and cannot be used for an Agent proxy.", handle))
	}
	return nil
}

// parseAgentProxyProtocolFilter validates the optional list filter. An omitted
// parameter (nil) means every protocol; a supplied empty or unsupported value is
// a 400 rather than a silently ignored filter, which would return rows the
// caller did not ask for.
func parseAgentProxyProtocolFilter(protocol *string) (model.AgentProxyProtocol, error) {
	if protocol == nil {
		return "", nil
	}
	if !model.IsSupportedAgentProxyProtocol(model.AgentProxyProtocol(*protocol)) {
		return "", apperror.ValidationFailed.New(
			fmt.Sprintf("The protocol filter %q is not supported. Supported protocols: %s.",
				*protocol, strings.Join(model.SupportedAgentProxyProtocols(), ", ")))
	}
	return model.AgentProxyProtocol(*protocol), nil
}
