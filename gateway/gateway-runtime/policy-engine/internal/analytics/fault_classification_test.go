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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
)

// classifiedEvent is an event as classifyFault leaves it for a gateway-produced 401 — the
// state applyFaultDetails always finds, since it runs after the classifier.
func classifiedEvent() *dto.Event {
	return &dto.Event{
		ProxyResponseCode: 401,
		ErrorType:         string(dto.FaultCategoryAuth),
		Error:             &dto.Error{ErrorCode: 401, ErrorMessage: dto.AuthenticationFailure},
	}
}

// A successful request must not acquire an error object. Event.Error is a pointer precisely
// so "no failure" is expressible, and an empty struct would read as a fault with no detail.
func TestApplyFaultDetails_SuccessfulRequestStaysClean(t *testing.T) {
	event := &dto.Event{ProxyResponseCode: 200}

	applyFaultDetails(event, map[string]string{}, map[string]interface{}{})

	assert.Nil(t, event.Error, "no fault metadata means no error object")
	assert.Empty(t, event.ErrorType)
	assert.Empty(t, event.Properties, "and no fault keys leak into the property bag")
}

// The established fields keep the classifier's values — the status as errorCode, its
// category and subcategory — and the fault flow's detail is added beside them. A consumer
// built on the established fields reads the same values with or without the fault flow.
func TestApplyFaultDetails_AddsDetailWithoutChangingEstablishedFields(t *testing.T) {
	event := classifiedEvent()
	kv := map[string]string{
		dto.PropKeyFaultCode:        "900902",
		dto.PropKeyFaultType:        "authentication",
		dto.PropKeyFaultMessage:     "Valid credentials required",
		dto.PropKeyFaultPolicy:      "jwt-auth",
		dto.PropKeyFaultPolicyPhase: "request_headers",
		dto.PropKeyFaultSource:      "gateway",
		dto.PropKeyFaultDirection:   "Request",
	}

	applyFaultDetails(event, kv, map[string]interface{}{})

	require.NotNil(t, event.Error)
	assert.Equal(t, 401, event.Error.ErrorCode, "errorCode stays the HTTP status")
	assert.Equal(t, dto.AuthenticationFailure, event.Error.ErrorMessage)
	assert.Equal(t, string(dto.FaultCategoryAuth), event.ErrorType)

	assert.Equal(t, 900902, event.Error.Wso2ErrorCode, "the fault code has its own field")
	assert.Equal(t, "authentication", event.Error.Type)
	assert.Equal(t, "Request", event.Error.Direction)
	assert.Equal(t, "Valid credentials required", event.Error.Summary)
	assert.Equal(t, "jwt-auth", event.Error.Policy)
	assert.Equal(t, "request_headers", event.Error.PolicyPhase)
	assert.Equal(t, "gateway", event.Error.Source)
	assert.Nil(t, event.Error.Guardrail, "nil for every non-guardrail failure")
	assert.Empty(t, event.Properties, "fault detail belongs on Error, not in Properties")
}

// A guardrail: OTHER by status, "guardrail" by its own account, and the upstream's original
// status — the one value that exists nowhere else once the guardrail rewrote it.
func TestApplyFaultDetails_Guardrail(t *testing.T) {
	event := &dto.Event{
		ProxyResponseCode: 422,
		ErrorType:         string(dto.FaultCategoryOther),
		Error:             &dto.Error{ErrorCode: 422, ErrorMessage: dto.OtherUnclassified},
	}
	kv := map[string]string{
		dto.PropKeyFaultCode:            "906201",
		dto.PropKeyFaultType:            "guardrail",
		dto.PropKeyFaultGuardrail:       "word-count-guardrail",
		dto.PropKeyFaultGuardrailAction: "GUARDRAIL_INTERVENED",
		dto.PropKeyFaultGuardrailReason: "word count out of range",
	}

	applyFaultDetails(event, kv, map[string]interface{}{dto.PropKeyFaultOriginalStatus: float64(200)})

	assert.Equal(t, 422, event.Error.ErrorCode)
	assert.Equal(t, 906201, event.Error.Wso2ErrorCode)
	assert.Equal(t, "guardrail", event.Error.Type)
	assert.Equal(t, 200, event.Error.OriginalStatus)
	require.NotNil(t, event.Error.Guardrail)
	assert.Equal(t, "word-count-guardrail", event.Error.Guardrail.Name)
	assert.Equal(t, "GUARDRAIL_INTERVENED", event.Error.Guardrail.Action)
	assert.Equal(t, "word count out of range", event.Error.Guardrail.Reason)
}

