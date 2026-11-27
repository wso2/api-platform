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
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/transform"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/upstreamguard"
)

// recorder captures the order in which the router's clusters and the policy engine's records
// change, which is what keeps a request from being routed to a cluster Envoy lacks.
type recorder struct {
	mu        sync.Mutex
	ops       []string
	resources map[string]*storage.LazyResource
	store     *storage.BackendStore

	// failPushes makes the next N record pushes fail after the record was stored, as the
	// lazy-resource manager does when its snapshot update fails.
	failPushes int
	// failSnapshots makes the next N cluster snapshot updates fail.
	failSnapshots int
	// clusters lists the Backends a snapshot holds, as the xDS translator would read them;
	// nil reads the store.
	clusters func() []*models.StoredBackend
	// unbuilt names Backends whose clusters the translation fails to build and skips.
	unbuilt map[string]bool
	// built and applied stand in for the translator's report of the clusters it built and for
	// the snapshot manager's after-apply hook, which run inside a successful UpdateSnapshot.
	built   func(clusterNames []string)
	applied func()

	// snapshot is, by cluster name, what the last applied snapshot holds (nil before the first),
	// and violations lists every moment a record named a cluster that snapshot lacked, or one
	// built from a revision with another base path: the moments a request could have paired one
	// revision's base path with another's endpoints, or reached no cluster.
	snapshot   map[string]*models.StoredBackend
	violations []string
}

// checkPaired records a violation if the record does not name a cluster of the last applied
// snapshot built from a revision with the record's base path. The caller holds r.mu.
func (r *recorder) checkPaired(when string, resource *storage.LazyResource) {
	if r.snapshot == nil {
		return
	}
	cluster, _ := resource.Resource["clusterName"].(string)
	basePath, _ := resource.Resource["basePath"].(string)
	backend, ok := r.snapshot[cluster]
	switch {
	case !ok:
		r.violations = append(r.violations, fmt.Sprintf("%s: record %s names %s, which the snapshot lacks", when, resource.ID, cluster))
	case backend.BasePath() != basePath:
		r.violations = append(r.violations, fmt.Sprintf("%s: record %s has base path %q, its cluster %s was built with %q",
			when, resource.ID, basePath, cluster, backend.BasePath()))
	}
}

// The syncer's cluster view hears which clusters the translation built.
var _ transform.BuiltClusterRecorder = ClusterView{}

// wire connects the recorder to a syncer the way the controller connects the xDS translator and
// the snapshot manager.
func (r *recorder) wire(syncer *Syncer) {
	r.clusters = syncer.ClusterView().ListBackends
	r.built = syncer.ClusterView().ClustersBuilt
	r.applied = func() { syncer.markInstalled() }
}

func newRecorder(store *storage.BackendStore) *recorder {
	return &recorder{resources: map[string]*storage.LazyResource{}, store: store}
}

func (r *recorder) GetResourcesByType(resourceType string) map[string]*storage.LazyResource {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]*storage.LazyResource{}
	for id, res := range r.resources {
		if res.ResourceType == resourceType {
			out[id] = res
		}
	}
	return out
}

func (r *recorder) StoreResource(resource *storage.LazyResource, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checkPaired("record push", resource)
	r.resources[resource.ID] = resource
	return r.push("record+" + resource.ID)
}

func (r *recorder) RemoveResourceByIDAndType(id, _, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.resources, id)
	return r.push("record-" + id)
}

func (r *recorder) RefreshSnapshot() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.push("refresh")
}

// push records a push to the policy engine, or fails it. Either way the store has changed.
func (r *recorder) push(op string) error {
	if r.failPushes > 0 {
		r.failPushes--
		r.ops = append(r.ops, "failed:"+op)
		return errors.New("snapshot update failed")
	}
	r.ops = append(r.ops, op)
	return nil
}

func (r *recorder) UpdateSnapshot(context.Context, string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failSnapshots > 0 {
		r.failSnapshots--
		r.ops = append(r.ops, "failed:snapshot")
		return errors.New("cds update failed")
	}
	list := r.store.ListBackends
	if r.clusters != nil {
		list = r.clusters
	}
	names := ""
	built := []string{}
	snapshot := map[string]*models.StoredBackend{}
	for _, e := range list() {
		if r.unbuilt[e.GetHandle()] {
			continue
		}
		names += e.GetHandle() + ","
		built = append(built, e.ClusterName())
		snapshot[e.ClusterName()] = e
	}
	if r.built != nil {
		r.built(built)
	}
	r.ops = append(r.ops, "snapshot["+names+"]")
	r.snapshot = snapshot
	for _, resource := range r.resources {
		r.checkPaired("snapshot", resource)
	}
	if r.applied != nil {
		r.applied()
	}
	return nil
}

func (r *recorder) take() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	ops := r.ops
	r.ops = nil
	return ops
}

