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

package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	commonmodels "github.com/wso2/api-platform/common/models"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/secrets"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

var kindsDiscard = slog.New(slog.NewTextHandler(io.Discard, nil))

// kindsEncryption is a reversible stand-in for the AES-GCM provider, so the
// Secret adapter can run against a real SecretService.
type kindsEncryption struct{}

func (kindsEncryption) Name() string { return "kinds-test" }
func (kindsEncryption) Encrypt(plaintext []byte) (*encryption.EncryptedPayload, error) {
	return &encryption.EncryptedPayload{
		Provider: "kinds-test", KeyVersion: "v1", Ciphertext: append([]byte("enc:"), plaintext...),
	}, nil
}
func (kindsEncryption) Decrypt(payload *encryption.EncryptedPayload) ([]byte, error) {
	return []byte(strings.TrimPrefix(string(payload.Ciphertext), "enc:")), nil
}
func (kindsEncryption) HealthCheck() error { return nil }

type kindsFixture struct {
	h  *McpHandler
	db *MockStorage
}

// newKindsFixture builds an McpHandler over the real services, with an LLM service
// that knows the set-headers policy an LlmProxy needs.
func newKindsFixture(t *testing.T) *kindsFixture {
	t.Helper()
	server := createTestAPIServer()
	server.llmDeploymentService = utils.NewLLMDeploymentService(server.store, server.db, nil, nil,
		map[string]*api.LLMProviderTemplate{}, server.deploymentService, server.routerConfig,
		utils.NewStaticPolicyVersionResolver(map[string]string{"set-headers": "v1"}), nil)

	providers, err := encryption.NewProviderManager([]encryption.EncryptionProvider{kindsEncryption{}}, kindsDiscard)
	require.NoError(t, err)

	h := newMcpHandler(McpHandlerParams{
		RestAPIService:       server.restAPIService,
		MCPDeploymentService: server.mcpDeploymentService,
		LLMDeploymentService: server.llmDeploymentService,
		AgentService:         server.agentService,
		SecretService:        secrets.NewSecretsService(server.db, providers, kindsDiscard),
		APIKeyService:        server.apiKeyService,
		Logger:               kindsDiscard,
	})
	db, ok := server.db.(*MockStorage)
	require.True(t, ok)
	return &kindsFixture{h: h, db: db}
}

func kindsLLMProviderYAML(handle, template, displayName, auth string) []byte {
	return []byte(`apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProvider
metadata:
  name: ` + handle + `
spec:
  displayName: ` + displayName + `
  version: v1.0
  template: ` + template + `
  accessControl:
    mode: allow_all
  upstream:
    url: https://api.example.com/v1
` + auth)
}

func kindsLLMProxyYAML(handle, provider, displayName string) []byte {
	return []byte(`apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProxy
metadata:
  name: ` + handle + `
spec:
  displayName: ` + displayName + `
  version: v1.0
  context: /` + handle + `
  provider:
    id: ` + provider + `
`)
}

func kindsAgentYAML(displayName string) []byte {
	return []byte(`apiVersion: gateway.api-platform.wso2.com/v1
kind: Agent
metadata:
  name: trip-planner-v1.0
spec:
  displayName: ` + displayName + `
  version: v1.0
  context: /trip-planner/$version
  upstream:
    url: http://a2a-trip-planner:9099
  a2a:
    protocolVersion: "1.0"
    operationConfigs:
      transports:
        - protocolBinding: JSONRPC
          pathPrefix: /
`)
}

func kindsSecretYAML(value string) []byte {
	return []byte(`apiVersion: gateway.api-platform.wso2.com/v1
kind: Secret
metadata:
  name: db-pass
spec:
  displayName: DB Password
  value: ` + value + `
`)
}

