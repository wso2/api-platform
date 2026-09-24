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

package controlplane

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// Agent API keys reach the gateway two ways: the kind-agnostic apikey.* events
// the control plane broadcasts when a key is created, updated or revoked, and the
// bulk backfill run on every (re)connect. Both are exercised here against the
// real SQLite store an Agent is deployed into.

const (
	agentKeyName = "agent-consumer-key"
	agentKeyUUID = "0199a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"
	// agentKeyHashes is the apiKeyHashes value the control plane sends: a JSON
	// object of algorithm → hex digest, never the plain key.
	agentKeyHashes = `{"sha256": "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"}`
)

// keyBackfillServer answers the gateway-internal /{kind}/api-keys routes. It
// records the paths it was asked for, in order, and serves bodies[path] (or an
// empty array) for each.
type keyBackfillServer struct {
	*httptest.Server
	mu     sync.Mutex
	paths  []string
	bodies map[string]string
}

func newKeyBackfillServer(t *testing.T, bodies map[string]string) *keyBackfillServer {
	t.Helper()
	s := &keyBackfillServer{bodies: bodies}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.paths = append(s.paths, r.URL.Path)
		body, ok := s.bodies[r.URL.Path]
		s.mu.Unlock()
		if !ok {
			body = `[]`
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *keyBackfillServer) requested() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.paths...)
}

// withKeyServices wires the API-key collaborators the harness does not need for
// deployment events, pointing the backfill at srv.
func (h *agentEventsHarness) withKeyServices(srv *keyBackfillServer) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h.client.ctx = context.Background()
	h.client.apiKeyStore = storage.NewAPIKeyStore(logger)
	h.client.apiKeyService = utils.NewAPIKeyService(h.client.store, h.db, nil, &config.APIKeyConfig{
		APIKeysPerUserPerAPI: 10,
		Algorithm:            constants.HashingAlgorithmSHA256,
		MinKeyLength:         constants.DefaultMinAPIKeyLength,
		MaxKeyLength:         constants.DefaultMaxAPIKeyLength,
	}, h.hub, agentEvtGatewayID)
	if srv != nil {
		h.client.apiUtilsService = utils.NewAPIUtilsService(utils.PlatformAPIConfig{
			BaseURL: srv.URL,
			Token:   agentEvtToken,
		}, srv.Client(), logger)
	}
}

func (h *agentEventsHarness) apiKeyEvents(action string) []eventhub.Event {
	var out []eventhub.Event
	for _, e := range h.hub.publishedEvents {
		if e.event.EventType == eventhub.EventTypeAPIKey && e.event.Action == action {
			out = append(out, e.event)
		}
	}
	return out
}

func apiKeyCreatedEvt(t *testing.T, artifactID string, expiresAt *time.Time) map[string]any {
	now := time.Now().UTC()
	payload := map[string]any{
		"uuid":         agentKeyUUID,
		"apiId":        artifactID,
		"name":         agentKeyName,
		"apiKeyHashes": agentKeyHashes,
		"maskedApiKey": "***6e7ae",
		"createdAt":    now.Format(time.RFC3339),
		"updatedAt":    now.Format(time.RFC3339),
	}
	if expiresAt != nil {
		payload["expiresAt"] = expiresAt.Format(time.RFC3339)
	}
	return agentEvtEvent(t, "apikey.created", payload, "corr-key-created")
}

// --- event path -------------------------------------------------------------

// An apikey.created event for a deployed Agent stores an active key against the
// Agent's artifact UUID and announces it to every replica — with no Agent-specific
// code on the gateway, because api_keys association is artifact_uuid only.
func TestHandleAPIKeyCreatedEvent_StoresKeyForDeployedAgent(t *testing.T) {
	h := newAgentEventsHarness(t)
	h.deploy(t, agentEvtID, "dep-1", time.Now())
	h.withKeyServices(nil)

	h.client.handleAPIKeyCreatedEvent(apiKeyCreatedEvt(t, agentEvtID, nil))

	key, err := h.db.GetAPIKeysByAPIAndName(agentEvtID, agentKeyName)
	require.NoError(t, err)
	require.NotNil(t, key)
	assert.Equal(t, agentKeyUUID, key.UUID)
	assert.Equal(t, agentEvtID, key.ArtifactUUID)
	assert.Equal(t, models.APIKeyStatusActive, key.Status)
	assert.NotEmpty(t, key.APIKey, "the key must be stored in its hashed, verifiable form")
	assert.NotContains(t, key.APIKey, "{", "the stored key is the digest, not the hashes JSON document")

	created := h.apiKeyEvents("CREATE")
	require.Len(t, created, 1, "the key must be announced so every replica rebuilds its API-key snapshot")
}

