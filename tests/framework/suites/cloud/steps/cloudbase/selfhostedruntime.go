/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudbase

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// keyLocalGateways holds the runner's localGatewayRegistry in its runner context, so a runner
// only ever reaches the gateways its own scenarios started.
const keyLocalGateways = "cloudLocalGateways"

// Stopper is the part of a compose stack a local gateway's lifecycle needs.
type Stopper interface {
	Stop(ctx context.Context) error
}

// localGateway is a gateway a scenario started and the runtime origin approved for it, empty until
// the origin is known.
type localGateway struct {
	stack  Stopper
	origin string
}

// localGatewayRegistry holds a runner's local gateways by suffix. A local origin is a plain-http
// loopback address, which the data-plane check refuses for any other target: only an origin of a
// gateway listed here, and still running, is let through.
type localGatewayRegistry struct {
	mu       sync.Mutex
	gateways map[string]*localGateway
}

// LocalGatewaysOf returns the runner's registry, creating it on first use when create is set.
func LocalGatewaysOf(ctx context.Context, create bool) (*localGatewayRegistry, error) {
	if value, ok := tcontext.Get(ctx, keyLocalGateways); ok {
		if registry, ok := value.(*localGatewayRegistry); ok {
			return registry, nil
		}
		return nil, fmt.Errorf("runner context value %q is not a local gateway registry", keyLocalGateways)
	}
	if !create {
		return nil, nil
	}
	registry := &localGatewayRegistry{gateways: map[string]*localGateway{}}
	if err := tcontext.Set(ctx, keyLocalGateways, registry); err != nil {
		return nil, err
	}
	return registry, nil
}

// Add records a runner's local gateway so cleanup can stop it.
func (r *localGatewayRegistry) Add(suffix string, stack Stopper) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gateways[suffix] = &localGateway{stack: stack}
}

// Approve records the origin a runner's local gateway may be reached at.
func (r *localGatewayRegistry) Approve(suffix, origin string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if gateway := r.gateways[suffix]; gateway != nil {
		gateway.origin = origin
	}
}

func (r *localGatewayRegistry) approved(origin string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, gateway := range r.gateways {
		if gateway.origin != "" && gateway.origin == origin {
			return true
		}
	}
	return false
}

// Stop stops the gateway with the suffix and forgets it, approval included. A gateway that fails
// to stop stays registered and approved, since it may still be serving.
func (r *localGatewayRegistry) Stop(ctx context.Context, suffix string) error {
	r.mu.Lock()
	gateway := r.gateways[suffix]
	r.mu.Unlock()
	if gateway == nil {
		return nil
	}
	if err := gateway.stack.Stop(ctx); err != nil {
		return fmt.Errorf("stopping local gateway %s: %w", suffix, err)
	}
	r.mu.Lock()
	delete(r.gateways, suffix)
	r.mu.Unlock()
	return nil
}

// isLocalGatewayTarget reports whether target is on the runtime of a gateway the calling runner
// started and has not stopped.
func isLocalGatewayTarget(ctx context.Context, target *url.URL) bool {
	if target == nil || target.User != nil || target.Host == "" {
		return false
	}
	registry, err := LocalGatewaysOf(ctx, false)
	if err != nil || registry == nil {
		return false
	}
	return registry.approved(strings.ToLower(target.Scheme + "://" + target.Host))
}
