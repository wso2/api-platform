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

package storage

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

var agentCounter int

// createTestAgentConfig builds an Agent artifact with enough of the a2a subtree
// to prove the nested structure survives a database round-trip — a single
// scalar would pass even if the JSON payload were flattened on the way in.
func createTestAgentConfig() *models.StoredConfig {
	agentCounter++
	handle := fmt.Sprintf("test-agent-%d", agentCounter)

	pathPrefix := "/rpc"
	cardPath := "/.well-known/agent-card.json"

	agentConfig := api.AgentConfiguration{
		ApiVersion: api.AgentConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.AgentConfigurationKindAgent,
		Metadata:   api.Metadata{Name: handle},
		Spec: api.AgentConfigData{
			DisplayName: fmt.Sprintf("Test Agent %d", agentCounter),
			Version:     "v1.0",
			Context:     strPtrAgent(fmt.Sprintf("/test-agent-%d", agentCounter)),
			Upstream: api.AgentConfigData_Upstream{
				Url: strPtrAgent("https://agent.internal"),
			},
			A2a: api.A2AConfig{
				ProtocolVersion: "1.0",
				OperationConfigs: api.A2AOperationConfigs{
					Transports: []api.A2ATransport{
						{ProtocolBinding: api.JSONRPC, PathPrefix: &pathPrefix},
					},
					Policies: &[]api.Policy{
						{Name: "jwt-auth", Version: "v1", Params: &map[string]interface{}{
							"issuer": "https://idp.example.com",
						}},
					},
					Operations: &[]api.A2AOperationConfig{
						{Name: api.SendMessage},
					},
				},
				AgentCard: api.A2AAgentCard{
					Public: api.A2APublicAgentCard{
						Mode: api.A2APublicAgentCardModeManaged,
						Path: &cardPath,
						Content: &api.A2AAgentCardDocument{
							"name":                "Test Agent",
							"description":         "Round-trips through storage",
							"version":             "1.0.0",
							"defaultInputModes":   []interface{}{"text/plain"},
							"defaultOutputModes":  []interface{}{"text/plain"},
							"skills":              []interface{}{},
							"capabilities":        map[string]interface{}{"streaming": true},
							"supportedInterfaces": []interface{}{map[string]interface{}{"protocolBinding": "JSONRPC", "protocolVersion": "1.0", "url": "https://agents.example.com/test/rpc"}},
						},
					},
				},
			},
		},
	}

	return &models.StoredConfig{
		UUID:                fmt.Sprintf("test-agent-uuid-%d", agentCounter),
		Kind:                models.KindAgent,
		Handle:              handle,
		DisplayName:         fmt.Sprintf("Test Agent %d", agentCounter),
		Version:             "v1.0",
		Configuration:       agentConfig,
		SourceConfiguration: agentConfig,
		DesiredState:        models.StateDeployed,
		Origin:              models.OriginGatewayAPI,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
}

func strPtrAgent(s string) *string { return &s }

func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	assert.NilError(t, err)
	return string(b)
}

// assertAgentRoundTripped checks the parts of an Agent that a lossy persistence
// path would quietly drop.
func assertAgentRoundTripped(t *testing.T, cfg *models.StoredConfig) {
	t.Helper()

	// Non-nil Configuration is the whole point of the RestApi lane: Agent has no
	// hydrate step, so whatever a read path returns is what the transformer sees.
	assert.Assert(t, cfg.Configuration != nil, "Configuration is nil; nothing rebuilds it for Agent")

	agent, ok := cfg.Configuration.(api.AgentConfiguration)
	assert.Assert(t, ok, "Configuration is %T, want api.AgentConfiguration", cfg.Configuration)

	source, ok := cfg.SourceConfiguration.(api.AgentConfiguration)
	assert.Assert(t, ok, "SourceConfiguration is %T, want api.AgentConfiguration", cfg.SourceConfiguration)
	// Compared as JSON rather than structurally: the generated upstream type
	// holds a union with an unexported field, which a reflective comparison
	// cannot walk. JSON is also the form these are actually stored in.
	assert.Equal(t, mustJSON(t, agent), mustJSON(t, source),
		"Configuration and SourceConfiguration diverged; the Agent lane sets both from one payload")

	assert.Equal(t, agent.Kind, api.AgentConfigurationKindAgent)
	assert.Equal(t, string(agent.Spec.A2a.ProtocolVersion), "1.0")
	assert.Equal(t, len(agent.Spec.A2a.OperationConfigs.Transports), 1)
	assert.Equal(t, agent.Spec.A2a.OperationConfigs.Transports[0].ProtocolBinding, api.JSONRPC)

	assert.Assert(t, agent.Spec.A2a.OperationConfigs.Policies != nil)
	assert.Equal(t, (*agent.Spec.A2a.OperationConfigs.Policies)[0].Name, "jwt-auth")

	// The embedded Agent Card is free-form JSON; nested values are where a
	// re-serialisation bug would show up first.
	content := agent.Spec.A2a.AgentCard.Public.Content
	assert.Assert(t, content != nil, "Agent Card content was dropped")
	card := map[string]interface{}(*content)
	assert.Equal(t, card["name"], "Test Agent")
	capabilities, ok := card["capabilities"].(map[string]interface{})
	assert.Assert(t, ok, "capabilities is %T after round-trip", card["capabilities"])
	assert.Equal(t, capabilities["streaming"], true)
	interfaces, ok := card["supportedInterfaces"].([]interface{})
	assert.Assert(t, ok, "supportedInterfaces is %T after round-trip", card["supportedInterfaces"])
	assert.Equal(t, len(interfaces), 1)
}