type events struct {
	mu   sync.Mutex
	sent []eventhub.Event
}

func (e *events) PublishEvent(_ string, event eventhub.Event) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sent = append(e.sent, event)
	return nil
}

type fixture struct {
	svc      *Service
	db       storage.BackendStorage
	store    *storage.BackendStore
	rec      *recorder
	events   *events
	deployed []*models.StoredConfig
}

func dynamicEndpointDefinitions() map[string]models.PolicyDefinition {
	params := map[string]interface{}{"type": "object", "properties": map[string]interface{}{
		"targetUpstream":    map[string]interface{}{"type": "string", config.BackendRoutingAnnotation: config.BackendRoutingName},
		"backendExpression": map[string]interface{}{"type": "string", config.BackendRoutingAnnotation: config.BackendRoutingExpression},
	}}
	return map[string]models.PolicyDefinition{
		"dynamic-endpoint|v1.1.0": {Name: "dynamic-endpoint", Version: "v1.1.0", Parameters: &params},
	}
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	db, err := storage.NewStorage(storage.BackendConfig{Type: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "gw.db"), GatewayID: "gw"}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	es, ok := storage.AsBackendStorage(db)
	require.True(t, ok)

	f := &fixture{db: es, store: storage.NewBackendStore(), events: &events{}}
	f.rec = newRecorder(f.store)
	syncer := NewSyncer(es, f.store, f.rec, NewPublisher(f.store, f.rec), logger)
	f.rec.wire(syncer)
	t.Cleanup(func() {
		assert.Empty(t, f.rec.violations, "a published record was not paired with its own revision's cluster")
	})
	f.svc = NewService(ServiceConfig{
		DB:                es,
		PolicyDefinitions: dynamicEndpointDefinitions(),
		DeployedRestAPIs:  func() ([]*models.StoredConfig, error) { return f.deployed, nil },
		Syncer:            syncer,
		Events:            f.events,
		GatewayID:         "gw",
		Logger:            logger,
	})
	return f
}

const tenantA = `apiVersion: gateway.choreolabs.com/v1
kind: Backend
metadata:
  name: tenant-a
  labels:
    team: orders
spec:
  basePath: /v1
  endpoints:
    - url: http://orders-tenant-a:9080
`

func yamlReq(body string) Request {
	return Request{Body: []byte(body), ContentType: "application/yaml", CorrelationID: "corr-1"}
}

func TestService_CreatePublishesClusterThenRecordAndTellsReplicas(t *testing.T) {
	f := newFixture(t)

	stored, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)
	assert.Equal(t, "tenant-a", stored.GetHandle())
	assert.Equal(t, models.OriginGatewayAPI, stored.Origin)

	assert.Equal(t, []string{"snapshot[tenant-a,]", "record+tenant-a"}, f.rec.take(),
		"the cluster reaches the router before the policy engine can pick it")

	record := f.rec.resources["tenant-a"].Resource
	assert.Equal(t, stored.ClusterName(), record["clusterName"])
	assert.True(t, strings.HasPrefix(stored.ClusterName(), "backend_tenant-a_"), stored.ClusterName())
	assert.Equal(t, "/v1", record["basePath"])
	assert.Equal(t, map[string]interface{}{"team": "orders"}, record["labels"])

	require.Len(t, f.events.sent, 1)
	assert.Equal(t, eventhub.EventTypeBackend, f.events.sent[0].EventType)
	assert.Equal(t, "CREATE", f.events.sent[0].Action)
	assert.Equal(t, stored.UUID, f.events.sent[0].EntityID)
}

func TestService_DuplicateNameConflicts(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)
	_, err = f.svc.Create(yamlReq(tenantA))
	assert.True(t, storage.IsConflictError(err), "got %v", err)
}

func TestService_InvalidConfigurationIsAValidationError(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Create(yamlReq(`apiVersion: gateway.choreolabs.com/v1
kind: Backend
metadata:
  name: Tenant_A
spec:
  endpoints: []
`))
	var invalid *ValidationError
	require.True(t, errors.As(err, &invalid), "got %v", err)
	assert.True(t, errors.Is(err, ErrValidation))
	assert.Empty(t, f.rec.take(), "nothing is published for an invalid Backend")
}

