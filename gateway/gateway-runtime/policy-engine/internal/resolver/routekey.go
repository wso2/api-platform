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

package resolver

// RouteKeyResolver trivially returns the route key as the policy chain key.
// Used by RestAPI, LLM Provider, and LLM Proxy kinds where each route has
// exactly one policy chain, keyed by the same route name.
type RouteKeyResolver struct{}

func (r *RouteKeyResolver) Name() string { return "route-key" }

func (r *RouteKeyResolver) Requirements() ResolverRequirements {
	return ResolverRequirements{
		BufferBody: false,
		Headers:    false,
	}
}

func (r *RouteKeyResolver) Resolve(ctx ResolverContext) (string, error) {
	return ctx.RouteKey, nil
}