// TestAgentSurvivesRestart is Section 2's headline: an Agent written before a
// restart is readable after one, through every read path a restart uses.
// GetAllConfigs in particular is what populates the in-memory cache at startup.
func TestAgentSurvivesRestart(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.db.Close()

	original := createTestAgentConfig()
	assert.NilError(t, storage.SaveConfig(original))

	t.Run("GetConfig", func(t *testing.T) {
		got, err := storage.GetConfig(original.UUID)
		assert.NilError(t, err)
		assertAgentRoundTripped(t, got)
		assert.Equal(t, got.Handle, original.Handle)
		assert.Equal(t, got.Kind, models.KindAgent)
	})

	t.Run("GetConfigByKindAndHandle", func(t *testing.T) {
		got, err := storage.GetConfigByKindAndHandle(models.KindAgent, original.Handle)
		assert.NilError(t, err)
		assertAgentRoundTripped(t, got)
	})

	t.Run("GetConfigByKindNameAndVersion", func(t *testing.T) {
		got, err := storage.GetConfigByKindNameAndVersion(models.KindAgent, original.DisplayName, original.Version)
		assert.NilError(t, err)
		assertAgentRoundTripped(t, got)
	})

	t.Run("GetAllConfigs", func(t *testing.T) {
		configs, err := storage.GetAllConfigs()
		assert.NilError(t, err)

		var found *models.StoredConfig
		for _, cfg := range configs {
			if cfg.UUID == original.UUID {
				found = cfg
			}
		}
		assert.Assert(t, found != nil, "Agent missing from GetAllConfigs; a restart would not load it")
		assertAgentRoundTripped(t, found)
	})

	t.Run("GetAllConfigsByKind", func(t *testing.T) {
		configs, err := storage.GetAllConfigsByKind(models.KindAgent)
		assert.NilError(t, err)
		assert.Equal(t, len(configs), 1)
		assertAgentRoundTripped(t, configs[0])
	})
}

// TestAgentUniquenessConstraints checks the artifacts-table constraints apply to
// Agents. The agents table carries no name/handle columns of its own, so these
// are inherited rather than restated — which is exactly why they are worth
// asserting for this kind.
func TestAgentUniquenessConstraints(t *testing.T) {
	t.Run("same handle is rejected", func(t *testing.T) {
		storage := setupTestStorage(t)
		defer storage.db.Close()

		first := createTestAgentConfig()
		assert.NilError(t, storage.SaveConfig(first))

		duplicate := createTestAgentConfig()
		duplicate.Handle = first.Handle

		assert.Assert(t, storage.SaveConfig(duplicate) != nil, "a second Agent with the same handle was accepted")
	})

	t.Run("same display name and version is rejected", func(t *testing.T) {
		storage := setupTestStorage(t)
		defer storage.db.Close()

		first := createTestAgentConfig()
		assert.NilError(t, storage.SaveConfig(first))

		duplicate := createTestAgentConfig()
		duplicate.DisplayName = first.DisplayName
		duplicate.Version = first.Version

		assert.Assert(t, storage.SaveConfig(duplicate) != nil, "a second Agent with the same display name and version was accepted")
	})

	t.Run("a RestApi may reuse an Agent's handle", func(t *testing.T) {
		storage := setupTestStorage(t)
		defer storage.db.Close()

		agent := createTestAgentConfig()
		assert.NilError(t, storage.SaveConfig(agent))

		// The unique constraints are scoped by kind, so this must be allowed —
		// asserting it keeps a future "tighten uniqueness" change honest.
		restAPI := createTestStoredConfig()
		restAPI.Handle = agent.Handle
		assert.NilError(t, storage.SaveConfig(restAPI))
	})
}