// A failure declared on a status below 400 — the one the classifier cannot see — gets the
// established fields in the form an unclassified failure has always had.
func TestApplyFaultDetails_DeclaredBelow400GetsTheEstablishedShape(t *testing.T) {
	event := &dto.Event{ProxyResponseCode: 200}

	applyFaultDetails(event, map[string]string{
		dto.PropKeyFaultCode: "960001",
		dto.PropKeyFaultType: "validation",
	}, map[string]interface{}{})

	require.NotNil(t, event.Error)
	assert.Equal(t, 200, event.Error.ErrorCode)
	assert.Equal(t, dto.OtherUnclassified, event.Error.ErrorMessage)
	assert.Equal(t, string(dto.FaultCategoryOther), event.ErrorType)
	assert.Equal(t, 960001, event.Error.Wso2ErrorCode)
}

// A router failure the engine described without a code keeps the classifier's more specific
// category and reports its source; there is simply no wso2ErrorCode.
func TestApplyFaultDetails_NoCode(t *testing.T) {
	event := &dto.Event{
		ProxyResponseCode: 503,
		ErrorType:         string(dto.FaultCategoryTargetConnectivity),
		Error:             &dto.Error{ErrorCode: 503, ErrorMessage: dto.TargetConnectivityOther},
	}

	applyFaultDetails(event, map[string]string{
		dto.PropKeyFaultSource: "router",
		dto.PropKeyFaultType:   "upstream",
	}, map[string]interface{}{})

	assert.Equal(t, string(dto.FaultCategoryTargetConnectivity), event.ErrorType)
	assert.Equal(t, dto.TargetConnectivityOther, event.Error.ErrorMessage)
	assert.Equal(t, 0, event.Error.Wso2ErrorCode)
	assert.Equal(t, "router", event.Error.Source)
}

// A non-numeric code is a policy bug. It must not panic or drop the error; it stays visible
// where a reader of the error will actually see it.
func TestApplyFaultDetails_NonNumericCodeIsDiagnosable(t *testing.T) {
	event := classifiedEvent()

	applyFaultDetails(event, map[string]string{dto.PropKeyFaultCode: "GW-1234"}, map[string]interface{}{})

	assert.Equal(t, 0, event.Error.Wso2ErrorCode)
	assert.Contains(t, event.Error.Summary, "GW-1234",
		"kept so the bug is findable rather than silently swallowed")
}

// The wire shape: the two established keys first and unchanged, the new ones beside them.
func TestApplyFaultDetails_SerialisedShape(t *testing.T) {
	event := classifiedEvent()

	applyFaultDetails(event, map[string]string{
		dto.PropKeyFaultCode:   "900902",
		dto.PropKeyFaultSource: "gateway",
	}, map[string]interface{}{})

	raw, err := json.Marshal(event.Error)
	require.NoError(t, err)
	assert.JSONEq(t,
		`{"errorCode":401,"errorMessage":"AUTHENTICATION_FAILURE","wso2ErrorCode":900902,"source":"gateway"}`,
		string(raw))
}

// End to end through prepareAnalyticEvent, with the access-log classification AND the
// collector's metadata both present — the combination that broke when the status classifier
// landed after this feature and replaced the error object outright.
func TestPrepareAnalyticEvent_FaultDetailsSurviveTheStatusClassification(t *testing.T) {
	entry := createLogEntryWithMetadataValues(map[string]*structpb.Value{
		dto.PropKeyFaultCode:           structpb.NewStringValue("906201"),
		dto.PropKeyFaultType:           structpb.NewStringValue("guardrail"),
		dto.PropKeyFaultPolicy:         structpb.NewStringValue("word-count-guardrail"),
		dto.PropKeyFaultSource:         structpb.NewStringValue("gateway"),
		dto.PropKeyFaultOriginalStatus: structpb.NewNumberValue(200),
	})
	entry.Response.ResponseCode = wrapperspb.UInt32(422)
	entry.Response.ResponseCodeDetails = "ext_proc"

	event := NewAnalytics(&config.Config{}).prepareAnalyticEvent(entry)

	require.NotNil(t, event.Error)
	assert.Equal(t, 422, event.Error.ErrorCode, "the established errorCode is the status")
	assert.Equal(t, dto.OtherUnclassified, event.Error.ErrorMessage)
	assert.Equal(t, string(dto.FaultCategoryOther), event.ErrorType)
	assert.Equal(t, 906201, event.Error.Wso2ErrorCode, "and the fault flow's detail survives")
	assert.Equal(t, "guardrail", event.Error.Type)
	assert.Equal(t, "word-count-guardrail", event.Error.Policy)
	assert.Equal(t, "gateway", event.Error.Source)
	assert.Equal(t, 200, event.Error.OriginalStatus)
}
