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
	"strings"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
)

// trafficLogTimestampFormat is RFC 3339 with millisecond precision.
const trafficLogTimestampFormat = "2006-01-02T15:04:05.000Z07:00"

// TrafficLogEvent is the JSON shape written to stdout by the Log publisher.
// It is intentionally separate from dto.Event (shaped for Moesif) so its field
// names, schema, and presence rules can evolve independently. All string fields
// carry omitempty so absent or unknown values produce no key rather than "".
type TrafficLogEvent struct {
	Timestamp       string                 `json:"timestamp,omitempty"`
	CorrelationID   string                 `json:"correlationId,omitempty"`
	Status          int                    `json:"status,omitempty"`
	API             *TrafficLogAPI         `json:"api,omitempty"`
	Operation       *TrafficLogOperation   `json:"operation,omitempty"`
	Target          *TrafficLogTarget      `json:"target,omitempty"`
	Application     *TrafficLogApplication `json:"application,omitempty"`
	Client          *TrafficLogClient        `json:"client,omitempty"`
	Latencies       *dto.TrafficLogLatencies `json:"latencies,omitempty"`
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

// toTrafficLogEvent translates a dto.Event and its traffic-log directive into the
// traffic-log-specific output shape, applying per-flow header filtering, global
// header masking, and payload truncation.
func (l *Log) toTrafficLogEvent(event *dto.Event, dir *dto.TrafficLogDirective) *TrafficLogEvent {
	tl := &TrafficLogEvent{
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
		tl.API = &TrafficLogAPI{
			ID:        event.API.APIID,
			Name:      event.API.APIName,
			Version:   event.API.APIVersion,
			Context:   event.API.APIContext,
			Kind:      event.API.APIType,
			ProjectID: event.API.ProjectID,
		}
	}

	if event.Operation != nil {
		tl.Operation = &TrafficLogOperation{
			Method: event.Operation.APIMethod,
			Path:   event.Operation.APIResourceTemplate,
		}
	}

	if event.Target != nil {
		tl.Target = &TrafficLogTarget{
			StatusCode:  event.Target.TargetResponseCode,
			Destination: event.Target.Destination,
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

	// When a fields selection is configured it is authoritative over presence:
	// the per-flow Headers/Payload booleans are ignored and applyFieldsProjection
	// in Publish decides what survives. Global masking always applies; per-API
	// maskedHeaders are merged in here and passed down.
	hasFieldsSelection := dir.Fields != nil && (len(dir.Fields.Only) > 0 || len(dir.Fields.Exclude) > 0)

	// Effective header mask: global config merged with any per-API additions.
	mask := l.maskedHeaders
	if len(dir.MaskedHeaders) > 0 {
		merged := make(map[string]bool, len(l.maskedHeaders)+len(dir.MaskedHeaders))
		for k, v := range l.maskedHeaders {
			merged[k] = v
		}
		for _, h := range dir.MaskedHeaders {
			merged[strings.ToLower(h)] = true
		}
		mask = merged
	}

	// Request flow
	if raw, ok := event.Properties[dto.PropKeyRequestHeaders].(string); ok {
		if hasFieldsSelection || (dir.Request != nil && dir.Request.Headers) {
			if headers := parseHeadersFromString(raw); headers != nil {
				masked := l.maskHeaders(headers, mask)
				if dir.Request != nil {
					dropHeaders(masked, dir.Request.ExcludeHeaders)
				}
				tl.RequestHeaders = masked
			}
		}
	}
	if p, ok := event.Properties[dto.PropKeyRequestPayload].(string); ok && p != "" {
		if hasFieldsSelection || (dir.Request != nil && dir.Request.Payload) {
			tl.RequestBody = l.truncatePayload(p)
		}
	}

	// Response flow
	if raw, ok := event.Properties[dto.PropKeyResponseHeaders].(string); ok {
		if hasFieldsSelection || (dir.Response != nil && dir.Response.Headers) {
			if headers := parseHeadersFromString(raw); headers != nil {
				masked := l.maskHeaders(headers, mask)
				if dir.Response != nil {
					dropHeaders(masked, dir.Response.ExcludeHeaders)
				}
				tl.ResponseHeaders = masked
			}
		}
	}
	if p, ok := event.Properties[dto.PropKeyResponsePayload].(string); ok && p != "" {
		if hasFieldsSelection || (dir.Response != nil && dir.Response.Payload) {
			tl.ResponseBody = l.truncatePayload(p)
		}
	}

	if len(dir.Properties) > 0 {
		tl.Properties = dir.Properties
	}

	return tl
}
