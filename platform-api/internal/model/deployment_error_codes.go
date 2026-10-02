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

package model

// Deployment status reasons. A reason is a code, never free text: it is stored
// in deployment_status.status_reason (VARCHAR(50)) and returned verbatim as a
// deployment's statusReason.
const (
	DeploymentErrorTimeout        = "DEPLOYMENT_TIMEOUT"
	DeploymentErrorGatewayFailure = "GATEWAY_PROCESSING_ERROR"
	// DeploymentErrorIDMismatch is acked by a gateway asked to undeploy a
	// deployment other than the one it holds.
	DeploymentErrorIDMismatch = "DEPLOYMENT_ID_MISMATCH"
)

// Agent proxy failure reasons acked by the gateway, as the ack's errorCode
// (gateway-controller pkg/controlplane, agentAckFailureCode). They are stored
// like any other gateway reason; the names are listed here so the reason
// vocabulary a deployment can report is documented in one place.
const (
	// DeploymentErrorAgentArtifactFetchFailed: the gateway could not fetch the
	// deployment artifact from the control plane, or it was not a readable ZIP.
	DeploymentErrorAgentArtifactFetchFailed = "AGENT_ARTIFACT_FETCH_FAILED"
	// DeploymentErrorAgentValidationFailed: the gateway rejected the Agent
	// definition as unparsable or invalid under its own validation rules.
	DeploymentErrorAgentValidationFailed = "AGENT_VALIDATION_FAILED"
	// DeploymentErrorAgentRenderFailed: the gateway could not resolve a template
	// or secret reference in the Agent definition.
	DeploymentErrorAgentRenderFailed = "AGENT_CONFIG_RENDER_FAILED"
	// DeploymentErrorAgentConflict: the gateway already holds another Agent with
	// the same handle, or the same name and version.
	DeploymentErrorAgentConflict = "AGENT_CONFLICT"
)

var DeploymentErrorMessages = map[string]string{
	DeploymentErrorTimeout:                  "Deployment timed out waiting for gateway acknowledgement",
	DeploymentErrorGatewayFailure:           "Gateway failed to process the deployment",
	DeploymentErrorIDMismatch:               "Gateway holds a different deployment than the one addressed",
	DeploymentErrorAgentArtifactFetchFailed: "Gateway could not retrieve the Agent deployment artifact",
	DeploymentErrorAgentValidationFailed:    "Gateway rejected the Agent definition as invalid",
	DeploymentErrorAgentRenderFailed:        "Gateway could not resolve a template or secret reference in the Agent definition",
	DeploymentErrorAgentConflict:            "Gateway already holds a conflicting Agent",
}
