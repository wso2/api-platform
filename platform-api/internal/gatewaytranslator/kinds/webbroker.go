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

// WebBroker is the WebBroker API kind (event gateway). The kind first shipped
// in gateway 1.2.0; a deploy to an older gateway is refused before translation.
// It has had a single stored shape and needs no step of its own.
var WebBroker = translate.Kind{
	GatewayKind:       constants.WebBrokerApi,
	MinGatewayVersion: gwversion.MinWebBrokerKindGatewayVersion,
	Normalize:         onlyType[*model.WebBrokerAPIDeploymentYAML](),
}