func TestService_UpdateCannotRenameAndRepublishesTheRecord(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)
	f.rec.take()

	_, err = f.svc.Update("tenant-b", yamlReq(tenantA))
	assert.True(t, errors.Is(err, ErrNotFound))

	renamed := `apiVersion: gateway.choreolabs.com/v1
kind: Backend
metadata:
  name: tenant-z
spec:
  endpoints:
    - url: http://x:1
`
	_, err = f.svc.Update("tenant-a", yamlReq(renamed))
	var invalid *ValidationError
	require.True(t, errors.As(err, &invalid))

	moved := `apiVersion: gateway.choreolabs.com/v1
kind: Backend
metadata:
  name: tenant-a
spec:
  basePath: /v2
  endpoints:
    - url: http://orders-tenant-a-v2:9080
`
	updated, err := f.svc.Update("tenant-a", yamlReq(moved))
	require.NoError(t, err)
	assert.Equal(t, "/v2", updated.BasePath())
	assert.Equal(t, "/v2", f.rec.resources["tenant-a"].Resource["basePath"])
	assert.Equal(t, map[string]interface{}{}, f.rec.resources["tenant-a"].Resource["labels"])
}

func restAPIWithPolicies(handle string, apiPolicies []api.Policy, opPolicies []api.Policy) *models.StoredConfig {
	spec := api.APIConfigData{Operations: []api.Operation{{}}}
	if apiPolicies != nil {
		spec.Policies = &apiPolicies
	}
	if opPolicies != nil {
		spec.Operations[0].Policies = &opPolicies
	}
	return &models.StoredConfig{
		UUID: handle, Handle: handle, Kind: models.KindRestApi, DesiredState: models.StateDeployed,
		Configuration: api.RestAPI{Spec: spec},
	}
}

func TestService_DeleteIsRefusedWhileADeployedAPINamesTheBackendAsItsFixedTarget(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)

	fixed := map[string]interface{}{"targetUpstream": "tenant-a"}
	header := map[string]interface{}{"backendExpression": "request.Headers['x-backend'][0]"}
	f.deployed = []*models.StoredConfig{
		restAPIWithPolicies("orders-v1.0", nil, []api.Policy{{Name: "dynamic-endpoint", Version: "v1", Params: &fixed}}),
		restAPIWithPolicies("catalog-v1.0", []api.Policy{{Name: "dynamic-endpoint", Params: &header}}, nil),
	}

	_, err = f.svc.Delete("tenant-a", "corr-2")
	var inUse *InUseError
	require.True(t, errors.As(err, &inUse), "got %v", err)
	require.Len(t, inUse.References, 1, "expression-mode use cannot be guarded and does not block")
	assert.Equal(t, Reference{APIHandle: "orders-v1.0", FieldPath: "spec.operations[0].policies[0].params.targetUpstream"}, inUse.References[0])
	assert.Contains(t, inUse.Error(), "orders-v1.0")

	_, ok := f.store.Get("tenant-a")
	assert.True(t, ok, "a refused delete changes nothing")
}

func TestService_DeleteIsAllowedWhenTheAPIsOwnUpstreamDefinitionHasTheName(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)

	fixed := map[string]interface{}{"targetUpstream": "tenant-a"}
	shadowing := restAPIWithPolicies("orders-v1.0", nil, []api.Policy{{Name: "dynamic-endpoint", Version: "v1", Params: &fixed}})
	restCfg := shadowing.Configuration.(api.RestAPI)
	restCfg.Spec.UpstreamDefinitions = &[]api.UpstreamDefinition{{Name: "tenant-a"}}
	shadowing.Configuration = restCfg
	f.deployed = []*models.StoredConfig{shadowing}

	_, err = f.svc.Delete("tenant-a", "corr-4")
	require.NoError(t, err, "the API routes to its own upstream definition, never to the Backend")
}

func TestService_DeleteWithdrawsTheRecordBeforeTheCluster(t *testing.T) {
	f := newFixture(t)
	stored, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)
	f.rec.take()

	_, err = f.svc.Delete("tenant-a", "corr-3")
	require.NoError(t, err)
	assert.Equal(t, []string{"record-tenant-a", "snapshot[]"}, f.rec.take(),
		"the policy engine stops picking the Backend before its cluster goes")

	_, err = f.svc.Get("tenant-a")
	assert.True(t, errors.Is(err, ErrNotFound))
	last := f.events.sent[len(f.events.sent)-1]
	assert.Equal(t, "DELETE", last.Action)
	assert.Equal(t, stored.UUID, last.EntityID)
}

func TestSyncer_ReplicaEventsConvergeOnTheDatabase(t *testing.T) {
	f := newFixture(t)
	stored, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)

	// A second replica: its own memory and records, the shared database.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	replicaStore := storage.NewBackendStore()
	replicaRec := newRecorder(replicaStore)
	replica := NewSyncer(f.db, replicaStore, replicaRec, NewPublisher(replicaStore, replicaRec), logger)
	replicaRec.wire(replica)

	replica.HandleBackendEvent(eventhub.Event{Action: "CREATE", EntityID: stored.UUID, EventID: "e1"})
	replica.HandleBackendEvent(eventhub.Event{Action: "CREATE", EntityID: stored.UUID, EventID: "e1"}) // duplicate
	_, ok := replicaStore.Get("tenant-a")
	assert.True(t, ok)
	assert.Equal(t, []string{"snapshot[tenant-a,]", "record+tenant-a", "snapshot[tenant-a,]"}, replicaRec.take(),
		"a duplicate event is harmless: the unchanged record is not re-sent")

	// The Backend is deleted before this replica sees the (reordered) update: it converges.
	require.NoError(t, f.db.DeleteBackend(stored.UUID))
	replica.HandleBackendEvent(eventhub.Event{Action: "UPDATE", EntityID: stored.UUID, EventID: "e2"})
	_, ok = replicaStore.Get("tenant-a")
	assert.False(t, ok)
	assert.Empty(t, replicaRec.resources)
}

