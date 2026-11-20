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

package backend

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/upstreamguard"
)

// SnapshotUpdater rebuilds and pushes the Envoy xDS snapshot.
type SnapshotUpdater interface {
	UpdateSnapshot(ctx context.Context, correlationID string) error
}

// Syncer applies Backend changes on this replica: from the database into memory, then into the
// router's clusters and the policy engine's records.
//
// Two rules keep a request from reaching a cluster Envoy lacks, or a cluster that does not match
// its record:
//   - A record is published only for a Backend revision whose cluster was in the last xDS
//     snapshot that was applied (the installed set). A revision whose cluster update failed keeps
//     its previous record, which still matches the cluster Envoy holds.
//   - A deleted Backend's cluster stays in every snapshot (the retained set) until its record has
//     been withdrawn.
type Syncer struct {
	// mu serialises each read from the database with its apply, for local changes and replica
	// events alike, so an older read can never overwrite a newer one in memory.
	mu sync.Mutex

	db        storage.BackendStorage
	store     *storage.BackendStore
	snapshot  SnapshotUpdater
	publisher *Publisher
	logger    *slog.Logger

	// stateMu guards retained, listed and installed, and makes a delete's move from the store to
	// retained one step for the cluster listing (both hold it). It is never held while the snapshot
	// updater or the publisher runs.
	stateMu sync.Mutex
	// retained holds Backends removed from the store whose records may still be published.
	retained map[string]*models.StoredBackend
	// listed is what the cluster view last returned to a translation.
	listed map[string]*models.StoredBackend
	// installed is the set of Backend revisions in the last applied snapshot.
	installed map[string]*models.StoredBackend
}

// NewSyncer creates a Syncer. snapshot and publisher may be nil in tests. The publisher is bound
// to the syncer's installed revisions, so it only ever publishes records whose clusters Envoy has.
func NewSyncer(db storage.BackendStorage, store *storage.BackendStore, snapshot SnapshotUpdater, publisher *Publisher, logger *slog.Logger) *Syncer {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Syncer{
		db: db, store: store, snapshot: snapshot, publisher: publisher, logger: logger,
		retained:  map[string]*models.StoredBackend{},
		listed:    map[string]*models.StoredBackend{},
		installed: map[string]*models.StoredBackend{},
	}
	if publisher != nil {
		publisher.backends = RecordView{s: s}
	}
	return s
}

// ClusterView lists the Backends whose clusters belong in the xDS snapshot: every stored Backend,
// plus deleted ones whose records are not withdrawn yet. Give it to the xDS translator.
func (s *Syncer) ClusterView() ClusterView { return ClusterView{s: s} }

// ClusterView is the Backends the router must have clusters for.
type ClusterView struct{ s *Syncer }

// ListBackends lists the Backends for a translation and remembers them, so SnapshotApplied knows
// which revisions the applied snapshot holds. The store and the retained set are read in one
// stateMu section, which a delete also holds while it moves a Backend from one to the other, so
// a listing never misses a Backend whose record may still be published.
func (v ClusterView) ListBackends() []*models.StoredBackend {
	s := v.s
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	byName := map[string]*models.StoredBackend{}
	for _, backend := range s.store.ListBackends() {
		byName[backend.GetHandle()] = backend
	}
	for name, backend := range s.retained {
		if _, stored := byName[name]; !stored {
			byName[name] = backend
		}
	}
	s.listed = byName
	return sorted(byName)
}

// ClustersBuilt narrows the last listing to the Backends whose clusters the translation built.
// The translator skips a Backend whose cluster cannot be built, so that revision is not in the
// applied snapshot; leaving it out keeps it from being installed and getting a record.
func (v ClusterView) ClustersBuilt(names []string) {
	s := v.s
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	built := make(map[string]*models.StoredBackend, len(names))
	for _, name := range names {
		if backend, ok := s.listed[name]; ok {
			built[name] = backend
		}
	}
	s.listed = built
}

// RecordView lists the Backend revisions whose records may be published: the installed revision
// of every Backend still in the store.
type RecordView struct{ s *Syncer }

