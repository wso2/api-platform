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

package deploymenttransform

import (
	"fmt"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// init registers apiVersion down-convert transformations for the artifact kinds
// whose generators stamp the canonical apiVersion but carry no policy-list split
// (RestApi, WebSubApi, Mcp).
//
// Generators stamp the canonical apiVersion (GatewayApiVersion, i.e. ".../v1").
// Gateways < 1.2.0 only accept the legacy ".../v1alpha1" value, so for those
// targets we rewrite the apiVersion field. Unlike the LLM kinds, these artifacts
// have no globalPolicies/operationPolicies split, so the apiVersion swap is the
// only conversion needed.
func init() {
	minVer := ParseVersion(MinSplitPoliciesVersion)
	isOldGateway := func(target Version) bool { return !target.GTE(minVer) }

	// RestApi and WebSubApi both marshal from *dto.APIDeploymentYAML.
	downgradeAPIDeploymentVersion := func(payload any) error {
		artifact, ok := payload.(*dto.APIDeploymentYAML)
		if !ok {
			return fmt.Errorf("expected *dto.APIDeploymentYAML, got %T", payload)
		}
		artifact.ApiVersion = constants.GatewayApiVersionV1Alpha1
		return nil
	}

	for _, kind := range []string{constants.RestApi, constants.WebSubApi} {
		defaultRegistry.Register(Transformation{
			Name:        "apiversion/downconvert-pre-1.2.0/" + kind,
			Kind:        kind,
			AppliesWhen: isOldGateway,
			Apply:       downgradeAPIDeploymentVersion,
		})
	}

	// Mcp marshals from *model.MCPProxyDeploymentYAML (a different struct).
	defaultRegistry.Register(Transformation{
		Name:        "apiversion/downconvert-pre-1.2.0/" + constants.MCPProxy,
		Kind:        constants.MCPProxy,
		AppliesWhen: isOldGateway,
		Apply: func(payload any) error {
			artifact, ok := payload.(*model.MCPProxyDeploymentYAML)
			if !ok {
				return fmt.Errorf("expected *model.MCPProxyDeploymentYAML, got %T", payload)
			}
			artifact.ApiVersion = constants.GatewayApiVersionV1Alpha1
			return nil
		},
	})
}
