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

package normalizer

import (
	"fmt"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// Agent proxies are registered under the gateway document kind (Agent), not the
// control-plane kind (AgentProxy): the translator consumes deployment YAML, and
// that YAML is in the gateway's vocabulary.
func init() {
	shapeHandlers[constants.GatewayKindAgent] = normalizeAgent
}

// normalizeAgent is the identity: Agent has had exactly one stored shape, so
// there is nothing to up-convert. It is registered anyway so the kind is covered
// explicitly rather than by the unregistered-kind default, and so a caller that
// hands the Agent kind some other deployment struct fails here instead of
// shipping it.
func normalizeAgent(_ string, payload any) error {
	if _, ok := payload.(*model.AgentProxyDeploymentYAML); !ok {
		return fmt.Errorf("expected *model.AgentProxyDeploymentYAML, got %T", payload)
	}
	return nil
}
