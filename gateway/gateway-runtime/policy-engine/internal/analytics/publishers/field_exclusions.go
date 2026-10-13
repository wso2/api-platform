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

// trafficLogFieldClearers maps each top-level JSON key of TrafficLogEvent to the
// function that clears it. It is the only list of those keys: compileFieldExclusions
// and applyToStruct both use it, and a test checks it against the struct's JSON
// tags, so a field added to TrafficLogEvent without an entry here fails the build's
// tests instead of being logged despite an exclusion. A key with no entry here is
// still excluded, through the slower JSON projection (residual).
var trafficLogFieldClearers = map[string]func(*TrafficLogEvent){
	"component":       func(tl *TrafficLogEvent) { tl.Component = "" },
	"timestamp":       func(tl *TrafficLogEvent) { tl.Timestamp = "" },
	"correlationId":   func(tl *TrafficLogEvent) { tl.CorrelationID = "" },
	"status":          func(tl *TrafficLogEvent) { tl.Status = 0 },
	"api":             func(tl *TrafficLogEvent) { tl.API = nil },
	"operation":       func(tl *TrafficLogEvent) { tl.Operation = nil },
	"target":          func(tl *TrafficLogEvent) { tl.Target = nil },
	"application":     func(tl *TrafficLogEvent) { tl.Application = nil },
	"client":          func(tl *TrafficLogEvent) { tl.Client = nil },
	"latencies":       func(tl *TrafficLogEvent) { tl.Latencies = nil },
	"requestHeaders":  func(tl *TrafficLogEvent) { tl.RequestHeaders = nil },
	"responseHeaders": func(tl *TrafficLogEvent) { tl.ResponseHeaders = nil },
	"requestBody":     func(tl *TrafficLogEvent) { tl.RequestBody = "" },
	"responseBody":    func(tl *TrafficLogEvent) { tl.ResponseBody = "" },
	"properties":      func(tl *TrafficLogEvent) { tl.Properties = nil },
	"errorType":       func(tl *TrafficLogEvent) { tl.ErrorType = "" },
	"error":           func(tl *TrafficLogEvent) { tl.Error = nil },
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
		case len(parts) == 1 && trafficLogFieldClearers[name] != nil:
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
		trafficLogFieldClearers[name](tl)
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
