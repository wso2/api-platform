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

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

// A deployment type that does not implement both accessors is silently skipped
// by version translation rather than failing, so pin the interface at compile
// time. This assertion is about the type satisfying the contract — Agent proxies
// deliberately register no v1alpha1 shape handler, because a gateway old enough
// to want one has no Agent kind at all; that deploy is refused at pre-flight.
var _ deploymentArtifact = (*model.AgentProxyDeploymentYAML)(nil)

func TestAgentProxyDeploymentYAMLImplementsVersionAccessors(t *testing.T) {
	artifact := &model.AgentProxyDeploymentYAML{ApiVersion: "gateway.api-platform.wso2.com/v1"}

	assert.Equal(t, "gateway.api-platform.wso2.com/v1", artifact.GetApiVersion())

	artifact.SetApiVersion("gateway.api-platform.wso2.com/v1alpha1")
	assert.Equal(t, "gateway.api-platform.wso2.com/v1alpha1", artifact.GetApiVersion())
}
