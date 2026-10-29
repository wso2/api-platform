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

package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"

	"github.com/wso2/api-platform/httpkit/httputil"
)

// AuditLogHandler handles audit log reads.
type AuditLogHandler struct {
	auditLogRepo repository.AuditLogRepository
	identity     *service.IdentityService
	slogger      *slog.Logger
}

func NewAuditLogHandler(auditLogRepo repository.AuditLogRepository, identity *service.IdentityService, slogger *slog.Logger) *AuditLogHandler {
	return &AuditLogHandler{auditLogRepo: auditLogRepo, identity: identity, slogger: slogger}
}

// ListAuditLogs handles GET /api/v0.9/audit-logs
func (h *AuditLogHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	q := r.URL.Query()
	filter := repository.AuditLogFilter{OrganizationUUID: orgID}

	startTime, err := parseAuditTime(r, "startTime")
	if err != nil {
		return err
	}
	endTime, err := parseAuditTime(r, "endTime")
	if err != nil {
		return err
	}
	if startTime != nil && endTime != nil && !endTime.After(*startTime) {
		return apperror.AuditInvalidFilter.New("endTime must be strictly after startTime")
	}
	filter.StartTime, filter.EndTime = startTime, endTime

	if v := strings.TrimSpace(q.Get("action")); v != "" {
		switch v {
		case model.AuditActionCreate, model.AuditActionUpdate, model.AuditActionDelete,
			model.AuditActionDeploy, model.AuditActionUndeploy, model.AuditActionRestore,
			model.AuditActionRevoke:
		default:
			return apperror.AuditInvalidFilter.New(fmt.Sprintf("unknown action %q", v))
		}
		filter.Action = v
	}
	if v := strings.TrimSpace(q.Get("outcome")); v != "" {
		switch v {
		case model.AuditOutcomeSucceeded, model.AuditOutcomeFailed, model.AuditOutcomeDenied:
		default:
			return apperror.AuditInvalidFilter.New(fmt.Sprintf("unknown outcome %q", v))
		}
		filter.Outcome = v
	}
	for _, v := range q["resourceType"] {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		switch v {
		case model.AuditResourceTypeRestAPI, model.AuditResourceTypeApplication,
			model.AuditResourceTypeLLMProvider, model.AuditResourceTypeLLMAPIKey,
			model.AuditResourceTypeSubscription, model.AuditResourceTypeSubscriptionPlan,
			model.AuditResourceTypeMCP, model.AuditResourceTypeAPIKey,
			model.AuditResourceTypeDeployment,
			model.AuditResourceTypeGateway, model.AuditResourceTypeCustomPolicy,
			model.AuditResourceTypeProject, model.AuditResourceTypeOrganization,
			model.AuditResourceTypeAPIPortal, model.AuditResourceTypeAPIDefinition,
			model.AuditResourceTypeLLMProviderTemplate, model.AuditResourceTypeLLMProxy,
			model.AuditResourceTypeWebSubAPI, model.AuditResourceTypeWebBrokerAPI,
			model.AuditResourceTypeSecret, model.AuditResourceTypeGraphQLAPI,
			model.AuditResourceTypeAgentProxy, model.AuditResourceTypeAPIDocument,
			model.AuditResourceTypeMeter:
		default:
			return apperror.AuditInvalidFilter.New(fmt.Sprintf("unknown resourceType %q", v))
		}
		if !slices.Contains(filter.ResourceTypes, v) {
			filter.ResourceTypes = append(filter.ResourceTypes, v)
		}
	}
	if v := strings.TrimSpace(q.Get("performedBy")); v != "" {
		if len(v) > 200 {
			return apperror.AuditInvalidFilter.New("actor must be at most 200 characters")
		}
		filter.PerformedBy = v
	}
	if v := strings.TrimSpace(q.Get("resourceId")); v != "" {
		if len(v) > 40 {
			return apperror.AuditInvalidFilter.New("resourceId must be at most 40 characters")
		}
		filter.ResourceUUID = v
	}

	limit, offset := parsePagination(r)

	if filter.PerformedBy != "" {
		uuid, err := h.identity.UUIDForSub(filter.PerformedBy)
		if err != nil {
			return serviceError(err, "failed to resolve actor filter")
		}
		if uuid == "" {
			httputil.WriteJSON(w, http.StatusOK, api.AuditEventListResponse{
				List:       []api.AuditEvent{},
				Count:      0,
				Pagination: api.Pagination{Total: 0, Offset: offset, Limit: limit},
			})
			return nil
		}
		filter.PerformedBy = uuid
	}

	total, err := h.auditLogRepo.Count(filter)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to count audit logs for org %s", orgID))
	}
	rows, err := h.auditLogRepo.List(filter, limit, offset)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to list audit logs for org %s", orgID))
	}

	uuids := make([]string, len(rows))
	for i, row := range rows {
		uuids[i] = row.PerformedBy
	}
	performedByMap, err := h.identity.SubsForUUIDs(uuids)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to resolve actor identities for org %s", orgID))
	}

	resp := api.AuditEventListResponse{
		List:       make([]api.AuditEvent, 0, len(rows)),
		Pagination: api.Pagination{Total: total, Offset: offset, Limit: limit},
	}
	for _, row := range rows {
		resp.List = append(resp.List, h.toAuditEvent(row, performedByMap))
	}
	resp.Count = len(resp.List)

	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *AuditLogHandler) toAuditEvent(r *model.AuditLog, performedByMap map[string]string) api.AuditEvent {
	ev := api.AuditEvent{
		Id:             r.UUID,
		OrganizationId: r.OrganizationUUID,
		Action:         api.AuditAction(r.Action),
		ResourceType:   api.AuditResourceType(r.ResourceType),
		PerformedAt:    r.PerformedAt.UTC(),
		Outcome:        api.AuditOutcome(r.Outcome),
	}
	if r.CorrelationID != "" {
		s := r.CorrelationID
		ev.CorrelationId = &s
	}
	if r.ResourceUUID != "" {
		s := r.ResourceUUID
		ev.ResourceId = &s
	}
	if r.ResourceName != "" {
		s := r.ResourceName
		ev.ResourceName = &s
	}
	if sub := performedByMap[r.PerformedBy]; sub != "" && sub != constants.DeletedUser {
		ev.PerformedBy = &sub
	}
	// event_data is stored as JSON; unmarshal so the caller sees a structured
	// object. A failed unmarshal leaves EventData nil — audit truth over payload fidelity.
	if r.EventData != "" {
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(r.EventData), &payload); err == nil {
			ev.EventData = &payload
		}
	}
	return ev
}

func parseAuditTime(r *http.Request, key string) (*time.Time, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, apperror.AuditInvalidFilter.New(fmt.Sprintf("%s is not a valid RFC 3339 timestamp", key))
	}
	utc := t.UTC()
	return &utc, nil
}

func (h *AuditLogHandler) RegisterRoutes(mux router.Router) {
	base := constants.APIBasePath + "/audit-logs"
	mux.HandleFunc("GET "+base, middleware.MapErrors(h.slogger, h.ListAuditLogs))
}
