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

package utils

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

const tenantAYAML = `apiVersion: gateway.api-platform.wso2.com/v1
kind: Endpoint
metadata:
  name: tenant-a
  labels:
    team: orders
spec:
  displayName: Orders backend for tenant A
  basePath: /v1
  upstreams:
    - url: http://orders-tenant-a:9080
      weight: 80
    - url: http://orders-tenant-a-2:9080
      weight: 20
  timeout:
    connect: 2s
`

// recordingEndpointEvents records the replica events the service publishes.
type recordingEndpointEvents struct {
	events []eventhub.Event
	err    error
}

func (r *recordingEndpointEvents) PublishEvent(_ string, event eventhub.Event) error {
	r.events = append(r.events, event)
	return r.err
}

// recordingEndpointSyncer applies the database to the in-memory store the way
// EndpointPublisher does, and counts the calls.
type recordingEndpointSyncer struct {
	db    *testMockDB
	store *storage.ConfigStore
	calls int
	err   error
}

func (r *recordingEndpointSyncer) Sync(context.Context, string) error {
	r.calls++
	if r.err != nil {
		return r.err
	}
	wanted := map[string]bool{}
	for _, endpoint := range r.db.endpoints {
		wanted[endpoint.GetHandle()] = true
	}
	for _, existing := range r.store.GetAllEndpoints() {
		if !wanted[existing.GetHandle()] {
			r.store.DeleteEndpoint(existing.GetHandle())
		}
	}
	for _, endpoint := range r.db.endpoints {
		if err := r.store.SetEndpoint(endpoint); err != nil {
			return err
		}
	}
	return nil
}

type endpointServiceFixture struct {
	service *EndpointService
	db      *testMockDB
	store   *storage.ConfigStore
	syncer  *recordingEndpointSyncer
	events  *recordingEndpointEvents
}

// endpointServiceRoutingDefs holds a routing policy shaped like dynamic-endpoint v1.1.
func endpointServiceRoutingDefs() map[string]models.PolicyDefinition {
	return map[string]models.PolicyDefinition{
		"dynamic-endpoint|v1.1.0": {
			Name: "dynamic-endpoint", Version: "v1.1.0",
			Parameters: &map[string]interface{}{"properties": map[string]interface{}{
				"targetUpstream": map[string]interface{}{"type": "string"},
				"targetEndpoint": map[string]interface{}{"type": "string", config.EndpointRoutingAnnotation: config.EndpointRoutingEndpointName},
				"endpointHeader": map[string]interface{}{"type": "string", config.EndpointRoutingAnnotation: config.EndpointRoutingHeaderName},
			}},
		},
	}
}

