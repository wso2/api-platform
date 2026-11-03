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

package analytics

import (
	"strconv"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
)

// Adding the fault flow's account of a failure to an analytics event.
//
// Analytics is built from Envoy's access log, which carries a status and code_details and
// knows nothing about which policy rejected the request or why. The collector's OnFault runs
// as the last entry of every API's fault chain and stamps the resolved failure into analytics
// metadata, which arrives here as the PropKeyFault* keys.
//
// The established error fields — errorCode, errorMessage and the event's errorType — are
// classifyFault's, from the status and Envoy's response flags, and this file never changes
// them: a dashboard built on them must read the same values with or without the fault flow.
// Everything the fault flow knows goes into fields that did not exist before, the code
// included (wso2ErrorCode).

// applyFaultDetails adds what the collector's OnFault stamped to the event's error object.
//
// Runs AFTER classifyFault, so it adds to the object that one built rather than replacing it.
//
// The PropKeyFault* keys are TRANSPORT between two Go modules: the collector reaches the
// engine over Envoy's dynamic metadata, which is a flat map. They become typed dto.Error
// fields here and are deliberately NOT copied into Event.Properties.
//
// Absent metadata means the request did not go through the fault flow, so the event is left
// as it was — a successful request must not acquire an empty error object.
func applyFaultDetails(
	event *dto.Event,
	keyValuePairs map[string]string,
	typedValuePairs map[string]interface{},
) {
	str := func(key string) string { return keyValuePairs[key] }

	codeStr := str(dto.PropKeyFaultCode)
	faultType := str(dto.PropKeyFaultType)

	// Nothing the collector stamped at all: not a fault path. Leave the event untouched.
	if codeStr == "" && faultType == "" && str(dto.PropKeyFaultSource) == "" &&
		str(dto.PropKeyFaultPolicy) == "" {
		return
	}

	// classifyFault builds the error object for every status of 400 or above. The one fault
	// it cannot see is a policy declaring a failure on a lower status — a GraphQL-style 200
	// with the failure in the body. That gets the established fields in the same form an
	// unclassified failure has always had: the status, UNCLASSIFIED, OTHER.
	if event.Error == nil {
		event.Error = &dto.Error{
			ErrorCode:    event.ProxyResponseCode,
			ErrorMessage: dto.OtherUnclassified,
		}
		if event.ErrorType == "" {
			event.ErrorType = string(dto.FaultCategoryOther)
		}
	}

	e := event.Error
	e.Type = faultType
	e.Direction = str(dto.PropKeyFaultDirection)
	e.Summary = str(dto.PropKeyFaultMessage)
	e.Policy = str(dto.PropKeyFaultPolicy)
	e.PolicyPhase = str(dto.PropKeyFaultPolicyPhase)
	e.Source = str(dto.PropKeyFaultSource)

	// Numbers arrive through structpb as float64, so they are read from the typed map
	// rather than parsed back out of the stringified one.
	if v, ok := typedValuePairs[dto.PropKeyFaultOriginalStatus].(float64); ok {
		e.OriginalStatus = int(v)
	}
	if v, ok := typedValuePairs[dto.PropKeyFaultJSONRPCCode].(float64); ok {
		e.JSONRPCCode = int(v)
	}

	if name := str(dto.PropKeyFaultGuardrail); name != "" ||
		str(dto.PropKeyFaultGuardrailAction) != "" || str(dto.PropKeyFaultGuardrailReason) != "" {
		e.Guardrail = &dto.ErrorGuardrail{
			Name:   name,
			Action: str(dto.PropKeyFaultGuardrailAction),
			Reason: str(dto.PropKeyFaultGuardrailReason),
		}
	}

	// A router failure the engine described without a code reports everything above and no
	// wso2ErrorCode. A non-numeric code is a policy bug: kept visible in Summary, where a
	// reader of the error will see it, rather than swallowed.
	if code, err := strconv.Atoi(codeStr); err == nil {
		e.Wso2ErrorCode = code
	} else if codeStr != "" && e.Summary == "" {
		e.Summary = "unclassified fault code " + codeStr
	}
}
