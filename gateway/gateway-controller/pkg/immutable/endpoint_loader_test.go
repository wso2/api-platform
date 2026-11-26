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

package immutable

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

const endpointArtifact = `apiVersion: gateway.api-platform.wso2.com/v1
kind: Endpoint
metadata:
  name: tenant-a
  labels:
    team: orders
spec:
  basePath: /v1
  upstreams:
    - url: http://orders-tenant-a:9080
`

// newEndpointGW builds the Agent-only gateway and adds the Endpoint service
// over the same database. It returns the in-memory store the service applies
// Endpoints to.
func newEndpointGW(t *testing.T, artifactsDir string) (*ImmutableGW, storage.Storage, *storage.ConfigStore) {
	t.Helper()
	gw, db := newAgentOnlyGW(t, artifactsDir)
	store := storage.NewConfigStore()
	gw.SetEndpointService(utils.NewEndpointService(
		db, store, utils.NewEndpointPublisher(db, store, nil, nil), discardEventHub{}, "test-gateway",
		config.NewEndpointValidator(nil), nil, slog.New(slog.NewTextHandler(io.Discard, nil)),
	))
	return gw, db, store
}

func TestLoadArtifacts_AppliesEndpoint(t *testing.T) {
	dir := t.TempDir()
	writeArtifact(t, dir, "tenant-a.yaml", endpointArtifact)
	gw, db, store := newEndpointGW(t, dir)

	require.NoError(t, gw.LoadArtifacts(slog.New(slog.NewTextHandler(io.Discard, nil))))

	stored, err := db.GetEndpointByHandle("tenant-a")
	require.NoError(t, err)
	assert.Equal(t, "/v1", stored.BasePath())
	assert.Equal(t, map[string]string{"team": "orders"}, stored.Labels())
	_, routable := store.GetEndpoint("tenant-a")
	assert.True(t, routable, "the Endpoint must reach the store the translator reads")
}

func TestLoadArtifacts_AppliesEndpointFromJSON(t *testing.T) {
	dir := t.TempDir()
	writeArtifact(t, dir, "tenant-b.json", `{"apiVersion":"gateway.api-platform.wso2.com/v1","kind":"Endpoint",
		"metadata":{"name":"tenant-b"},"spec":{"upstreams":[{"url":"http://orders-tenant-b:9080"}]}}`)
	gw, db, _ := newEndpointGW(t, dir)

	require.NoError(t, gw.LoadArtifacts(slog.New(slog.NewTextHandler(io.Discard, nil))))

	_, err := db.GetEndpointByHandle("tenant-b")
	require.NoError(t, err)
}

// Endpoints and the artifacts beside them load whatever the file order is.
func TestLoadArtifacts_EndpointsLoadAlongsideOtherKinds(t *testing.T) {
	dir := t.TempDir()
	// "a-" sorts the Agent's file before the Endpoint's.
	writeArtifact(t, dir, "a-weather-agent.yaml", agentArtifact)
	writeArtifact(t, dir, "z-tenant-a.yaml", endpointArtifact)
	gw, db, _ := newEndpointGW(t, dir)

	require.NoError(t, gw.LoadArtifacts(slog.New(slog.NewTextHandler(io.Discard, nil))))

	_, err := db.GetEndpointByHandle("tenant-a")
	require.NoError(t, err)
	endpoints, err := db.GetAllEndpoints()
	require.NoError(t, err)
	assert.Len(t, endpoints, 1)
}

func TestLoadArtifacts_InvalidEndpointFailsTheLoad(t *testing.T) {
	dir := t.TempDir()
	writeArtifact(t, dir, "bad.yaml", `apiVersion: gateway.api-platform.wso2.com/v1
kind: Endpoint
metadata:
  name: Not_A_Valid_Name
spec:
  upstreams:
    - url: http://orders:9080
`)
	gw, db, _ := newEndpointGW(t, dir)

	err := gw.LoadArtifacts(slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad.yaml")
	endpoints, listErr := db.GetAllEndpoints()
	require.NoError(t, listErr)
	assert.Empty(t, endpoints)
}

// A gateway built without Endpoint support reports the artifact rather than
// skipping it silently.
func TestLoadArtifacts_EndpointWithoutServiceFails(t *testing.T) {
	dir := t.TempDir()
	writeArtifact(t, dir, "tenant-a.yaml", endpointArtifact)
	gw, _ := newAgentOnlyGW(t, dir)

	err := gw.LoadArtifacts(slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endpoints are not supported")
}
