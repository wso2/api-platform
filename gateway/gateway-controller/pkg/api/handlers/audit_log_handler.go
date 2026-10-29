/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 */

package handlers

import (
	"encoding/json"
	"net/http"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/httpkit/httputil"
)

const (
	auditDefaultLimit = 20
	auditMaxLimit     = 100
)

// ListAuditLogs implements ServerInterface.ListAuditLogs (GET /audit-logs).
// Results are always scoped to the store's gateway_id — no caller-supplied filter.
func (s *APIServer) ListAuditLogs(w http.ResponseWriter, r *http.Request, params api.ListAuditLogsParams) {
	filter, limit, offset, ok := s.parseAuditParams(w, r, params)
	if !ok {
		return
	}

	total, err := s.db.CountAuditLogs(filter)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to count audit log entries",
		})
		return
	}

	rows, err := s.db.ListAuditLogs(filter, limit, offset)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to retrieve audit log entries",
		})
		return
	}

	events := make([]api.AuditEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, toAPIAuditEvent(row))
	}

	httputil.WriteJSON(w, http.StatusOK, api.AuditEventListResponse{
		Count:      len(events),
		Events:     events,
		Pagination: api.Pagination{Total: total, Offset: offset, Limit: limit},
	})
}

// parseAuditParams validates and extracts query parameters for ListAuditLogs.
// Writes an error response and returns false on validation failure.
func (s *APIServer) parseAuditParams(w http.ResponseWriter, r *http.Request, params api.ListAuditLogsParams) (storage.AuditLogFilter, int, int, bool) {
	var filter storage.AuditLogFilter

	// Time range
	if params.StartTime != nil {
		t := params.StartTime.UTC()
		filter.StartTime = &t
	}
	if params.EndTime != nil {
		t := params.EndTime.UTC()
		filter.EndTime = &t
	}
	if filter.StartTime != nil && filter.EndTime != nil && !filter.EndTime.After(*filter.StartTime) {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "endTime must be after startTime",
		})
		return storage.AuditLogFilter{}, 0, 0, false
	}

	// Enum filters: values are already constrained by the spec enum; validate
	// defensively in case the middleware binding is bypassed in tests.
	if params.Action != nil {
		switch *params.Action {
		case api.AuditActionCreate, api.AuditActionUpdate, api.AuditActionDelete, api.AuditActionRevoke:
			filter.Action = string(*params.Action)
		default:
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: "invalid action filter; must be one of: create, update, delete, revoke",
			})
			return storage.AuditLogFilter{}, 0, 0, false
		}
	}

	if params.Outcome != nil {
		switch *params.Outcome {
		case api.AuditOutcomeSucceeded, api.AuditOutcomeFailed, api.AuditOutcomeDenied:
			filter.Outcome = string(*params.Outcome)
		default:
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: "invalid outcome filter; must be one of: succeeded, failed, denied",
			})
			return storage.AuditLogFilter{}, 0, 0, false
		}
	}

	if params.ResourceType != nil {
		switch *params.ResourceType {
		case api.AuditResourceTypeRestApi,
			api.AuditResourceTypeGraphQLApi,
			api.AuditResourceTypeLlmProvider,
			api.AuditResourceTypeLlmProviderTemplate,
			api.AuditResourceTypeLlmProxy,
			api.AuditResourceTypeMcpProxy,
			api.AuditResourceTypeAgent,
			api.AuditResourceTypeApiKey,
			api.AuditResourceTypeSubscriptionPlan,
			api.AuditResourceTypeSubscription,
			api.AuditResourceTypeCertificate,
			api.AuditResourceTypeSecret:
			filter.ResourceType = string(*params.ResourceType)
		default:
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: "invalid resourceType filter",
			})
			return storage.AuditLogFilter{}, 0, 0, false
		}
	}

	// Free-text filters (length-capped)
	if params.PerformedBy != nil {
		if len(*params.PerformedBy) > 200 {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: "performedBy exceeds maximum length of 200 characters",
			})
			return storage.AuditLogFilter{}, 0, 0, false
		}
		filter.PerformedBy = *params.PerformedBy
	}
	if params.ResourceId != nil {
		if len(*params.ResourceId) > 40 {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: "resourceId exceeds maximum length of 40 characters",
			})
			return storage.AuditLogFilter{}, 0, 0, false
		}
		filter.ResourceUUID = *params.ResourceId
	}

	// Pagination
	limit := auditDefaultLimit
	if params.Limit != nil {
		limit = int(*params.Limit)
		if limit <= 0 {
			limit = auditDefaultLimit
		} else if limit > auditMaxLimit {
			limit = auditMaxLimit
		}
	}
	offset := 0
	if params.Offset != nil {
		offset = int(*params.Offset)
		if offset < 0 {
			offset = 0
		}
	}

	return filter, limit, offset, true
}

// toAPIAuditEvent converts a storage model row to the generated API type.
func toAPIAuditEvent(row *models.AuditLog) api.AuditEvent {
	e := api.AuditEvent{
		Id:           row.UUID,
		Action:       api.AuditAction(row.Action),
		ResourceType: api.AuditResourceType(row.ResourceType),
		Outcome:      api.AuditOutcome(row.Outcome),
		PerformedAt:  row.PerformedAt,
	}
	if row.CorrelationID != "" {
		e.CorrelationId = &row.CorrelationID
	}
	if row.ResourceUUID != "" {
		e.ResourceId = &row.ResourceUUID
	}
	if row.ResourceName != "" {
		e.ResourceName = &row.ResourceName
	}
	if row.PerformedBy != "" {
		e.PerformedBy = &row.PerformedBy
	}
	if row.EventData != "" {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(row.EventData), &m); err == nil {
			e.EventData = &m
		}
	}
	return e
}
