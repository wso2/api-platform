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
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/gwversion"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/translate"
)

// LLMProvider is the LLM provider kind.
//
// Stored shapes: platform data version 1.0 kept every policy in one flat
// `policies` list; 1.1 splits them into globalPolicies and operationPolicies.
// Normalize brings a 1.0 artifact up to the split shape. Older gateways need
// two adaptations, applied in this order.
var LLMProvider = translate.Kind{
	GatewayKind: constants.LLMProvider,
	Normalize:   llmProviderNormalize,
	Steps: []translate.Step{
		{
			Below: gwversion.MinGatewayV1Version,
			Name:  "flatten globalPolicies and operationPolicies into the legacy policies list",
			Apply: llmProviderFlattenPolicies,
		},
		{
			Below: gwversion.MinLLMUpstreamAuthTypeNoneOtherVersion,
			Name:  "adapt spec.upstream.auth to the auth types older validators accept",
			Apply: llmProviderUpstreamAuth,
		},
	},
}

// llmProviderNormalize folds a legacy flat policies list into the split lists.
// An artifact already in the split shape is left unchanged.
func llmProviderNormalize(_ string, artifact any) error {
	a, err := expect[*dto.LLMProviderDeploymentYAML](artifact)
	if err != nil {
		return err
	}
	if len(a.Spec.Policies) == 0 {
		return nil
	}
	splitLegacyPolicies(a.Spec.Policies, &a.Spec.GlobalPolicies, &a.Spec.OperationPolicies)
	a.Spec.Policies = nil
	return nil
}

// llmProviderFlattenPolicies is the inverse of llmProviderNormalize for
// gateways that only understand the flat list.
func llmProviderFlattenPolicies(artifact any, _ *translate.Report) error {
	a, err := expect[*dto.LLMProviderDeploymentYAML](artifact)
	if err != nil {
		return err
	}
	flattenPolicyLists(a.Spec.GlobalPolicies, a.Spec.OperationPolicies, &a.Spec.Policies)
	a.Spec.GlobalPolicies = nil
	a.Spec.OperationPolicies = nil
	return nil
}

// llmProviderUpstreamAuth applies the shared upstream-auth rule to
// spec.upstream.auth.
func llmProviderUpstreamAuth(artifact any, r *translate.Report) error {
	a, err := expect[*dto.LLMProviderDeploymentYAML](artifact)
	if err != nil {
		return err
	}
	downConvertAPIUpstreamAuth(llmUpstreamAuthRule, &a.Spec.Upstream.Auth, constants.LLMProvider, "spec.upstream.auth", r)
	return nil
}
