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

package kernel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// openAIRouteCtx is an execution context on the model endpoint's route: an LlmProxy — a kind
// that formats nothing — whose route names the openai error format.
func openAIRouteCtx(t *testing.T) *PolicyExecutionContext {
	t.Helper()
	ec := faultExecCtx(t, nil, nil)
	ec.sharedCtx.APIKind = policy.APIKindLlmProxy
	ec.errorFormat = "openai"
	return ec
}

// An authentication policy writes its own body and describes nothing. On a route that names
// the openai format that body is rendered over, so an OpenAI SDK reads the 401 — status and
// the policy's other headers kept.
func TestRouteFormat_ProducerBodyInAnotherShapeIsRendered(t *testing.T) {
	ec := openAIRouteCtx(t)

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 401,
		Headers:    map[string]string{"www-authenticate": "Bearer", contentTypeHeader: "application/json"},
		Body:       []byte(`{"error":"Unauthorized","message":"Valid API key required"}`),
	})

	assert.Equal(t, 401, out.StatusCode)
	assert.Equal(t, "Bearer", out.Headers["www-authenticate"])
	assert.Equal(t, "application/json", out.Headers[contentTypeHeader])
	var doc struct {
		Error map[string]any `json:"error"`
	}
	require.NoError(t, json.Unmarshal(out.Body, &doc), "body: %s", out.Body)
	assert.Equal(t, "Invalid or expired credentials.", doc.Error["message"])
	assert.Equal(t, "authentication_error", doc.Error["type"])
	assert.Nil(t, doc.Error["code"])
}

// A body already in the route's format stands: the model endpoint's own dispatch errors keep
// the param and code no renderer could know.
func TestRouteFormat_ProducerBodyInTheFormatStands(t *testing.T) {
	ec := openAIRouteCtx(t)
	own := []byte(`{"error":{"message":"You must provide a model parameter.","type":"invalid_request_error",` +
		`"param":"model","code":"missing_model"}}`)

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{StatusCode: 400, Body: own})

	assert.Equal(t, own, out.Body)
}

// Without a route format the rule is unchanged: a body that describes nothing is the
// policy's decision, on every existing proxy.
func TestRouteFormat_NoRouteFormatKeepsTheProducerBody(t *testing.T) {
	ec := faultExecCtx(t, nil, nil)
	ec.sharedCtx.APIKind = policy.APIKindLlmProxy
	body := []byte(`{"error":"Unauthorized","message":"Valid API key required"}`)

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{StatusCode: 401, Body: body})

	assert.Equal(t, body, out.Body)
}

// The response-phase producer path asks the same question.
func TestProducerBodyStands(t *testing.T) {
	ec := openAIRouteCtx(t)
	assert.False(t, ec.producerBodyStands([]byte(`{"error":"Unauthorized"}`)))
	assert.False(t, ec.producerBodyStands([]byte(`Too Many Requests`)))
	assert.True(t, ec.producerBodyStands([]byte(`{"error":{"message":"Rate limited"}}`)))

	ec.errorFormat = ""
	assert.True(t, ec.producerBodyStands([]byte(`{"error":"Unauthorized"}`)))
}
