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

package policyxds

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

// The gateway health routes are always published to the policy engine with an empty
// chain, whether or not any API is deployed, so a policy engine that predates their
// handling passes health probes through instead of rejecting them.
func TestUpdateSnapshotPublishesGatewayHealthRoutes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		withAPI  bool
		apiRoute string
	}{
		{name: "no APIs deployed"},
		{name: "an API deployed", withAPI: true, apiRoute: "GET|/petstore/v1/pets|localhost"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			sm := NewSnapshotManager(logger)
			pm := NewPolicyManager(sm, logger)
			store := newTestRuntimeStore()
			sm.SetRuntimeStore(store)
			pm.SetRuntimeStore(store)
			if tc.withAPI {
				require.NoError(t, pm.AddRuntimeConfig("RestApi:petstore", restRDC()))
			}

			require.NoError(t, sm.UpdateSnapshot(context.Background()))

			routes := sm.routeCache.GetResources()
			chains := sm.policyCache.GetResources()
			for routeKey, path := range map[string]string{
				constants.GatewayReadyRouteName:   constants.GatewayReadyPath,
				constants.GatewayHealthyRouteName: constants.GatewayHealthyPath,
			} {
				require.Contains(t, routes, routeKey)
				rc := decodeRouteConfig(t, routes[routeKey])
				assert.Equal(t, routeKey, rc["route_key"])
				assert.Equal(t, "route-key", rc["resolver_name"], "directly resolved by its route key")
				assert.Equal(t, routeKey, rc["canonical_chain_key"])
				assert.NotContains(t, rc, "default_upstream", "the health routes have no upstream")
				metadata := rc["metadata"].(map[string]interface{})
				assert.Equal(t, path, metadata["path"])
				assert.Empty(t, metadata["uuid"], "no API identity")

				require.Contains(t, chains, routeKey)
				chain := decodeRouteConfig(t, chains[routeKey])
				configured := chain["configuration"].(map[string]interface{})["routes"].([]interface{})
				require.Len(t, configured, 1)
				entry := configured[0].(map[string]interface{})
				assert.Equal(t, routeKey, entry["route_key"])
				assert.Empty(t, entry["policies"], "an empty chain runs no policy")
			}
			if tc.withAPI {
				assert.Contains(t, routes, tc.apiRoute, "the API's own routes are still published")
			}
		})
	}
}