// kindsJSON renders a response body exactly as the MCP SDK would put it on
// the wire, so assertions are made on what a client actually receives.
func kindsJSON(t *testing.T, v any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

// kindsDig walks nested JSON objects by key and fails cleanly on a missing step.
func kindsDig(t *testing.T, m map[string]any, keys ...string) any {
	t.Helper()
	var cur any = m
	for _, k := range keys {
		obj, ok := cur.(map[string]any)
		require.Truef(t, ok, "expected an object before %q, got %T", k, cur)
		cur, ok = obj[k]
		require.Truef(t, ok, "missing key %q", k)
	}
	return cur
}

var kindsCaller = &commonmodels.AuthContext{UserID: "alice", Roles: []string{"admin"}}

func TestNormalizeKindAcceptsEverySpelling(t *testing.T) {
	for raw, want := range map[string]string{
		"RestApi": models.KindRestApi, "rest-api": models.KindRestApi, " REST_API ": models.KindRestApi,
		"Mcp": models.KindMcp, "MCPProxy": models.KindMcp, "mcp-proxy": models.KindMcp,
		"LlmProxy": models.KindLlmProxy, "llm proxy": models.KindLlmProxy,
		"LlmProvider": models.KindLlmProvider, "llm-provider": models.KindLlmProvider,
		"LlmProviderTemplate": models.KindLlmProviderTemplate, "llm_provider_template": models.KindLlmProviderTemplate,
		"Agent": models.KindAgent, "AGENT": models.KindAgent,
		"Secret": models.KindSecret, "secret": models.KindSecret,
	} {
		got, err := normalizeKind(raw)
		require.NoErrorf(t, err, "spelling %q", raw)
		assert.Equalf(t, want, got, "spelling %q", raw)
	}

	_, err := normalizeKind("Widget")
	require.Error(t, err)
	assert.Equal(t,
		`unknown kind "Widget"; supported kinds: Agent, LlmProvider, LlmProviderTemplate, LlmProxy, Mcp, RestApi, Secret`,
		err.Error())
}

func TestCanonicalKindsIsSortedAndDeduplicated(t *testing.T) {
	assert.Equal(t,
		[]string{"Agent", "LlmProvider", "LlmProviderTemplate", "LlmProxy", "Mcp", "RestApi", "Secret"},
		canonicalKinds(), "two aliases map to Mcp, yet it must appear once")
}

// Each kind's collection is the authorization anchor: an MCP call is authorized
// as the REST operation on it, so a wrong path here authorizes the wrong route.
func TestBuildKindRegistryWiresEveryKind(t *testing.T) {
	f := newKindsFixture(t)

	want := []struct {
		kind       string
		collection string
		routable   bool
		keys       bool
	}{
		{models.KindRestApi, "/rest-apis", true, true},
		{models.KindMcp, "/mcp-proxies", true, false},
		{models.KindLlmProxy, "/llm-proxies", true, true},
		{models.KindLlmProvider, "/llm-providers", true, true},
		{models.KindAgent, "/agents", true, true},
		{models.KindLlmProviderTemplate, "/llm-provider-templates", false, false},
		{models.KindSecret, "/secrets", false, false},
	}
	require.Len(t, f.h.kinds, len(want))
	for _, w := range want {
		ops, ok := f.h.kinds[w.kind]
		require.Truef(t, ok, "%s missing from the registry", w.kind)
		assert.Equal(t, w.kind, ops.Kind)
		assert.Equalf(t, w.collection, ops.Collection, "%s collection", w.kind)
		assert.Equalf(t, w.routable, ops.Routable, "%s routable", w.kind)
		assert.Equalf(t, w.keys, ops.Keys != nil, "%s key-bearing", w.kind)
	}
}

// A kind whose service is not wired is left out rather than registered with a
// nil service that would panic on first use.
func TestBuildKindRegistryDropsWhatIsNotWired(t *testing.T) {
	h := newMcpHandler(McpHandlerParams{Logger: kindsDiscard})

	assert.NotContains(t, h.kinds, models.KindSecret, "no secret service means no Secret kind")
	for kind, ops := range h.kinds {
		assert.Nilf(t, ops.Keys, "%s must not advertise API keys without a key service", kind)
	}
	_, err := h.resolveKind("Secret", classAny)
	require.Error(t, err)
	assert.Equal(t, `kind "Secret" is not supported by this gateway`, err.Error())
}

func TestResolveKindEnforcesTheClass(t *testing.T) {
	f := newKindsFixture(t)

	ops, err := f.h.resolveKind("mcp-proxy", classRoutable)
	require.NoError(t, err)
	assert.Equal(t, models.KindMcp, ops.Kind)

	ops, err = f.h.resolveKind("secret", classConfig)
	require.NoError(t, err)
	assert.Equal(t, models.KindSecret, ops.Kind)

	for _, kind := range []string{"RestApi", "Secret"} {
		_, err = f.h.resolveKind(kind, classAny)
		assert.NoErrorf(t, err, "the read tools accept %s", kind)
	}

	_, err = f.h.resolveKind("Secret", classRoutable)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `kind "Secret" is a supporting configuration`)
	assert.Contains(t, err.Error(), "accept: Agent, LlmProvider, LlmProxy, Mcp, RestApi")

	_, err = f.h.resolveKind("RestApi", classConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `kind "RestApi" is a routable resource`)
	assert.Contains(t, err.Error(), "accept: LlmProviderTemplate, Secret")
}

