/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package utils

import (
	"fmt"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/modelgateway"
)

// ModelProviderGroupPolicies builds the provider-specific policies one provider
// gets in a Model chain on one endpoint, in the order they run: its transformer,
// the credential presented to it, and — for a provider reached through its own
// route — the internal loopback marker.
//
// They are built by the same builders a multi-provider LLM proxy uses, so a Model
// target and a proxy's additional provider present a credential the same way: the
// same policy, the same valuePrefix handling for a provider route's api-key-auth,
// the same move of a set-headers credential into the request-body phase. Every one
// is gated on `selected_provider` naming this provider, which the model system
// policy sets once it has resolved the request to a target; none runs on a
// request routed elsewhere.
//
// target is any one of the Model's targets on the provider. Everything read here
// is a property of the provider, not of the target, except a routed target's
// credential — and a Model's targets on one provider route present one credential,
// which modelgateway.Resolve checks. transformer is the one the endpoint's
// spec.transformers chose for the provider (modelgateway.Bind), nil when the
// provider passes through.
func (t *LLMProviderTransformer) ModelProviderGroupPolicies(target *modelgateway.Target,
	transformer *modelgateway.Transformer) ([]api.Policy, error) {
	if target == nil || target.Provider == nil {
		return nil, fmt.Errorf("model target has no resolved provider")
	}
	provider := target.ProviderHandle
	condition := selectedProviderExecutionCondition(provider, false)
	var out []api.Policy

	if tr := transformer; tr != nil {
		params := make(map[string]interface{}, len(tr.Params)+1)
		for k, v := range tr.Params {
			params[k] = v
		}
		// The transformer names the upstream it routes to by this, and the
		// compiler names the provider's cluster by it, so they agree by
		// construction.
		params["providerId"] = provider
		cond := condition
		out = append(out, api.Policy{
			Name:               tr.PolicyName,
			Version:            tr.Version,
			Params:             &params,
			ExecutionCondition: &cond,
		})
	}

	var auth *api.LLMUpstreamAuth
	valuePrefix := ""
	field := fmt.Sprintf("targets[%s].auth", target.Name)
	switch target.Mode {
	case modelgateway.ModeDirect:
		auth = directTargetAuth(target)
		field = fmt.Sprintf("provider[%s].upstream.auth", provider)
	case modelgateway.ModeRoute:
		auth = target.RouteAuth
		valuePrefix = target.RouteValuePrefix
	}
	if auth != nil {
		pol, err := t.proxyUpstreamAuthPolicy(auth, valuePrefix, field)
		if err != nil {
			return nil, err
		}
		if pol != nil {
			cond := condition
			pol.ExecutionCondition = &cond
			// The provider is chosen while the request body is processed, so a
			// set-headers credential must wait for that choice.
			*pol = withRequestBodyPhase(*pol)
			out = append(out, *pol)
		}
	}

	if target.Mode == modelgateway.ModeRoute {
		marker, err := t.proxyInternalLoopbackMarkerPolicy()
		if err != nil {
			return nil, err
		}
		cond := condition
		marker.ExecutionCondition = &cond
		*marker = withRequestBodyPhase(*marker)
		out = append(out, *marker)
	}

	return out, nil
}

// directTargetAuth is a direct target's credential: the provider's own upstream
// auth, in the shape the proxy credential builder takes. The two auth blocks are
// the same schema under two generated names.
func directTargetAuth(target *modelgateway.Target) *api.LLMUpstreamAuth {
	src := target.Provider.Config.Spec.Upstream.Auth
	if src == nil {
		return nil
	}
	return &api.LLMUpstreamAuth{
		Type:          api.LLMUpstreamAuthType(src.Type),
		Header:        src.Header,
		Value:         src.Value,
		PolicyName:    src.PolicyName,
		PolicyVersion: src.PolicyVersion,
		PolicyParams:  src.PolicyParams,
	}
}
