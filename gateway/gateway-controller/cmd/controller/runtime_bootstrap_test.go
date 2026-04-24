package main

import (
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
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
		newDiscardLogger(),
		true,
	)

	assert.NoError(t, err)
}