func TestSyncer_LoadAllAndPublishAtStartup(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	freshStore := storage.NewBackendStore()
	freshRec := newRecorder(freshStore)
	// A stale record from before a restart is removed by the first full reconcile.
	freshRec.resources["gone"] = &storage.LazyResource{ID: "gone", ResourceType: LazyResourceType}
	publisher := NewPublisher(freshStore, freshRec)
	syncer := NewSyncer(f.db, freshStore, freshRec, publisher, logger)
	require.NoError(t, syncer.LoadAll())

	// Nothing is published before the first snapshot holds the clusters.
	require.NoError(t, publisher.Publish(""))
	_, ok := freshRec.resources["tenant-a"]
	assert.False(t, ok)

	// The first applied snapshot installs the loaded Backends; the next publish reconciles.
	syncer.ClusterView().ListBackends()
	syncer.markInstalled()
	require.NoError(t, publisher.Publish(""))

	_, ok = freshRec.resources["tenant-a"]
	assert.True(t, ok)
	_, ok = freshRec.resources["gone"]
	assert.False(t, ok)
}

// An API the database no longer holds can linger in the in-memory store until the replica
// converges. The guard reads the database, so deleting the Backend right after the API is not
// refused; without a database it falls back to the store.
func TestDeployedRestAPIsFrom_ReadsTheDatabaseWhenThereIsOne(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	db, err := storage.NewStorage(storage.BackendConfig{Type: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "gw.db"), GatewayID: "gw"}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	stale := restAPIWithPolicies("deleted-api", nil, nil)
	stale.DisplayName, stale.Version = "deleted-api", "v1.0"
	store := storage.NewConfigStore()
	require.NoError(t, store.Add(stale))

	withDB, err := DeployedRestAPIsFrom(db, store)()
	require.NoError(t, err)
	assert.Empty(t, withDB, "a deleted API still in memory must not block the delete")

	storeOnly, err := DeployedRestAPIsFrom(nil, store)()
	require.NoError(t, err)
	require.Len(t, storeOnly, 1)
	assert.Equal(t, "deleted-api", storeOnly[0].Handle)
}

// A push to the policy engine that fails after the store changed leaves the publisher stale; the
// next Publish pushes the whole store again before reconciling.
func TestPublisher_RetriesAFailedPushOnTheNextPublish(t *testing.T) {
	f := newFixture(t)
	f.rec.failPushes = 1
	_, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err, "the Backend is stored; the push is retried")
	assert.True(t, f.svc.syncer.publisher.Stale())
	f.rec.take()

	require.NoError(t, f.svc.syncer.publisher.Publish("retry"))
	assert.Equal(t, []string{"refresh"}, f.rec.take(),
		"the record was already in the store, so only the full re-push was missing")
	assert.False(t, f.svc.syncer.publisher.Stale())
}

// If the cluster update fails, the record is not published, so no policy can select a Backend
// whose cluster Envoy lacks.
func TestSyncer_DoesNotPublishARecordWhenTheClusterUpdateFails(t *testing.T) {
	f := newFixture(t)
	f.rec.failSnapshots = 1
	_, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)
	assert.Equal(t, []string{"failed:snapshot"}, f.rec.take())
	_, published := f.rec.resources["tenant-a"]
	assert.False(t, published)
}

// If withdrawing the record fails, the cluster stays, and a repeated delete event finishes the
// withdrawal.
func TestSyncer_KeepsTheClusterUntilTheRecordIsWithdrawn(t *testing.T) {
	f := newFixture(t)
	stored, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)
	f.rec.take()

	f.rec.failPushes = 1
	_, err = f.svc.Delete("tenant-a", "corr-del")
	require.NoError(t, err)
	assert.Equal(t, []string{"failed:record-tenant-a"}, f.rec.take(), "no cluster removal yet")

	f.svc.syncer.HandleBackendEvent(eventhub.Event{Action: "DELETE", EntityID: stored.UUID, EventID: "replay"})
	assert.Equal(t, []string{"refresh", "snapshot[]"}, f.rec.take())
}

