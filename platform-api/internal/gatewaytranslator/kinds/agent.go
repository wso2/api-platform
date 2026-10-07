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

package kinds

import (
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/gwversion"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/translate"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// Agent is the Agent proxy kind: AgentProxy on the control plane, Agent in the
// artifact the gateway receives.
//
// The kind first shipped in gateway 2026.09.24. A deploy to an older gateway is
// refused before translation (see gatewaytranslator.EnsureKindSupported), so
// no step is needed: every gateway that has the kind understands the current
// shape. Normalize only checks the payload type.
var Agent = translate.Kind{
	GatewayKind:       constants.GatewayKindAgent,
	MinGatewayVersion: gwversion.MinAgentKindGatewayVersion,
	Normalize:         onlyType[*model.AgentProxyDeploymentYAML](),
}
