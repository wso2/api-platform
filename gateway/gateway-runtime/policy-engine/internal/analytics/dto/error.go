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

// Error represents the error attributes in an analytics event.
//
// The first two fields are the established shape and keep their names, JSON keys and
// meaning: ErrorCode is the HTTP status the client received, and ErrorMessage the
// classification enum (AUTHENTICATION_FAILURE, API_LEVEL_LIMIT_EXCEEDED, ...) derived from it.
// The fault flow never changes either — its code is Wso2ErrorCode, and its human-readable
// text is Summary.
//
// Every added field is omitempty, so a failure the fault flow did not describe still
// serialises as the two-field object the established shape expects.
//
// Deliberately no Status field: Event.ProxyResponseCode already carries the status the client
// received. OriginalStatus IS here, because once a guardrail turns a 200 into a 446 the
// upstream's own status is gone from every other view.
//
// Deliberately no Description and no guardrail Assessments: for a guardrail rejection those
// hold the content the guardrail existed to stop, and an event is forwarded to external
// publishers.
type Error struct {
	ErrorCode    int              `json:"errorCode"`
	ErrorMessage FaultSubCategory `json:"errorMessage"`

	// Wso2ErrorCode is the fault code the failing policy or the engine declared — 900902,
	// 906201, ... (sdk/core/policy/v1alpha2 fault_codes.go). Absent when the failure carried
	// no numeric code, such as a router failure the engine described without one.
	Wso2ErrorCode int `json:"wso2ErrorCode,omitempty"`

	// Type is the failing policy's own class — "authentication", "guardrail", "upstream".
	// It sits alongside Event.ErrorType rather than replacing it: that one is the
	// category derived from the status, this one is what the policy said it was, and a
	// guardrail shows why both are wanted — it categorises as OTHER while its type is
	// precisely "guardrail".
	Type string `json:"type,omitempty"`
	// Direction states which side was rejected: the content the caller sent, or the content
	// the upstream returned. Two operationally different events that share a status code.
	Direction string `json:"direction,omitempty"`
	// Summary is the client-facing message. Named Summary, not Message, because
	// ErrorMessage above is already taken by the classification enum.
	Summary string `json:"summary,omitempty"`
	// Policy and PolicyPhase name what failed and where. Empty when no policy did — an
	// infrastructure failure — which means "not caused by a policy", never "unknown".
	Policy      string `json:"policy,omitempty"`
	PolicyPhase string `json:"policyPhase,omitempty"`
	// Source is which actor produced the response: gateway, backend, router, noRoute. The
	// one thing a status cannot say — a backend's own 503 and the router's are the same
	// number and mean opposite things.
	Source string `json:"source,omitempty"`
	// OriginalStatus is the upstream's status before a policy changed it. Absent when
	// nothing changed it, and absent for a rejection, where no upstream response existed.
	OriginalStatus int `json:"originalStatus,omitempty"`
	// Guardrail carries the intervention detail, and is nil for every non-guardrail
	// failure.
	Guardrail *ErrorGuardrail `json:"guardrail,omitempty"`
	// JSONRPCCode is the wire-level code for a JSON-RPC caller (MCP today), and is 0 for
	// every other protocol.
	JSONRPCCode int `json:"jsonRpcCode,omitempty"`
}

// ErrorGuardrail is the guardrail-specific half of Error.
//
// Assessments are deliberately absent: for a response guardrail they are the blocked content
// itself. The name, action and reason carry the operationally useful part without the payload.
type ErrorGuardrail struct {
	Name   string `json:"name,omitempty"`
	Action string `json:"action,omitempty"`
	Reason string `json:"reason,omitempty"`
}
