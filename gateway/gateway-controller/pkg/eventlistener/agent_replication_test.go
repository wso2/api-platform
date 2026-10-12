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

package eventlistener

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/agent"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// failingSecretResolver stands in for a secret store from which every secret has
// been deleted.
type failingSecretResolver struct{}

func (failingSecretResolver) Resolve(string) (string, error) {
	return "", errors.New("secret not found")
}

// envoyRouteCountFor counts the routes in the replica's current Envoy snapshot
// whose match names context, so an assertion is about one Agent's routes rather
// than the snapshot as a whole. A replica that has never built a snapshot has
// none.
func envoyRouteCountFor(t *testing.T, sm *xds.SnapshotManager, context string) int {
	t.Helper()
	snap, err := sm.GetCache().GetSnapshot("router-node")
	if err != nil {
		return 0
	}
	count := 0
	for _, res := range snap.GetResources(resource.RouteType) {
		routeCfg, ok := res.(*route.RouteConfiguration)
		if !ok {
			continue
		}
		for _, vh := range routeCfg.GetVirtualHosts() {
			for _, r := range vh.GetRoutes() {
				if strings.Contains(r.GetMatch().String(), context) {
					count++
				}
			}
		}
	}
	return count
}

// fanOutEventHub delivers every published event synchronously to each replica's
// listener, in publish order — the delivery the real hub provides to every
// controller sharing a gateway ID, the publisher included, minus the polling.
type fanOutEventHub struct {
	mu        sync.Mutex
	replicas  []*agentReplica
	published []eventhub.Event
}

func (h *fanOutEventHub) Initialize() error            { return nil }
func (h *fanOutEventHub) RegisterGateway(string) error { return nil }

func (h *fanOutEventHub) PublishEvent(_ string, event eventhub.Event) error {
	h.mu.Lock()
	h.published = append(h.published, event)
	replicas := append([]*agentReplica(nil), h.replicas...)
	h.mu.Unlock()
	for _, replica := range replicas {
		replica.listener.handleEvent(event)
	}
	return nil
}

func (h *fanOutEventHub) Subscribe(string) (<-chan eventhub.Event, error) { return nil, nil }
func (h *fanOutEventHub) Unsubscribe(string, <-chan eventhub.Event) error { return nil }
func (h *fanOutEventHub) UnsubscribeAll(string) error                     { return nil }
func (h *fanOutEventHub) CleanUpEvents() error                            { return nil }
func (h *fanOutEventHub) Close() error                                    { return nil }

func (h *fanOutEventHub) actions() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, 0, len(h.published))
	for _, e := range h.published {
		out = append(out, e.Action)
	}
	return out
}

// replicatedAgentYAML is a control-plane Agent artifact in the gateway shape the
// deploy event path applies: a managed public card whose advertised interface
// matches the JSONRPC transport's effective path.
const replicatedAgentYAML = `
apiVersion: gateway.api-platform.wso2.com/v1
kind: Agent
metadata:
  name: replicated-agent
spec:
  displayName: Replicated Agent
  version: v1.0
  context: /replicated-agent
  upstream:
    url: https://replicated.internal
  a2a:
    protocolVersion: "1.0"
    operationConfigs:
      transports:
        - protocolBinding: JSONRPC
          pathPrefix: /rpc
    agentCard:
      public:
        mode: managed
        content: {
          "name": "Replicated Agent",
          "version": "1.0.0",
          "supportedInterfaces": [
            {"protocolBinding": "JSONRPC", "protocolVersion": "1.0", "url": "https://agents.example.com/replicated-agent/rpc"}
          ],
          "capabilities": {},
          "defaultInputModes": ["text/plain"],
          "defaultOutputModes": ["text/plain"],
          "skills": []
        }
`

