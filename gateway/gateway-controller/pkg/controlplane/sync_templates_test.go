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

package controlplane

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

const (
	tmplEvtProviderID   = "11111111-2222-3333-4444-555555555555"
	tmplEvtDeploymentID = "dep-tmpl-1"
	tmplEvtHandle       = "acme-llm"
)

func tmplEvtProviderYAML(templateHandle string) string {
	return tmplEvtProviderYAMLNamed("acme-prod", templateHandle)
}

func tmplEvtProviderYAMLNamed(name, templateHandle string) string {
	return `apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProvider
metadata:
  name: ` + name + `
spec:
  displayName: ` + name + `
  version: v1.0
  template: ` + templateHandle + `
  upstream:
    url: https://inference.acme.example/v1
  accessControl:
    mode: allow_all
`
}

func tmplEvtTemplateYAML(name string) string {
	return tmplEvtTemplateYAMLWith(name, "$.meta.tokens.in")
}

// tmplEvtTemplateYAMLWith renders the fixture template with promptTokens read
// from promptIdentifier, so a test can serve an edited copy.
func tmplEvtTemplateYAMLWith(name, promptIdentifier string) string {
	return `apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProviderTemplate
metadata:
  name: ` + name + `
spec:
  displayName: Acme Inference
  promptTokens:
    location: payload
    identifier: ` + promptIdentifier + `
  completionTokens:
    location: payload
    identifier: $.meta.tokens.out
`
}

func tmplEvtZip(t *testing.T, entryName, yamlContent string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create(entryName)
	require.NoError(t, err)
	_, err = f.Write([]byte(yamlContent))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

type tmplEvtBatchEntry struct {
	deploymentID, fileName, yaml string
}

// tmplEvtBatch packages deployments as the control plane's fetch-batch does:
// a tar.gz with one directory per deployment ID holding the artifact's YAML.
func tmplEvtBatch(t *testing.T, entries ...tmplEvtBatchEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name: e.deploymentID + "/" + e.fileName, Mode: 0o644, Size: int64(len(e.yaml)), ModTime: time.Now(),
		}))
		_, err := tw.Write([]byte(e.yaml))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

// templateControlPlane serves an LLM provider, LLM provider templates and the
// sync batch, and counts template fetches.
type templateControlPlane struct {
	mu             sync.Mutex
	providerYAML   string
	templates      map[string]string // handle -> YAML served for it
	secrets        map[string]string // secret handle -> value served for it
	batch          []byte
	templateFetchs []string
}

func (cp *templateControlPlane) templateFetches() []string {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	return append([]string(nil), cp.templateFetchs...)
}

type recordingPusher struct {
	mu     sync.Mutex
	pushes int
}

func (p *recordingPusher) IsConnected() bool { return true }
func (p *recordingPusher) IsOnPrem() bool    { return false }
func (p *recordingPusher) PushArtifact(string, *models.StoredConfig, string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pushes++
	return nil
}
func (p *recordingPusher) SubmitArtifactPush(task func()) {
	p.mu.Lock()
	p.pushes++
	p.mu.Unlock()
}

func (p *recordingPusher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pushes
}

