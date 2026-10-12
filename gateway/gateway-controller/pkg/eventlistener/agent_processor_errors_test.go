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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// A create/update event for an Agent whose row is gone (deleted between publish
// and consumption) changes nothing on the replica.
func TestHandleEvent_AgentCreate_MissingRowIsIgnored(t *testing.T) {
	db := setupSQLiteDBForEventListenerTests(t)
	replica := newAgentReplica(t, db)
	snapshotVersionBefore := replica.store.GetSnapshotVersion()

	replica.listener.handleEvent(agentEvent("CREATE", "agent-row-gone", "corr-agent-row-gone"))

	assert.Empty(t, replica.store.GetAll())
	assert.Empty(t, replica.runtimeStore.GetAll())
	assert.Equal(t, snapshotVersionBefore, replica.store.GetSnapshotVersion())
}

// A deployed Agent whose templates no longer resolve is not converged: pushing
// the unrendered template text to the runtime would ship a literal
// `{{ secret … }}` as a policy parameter.
func TestHandleEvent_AgentCreate_RenderFailureConvergesNothing(t *testing.T) {
	db := setupSQLiteDBForEventListenerTests(t)
	cfg := testAgentStoredConfig("agent-render-fail-id", "weather-agent", "Weather Agent", "v1.0",
		models.StateDeployed, withAgentOperationPolicy(api.Policy{
			Name:    "rate-limit",
			Version: "v1",
			Params:  &map[string]any{"limit": `{{ secret "agent-limit" }}`},
		}))
	require.NoError(t, db.SaveConfig(cfg))

	replica := newAgentReplica(t, db)
	replica.listener.secretResolver = failingSecretResolver{}

	replica.listener.handleEvent(agentEvent("CREATE", cfg.UUID, "corr-agent-render-fail"))

	_, err := replica.store.Get(cfg.UUID)
	require.ErrorIs(t, err, storage.ErrNotFound)
	assert.Empty(t, replica.runtimeStore.GetAll())
}

// occupyAgentHandle places a different Agent under handle in the replica's
// store, so a later Add or Update of another Agent with that handle conflicts.
func occupyAgentHandle(t *testing.T, replica *agentReplica, uuid, handle string) {
	t.Helper()
	require.NoError(t, replica.store.Add(
		testAgentStoredConfig(uuid, handle, "Occupant "+uuid, "v9.9", models.StateDeployed)))
}

// When the in-memory store refuses a new Agent (its handle is held by another
// configuration), neither lane is touched — for a deployed Agent and for one
// that arrives already undeployed.
func TestHandleEvent_AgentCreate_StoreAddConflictConvergesNothing(t *testing.T) {
	for _, state := range []models.DesiredState{models.StateDeployed, models.StateUndeployed} {
		t.Run(string(state), func(t *testing.T) {
			db := setupSQLiteDBForEventListenerTests(t)
			cfg := testAgentStoredConfig("agent-add-conflict-id", "weather-agent", "Weather Agent", "v1.0", state)
			require.NoError(t, db.SaveConfig(cfg))

			replica := newAgentReplica(t, db)
			occupyAgentHandle(t, replica, "occupant-id", cfg.Handle)
			snapshotVersionBefore := replica.store.GetSnapshotVersion()

			replica.listener.handleEvent(agentEvent("CREATE", cfg.UUID, "corr-agent-add-conflict"))

			_, err := replica.store.Get(cfg.UUID)
			require.ErrorIs(t, err, storage.ErrNotFound)
			assert.Len(t, replica.store.GetAll(), 1, "only the occupant remains")
			assert.Empty(t, replica.runtimeStore.GetAll())
			assert.Equal(t, snapshotVersionBefore, replica.store.GetSnapshotVersion(),
				"a refused store write must not rebuild the Envoy snapshot")
		})
	}
}

// When the in-memory store refuses an update (the Agent was renamed onto a
// handle another configuration holds), the replica keeps the copy it had.
func TestHandleEvent_AgentUpdate_StoreUpdateConflictKeepsPreviousCopy(t *testing.T) {
	db := setupSQLiteDBForEventListenerTests(t)
	cfg := testAgentStoredConfig("agent-update-conflict-id", "weather-agent", "Weather Agent", "v1.0", models.StateDeployed)
	require.NoError(t, db.SaveConfig(cfg))

	replica := newAgentReplica(t, db)
	replica.listener.handleEvent(agentEvent("CREATE", cfg.UUID, "corr-agent-create"))
	_, err := replica.store.Get(cfg.UUID)
	require.NoError(t, err, "precondition: the Agent converged")

	occupyAgentHandle(t, replica, "occupant-id", "taken-agent")
	renamed := testAgentStoredConfig(cfg.UUID, "taken-agent", cfg.DisplayName, cfg.Version, models.StateDeployed)
	require.NoError(t, db.UpdateConfig(renamed))

	replica.listener.handleEvent(agentEvent("UPDATE", cfg.UUID, "corr-agent-update-conflict"))

	stored, err := replica.store.Get(cfg.UUID)
	require.NoError(t, err)
	assert.Equal(t, "weather-agent", stored.Handle, "the refused update must not replace the stored copy")
	_, exists := replica.runtimeStore.Get(storage.Key(models.KindAgent, "taken-agent"))
	assert.False(t, exists, "no chains converge under the refused handle")
}

// A failure to drop the runtime config is logged, not fatal: the undeploy and
// the delete still take effect in the config store and the Envoy snapshot.
func TestHandleEvent_AgentRuntimeConfigRemovalFailureIsNotFatal(t *testing.T) {
	t.Run("undeploy", func(t *testing.T) {
		db := setupSQLiteDBForEventListenerTests(t)
		cfg := testAgentStoredConfig("agent-undeploy-rm-fail-id", "weather-agent", "Weather Agent", "v1.0", models.StateUndeployed)
		require.NoError(t, db.SaveConfig(cfg))

		replica := newAgentReplica(t, db)
		// A policy manager with no runtime store fails every removal.
		replica.listener.policyManager = policyxds.NewPolicyManager(nil, newTestLogger())

		replica.listener.handleEvent(agentEvent("UPDATE", cfg.UUID, "corr-agent-undeploy-rm-fail"))

		stored, err := replica.store.Get(cfg.UUID)
		require.NoError(t, err)
		assert.Equal(t, models.StateUndeployed, stored.DesiredState)
	})

	t.Run("delete", func(t *testing.T) {
		db := setupSQLiteDBForEventListenerTests(t)
		cfg := testAgentStoredConfig("agent-delete-rm-fail-id", "weather-agent", "Weather Agent", "v1.0", models.StateDeployed)
		require.NoError(t, db.SaveConfig(cfg))

		replica := newAgentReplica(t, db)
		replica.listener.handleEvent(agentEvent("CREATE", cfg.UUID, "corr-agent-create"))
		_, err := replica.store.Get(cfg.UUID)
		require.NoError(t, err, "precondition: the Agent converged")

		replica.listener.policyManager = policyxds.NewPolicyManager(nil, newTestLogger())
		snapshotVersionBefore := replica.store.GetSnapshotVersion()

		replica.listener.handleEvent(agentEvent("DELETE", cfg.UUID, "corr-agent-delete-rm-fail"))

		_, err = replica.store.Get(cfg.UUID)
		require.ErrorIs(t, err, storage.ErrNotFound)
		assert.Greater(t, replica.store.GetSnapshotVersion(), snapshotVersionBefore,
			"the Envoy snapshot still drops the deleted Agent")
	})
}