// A delete for an old UUID never removes a Backend created again under the same name.
func TestBackendStore_DeleteByUUIDLeavesAReplacementInPlace(t *testing.T) {
	store := storage.NewBackendStore()
	old := &models.StoredBackend{UUID: "u-old", Configuration: api.BackendRequest{Metadata: api.Metadata{Name: "tenant-a"}}}
	replacement := &models.StoredBackend{UUID: "u-new", Configuration: api.BackendRequest{Metadata: api.Metadata{Name: "tenant-a"}}}
	require.NoError(t, store.Upsert(old))
	require.NoError(t, store.Upsert(replacement))

	assert.Nil(t, store.DeleteByUUID("u-old"))
	got, ok := store.Get("tenant-a")
	require.True(t, ok)
	assert.Equal(t, "u-new", got.UUID)
	assert.Equal(t, replacement, store.DeleteByUUID("u-new"))
}

// The network access control that applies to API upstream URLs applies to a Backend's endpoints:
// a refused Backend is rejected on create, and one already stored is not served after a restart.
func TestService_NetworkAccessControlAppliesToEndpoints(t *testing.T) {
	f := newFixture(t)
	refuseOrdersHosts := func(kind string, rendered any) error {
		cfg, ok := rendered.(*api.BackendRequest)
		if !ok || kind != string(models.KindBackend) {
			return nil
		}
		for i, e := range cfg.Spec.Endpoints {
			if strings.Contains(e.Url, "orders-tenant-a") {
				return &upstreamguard.RefusedError{FieldPath: fmt.Sprintf("spec.endpoints[%d].url", i)}
			}
		}
		return nil
	}

	// Stored while access control was off.
	_, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)

	templateengine.SetPostRenderCheck(refuseOrdersHosts)
	t.Cleanup(func() { templateengine.SetPostRenderCheck(nil) })

	_, err = f.svc.Create(yamlReq(strings.Replace(tenantA, "name: tenant-a", "name: tenant-c", 1)))
	var invalid *ValidationError
	require.True(t, errors.As(err, &invalid), "got %v", err)
	require.Len(t, invalid.Errors, 1)
	assert.Equal(t, "spec.endpoints[0].url", invalid.Errors[0].Field)
	assert.NotContains(t, invalid.Errors[0].Message, "orders-tenant-a", "the refused URL is never echoed")

	restarted := storage.NewBackendStore()
	require.NoError(t, NewSyncer(f.db, restarted, nil, nil, nil).LoadAll())
	_, served := restarted.Get("tenant-a")
	assert.False(t, served, "a stored Backend the access control refuses gets no cluster or record")
}

const tenantB = `apiVersion: gateway.choreolabs.com/v1
kind: Backend
metadata:
  name: tenant-b
spec:
  basePath: /b
  endpoints:
    - url: http://orders-tenant-b:9080
`

// An update whose cluster update failed is not published by a later sync either: deleting
// another Backend reconciles every record, and the failed revision keeps its previous record,
// which matches the cluster Envoy still has.
func TestSyncer_AFailedRevisionKeepsItsPreviousRecordThroughLaterSyncs(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)
	_, err = f.svc.Create(yamlReq(tenantB))
	require.NoError(t, err)

	f.rec.failSnapshots = 1
	moved := strings.Replace(tenantA, "basePath: /v1", "basePath: /v2", 1)
	_, err = f.svc.Update("tenant-a", yamlReq(moved))
	require.NoError(t, err)
	assert.Equal(t, "/v1", f.rec.resources["tenant-a"].Resource["basePath"])

	_, err = f.svc.Delete("tenant-b", "corr-b")
	require.NoError(t, err)
	assert.Equal(t, "/v1", f.rec.resources["tenant-a"].Resource["basePath"],
		"the /v2 revision has no cluster yet, so its record is not published")
	_, published := f.rec.resources["tenant-b"]
	assert.False(t, published)

	// The delete's snapshot held the /v2 cluster; the next sync publishes its record.
	require.NoError(t, f.svc.syncer.publisher.Publish("next"))
	assert.Equal(t, "/v2", f.rec.resources["tenant-a"].Resource["basePath"])
}

// After a failed withdrawal, a later sync keeps the deleted Backend's cluster in its snapshot
// until the record is withdrawn, and only then drops it.
func TestSyncer_AFailedWithdrawalKeepsTheClusterThroughLaterSnapshots(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Create(yamlReq(tenantB))
	require.NoError(t, err)
	f.rec.take()

	f.rec.failPushes = 1
	_, err = f.svc.Delete("tenant-b", "corr-del")
	require.NoError(t, err)
	assert.Equal(t, []string{"failed:record-tenant-b"}, f.rec.take())

	_, err = f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)
	assert.Equal(t, []string{
		"snapshot[tenant-a,tenant-b,]", // tenant-b's cluster stays while its record may be published
		"refresh",                      // the failed withdrawal is pushed again
		"record+tenant-a",
		"snapshot[tenant-a,]", // only now does tenant-b's cluster go
	}, f.rec.take())
}