// newTemplateEventsHarness is the agent events harness (a real SQLite store and
// a WebSocket that receives acks) with the LLM deployment services wired up and
// the API utils service pointed at a control plane serving providers and
// templates.
func newTemplateEventsHarness(t *testing.T) (*agentEventsHarness, *templateControlPlane, *recordingPusher) {
	t.Helper()
	h := newAgentEventsHarness(t)

	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiService := utils.NewAPIDeploymentService(h.client.store, h.db, nil, nil, routerConfig, h.hub,
		agentEvtGatewayID, agentEvtSecretResolver{}, nil)
	h.client.deploymentService = apiService
	llmService := utils.NewLLMDeploymentService(h.client.store, h.db, nil, nil, nil,
		apiService, routerConfig, nil, nil)
	pusher := &recordingPusher{}
	llmService.SetControlPlanePusher(pusher, true)
	h.client.llmDeploymentService = llmService

	cp := &templateControlPlane{templates: map[string]string{}, secrets: map[string]string{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/internal/v1/secrets/", func(w http.ResponseWriter, r *http.Request) {
		handle := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/internal/v1/secrets/"), "/value")
		cp.mu.Lock()
		value, ok := cp.secrets[handle]
		cp.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"value": value})
	})
	mux.HandleFunc("/api/internal/v1/llm-providers/", func(w http.ResponseWriter, r *http.Request) {
		cp.mu.Lock()
		body := cp.providerYAML
		cp.mu.Unlock()
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(tmplEvtZip(t, "llm-provider-"+tmplEvtProviderID+".yaml", body))
	})
	mux.HandleFunc("/api/internal/v1/llm-provider-templates/", func(w http.ResponseWriter, r *http.Request) {
		handle := strings.TrimPrefix(r.URL.Path, "/api/internal/v1/llm-provider-templates/")
		cp.mu.Lock()
		cp.templateFetchs = append(cp.templateFetchs, handle)
		body, ok := cp.templates[handle]
		cp.mu.Unlock()
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":404,"message":"Not Found"}`))
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(tmplEvtZip(t, "llm-provider-template-"+handle+".yaml", body))
	})
	mux.HandleFunc("/api/internal/v1/deployments/fetch-batch", func(w http.ResponseWriter, r *http.Request) {
		cp.mu.Lock()
		body := cp.batch
		cp.mu.Unlock()
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(body)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	h.client.apiUtilsService = utils.NewAPIUtilsService(utils.PlatformAPIConfig{
		BaseURL: server.URL + "/api/internal/v1",
		Token:   agentEvtToken,
	}, testHTTPClient(false), h.client.logger)

	return h, cp, pusher
}

func tmplEvtDeployedEvent(t *testing.T) map[string]any {
	return tmplEvtDeployedEventFor(t, tmplEvtDeploymentID, time.Now())
}

// tmplEvtDeployedEventFor is a redeploy of the fixture provider: a new
// deployment ID and a later performedAt, so the gateway does not treat it as a
// stale repeat.
func tmplEvtDeployedEventFor(t *testing.T, deploymentID string, performedAt time.Time) map[string]any {
	return agentEvtEvent(t, "llmprovider.deployed", map[string]any{
		"providerId": tmplEvtProviderID, "deploymentId": deploymentID, "performedAt": performedAt,
	}, "corr-provider-"+deploymentID)
}

func tmplEvtTemplateEvents(hub *mockControlPlaneEventHub, action string) []eventhub.Event {
	var events []eventhub.Event
	for _, e := range hub.publishedEvents {
		if e.event.EventType == eventhub.EventTypeLLMTemplate && e.event.Action == action {
			events = append(events, e.event)
		}
	}
	return events
}

func tmplEvtTemplateCreateEvents(hub *mockControlPlaneEventHub) []eventhub.Event {
	return tmplEvtTemplateEvents(hub, "CREATE")
}

func TestProviderDeploy_FetchesMissingTemplateFromControlPlane(t *testing.T) {
	h, cp, pusher := newTemplateEventsHarness(t)
	cp.providerYAML = tmplEvtProviderYAML(tmplEvtHandle)
	cp.templates[tmplEvtHandle] = tmplEvtTemplateYAML(tmplEvtHandle)

	h.client.handleLLMProviderDeployedEvent(tmplEvtDeployedEvent(t))

	ack := h.nextAck(t)
	assert.Equal(t, "success", ack.Status, "provider deploy must succeed once its template is fetched (error code %q)", ack.ErrorCode)
	assert.Equal(t, []string{tmplEvtHandle}, cp.templateFetches())

	stored, err := h.db.GetLLMProviderTemplateByHandle(tmplEvtHandle)
	require.NoError(t, err, "the fetched template must be saved")
	assert.Equal(t, tmplEvtHandle, stored.GetHandle())
	assert.Equal(t, "Acme Inference", stored.Configuration.Spec.DisplayName)

	creates := tmplEvtTemplateCreateEvents(h.hub)
	require.Len(t, creates, 1, "saving the template publishes one CREATE event")
	assert.NotEqual(t, "corr-provider-"+tmplEvtDeploymentID, creates[0].EventID,
		"the template event must not reuse the provider's event ID, which the event hub would deduplicate")

	assert.True(t, stored.IsFromControlPlane(), "the gateway marks a template it fetched from the control plane")

	assert.Zero(t, pusher.count(), "a template fetched from the control plane is never pushed back to it")
}

func TestProviderDeploy_TemplateNotFromControlPlane_NeverFetched(t *testing.T) {
	h, cp, _ := newTemplateEventsHarness(t)
	cp.providerYAML = tmplEvtProviderYAML(tmplEvtHandle)
	cp.templates[tmplEvtHandle] = tmplEvtTemplateYAML(tmplEvtHandle)

	_, err := h.client.llmDeploymentService.CreateLLMProviderTemplate(utils.LLMTemplateParams{
		Spec: []byte(tmplEvtTemplateYAML(tmplEvtHandle)), ContentType: "application/yaml",
		CorrelationID: "corr-preexisting", Logger: h.client.logger,
	})
	require.NoError(t, err, "precondition: the gateway holds a template applied through its management API")

	h.client.handleLLMProviderDeployedEvent(tmplEvtDeployedEvent(t))

	ack := h.nextAck(t)
	assert.Equal(t, "success", ack.Status, "error code %q", ack.ErrorCode)
	assert.Empty(t, cp.templateFetches(),
		"a template that did not come from the control plane is used as it is, without even a fetch")
}

func TestProviderDeploy_ControlPlaneHasNoTemplate_FailsAsBefore(t *testing.T) {
	h, cp, _ := newTemplateEventsHarness(t)
	cp.providerYAML = tmplEvtProviderYAML(tmplEvtHandle) // no template served: 404

	h.client.handleLLMProviderDeployedEvent(tmplEvtDeployedEvent(t))

	ack := h.nextAck(t)
	assert.Equal(t, "failed", ack.Status, "without the template the provider deploy fails, as it did before")
	assert.Equal(t, []string{tmplEvtHandle}, cp.templateFetches())
	_, err := h.db.GetLLMProviderTemplateByHandle(tmplEvtHandle)
	assert.True(t, storage.IsNotFoundError(err), "nothing is saved for a template the control plane lacks")
}

func TestProviderDeploy_ControlPlaneServesWrongTemplate_Rejected(t *testing.T) {
	h, cp, _ := newTemplateEventsHarness(t)
	cp.providerYAML = tmplEvtProviderYAML(tmplEvtHandle)
	cp.templates[tmplEvtHandle] = tmplEvtTemplateYAML("some-other-template")

	h.client.handleLLMProviderDeployedEvent(tmplEvtDeployedEvent(t))

	ack := h.nextAck(t)
	assert.Equal(t, "failed", ack.Status)
	_, err := h.db.GetLLMProviderTemplateByHandle(tmplEvtHandle)
	assert.True(t, storage.IsNotFoundError(err), "a template whose name differs from the requested handle is not saved")
	_, err = h.db.GetLLMProviderTemplateByHandle("some-other-template")
	assert.True(t, storage.IsNotFoundError(err), "nor under its own name")
}

func TestProviderDeploy_UnsafeTemplateHandle_NotFetched(t *testing.T) {
	h, cp, _ := newTemplateEventsHarness(t)
	cp.providerYAML = tmplEvtProviderYAML(`"../secrets"`)

	h.client.handleLLMProviderDeployedEvent(tmplEvtDeployedEvent(t))

	ack := h.nextAck(t)
	assert.Equal(t, "failed", ack.Status)
	assert.Empty(t, cp.templateFetches(), "a handle that is not a single safe path segment is never requested")
}

func TestProviderSync_FetchesMissingTemplateBeforeProvider(t *testing.T) {
	h, cp, _ := newTemplateEventsHarness(t)
	cp.templates[tmplEvtHandle] = tmplEvtTemplateYAML(tmplEvtHandle)
	cp.batch = tmplEvtBatch(t, tmplEvtBatchEntry{tmplEvtDeploymentID, "llm-provider-" + tmplEvtProviderID + ".yaml",
		tmplEvtProviderYAML(tmplEvtHandle)})

	h.client.processSyncFetchBatch([]models.ControlPlaneDeployment{{
		ArtifactID:   tmplEvtProviderID,
		DeploymentID: tmplEvtDeploymentID,
		Kind:         string(models.KindLlmProvider),
		State:        "DEPLOYED",
		DeployedAt:   time.Now().Truncate(time.Millisecond),
	}}, agentEvtGatewayID)

	assert.Equal(t, []string{tmplEvtHandle}, cp.templateFetches())
	_, err := h.db.GetLLMProviderTemplateByHandle(tmplEvtHandle)
	require.NoError(t, err, "the startup sync saves the provider's template")
	cfg, err := h.db.GetConfig(tmplEvtProviderID)
	require.NoError(t, err, "and then deploys the provider")
	assert.Equal(t, "acme-prod", cfg.Handle)
}

func TestLLMProviderTemplateHandle(t *testing.T) {
	assert.Equal(t, "acme-llm", llmProviderTemplateHandle([]byte(tmplEvtProviderYAML("acme-llm"))))
	assert.Equal(t, "acme-llm", llmProviderTemplateHandle([]byte(tmplEvtProviderYAML(`"  acme-llm  "`))))
	assert.Empty(t, llmProviderTemplateHandle([]byte("kind: LlmProvider\nspec: {}\n")))
	assert.Empty(t, llmProviderTemplateHandle([]byte("spec: [unterminated")))
}

// deployAndAck deploys the fixture provider under deploymentID and returns its ack.
func deployAndAck(t *testing.T, h *agentEventsHarness, deploymentID string, performedAt time.Time) DeploymentAckPayload {
	t.Helper()
	h.client.handleLLMProviderDeployedEvent(tmplEvtDeployedEventFor(t, deploymentID, performedAt))
	return h.nextAck(t)
}

func TestProviderRedeploy_EditedControlPlaneTemplate_IsRefreshed(t *testing.T) {
	h, cp, pusher := newTemplateEventsHarness(t)
	cp.providerYAML = tmplEvtProviderYAML(tmplEvtHandle)
	cp.templates[tmplEvtHandle] = tmplEvtTemplateYAML(tmplEvtHandle)
	first := time.Now().Truncate(time.Millisecond)
	require.Equal(t, "success", deployAndAck(t, h, "dep-1", first).Status, "precondition: first deploy fetches the template")

	// The template is edited in the control plane, then the provider is redeployed.
	cp.mu.Lock()
	cp.templates[tmplEvtHandle] = tmplEvtTemplateYAMLWith(tmplEvtHandle, "$.usage.input_tokens")
	cp.mu.Unlock()
	ack := deployAndAck(t, h, "dep-2", first.Add(time.Second))
	assert.Equal(t, "success", ack.Status, "error code %q", ack.ErrorCode)

	assert.Equal(t, []string{tmplEvtHandle, tmplEvtHandle}, cp.templateFetches(), "every deploy refreshes a control-plane template")
	stored, err := h.db.GetLLMProviderTemplateByHandle(tmplEvtHandle)
	require.NoError(t, err)
	require.NotNil(t, stored.Configuration.Spec.PromptTokens)
	assert.Equal(t, "$.usage.input_tokens", stored.Configuration.Spec.PromptTokens.Identifier, "the edit reached the gateway")
	assert.True(t, stored.IsFromControlPlane(), "the refreshed copy keeps its origin mark")
	assert.Len(t, tmplEvtTemplateEvents(h.hub, "UPDATE"), 1, "the change publishes one UPDATE event")
	assert.Zero(t, pusher.count(), "a refreshed template is never pushed back to the control plane")
}

func TestProviderRedeploy_UnchangedControlPlaneTemplate_NoUpdate(t *testing.T) {
	h, cp, _ := newTemplateEventsHarness(t)
	cp.providerYAML = tmplEvtProviderYAML(tmplEvtHandle)
	cp.templates[tmplEvtHandle] = tmplEvtTemplateYAML(tmplEvtHandle)
	first := time.Now().Truncate(time.Millisecond)
	require.Equal(t, "success", deployAndAck(t, h, "dep-1", first).Status)
	before, err := h.db.GetLLMProviderTemplateByHandle(tmplEvtHandle)
	require.NoError(t, err)

	require.Equal(t, "success", deployAndAck(t, h, "dep-2", first.Add(time.Second)).Status)

	assert.Len(t, cp.templateFetches(), 2)
	assert.Len(t, tmplEvtTemplateCreateEvents(h.hub), 1)
	assert.Empty(t, tmplEvtTemplateEvents(h.hub, "UPDATE"), "an unchanged template is not rewritten")
	after, err := h.db.GetLLMProviderTemplateByHandle(tmplEvtHandle)
	require.NoError(t, err)
	assert.Equal(t, before.UpdatedAt, after.UpdatedAt)
}

func TestProviderRedeploy_ControlPlaneDropsTemplate_KeepsHeldCopy(t *testing.T) {
	h, cp, _ := newTemplateEventsHarness(t)
	cp.providerYAML = tmplEvtProviderYAML(tmplEvtHandle)
	cp.templates[tmplEvtHandle] = tmplEvtTemplateYAML(tmplEvtHandle)
	first := time.Now().Truncate(time.Millisecond)
	require.Equal(t, "success", deployAndAck(t, h, "dep-1", first).Status)

	cp.mu.Lock()
	delete(cp.templates, tmplEvtHandle)
	cp.mu.Unlock()
	ack := deployAndAck(t, h, "dep-2", first.Add(time.Second))

	assert.Equal(t, "success", ack.Status, "a refresh that finds nothing does not break the provider (error code %q)", ack.ErrorCode)
	_, err := h.db.GetLLMProviderTemplateByHandle(tmplEvtHandle)
	assert.NoError(t, err, "the gateway keeps the copy it holds")
}

func TestApplyControlPlaneTemplate_NeverReplacesTemplateFromAnotherOrigin(t *testing.T) {
	h, _, _ := newTemplateEventsHarness(t)
	_, err := h.client.llmDeploymentService.CreateLLMProviderTemplate(utils.LLMTemplateParams{
		Spec: []byte(tmplEvtTemplateYAML(tmplEvtHandle)), ContentType: "application/yaml",
		CorrelationID: "corr-hand-applied", Logger: h.client.logger,
	})
	require.NoError(t, err, "precondition: a template applied through the management API")

	stored, result, err := h.client.llmDeploymentService.ApplyControlPlaneLLMProviderTemplate(utils.LLMTemplateParams{
		Spec:          []byte(tmplEvtTemplateYAMLWith(tmplEvtHandle, "$.from.control.plane")),
		ContentType:   "application/yaml",
		CorrelationID: "corr-cp",
		Logger:        h.client.logger,
	})
	require.NoError(t, err)
	assert.Equal(t, utils.ControlPlaneTemplateNotOwned, result)
	assert.False(t, stored.IsFromControlPlane())

	held, err := h.db.GetLLMProviderTemplateByHandle(tmplEvtHandle)
	require.NoError(t, err)
	assert.Equal(t, "$.meta.tokens.in", held.Configuration.Spec.PromptTokens.Identifier, "the hand-applied template is untouched")
	assert.Empty(t, tmplEvtTemplateEvents(h.hub, "UPDATE"))
}

func TestProviderSync_SharedTemplateFetchedOncePerBatch(t *testing.T) {
	h, cp, _ := newTemplateEventsHarness(t)
	cp.templates[tmplEvtHandle] = tmplEvtTemplateYAML(tmplEvtHandle)
	secondProviderID := "66666666-7777-8888-9999-000000000000"
	cp.batch = tmplEvtBatch(t,
		tmplEvtBatchEntry{"dep-a", "llm-provider-" + tmplEvtProviderID + ".yaml", tmplEvtProviderYAMLNamed("acme-prod", tmplEvtHandle)},
		tmplEvtBatchEntry{"dep-b", "llm-provider-" + secondProviderID + ".yaml", tmplEvtProviderYAMLNamed("acme-dev", tmplEvtHandle)},
	)
	deployedAt := time.Now().Truncate(time.Millisecond)

	h.client.processSyncFetchBatch([]models.ControlPlaneDeployment{
		{ArtifactID: tmplEvtProviderID, DeploymentID: "dep-a", Kind: string(models.KindLlmProvider), State: "DEPLOYED", DeployedAt: deployedAt},
		{ArtifactID: secondProviderID, DeploymentID: "dep-b", Kind: string(models.KindLlmProvider), State: "DEPLOYED", DeployedAt: deployedAt},
	}, agentEvtGatewayID)

	assert.Equal(t, []string{tmplEvtHandle}, cp.templateFetches(), "two providers on one template cost one fetch")
	for _, id := range []string{tmplEvtProviderID, secondProviderID} {
		_, err := h.db.GetConfig(id)
		assert.NoError(t, err, "provider %s deployed", id)
	}
}

func TestProviderDeploy_TemplateSecretRefsSyncedBeforeApply(t *testing.T) {
	h, cp, _ := newTemplateEventsHarness(t)
	syncer := newMockSecretSyncer()
	h.client.secretSyncer = syncer
	cp.providerYAML = tmplEvtProviderYAML(tmplEvtHandle)
	cp.templates[tmplEvtHandle] = `apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProviderTemplate
metadata:
  name: ` + tmplEvtHandle + `
spec:
  displayName: '{{ secret "acme-display-name" }}'
  promptTokens:
    location: payload
    identifier: $.meta.tokens.in
`
	cp.secrets["acme-display-name"] = "Acme Inference"

	h.client.handleLLMProviderDeployedEvent(tmplEvtDeployedEvent(t))

	ack := h.nextAck(t)
	assert.Equal(t, "success", ack.Status, "error code %q", ack.ErrorCode)
	assert.Equal(t, "Acme Inference", syncer.upserted["acme-display-name"],
		"a secret the fetched template references is synced before the template is rendered")
	_, err := h.db.GetLLMProviderTemplateByHandle(tmplEvtHandle)
	assert.NoError(t, err)
}

func TestProviderDeploy_TemplateFetchFails_NoSecretSync(t *testing.T) {
	h, cp, _ := newTemplateEventsHarness(t)
	syncer := newMockSecretSyncer()
	h.client.secretSyncer = syncer
	cp.providerYAML = tmplEvtProviderYAML(tmplEvtHandle) // no template served: 404
	cp.secrets["acme-display-name"] = "Acme Inference"

	h.client.handleLLMProviderDeployedEvent(tmplEvtDeployedEvent(t))

	assert.Equal(t, "failed", h.nextAck(t).Status, "a provider whose template is missing fails as before")
	assert.Empty(t, syncer.upserted, "a failed fetch syncs nothing")
}

// templateStorageFault wraps the harness store with template reads and saves
// that can be made to fail.
type templateStorageFault struct {
	storage.Storage
	getErr, saveErr error
}

func (s *templateStorageFault) GetLLMProviderTemplateByHandle(handle string) (*models.StoredLLMProviderTemplate, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.Storage.GetLLMProviderTemplateByHandle(handle)
}

func (s *templateStorageFault) SaveLLMProviderTemplate(t *models.StoredLLMProviderTemplate) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	return s.Storage.SaveLLMProviderTemplate(t)
}