func TestResolveKeyBearing(t *testing.T) {
	f := newKindsFixture(t)

	ops, err := f.h.resolveKeyBearing("llm-proxy")
	require.NoError(t, err)
	assert.Equal(t, models.KindLlmProxy, ops.Kind)

	for _, kind := range []string{"Mcp", "LlmProviderTemplate", "Secret"} {
		_, err := f.h.resolveKeyBearing(kind)
		require.Errorf(t, err, "%s has no API keys", kind)
		assert.Contains(t, err.Error(), "the API key tools accept: Agent, LlmProvider, LlmProxy, RestApi")
	}
}

type kindsCase struct {
	kind         string
	handle       string
	displayName  string
	renamed      string
	setup        func(t *testing.T, f *kindsFixture)
	create       []byte
	update       []byte
	routable     bool
	notFoundText string
}

func kindsCases(t *testing.T) []kindsCase {
	withTemplate := func(t *testing.T, f *kindsFixture) {
		_, err := f.h.kinds[models.KindLlmProviderTemplate].Create(
			createLLMTemplateBody(t, "base-template", "Base Template"), "setup", kindsDiscard)
		require.NoError(t, err)
	}
	withProvider := func(t *testing.T, f *kindsFixture) {
		withTemplate(t, f)
		_, err := f.h.kinds[models.KindLlmProvider].Create(
			kindsLLMProviderYAML("base-provider", "base-template", "Base Provider", ""), "setup", kindsDiscard)
		require.NoError(t, err)
	}

	return []kindsCase{
		{kind: models.KindRestApi, handle: "orders", displayName: "Orders", renamed: "Orders Renamed",
			create:   createTestRestAPIRequestBody(t, "orders", "Orders", "v1.0", "/orders"),
			update:   createTestRestAPIRequestBody(t, "orders", "Orders Renamed", "v1.0", "/orders"),
			routable: true},
		{kind: models.KindMcp, handle: "tools", displayName: "Tools", renamed: "Tools Renamed",
			create:   createTestMCPRequestBody(t, "tools", "Tools", "v1.0", "/tools"),
			update:   createTestMCPRequestBody(t, "tools", "Tools Renamed", "v1.0", "/tools"),
			routable: true},
		{kind: models.KindLlmProvider, handle: "openai", displayName: "OpenAI", renamed: "OpenAI Renamed",
			setup:    withTemplate,
			create:   kindsLLMProviderYAML("openai", "base-template", "OpenAI", ""),
			update:   kindsLLMProviderYAML("openai", "base-template", "OpenAI Renamed", ""),
			routable: true},
		{kind: models.KindLlmProxy, handle: "chat-proxy", displayName: "Chat Proxy", renamed: "Chat Proxy Renamed",
			setup:    withProvider,
			create:   kindsLLMProxyYAML("chat-proxy", "base-provider", "Chat Proxy"),
			update:   kindsLLMProxyYAML("chat-proxy", "base-provider", "Chat Proxy Renamed"),
			routable: true},
		{kind: models.KindAgent, handle: "trip-planner-v1.0", displayName: "Trip Planner", renamed: "Trip Planner Renamed",
			create:   kindsAgentYAML("Trip Planner"),
			update:   kindsAgentYAML("Trip Planner Renamed"),
			routable: true},
		{kind: models.KindLlmProviderTemplate, handle: "custom-template", displayName: "Custom", renamed: "Custom Renamed",
			create: createLLMTemplateBody(t, "custom-template", "Custom"),
			update: createLLMTemplateBody(t, "custom-template", "Custom Renamed")},
		// A Secret's displayName is not what changes on update; its value is,
		// and that is asserted separately below. Both manifests keep the name.
		{kind: models.KindSecret, handle: "db-pass", displayName: "DB Password", renamed: "DB Password",
			create: kindsSecretYAML("s3cret-one"),
			update: kindsSecretYAML("s3cret-two")},
	}
}

