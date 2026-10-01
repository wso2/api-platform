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

package publishers

import (
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
)

// trafficLogTimestampFormat is RFC 3339 with millisecond precision.
const trafficLogTimestampFormat = "2006-01-02T15:04:05.000Z07:00"

const trafficLogComponent = "pol"

// TrafficLogEvent is the JSON shape written to stdout by the Log publisher.
// It is intentionally separate from dto.Event (shaped for Moesif) so its field
// names, schema, and presence rules can evolve independently. All string fields
// carry omitempty so absent or unknown values produce no key rather than "".
// The API/Operation/Target pointer fields are additionally left nil (dropped
// entirely, rather than emitted as "{}") when every one of their own fields
// resolves to its zero value — see toTrafficLogEvent.
type TrafficLogEvent struct {
	// Component names the emitting process, not the record type: the policy
	// engine's application logs carry the same value.
	Component     string                   `json:"component,omitempty"`
	Timestamp     string                   `json:"timestamp,omitempty"`
	CorrelationID string                   `json:"correlationId,omitempty"`
	Status        int                      `json:"status,omitempty"`
	API           *TrafficLogAPI           `json:"api,omitempty"`
	Operation     *TrafficLogOperation     `json:"operation,omitempty"`
	Target        *TrafficLogTarget        `json:"target,omitempty"`
	Application   *TrafficLogApplication   `json:"application,omitempty"`
	Client        *TrafficLogClient        `json:"client,omitempty"`
	Latencies     *dto.TrafficLogLatencies `json:"latencies,omitempty"`
	// ErrorType and Error describe the failure, and are absent on a successful request.
	//
	// Structural fields like these are always on, guarded only by presence — the per-flow
	// Headers/Payload booleans gate the two things that carry caller content, and the
	// failure classification is not one of them: the collector deliberately never stamps
	// the fault's Description or a guardrail's Assessments, which are the only parts of a
	// failure that could hold blocked content. An operator who still does not want them
	// has fields.exclude, which takes "error" or a dotted path like "error.summary".
	//
	// Shaped as a sibling pair rather than one nested object because that is what
	// dto.Event carries: errorType is the flat category field. Mirroring the
	// canonical event keeps the two outputs comparable and avoids a third error struct to
	// keep in step.
	ErrorType       string                 `json:"errorType,omitempty"`
	Error           *dto.Error             `json:"error,omitempty"`
	RequestHeaders  map[string]string      `json:"requestHeaders,omitempty"`
	ResponseHeaders map[string]string      `json:"responseHeaders,omitempty"`
	RequestBody     string                 `json:"requestBody,omitempty"`
	ResponseBody    string                 `json:"responseBody,omitempty"`
	Properties      map[string]interface{} `json:"properties,omitempty"`
}

// TrafficLogAPI identifies the API that processed the request.
type TrafficLogAPI struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Version   string `json:"version,omitempty"`
	Context   string `json:"context,omitempty"`
	Kind      string `json:"kind,omitempty"`
	ProjectID string `json:"projectId,omitempty"`
}

// TrafficLogOperation describes the matched operation within the API.
type TrafficLogOperation struct {
	Method string `json:"method,omitempty"`
	Path   string `json:"path,omitempty"`
}

// TrafficLogTarget holds upstream response information.
type TrafficLogTarget struct {
	StatusCode  int    `json:"statusCode,omitempty"`
	Destination string `json:"destination,omitempty"`
}

// TrafficLogApplication is present only for authenticated requests.
type TrafficLogApplication struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Owner   string `json:"owner,omitempty"`
	KeyType string `json:"keyType,omitempty"`
}

// TrafficLogClient holds downstream caller information.
type TrafficLogClient struct {
	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`
}

// toTrafficLogEvent translates a dto.Event and its directive into the
// traffic-log-specific output shape, applying per-flow header filtering, header
// masking, and payload truncation.
func (l *Log) toTrafficLogEvent(event *dto.Event, dir *dto.TrafficLogDirective) *TrafficLogEvent {
	tl := &TrafficLogEvent{
		Component: trafficLogComponent,
		Status:    event.ProxyResponseCode,
		Latencies: event.TrafficLogLatencies,
	}

	if !event.RequestTimestamp.IsZero() {
		tl.Timestamp = event.RequestTimestamp.UTC().Format(trafficLogTimestampFormat)
	}

	if event.MetaInfo != nil {
		tl.CorrelationID = event.MetaInfo.CorrelationID
	}

	if event.API != nil {
		api := TrafficLogAPI{
			ID:        event.API.APIID,
			Name:      event.API.APIName,
			Version:   event.API.APIVersion,
			Context:   event.API.APIContext,
			Kind:      event.API.APIType,
			ProjectID: event.API.ProjectID,
		}
		if api != (TrafficLogAPI{}) {
			tl.API = &api
		}
	}

	if event.Operation != nil {
		op := TrafficLogOperation{
			Method: event.Operation.APIMethod,
			Path:   event.Operation.APIResourceTemplate,
		}
		if op != (TrafficLogOperation{}) {
			tl.Operation = &op
		}
	}

	if event.Target != nil {
		target := TrafficLogTarget{
			StatusCode:  event.Target.TargetResponseCode,
			Destination: event.Target.Destination,
		}
		if target != (TrafficLogTarget{}) {
			tl.Target = &target
		}
	}

	// Application is only meaningful for authenticated requests.
	if a := event.Application; a != nil && (a.ApplicationID != "" || a.ApplicationName != "") {
		tl.Application = &TrafficLogApplication{
			ID:      a.ApplicationID,
			Name:    a.ApplicationName,
			Owner:   a.ApplicationOwner,
			KeyType: a.KeyType,
		}
	}

	if event.UserIP != "" || event.UserAgentHeader != "" {
		tl.Client = &TrafficLogClient{
			IP:        event.UserIP,
			UserAgent: event.UserAgentHeader,
		}
	}

	// Shared, not copied: dto.Error is treated as immutable once prepareAnalyticEvent has
	// built it, and every publisher gets the same pointer. Nothing downstream mutates it,
	// and the alternative — a per-publisher copy — would silently diverge the moment a
	// field is added to dto.Error.
	tl.ErrorType = event.ErrorType
	tl.Error = event.Error

	// fields.exclude only trims fields/sub-keys that the per-flow Headers/Payload
	// booleans below already turned on — it is a subtractive projection over the
	// enabled set, never an independent "log everything except X" switch. Setting
	// exclude_fields alone, with every request_*/response_* toggle left at its
	// false default, still logs no headers/bodies.
	if dir.Request != nil && dir.Request.Headers {
		if headers := headersFromEventProperty(event.Properties[dto.PropKeyRequestHeaders]); headers != nil {
			tl.RequestHeaders = maskHeaders(headers, l.maskedHeaders)
		}
	}
	if p, ok := event.Properties[dto.PropKeyRequestPayload].(string); ok && p != "" && dir.Request != nil && dir.Request.Payload {
		tl.RequestBody = l.truncatePayload(p)
	}

	// Response flow
	if dir.Response != nil && dir.Response.Headers {
		if headers := headersFromEventProperty(event.Properties[dto.PropKeyResponseHeaders]); headers != nil {
			tl.ResponseHeaders = maskHeaders(headers, l.maskedHeaders)
		}
	}
	if p, ok := event.Properties[dto.PropKeyResponsePayload].(string); ok && p != "" && dir.Response != nil && dir.Response.Payload {
		tl.ResponseBody = l.truncatePayload(p)
	}

	if len(dir.Properties) > 0 {
		tl.Properties = dir.Properties
	}

	return tl
}
