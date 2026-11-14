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

package modelgateway

import (
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// A Model's routing policy takes the policy's own parameters, exactly as the
// policy defines them: a model is named in its model fields (`model`,
// `defaultModel`, `modelName`) and a provider in its provider fields. The
// controller passes them to the router as written. The model system policy's
// route job then matches the router's choice to one of the Model's targets — by
// provider and upstream model, by a target's name, or by either alone — so a
// Model's author may name a target by its name or by its upstream model.

// SelectionPhase is the request phase a router makes its choice in.
//
// It matters because an upstream credential that is applied in the request-header
// phase (oauth2-generator) is gated on the selected provider, and that gate is
// evaluated before any body-phase policy runs. A router that chooses from the body
// therefore cannot serve a provider whose credential is header-phase only.
type SelectionPhase string

const (
	PhaseHeaders SelectionPhase = "headers"
	PhaseBody    SelectionPhase = "body"
)

// RouterInfo is what the controller knows about one routing policy.
type RouterInfo struct {
	Phase SelectionPhase
}

// builtinRouters are the model routers shipped with the gateway. A router not
// listed here declares itself with the x-wso2-model-routing extension on its
// parameter schema (see RouterInfoFor).
//
// A router whose choice can depend on the body is recorded as body-phase even
// where it may also choose earlier: the phase is used to refuse configurations
// that could apply one provider's credential to another's request, so the
// conservative answer is the safe one.
var builtinRouters = map[string]RouterInfo{
	"model-round-robin":          {Phase: PhaseHeaders},
	"model-weighted-round-robin": {Phase: PhaseHeaders},
	"semantic-model-routing":     {Phase: PhaseBody},
	"intelligent-model-routing":  {Phase: PhaseBody},
	"typesafe-jev-model-routing": {Phase: PhaseBody},
	"time-based-model-routing":   {Phase: PhaseBody},
	"cost-based-model-routing":   {Phase: PhaseBody},
	"llm-header-router":          {Phase: PhaseHeaders},
}

// ModelRoutingSchemaExtension is the parameter-schema extension a custom router
// declares itself a model router with, and the phase it chooses in:
//
//	parameters:
//	  type: object
//	  x-wso2-model-routing:
//	    phase: headers     # headers | body; body when omitted
const ModelRoutingSchemaExtension = "x-wso2-model-routing"

// RouterInfoFor returns what the controller knows about a routing policy: from the
// built-in table for a shipped router, or from the extension on its definition's
// parameter schema. ok is false for a policy that is not a model router.
func RouterInfoFor(name string, def *models.PolicyDefinition) (RouterInfo, bool) {
	if info, ok := builtinRouters[name]; ok {
		return info, true
	}
	if def == nil || def.Parameters == nil {
		return RouterInfo{}, false
	}
	ext, ok := (*def.Parameters)[ModelRoutingSchemaExtension].(map[string]interface{})
	if !ok {
		return RouterInfo{}, false
	}
	info := RouterInfo{Phase: PhaseBody}
	if phase, _ := ext["phase"].(string); SelectionPhase(phase) == PhaseHeaders {
		info.Phase = PhaseHeaders
	}
	return info, true
}
