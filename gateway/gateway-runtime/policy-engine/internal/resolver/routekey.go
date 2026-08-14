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

import (
	"context"
	"errors"
)

// RouteKeyResolver is the identity resolver: the request carries no operation
// identifier of its own, so the route's canonical chain key is the answer. Used by
// RestApi, WebSubApi, Mcp-as-shipped-today, LlmProvider and LlmProxy, where each route
// has exactly one policy chain.
//
// It is a real registry entry rather than a special case in the binding path, and it
// still costs nothing per request: the resolution it prepares is entirely static, so
// the kernel binds from the stored result without building a request view or calling
// Resolve.
type RouteKeyResolver struct{}

// Name returns the wire value for identity resolution.
func (*RouteKeyResolver) Name() string { return RouteKeyResolverName }

// Prepare captures the route's effective chain key.
//
// It deliberately does not re-apply the fallback to RouteKey: ingest already resolved
// the effective value, so an empty one here means the ingest layer is broken, not that
// this route wants its route key. Applying it a second time would create a second
// place for the two to disagree.
func (*RouteKeyResolver) Prepare(cfg ResolverRouteConfig) (PreparedResolver, error) {
	if cfg.CanonicalChainKey == "" {
		return nil, errors.New("route-key resolver requires an effective chain key")
	}
	return &preparedRouteKey{key: cfg.CanonicalChainKey}, nil
}

// preparedRouteKey is one identity route, holding only the key its requests bind to.
type preparedRouteKey struct {
	key string
}

// Requirements reports that nothing about the request is needed.
func (*preparedRouteKey) Requirements() RequestRequirements {
	return RequestRequirements{Body: BodyNotRequired}
}

// StaticResolution is the whole of this resolver's work, done once at ingest.
func (r *preparedRouteKey) StaticResolution() Resolution {
	return Resolution{Target: TargetDirectRoute, ChainKey: r.key}
}

// Resolve returns the same static resolution. Reached only by a caller that ignores
// StaticPreparedResolver; the kernel does not, so this never runs on the request path.
func (r *preparedRouteKey) Resolve(context.Context, RequestView) (Resolution, error) {
	return r.StaticResolution(), nil
}