// TestAgentUpdateAndDelete covers the remaining write paths against the agents
// table, including the cascade from artifacts.
func TestAgentUpdateAndDelete(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.db.Close()

	agent := createTestAgentConfig()
	assert.NilError(t, storage.SaveConfig(agent))

	updated, ok := agent.Configuration.(api.AgentConfiguration)
	assert.Assert(t, ok)
	updated.Spec.DisplayName = "Renamed Agent"
	agent.Configuration = updated
	agent.SourceConfiguration = updated
	agent.DisplayName = "Renamed Agent"
	assert.NilError(t, storage.UpdateConfig(agent))

	got, err := storage.GetConfig(agent.UUID)
	assert.NilError(t, err)
	assertAgentRoundTripped(t, got)
	reread, ok := got.Configuration.(api.AgentConfiguration)
	assert.Assert(t, ok)
	assert.Equal(t, reread.Spec.DisplayName, "Renamed Agent")

	assert.NilError(t, storage.DeleteConfig(agent.UUID))

	_, err = storage.GetConfig(agent.UUID)
	assert.Assert(t, err != nil, "Agent still readable after delete")

	// The agents row must go with the artifact, not linger and collide with a
	// later Agent reusing the UUID.
	var rows int
	assert.NilError(t, storage.db.QueryRow("SELECT COUNT(*) FROM agents WHERE uuid = ?", agent.UUID).Scan(&rows))
	assert.Equal(t, rows, 0, "agents row survived the artifact delete")
}

// TestAgentSignedCardColumns covers the two columns nothing populates yet.
// They are written and read here directly so the plumbing is proven before card
// signing depends on it, and so the NULL default is a tested state rather than
// an assumed one.
func TestAgentSignedCardColumns(t *testing.T) {
	t.Run("nil by default", func(t *testing.T) {
		storage := setupTestStorage(t)
		defer storage.db.Close()

		agent := createTestAgentConfig()
		assert.NilError(t, storage.SaveConfig(agent))

		got, err := storage.GetConfig(agent.UUID)
		assert.NilError(t, err)
		assert.Assert(t, got.SignedPublicCard() == nil, "unsigned Agent came back with a signed card")
		assert.Assert(t, got.SignedProtectedCard() == nil)
		// A nil Agent must mean exactly one thing — no gateway-generated cards —
		// so an empty struct is never allocated for a row with both columns NULL.
		assert.Assert(t, got.Agent == nil, "an empty AgentArtifact was allocated for an unsigned Agent")
	})

	t.Run("round-trips through every read path", func(t *testing.T) {
		storage := setupTestStorage(t)
		defer storage.db.Close()

		const signed = `{"name":"Test Agent","signatures":[{"protected":"eyJhbGciOiJFUzI1NiJ9","signature":"c2ln"}]}`

		agent := createTestAgentConfig()
		agent.Agent = &models.AgentArtifact{SignedPublicCard: strPtrAgent(signed)}
		assert.NilError(t, storage.SaveConfig(agent))

		got, err := storage.GetConfig(agent.UUID)
		assert.NilError(t, err)
		assert.Assert(t, got.SignedPublicCard() != nil, "signed card was not persisted")
		assert.Equal(t, *got.SignedPublicCard(), signed)

		// A restart reads through GetAllConfigs. Signatures are produced only on
		// deploy, so if this path dropped them the gateway would come back up
		// serving an unsigned card with nothing to indicate why.
		configs, err := storage.GetAllConfigsByKind(models.KindAgent)
		assert.NilError(t, err)
		assert.Equal(t, len(configs), 1)
		assert.Assert(t, configs[0].SignedPublicCard() != nil, "signed card lost on the startup read path")
		assert.Equal(t, *configs[0].SignedPublicCard(), signed)

		all, err := storage.GetAllConfigs()
		assert.NilError(t, err)
		for _, cfg := range all {
			if cfg.UUID == agent.UUID {
				assert.Assert(t, cfg.SignedPublicCard() != nil, "signed card lost in the cross-kind union")
				assert.Equal(t, *cfg.SignedPublicCard(), signed)
			}
		}
	})

	t.Run("an update that omits the signature clears it", func(t *testing.T) {
		storage := setupTestStorage(t)
		defer storage.db.Close()

		agent := createTestAgentConfig()
		agent.Agent = &models.AgentArtifact{SignedPublicCard: strPtrAgent(`{"stale":"signature"}`)}
		assert.NilError(t, storage.SaveConfig(agent))

		// A signature is only valid for the exact card bytes it was computed
		// over, so an update that changes the card without re-signing must drop
		// the old signature rather than keep serving one that verifies against
		// content nobody receives.
		agent.Agent = nil
		assert.NilError(t, storage.UpdateConfig(agent))

		got, err := storage.GetConfig(agent.UUID)
		assert.NilError(t, err)
		assert.Assert(t, got.SignedPublicCard() == nil, "a stale signature survived an update that did not re-sign")
		assert.Assert(t, got.Agent == nil)
	})
}

