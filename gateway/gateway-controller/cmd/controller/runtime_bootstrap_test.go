package main

import (
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

type fakeSecretResolver struct {
	errors map[string]error
}

func (f *fakeSecretResolver) Resolve(handle string) (string, error) {
	if f.errors != nil {
		if err, ok := f.errors[handle]; ok {
			return "", err
		}
	}
	return "resolved-" + handle, nil
}

// configWithTemplateSpec is a minimal struct that marshals to JSON with a "spec" field
// containing template expressions, used to trigger template rendering failures in tests.
type configWithTemplateSpec struct {
	Spec map[string]any `json:"spec"`
}

type fakeRuntimeTransformer struct {
	calls []string
}

func (f *fakeRuntimeTransformer) Transform(cfg *models.StoredConfig) (*models.RuntimeDeployConfig, error) {
	f.calls = append(f.calls, storage.Key(cfg.Kind, cfg.Handle))
	return &models.RuntimeDeployConfig{
		Metadata: models.Metadata{
			UUID:        cfg.UUID,
			Kind:        cfg.Kind,
			Handle:      cfg.Handle,
			DisplayName: cfg.DisplayName,
			Version:     cfg.Version,
		},
	}, nil
}

func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestLoadRuntimeConfigsFromExistingAPIConfigurations_LoadsFromDatabase(t *testing.T) {
	runtimeStore := storage.NewRuntimeConfigStore()
	transformer := &fakeRuntimeTransformer{}

	configs := []*models.StoredConfig{
		{UUID: "rest-1", Kind: models.KindRestApi, Handle: "rest-api", DisplayName: "Rest API", Version: "v1"},
		{UUID: "provider-1", Kind: models.KindLlmProvider, Handle: "provider-a", DisplayName: "Provider A", Version: "v1"},
		{UUID: "proxy-1", Kind: models.KindLlmProxy, Handle: "proxy-a", DisplayName: "Proxy A", Version: "v1"},
		{UUID: "ignored-1", Kind: "UnknownKind", Handle: "ignored", DisplayName: "Ignored", Version: "v1"},
	}

	loadedCount, err := loadRuntimeConfigsFromExistingAPIConfigurations(
		configs,
		runtimeStore,
		nil,
		transformer,
		newDiscardLogger(),
		false,
		nil,
	)

	assert.NoError(t, err)
	assert.Equal(t, 3, loadedCount)
	assert.ElementsMatch(t, []string{
		storage.Key(models.KindRestApi, "rest-api"),
		storage.Key(models.KindLlmProvider, "provider-a"),
		storage.Key(models.KindLlmProxy, "proxy-a"),
	}, transformer.calls)

	_, ok := runtimeStore.Get(storage.Key(models.KindRestApi, "rest-api"))
	assert.True(t, ok)
	_, ok = runtimeStore.Get(storage.Key(models.KindLlmProvider, "provider-a"))
	assert.True(t, ok)
	_, ok = runtimeStore.Get(storage.Key(models.KindLlmProxy, "proxy-a"))
	assert.True(t, ok)
	_, ok = runtimeStore.Get(storage.Key("UnknownKind", "ignored"))
	assert.False(t, ok)
}

// A model endpoint is compiled at startup like any API, so a restarted controller
// serves it without waiting for an event; a Model has no runtime config of its own.
func TestLoadRuntimeConfigsFromExistingAPIConfigurations_LoadsModelEndpoints(t *testing.T) {
	runtimeStore := storage.NewRuntimeConfigStore()
	transformer := &fakeRuntimeTransformer{}

	configs := []*models.StoredConfig{
		{UUID: "endpoint-1", Kind: models.KindModelEndpoint, Handle: "openai", DisplayName: "OpenAI", Version: "v1.0"},
		{UUID: "model-1", Kind: models.KindModel, Handle: "gpt", DisplayName: "GPT", Version: "v1.0"},
	}
	loadedCount, err := loadRuntimeConfigsFromExistingAPIConfigurations(configs, runtimeStore, nil, transformer,
		newDiscardLogger(), false)

	assert.NoError(t, err)
	assert.Equal(t, 1, loadedCount)
	assert.Equal(t, []string{storage.Key(models.KindModelEndpoint, "openai")}, transformer.calls)
	_, ok := runtimeStore.Get(storage.Key(models.KindModelEndpoint, "openai"))
	assert.True(t, ok)
}

func TestLoadRuntimeConfigsFromExistingAPIConfigurations_SkipsWebSubAPI(t *testing.T) {
	runtimeStore := storage.NewRuntimeConfigStore()
	transformer := &fakeRuntimeTransformer{}

	configs := []*models.StoredConfig{
		{UUID: "websub-1", Kind: models.KindWebSubApi, Handle: "websub-api", DisplayName: "WebSub API", Version: "v1"},
		{UUID: "rest-1", Kind: models.KindRestApi, Handle: "rest-api", DisplayName: "Rest API", Version: "v1"},
	}

	loadedCount, err := loadRuntimeConfigsFromExistingAPIConfigurations(
		configs,
		runtimeStore,
		nil,
		transformer,
		newDiscardLogger(),
		false,
		nil,
	)

	assert.NoError(t, err)
	assert.Equal(t, 1, loadedCount)
	assert.Equal(t, []string{storage.Key(models.KindRestApi, "rest-api")}, transformer.calls)

	_, ok := runtimeStore.Get(storage.Key(models.KindWebSubApi, "websub-api"))
	assert.False(t, ok)
	_, ok = runtimeStore.Get(storage.Key(models.KindRestApi, "rest-api"))
	assert.True(t, ok)
}

func TestLoadRuntimeConfigsFromExistingAPIConfigurations_ContinuesAfterRenderFailureWhenSkippingInvalid(t *testing.T) {
	runtimeStore := storage.NewRuntimeConfigStore()
	secretResolver := &fakeSecretResolver{
		errors: map[string]error{
			"bad-key": fmt.Errorf("secret not found"),
		},
	}
	transformer := &fakeRuntimeTransformer{}

	configs := []*models.StoredConfig{
		{
			UUID: "rest-1", Kind: models.KindRestApi, Handle: "bad-rest", DisplayName: "Bad Rest API", Version: "v1",
			Configuration: configWithTemplateSpec{Spec: map[string]any{"displayName": `{{ secret "bad-key" }}`}},
		},
		{UUID: "provider-1", Kind: models.KindLlmProvider, Handle: "provider-a", DisplayName: "Provider A", Version: "v1"},
	}

	loadedCount, err := loadRuntimeConfigsFromExistingAPIConfigurations(
		configs,
		runtimeStore,
		secretResolver,
		transformer,
		newDiscardLogger(),
		true,
		nil,
	)

	assert.NoError(t, err)
	assert.Equal(t, 1, loadedCount)
	assert.Equal(t, []string{storage.Key(models.KindLlmProvider, "provider-a")}, transformer.calls)

	_, ok := runtimeStore.Get(storage.Key(models.KindRestApi, "bad-rest"))
	assert.False(t, ok)
	_, ok = runtimeStore.Get(storage.Key(models.KindLlmProvider, "provider-a"))
	assert.True(t, ok)
}

func TestLoadRuntimeConfigsFromExistingAPIConfigurations_FailsFastOnRenderFailureByDefault(t *testing.T) {
	runtimeStore := storage.NewRuntimeConfigStore()
	secretResolver := &fakeSecretResolver{
		errors: map[string]error{
			"bad-key": fmt.Errorf("secret not found"),
		},
	}
	transformer := &fakeRuntimeTransformer{}

	configs := []*models.StoredConfig{
		{
			UUID: "rest-1", Kind: models.KindRestApi, Handle: "bad-rest", DisplayName: "Bad Rest API", Version: "v1",
			Configuration: configWithTemplateSpec{Spec: map[string]any{"displayName": `{{ secret "bad-key" }}`}},
		},
		{UUID: "provider-1", Kind: models.KindLlmProvider, Handle: "provider-a", DisplayName: "Provider A", Version: "v1"},
	}

	loadedCount, err := loadRuntimeConfigsFromExistingAPIConfigurations(
		configs,
		runtimeStore,
		secretResolver,
		transformer,
		newDiscardLogger(),
		false,
		nil,
	)

	assert.Equal(t, 0, loadedCount)
	assert.ErrorContains(t, err, "failed to render config for startup rest-1")
	assert.Empty(t, transformer.calls)

	_, ok := runtimeStore.Get(storage.Key(models.KindRestApi, "bad-rest"))
	assert.False(t, ok)
	_, ok = runtimeStore.Get(storage.Key(models.KindLlmProvider, "provider-a"))
	assert.False(t, ok)
}

func TestHydrateStoredConfigsFromDatabaseOnStartup_FailsFastByDefault(t *testing.T) {
	configStore := storage.NewConfigStore()
	err := configStore.Add(&models.StoredConfig{
		UUID:                "mcp-1",
		Kind:                models.KindMcp,
		Handle:              "bad-mcp",
		DisplayName:         "Bad MCP",
		Version:             "v1",
		SourceConfiguration: struct{}{},
	})
	assert.NoError(t, err)

	err = hydrateStoredConfigsFromDatabaseOnStartup(
		configStore,
		nil,
		nil,
		nil,
		nil,
		newDiscardLogger(),
		false,
	)

	assert.ErrorContains(t, err, "failed to hydrate stored MCP proxy configuration")
}

func TestHydrateStoredConfigsFromDatabaseOnStartup_SkipsInvalidConfigsWhenEnabled(t *testing.T) {
	configStore := storage.NewConfigStore()
	err := configStore.Add(&models.StoredConfig{
		UUID:                "mcp-1",
		Kind:                models.KindMcp,
		Handle:              "bad-mcp",
		DisplayName:         "Bad MCP",
		Version:             "v1",
		SourceConfiguration: struct{}{},
	})
	assert.NoError(t, err)

	err = hydrateStoredConfigsFromDatabaseOnStartup(
		configStore,
		nil,
		nil,
		nil,
		nil,
		newDiscardLogger(),
		true,
	)

	assert.NoError(t, err)
}

// restrictTemplateEnv installs an env access allow list for the test and
// restores the previous policy afterwards.
func restrictTemplateEnv(t *testing.T, names ...string) {
	t.Helper()
	policy, err := funcs.NewEnvAccessPolicy(names)
	require.NoError(t, err)
	prev := funcs.SetEnvAccessPolicy(policy)
	t.Cleanup(func() { funcs.SetEnvAccessPolicy(prev) })
}

func refusedEnvRestAPI(uuid, handle string, state models.DesiredState) *models.StoredConfig {
	return &models.StoredConfig{
		UUID: uuid, Kind: models.KindRestApi, Handle: handle, DisplayName: handle, Version: "v1",
		DesiredState:  state,
		Configuration: configWithTemplateSpec{Spec: map[string]any{"header": `{{ env "STARTUP_REFUSED_SECRET" }}`}},
	}
}

// An artifact that reads a refused variable is undeployed and its undeploy is
// handed to the callback, whether or not invalid deployments are skipped; the
// other artifacts still load.
func TestLoadRuntimeConfigsFromExistingAPIConfigurations_UndeploysEnvRefusedArtifact(t *testing.T) {
	restrictTemplateEnv(t, "BACKEND_URL")
	t.Setenv("STARTUP_REFUSED_SECRET", "s3cr3t")

	for _, skipInvalid := range []bool{false, true} {
		t.Run(fmt.Sprintf("skipInvalid=%v", skipInvalid), func(t *testing.T) {
			runtimeStore := storage.NewRuntimeConfigStore()
			transformer := &fakeRuntimeTransformer{}
			refused := refusedEnvRestAPI("rest-refused", "refused-api", models.StateDeployed)
			valid := &models.StoredConfig{UUID: "rest-ok", Kind: models.KindRestApi, Handle: "ok-api", DisplayName: "OK", Version: "v1", DesiredState: models.StateDeployed}

			var undeployed []string
			loadedCount, err := loadRuntimeConfigsFromExistingAPIConfigurations(
				[]*models.StoredConfig{refused, valid},
				runtimeStore,
				&fakeSecretResolver{},
				transformer,
				newDiscardLogger(),
				skipInvalid,
				func(cfg *models.StoredConfig) error {
					undeployed = append(undeployed, cfg.UUID)
					return nil
				},
			)

			require.NoError(t, err)
			assert.Equal(t, 1, loadedCount)
			assert.Equal(t, []string{"rest-refused"}, undeployed)
			assert.Equal(t, models.StateUndeployed, refused.DesiredState)
			assert.Equal(t, models.StateDeployed, valid.DesiredState)
			_, ok := runtimeStore.Get(storage.Key(models.KindRestApi, "refused-api"))
			assert.False(t, ok, "a refused artifact must not get a policy chain")
			_, ok = runtimeStore.Get(storage.Key(models.KindRestApi, "ok-api"))
			assert.True(t, ok)
		})
	}
}

func TestLoadRuntimeConfigsFromExistingAPIConfigurations_AlreadyUndeployedEnvRefusedArtifactIsLeftAlone(t *testing.T) {
	restrictTemplateEnv(t, "BACKEND_URL")
	refused := refusedEnvRestAPI("rest-refused", "refused-api", models.StateUndeployed)

	called := false
	loadedCount, err := loadRuntimeConfigsFromExistingAPIConfigurations(
		[]*models.StoredConfig{refused},
		storage.NewRuntimeConfigStore(),
		&fakeSecretResolver{},
		&fakeRuntimeTransformer{},
		newDiscardLogger(),
		false,
		func(*models.StoredConfig) error { called = true; return nil },
	)

	require.NoError(t, err)
	assert.Zero(t, loadedCount)
	assert.False(t, called)
	assert.Equal(t, models.StateUndeployed, refused.DesiredState)
}

// A failure to persist the undeploy is logged; the artifact stays undeployed for
// this run and loading continues.
func TestLoadRuntimeConfigsFromExistingAPIConfigurations_ContinuesWhenUndeployCannotBePersisted(t *testing.T) {
	restrictTemplateEnv(t, "BACKEND_URL")
	refused := refusedEnvRestAPI("rest-refused", "refused-api", models.StateDeployed)
	valid := &models.StoredConfig{UUID: "rest-ok", Kind: models.KindRestApi, Handle: "ok-api", DisplayName: "OK", Version: "v1"}

	loadedCount, err := loadRuntimeConfigsFromExistingAPIConfigurations(
		[]*models.StoredConfig{refused, valid},
		storage.NewRuntimeConfigStore(),
		&fakeSecretResolver{},
		&fakeRuntimeTransformer{},
		newDiscardLogger(),
		false,
		func(*models.StoredConfig) error { return fmt.Errorf("database unavailable") },
	)

	require.NoError(t, err)
	assert.Equal(t, 1, loadedCount)
	assert.Equal(t, models.StateUndeployed, refused.DesiredState)
}

// End to end against a real SQLite store: an API deployed before the allow list
// was set, whose upstream reads a now-refused variable, is persisted as
// undeployed (pending sync for a gateway-originated API) when the controller
// next starts, while a valid API still loads. A later startup leaves it alone.
func TestStartup_PersistsUndeployOfEnvRefusedAPI(t *testing.T) {
	metrics.Init()
	db, err := storage.NewStorage(storage.BackendConfig{
		Type:       "sqlite",
		SQLitePath: filepath.Join(t.TempDir(), "gateway.db"),
		GatewayID:  "platform-gateway-id",
	}, newDiscardLogger())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	refused := storedRestAPI("refused-api", "", nil)
	refusedAPI := refused.Configuration.(api.RestAPI)
	leakURL := `http://backend/{{ env "STARTUP_REFUSED_SECRET" }}`
	refusedAPI.Spec.Upstream.Main.Url = &leakURL
	refused.Configuration, refused.SourceConfiguration = refusedAPI, refusedAPI
	refused.DisplayName, refused.Version = "refused-api", "v1.0.0"
	refused.DesiredState, refused.Origin = models.StateDeployed, models.OriginGatewayAPI
	require.NoError(t, db.SaveConfig(refused))

	valid := storedRestAPI("valid-api", "", nil)
	validAPI := valid.Configuration.(api.RestAPI)
	okURL := "http://backend"
	validAPI.Spec.Upstream.Main.Url = &okURL
	valid.Configuration, valid.SourceConfiguration = validAPI, validAPI
	valid.DisplayName, valid.Version = "valid-api", "v1.0.0"
	valid.DesiredState, valid.Origin = models.StateDeployed, models.OriginGatewayAPI
	require.NoError(t, db.SaveConfig(valid))

	restrictTemplateEnv(t, "BACKEND_URL")
	t.Setenv("STARTUP_REFUSED_SECRET", "s3cr3t")

	startup := func() (int, *storage.RuntimeConfigStore) {
		configs, err := db.GetAllConfigs()
		require.NoError(t, err)
		runtimeStore := storage.NewRuntimeConfigStore()
		loaded, err := loadRuntimeConfigsFromExistingAPIConfigurations(
			configs, runtimeStore, &fakeSecretResolver{}, &fakeRuntimeTransformer{}, newDiscardLogger(),
			false, utils.NewEnvRefusedUndeployer(db, nil, "platform-gateway-id", newDiscardLogger()),
		)
		require.NoError(t, err)
		return loaded, runtimeStore
	}

	loaded, runtimeStore := startup()
	assert.Equal(t, 1, loaded)
	_, ok := runtimeStore.Get(storage.Key(models.KindRestApi, "valid-api"))
	assert.True(t, ok)
	_, ok = runtimeStore.Get(storage.Key(models.KindRestApi, "refused-api"))
	assert.False(t, ok)

	persisted, err := db.GetConfig(refused.UUID)
	require.NoError(t, err)
	assert.Equal(t, models.StateUndeployed, persisted.DesiredState)
	assert.Equal(t, models.CPSyncStatusPending, persisted.CPSyncStatus)
	source := persisted.SourceConfiguration.(api.RestAPI)
	assert.Equal(t, leakURL, *source.Spec.Upstream.Main.Url, "the stored definition stays unrendered")

	stillValid, err := db.GetConfig(valid.UUID)
	require.NoError(t, err)
	assert.Equal(t, models.StateDeployed, stillValid.DesiredState)

	// The next startup finds it already undeployed and leaves it as it is.
	loaded, _ = startup()
	assert.Equal(t, 1, loaded)
	persisted, err = db.GetConfig(refused.UUID)
	require.NoError(t, err)
	assert.Equal(t, models.StateUndeployed, persisted.DesiredState)
}