// ListBackends lists the publishable revisions.
func (v RecordView) ListBackends() []*models.StoredBackend {
	s := v.s
	out := map[string]*models.StoredBackend{}
	s.stateMu.Lock()
	for name, backend := range s.installed {
		if _, stored := s.store.Get(name); stored {
			out[name] = backend
		}
	}
	s.stateMu.Unlock()
	return sorted(out)
}

// SnapshotApplied records that the xDS snapshot built from the last cluster listing was applied.
// Wire it as the snapshot manager's after-apply hook: it is how Backend revisions become installed,
// for the syncer's own snapshots and for those another change (an API deploy, startup) triggers.
// Records of newly installed revisions are then published in the background.
func (s *Syncer) SnapshotApplied() {
	if s.markInstalled() {
		go s.publish("")
	}
}

// markInstalled makes the last listing the installed set and reports whether it changed.
func (s *Syncer) markInstalled() bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	changed := len(s.listed) != len(s.installed)
	for name, backend := range s.listed {
		if s.installed[name] != backend {
			changed = true
		}
	}
	s.installed = s.listed
	return changed
}

// LoadAll loads every Backend from the database into memory. Called at startup, before API
// hydration and the first snapshot; the first applied snapshot then installs them and publishes
// their records.
func (s *Syncer) LoadAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	backends, err := s.db.GetAllBackends()
	if err != nil {
		return fmt.Errorf("loading backends: %w", err)
	}
	permitted := make([]*models.StoredBackend, 0, len(backends))
	for _, backend := range backends {
		if s.permitted(backend) {
			permitted = append(permitted, backend)
		}
	}
	s.store.ReplaceAll(permitted)
	s.logger.Info("Loaded global backends", slog.Int("count", len(permitted)))
	return nil
}

// permitted reports whether the gateway's network access control allows every endpoint of the
// Backend. A refused Backend stays in the database but gets no cluster and no record, so the
// gateway never dials it; the operator sees why in the log.
func (s *Syncer) permitted(backend *models.StoredBackend) bool {
	err := templateengine.RunPostRenderCheck(string(models.KindBackend), &backend.Configuration)
	if err == nil {
		return true
	}
	if upstreamguard.IsRefused(err) {
		s.logger.Error("Global backend refused by network access control; it is not served",
			slog.String("name", backend.GetHandle()),
			slog.String("field", upstreamguard.RefusedFieldPath(err)),
			slog.String("reason", upstreamguard.RefusalReason(err)))
	} else {
		s.logger.Error("Failed to check global backend against network access control; it is not served",
			slog.String("name", backend.GetHandle()), slog.Any("error", err))
	}
	return false
}

// HandleBackendEvent applies one replica-sync event. It is idempotent: the database is read
// again, so a duplicate or reordered event converges on the stored state.
func (s *Syncer) HandleBackendEvent(event eventhub.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch event.Action {
	case "CREATE", "UPDATE":
		s.applyUpsert(event.EntityID, event.EventID)
	case "DELETE":
		s.applyDelete(event.EntityID, event.EventID)
	default:
		s.logger.Warn("Unknown backend event action",
			slog.String("action", event.Action), slog.String("entity_id", event.EntityID))
	}
}

// applyUpsert stores the Backend and pushes its cluster; once that snapshot is applied the
// revision is installed and its record published. If the cluster update fails, the previous
// revision stays installed and keeps its record.
func (s *Syncer) applyUpsert(uuid, correlationID string) {
	backend, err := s.db.GetBackend(uuid)
	if err != nil {
		if storage.IsNotFoundError(err) {
			// Deleted after this event was published; the delete event converges the state.
			s.applyDelete(uuid, correlationID)
			return
		}
		s.logger.Error("Failed to load backend for sync", slog.String("uuid", uuid), slog.Any("error", err))
		return
	}
	if !s.permitted(backend) {
		// A Backend the access control refuses is withdrawn if an earlier version was served.
		s.applyDelete(uuid, correlationID)
		return
	}
	if err := s.store.Upsert(backend); err != nil {
		s.logger.Error("Failed to store backend in memory", slog.String("uuid", uuid), slog.Any("error", err))
		return
	}
	if !s.updateSnapshot(correlationID) {
		s.logger.Error("Failed to update xDS snapshot for backend change; its previous revision stays in service",
			slog.String("name", backend.GetHandle()))
		return
	}
	if s.publish(correlationID) {
		s.dropWithdrawn(correlationID)
	}
	s.logger.Info("Applied global backend", slog.String("name", backend.GetHandle()), slog.String("event_id", correlationID))
}