func TestEveryKindCreatesReadsListsUpdatesAndDeletes(t *testing.T) {
	for _, tc := range kindsCases(t) {
		t.Run(tc.kind, func(t *testing.T) {
			f := newKindsFixture(t)
			if tc.setup != nil {
				tc.setup(t, f)
			}
			ops := f.h.kinds[tc.kind]
			require.NotNil(t, ops)

			created, err := ops.Create(tc.create, "corr-create", kindsDiscard)
			require.NoError(t, err)
			body := kindsJSON(t, created)
			assert.Equal(t, tc.kind, body["kind"])
			assert.Equal(t, tc.handle, kindsDig(t, body, "metadata", "name"))
			assert.Equal(t, tc.displayName, kindsDig(t, body, "spec", "displayName"))

			got, err := ops.Get(tc.handle)
			require.NoError(t, err)
			body = kindsJSON(t, got)
			assert.Equal(t, tc.handle, kindsDig(t, body, "metadata", "name"))

			rows, err := ops.List()
			require.NoError(t, err)
			var row *listedResource
			for i := range rows {
				if rows[i].ID == tc.handle {
					row = &rows[i]
				}
			}
			require.NotNilf(t, row, "%s %q missing from the list", tc.kind, tc.handle)
			assert.Equal(t, tc.displayName, row.DisplayName)
			assert.NotNil(t, row.Resource, "each row carries the full manifest for the one-kind listing")
			if tc.routable {
				assert.Equal(t, "v1.0", row.Version)
				assert.Equal(t, string(models.StateDeployed), row.State)
			} else {
				assert.Empty(t, row.State, "supporting configuration has no deployment state")
			}

			updated, err := ops.Update(tc.handle, tc.update, "corr-update", kindsDiscard)
			require.NoError(t, err)
			body = kindsJSON(t, updated)
			assert.Equal(t, tc.renamed, kindsDig(t, body, "spec", "displayName"))

			require.NoError(t, ops.Delete(tc.handle, "corr-delete", kindsDiscard))
			_, err = ops.Get(tc.handle)
			assert.Errorf(t, err, "%s %q must be gone after delete", tc.kind, tc.handle)
		})
	}
}

// Every service error reaches the tool layer unchanged; mapping it to a
// message is the tool's job, not the adapter's.
func TestEveryKindReturnsServiceErrors(t *testing.T) {
	for _, tc := range kindsCases(t) {
		t.Run(tc.kind, func(t *testing.T) {
			f := newKindsFixture(t)
			ops := f.h.kinds[tc.kind]

			_, err := ops.Create([]byte("kind: Nonsense\n"), "c", kindsDiscard)
			assert.Error(t, err, "an invalid manifest is rejected")

			_, err = ops.Get("does-not-exist")
			assert.Error(t, err)

			_, err = ops.Update("does-not-exist", tc.create, "c", kindsDiscard)
			assert.Error(t, err)

			assert.Error(t, ops.Delete("does-not-exist", "c", kindsDiscard))
		})
	}
}

// An LLM provider's upstream credential is accepted on write and never
// rendered back, on the create response or on a later read.
func TestLLMProviderNeverEchoesItsUpstreamCredential(t *testing.T) {
	f := newKindsFixture(t)
	_, err := f.h.kinds[models.KindLlmProviderTemplate].Create(
		createLLMTemplateBody(t, "base-template", "Base"), "c", kindsDiscard)
	require.NoError(t, err)
	const credential = "sk-must-never-come-back-0123456789"
	auth := "    auth:\n      type: api-key\n      header: Authorization\n      value: " + credential + "\n"

	created, err := f.h.kinds[models.KindLlmProvider].Create(
		kindsLLMProviderYAML("openai", "base-template", "OpenAI", auth), "c", kindsDiscard)
	require.NoError(t, err)
	got, err := f.h.kinds[models.KindLlmProvider].Get("openai")
	require.NoError(t, err)
	rows, err := f.h.kinds[models.KindLlmProvider].List()
	require.NoError(t, err)

	for name, v := range map[string]any{"create": created, "get": got, "list": rows[0].Resource} {
		raw, err := json.Marshal(v)
		require.NoError(t, err)
		assert.NotContainsf(t, string(raw), credential, "%s leaked the credential", name)
	}
	assert.Equal(t, "Authorization", kindsDig(t, kindsJSON(t, got), "spec", "upstream", "auth", "header"),
		"the auth block itself is kept; only the credential is cleared")
}