// An apikey.updated event — what a control-plane PUT sends, as for REST API
// keys — replaces the Agent key's material with the rotated hashes and applies
// the new expiry.
func TestHandleAPIKeyUpdatedEvent_RotatesAgentKey(t *testing.T) {
	h := newAgentEventsHarness(t)
	h.deploy(t, agentEvtID, "dep-1", time.Now())
	h.withKeyServices(nil)
	h.client.handleAPIKeyCreatedEvent(apiKeyCreatedEvt(t, agentEvtID, nil))

	before, err := h.db.GetAPIKeysByAPIAndName(agentEvtID, agentKeyName)
	require.NoError(t, err)
	require.NotNil(t, before)

	const rotatedHashes = `{"sha256": "fcde2b2edba56bf408601fb721fe9b5c338d10ee429ea04fae5511b68fbf8fb9"}`
	expiresAt := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	updated := agentEvtEvent(t, "apikey.updated", map[string]any{
		"apiId":        agentEvtID,
		"keyName":      agentKeyName,
		"apiKeyHashes": rotatedHashes,
		"maskedApiKey": "***f8fb9",
		"expiresAt":    expiresAt.Format(time.RFC3339),
		"updatedAt":    time.Now().Add(time.Second).UTC().Format(time.RFC3339),
	}, "corr-key-updated")
	h.client.handleAPIKeyUpdatedEvent(updated)

	after, err := h.db.GetAPIKeysByAPIAndName(agentEvtID, agentKeyName)
	require.NoError(t, err)
	require.NotNil(t, after)
	assert.NotEqual(t, before.APIKey, after.APIKey, "the update must replace the key material")
	assert.Contains(t, after.APIKey, "fcde2b2edba56bf408601fb721fe9b5c338d10ee429ea04fae5511b68fbf8fb9")
	assert.Equal(t, before.UUID, after.UUID, "a rotation keeps the key's identity")
	assert.Equal(t, models.APIKeyStatusActive, after.Status)
	require.NotNil(t, after.ExpiresAt)
	assert.True(t, after.ExpiresAt.Equal(expiresAt), "expiry: got %v, want %v", after.ExpiresAt, expiresAt)
	assert.NotEmpty(t, h.apiKeyEvents("UPDATE"), "the rotation must be announced to replicas")
}

// An apikey.revoked event for an Agent key takes it out of service.
func TestHandleAPIKeyRevokedEvent_RevokesAgentKey(t *testing.T) {
	h := newAgentEventsHarness(t)
	h.deploy(t, agentEvtID, "dep-1", time.Now())
	h.withKeyServices(nil)
	h.client.handleAPIKeyCreatedEvent(apiKeyCreatedEvt(t, agentEvtID, nil))

	h.client.handleAPIKeyRevokedEvent(agentEvtEvent(t, "apikey.revoked", map[string]any{
		"apiId":   agentEvtID,
		"keyName": agentKeyName,
	}, "corr-key-revoked"))

	h.assertNoAgentKey(t, "a revoked key must be taken out of service")
	assert.NotEmpty(t, h.apiKeyEvents("DELETE"), "the revocation must be announced to replicas")
}

// assertNoAgentKey proves the fixture key is no longer stored for the Agent.
func (h *agentEventsHarness) assertNoAgentKey(t *testing.T, msg string) {
	t.Helper()
	key, err := h.db.GetAPIKeysByAPIAndName(agentEvtID, agentKeyName)
	if err != nil {
		require.True(t, storage.IsNotFoundError(err), "unexpected error: %v", err)
		return
	}
	assert.Nil(t, key, msg)
}

// --- bulk backfill ------------------------------------------------------------

// On reconnect the gateway pulls Agent keys from /agents/api-keys and stores
// them against the local Agent. The five pre-existing kinds are still fetched,
// in their original order, with Agent appended after them.
func TestSyncAPIKeysForExistingArtifacts_BackfillsAgentKeys(t *testing.T) {
	h := newAgentEventsHarness(t)
	h.deploy(t, agentEvtID, "dep-1", time.Now())

	srv := newKeyBackfillServer(t, map[string]string{
		"/agents/api-keys": `[{
			"uuid": "` + agentKeyUUID + `",
			"name": "` + agentKeyName + `",
			"maskedApiKey": "***6e7ae",
			"apiKeyHashes": {"sha256": "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"},
			"artifactUuid": "` + agentEvtID + `",
			"status": "active",
			"createdAt": "2026-09-20T10:00:00Z",
			"updatedAt": "2026-09-20T10:00:00Z",
			"source": "external"
		}]`,
	})
	h.withKeyServices(srv)

	h.client.syncAPIKeysForExistingArtifacts(agentEvtGatewayID)

	assert.Equal(t, []string{
		"/apis/api-keys",
		"/websub-apis/api-keys",
		"/webbroker-apis/api-keys",
		"/llm-providers/api-keys",
		"/llm-proxies/api-keys",
		"/agents/api-keys",
	}, srv.requested(), "every existing kind is still synced, and Agent is added")

	key, err := h.db.GetAPIKeysByAPIAndName(agentEvtID, agentKeyName)
	require.NoError(t, err)
	require.NotNil(t, key, "the backfilled Agent key must be stored")
	assert.Equal(t, agentEvtID, key.ArtifactUUID)
	assert.Equal(t, models.APIKeyStatusActive, key.Status)
}

