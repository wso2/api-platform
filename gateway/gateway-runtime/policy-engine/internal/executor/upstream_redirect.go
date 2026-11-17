/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package executor

import "context"

// UpstreamRedirectHook is told the upstream definition a policy names with UpstreamName,
// as that policy's result is applied and before the next policy runs, so the caller can
// show later policies the upstream the request now goes to.
type UpstreamRedirectHook func(name string)

type upstreamRedirectHookKey struct{}

// WithUpstreamRedirectHook returns ctx carrying hook for the chain executions it is passed to.
func WithUpstreamRedirectHook(ctx context.Context, hook UpstreamRedirectHook) context.Context {
	if hook == nil {
		return ctx
	}
	return context.WithValue(ctx, upstreamRedirectHookKey{}, hook)
}

// noteUpstreamRedirect tells ctx's hook, if it has one, the upstream a policy named.
func noteUpstreamRedirect(ctx context.Context, name *string) {
	if name == nil || *name == "" {
		return
	}
	if hook, ok := ctx.Value(upstreamRedirectHookKey{}).(UpstreamRedirectHook); ok {
		hook(*name)
	}
}
