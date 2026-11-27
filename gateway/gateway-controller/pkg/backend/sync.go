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

// MemoryStore is the in-memory Backend store the syncer keeps in step with the database;
// *storage.BackendStore implements it.
type MemoryStore interface {
	Get(name string) (*models.StoredBackend, bool)
	ListBackends() []*models.StoredBackend
	Upsert(backend *models.StoredBackend) error
	DeleteByUUID(uuid string) *models.StoredBackend
	ReplaceAll(backends []*models.StoredBackend)
}

var _ MemoryStore = (*storage.BackendStore)(nil)

// stateLock is the syncer's state mutex. afterRelease, set only by tests, runs right after each
// release: at that point any other goroutine, such as a cluster listing, can see the state, so a
// test can check the state is consistent there.
type stateLock struct {
	mu           sync.Mutex
	afterRelease func()
}

func (l *stateLock) Lock() { l.mu.Lock() }

func (l *stateLock) Unlock() {
	l.mu.Unlock()
	if l.afterRelease != nil {
		l.afterRelease()
	}
}

// Syncer applies Backend changes on this replica: from the database into memory, then into the
// router's clusters and the policy engine's records.
//
// Every revision of a Backend (see models.StoredBackend.Revision) has its own cluster, and a
// record names the cluster of the revision it describes. Three rules keep each record paired with
// a cluster Envoy holds, built from the same revision:
//   - A record is published only for a revision whose cluster was in the last xDS snapshot that
//     was applied (the installed set). Until a new revision is installed, its Backend keeps the
//     record of the revision before it.
//   - A revision whose record the policy engine may hold (the referenced set) keeps its cluster in
//     every snapshot, alongside the Backend's current revision. During an update both clusters are
//     in service: the old record names the old cluster, and the new record, once published, the
//     new one.
//   - Once a publish has succeeded, only the revisions it published stay referenced, and the
//     clusters no record names any more (a deleted Backend's, an updated one's previous revision)
//     are released with a further snapshot.
type Syncer struct {
	// mu serialises each read from the database with its apply, and every publish, for local
	// changes, replica events and background reconciles alike, so an older read can never
	// overwrite a newer one in memory.
	mu sync.Mutex

	db        storage.BackendStorage
	store     MemoryStore
	snapshot  SnapshotUpdater
	publisher *Publisher
	logger    *slog.Logger

	// stateMu guards referenced, records, listed and installed, and makes each cluster listing and
	// each store change one step against the other. It is never held while the snapshot updater
	// or the publisher runs.
	stateMu stateLock
	// referenced holds, by cluster name, the revisions whose records the policy engine may hold.
	referenced map[string]*models.StoredBackend
	// records holds, by Backend name, the revision whose record the last publish pushed.
	records map[string]*models.StoredBackend
	// listed is, by cluster name, what the cluster view last returned to a translation.
	listed map[string]*models.StoredBackend
	// installed is, by cluster name, the set of revisions in the last applied snapshot.
	installed map[string]*models.StoredBackend
}

// NewSyncer creates a Syncer. snapshot and publisher may be nil in tests. The publisher is bound
// to the syncer's installed revisions, so it only ever publishes records whose clusters Envoy has.
func NewSyncer(db storage.BackendStorage, store MemoryStore, snapshot SnapshotUpdater, publisher *Publisher, logger *slog.Logger) *Syncer {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Syncer{
		db: db, store: store, snapshot: snapshot, publisher: publisher, logger: logger,
		referenced: map[string]*models.StoredBackend{},
		records:    map[string]*models.StoredBackend{},
		listed:     map[string]*models.StoredBackend{},
		installed:  map[string]*models.StoredBackend{},
	}
	if publisher != nil {
		publisher.backends = RecordView{s: s}
	}
	return s
}

// ClusterView lists the Backend revisions whose clusters belong in the xDS snapshot: the current
// revision of every stored Backend, plus every revision a record may still name. Give it to the
// xDS translator.
func (s *Syncer) ClusterView() ClusterView { return ClusterView{s: s} }

// ClusterView is the Backend revisions the router must have clusters for.
type ClusterView struct{ s *Syncer }