// The agent body echoes the context resolved, as the REST handler does,
// rather than the stored $version placeholder.
func TestAgentBodyResolvesTheContext(t *testing.T) {
	f := newKindsFixture(t)

	created, err := f.h.kinds[models.KindAgent].Create(kindsAgentYAML("Trip Planner"), "c", kindsDiscard)
	require.NoError(t, err)
	assert.Equal(t, "/trip-planner/v1.0", kindsDig(t, kindsJSON(t, created), "spec", "context"))

	got, err := f.h.kinds[models.KindAgent].Get("trip-planner-v1.0")
	require.NoError(t, err)
	assert.Equal(t, "/trip-planner/v1.0", kindsDig(t, kindsJSON(t, got), "spec", "context"))
}

// A Secret's value is returned only by Get, exactly as GET /secrets/{id} does.
// Create, Update and List never carry it.
func TestSecretValueIsReturnedOnlyByGet(t *testing.T) {
	f := newKindsFixture(t)
	ops := f.h.kinds[models.KindSecret]

	created, err := ops.Create(kindsSecretYAML("s3cret-one"), "c", kindsDiscard)
	require.NoError(t, err)
	assert.NotContains(t, kindsDig(t, kindsJSON(t, created), "spec"), "value")

	got, err := ops.Get("db-pass")
	require.NoError(t, err)
	assert.Equal(t, "s3cret-one", kindsDig(t, kindsJSON(t, got), "spec", "value"))

	rows, err := ops.List()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	raw, err := json.Marshal(rows[0].Resource)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "s3cret-one", "a list row must never carry the value")

	updated, err := ops.Update("db-pass", kindsSecretYAML("s3cret-two"), "c", kindsDiscard)
	require.NoError(t, err)
	assert.NotContains(t, kindsDig(t, kindsJSON(t, updated), "spec"), "value")

	got, err = ops.Get("db-pass")
	require.NoError(t, err)
	assert.Equal(t, "s3cret-two", kindsDig(t, kindsJSON(t, got), "spec", "value"), "the update took effect")
}

// kindsCorruptRow is a stored configuration whose source cannot be marshalled,
// standing in for a row the renderer cannot read back.
func kindsCorruptRow(kind, handle string) *models.StoredConfig {
	return &models.StoredConfig{UUID: "corrupt-" + handle, Kind: kind, Handle: handle, SourceConfiguration: make(chan int)}
}

// A row that cannot be rendered fails with a fixed message: the underlying
// marshal error is logged, never returned.
func TestBodyHelpersFailWithoutLeakingTheCause(t *testing.T) {
	h := &McpHandler{logger: kindsDiscard}
	for _, tc := range []struct {
		name   string
		render func(*slog.Logger, *models.StoredConfig) (any, error)
		want   string
	}{
		{"MCP proxy", h.mcpProxyBody, "failed to read stored MCP proxy configuration"},
		{"LLM proxy", h.llmProxyBody, "failed to read stored LLM proxy configuration"},
		{"LLM provider", h.llmProviderBody, "failed to read stored LLM provider configuration"},
		{"agent", h.agentBody, "failed to read stored agent configuration"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := tc.render(kindsDiscard, kindsCorruptRow("Any", "broken"))
			require.Error(t, err)
			assert.Nil(t, body)
			assert.Equal(t, tc.want, err.Error())
		})
	}
}

// One unreadable row fails the whole list rather than silently returning a
// shorter one the caller would take as complete.
func TestListFailsOnAnUnreadableRow(t *testing.T) {
	for _, kind := range []string{models.KindMcp, models.KindLlmProvider, models.KindLlmProxy, models.KindAgent} {
		t.Run(kind, func(t *testing.T) {
			f := newKindsFixture(t)
			require.NoError(t, f.db.SaveConfig(kindsCorruptRow(kind, "broken")))

			rows, err := f.h.kinds[kind].List()

			require.Error(t, err)
			assert.Nil(t, rows)
			assert.Contains(t, err.Error(), "failed to read stored")
		})
	}
}

