/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package controlplane

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

func saveTestMCPProxy(t *testing.T, db storage.Storage, uuid, cpArtifactID, handle string) {
	t.Helper()
	proxy := api.MCPProxyConfiguration{
		ApiVersion: api.MCPProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.MCPProxyConfigurationKindMcp,
		Metadata:   api.Metadata{Name: handle},
		Spec:       api.MCPProxyConfigData{DisplayName: handle, Version: "v1.0"},
	}
	require.NoError(t, db.SaveConfig(&models.StoredConfig{
		UUID: uuid, CPArtifactID: cpArtifactID, Kind: models.KindMcp, Handle: handle, DisplayName: handle,
		Version: "v1.0", SourceConfiguration: proxy, Configuration: proxy, DesiredState: models.StateDeployed,
	}))
}

// The bulk sync covers MCP proxies; a proxy the control plane can't answer for keeps its subscriptions.
func TestClient_syncSubscriptionsForExistingAPIs_CoversMCPProxies(t *testing.T) {
	metrics.Init()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := storage.NewStorage(storage.BackendConfig{
		Type: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "mcp-sync.db"), GatewayID: "test-gateway",
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	saveTestMCPProxy(t, db, "mcp-synced", "mcp-cp", "synced-mcp")
	saveTestMCPProxy(t, db, "mcp-unknown", "", "unknown-mcp")
	for _, sub := range []*models.Subscription{
		{ID: "sub-stale", APIID: "mcp-synced", SubscriptionToken: "stale-token", Status: models.SubscriptionStatusActive},
		{ID: "sub-kept", APIID: "mcp-unknown", SubscriptionToken: "kept-token", Status: models.SubscriptionStatusActive},
	} {
		require.NoError(t, db.SaveSubscription(sub))
	}

	var fetched []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetched = append(fetched, r.URL.Path)
		switch r.URL.Path {
		case "/apis/mcp-cp/subscriptions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `[{"id":"sub-cp","apiId":"mcp-cp","subscriptionToken":"cp-token","status":"ACTIVE"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := &Client{
		logger: logger, db: db, eventHub: &mockControlPlaneEventHub{}, gatewayID: "test-gateway",
		ctx: context.Background(), state: &ConnectionState{}, stopChan: make(chan struct{}),
		apiUtilsService: utils.NewAPIUtilsService(utils.PlatformAPIConfig{BaseURL: srv.URL, Token: "t"}, srv.Client(), logger),
	}
	client.syncSubscriptionsForExistingAPIs("test-gateway")

	assert.ElementsMatch(t, []string{"/apis/mcp-cp/subscriptions", "/apis/mcp-unknown/subscriptions"}, fetched)

	synced, err := db.ListSubscriptionsByAPI("mcp-synced", "", nil, nil)
	require.NoError(t, err)
	require.Len(t, synced, 1)
	assert.Equal(t, "sub-cp", synced[0].ID)

	kept, err := db.ListSubscriptionsByAPI("mcp-unknown", "", nil, nil)
	require.NoError(t, err)
	require.Len(t, kept, 1)
	assert.Equal(t, "sub-kept", kept[0].ID)
}

type restListFailingStorage struct{ storage.Storage }

func (s restListFailingStorage) GetAllConfigsByKind(kind string) ([]*models.StoredConfig, error) {
	if kind == models.KindRestApi {
		return nil, errors.New("database unavailable")
	}
	return s.Storage.GetAllConfigsByKind(kind)
}

// When one kind can't be listed, the other kind still syncs.
func TestClient_syncSubscriptionsForExistingAPIs_SkipsKindThatCantBeListed(t *testing.T) {
	metrics.Init()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := storage.NewStorage(storage.BackendConfig{
		Type: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "mcp-sync-fail.db"), GatewayID: "test-gateway",
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	saveTestMCPProxy(t, db, "mcp-listed", "", "listed-mcp")

	var fetched []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetched = append(fetched, r.URL.Path)
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := &Client{
		logger: logger, db: restListFailingStorage{db}, eventHub: &mockControlPlaneEventHub{}, gatewayID: "test-gateway",
		ctx: context.Background(), state: &ConnectionState{}, stopChan: make(chan struct{}),
		apiUtilsService: utils.NewAPIUtilsService(utils.PlatformAPIConfig{BaseURL: srv.URL, Token: "t"}, srv.Client(), logger),
	}
	client.syncSubscriptionsForExistingAPIs("test-gateway")
	assert.Equal(t, []string{"/apis/mcp-listed/subscriptions"}, fetched)
}
