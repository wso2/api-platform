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

import "fmt"

// PolicyChainResolver resolves which PolicyChain to apply for a given request.
// Like a policy, it declares what request data it needs before it can run.
type PolicyChainResolver interface {
	// Name returns the resolver's registered name (e.g. "route-key", "mcp-tool").
	Name() string

	// Requirements declares what request data the resolver needs.
	Requirements() ResolverRequirements

	// Resolve returns the policy chain key for the given request context.
	Resolve(ctx ResolverContext) (string, error)
}

// ResolverRequirements declares what request data a resolver needs.
type ResolverRequirements struct {
	// BufferBody means the whole request body must be buffered before Resolve() is called.
	BufferBody bool
	// Headers means request headers must be available.
	Headers bool
}

// ResolverContext contains request data available to the resolver.
type ResolverContext struct {
	RouteKey string
	Headers  map[string][]string
	Body     []byte
}

// Registry holds registered resolvers by name.
var Registry = map[string]PolicyChainResolver{}

// Register adds a resolver to the global registry.
func Register(r PolicyChainResolver) {
	Registry[r.Name()] = r
}

// Get returns a resolver by name.
func Get(name string) (PolicyChainResolver, error) {
	r, ok := Registry[name]
	if !ok {
		return nil, fmt.Errorf("resolver not found: %s", name)
	}
	return r, nil
}

func init() {
	// Register built-in resolvers
	Register(&RouteKeyResolver{})
}