func newEndpointServiceFixture(t *testing.T) *endpointServiceFixture {
	t.Helper()
	db := newTestMockDB()
	store := storage.NewConfigStore()
	syncer := &recordingEndpointSyncer{db: db, store: store}
	events := &recordingEndpointEvents{}
	service := NewEndpointService(db, store, syncer, events, "gateway-1",
		config.NewEndpointValidator(nil), config.NewEndpointRouting(endpointServiceRoutingDefs()),
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	return &endpointServiceFixture{service: service, db: db, store: store, syncer: syncer, events: events}
}

func yamlParams(spec string) EndpointParams {
	return EndpointParams{Spec: []byte(spec), ContentType: "application/yaml", CorrelationID: "corr-1"}
}

// deployedAPITargeting stores a deployed RestApi whose policies are given at
// the API level and on its single operation.
func (f *endpointServiceFixture) deployedAPITargeting(t *testing.T, handle string, apiPolicies, opPolicies []api.Policy, state models.DesiredState) {
	t.Helper()
	restAPI := api.RestAPI{
		Kind:     api.RestAPIKindRestApi,
		Metadata: api.Metadata{Name: handle},
		Spec: api.APIConfigData{
			DisplayName: handle, Version: "v1.0", Context: "/" + handle,
			Operations: []api.Operation{{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/items")}},
		},
	}
	if apiPolicies != nil {
		restAPI.Spec.Policies = &apiPolicies
	}
	if opPolicies != nil {
		restAPI.Spec.Operations[0].Policies = &opPolicies
	}
	require.NoError(t, f.db.SaveConfig(&models.StoredConfig{
		UUID: "uuid-" + handle, Kind: models.KindRestApi, Handle: handle,
		Configuration: restAPI, SourceConfiguration: restAPI, DesiredState: state,
	}))
}

func targetEndpointPolicy(name string) api.Policy {
	return api.Policy{Name: "dynamic-endpoint", Version: "v1", Params: &map[string]interface{}{"targetEndpoint": name}}
}

func TestEndpointService_Create(t *testing.T) {
	f := newEndpointServiceFixture(t)

	stored, err := f.service.Create(yamlParams(tenantAYAML))
	require.NoError(t, err)

	assert.NotEmpty(t, stored.UUID)
	assert.Equal(t, "tenant-a", stored.GetHandle())
	assert.Equal(t, "/v1", stored.BasePath())
	assert.Equal(t, map[string]string{"team": "orders"}, stored.Labels())
	require.Len(t, stored.Configuration.Spec.Upstreams, 2)
	assert.Equal(t, 80, *stored.Configuration.Spec.Upstreams[0].Weight)

	// Durable, applied on this replica, and announced to the others.
	persisted, err := f.db.GetEndpointByHandle("tenant-a")
	require.NoError(t, err)
	assert.Equal(t, stored.UUID, persisted.UUID)
	_, inStore := f.store.GetEndpoint("tenant-a")
	assert.True(t, inStore)
	assert.Equal(t, 1, f.syncer.calls)
	require.Len(t, f.events.events, 1)
	event := f.events.events[0]
	assert.Equal(t, eventhub.EventTypeEndpoint, event.EventType)
	assert.Equal(t, "CREATE", event.Action)
	assert.Equal(t, stored.UUID, event.EntityID)
	assert.Equal(t, "gateway-1", event.GatewayID)
	assert.Equal(t, "corr-1", event.EventID)
}

func TestEndpointService_CreateAcceptsJSON(t *testing.T) {
	f := newEndpointServiceFixture(t)
	body := `{"apiVersion":"gateway.api-platform.wso2.com/v1","kind":"Endpoint",
		"metadata":{"name":"tenant-b"},"spec":{"upstreams":[{"url":"http://orders-tenant-b:9080"}]}}`

	stored, err := f.service.Create(EndpointParams{Spec: []byte(body), ContentType: "application/json"})
	require.NoError(t, err)
	assert.Equal(t, "tenant-b", stored.GetHandle())
}

// A client cannot set the server-managed status.
func TestEndpointService_CreateDropsClientStatus(t *testing.T) {
	f := newEndpointServiceFixture(t)
	stored, err := f.service.Create(yamlParams(tenantAYAML + "status:\n  id: spoofed\n"))
	require.NoError(t, err)
	assert.Nil(t, stored.Configuration.Status)
}

func TestEndpointService_CreateRejectsInvalidConfigurations(t *testing.T) {
	for name, spec := range map[string]string{
		"unparseable":     "::: not yaml",
		"bad name":        strings.Replace(tenantAYAML, "name: tenant-a", "name: Tenant_A", 1),
		"wrong kind":      strings.Replace(tenantAYAML, "kind: Endpoint", "kind: RestApi", 1),
		"url with a path": strings.Replace(tenantAYAML, "http://orders-tenant-a:9080", "http://orders-tenant-a:9080/v1", 1),
		"no upstreams":    "apiVersion: gateway.api-platform.wso2.com/v1\nkind: Endpoint\nmetadata:\n  name: tenant-a\nspec: {}\n",
	} {
		t.Run(name, func(t *testing.T) {
			f := newEndpointServiceFixture(t)
			_, err := f.service.Create(yamlParams(spec))
			require.ErrorIs(t, err, ErrEndpointValidation)
			assert.Empty(t, f.db.endpoints, "nothing is stored")
			assert.Zero(t, f.syncer.calls)
			assert.Empty(t, f.events.events)
		})
	}
}

func TestEndpointService_CreateConflictsOnAnExistingName(t *testing.T) {
	f := newEndpointServiceFixture(t)
	_, err := f.service.Create(yamlParams(tenantAYAML))
	require.NoError(t, err)

	_, err = f.service.Create(yamlParams(tenantAYAML))
	require.Error(t, err)
	assert.True(t, storage.IsConflictError(err))
	assert.Len(t, f.db.endpoints, 1)
	assert.Len(t, f.events.events, 1, "no event for the refused create")
}

func TestEndpointService_Update(t *testing.T) {
	f := newEndpointServiceFixture(t)
	created, err := f.service.Create(yamlParams(tenantAYAML))
	require.NoError(t, err)

	moved := strings.Replace(tenantAYAML, "basePath: /v1", "basePath: /v2", 1)
	updated, err := f.service.Update("tenant-a", yamlParams(moved))
	require.NoError(t, err)

	assert.Equal(t, created.UUID, updated.UUID, "an update keeps the Endpoint's identity")
	assert.Equal(t, "/v2", updated.BasePath())
	assert.Equal(t, created.CreatedAt, updated.CreatedAt)
	persisted, _ := f.db.GetEndpointByHandle("tenant-a")
	assert.Equal(t, "/v2", persisted.BasePath())
	assert.Equal(t, 2, f.syncer.calls)
	require.Len(t, f.events.events, 2)
	assert.Equal(t, "UPDATE", f.events.events[1].Action)
}

func TestEndpointService_UpdateErrors(t *testing.T) {
	f := newEndpointServiceFixture(t)
	_, err := f.service.Create(yamlParams(tenantAYAML))
	require.NoError(t, err)

	_, err = f.service.Update("missing", yamlParams(tenantAYAML))
	require.ErrorIs(t, err, ErrEndpointNotFound)

	// The name is the Endpoint's cluster name and what requests select it by.
	renamed := strings.Replace(tenantAYAML, "name: tenant-a", "name: tenant-b", 1)
	_, err = f.service.Update("tenant-a", yamlParams(renamed))
	require.ErrorIs(t, err, ErrEndpointValidation)
	assert.Contains(t, err.Error(), "cannot change endpoint name")

	_, err = f.service.Update("tenant-a", yamlParams(strings.Replace(tenantAYAML, "connect: 2s", "connect: soon", 1)))
	require.ErrorIs(t, err, ErrEndpointValidation)

	assert.Len(t, f.events.events, 1, "only the create was announced")
}

func TestEndpointService_Delete(t *testing.T) {
	f := newEndpointServiceFixture(t)
	created, err := f.service.Create(yamlParams(tenantAYAML))
	require.NoError(t, err)

	deleted, err := f.service.Delete("tenant-a", "corr-2", nil)
	require.NoError(t, err)
	assert.Equal(t, created.UUID, deleted.UUID)

	_, err = f.service.Get("tenant-a")
	require.ErrorIs(t, err, ErrEndpointNotFound)
	_, inStore := f.store.GetEndpoint("tenant-a")
	assert.False(t, inStore)
	require.Len(t, f.events.events, 2)
	assert.Equal(t, "DELETE", f.events.events[1].Action)
	assert.Equal(t, created.UUID, f.events.events[1].EntityID)

	_, err = f.service.Delete("tenant-a", "", nil)
	require.ErrorIs(t, err, ErrEndpointNotFound)
}

// An Endpoint a deployed API names as its fixed target cannot be deleted.
func TestEndpointService_DeleteIsRefusedWhileStaticallyTargeted(t *testing.T) {
	tests := []struct {
		name        string
		apiPolicies []api.Policy
		opPolicies  []api.Policy
	}{
		{"at the API level", []api.Policy{targetEndpointPolicy("tenant-a")}, nil},
		{"on an operation", nil, []api.Policy{targetEndpointPolicy("tenant-a")}},
		{"as the fallback of header mode", []api.Policy{{Name: "dynamic-endpoint", Version: "v1", Params: &map[string]interface{}{
			"endpointHeader": "x-endpoint", "targetEndpoint": "tenant-a",
		}}}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newEndpointServiceFixture(t)
			_, err := f.service.Create(yamlParams(tenantAYAML))
			require.NoError(t, err)
			f.deployedAPITargeting(t, "orders-v1.0", tc.apiPolicies, tc.opPolicies, models.StateDeployed)
			f.deployedAPITargeting(t, "billing-v1.0", tc.apiPolicies, tc.opPolicies, models.StateDeployed)

			_, err = f.service.Delete("tenant-a", "", nil)
			require.ErrorIs(t, err, ErrEndpointInUse)
			assert.Contains(t, err.Error(),
				"Endpoint 'tenant-a' is the fixed target of deployed APIs: billing-v1.0, orders-v1.0")

			_, err = f.service.Get("tenant-a")
			require.NoError(t, err, "the Endpoint must still exist")
			assert.Len(t, f.events.events, 1, "no DELETE event for the refused delete")
		})
	}
}

func TestEndpointService_DeleteIsAllowedWhenNothingStaticallyTargetsIt(t *testing.T) {
	tests := []struct {
		name     string
		policies []api.Policy
		state    models.DesiredState
	}{
		// Header-mode use happens at request time and cannot be guarded; after
		// the delete those requests are rejected, never misrouted.
		{"header mode only", []api.Policy{{Name: "dynamic-endpoint", Version: "v1", Params: &map[string]interface{}{"endpointHeader": "x-endpoint"}}}, models.StateDeployed},
		{"another Endpoint is targeted", []api.Policy{targetEndpointPolicy("tenant-b")}, models.StateDeployed},
		{"the same name as an API-local upstream", []api.Policy{{Name: "dynamic-endpoint", Version: "v1", Params: &map[string]interface{}{"targetUpstream": "tenant-a"}}}, models.StateDeployed},
		{"the API is undeployed", []api.Policy{targetEndpointPolicy("tenant-a")}, models.StateUndeployed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newEndpointServiceFixture(t)
			_, err := f.service.Create(yamlParams(tenantAYAML))
			require.NoError(t, err)
			f.deployedAPITargeting(t, "orders-v1.0", tc.policies, nil, tc.state)

			_, err = f.service.Delete("tenant-a", "", nil)
			require.NoError(t, err)
		})
	}
}

// The in-memory store converges from the database asynchronously, so a
// reference held only there still blocks the delete.
func TestEndpointService_DeleteGuardReadsTheInMemoryStoreToo(t *testing.T) {
	f := newEndpointServiceFixture(t)
	_, err := f.service.Create(yamlParams(tenantAYAML))
	require.NoError(t, err)

	policies := []api.Policy{targetEndpointPolicy("tenant-a")}
	restAPI := api.RestAPI{
		Kind: api.RestAPIKindRestApi, Metadata: api.Metadata{Name: "orders-v1.0"},
		Spec: api.APIConfigData{DisplayName: "orders", Version: "v1.0", Context: "/orders", Policies: &policies},
	}
	require.NoError(t, f.store.Add(&models.StoredConfig{
		UUID: "uuid-orders", Kind: models.KindRestApi, Handle: "orders-v1.0", DisplayName: "orders", Version: "v1.0",
		Configuration: restAPI, SourceConfiguration: restAPI, DesiredState: models.StateDeployed,
	}))

	_, err = f.service.Delete("tenant-a", "", nil)
	require.ErrorIs(t, err, ErrEndpointInUse)
}

// The change is durable once written, so a failure to apply or announce it is
// logged, not returned: every replica reconciles from the database again.
func TestEndpointService_ApplyFailuresDoNotFailTheRequest(t *testing.T) {
	f := newEndpointServiceFixture(t)
	f.syncer.err = errors.New("snapshot failed")
	f.events.err = errors.New("event hub down")

	stored, err := f.service.Create(yamlParams(tenantAYAML))
	require.NoError(t, err)
	_, err = f.db.GetEndpoint(stored.UUID)
	require.NoError(t, err)
	assert.Len(t, f.events.events, 1, "the event is still attempted after a failed sync")
}

func TestEndpointService_ListIsOrderedByName(t *testing.T) {
	f := newEndpointServiceFixture(t)
	for _, name := range []string{"tenant-c", "tenant-a", "tenant-b"} {
		_, err := f.service.Create(yamlParams(strings.Replace(tenantAYAML, "name: tenant-a", "name: "+name, 1)))
		require.NoError(t, err)
	}

	endpoints, err := f.service.List()
	require.NoError(t, err)
	var names []string
	for _, endpoint := range endpoints {
		names = append(names, endpoint.GetHandle())
	}
	assert.Equal(t, []string{"tenant-a", "tenant-b", "tenant-c"}, names)
}

func TestMissingStaticTargets(t *testing.T) {
	routing := config.NewEndpointRouting(endpointServiceRoutingDefs())
	store := storage.NewConfigStore()
	require.NoError(t, store.SetEndpoint(testEndpoint("tenant-a", "http://orders-a:9080")))

	apiLevel := []api.Policy{targetEndpointPolicy("tenant-a"), targetEndpointPolicy("tenant-z")}
	opLevel := []api.Policy{targetEndpointPolicy("tenant-b"), targetEndpointPolicy("tenant-z")}
	restAPI := api.RestAPI{Spec: api.APIConfigData{
		Policies:   &apiLevel,
		Operations: []api.Operation{{Policies: &opLevel}},
	}}

	assert.Equal(t, []string{"tenant-b", "tenant-z"}, MissingStaticTargets(routing, store, restAPI),
		"sorted, without duplicates, and without the Endpoint that exists")
	assert.Empty(t, MissingStaticTargets(nil, store, restAPI))
	assert.Empty(t, MissingStaticTargets(routing, nil, restAPI))
	assert.Empty(t, MissingStaticTargets(routing, store, api.RestAPI{}))
}

func TestIsControlPlanePushKind(t *testing.T) {
	assert.False(t, IsControlPlanePushKind(models.KindEndpoint))
	for _, kind := range []string{models.KindRestApi, models.KindLlmProvider, models.KindLlmProviderTemplate, models.KindMcp, models.KindAgent} {
		assert.True(t, IsControlPlanePushKind(kind), kind)
	}
}
