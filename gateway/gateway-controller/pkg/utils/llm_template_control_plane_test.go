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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// templateFaultDB is the in-memory test DB with template reads and writes that
// can be made to fail.
type templateFaultDB struct {
	*testMockDB
	getErr, saveErr, updateErr error
}

func (d *templateFaultDB) GetLLMProviderTemplateByHandle(handle string) (*models.StoredLLMProviderTemplate, error) {
	if d.getErr != nil {
		return nil, d.getErr
	}
	return d.testMockDB.GetLLMProviderTemplateByHandle(handle)
}

func (d *templateFaultDB) SaveLLMProviderTemplate(t *models.StoredLLMProviderTemplate) error {
	if d.saveErr != nil {
		return d.saveErr
	}
	return d.testMockDB.SaveLLMProviderTemplate(t)
}

func (d *templateFaultDB) UpdateLLMProviderTemplate(t *models.StoredLLMProviderTemplate) error {
	if d.updateErr != nil {
		return d.updateErr
	}
	return d.testMockDB.UpdateLLMProviderTemplate(t)
}

func newControlPlaneTemplateService(t *testing.T) (*LLMDeploymentService, *templateFaultDB, *mockLLMEventHub) {
	t.Helper()
	db := &templateFaultDB{testMockDB: newTestMockDB()}
	hub := &mockLLMEventHub{}
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiService := newTestAPIDeploymentServiceWithHub(store, db, nil, nil, routerConfig, hub, testGatewayID)
	return NewLLMDeploymentService(store, db, nil, nil, nil, apiService, routerConfig, nil, nil), db, hub
}

func cpTemplate(promptIdentifier string, extraSpec string) LLMTemplateParams {
	return LLMTemplateParams{
		Spec: []byte(`apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProviderTemplate
metadata:
  name: acme-llm
  annotations:
    example.com/team: ai
spec:
  displayName: Acme Inference
  promptTokens:
    location: payload
    identifier: ` + promptIdentifier + `
` + extraSpec),
		ContentType:   "application/yaml",
		CorrelationID: "corr-template",
	}
}

func templateEvents(hub *mockLLMEventHub, action string) []eventhub.Event {
	var events []eventhub.Event
	for _, e := range hub.publishedEvents {
		if e.event.EventType == eventhub.EventTypeLLMTemplate && e.event.Action == action {
			events = append(events, e.event)
		}
	}
	return events
}

func TestApplyControlPlaneTemplate_CreatesAndMarks(t *testing.T) {
	svc, db, hub := newControlPlaneTemplateService(t)

	stored, result, err := svc.ApplyControlPlaneLLMProviderTemplate(cpTemplate("$.usage.in", ""))

	require.NoError(t, err)
	assert.Equal(t, ControlPlaneTemplateCreated, result)
	assert.True(t, stored.IsFromControlPlane(), "the gateway marks a template it saves from the control plane")
	assert.Equal(t, "ai", (*stored.Configuration.Metadata.Annotations)["example.com/team"], "other annotations are kept")
	held, err := db.GetLLMProviderTemplateByHandle("acme-llm")
	require.NoError(t, err)
	assert.Equal(t, stored.UUID, held.UUID)
	assert.Len(t, templateEvents(hub, "CREATE"), 1)
}

func TestApplyControlPlaneTemplate_OverridesAnOriginTheControlPlaneSends(t *testing.T) {
	svc, _, _ := newControlPlaneTemplateService(t)
	params := cpTemplate("$.usage.in", "")
	params.Spec = []byte(`apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProviderTemplate
metadata:
  name: acme-llm
  annotations:
    gateway.api-platform.wso2.com/origin: gateway_api
spec:
  displayName: Acme Inference
`)

	stored, _, err := svc.ApplyControlPlaneLLMProviderTemplate(params)

	require.NoError(t, err)
	assert.True(t, stored.IsFromControlPlane(), "the origin mark is the gateway's own, never taken from the payload")
}

func TestApplyControlPlaneTemplate_UnchangedWritesNothing(t *testing.T) {
	svc, _, hub := newControlPlaneTemplateService(t)
	first, _, err := svc.ApplyControlPlaneLLMProviderTemplate(cpTemplate("$.usage.in", ""))
	require.NoError(t, err)

	again, result, err := svc.ApplyControlPlaneLLMProviderTemplate(cpTemplate("$.usage.in", ""))

	require.NoError(t, err)
	assert.Equal(t, ControlPlaneTemplateUnchanged, result)
	assert.Equal(t, first.UUID, again.UUID)
	assert.Empty(t, templateEvents(hub, "UPDATE"))
}

