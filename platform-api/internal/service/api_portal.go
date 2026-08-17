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
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// validateAPIPortalURL enforces input-time constraints on a caller-supplied
// portal URL:
//   - Empty is valid — the URL is populated later by the provisioner in the
//     cloud flow, and OSS may register a portal before the URL is known.
//   - Non-empty must parse as an absolute URL with a host, and use the https
//     scheme. This blocks stored SSRF via `file://`, `javascript:`, and any
//     plain-http URL that could be pointed at instance-metadata endpoints such
//     as http://169.254.169.254/.
//
// Deeper outbound-hardening (private-IP blocklist, DNS-rebinding checks,
// redirect controls) is intentionally NOT enforced here — it belongs in the
// shared outbound HTTP client the publisher will build later, so every
// outbound integration gets the same protection uniformly.
func validateAPIPortalURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", apperror.ValidationFailed.New("The url field is not a valid URL.")
	}
	if !u.IsAbs() || u.Host == "" {
		return "", apperror.ValidationFailed.New("The url field must be an absolute URL with a host.")
	}
	if u.Scheme != "https" {
		return "", apperror.ValidationFailed.New("The url field must use the https scheme.")
	}
	return u.String(), nil
}

// APIPortalService encapsulates business logic for the /api-portals resource.
// The handler layer translates OpenAPI-generated request/response DTOs into
// the service's own request structs so the service stays independent of the
// generated code.
type APIPortalService struct {
	portalRepo repository.APIPortalRepository
	orgRepo    repository.OrganizationRepository
	auditRepo  repository.AuditRepository
	identity   *IdentityService
	slogger    *slog.Logger
}

// NewAPIPortalService constructs an APIPortalService.
func NewAPIPortalService(
	portalRepo repository.APIPortalRepository,
	orgRepo repository.OrganizationRepository,
	auditRepo repository.AuditRepository,
	identity *IdentityService,
	slogger *slog.Logger,
) *APIPortalService {
	return &APIPortalService{
		portalRepo: portalRepo,
		orgRepo:    orgRepo,
		auditRepo:  auditRepo,
		identity:   identity,
		slogger:    slogger,
	}
}

// CreateAPIPortalRequest is the service-layer input for creating an API Portal.
// Fields mirror the OpenAPI CreateApiPortalRequest but stay independent of the
// generated types.
type CreateAPIPortalRequest struct {
	Handle         string
	Name           string
	Description    string
	URL            string
	WorkflowStatus string // optional; defaults to "pending"
	AuthType       string
	Configuration  map[string]interface{}
}

// UpdateAPIPortalRequest carries mutable fields for a partial update. Pointer
// fields distinguish "not sent" (nil) from "sent as empty" (non-nil, empty).
// Only whitelisted fields are respected here; Handle, ID, OrganizationID,
// CreatedAt, CreatedBy are ignored per the design's immutability rules.
type UpdateAPIPortalRequest struct {
	Name           *string
	Description    *string
	URL            *string
	WorkflowStatus *string
	AuthType       *string
	Configuration  map[string]interface{} // when nil, the existing configuration is preserved
}

// APIPortalListOptions bundles the pagination + filter inputs for List.
type APIPortalListOptions struct {
	repository.ListOptions
	WorkflowStatus *string
}

// APIPortalListResponse is the service-layer list result. The handler wraps
// this in the OpenAPI-generated envelope.
type APIPortalListResponse struct {
	Count      int
	List       []*model.APIPortal
	Pagination PaginationInfo
}

// PaginationInfo is the {total, offset, limit} triplet returned in list responses.
type PaginationInfo struct {
	Total  int
	Offset int
	Limit  int
}

// CreateAPIPortal validates the request, enforces uniqueness of the handle,
// and inserts a new row scoped to orgID.
func (s *APIPortalService) CreateAPIPortal(req *CreateAPIPortalRequest, orgID, createdBy string) (*model.APIPortal, error) {
	if req == nil {
		return nil, apperror.ValidationFailed.New("The request body is required.")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, apperror.ValidationFailed.New("The name field is required.")
	}
	if err := utils.ValidateHandle(strings.TrimSpace(req.Handle)); err != nil {
		return nil, err
	}
	authType := strings.TrimSpace(req.AuthType)
	if !constants.ValidAPIPortalAuthTypes[authType] {
		return nil, apperror.ValidationFailed.New(
			fmt.Sprintf("The authType %q is not supported.", authType))
	}
	workflowStatus := strings.TrimSpace(req.WorkflowStatus)
	if workflowStatus == "" {
		workflowStatus = constants.APIPortalWorkflowStatusPending
	} else if !constants.ValidAPIPortalWorkflowStatuses[workflowStatus] {
		return nil, apperror.ValidationFailed.New(
			fmt.Sprintf("The workflowStatus %q is not supported.", workflowStatus))
	}
	portalURL, err := validateAPIPortalURL(req.URL)
	if err != nil {
		return nil, err
	}

	org, err := s.orgRepo.GetOrganizationByUUID(orgID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, apperror.OrganizationNotFound.New()
	}

	exists, err := s.portalRepo.Exists(strings.TrimSpace(req.Handle), orgID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperror.APIPortalExists.New()
	}

	actor := strings.TrimSpace(createdBy)
	portal := &model.APIPortal{
		ID:             uuid.New().String(),
		OrganizationID: orgID,
		Handle:         strings.TrimSpace(req.Handle),
		Name:           name,
		Description:    strings.TrimSpace(req.Description),
		URL:            portalURL,
		WorkflowStatus: workflowStatus,
		AuthType:       authType,
		Configuration:  req.Configuration,
		CreatedBy:      actor,
		UpdatedBy:      actor,
	}

	if err := s.portalRepo.Create(portal); err != nil {
		if repository.IsUniqueViolation(err) {
			// A concurrent create won the race between Exists and INSERT.
			return nil, apperror.APIPortalExists.New()
		}
		return nil, err
	}
	_ = s.auditRepo.Record("CREATE", portal.ID, "api_portal", orgID, actor)
	return portal, nil
}

