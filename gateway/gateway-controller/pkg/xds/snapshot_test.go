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

package xds

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func makeRestAPI(uuid, name, ctx string) *models.StoredConfig {
	cfg := api.RestAPI{
		Kind:     api.RestAPIKindRestApi,
		Metadata: api.Metadata{Name: name},
		Spec: api.APIConfigData{
			DisplayName: name,
			Version:     "v1.0",
			Context:     ctx,
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: api.Upstream{Url: api.Ptr("http://backend:8080")},
			},
			Operations: []api.Operation{
				{Method: api.Ptr(api.OperationMethodGET), Path: api.Ptr("/resource")},
			},
		},
	}
	return &models.StoredConfig{
		UUID:                uuid,
		Kind:                models.KindRestApi,
		Handle:              name,
		DisplayName:         name,
		Version:             "v1.0",
		DesiredState:        models.StateDeployed,
		Configuration:       cfg,
		SourceConfiguration: cfg,
	}
}

// TestConcurrentUpdateSnapshot reproduces a race where two goroutines call
// UpdateSnapshot and the final snapshot only contains api-one instead of
// both api-one and api-two. The slower goroutine reads the store before
// api-two is added, but writes last with a higher version, overwriting
// the correct snapshot.
//
// A test hook forces this interleaving:
//
//	Step 1: A: GetAll() → [api-one]
//	             ↓ hook blocks A
//	Step 2: Main: store.Add(api-two)
//	Step 3: B: GetAll() → [api-one, api-two]
//	           SetSnapshot(v2, both)    ✓
//	             ↓ hook releases A
//	Step 4: A: SetSnapshot(v3, [api-one])  ✗ api-two lost
//
// Without mutex: A wins (higher version, stale data) → FAIL
// With mutex:    A blocks until B finishes, then re-reads → PASS
func TestConcurrentUpdateSnapshot(t *testing.T) {
	t.Run("stale GetAll cannot overwrite a newer complete snapshot", func(t *testing.T) {
		metrics.Init()
		store := storage.NewConfigStore()

		if err := store.Add(makeRestAPI("uuid-api-1", "api-one", "/api-one")); err != nil {
			t.Fatalf("Add api-one: %v", err)
		}

		sm := NewSnapshotManager(store, createTestLogger(), testRouterConfig(), nil, testConfig())

		// Step 1: A calls UpdateSnapshot, blocks after GetAll([api-one])
		aGotAll := make(chan struct{})
		bDone := make(chan struct{})

		// Only block the first caller (A); let B pass through.
		var hooked atomic.Bool
		sm.afterGetAll = func() {
			if !hooked.CompareAndSwap(false, true) {
				return
			}
			close(aGotAll)
			<-bDone
		}

		errA := make(chan error, 1)
		go func() {
			errA <- sm.UpdateSnapshot(context.Background(), "corr-A")
		}()
		<-aGotAll

		// Step 2: add api-two behind A's back
		if err := store.Add(makeRestAPI("uuid-api-2", "api-two", "/api-two")); err != nil {
			t.Fatalf("Add api-two: %v", err)
		}

		// Step 3: B sees [api-one, api-two], completes, releases A
		errB := make(chan error, 1)
		go func() {
			err := sm.UpdateSnapshot(context.Background(), "corr-B")
			close(bDone)
			errB <- err
		}()

		// Step 4: A resumes with stale [api-one], overwrites B's snapshot
		if err := <-errB; err != nil {
			t.Fatalf("goroutine B UpdateSnapshot: %v", err)
		}
		if err := <-errA; err != nil {
			t.Fatalf("goroutine A UpdateSnapshot: %v", err)
		}

		assertSnapshotContainsAPIs(t, sm, []string{"/api-one", "/api-two"})
	})
}

func assertSnapshotContainsAPIs(t *testing.T, sm *SnapshotManager, expectedContexts []string) {
	t.Helper()

	snap, err := sm.GetCache().GetSnapshot("router-node")
	if err != nil {
		t.Fatalf("GetSnapshot: %v", err)
	}

	var routePaths []string
	for _, res := range snap.GetResources(resource.RouteType) {
		routeCfg, ok := res.(*route.RouteConfiguration)
		if !ok {
			continue
		}
		for _, vh := range routeCfg.GetVirtualHosts() {
			for _, r := range vh.GetRoutes() {
				if regex, ok := r.GetMatch().GetPathSpecifier().(*route.RouteMatch_SafeRegex); ok {
					routePaths = append(routePaths, regex.SafeRegex.GetRegex())
				}
			}
		}
	}

	for _, ctx := range expectedContexts {
		found := false
		for _, p := range routePaths {
			if strings.Contains(p, ctx) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("snapshot after concurrent UpdateSnapshot is missing %s; got routes: %v", ctx, routePaths)
		}
	}
}