// A snapshot that another change triggered (an API deploy, startup) also installs a pending
// revision, and its record is published in the background.
func TestSyncer_SnapshotAppliedElsewhereInstallsAndPublishesPendingRevisions(t *testing.T) {
	f := newFixture(t)
	f.rec.failSnapshots = 1
	_, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)
	_, published := f.rec.resources["tenant-a"]
	require.False(t, published)

	// The other change's snapshot, applied with the real after-apply hook.
	f.rec.applied = f.svc.syncer.SnapshotApplied
	require.NoError(t, f.rec.UpdateSnapshot(context.Background(), "api-deploy"))
	require.Eventually(t, func() bool {
		f.rec.mu.Lock()
		defer f.rec.mu.Unlock()
		_, ok := f.rec.resources["tenant-a"]
		return ok
	}, 5*time.Second, 10*time.Millisecond)
}

// A delete after a failed update retains the revision Envoy holds, not the pending one, so a
// later snapshot cannot pair the pending revision's cluster with the published record.
func TestSyncer_ADeleteAfterAFailedUpdateRetainsTheInstalledRevision(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)

	f.rec.failSnapshots = 1
	moved := strings.NewReplacer("basePath: /v1", "basePath: /v2",
		"http://orders-tenant-a:9080", "http://orders-tenant-a-v2:9080").Replace(tenantA)
	_, err = f.svc.Update("tenant-a", yamlReq(moved))
	require.NoError(t, err)

	require.Equal(t, "/v1", f.rec.resources["tenant-a"].Resource["basePath"], "the /v2 revision was never installed")
	f.rec.take()

	f.rec.failPushes = 1
	_, err = f.svc.Delete("tenant-a", "corr-del")
	require.NoError(t, err)
	require.Equal(t, []string{"failed:record-tenant-a"}, f.rec.take(), "the withdrawal failed, so the /v1 record may still be in the policy engine")

	// An unrelated snapshot (an API deploy) lists the clusters while the record is still published.
	listed := f.svc.syncer.ClusterView().ListBackends()
	require.Len(t, listed, 1)
	assert.Equal(t, "/v1", listed[0].BasePath())
	assert.Equal(t, "http://orders-tenant-a:9080", listed[0].Configuration.Spec.Endpoints[0].Url,
		"the retained cluster is the one the published record describes")
}

// A normal delete retains the installed revision until its record is withdrawn.
func TestSyncer_ADeleteRetainsTheCurrentInstalledRevision(t *testing.T) {
	f := newFixture(t)
	stored, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)

	f.rec.failPushes = 1
	_, err = f.svc.Delete("tenant-a", "corr-del")
	require.NoError(t, err)
	listed := f.svc.syncer.ClusterView().ListBackends()
	require.Len(t, listed, 1)
	assert.Equal(t, stored.UUID, listed[0].UUID)
}

// lockCheckingStore records each store call that must be atomic with the cluster listing but ran
// without the syncer's stateMu held. The syncer is driven from one goroutine, so TryLock fails
// exactly when that goroutine holds the lock.
type lockCheckingStore struct {
	*storage.BackendStore
	syncer   *Syncer
	unlocked []string
}

func (l *lockCheckingStore) check(call string) {
	if l.syncer.stateMu.mu.TryLock() {
		l.syncer.stateMu.mu.Unlock()
		l.unlocked = append(l.unlocked, call)
	}
}

func (l *lockCheckingStore) DeleteByUUID(uuid string) *models.StoredBackend {
	l.check("DeleteByUUID")
	return l.BackendStore.DeleteByUUID(uuid)
}

func (l *lockCheckingStore) ListBackends() []*models.StoredBackend {
	l.check("ListBackends")
	return l.BackendStore.ListBackends()
}

// A delete removes the Backend from the store under stateMu, in the same section that retains it,
// and the cluster listing reads the store under that lock too, so no listing can run between the
// two steps and find the Backend in neither while its record may be published.
func TestSyncer_MovesADeletedBackendToRetainedUnderTheListingLock(t *testing.T) {
	f := newFixture(t)
	stored, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	store := &lockCheckingStore{BackendStore: storage.NewBackendStore()}
	rec := newRecorder(store.BackendStore)
	syncer := NewSyncer(f.db, store, rec, NewPublisher(store.BackendStore, rec), logger)
	store.syncer = syncer
	rec.wire(syncer)

	syncer.HandleBackendEvent(eventhub.Event{Action: "CREATE", EntityID: stored.UUID, EventID: "e1"})
	require.NoError(t, f.db.DeleteBackend(stored.UUID))
	rec.failPushes = 1 // the withdrawal fails, so the cluster must stay listed
	syncer.HandleBackendEvent(eventhub.Event{Action: "DELETE", EntityID: stored.UUID, EventID: "e2"})

	assert.Empty(t, store.unlocked, "these store calls ran outside the cluster listing's lock")
	listed := syncer.ClusterView().ListBackends()
	require.Len(t, listed, 1, "the deleted Backend is retained until its record is withdrawn")
	assert.Equal(t, "tenant-a", listed[0].GetHandle())
}