// GetAPIPortal returns a single API Portal identified by its handle (wire ID) within orgID.
func (s *APIPortalService) GetAPIPortal(handle, orgID string) (*model.APIPortal, error) {
	portal, err := s.portalRepo.GetByHandleAndOrgID(strings.TrimSpace(handle), orgID)
	if err != nil {
		return nil, err
	}
	if portal == nil {
		return nil, apperror.APIPortalNotFound.New()
	}
	return portal, nil
}

// ListAPIPortals returns a page of API Portals in the organization, honoring
// the requested pagination + filter options. Limit/Offset are normalized here.
func (s *APIPortalService) ListAPIPortals(orgID string, opts APIPortalListOptions) (*APIPortalListResponse, error) {
	org, err := s.orgRepo.GetOrganizationByUUID(orgID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, apperror.OrganizationNotFound.New()
	}
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.Limit > 100 {
		opts.Limit = 100
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	if opts.WorkflowStatus != nil {
		trimmed := strings.TrimSpace(*opts.WorkflowStatus)
		if trimmed == "" {
			opts.WorkflowStatus = nil
		} else if !constants.ValidAPIPortalWorkflowStatuses[trimmed] {
			return nil, apperror.ValidationFailed.New(
				fmt.Sprintf("The workflowStatus %q is not supported.", trimmed))
		} else {
			opts.WorkflowStatus = &trimmed
		}
	}

	total, err := s.portalRepo.Count(orgID, opts.WorkflowStatus, opts.Search)
	if err != nil {
		return nil, err
	}
	page, err := s.portalRepo.ListPaginated(orgID, opts.WorkflowStatus, opts.ListOptions)
	if err != nil {
		return nil, err
	}
	return &APIPortalListResponse{
		Count:      len(page),
		List:       page,
		Pagination: PaginationInfo{Total: total, Offset: opts.Offset, Limit: opts.Limit},
	}, nil
}

// UpdateAPIPortal loads the row, applies only the whitelisted mutations from req,
// persists the change, and returns the updated row.
func (s *APIPortalService) UpdateAPIPortal(handle string, req *UpdateAPIPortalRequest, orgID, updatedBy string) (*model.APIPortal, error) {
	if req == nil {
		return nil, apperror.ValidationFailed.New("The request body is required.")
	}
	portal, err := s.portalRepo.GetByHandleAndOrgID(strings.TrimSpace(handle), orgID)
	if err != nil {
		return nil, err
	}
	if portal == nil {
		return nil, apperror.APIPortalNotFound.New()
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, apperror.ValidationFailed.New("The name field cannot be empty.")
		}
		portal.Name = name
	}
	if req.Description != nil {
		portal.Description = strings.TrimSpace(*req.Description)
	}
	if req.URL != nil {
		portalURL, err := validateAPIPortalURL(*req.URL)
		if err != nil {
			return nil, err
		}
		portal.URL = portalURL
	}
	if req.WorkflowStatus != nil {
		ws := strings.TrimSpace(*req.WorkflowStatus)
		if !constants.ValidAPIPortalWorkflowStatuses[ws] {
			return nil, apperror.ValidationFailed.New(
				fmt.Sprintf("The workflowStatus %q is not supported.", ws))
		}
		portal.WorkflowStatus = ws
	}
	if req.AuthType != nil {
		at := strings.TrimSpace(*req.AuthType)
		if !constants.ValidAPIPortalAuthTypes[at] {
			return nil, apperror.ValidationFailed.New(
				fmt.Sprintf("The authType %q is not supported.", at))
		}
		portal.AuthType = at
	}
	if req.Configuration != nil {
		portal.Configuration = req.Configuration
	}
	portal.UpdatedBy = strings.TrimSpace(updatedBy)

	if err := s.portalRepo.Update(portal); err != nil {
		return nil, err
	}
	_ = s.auditRepo.Record("UPDATE", portal.ID, "api_portal", orgID, portal.UpdatedBy)
	return portal, nil
}

// DeleteAPIPortal removes the API Portal identified by its handle, org-scoped.
func (s *APIPortalService) DeleteAPIPortal(handle, orgID, actor string) error {
	portal, err := s.portalRepo.GetByHandleAndOrgID(strings.TrimSpace(handle), orgID)
	if err != nil {
		return err
	}
	if portal == nil {
		return apperror.APIPortalNotFound.New()
	}
	if err := s.portalRepo.Delete(portal.ID, orgID); err != nil {
		return err
	}
	_ = s.auditRepo.Record("DELETE", portal.ID, "api_portal", orgID, strings.TrimSpace(actor))
	return nil
}