// TestAgentUpsert covers the third write path. The event listener that
// propagates an Agent between replicas upserts rather than inserts, so this is
// the path a multi-replica deployment actually takes.
func TestAgentUpsert(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.db.Close()

	agent := createTestAgentConfig()
	agent.DeploymentID = "dep-1"

	applied, err := storage.UpsertConfig(agent)
	assert.NilError(t, err)
	assert.Assert(t, applied, "first upsert was skipped")

	got, err := storage.GetConfig(agent.UUID)
	assert.NilError(t, err)
	assertAgentRoundTripped(t, got)

	// Second upsert of the same artifact: an update, not a duplicate row.
	updated, ok := agent.Configuration.(api.AgentConfiguration)
	assert.Assert(t, ok)
	updated.Spec.DisplayName = "Upserted Agent"
	agent.Configuration = updated
	agent.SourceConfiguration = updated
	agent.DisplayName = "Upserted Agent"
	agent.DeploymentID = "dep-2"
	agent.Agent = &models.AgentArtifact{SignedPublicCard: strPtrAgent(`{"signed":"card"}`)}

	applied, err = storage.UpsertConfig(agent)
	assert.NilError(t, err)
	assert.Assert(t, applied, "second upsert was skipped")

	got, err = storage.GetConfig(agent.UUID)
	assert.NilError(t, err)
	assertAgentRoundTripped(t, got)
	reread, ok := got.Configuration.(api.AgentConfiguration)
	assert.Assert(t, ok)
	assert.Equal(t, reread.Spec.DisplayName, "Upserted Agent")
	assert.Assert(t, got.SignedPublicCard() != nil, "upsert did not persist the signed card")
	assert.Equal(t, *got.SignedPublicCard(), `{"signed":"card"}`)

	var rows int
	assert.NilError(t, storage.db.QueryRow("SELECT COUNT(*) FROM agents WHERE uuid = ?", agent.UUID).Scan(&rows))
	assert.Equal(t, rows, 1, "upsert inserted a duplicate agents row")
}

// TestSignedCardColumnsAreAgentOnly is the regression half of threading two new
// columns through shared storage code: every other kind must read back exactly
// as before, with both fields nil.
func TestSignedCardColumnsAreAgentOnly(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.db.Close()

	restAPI := createTestStoredConfig()
	assert.NilError(t, storage.SaveConfig(restAPI))

	agent := createTestAgentConfig()
	assert.NilError(t, storage.SaveConfig(agent))

	byUUID, err := storage.GetConfig(restAPI.UUID)
	assert.NilError(t, err)
	assert.Assert(t, byUUID.SignedPublicCard() == nil, "RestApi read back with a signed card")
	assert.Assert(t, byUUID.SignedProtectedCard() == nil)
	assert.Assert(t, byUUID.Agent == nil, "a non-Agent artifact was given an AgentArtifact")
	assert.Assert(t, byUUID.Configuration != nil, "RestApi lost its Configuration")

	byKind, err := storage.GetAllConfigsByKind(models.KindRestApi)
	assert.NilError(t, err)
	assert.Equal(t, len(byKind), 1)
	assert.Assert(t, byKind[0].SignedPublicCard() == nil)
	assert.Assert(t, byKind[0].Configuration != nil)

	// The cross-kind union projects NULL placeholders for non-Agent tables. If
	// the arity or the column order were wrong, this scan is where it surfaces.
	all, err := storage.GetAllConfigs()
	assert.NilError(t, err)
	assert.Equal(t, len(all), 2)
	for _, cfg := range all {
		assert.Assert(t, cfg.Configuration != nil, "%s lost its Configuration in the union", cfg.Kind)
		if cfg.Kind != models.KindAgent {
			assert.Assert(t, cfg.SignedPublicCard() == nil, "%s read back with a signed card", cfg.Kind)
			assert.Assert(t, cfg.SignedProtectedCard() == nil)
			assert.Assert(t, cfg.Agent == nil, "%s was given an AgentArtifact", cfg.Kind)
		}
	}
}
