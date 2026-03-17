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

import "time"

// DeploymentAckPayload represents the acknowledgement message sent by the gateway
// after processing a deployment or undeployment event.
type DeploymentAckPayload struct {
	DeploymentID string    `json:"deploymentId"`
	ArtifactID   string    `json:"artifactId"`
	ResourceType string    `json:"resourceType"` // "api", "llmprovider", "llmproxy"
	Action       string    `json:"action"`       // "deploy", "undeploy"
	Status       string    `json:"status"`       // "success", "failed"
	PerformedAt  time.Time `json:"performedAt"`
	ErrorCode    string    `json:"errorCode,omitempty"`
}