// withTemplateStorageFault rebuilds the harness's LLM deployment service on a
// store whose template reads and saves fail as configured.
func withTemplateStorageFault(h *agentEventsHarness, fault *templateStorageFault) {
	fault.Storage = h.db
	h.client.llmDeploymentService = utils.NewLLMDeploymentService(h.client.store, fault, nil, nil, nil,
		h.client.deploymentService, &config.RouterConfig{ListenerPort: 8080}, nil, nil)
}

func TestEnsureTemplate_WithoutServices_DoesNothing(t *testing.T) {
	c := &Client{logger: newAgentEventsHarness(t).client.logger}
	assert.NotPanics(t, func() {
		c.ensureLLMProviderTemplateFromYAML([]byte(tmplEvtProviderYAML(tmplEvtHandle)), "corr", nil)
	})
}

func TestEnsureTemplate_LookupFailure_SkipsFetch(t *testing.T) {
	h, cp, _ := newTemplateEventsHarness(t)
	cp.templates[tmplEvtHandle] = tmplEvtTemplateYAML(tmplEvtHandle)
	withTemplateStorageFault(h, &templateStorageFault{getErr: errors.New("database locked")})

	h.client.ensureLLMProviderTemplateFromYAML([]byte(tmplEvtProviderYAML(tmplEvtHandle)), "corr", nil)

	assert.Empty(t, cp.templateFetches(), "when the gateway cannot tell what it holds, it does not fetch")
}