// A key revoked in the control plane while the gateway was disconnected is no
// longer returned by the backfill, so the reconcile step removes it from the
// Agent — the same deletion path every other kind uses.
func TestSyncAPIKeysForExistingArtifacts_ReconcilesAgentKeyRemovedUpstream(t *testing.T) {
	h := newAgentEventsHarness(t)
	h.deploy(t, agentEvtID, "dep-1", time.Now())
	h.withKeyServices(nil)
	h.client.handleAPIKeyCreatedEvent(apiKeyCreatedEvt(t, agentEvtID, nil))
	existing, err := h.db.GetAPIKeysByAPIAndName(agentEvtID, agentKeyName)
	require.NoError(t, err)
	require.NotNil(t, existing, "precondition: the key is present before the backfill")

	srv := newKeyBackfillServer(t, nil) // the control plane no longer reports the key
	h.withKeyServices(srv)

	h.client.syncAPIKeysForExistingArtifacts(agentEvtGatewayID)

	h.assertNoAgentKey(t, "a key the control plane no longer reports must be reconciled away")
	assert.NotEmpty(t, h.apiKeyEvents("DELETE"), "the reconcile deletion must be announced to replicas")
}

// On-prem APIM exposes no /agents/api-keys route, so an on-prem gateway must not
// call it — only the RestApi backfill runs there.
func TestSyncAPIKeysForExistingArtifacts_OnPremSkipsAgentBackfill(t *testing.T) {
	h := newAgentEventsHarness(t)
	h.deploy(t, agentEvtID, "dep-1", time.Now())
	srv := newKeyBackfillServer(t, nil)
	h.withKeyServices(srv)
	h.client.gatewayPath = "/internal/gateway" // marks the control plane as on-prem

	h.client.syncAPIKeysForExistingArtifacts(agentEvtGatewayID)

	assert.Equal(t, []string{"/apis/api-keys"}, srv.requested())
}

// Every kind the bulk sync iterates must have a FetchAPIKeysByKind path arm —
// otherwise that kind's backfill fails on every reconnect with nothing but a
// warning log to show for it.
func TestAPIKeyBulkSyncKinds_AllHaveFetchPath(t *testing.T) {
	srv := newKeyBackfillServer(t, nil)
	svc := utils.NewAPIUtilsService(utils.PlatformAPIConfig{BaseURL: srv.URL, Token: agentEvtToken},
		srv.Client(), slog.New(slog.NewTextHandler(io.Discard, nil)))

	require.True(t, slices.Contains(apiKeyBulkSyncKinds, models.KindAgent), "Agent keys must be bulk-synced")
	for _, kind := range apiKeyBulkSyncKinds {
		_, err := svc.FetchAPIKeysByKind(kind, "")
		assert.NoError(t, err, "kind %s has no API-key backfill path", kind)
	}
	assert.False(t, onPremSupportedAPIKeyKinds[models.KindAgent],
		"on-prem APIM has no Agent backfill route; Agent must stay cloud-only")
}

// FetchAPIKeysByKind selects /agents/api-keys for the gateway's Agent kind and
// forwards the issuer filter, exactly as it does for the other kinds.
func TestFetchAPIKeysByKind_AgentPathAndIssuer(t *testing.T) {
	var gotPath, gotIssuer, gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotIssuer, gotToken = r.URL.Path, r.URL.Query().Get("issuer"), r.Header.Get("api-key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"uuid": "k1", "name": "active-key", "maskedApiKey": "***aaaaa",
			"apiKeyHashes": {"sha256": "abc"}, "artifactUuid": "` + agentEvtID + `", "status": "active"
		}, {
			"uuid": "k2", "name": "revoked-key", "maskedApiKey": "***bbbbb",
			"apiKeyHashes": {"sha256": "def"}, "artifactUuid": "` + agentEvtID + `", "status": "revoked"
		}]`))
	}))
	t.Cleanup(srv.Close)
	svc := utils.NewAPIUtilsService(utils.PlatformAPIConfig{BaseURL: srv.URL, Token: agentEvtToken},
		srv.Client(), slog.New(slog.NewTextHandler(io.Discard, nil)))

	keys, err := svc.FetchAPIKeysByKind(models.KindAgent, "api-platform-devportal")
	require.NoError(t, err)
	assert.Equal(t, "/agents/api-keys", gotPath)
	assert.Equal(t, "api-platform-devportal", gotIssuer)
	assert.Equal(t, agentEvtToken, gotToken)
	require.Len(t, keys, 1, "only active keys are synced")
	assert.Equal(t, "active-key", keys[0].Name)
}