func TestListReturnsTheStorageError(t *testing.T) {
	for _, kind := range []string{models.KindMcp, models.KindAgent} {
		t.Run(kind, func(t *testing.T) {
			f := newKindsFixture(t)
			f.db.getErr = errors.New("database is locked")

			_, err := f.h.kinds[kind].List()

			assert.Error(t, err)
		})
	}
}

func TestStoredConfigRowCarriesTheSummaryFields(t *testing.T) {
	cfg := &models.StoredConfig{
		Handle: "orders", DisplayName: "Orders", Version: "v2.0", DesiredState: models.StateUndeployed,
	}
	body := map[string]any{"full": "manifest"}

	assert.Equal(t, listedResource{
		ID: "orders", DisplayName: "Orders", Version: "v2.0", State: "undeployed", Resource: body,
	}, storedConfigRow(cfg, body))
}

func TestDerefString(t *testing.T) {
	assert.Equal(t, "", derefString(nil))
	v := "v1.0"
	assert.Equal(t, "v1.0", derefString(&v))
}

// Keys are issued with a name because MockStorage does not return storage.ErrNotFound
// from the lookup that name generation relies on.
func TestAPIKeyOpsRunTheWholeKeyLifecycle(t *testing.T) {
	f := newKindsFixture(t)
	_, err := f.h.kinds[models.KindRestApi].Create(
		createTestRestAPIRequestBody(t, "orders", "Orders", "v1.0", "/orders"), "c", kindsDiscard)
	require.NoError(t, err)
	keys := f.h.kinds[models.KindRestApi].Keys
	name := func(s string) *string { return &s }

	issued, err := keys.Issue("orders", api.APIKeyCreationRequest{Name: name("prod-key")}, kindsCaller, "c", kindsDiscard)
	require.NoError(t, err)
	generated := kindsDig(t, kindsJSON(t, issued), "apiKey", "apiKey").(string)
	assert.True(t, strings.HasPrefix(generated, "apip_"), "a generated key is returned in plain text once")
	assert.Equal(t, "alice", kindsDig(t, kindsJSON(t, issued), "apiKey", "createdBy"))

	external := strings.Repeat("x", 40)
	registered, err := keys.Issue("orders",
		api.APIKeyCreationRequest{Name: name("ext-key"), ApiKey: &external}, kindsCaller, "c", kindsDiscard)
	require.NoError(t, err)
	assert.NotContains(t, kindsJSON(t, registered)["apiKey"], "apiKey", "a caller-supplied value is never echoed")

	listed, err := keys.List("orders", kindsCaller, "c", kindsDiscard)
	require.NoError(t, err)
	raw, err := json.Marshal(listed)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), generated, "list returns masked values only")
	assert.Contains(t, string(raw), `"prod-key"`)
	assert.Contains(t, string(raw), `"ext-key"`)

	rotated, err := keys.Rotate("orders", "prod-key", api.APIKeyRegenerationRequest{}, kindsCaller, "c", kindsDiscard)
	require.NoError(t, err)
	newValue := kindsDig(t, kindsJSON(t, rotated), "apiKey", "apiKey").(string)
	assert.True(t, strings.HasPrefix(newValue, "apip_"))
	assert.NotEqual(t, generated, newValue)

	replacement := strings.Repeat("y", 40)
	_, err = keys.Update("orders", "ext-key", api.APIKeyCreationRequest{ApiKey: &replacement}, kindsCaller, "c", kindsDiscard)
	require.NoError(t, err, "a supplied value can be installed over a key that was itself supplied")

	_, err = keys.Update("orders", "prod-key", api.APIKeyCreationRequest{ApiKey: &replacement}, kindsCaller, "c", kindsDiscard)
	require.Error(t, err)
	assert.True(t, storage.IsOperationNotAllowedError(err), "a generated key cannot be overwritten: %v", err)

	_, err = keys.Revoke("orders", "prod-key", kindsCaller, "c", kindsDiscard)
	require.NoError(t, err)
}