// ListBackends lists the revisions for a translation and remembers them, so SnapshotApplied knows
// which revisions the applied snapshot holds. The store and the referenced set are read in one
// stateMu section, which every store change also holds, so a listing never misses a revision whose
// record may be published.
func (v ClusterView) ListBackends() []*models.StoredBackend {
	s := v.s
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	byCluster := map[string]*models.StoredBackend{}
	for _, backend := range s.store.ListBackends() {
		byCluster[backend.ClusterName()] = backend
	}
	for cluster, backend := range s.referenced {
		if _, listed := byCluster[cluster]; !listed {
			byCluster[cluster] = backend
		}
	}
	s.listed = byCluster
	return sortedByCluster(byCluster)
}

// ClustersBuilt narrows the last listing to the revisions whose clusters the translation built.
// The translator skips a cluster that cannot be built, so that revision is not in the applied
// snapshot; leaving it out keeps it from being installed and getting a record.
func (v ClusterView) ClustersBuilt(clusterNames []string) {
	s := v.s
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	built := make(map[string]*models.StoredBackend, len(clusterNames))
	for _, cluster := range clusterNames {
		if backend, ok := s.listed[cluster]; ok {
			built[cluster] = backend
		}
	}
	s.listed = built
}

// RecordView lists the revisions whose records are published: for every stored Backend, its
// current revision once that revision's cluster is installed, and until then the revision the
// last publish pushed, while its cluster is installed. Only the publisher reads it, under the
// syncer's mu.
type RecordView struct{ s *Syncer }

// ListBackends lists the revisions to publish, one per Backend, and marks them referenced: from
// this push on, their clusters stay until a successful publish replaces them. A previous revision
// must be of the same Backend (the same UUID): one deleted and created again under its name is a
// different resource, so the deleted one's record is never published for it.
func (v RecordView) ListBackends() []*models.StoredBackend {
	s := v.s
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	out := map[string]*models.StoredBackend{}
	for _, stored := range s.store.ListBackends() {
		name := stored.GetHandle()
		if _, ok := s.installed[stored.ClusterName()]; ok {
			out[name] = stored
			continue
		}
		if previous, ok := s.records[name]; ok && previous.UUID == stored.UUID {
			if _, installed := s.installed[previous.ClusterName()]; installed {
				out[name] = previous
			}
		}
	}
	s.records = out
	for _, backend := range out {
		s.referenced[backend.ClusterName()] = backend
	}
	return sortedByCluster(out)
}

// SnapshotApplied records that the xDS snapshot built from the last cluster listing was applied.
// Wire it as the snapshot manager's after-apply hook: it is how Backend revisions become installed,
// for the syncer's own snapshots and for those another change (an API deploy, startup) triggers.
// Records of newly installed revisions are then published in the background.
func (s *Syncer) SnapshotApplied() {
	if s.markInstalled() {
		go s.reconcileInBackground()
	}
}

// reconcileInBackground publishes the records after a snapshot that changed the installed
// revisions, on the same serialised path as Backend events. Like a Backend event, a publish that
// succeeds also releases the clusters no record names any more; if it fails, they stay. It never
// runs inside the snapshot manager's apply hook, which holds the snapshot manager's lock that
// releasing a cluster needs.
func (s *Syncer) reconcileInBackground() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.publish("") {
		s.releaseUnreferenced("")
	}
}