// A Backend deleted and created again under its name is a different resource. While the
// replacement's cluster is not installed, a later sync never re-publishes the deleted Backend's
// record, which would send requests to the deleted endpoints instead of failing closed.
func TestSyncer_NeverRepublishesADeletedBackendForAReplacementOfTheSameName(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)
	_, err = f.svc.Create(yamlReq(tenantB))
	require.NoError(t, err)

	f.rec.failSnapshots = 1 // the record is withdrawn, but the snapshot without the cluster fails
	_, err = f.svc.Delete("tenant-a", "corr-del-a")
	require.NoError(t, err)
	_, published := f.rec.resources["tenant-a"]
	require.False(t, published)

	f.rec.failSnapshots = 1 // the replacement's cluster is not installed either
	replacement := strings.Replace(tenantA, "basePath: /v1", "basePath: /v9", 1)
	_, err = f.svc.Create(yamlReq(replacement))
	require.NoError(t, err)

	_, err = f.svc.Delete("tenant-b", "corr-del-b") // reconciles every record
	require.NoError(t, err)
	_, published = f.rec.resources["tenant-a"]
	assert.False(t, published, "the deleted tenant-a's record came back for its replacement")
}

// A Backend whose cluster the translation cannot build is not in the applied snapshot, so it is
// not installed and gets no record; it is published once a snapshot holds its cluster.
func TestSyncer_ABackendWhoseClusterIsNotBuiltGetsNoRecord(t *testing.T) {
	f := newFixture(t)
	f.rec.unbuilt = map[string]bool{"tenant-a": true}
	_, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)
	_, err = f.svc.Create(yamlReq(tenantB))
	require.NoError(t, err)
	assert.Equal(t, []string{"snapshot[]", "snapshot[tenant-b,]", "record+tenant-b"}, f.rec.take())
	_, published := f.rec.resources["tenant-a"]
	assert.False(t, published)

	f.rec.unbuilt = nil
	moved := strings.Replace(tenantB, "basePath: /b", "basePath: /b2", 1)
	_, err = f.svc.Update("tenant-b", yamlReq(moved))
	require.NoError(t, err)
	_, published = f.rec.resources["tenant-a"]
	assert.True(t, published, "the next snapshot built its cluster")
}

// Whenever a delete releases the syncer's state lock, a cluster listing that took the lock next
// would still find the Backend, in the store or retained, while its record may be published: the
// removal and the retention are never split across two critical sections.
func TestSyncer_ADeletedBackendIsListableAtEveryReleaseOfTheStateLock(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)
	f.rec.failPushes = 1 // the withdrawal fails, so the Backend must stay listable throughout

	syncer := f.svc.syncer
	releases, missed := 0, 0
	syncer.stateMu.afterRelease = func() {
		releases++
		syncer.stateMu.mu.Lock() // as a listing would, without re-entering this hook
		defer syncer.stateMu.mu.Unlock()
		if !listableLocked(syncer, "tenant-a") {
			missed++
		}
	}
	_, err = f.svc.Delete("tenant-a", "corr-del")
	syncer.stateMu.afterRelease = nil
	require.NoError(t, err)

	require.Positive(t, releases)
	assert.Zero(t, missed, "a listing at a release of the state lock would have missed the Backend")
}

// A background publish, after a snapshot another change applied, that completes a failed
// withdrawal also releases the deleted Backend's cluster, as a Backend event does.
func TestSyncer_ABackgroundPublishReleasesWithdrawnClusters(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)
	f.rec.failPushes = 1
	_, err = f.svc.Delete("tenant-a", "corr-del")
	require.NoError(t, err)
	require.True(t, listable(f.svc.syncer, "tenant-a"), "the withdrawal failed, so the cluster stays")

	f.rec.failSnapshots = 1 // tenant-b is stored but not installed
	_, err = f.svc.Create(yamlReq(tenantB))
	require.NoError(t, err)
	f.rec.take()

	// Another change's snapshot, applied with the real after-apply hook, holds tenant-b's cluster
	// and tenant-a's, which its record may still name.
	f.rec.applied = f.svc.syncer.SnapshotApplied
	require.NoError(t, f.rec.UpdateSnapshot(context.Background(), "api-deploy"))

	require.Eventually(t, func() bool { return !listable(f.svc.syncer, "tenant-a") && !installed(f.svc.syncer, "tenant-a") },
		2*time.Second, 5*time.Millisecond, "the withdrawn Backend's cluster was not released")
	listed := f.svc.syncer.ClusterView().ListBackends()
	require.Len(t, listed, 1)
	assert.Equal(t, "tenant-b", listed[0].GetHandle())
	assert.Contains(t, f.rec.take(), "snapshot[tenant-b,]")
}

