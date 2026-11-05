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

package restapi

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

const (
	syncTestGatewayID  = "test-gateway"
	syncTestArtifactID = "0199a1b2-0000-7000-8000-00000000d001"
	syncTestHandle     = "resumed-agent-api"
)

// restAPIWithArtifactID is a RestApi as the platform renders it for an agent: the
// artifact-id annotation pins the UUID, so a re-applied RestApi gets the same UUID.
var restAPIWithArtifactID = []byte(`
apiVersion: gateway.api-platform.wso2.com/v1
kind: RestApi
metadata:
  name: ` + syncTestHandle + `
  annotations:
    gateway.api-platform.wso2.com/artifact-id: ` + syncTestArtifactID + `
spec:
  displayName: Resumed Agent API
  version: v1.0
  context: /resumed-agent
  upstream:
    main:
      url: https://example.com
  operations:
    - method: GET
      path: /
`)

type discardEventHub struct{}

func (discardEventHub) Initialize() error                               { return nil }
func (discardEventHub) RegisterGateway(string) error                    { return nil }
func (discardEventHub) PublishEvent(string, eventhub.Event) error       { return nil }
func (discardEventHub) Subscribe(string) (<-chan eventhub.Event, error) { return nil, nil }
func (discardEventHub) Unsubscribe(string, <-chan eventhub.Event) error { return nil }
func (discardEventHub) UnsubscribeAll(string) error                     { return nil }
func (discardEventHub) CleanUpEvents() error                            { return nil }
func (discardEventHub) Close() error                                    { return nil }

// keySyncRecorder is a connected cloud control plane client that records the
// artifacts whose API keys the service asks it to pull.
type keySyncRecorder struct {
	synced chan string
}

func (r *keySyncRecorder) IsConnected() bool { return true }
func (r *keySyncRecorder) PushArtifact(string, *models.StoredConfig, string) error {
	return nil
}
func (r *keySyncRecorder) SyncArtifactsToOnPremAPIM(*utils.APIMConfig) error { return nil }
func (r *keySyncRecorder) IsOnPrem() bool                                    { return false }
func (r *keySyncRecorder) GetAPIMConfig() *utils.APIMConfig                  { return nil }
func (r *keySyncRecorder) SubmitAPIMSync(func())                             {}
func (r *keySyncRecorder) SubmitAPIMSyncAndWait(func())                      {}
func (r *keySyncRecorder) SubmitArtifactPush(func())                         {}
func (r *keySyncRecorder) SyncAPIKeysForArtifact(cfg *models.StoredConfig) {
	r.synced <- cfg.UUID
}

func (r *keySyncRecorder) expectSync(t *testing.T, msg string) {
	t.Helper()
	select {
	case id := <-r.synced:
		assert.Equal(t, syncTestArtifactID, id, msg)
	case <-time.After(2 * time.Second):
		t.Fatal(msg + ": no key sync was requested")
	}
}

func (r *keySyncRecorder) expectNoSync(t *testing.T, msg string) {
	t.Helper()
	select {
	case id := <-r.synced:
		t.Fatalf("%s: unexpected key sync for %s", msg, id)
	case <-time.After(200 * time.Millisecond):
	}
}

func newKeySyncTestService(t *testing.T) (*RestAPIService, *keySyncRecorder) {
	t.Helper()
	metrics.Init()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := storage.NewStorage(storage.BackendConfig{
		Type:       "sqlite",
		SQLitePath: filepath.Join(t.TempDir(), "restapi-key-sync.db"),
		GatewayID:  syncTestGatewayID,
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	routerCfg := &config.RouterConfig{GatewayHost: "localhost"}
	systemCfg := &config.Config{
		Controller: config.Controller{Server: config.ServerConfig{GatewayID: syncTestGatewayID}},
	}
	deploymentService := utils.NewAPIDeploymentService(
		nil, db, nil, config.NewAPIValidator(), routerCfg, discardEventHub{}, syncTestGatewayID, nil, nil)
	recorder := &keySyncRecorder{synced: make(chan string, 4)}
	svc := NewRestAPIService(
		nil, db, nil, nil, deploymentService, nil, recorder,
		routerCfg, systemCfg, nil, config.NewParser(), config.NewAPIValidator(),
		logger, discardEventHub{}, nil,
	)
	return svc, recorder
}

func createRestAPI(t *testing.T, svc *RestAPIService) *CreateResult {
	t.Helper()
	res, err := svc.Create(CreateParams{
		Body:          restAPIWithArtifactID,
		ContentType:   "application/yaml",
		CorrelationID: "corr-create",
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	require.NoError(t, err)
	return res
}

// Deleting a RestApi deletes its API keys. When it is applied again with the same
// UUID (an agent that is suspended and resumed), the service must ask the control
// plane for the keys, or every key it still reports as active is rejected. A plain
// update keeps its keys and must not trigger a pull.
func TestCreate_PullsAPIKeysOnlyForNewArtifacts(t *testing.T) {
	svc, recorder := newKeySyncTestService(t)

	first := createRestAPI(t, svc)
	require.False(t, first.IsUpdate)
	assert.Equal(t, syncTestArtifactID, first.StoredConfig.UUID, "the artifact-id annotation pins the UUID")
	recorder.expectSync(t, "first deployment")

	second := createRestAPI(t, svc)
	require.True(t, second.IsUpdate)
	recorder.expectNoSync(t, "an update keeps its keys")

	_, err := svc.Delete(DeleteParams{
		Handle:        syncTestHandle,
		CorrelationID: "corr-delete",
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	require.NoError(t, err)

	recreated := createRestAPI(t, svc)
	require.False(t, recreated.IsUpdate, "a re-applied RestApi is a create, not an update")
	assert.Equal(t, syncTestArtifactID, recreated.StoredConfig.UUID)
	recorder.expectSync(t, "re-created after delete")
}
