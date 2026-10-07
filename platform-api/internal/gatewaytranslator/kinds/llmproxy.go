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

// LLMProxy is the LLM proxy kind.
//
// Stored shapes: platform data version 1.0 kept every policy in one flat
// `policies` list; 1.1 splits them into globalPolicies and operationPolicies.
// Normalize brings a 1.0 artifact up to the split shape. Older gateways need
// three adaptations, applied in this order.
var LLMProxy = translate.Kind{
	GatewayKind: constants.LLMProxy,
	Normalize:   llmProxyNormalize,
	Steps: []translate.Step{
		{
			Below: gwversion.MinGatewayV1Version,
			Name:  "flatten globalPolicies and operationPolicies into the legacy policies list",
			Apply: llmProxyFlattenPolicies,
		},
		{
			Below: gwversion.MinGatewayV1Version,
			Name:  "drop spec.additionalProviders",
			Apply: llmProxyDropAdditionalProviders,
		},
		{
			Below: gwversion.MinUpstreamAuthTypeNoneOtherVersion,
			Name:  "adapt spec.provider.auth to the auth types older validators accept",
			Apply: llmProxyProviderAuth,
		},
	},
}

// llmProxyNormalize folds a legacy flat policies list into the split lists.
// An artifact already in the split shape is left unchanged.
func llmProxyNormalize(_ string, artifact any) error {
	a, err := expect[*dto.LLMProxyDeploymentYAML](artifact)
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

// llmProxyFlattenPolicies is the inverse of llmProxyNormalize for gateways
// that only understand the flat list.
func llmProxyFlattenPolicies(artifact any, _ *translate.Report) error {
	a, err := expect[*dto.LLMProxyDeploymentYAML](artifact)
	if err != nil {
		return err
	}
	flattenPolicyLists(a.Spec.GlobalPolicies, a.Spec.OperationPolicies, &a.Spec.Policies)
	a.Spec.GlobalPolicies = nil
	a.Spec.OperationPolicies = nil
	return nil
}

// llmProxyDropAdditionalProviders removes the additional-provider list, which
// older gateways do not know and would otherwise drop silently. One warning
// is recorded per provider so the operator can see exactly what the gateway
// will not route.
func llmProxyDropAdditionalProviders(artifact any, r *translate.Report) error {
	a, err := expect[*dto.LLMProxyDeploymentYAML](artifact)
	if err != nil {
		return err
	}
	for _, ap := range a.Spec.AdditionalProviders {
		r.Warn(constants.LLMProxy, "spec.additionalProviders",
			"additional provider %q dropped: gateways below %s route every request to the primary provider",
			ap.ID, gwversion.MinGatewayV1Version)
	}
	a.Spec.AdditionalProviders = nil
	return nil
}

// llmProxyProviderAuth applies the shared upstream-auth rule to
// spec.provider.auth.
func llmProxyProviderAuth(artifact any, r *translate.Report) error {
	a, err := expect[*dto.LLMProxyDeploymentYAML](artifact)
	if err != nil {
		return err
	}
	downConvertAPIUpstreamAuth(&a.Spec.Provider.Auth, constants.LLMProxy, "spec.provider.auth", r)
	return nil
}