// Two controllers share a database and an event hub. Every write goes through
// one of them, via the same AgentService calls the control-plane event handlers
// make; the other learns of it only from the event. Deploy, undeploy, redeploy
// and delete must each leave both replicas holding identical Agent state in the
// config store, the Envoy snapshot and the policy snapshot.
func TestAgentLifecycle_ConvergesAcrossTwoReplicas(t *testing.T) {
	metrics.Init()
	db := setupSQLiteDBForEventListenerTests(t)

	replicaA := newAgentReplica(t, db)
	replicaB := newAgentReplica(t, db)
	hub := &fanOutEventHub{replicas: []*agentReplica{replicaA, replicaB}}

	// The writer is replica A's control-plane-facing service. It writes the
	// shared database and announces; it never touches either replica's stores.
	service := agent.NewAgentService(
		replicaA.store,
		db,
		config.NewParser(),
		config.NewAgentValidator(),
		newTestLogger(),
		hub,
		stubSecretResolver{value: "42"},
		"test-gateway",
	)

	const artifactID = "5f0b2a7e-3c1d-4e8f-9a6b-7c2d1e0f3a4b"
	const handle = "replicated-agent"
	runtimeKey := storage.Key(models.KindAgent, handle)

	assertConverged := func(t *testing.T, stage string, wantState models.DesiredState, wantChains bool) {
		t.Helper()
		for name, replica := range map[string]*agentReplica{"A": replicaA, "B": replicaB} {
			stored, err := replica.store.Get(artifactID)
			require.NoErrorf(t, err, "%s: replica %s store", stage, name)
			assert.Equalf(t, wantState, stored.DesiredState, "%s: replica %s desired state", stage, name)

			_, hasChains := replica.runtimeStore.Get(runtimeKey)
			assert.Equalf(t, wantChains, hasChains, "%s: replica %s policy chains", stage, name)

			routes := envoyRouteCountFor(t, replica.snapshotManager, "/"+handle)
			if wantChains {
				assert.NotZerof(t, routes, "%s: replica %s Envoy routes", stage, name)
			} else {
				assert.Zerof(t, routes, "%s: replica %s Envoy routes", stage, name)
			}
		}
	}

	// Deploy.
	deployedAt := time.Now().Truncate(time.Millisecond)
	_, err := service.CreateFromYAML([]byte(replicatedAgentYAML), artifactID, "dep-1", &deployedAt,
		"corr-deploy", newTestLogger())
	require.NoError(t, err)
	assertConverged(t, "deploy", models.StateDeployed, true)

	// Undeploy.
	undeployedAt := deployedAt.Add(time.Second)
	_, err = service.Undeploy(agent.UndeployParams{
		ID: artifactID, DeploymentID: "dep-1", PerformedAt: &undeployedAt,
		CorrelationID: "corr-undeploy", Logger: newTestLogger(),
	})
	require.NoError(t, err)
	assertConverged(t, "undeploy", models.StateUndeployed, false)

	// Redeploy (restore sends a deploy event, which lands here).
	redeployedAt := undeployedAt.Add(time.Second)
	_, err = service.CreateFromYAML([]byte(replicatedAgentYAML), artifactID, "dep-2", &redeployedAt,
		"corr-redeploy", newTestLogger())
	require.NoError(t, err)
	assertConverged(t, "redeploy", models.StateDeployed, true)

	// Delete.
	_, err = service.Delete(agent.DeleteParams{Handle: handle, CorrelationID: "corr-delete", Logger: newTestLogger()})
	require.NoError(t, err)
	for name, replica := range map[string]*agentReplica{"A": replicaA, "B": replicaB} {
		_, err := replica.store.Get(artifactID)
		assert.ErrorIsf(t, err, storage.ErrNotFound, "delete: replica %s store", name)
		_, hasChains := replica.runtimeStore.Get(runtimeKey)
		assert.Falsef(t, hasChains, "delete: replica %s policy chains", name)
		assert.Zerof(t, envoyRouteCountFor(t, replica.snapshotManager, "/"+handle), "delete: replica %s Envoy routes", name)
	}

	assert.Equal(t, []string{"CREATE", "UPDATE", "UPDATE", "DELETE"}, hub.actions(),
		"one announcement per write, each with the action its replicas converge on")
}
