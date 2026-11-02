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

package graphqlapi

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// Test setup

// fakeStore is a storage.Storage that implements only the stored-config methods
// GraphQL API operations use. storage.Storage is embedded as a nil interface, so
// any other database call panics rather than silently returning a zero value.
type fakeStore struct {
	storage.Storage

	configs map[string]*models.StoredConfig
	calls   *[]string

	getErr    error
	upsertErr error
	deleteErr error
}

func (s *fakeStore) GetConfig(id string) (*models.StoredConfig, error) {
	*s.calls = append(*s.calls, "get_config")
	if s.getErr != nil {
		return nil, s.getErr
	}
	cfg, ok := s.configs[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return cfg, nil
}

func (s *fakeStore) GetConfigByKindAndHandle(kind, handle string) (*models.StoredConfig, error) {
	*s.calls = append(*s.calls, "get_config_by_handle")
	if s.getErr != nil {
		return nil, s.getErr
	}
	for _, cfg := range s.configs {
		if cfg.Kind == kind && cfg.Handle == handle {
			return cfg, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (s *fakeStore) GetConfigByKindNameAndVersion(kind, displayName, version string) (*models.StoredConfig, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	for _, cfg := range s.configs {
		if cfg.Kind == kind && cfg.DisplayName == displayName && cfg.Version == version {
			return cfg, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (s *fakeStore) GetAllConfigsByKind(kind string) ([]*models.StoredConfig, error) {
	*s.calls = append(*s.calls, "list_configs")
	if s.getErr != nil {
		return nil, s.getErr
	}
	out := make([]*models.StoredConfig, 0, len(s.configs))
	for _, cfg := range s.configs {
		if cfg.Kind == kind {
			out = append(out, cfg)
		}
	}
	return out, nil
}

func (s *fakeStore) UpsertConfig(cfg *models.StoredConfig) (bool, error) {
	*s.calls = append(*s.calls, "upsert_config")
	if s.upsertErr != nil {
		return false, s.upsertErr
	}
	if existing, ok := s.configs[cfg.UUID]; ok && existing.DeployedAt != nil && cfg.DeployedAt != nil &&
		!existing.DeployedAt.Before(*cfg.DeployedAt) {
		return false, nil
	}
	s.configs[cfg.UUID] = cfg
	return true, nil
}

func (s *fakeStore) DeleteConfig(id string) error {
	*s.calls = append(*s.calls, "delete_config")
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.configs, id)
	return nil
}

// recordingHub captures published events, recording each publish in the shared
// call log so a test can assert it came after the database write.
type recordingHub struct {
	calls      *[]string
	events     []eventhub.Event
	publishErr error
}

func (h *recordingHub) Initialize() error            { return nil }
func (h *recordingHub) RegisterGateway(string) error { return nil }
func (h *recordingHub) UnsubscribeAll(string) error  { return nil }
func (h *recordingHub) CleanUpEvents() error         { return nil }
func (h *recordingHub) Close() error                 { return nil }

func (h *recordingHub) PublishEvent(_ string, event eventhub.Event) error {
	*h.calls = append(*h.calls, "publish_event")
	if h.publishErr != nil {
		return h.publishErr
	}
	h.events = append(h.events, event)
	return nil
}

func (h *recordingHub) Subscribe(string) (<-chan eventhub.Event, error) { return nil, nil }

func (h *recordingHub) Unsubscribe(string, <-chan eventhub.Event) error { return nil }

// recordingPusher is a control-plane client whose submitted push tasks are
// recorded rather than run: the deploy-then-push task polls the config store on
// a ticker, so running it here would tie the test to that interval.
type recordingPusher struct {
	connected bool
	onPrem    bool
	submitted []func()
	pushed    []models.StoredConfig
}

func (p *recordingPusher) IsConnected() bool { return p.connected }
func (p *recordingPusher) IsOnPrem() bool    { return p.onPrem }
func (p *recordingPusher) PushArtifact(_ string, cfg *models.StoredConfig, _ string) error {
	p.pushed = append(p.pushed, *cfg)
	return nil
}
func (p *recordingPusher) SyncArtifactsToOnPremAPIM(*utils.APIMConfig) error { return nil }
func (p *recordingPusher) GetAPIMConfig() *utils.APIMConfig                  { return nil }
func (p *recordingPusher) SubmitAPIMSync(func())                             {}
func (p *recordingPusher) SubmitAPIMSyncAndWait(func())                      {}
func (p *recordingPusher) SubmitArtifactPush(task func())                    { p.submitted = append(p.submitted, task) }

type testEnv struct {
	svc    *GraphQLAPIService
	store  *fakeStore
	hub    *recordingHub
	pusher *recordingPusher
	cfg    *config.Config
	calls  *[]string
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestEnv builds a service over the real APIDeploymentService, so Create and
// Update run the same deployment the controller does.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	calls := &[]string{}
	store := &fakeStore{configs: map[string]*models.StoredConfig{}, calls: calls}
	hub := &recordingHub{calls: calls}
	pusher := &recordingPusher{connected: true}
	routerCfg := &config.RouterConfig{
		GatewayHost: "localhost",
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "localhost"},
			Sandbox: config.VHostEntry{Default: "sandbox-localhost"},
		},
	}
	systemCfg := &config.Config{}
	systemCfg.Controller.ControlPlane.DeploymentSyncEnabled = true

	deploymentService := utils.NewAPIDeploymentService(storage.NewConfigStore(), store, nil, config.NewAPIValidator(),
		routerCfg, hub, "test-gateway", nil, &http.Client{Timeout: time.Second})
	svc := NewGraphQLAPIService(storage.NewConfigStore(), store, deploymentService, config.NewParser(), hub,
		"test-gateway", pusher, systemCfg)

	return &testEnv{svc: svc, store: store, hub: hub, pusher: pusher, cfg: systemCfg, calls: calls}
}

func manifest(handle, displayName, version string) []byte {
	return []byte(fmt.Sprintf(`apiVersion: gateway.api-platform.wso2.com/v1
kind: GraphQLApi
metadata:
  name: %s
spec:
  displayName: %q
  version: %q
  context: "/%s"
  upstream:
    main:
      url: https://backend.example.com/graphql
`, handle, displayName, version, handle))
}

// seed stores a GraphQL API directly, with no deploy timestamp, so a later
// deploy of it is never treated as stale.
func (e *testEnv) seed(uuid, handle string, origin models.Origin) *models.StoredConfig {
	cfg := &models.StoredConfig{
		UUID:         uuid,
		Kind:         graphQLAPIKind,
		Handle:       handle,
		DisplayName:  "Seeded",
		Version:      "v1.0",
		DesiredState: models.StateDeployed,
		Origin:       origin,
	}
	e.store.configs[uuid] = cfg
	return cfg
}

func TestNewGraphQLAPIService_PanicsOnMissingDependencies(t *testing.T) {
	calls := &[]string{}
	store := &fakeStore{configs: map[string]*models.StoredConfig{}, calls: calls}
	hub := &recordingHub{calls: calls}
	deploymentService := utils.NewAPIDeploymentService(nil, store, nil, config.NewAPIValidator(),
		&config.RouterConfig{}, hub, "test-gateway", nil, nil)
	parser := config.NewParser()
	systemCfg := &config.Config{}

	assert.PanicsWithValue(t, "GraphQLAPIService requires non-nil storage", func() {
		NewGraphQLAPIService(nil, nil, deploymentService, parser, hub, "gw", nil, systemCfg)
	})
	assert.PanicsWithValue(t, "GraphQLAPIService requires APIDeploymentService", func() {
		NewGraphQLAPIService(nil, store, nil, parser, hub, "gw", nil, systemCfg)
	})
	assert.PanicsWithValue(t, "GraphQLAPIService requires a parser", func() {
		NewGraphQLAPIService(nil, store, deploymentService, nil, hub, "gw", nil, systemCfg)
	})
	assert.PanicsWithValue(t, "GraphQLAPIService requires non-nil EventHub", func() {
		NewGraphQLAPIService(nil, store, deploymentService, parser, nil, "gw", nil, systemCfg)
	})
	assert.PanicsWithValue(t, "GraphQLAPIService requires non-nil system config", func() {
		NewGraphQLAPIService(nil, store, deploymentService, parser, hub, "gw", nil, nil)
	})
}

// Create

func TestCreate_StoresAsGatewayOriginAndPublishes(t *testing.T) {
	env := newTestEnv(t)

	stored, err := env.svc.Create(CreateParams{
		Body: manifest("orders", "Orders", "v1.0"), ContentType: "application/yaml",
		CorrelationID: "corr-1", Logger: testLogger(),
	})
	require.NoError(t, err)

	assert.Equal(t, graphQLAPIKind, stored.Kind)
	assert.Equal(t, "orders", stored.Handle)
	assert.Equal(t, models.OriginGatewayAPI, stored.Origin)
	assert.NotEmpty(t, stored.UUID, "a UUID is generated")
	assert.Contains(t, env.store.configs, stored.UUID)

	require.Len(t, env.hub.events, 1)
	assert.Equal(t, eventhub.EventTypeAPI, env.hub.events[0].EventType)
	assert.Equal(t, "CREATE", env.hub.events[0].Action)
	assert.Equal(t, "corr-1", env.hub.events[0].EventID)

	// The control-plane push is submitted once the deployment is stored.
	assert.Len(t, env.pusher.submitted, 1)
}

func TestCreate_ReturnsTheDeploymentErrorUnchanged(t *testing.T) {
	env := newTestEnv(t)
	body := []byte(`apiVersion: gateway.api-platform.wso2.com/v1
kind: GraphQLApi
metadata:
  name: orders
spec:
  displayName: "Orders"
  version: "v1.0"
  context: "/orders"
  upstream:
    main: {}
`)

	_, err := env.svc.Create(CreateParams{Body: body, ContentType: "application/yaml", Logger: testLogger()})

	var validationErr *utils.ValidationErrorListError
	require.ErrorAs(t, err, &validationErr, "the handler maps this type, so it must not be wrapped")
	assert.Empty(t, env.store.configs)
	assert.Empty(t, env.pusher.submitted)
}

func TestCreate_ConflictRemainsDetectable(t *testing.T) {
	env := newTestEnv(t)
	_, err := env.svc.Create(CreateParams{Body: manifest("orders", "Orders", "v1.0"), ContentType: "application/yaml", Logger: testLogger()})
	require.NoError(t, err)

	_, err = env.svc.Create(CreateParams{Body: manifest("orders", "Other", "v2.0"), ContentType: "application/yaml", Logger: testLogger()})

	assert.True(t, storage.IsConflictError(err))
}

func TestCreate_NoPushWhenDeploymentSyncIsDisabled(t *testing.T) {
	env := newTestEnv(t)
	env.cfg.Controller.ControlPlane.DeploymentSyncEnabled = false

	_, err := env.svc.Create(CreateParams{Body: manifest("orders", "Orders", "v1.0"), ContentType: "application/yaml", Logger: testLogger()})
	require.NoError(t, err)

	assert.Empty(t, env.pusher.submitted)
}

func TestCreate_NoPushWhenTheControlPlaneIsDisconnected(t *testing.T) {
	env := newTestEnv(t)
	env.pusher.connected = false

	_, err := env.svc.Create(CreateParams{Body: manifest("orders", "Orders", "v1.0"), ContentType: "application/yaml", Logger: testLogger()})
	require.NoError(t, err)

	assert.Empty(t, env.pusher.submitted)
}

// List and GetByHandle

func TestList_ReturnsOnlyGraphQLAPIs(t *testing.T) {
	env := newTestEnv(t)
	env.seed("gql-1", "orders", models.OriginGatewayAPI)
	env.seed("gql-2", "users", models.OriginGatewayAPI)
	env.store.configs["rest-1"] = &models.StoredConfig{UUID: "rest-1", Kind: string(models.KindRestApi), Handle: "rest"}

	configs, err := env.svc.List()

	require.NoError(t, err)
	assert.Len(t, configs, 2)
}

func TestList_StorageFailureIsReturned(t *testing.T) {
	env := newTestEnv(t)
	env.store.getErr = errors.New("database is locked")

	_, err := env.svc.List()

	assert.EqualError(t, err, "database is locked")
}

func TestGetByHandle(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		env := newTestEnv(t)
		seeded := env.seed("gql-1", "orders", models.OriginGatewayAPI)

		cfg, err := env.svc.GetByHandle("orders")

		require.NoError(t, err)
		assert.Same(t, seeded, cfg)
	})

	t.Run("missing", func(t *testing.T) {
		env := newTestEnv(t)

		_, err := env.svc.GetByHandle("nope")

		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("a RestApi with the same handle is not a GraphQL API", func(t *testing.T) {
		env := newTestEnv(t)
		env.store.configs["rest-1"] = &models.StoredConfig{UUID: "rest-1", Kind: string(models.KindRestApi), Handle: "orders"}

		_, err := env.svc.GetByHandle("orders")

		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("an unexpected storage failure is a load error", func(t *testing.T) {
		env := newTestEnv(t)
		env.store.getErr = errors.New("database is locked")

		_, err := env.svc.GetByHandle("orders")

		var loadErr *LoadError
		require.ErrorAs(t, err, &loadErr)
		assert.EqualError(t, loadErr.Cause, "database is locked")
		assert.NotErrorIs(t, err, ErrNotFound)
	})
}

// Update

func TestUpdate_KeepsTheUUIDAndOrigin(t *testing.T) {
	env := newTestEnv(t)
	env.seed("gql-1", "orders", models.OriginControlPlane)

	stored, err := env.svc.Update(UpdateParams{
		Handle: "orders", Body: manifest("orders", "Orders", "v2.0"), ContentType: "application/yaml",
		CorrelationID: "corr-2", Logger: testLogger(),
	})
	require.NoError(t, err)

	assert.Equal(t, "gql-1", stored.UUID, "an update redeploys under the stored UUID")
	assert.Equal(t, models.OriginControlPlane, stored.Origin, "an update keeps the stored origin")
	assert.Equal(t, "v2.0", env.store.configs["gql-1"].Version)
	require.Len(t, env.hub.events, 1)
	assert.Equal(t, "UPDATE", env.hub.events[0].Action)
	assert.Len(t, env.pusher.submitted, 1)
}

func TestUpdate_LookupErrors(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		env := newTestEnv(t)

		_, err := env.svc.Update(UpdateParams{Handle: "nope", Body: []byte("kind: ["), ContentType: "application/yaml", Logger: testLogger()})

		assert.ErrorIs(t, err, ErrNotFound, "the lookup comes before the parse")
	})

	t.Run("storage failure", func(t *testing.T) {
		env := newTestEnv(t)
		env.store.getErr = errors.New("database is locked")

		_, err := env.svc.Update(UpdateParams{Handle: "orders", Body: manifest("orders", "Orders", "v2.0"), ContentType: "application/yaml", Logger: testLogger()})

		var loadErr *LoadError
		assert.ErrorAs(t, err, &loadErr)
	})
}

func TestUpdate_RefusesBeforeWriting(t *testing.T) {
	t.Run("an unparseable body", func(t *testing.T) {
		env := newTestEnv(t)
		env.seed("gql-1", "orders", models.OriginGatewayAPI)

		_, err := env.svc.Update(UpdateParams{Handle: "orders", Body: []byte("kind: ["), ContentType: "application/yaml", Logger: testLogger()})

		var parseErr *ParseError
		require.ErrorAs(t, err, &parseErr)
		assert.NotContains(t, *env.calls, "upsert_config")
	})

	t.Run("a body naming another handle", func(t *testing.T) {
		env := newTestEnv(t)
		env.seed("gql-1", "orders", models.OriginGatewayAPI)

		_, err := env.svc.Update(UpdateParams{Handle: "orders", Body: manifest("other", "Orders", "v2.0"), ContentType: "application/yaml", Logger: testLogger()})

		var mismatchErr *HandleMismatchError
		require.ErrorAs(t, err, &mismatchErr)
		assert.Equal(t, "orders", mismatchErr.PathHandle)
		assert.Equal(t, "other", mismatchErr.YAMLHandle)
		assert.NotContains(t, *env.calls, "upsert_config", "a mismatched body must not rename the stored API")
		assert.Equal(t, "orders", env.store.configs["gql-1"].Handle)
	})
}

func TestUpdate_ReturnsTheDeploymentErrorUnchanged(t *testing.T) {
	env := newTestEnv(t)
	env.seed("gql-1", "orders", models.OriginGatewayAPI)
	env.store.upsertErr = errors.New("disk full")

	_, err := env.svc.Update(UpdateParams{Handle: "orders", Body: manifest("orders", "Orders", "v2.0"), ContentType: "application/yaml", Logger: testLogger()})

	require.Error(t, err)
	var loadErr *LoadError
	assert.False(t, errors.As(err, &loadErr), "a deployment failure is not a lookup failure")
	assert.Contains(t, err.Error(), "disk full")
	assert.Empty(t, env.pusher.submitted)
}

func TestUpdate_StaleDeployIsNotPushed(t *testing.T) {
	env := newTestEnv(t)
	seeded := env.seed("gql-1", "orders", models.OriginGatewayAPI)
	future := time.Now().Add(time.Hour)
	seeded.DeployedAt = &future

	_, err := env.svc.Update(UpdateParams{Handle: "orders", Body: manifest("orders", "Orders", "v2.0"), ContentType: "application/yaml", Logger: testLogger()})

	require.NoError(t, err)
	assert.Empty(t, env.hub.events)
	assert.Empty(t, env.pusher.submitted)
}

// Delete

func TestDelete_RemovesPublishesAndPushesTheUndeploy(t *testing.T) {
	env := newTestEnv(t)
	env.seed("gql-1", "orders", models.OriginGatewayAPI)

	require.NoError(t, env.svc.Delete(DeleteParams{Handle: "orders", CorrelationID: "corr-3", Logger: testLogger()}))

	assert.NotContains(t, env.store.configs, "gql-1")
	// The replica-sync event is published only after the database delete.
	assert.Equal(t, []string{"get_config_by_handle", "delete_config", "publish_event"}, *env.calls)
	require.Len(t, env.hub.events, 1)
	assert.Equal(t, eventhub.EventTypeAPI, env.hub.events[0].EventType)
	assert.Equal(t, "DELETE", env.hub.events[0].Action)
	assert.Equal(t, "gql-1", env.hub.events[0].EntityID)
	assert.Equal(t, "corr-3", env.hub.events[0].EventID)
	assert.Equal(t, "test-gateway", env.hub.events[0].GatewayID)

	require.Len(t, env.pusher.submitted, 1)
	env.pusher.submitted[0]()
	require.Len(t, env.pusher.pushed, 1)
	assert.Equal(t, "gql-1", env.pusher.pushed[0].UUID)
	assert.Equal(t, models.StateUndeployed, env.pusher.pushed[0].DesiredState)
}

func TestDelete_NoUndeployPushForAControlPlaneOrigin(t *testing.T) {
	env := newTestEnv(t)
	env.seed("gql-1", "orders", models.OriginControlPlane)

	require.NoError(t, env.svc.Delete(DeleteParams{Handle: "orders", Logger: testLogger()}))

	assert.Empty(t, env.pusher.submitted)
}

func TestDelete_Errors(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		env := newTestEnv(t)

		err := env.svc.Delete(DeleteParams{Handle: "nope", Logger: testLogger()})

		assert.ErrorIs(t, err, ErrNotFound)
		assert.NotContains(t, *env.calls, "delete_config")
	})

	t.Run("lookup failure", func(t *testing.T) {
		env := newTestEnv(t)
		env.store.getErr = errors.New("database is locked")

		err := env.svc.Delete(DeleteParams{Handle: "orders", Logger: testLogger()})

		var loadErr *LoadError
		assert.ErrorAs(t, err, &loadErr)
	})

	t.Run("delete failure publishes nothing", func(t *testing.T) {
		env := newTestEnv(t)
		env.seed("gql-1", "orders", models.OriginGatewayAPI)
		env.store.deleteErr = errors.New("database is locked")

		err := env.svc.Delete(DeleteParams{Handle: "orders", Logger: testLogger()})

		var deleteErr *DeleteError
		require.ErrorAs(t, err, &deleteErr)
		assert.EqualError(t, deleteErr.Cause, "database is locked")
		assert.Empty(t, env.hub.events)
		assert.Empty(t, env.pusher.submitted)
	})
}

func TestDelete_EventPublishFailureIsNotReturned(t *testing.T) {
	env := newTestEnv(t)
	env.seed("gql-1", "orders", models.OriginGatewayAPI)
	env.hub.publishErr = errors.New("event hub down")

	err := env.svc.Delete(DeleteParams{Handle: "orders", Logger: testLogger()})

	assert.NoError(t, err, "a failed publish is logged; the delete has already happened")
	assert.NotContains(t, env.store.configs, "gql-1")
}

// An on-prem gateway does not tell the control plane about a delete.
func TestDelete_OnPremDoesNotPushUndeploy(t *testing.T) {
	env := newTestEnv(t)
	env.pusher.onPrem = true
	env.seed("gql-1", "orders", models.OriginGatewayAPI)

	require.NoError(t, env.svc.Delete(DeleteParams{Handle: "orders", Logger: testLogger()}))

	assert.Empty(t, env.pusher.submitted)
}

// The deploy-then-push waits until the deployment lands in the config store,
// then pushes it to the control plane once.
func TestWaitForDeploymentAndPush_PushesOnceDeployed(t *testing.T) {
	env := newTestEnv(t)
	deployedAt := time.Now()
	require.NoError(t, env.svc.store.Add(&models.StoredConfig{
		UUID: "gql-1", Kind: graphQLAPIKind, Handle: "orders",
		DisplayName: "Orders", Version: "v1.0", DeployedAt: &deployedAt,
	}))

	env.svc.waitForDeploymentAndPush("gql-1", "corr-1", &deployedAt, testLogger())

	require.Len(t, env.pusher.pushed, 1)
	assert.Equal(t, "gql-1", env.pusher.pushed[0].UUID)
}

// An on-prem gateway never pushes a deployment to the control plane.
func TestWaitForDeploymentAndPush_SkipsOnPrem(t *testing.T) {
	env := newTestEnv(t)
	env.pusher.onPrem = true

	env.svc.waitForDeploymentAndPush("gql-1", "", nil, testLogger())

	assert.Empty(t, env.pusher.pushed)
}

func TestErrorMessages(t *testing.T) {
	cause := errors.New("boom")
	assert.Equal(t, "graphql api not found", ErrNotFound.Error())
	assert.Equal(t, "failed to load graphql api: boom", (&LoadError{Cause: cause}).Error())
	assert.Equal(t, "failed to parse configuration: boom", (&ParseError{Cause: cause}).Error())
	assert.Equal(t, "failed to delete graphql api: boom", (&DeleteError{Cause: cause}).Error())
	assert.Equal(t, "handle mismatch: path has 'a' but YAML metadata.name has 'b'",
		(&HandleMismatchError{PathHandle: "a", YAMLHandle: "b"}).Error())
	assert.ErrorIs(t, &LoadError{Cause: cause}, cause)
	assert.ErrorIs(t, &ParseError{Cause: cause}, cause)
	assert.ErrorIs(t, &DeleteError{Cause: cause}, cause)
}
