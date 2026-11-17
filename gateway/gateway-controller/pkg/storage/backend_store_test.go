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
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func newBackendTestStore(t *testing.T) *sqlStore {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	store, err := NewStorage(BackendConfig{Type: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "test.db"), GatewayID: "gw-1"}, logger)
	assert.NilError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	return store.(*sqlStore)
}

func storedBackend(uuid, name, basePath string) *models.StoredBackend {
	labels := map[string]string{"team": "orders"}
	return &models.StoredBackend{
		UUID: uuid,
		Configuration: api.BackendRequest{
			ApiVersion: api.BackendRequestApiVersionGatewayChoreolabsComv1,
			Kind:       api.BackendRequestKindBackend,
			Metadata:   api.Metadata{Name: name, Labels: &labels},
			Spec: api.BackendConfiguration{
				BasePath: &basePath,
				Endpoints: []struct {
					Url    string `json:"url" yaml:"url"`
					Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
				}{{Url: "http://" + name + ":9080"}},
			},
		},
	}
}

func TestBackendStorage_Lifecycle(t *testing.T) {
	db := newBackendTestStore(t)
	es, ok := AsBackendStorage(db)
	assert.Assert(t, ok)

	assert.NilError(t, es.SaveBackend(storedBackend("u-1", "tenant-b", "/v1")))
	assert.NilError(t, es.SaveBackend(storedBackend("u-2", "tenant-a", "/v2")))

	got, err := es.GetBackendByHandle("tenant-b")
	assert.NilError(t, err)
	assert.Equal(t, got.UUID, "u-1")
	assert.Equal(t, got.BasePath(), "/v1")
	assert.Equal(t, got.Origin, models.OriginGatewayAPI)
	assert.Equal(t, got.Labels()["team"], "orders")

	all, err := es.GetAllBackends()
	assert.NilError(t, err)
	assert.Equal(t, len(all), 2)
	assert.Equal(t, all[0].GetHandle(), "tenant-a", "ordered by name")

	updated := storedBackend("u-1", "tenant-b", "/v9")
	assert.NilError(t, es.UpdateBackend(updated))
	got, err = es.GetBackend("u-1")
	assert.NilError(t, err)
	assert.Equal(t, got.BasePath(), "/v9")

	assert.NilError(t, es.DeleteBackend("u-1"))
	_, err = es.GetBackend("u-1")
	assert.Assert(t, IsNotFoundError(err))
	assert.Assert(t, IsNotFoundError(es.DeleteBackend("u-1")))
	assert.Assert(t, IsNotFoundError(es.UpdateBackend(storedBackend("u-404", "tenant-x", "/x"))))
}

func TestBackendStorage_DuplicateNameConflicts(t *testing.T) {
	db := newBackendTestStore(t)
	assert.NilError(t, db.SaveBackend(storedBackend("u-1", "tenant-a", "/v1")))
	err := db.SaveBackend(storedBackend("u-2", "tenant-a", "/v1"))
	assert.Assert(t, IsConflictError(err), "got %v", err)
}

func TestBackendStorage_ArtifactRowStaysOutOfAPIsAndControlPlanePush(t *testing.T) {
	db := newBackendTestStore(t)
	assert.NilError(t, db.SaveBackend(storedBackend("u-1", "tenant-a", "/v1")))

	var kind, origin string
	var cpSync *string
	assert.NilError(t, db.queryRow(`SELECT kind, origin, cp_sync_status FROM artifacts WHERE uuid = ?`, "u-1").
		Scan(&kind, &origin, &cpSync))
	assert.Equal(t, kind, models.KindBackend)
	assert.Equal(t, origin, string(models.OriginGatewayAPI))
	assert.Assert(t, cpSync == nil, "no pending control-plane sync until the control plane imports Backends")

	configs, err := db.GetAllConfigs()
	assert.NilError(t, err)
	assert.Equal(t, len(configs), 0, "a Backend never enters API hydration")

	pending, err := db.GetPendingCPSyncArtifacts()
	assert.NilError(t, err)
	assert.Equal(t, len(pending), 0)
}

func TestBackendStore_InMemory(t *testing.T) {
	s := NewBackendStore()
	assert.NilError(t, s.Upsert(storedBackend("u-2", "tenant-b", "/b")))
	assert.NilError(t, s.Upsert(storedBackend("u-1", "tenant-a", "/a")))
	assert.Assert(t, s.Upsert(&models.StoredBackend{}) != nil)

	list := s.ListBackends()
	assert.Equal(t, len(list), 2)
	assert.Equal(t, list[0].GetHandle(), "tenant-a")

	got, ok := s.GetByUUID("u-2")
	assert.Assert(t, ok)
	assert.Equal(t, got.GetHandle(), "tenant-b")

	assert.Equal(t, s.Delete("tenant-b").UUID, "u-2")
	assert.Assert(t, s.Delete("tenant-b") == nil)

	s.ReplaceAll([]*models.StoredBackend{storedBackend("u-9", "tenant-z", "/z")})
	_, ok = s.Get("tenant-a")
	assert.Assert(t, !ok)
	_, ok = s.Get("tenant-z")
	assert.Assert(t, ok)
}

// The handle column indexes the name, so an update cannot rename the Backend stored under a UUID.
func TestBackendStorage_UpdateCannotRename(t *testing.T) {
	db := newBackendTestStore(t)
	es, _ := AsBackendStorage(db)
	assert.NilError(t, es.SaveBackend(storedBackend("u-1", "tenant-a", "/v1")))

	err := es.UpdateBackend(storedBackend("u-1", "tenant-b", "/v2"))
	assert.ErrorContains(t, err, "cannot be renamed")
	got, err := es.GetBackendByHandle("tenant-a")
	assert.NilError(t, err)
	assert.Equal(t, got.BasePath(), "/v1", "a refused rename changes nothing")

	assert.NilError(t, es.UpdateBackend(storedBackend("u-1", "tenant-a", "/v2")))
	got, err = es.GetBackendByHandle("tenant-a")
	assert.NilError(t, err)
	assert.Equal(t, got.BasePath(), "/v2")
}

// A Backend row belongs to its artifacts row: deleting the Backend removes both, and removing the
// artifacts row directly removes the Backend too, so no orphan Backend remains.
func TestBackendStorage_RowIsTiedToItsArtifactsRow(t *testing.T) {
	db := newBackendTestStore(t)
	es, _ := AsBackendStorage(db)
	assert.NilError(t, es.SaveBackend(storedBackend("u-1", "tenant-a", "/v1")))
	assert.NilError(t, es.SaveBackend(storedBackend("u-2", "tenant-b", "/v1")))

	count := func(query string, args ...any) int {
		var n int
		assert.NilError(t, db.db.QueryRow(query, args...).Scan(&n))
		return n
	}

	assert.NilError(t, es.DeleteBackend("u-1"))
	assert.Equal(t, count(`SELECT COUNT(*) FROM backends WHERE uuid = ?`, "u-1"), 0)
	assert.Equal(t, count(`SELECT COUNT(*) FROM artifacts WHERE uuid = ?`, "u-1"), 0)

	_, err := db.db.Exec(`DELETE FROM artifacts WHERE uuid = ?`, "u-2")
	assert.NilError(t, err)
	assert.Equal(t, count(`SELECT COUNT(*) FROM backends WHERE uuid = ?`, "u-2"), 0)
	_, err = es.GetBackend("u-2")
	assert.Assert(t, IsNotFoundError(err))
}