// listableLocked reports whether a cluster listing would include a revision of the named Backend:
// its stored revision, or one a record may still name. The caller holds the raw state mutex.
func listableLocked(s *Syncer, name string) bool {
	if _, ok := s.store.Get(name); ok {
		return true
	}
	for _, backend := range s.referenced {
		if backend.GetHandle() == name {
			return true
		}
	}
	return false
}

func listable(s *Syncer, name string) bool {
	s.stateMu.mu.Lock()
	defer s.stateMu.mu.Unlock()
	return listableLocked(s, name)
}

// installed reports whether the last applied snapshot holds a revision of the named Backend.
func installed(s *Syncer, name string) bool {
	s.stateMu.mu.Lock()
	defer s.stateMu.mu.Unlock()
	for _, backend := range s.installed {
		if backend.GetHandle() == name {
			return true
		}
	}
	return false
}

// An update gives the Backend a new revision with its own cluster. Both clusters are in service
// until the new record is published, so a request in between still pairs the old base path with
// the old endpoints; then the old cluster goes. The recorder checks the pairing at every step.
func TestSyncer_AnUpdateServesBothRevisionsUntilTheNewRecordIsPublished(t *testing.T) {
	f := newFixture(t)
	old, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)
	f.rec.take()

	moved := strings.NewReplacer("basePath: /v1", "basePath: /v2",
		"http://orders-tenant-a:9080", "http://orders-tenant-a-v2:9080").Replace(tenantA)
	updated, err := f.svc.Update("tenant-a", yamlReq(moved))
	require.NoError(t, err)
	require.NotEqual(t, old.ClusterName(), updated.ClusterName())

	assert.Equal(t, []string{
		"snapshot[tenant-a,tenant-a,]", // the new revision's cluster joins the old one
		"record+tenant-a",              // the record switches to the new cluster and base path
		"snapshot[tenant-a,]",          // the old cluster goes once no record names it
	}, f.rec.take())
	record := f.rec.resources["tenant-a"].Resource
	assert.Equal(t, updated.ClusterName(), record["clusterName"])
	assert.Equal(t, "/v2", record["basePath"])
	listed := f.svc.syncer.ClusterView().ListBackends()
	require.Len(t, listed, 1)
	assert.Equal(t, updated.ClusterName(), listed[0].ClusterName())
}

// If the new record's push fails, the policy engine may hold either record, so both clusters stay
// until a later publish succeeds.
func TestSyncer_AFailedRecordPushKeepsBothRevisionsInService(t *testing.T) {
	f := newFixture(t)
	old, err := f.svc.Create(yamlReq(tenantA))
	require.NoError(t, err)
	f.rec.take()

	f.rec.failPushes = 1
	moved := strings.Replace(tenantA, "basePath: /v1", "basePath: /v2", 1)
	updated, err := f.svc.Update("tenant-a", yamlReq(moved))
	require.NoError(t, err)
	assert.Equal(t, []string{"snapshot[tenant-a,tenant-a,]", "failed:record+tenant-a"}, f.rec.take())
	clusters := []string{}
	for _, backend := range f.svc.syncer.ClusterView().ListBackends() {
		clusters = append(clusters, backend.ClusterName())
	}
	assert.ElementsMatch(t, []string{old.ClusterName(), updated.ClusterName()}, clusters)

	// The next sync re-pushes the records and only then releases the old cluster.
	f.svc.syncer.HandleBackendEvent(eventhub.Event{Action: "UPDATE", EntityID: updated.UUID, EventID: "replay"})
	assert.Equal(t, []string{"snapshot[tenant-a,tenant-a,]", "refresh", "snapshot[tenant-a,]"}, f.rec.take())
}

// A templated Backend's record carries its endpoint template, which the policy engine expands per
// request; a plain Backend's record has no urlTemplate key at all.
func TestResource_PublishesATemplatedBackendsTemplate(t *testing.T) {
	stored := func(name, url string) *models.StoredBackend {
		return &models.StoredBackend{UUID: "u-" + name, Configuration: api.BackendRequest{
			Metadata: api.Metadata{Name: name},
			Spec: api.BackendConfiguration{Endpoints: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{{Url: url}}},
		}}
	}
	templated := stored("env-router", "https://url-{key}.url.fr")
	record := Resource(templated).Resource
	assert.Equal(t, "https://url-{key}.url.fr", record["urlTemplate"])
	assert.Equal(t, templated.ClusterName(), record["clusterName"])

	_, present := Resource(stored("tenant-a", "https://a.example.com")).Resource["urlTemplate"]
	assert.False(t, present)
}
