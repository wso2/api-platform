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

// Event represents analytics event data.
type Event struct {
	API               *ExtendedAPI                   `json:"api,omitempty" bson:"api"`
	Operation         *Operation             `json:"operation,omitempty" bson:"operation"`
	Target            *Target                `json:"target,omitempty" bson:"target"`
	Application       *Application           `json:"application,omitempty" bson:"application"`
	Subscription      *Subscription          `json:"subscription,omitempty" bson:"subscription"`
	Latencies         *Latencies             `json:"latencies,omitempty" bson:"latencies"`
	MetaInfo          *MetaInfo              `json:"metaInfo,omitempty" bson:"meta_info"`
	Error             *Error                 `json:"error,omitempty" bson:"error"`
	ProxyResponseCode int                    `json:"proxyResponseCode,omitempty" bson:"proxy_response_code"`
	RequestTimestamp  time.Time                 `json:"requestTimestamp,omitempty" bson:"request_timestamp"`
	UserAgentHeader   string                 `json:"userAgentHeader,omitempty" bson:"user_agent_header"`
	UserName          string                 `json:"userName,omitempty" bson:"user_name"`
	UserIP            string                 `json:"userIp,omitempty" bson:"user_ip"`
	ErrorType         string                 `json:"errorType,omitempty" bson:"error_type"`
	Properties        map[string]interface{} `json:"properties,omitempty" bson:"properties"`

	// TrafficLog carries the per-API stdout traffic-logging opt-in marker stamped
	// by the log-message policy (access-log mode). When nil, the API has not opted
	// in and the stdout traffic-logging publisher skips the event. It is
	// gating/presentation state only and is never serialized (json:"-") nor sent
	// to other publishers.
	TrafficLog *TrafficLogDirective `json:"-" bson:"-"`
}

// TrafficLogDirective is the presentation config carried in the traffic-log
// marker. Field names mirror the policy's marker JSON so it round-trips. A nil
// flow means that flow was not configured.
type TrafficLogDirective struct {
	Request  *TrafficLogFlow   `json:"request,omitempty"`
	Response *TrafficLogFlow   `json:"response,omitempty"`
	Fields   *TrafficLogFields `json:"fields,omitempty"`
	// Properties holds the policy's resolved customProperties (context references
	// already expanded at request time). The Log publisher emits them under
	// properties.custom on the log line.
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// TrafficLogFlow is the per-flow (request or response) presentation config.
type TrafficLogFlow struct {
	Payload        bool     `json:"payload"`
	Headers        bool     `json:"headers"`
	ExcludeHeaders []string `json:"excludeHeaders,omitempty"`
}

// TrafficLogFields selects which fields appear in the emitted line. When set it is
// authoritative over field presence: the per-flow Payload/Headers booleans are
// ignored (per-flow ExcludeHeaders and global masking still apply). Names are
// top-level keys (e.g. "latencies", "target") or dotted property paths
// (e.g. "properties.requestHeaders"). Mode "exclude" drops the named keys; any
// other value (default "include") keeps only the named keys.
type TrafficLogFields struct {
	Mode  string   `json:"mode,omitempty"`
	Names []string `json:"names,omitempty"`
}