// applyDelete withdraws the Backend's record first and its cluster second. The memory entry is
// removed by UUID, so a delete for an old UUID never removes a Backend created again under the
// same name. The cluster stays in every snapshot until the record is withdrawn; a failed
// withdrawal is retried by the next sync, including a repeated delete.
func (s *Syncer) applyDelete(uuid, correlationID string) {
	backend, pending := s.removeAndRetain(uuid)
	if backend == nil && !pending && !s.publisher.Stale() {
		return // already gone on this replica
	}
	if !s.publish(correlationID) {
		return
	}
	s.dropWithdrawn(correlationID)
	if backend != nil {
		s.logger.Info("Removed global backend", slog.String("name", backend.GetHandle()), slog.String("event_id", correlationID))
	}
}

// removeAndRetain removes the Backend from the store and, in the same stateMu section, moves the
// revision Envoy holds into the retained set, so no cluster listing finds the Backend in neither.
// That is the installed revision, not the stored one: after a failed cluster update the store
// holds a pending revision that never reached Envoy, while the published record still describes
// the installed one. A Backend that was never installed has no cluster or record to keep. It
// reports whether any deleted Backend's record may still be published.
func (s *Syncer) removeAndRetain(uuid string) (*models.StoredBackend, bool) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	backend := s.store.DeleteByUUID(uuid)
	if backend != nil {
		name := backend.GetHandle()
		if installed, ok := s.installed[name]; ok {
			s.retained[name] = installed
		}
	}
	return backend, len(s.retained) > 0
}

// dropWithdrawn releases the clusters of deleted Backends once a publish has withdrawn their
// records, and pushes a snapshot without them.
func (s *Syncer) dropWithdrawn(correlationID string) {
	if !s.hasRetained() {
		return
	}
	s.stateMu.Lock()
	s.retained = map[string]*models.StoredBackend{}
	s.stateMu.Unlock()
	if !s.updateSnapshot(correlationID) {
		s.logger.Error("Failed to update xDS snapshot to remove withdrawn backend clusters; they go with the next snapshot")
	}
}

func (s *Syncer) hasRetained() bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return len(s.retained) > 0
}

// updateSnapshot pushes the snapshot. The snapshot manager's after-apply hook (SnapshotApplied)
// installs the revisions it holds, inside the same critical section as the translation that
// listed them; marking them here, after UpdateSnapshot returns, could install the listing of
// another translation that has not been applied yet.
func (s *Syncer) updateSnapshot(correlationID string) bool {
	if s.snapshot == nil {
		// No router to update (tests, or a controller without xDS): the listing is installed.
		s.ClusterView().ListBackends()
		s.markInstalled()
		return true
	}
	if err := s.snapshot.UpdateSnapshot(context.Background(), correlationID); err != nil {
		s.logger.Error("Failed to update xDS snapshot for backend change", slog.Any("error", err))
		return false
	}
	return true
}

// publish reconciles the policy engine's records and reports whether it succeeded.
func (s *Syncer) publish(correlationID string) bool {
	if err := s.publisher.Publish(correlationID); err != nil {
		s.logger.Error("Failed to publish backend records to the policy engine; the next sync retries",
			slog.Any("error", err))
		return false
	}
	return true
}

func sorted(byName map[string]*models.StoredBackend) []*models.StoredBackend {
	out := make([]*models.StoredBackend, 0, len(byName))
	for _, backend := range byName {
		out = append(out, backend)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GetHandle() < out[j].GetHandle() })
	return out
}