func TestApplyControlPlaneTemplate_UpdatesAnOlderControlPlaneCopy(t *testing.T) {
	svc, db, hub := newControlPlaneTemplateService(t)
	first, _, err := svc.ApplyControlPlaneLLMProviderTemplate(cpTemplate("$.usage.in", ""))
	require.NoError(t, err)

	updated, result, err := svc.ApplyControlPlaneLLMProviderTemplate(cpTemplate("$.usage.input_tokens", ""))

	require.NoError(t, err)
	assert.Equal(t, ControlPlaneTemplateUpdated, result)
	assert.Equal(t, first.UUID, updated.UUID, "an update keeps the template's identity")
	assert.Equal(t, first.CreatedAt, updated.CreatedAt)
	assert.True(t, updated.IsFromControlPlane())
	held, err := db.GetLLMProviderTemplateByHandle("acme-llm")
	require.NoError(t, err)
	assert.Equal(t, "$.usage.input_tokens", held.Configuration.Spec.PromptTokens.Identifier)
	assert.Len(t, templateEvents(hub, "UPDATE"), 1)
}

func TestApplyControlPlaneTemplate_LeavesATemplateFromAnotherOrigin(t *testing.T) {
	svc, db, hub := newControlPlaneTemplateService(t)
	_, err := svc.CreateLLMProviderTemplate(cpTemplate("$.hand.applied", ""))
	require.NoError(t, err, "precondition: a template applied through the management API")

	stored, result, err := svc.ApplyControlPlaneLLMProviderTemplate(cpTemplate("$.from.control.plane", ""))

	require.NoError(t, err)
	assert.Equal(t, ControlPlaneTemplateNotOwned, result)
	assert.False(t, stored.IsFromControlPlane())
	held, err := db.GetLLMProviderTemplateByHandle("acme-llm")
	require.NoError(t, err)
	assert.Equal(t, "$.hand.applied", held.Configuration.Spec.PromptTokens.Identifier)
	assert.Empty(t, templateEvents(hub, "UPDATE"))
}

func TestApplyControlPlaneTemplate_RejectsAnInvalidTemplate(t *testing.T) {
	svc, db, _ := newControlPlaneTemplateService(t)
	params := cpTemplate("$.usage.in", "")
	params.Spec = []byte("kind: [unterminated")

	_, _, err := svc.ApplyControlPlaneLLMProviderTemplate(params)

	assert.ErrorIs(t, err, ErrLLMTemplateValidation)
	assert.Empty(t, db.templates, "nothing is saved")
}

func TestApplyControlPlaneTemplate_LookupFailure(t *testing.T) {
	svc, db, _ := newControlPlaneTemplateService(t)
	db.getErr = errors.New("database locked")

	_, _, err := svc.ApplyControlPlaneLLMProviderTemplate(cpTemplate("$.usage.in", ""))

	require.Error(t, err)
	assert.ErrorIs(t, err, db.getErr)
	assert.Empty(t, db.templates)
}

func TestApplyControlPlaneTemplate_SaveFailures(t *testing.T) {
	cases := []struct {
		name         string
		saveErr      error
		wantConflict bool
	}{
		{"conflict", storage.ErrConflict, true},
		{"already exists", errors.New("UNIQUE constraint: template already exists"), true},
		{"other", errors.New("disk full"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, db, hub := newControlPlaneTemplateService(t)
			db.saveErr = tc.saveErr

			_, _, err := svc.ApplyControlPlaneLLMProviderTemplate(cpTemplate("$.usage.in", ""))

			require.Error(t, err)
			assert.Equal(t, tc.wantConflict, storage.IsConflictError(err))
			assert.Empty(t, templateEvents(hub, "CREATE"), "nothing is announced when the save fails")
		})
	}
}

func TestApplyControlPlaneTemplate_UpdateFailure(t *testing.T) {
	svc, db, hub := newControlPlaneTemplateService(t)
	_, _, err := svc.ApplyControlPlaneLLMProviderTemplate(cpTemplate("$.usage.in", ""))
	require.NoError(t, err)
	db.updateErr = errors.New("disk full")

	_, _, err = svc.ApplyControlPlaneLLMProviderTemplate(cpTemplate("$.usage.input_tokens", ""))

	require.Error(t, err)
	assert.ErrorIs(t, err, db.updateErr)
	assert.Empty(t, templateEvents(hub, "UPDATE"))
}

func TestApplyControlPlaneTemplate_GroupIDAndVersionAreImmutable(t *testing.T) {
	cases := []struct {
		name, original, changed string
	}{
		{"groupId", "  groupId: acme\n", "  groupId: other\n"},
		{"version", "  version: v1.0\n", "  version: v2.0\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, db, hub := newControlPlaneTemplateService(t)
			_, _, err := svc.ApplyControlPlaneLLMProviderTemplate(cpTemplate("$.usage.in", tc.original))
			require.NoError(t, err)

			_, _, err = svc.ApplyControlPlaneLLMProviderTemplate(cpTemplate("$.usage.in", tc.changed))

			assert.ErrorIs(t, err, ErrLLMTemplateValidation)
			held, getErr := db.GetLLMProviderTemplateByHandle("acme-llm")
			require.NoError(t, getErr)
			assert.Equal(t, "$.usage.in", held.Configuration.Spec.PromptTokens.Identifier)
			assert.Empty(t, templateEvents(hub, "UPDATE"))
		})
	}
}
