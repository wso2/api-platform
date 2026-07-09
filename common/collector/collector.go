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

// Package collector holds logic shared by the gateway-controller and
// policy-engine's [collector] configuration: the collector is the shared
// data-capture pipeline (system policy + ALS transport) that gathers
// request/response headers and bodies for every consumer (analytics, stdout
// traffic logging). It has no on/off flag of its own — it is implicit,
// derived from whether a consumer is enabled — and both modules migrate the
// same set of deprecated [analytics] aliases onto it at load time. The two
// modules keep their own Config/CollectorConfig struct shapes (their transport
// types differ: GRPCEventServerConfig vs AccessLogsServiceConfig), so this
// package holds only the derivation/migration logic, not the structs.
package collector

import "log/slog"

// IsEnabled reports whether the collector should run: implicitly active
// whenever any consumer of the collected data (analytics or stdout traffic
// logging) is enabled, and off otherwise.
func IsEnabled(analyticsEnabled, trafficLoggingEnabled bool) bool {
	return analyticsEnabled || trafficLoggingEnabled
}

// CaptureFlags holds the deprecated [analytics] body-capture aliases being
// migrated onto [collector].
type CaptureFlags struct {
	SendRequestBody  bool
	SendResponseBody bool
	AllowPayloads    bool
}

// MigrateDeprecatedCapture maps the deprecated analytics.allow_payloads /
// analytics.send_request_body / analytics.send_response_body onto the
// collector's body-capture flags (when the collector flag is not already
// set), so existing configs keep working after capture settings moved under
// [collector]. These are analytics's own deprecated flags, so they are only
// honored while analyticsEnabled is true — otherwise a stale value left over
// from a disabled analytics setup could silently turn on body capture for an
// unrelated consumer (e.g. traffic_logging) enabled later. Directional
// aliases (send_request_body/send_response_body) take precedence over
// allow_payloads, which only fills in when neither directional flag is
// already set on the collector.
func MigrateDeprecatedCapture(analyticsEnabled bool, deprecated CaptureFlags, collectorSendRequestBody, collectorSendResponseBody *bool) {
	if !analyticsEnabled {
		return
	}
	if deprecated.SendRequestBody {
		if !*collectorSendRequestBody {
			slog.Warn("analytics.send_request_body is deprecated; use collector.request_body instead")
			*collectorSendRequestBody = true
		} else {
			slog.Warn("analytics.send_request_body is deprecated and collector.request_body is already configured; ignoring the analytics.send_request_body override")
		}
	}
	if deprecated.SendResponseBody {
		if !*collectorSendResponseBody {
			slog.Warn("analytics.send_response_body is deprecated; use collector.response_body instead")
			*collectorSendResponseBody = true
		} else {
			slog.Warn("analytics.send_response_body is deprecated and collector.response_body is already configured; ignoring the analytics.send_response_body override")
		}
	}
	if deprecated.AllowPayloads {
		slog.Warn("analytics.allow_payloads is deprecated; use collector.request_body and collector.response_body instead")
		if !*collectorSendRequestBody && !*collectorSendResponseBody {
			*collectorSendRequestBody = true
			*collectorSendResponseBody = true
		}
	}
}

// MigrateDeprecatedTransport maps a deprecated [analytics] transport-tuning
// alias (e.g. analytics.grpc_event_server on the controller,
// analytics.access_logs_service on the policy-engine) onto the collector's
// transport config when the collector's copy is still at its default, so
// existing configs keep working after transport tuning moved to
// [collector.server]. It is generic over the two modules' differing transport
// struct shapes (T is only ever compared and assigned, never inspected). This
// is analytics's own deprecated field, so it is only honored while
// analyticsEnabled is true — otherwise a stale value left over from a
// disabled analytics setup could silently reconfigure the transport for an
// unrelated consumer (e.g. traffic_logging) enabled later.
//
// deprecatedKey names the deprecated config key for the warning message
// (e.g. "analytics.grpc_event_server").
func MigrateDeprecatedTransport[T comparable](analyticsEnabled bool, deprecated T, collectorCfg *T, def T, deprecatedKey string) {
	if !analyticsEnabled || deprecated == def {
		return
	}
	if *collectorCfg == def {
		slog.Warn(deprecatedKey + " is deprecated; migrating it to collector.server")
		*collectorCfg = deprecated
	} else {
		slog.Warn(deprecatedKey + " is deprecated and collector.server is already configured; ignoring the " + deprecatedKey + " override")
	}
}