// markInstalled makes the last listing the installed set and reports whether it changed.
func (s *Syncer) markInstalled() bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	changed := len(s.listed) != len(s.installed)
	for cluster, backend := range s.listed {
		if s.installed[cluster] != backend {
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

// applyUpsert stores the Backend and pushes a snapshot with its new revision's cluster, next to
// the previous revision's, which the published record still names. Once that snapshot is applied
// the new revision is installed and its record published; then the previous cluster is released.
// If the snapshot update fails, the previous revision stays installed and keeps its record.
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
	if err := s.storeUpsert(backend); err != nil {
		s.logger.Error("Failed to store backend in memory", slog.String("uuid", uuid), slog.Any("error", err))
		return
	}
	if !s.updateSnapshot(correlationID) {
		s.logger.Error("Failed to update xDS snapshot for backend change; its previous revision stays in service",
			slog.String("name", backend.GetHandle()))
		return
	}
	if s.publish(correlationID) {
		s.releaseUnreferenced(correlationID)
	}
	s.logger.Info("Applied global backend", slog.String("name", backend.GetHandle()),
		slog.String("revision", backend.Revision()), slog.String("event_id", correlationID))
}

// applyDelete withdraws the Backend's record first and its cluster second. The memory entry is
// removed by UUID, so a delete for an old UUID never removes a Backend created again under the
// same name. A revision whose record may be published stays referenced, so its cluster stays in
// every snapshot until a publish has withdrawn the record; a failed withdrawal is retried by the
// next sync, including a repeated delete.
func (s *Syncer) applyDelete(uuid, correlationID string) {
	backend := s.storeDelete(uuid)
	if backend == nil && s.inSync() && !s.publisher.Stale() {
		return // already gone on this replica
	}
	if !s.publish(correlationID) {
		return
	}
	s.releaseUnreferenced(correlationID)
	if backend != nil {
		s.logger.Info("Removed global backend", slog.String("name", backend.GetHandle()), slog.String("event_id", correlationID))
	}
}

// storeUpsert and storeDelete change the store under stateMu, so a cluster listing sees either the
// state before a change or the state after it.
func (s *Syncer) storeUpsert(backend *models.StoredBackend) error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.store.Upsert(backend)
}

func (s *Syncer) storeDelete(uuid string) *models.StoredBackend {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.store.DeleteByUUID(uuid)
}

// inSync reports whether every referenced and every installed cluster is the current revision of
// a stored Backend: nothing is left to withdraw or release.
func (s *Syncer) inSync() bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	current := s.currentClusters()
	for cluster := range s.referenced {
		if !current[cluster] {
			return false
		}
	}
	for cluster := range s.installed {
		if !current[cluster] {
			return false
		}
	}
	return true
}

// currentClusters returns the cluster names of the stored Backends' current revisions. The caller
// holds stateMu.
func (s *Syncer) currentClusters() map[string]bool {
	current := map[string]bool{}
	for _, backend := range s.store.ListBackends() {
		current[backend.ClusterName()] = true
	}
	return current
}

// releaseUnreferenced pushes a snapshot without the clusters no record names any more: a deleted
// Backend's, and an updated Backend's previous revision. Called after a publish succeeded.
func (s *Syncer) releaseUnreferenced(correlationID string) {
	if !s.hasUnreferencedClusters() {
		return
	}
	if !s.updateSnapshot(correlationID) {
		s.logger.Error("Failed to update xDS snapshot to release backend clusters no record names; they go with the next snapshot")
	}
}

// hasUnreferencedClusters reports whether the installed snapshot holds a cluster that is neither
// a stored Backend's current revision nor named by a record.
func (s *Syncer) hasUnreferencedClusters() bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	current := s.currentClusters()
	for cluster := range s.installed {
		if _, ok := s.referenced[cluster]; !ok && !current[cluster] {
			return true
		}
	}
	return false
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

// publish reconciles the policy engine's records and reports whether it succeeded. On success the
// policy engine holds exactly the records this publish pushed, so only their revisions stay
// referenced; on failure every revision pushed so far stays referenced, since the policy engine
// may hold any of their records.
func (s *Syncer) publish(correlationID string) bool {
	if err := s.publisher.Publish(correlationID); err != nil {
		s.logger.Error("Failed to publish backend records to the policy engine; the next sync retries",
			slog.Any("error", err))
		return false
	}
	s.stateMu.Lock()
	s.referenced = make(map[string]*models.StoredBackend, len(s.records))
	for _, backend := range s.records {
		s.referenced[backend.ClusterName()] = backend
	}
	s.stateMu.Unlock()
	return true
}

func sortedByCluster(backends map[string]*models.StoredBackend) []*models.StoredBackend {
	out := make([]*models.StoredBackend, 0, len(backends))
	for _, backend := range backends {
		out = append(out, backend)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ClusterName() < out[j].ClusterName() })
	return out
}
