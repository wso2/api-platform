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
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/vault"
)

// flakyResolver is unavailable until up is set.
type flakyResolver struct{ up atomic.Bool }

func (f *flakyResolver) Resolve(string) (string, error) {
	if !f.up.Load() {
		return "", fmt.Errorf("%w: request to openbao failed", vault.ErrUnavailable)
	}
	return "https://backend.example.com", nil
}

func newRetryingListener(t *testing.T, store *storage.ConfigStore, db storage.Storage, resolver *flakyResolver) *EventListener {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	r := newRenderRetries()
	r.base, r.max = 10*time.Millisecond, 20*time.Millisecond
	return &EventListener{store: store, db: db, logger: newTestLogger(), secretResolver: resolver,
		retries: r, ctx: ctx, cancel: cancel}
}

func TestReplica_RetriesUntilSecretStoreRecoversAndAppliesLatestRevision(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	cfg := testRestStoredConfig("api-retry-id", "retry-api", "Retry API", "v1.0.0", models.StateDeployed)
	restAPI := cfg.SourceConfiguration.(api.RestAPI)
	restAPI.Spec.Upstream.Main.Url = stringPtr(`{{ secret "kv://bao/apps/backend" }}`)
	cfg.Configuration, cfg.SourceConfiguration = restAPI, restAPI
	require.NoError(t, db.SaveConfig(cfg))

	resolver := &flakyResolver{}
	listener := newRetryingListener(t, store, db, resolver)
	listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeAPI, Action: "CREATE", EntityID: cfg.UUID, EventID: "e1"})

	_, err := store.Get(cfg.UUID)
	assert.True(t, storage.IsNotFoundError(err), "nothing is applied while the secret is unreadable")

	// The first replay still fails and is rescheduled.
	replay := <-listener.retries.ch
	listener.handleEvent(replay)
	_, err = store.Get(cfg.UUID)
	assert.True(t, storage.IsNotFoundError(err))

	// A newer revision lands while the store is down; the replay must apply it.
	latest := testRestStoredConfig(cfg.UUID, cfg.Handle, "Retry API v2", cfg.Version, models.StateDeployed)
	latest.Configuration, latest.SourceConfiguration = restAPI, restAPI
	require.NoError(t, db.UpdateConfig(latest))
	resolver.up.Store(true)

	select {
	case replay = <-listener.retries.ch:
	case <-time.After(time.Second):
		t.Fatal("no replay after the second failure")
	}
	listener.handleEvent(replay)

	stored, err := store.Get(cfg.UUID)
	require.NoError(t, err)
	assert.Equal(t, "Retry API v2", stored.DisplayName, "the replay re-reads the latest stored revision")
	rendered := stored.Configuration.(api.RestAPI)
	assert.Equal(t, "https://backend.example.com", *rendered.Spec.Upstream.Main.Url)
	listener.retries.mu.Lock()
	defer listener.retries.mu.Unlock()
	assert.Empty(t, listener.retries.attempts, "backoff is cleared once applied")
}

func TestRetryIfUnavailable_IgnoresOtherErrorsAndCoalesces(t *testing.T) {
	listener := newRetryingListener(t, nil, nil, &flakyResolver{})
	event := eventhub.Event{EventType: eventhub.EventTypeAPI, EntityID: "a"}

	assert.False(t, listener.retryIfUnavailable(event, errors.New("secret not found")),
		"a bad config is not retried")
	assert.True(t, listener.retryIfUnavailable(event, vault.ErrUnavailable))
	assert.True(t, listener.retryIfUnavailable(event, vault.ErrUnavailable))

	<-listener.retries.ch
	select {
	case <-listener.retries.ch:
		t.Fatal("only one replay per entity is pending")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRenderRetries_DelayIsBounded(t *testing.T) {
	r := newRenderRetries()
	assert.Equal(t, retryBaseDelay, r.delay(1))
	assert.Equal(t, 2*retryBaseDelay, r.delay(2))
	assert.Equal(t, retryMaxDelay, r.delay(50))
}

func TestRetryDeferred_QueuesStartupArtifacts(t *testing.T) {
	listener := newRetryingListener(t, nil, nil, &flakyResolver{})
	listener.RetryDeferred([]*models.StoredConfig{
		{UUID: "llm-1", Kind: models.KindLlmProvider},
		{UUID: "ws-1", Kind: models.KindWebSubApi},
	})
	replay := <-listener.retries.ch
	assert.Equal(t, eventhub.EventTypeLLMProvider, replay.EventType)
	assert.Equal(t, "llm-1", replay.EntityID)
}
