/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package dto

import "time"

// Property keys written into Event.Properties by the analytics pipeline and read
// back by publishers (e.g. the Log publisher's masking and field-projection paths).
// Both sides must use these constants so a rename stays in one place.
const (
	PropKeyRequestHeaders  = "requestHeaders"
	PropKeyResponseHeaders = "responseHeaders"
	PropKeyRequestPayload  = "request_payload"
	PropKeyResponsePayload = "response_payload"

	// Auth-context property keys. Populated generically (auth-type-agnostic) by the
	// collector system policy (gateway/system-policies/analytics, a separate Go module —
	// hence the matching string literals there rather than a shared Go constant) whenever
	// SharedContext.AuthContext is set by any auth policy, so these are available
	// regardless of jwt-auth/opaque-token-auth/api-key-auth/etc. Key names match the raw
	// analytics metadata 1:1 (no case translation, unlike PropKeyRequestHeaders above), so
	// prepareAnalyticEvent copies them straight into Event.Properties. Consumed by the
	// stdout traffic-logging publisher's global "$ctx:auth.*" properties (see
	// internal/analytics/publishers/global_properties.go).
	PropKeyAuthUserID       = "x-wso2-user-id"
	PropKeyAuthType         = "x-wso2-auth-type"
	PropKeyAuthIssuer       = "x-wso2-auth-issuer"
	PropKeyAuthCredentialID = "x-wso2-auth-credential-id"
	PropKeyAuthTokenID      = "x-wso2-auth-token-id"
	PropKeyAuthAudience     = "x-wso2-auth-audience"
	PropKeyAuthScopes       = "x-wso2-auth-scopes"
	PropKeyAuthProperties   = "x-wso2-auth-properties"
	PropKeyAuthAuthorized   = "x-wso2-auth-authorized"
)

// Event represents analytics event data.
type Event struct {
	API               *ExtendedAPI           `json:"api,omitempty" bson:"api"`
	Operation         *Operation             `json:"operation,omitempty" bson:"operation"`
	Target            *Target                `json:"target,omitempty" bson:"target"`
	Application       *Application           `json:"application,omitempty" bson:"application"`
	Subscription      *Subscription          `json:"subscription,omitempty" bson:"subscription"`
	Latencies         *Latencies             `json:"latencies,omitempty" bson:"latencies"`
	MetaInfo          *MetaInfo              `json:"metaInfo,omitempty" bson:"meta_info"`
	Error             *Error                 `json:"error,omitempty" bson:"error"`
	ProxyResponseCode int                    `json:"proxyResponseCode,omitempty" bson:"proxy_response_code"`
	RequestTimestamp  time.Time              `json:"requestTimestamp,omitempty" bson:"request_timestamp"`
	UserAgentHeader   string                 `json:"userAgentHeader,omitempty" bson:"user_agent_header"`
	UserName          string                 `json:"userName,omitempty" bson:"user_name"`
	UserIP            string                 `json:"userIp,omitempty" bson:"user_ip"`
	ErrorType         string                 `json:"errorType,omitempty" bson:"error_type"`
	Properties        map[string]interface{} `json:"properties,omitempty" bson:"properties"`

	// TrafficLogLatencies carries microsecond-precision gateway/backend timings
	// for the stdout traffic-logging publisher. It is computed from the same ALS
	// CommonProperties timepoints as Latencies but at full precision, and is kept
	// separate so Moesif's millisecond units are unaffected. Never serialized
	// (json:"-") and not sent to other publishers.
	TrafficLogLatencies *TrafficLogLatencies `json:"-" bson:"-"`
}

// TrafficLogDirective is the presentation config for the stdout traffic-logging
// publisher, built from [traffic_logging] config (see
// publishers.buildGlobalDirective). A nil flow means that flow was not configured.
type TrafficLogDirective struct {
	Request  *TrafficLogFlow   `json:"request,omitempty"`
	Response *TrafficLogFlow   `json:"response,omitempty"`
	Fields   *TrafficLogFields `json:"fields,omitempty"`
	// Properties holds the resolved global properties (context references already
	// expanded at request time). The Log publisher emits them as a top-level
	// "properties" object on the log line.
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// TrafficLogFlow is the per-flow (request or response) presentation config.
type TrafficLogFlow struct {
	Payload bool `json:"payload"`
	Headers bool `json:"headers"`
}

// TrafficLogFields selects which fields appear in the emitted line. Exactly one
// of Only or Exclude should be set. Only keeps exactly the named fields; Exclude
// drops the named fields and keeps everything else. Names are top-level keys
// (e.g. "latencies", "requestHeaders") or dotted sub-key paths within map fields
// (e.g. "requestHeaders.authorization", "properties.env"). When set, this is
// authoritative over field presence; per-flow Payload/Headers booleans are ignored
// (global header masking still applies). If both are set, Only takes precedence.
type TrafficLogFields struct {
	Only    []string `json:"only,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}
