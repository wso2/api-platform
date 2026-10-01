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

// fieldExclusions is traffic_logging.exclude_fields compiled once at startup into
// the cheapest form that produces the same output.
//
// Applying exclude_fields to the serialized line costs a marshal, a top-level
// unmarshal, a decode/re-encode of the header object for every dotted header path,
// and a second marshal — per request. Nearly every real exclusion is either a
// top-level key or a single header name, and both can be dropped from the
// TrafficLogEvent before it is ever marshalled. Only the paths that cannot be
// expressed that way (e.g. "properties.claims.internal_debug") are kept in residual
// and still go through the JSON projection, so the projection runs only for
// configurations that actually need it.
type fieldExclusions struct {
	// topLevel holds excluded top-level JSON keys of TrafficLogEvent.
	topLevel map[string]bool
	// requestHeaders/responseHeaders hold lower-cased header names excluded via
	// "requestHeaders.<name>"/"responseHeaders.<name>". Header names are matched
	// case-insensitively, as the JSON projection does (see isHeaderField).
	requestHeaders  map[string]bool
	responseHeaders map[string]bool
	// residual holds the exclusions only the JSON projection can apply. Nil when
	// there are none.
	residual *dto.TrafficLogFields
}

// trafficLogTopLevelKeys lists the JSON keys of TrafficLogEvent that
// applyToStruct knows how to clear. It must be kept in sync with the struct tags.
var trafficLogTopLevelKeys = map[string]bool{
	"component": true, "timestamp": true, "correlationId": true, "status": true,
	"api": true, "operation": true, "target": true, "application": true,
	"client": true, "latencies": true, "requestHeaders": true, "responseHeaders": true,
	"requestBody": true, "responseBody": true, "properties": true,
}

// compileFieldExclusions splits the configured exclude list into struct-level and
// residual exclusions. Returns nil when nothing is excluded.
func compileFieldExclusions(exclude []string) *fieldExclusions {
	if len(exclude) == 0 {
		return nil
	}
	fe := &fieldExclusions{
		topLevel:        map[string]bool{},
		requestHeaders:  map[string]bool{},
		responseHeaders: map[string]bool{},
	}
	var residual []string
	for _, name := range exclude {
		parts := strings.Split(name, ".")
		switch {
		case len(parts) == 1 && trafficLogTopLevelKeys[name]:
			fe.topLevel[name] = true
		case len(parts) == 2 && parts[0] == "requestHeaders":
			fe.requestHeaders[strings.ToLower(parts[1])] = true
		case len(parts) == 2 && parts[0] == "responseHeaders":
			fe.responseHeaders[strings.ToLower(parts[1])] = true
		default:
			residual = append(residual, name)
		}
	}
	if len(residual) > 0 {
		fe.residual = &dto.TrafficLogFields{Exclude: residual}
	}
	return fe
}

// applyToStruct clears the excluded top-level fields of tl. Header sub-key
// exclusions are applied while the header maps are built (see filterAndMaskHeaders).
func (fe *fieldExclusions) applyToStruct(tl *TrafficLogEvent) {
	if fe == nil || len(fe.topLevel) == 0 {
		return
	}
	for name := range fe.topLevel {
		switch name {
		case "component":
			tl.Component = ""
		case "timestamp":
			tl.Timestamp = ""
		case "correlationId":
			tl.CorrelationID = ""
		case "status":
			tl.Status = 0
		case "api":
			tl.API = nil
		case "operation":
			tl.Operation = nil
		case "target":
			tl.Target = nil
		case "application":
			tl.Application = nil
		case "client":
			tl.Client = nil
		case "latencies":
			tl.Latencies = nil
		case "requestHeaders":
			tl.RequestHeaders = nil
		case "responseHeaders":
			tl.ResponseHeaders = nil
		case "requestBody":
			tl.RequestBody = ""
		case "responseBody":
			tl.ResponseBody = ""
		case "properties":
			tl.Properties = nil
		}
	}
}

// excludedRequestHeaders returns the lower-cased request header names to drop, or
// nil when there are none.
func (fe *fieldExclusions) excludedRequestHeaders() map[string]bool {
	if fe == nil || len(fe.requestHeaders) == 0 {
		return nil
	}
	return fe.requestHeaders
}

// excludedResponseHeaders returns the lower-cased response header names to drop,
// or nil when there are none.
func (fe *fieldExclusions) excludedResponseHeaders() map[string]bool {
	if fe == nil || len(fe.responseHeaders) == 0 {
		return nil
	}
	return fe.responseHeaders
}

// residualFields returns the exclusions that still need the JSON projection, or
// nil when there are none.
func (fe *fieldExclusions) residualFields() *dto.TrafficLogFields {
	if fe == nil {
		return nil
	}
	return fe.residual
}
