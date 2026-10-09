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
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/translate"
)

// RestAPI is the REST API kind.
//
// REST artifacts have had a single stored shape, and they carry nothing an
// older gateway rejects: executionCondition on a policy exists since 1.0.0 and
// an upstream target is url/ref only, with no auth block. The only adaptation
// is the CRD apiVersion swap that translate.Run applies to every kind.
var RestAPI = translate.Kind{
	GatewayKind: constants.RestApi,
	Normalize:   onlyType[*dto.APIDeploymentYAML](),
}