// Only the creator can rotate or update a key; the creator or an admin can revoke it.
func TestAPIKeyOpsEnforceTheCreatorCheck(t *testing.T) {
	f := newKindsFixture(t)
	_, err := f.h.kinds[models.KindRestApi].Create(
		createTestRestAPIRequestBody(t, "orders", "Orders", "v1.0", "/orders"), "c", kindsDiscard)
	require.NoError(t, err)
	keys := f.h.kinds[models.KindRestApi].Keys
	generatedName, suppliedName := "alice-key", "alice-ext-key"
	supplied := strings.Repeat("a", 40)
	_, err = keys.Issue("orders", api.APIKeyCreationRequest{Name: &generatedName}, kindsCaller, "c", kindsDiscard)
	require.NoError(t, err)
	_, err = keys.Issue("orders",
		api.APIKeyCreationRequest{Name: &suppliedName, ApiKey: &supplied}, kindsCaller, "c", kindsDiscard)
	require.NoError(t, err)

	bob := &commonmodels.AuthContext{UserID: "bob", Roles: []string{"developer"}}

	_, err = keys.Rotate("orders", generatedName, api.APIKeyRegenerationRequest{}, bob, "c", kindsDiscard)
	assert.Error(t, err, "bob cannot rotate alice's key")
	carol := &commonmodels.AuthContext{UserID: "carol", Roles: []string{"admin"}}
	_, err = keys.Rotate("orders", generatedName, api.APIKeyRegenerationRequest{}, carol, "c", kindsDiscard)
	assert.Error(t, err, "rotation is creator-only even for an admin")

	replacement := strings.Repeat("b", 40)
	_, err = keys.Update("orders", suppliedName, api.APIKeyCreationRequest{ApiKey: &replacement}, bob, "c", kindsDiscard)
	require.Error(t, err, "bob cannot install a value over alice's key")
	assert.Contains(t, err.Error(), "not authorized to update API key")

	_, err = keys.Revoke("orders", generatedName, bob, "c", kindsDiscard)
	require.Error(t, err, "bob cannot revoke alice's key")
	assert.Contains(t, err.Error(), "API key revocation failed")

	listed, err := keys.List("orders", kindsCaller, "c", kindsDiscard)
	require.NoError(t, err)
	raw, err := json.Marshal(listed)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"`+generatedName+`"`, "the refused revoke left the key in place")
	assert.NotContains(t, string(raw), strings.Repeat("b", 5), "the refused update did not install bob's value")

	// Revocation, unlike rotation, is also open to an admin: the key service
	// deliberately lets the admin role revoke any key of an API.
	_, err = keys.Revoke("orders", generatedName, carol, "c", kindsDiscard)
	assert.NoError(t, err, "an admin may revoke another user's key")

	_, err = keys.Revoke("orders", suppliedName, kindsCaller, "c", kindsDiscard)
	assert.NoError(t, err, "the creator may revoke her own key")
}

func TestAPIKeyOpsReportAMissingParent(t *testing.T) {
	f := newKindsFixture(t)
	keyName := "k1"

	_, err := f.h.kinds[models.KindRestApi].Keys.Issue("no-such-api",
		api.APIKeyCreationRequest{Name: &keyName}, kindsCaller, "c", kindsDiscard)

	require.Error(t, err)
	assert.True(t, storage.IsNotFoundError(err), "the tool layer maps this sentinel to its message: %v", err)
}

// The adapter passes its own kind to the key service instead of relying on the RestApi default.
func TestAPIKeyOpsPassTheKindExplicitly(t *testing.T) {
	f := newKindsFixture(t)
	_, err := f.h.kinds[models.KindLlmProviderTemplate].Create(
		createLLMTemplateBody(t, "base-template", "Base"), "c", kindsDiscard)
	require.NoError(t, err)
	_, err = f.h.kinds[models.KindLlmProvider].Create(
		kindsLLMProviderYAML("openai", "base-template", "OpenAI", ""), "c", kindsDiscard)
	require.NoError(t, err)
	keyName := "llm-key"

	_, err = f.h.kinds[models.KindLlmProvider].Keys.Issue("openai",
		api.APIKeyCreationRequest{Name: &keyName}, kindsCaller, "c", kindsDiscard)
	require.NoError(t, err)
	listed, err := f.h.kinds[models.KindLlmProvider].Keys.List("openai", kindsCaller, "c", kindsDiscard)
	require.NoError(t, err)
	raw, err := json.Marshal(listed)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"llm-key"`)

	_, err = f.h.kinds[models.KindRestApi].Keys.Issue("openai",
		api.APIKeyCreationRequest{Name: &keyName}, kindsCaller, "c", kindsDiscard)
	require.Error(t, err)
	assert.True(t, storage.IsNotFoundError(err), "openai is an LlmProvider, not a RestApi: %v", err)
}