func TestEnsureTemplate_ConcurrentSave_TreatedAsDone(t *testing.T) {
	h, cp, _ := newTemplateEventsHarness(t)
	cp.templates[tmplEvtHandle] = tmplEvtTemplateYAML(tmplEvtHandle)
	withTemplateStorageFault(h, &templateStorageFault{saveErr: storage.ErrConflict})

	assert.NotPanics(t, func() {
		h.client.ensureLLMProviderTemplateFromYAML([]byte(tmplEvtProviderYAML(tmplEvtHandle)), "corr", nil)
	})
	assert.Equal(t, []string{tmplEvtHandle}, cp.templateFetches())
	assert.Empty(t, tmplEvtTemplateCreateEvents(h.hub), "another deploy saved it; nothing more to announce")
}

func TestEnsureTemplate_InvalidTemplate_NotSaved(t *testing.T) {
	h, cp, _ := newTemplateEventsHarness(t)
	cp.templates[tmplEvtHandle] = `apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProviderTemplate
metadata:
  name: ` + tmplEvtHandle + `
spec:
  displayName: Acme Inference
  promptTokens: not-an-identifier
`

	h.client.ensureLLMProviderTemplateFromYAML([]byte(tmplEvtProviderYAML(tmplEvtHandle)), "corr", nil)

	assert.Equal(t, []string{tmplEvtHandle}, cp.templateFetches())
	_, err := h.db.GetLLMProviderTemplateByHandle(tmplEvtHandle)
	assert.True(t, storage.IsNotFoundError(err), "a template that fails validation is not saved")
}
