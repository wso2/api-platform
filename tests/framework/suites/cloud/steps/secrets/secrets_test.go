/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package secrets

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

func TestSecretReferenceEscapesTheQuotesForJSON(t *testing.T) {
	reference := secretReference("syn-sec-1")
	require.Equal(t, `{{ secret \"syn-sec-1\" }}`, reference)
	var decoded string
	require.NoError(t, json.Unmarshal([]byte(`"`+reference+`"`), &decoded))
	require.Equal(t, `{{ secret "syn-sec-1" }}`, decoded)
}

func TestSecretFormIsMultipart(t *testing.T) {
	body, contentType, err := secretForm([][2]string{{"id", "syn-sec-1"}, {"value", "sample"}})
	require.NoError(t, err)
	_, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	reader := multipart.NewReader(strings.NewReader(string(body)), params["boundary"])
	var names []string
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		names = append(names, part.FormName())
	}
	require.Equal(t, []string{"id", "value"}, names)
}

func TestCreateSecretDeregistersARefusedSecret(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		cloudbase.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "SERVICE_UNAVAILABLE"})
	})
	require.ErrorContains(t, steps.createSecret(ctx), "503")
	require.Empty(t, registry.Pending())
	require.Empty(t, cloudbase.OptionalValue(ctx, keySecretValue))
}

func TestCreateSecretFollowsAnIDThePlatformChanged(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		cloudbase.WriteJSON(w, http.StatusCreated, map[string]any{"id": "platform-chosen"})
	})
	require.NoError(t, steps.createSecret(ctx))
	require.Equal(t, "platform-chosen", cloudbase.OptionalValue(ctx, keySecretID))
	require.Equal(t, secretReference("platform-chosen"), cloudbase.OptionalValue(ctx, keySecretReference))
	pending := registry.Pending()
	require.Len(t, pending, 1)
	require.Equal(t, "platform-chosen", pending[0].ID)
}

func TestCreateSecretKeepsTheValueInScenarioStateAndRegistersCleanup(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	stub := &secretStub{}
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		stub.record(t, r)
		cloudbase.WriteJSON(w, http.StatusCreated, map[string]any{"id": stub.fields["id"], "displayName": stub.fields["displayName"]})
	})

	require.NoError(t, steps.createSecret(ctx))
	require.Equal(t, http.MethodPost, stub.method)
	require.Equal(t, "/secrets", stub.path)
	handle, err := cloudbase.RequiredValue(ctx, keySecretID)
	require.NoError(t, err)
	require.Equal(t, handle, stub.fields["id"])
	require.LessOrEqual(t, len(handle), 40, "a secret handle is at most 40 characters")
	value, err := cloudbase.RequiredValue(ctx, keySecretValue)
	require.NoError(t, err)
	require.Equal(t, value, stub.fields["value"])
	require.Equal(t, secretReference(handle), cloudbase.OptionalValue(ctx, keySecretReference))
	require.Equal(t, "GENERIC", stub.fields["type"])
	require.Equal(t, []cleanup.Resource{{Kind: KindSecret, ID: handle, Actor: cloudbase.ResourceActor, Description: "secret " + handle}},
		registry.Pending())
}

func TestRotateSecretKeepsThePreviousValue(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	stub := &secretStub{}
	status := http.StatusOK
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		stub.record(t, r)
		cloudbase.WriteJSON(w, status, map[string]any{"id": "syn-sec-1"})
	})
	require.ErrorContains(t, steps.rotateSecret(ctx), keySecretID)
	local.Set(keySecretID, "syn-sec-1")
	local.Set(keySecretValue, "sample-original")

	require.NoError(t, steps.rotateSecret(ctx))
	require.Equal(t, http.MethodPut, stub.method)
	require.Equal(t, "/secrets/syn-sec-1", stub.path)
	require.Equal(t, "sample-original", cloudbase.OptionalValue(ctx, keySecretPreviousValue))
	rotated := cloudbase.OptionalValue(ctx, keySecretValue)
	require.NotEqual(t, "sample-original", rotated)
	require.Equal(t, rotated, stub.fields["value"])

	status = http.StatusNotFound
	require.Error(t, steps.rotateSecret(ctx))
	require.Equal(t, rotated, cloudbase.OptionalValue(ctx, keySecretValue), "a refused rotation keeps the value")
}

func TestReadAndListSecretsRequireSuccess(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var paths []string
	status := http.StatusOK
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		cloudbase.WriteJSON(w, status, map[string]any{"id": "syn-sec-1"})
	})
	require.Error(t, steps.readSecret(ctx), "no secret has been chosen")
	local.Set(keySecretID, "syn-sec-1")
	require.NoError(t, steps.readSecret(ctx))
	require.NoError(t, steps.listSecrets(ctx))
	require.Equal(t, []string{"/secrets/syn-sec-1", "/secrets"}, paths)
	status = http.StatusNotFound
	require.Error(t, steps.readSecret(ctx))
	require.Error(t, steps.listSecrets(ctx))
}

func TestDeleteSecretDeregistersOnlyADeletedSecret(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	status := http.StatusConflict
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/secrets/syn-sec-1", r.URL.Path)
		if status == http.StatusNoContent {
			w.WriteHeader(status)
			return
		}
		cloudbase.WriteJSON(w, status, map[string]string{"code": "SECRET_IN_USE"})
	})
	local.Set(keySecretID, "syn-sec-1")
	require.NoError(t, cleanup.Register(ctx, cleanup.Resource{Kind: KindSecret, ID: "syn-sec-1", Actor: cloudbase.ResourceActor}))

	require.NoError(t, steps.deleteSecret(ctx), "a refusal is for the scenario to assert on")
	require.Len(t, registry.Pending(), 1, "a refused delete leaves the cleanup in place")
	response, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, response.StatusCode)

	status = http.StatusNoContent
	require.NoError(t, steps.deleteSecret(ctx))
	require.Empty(t, registry.Pending())
}

func TestAssertSecretHash(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	local.Set(keySecretValue, "sample-secret-1")

	cloudbase.PublishedResponse(t, steps.Steps, ctx, 200, `{"id":"s","hash":"hmac-sha256:0123"}`)
	require.NoError(t, steps.assertSecretHash(ctx))
	cloudbase.PublishedResponse(t, steps.Steps, ctx, 200, `{"id":"s"}`)
	require.ErrorContains(t, steps.assertSecretHash(ctx), "no hash")
	cloudbase.PublishedResponse(t, steps.Steps, ctx, 200, `{"id":"s","hash":"sample-secret-1"}`)
	require.ErrorContains(t, steps.assertSecretHash(ctx), "reveals its value")
}

func TestAssertSecretValueHidden(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	local.Set(keySecretValue, "sample-secret-1")

	cloudbase.PublishedResponse(t, steps.Steps, ctx, 200, `{"id":"syn-sec-1","hash":"hmac-sha256:abc"}`)
	require.NoError(t, steps.assertSecretValueHidden(ctx))
	cloudbase.PublishedResponse(t, steps.Steps, ctx, 200, `{"list":[{"id":"a"},{"id":"b"}]}`)
	require.NoError(t, steps.assertSecretValueHidden(ctx))

	cloudbase.PublishedResponse(t, steps.Steps, ctx, 200, `{"id":"syn-sec-1","note":"sample-secret-1"}`)
	err := steps.assertSecretValueHidden(ctx)
	require.ErrorContains(t, err, "contains the secret's value")
	require.NotContains(t, err.Error(), "sample-secret-1", "a leak is not repeated into the report")

	cloudbase.PublishedResponse(t, steps.Steps, ctx, 200, `{"list":[{"id":"a","value":"x"}]}`)
	require.ErrorContains(t, steps.assertSecretValueHidden(ctx), `"value" field`)
}

func TestClassifyEcho(t *testing.T) {
	echo := func(status int, value string) *httpx.Response {
		return &httpx.Response{StatusCode: status, Body: []byte(`{"headers":{"X-Probe":"` + value + `"}}`)}
	}
	require.Equal(t, echoCurrent, classifyEcho(echo(200, "new"), "x-probe", "new", "old"))
	require.Equal(t, echoPrevious, classifyEcho(echo(200, "old"), "x-probe", "new", "old"))
	require.Equal(t, echoOther, classifyEcho(echo(200, "else"), "x-probe", "new", "old"))
	require.Equal(t, echoOther, classifyEcho(echo(503, "new"), "x-probe", "new", "old"))
	require.Equal(t, echoOther, classifyEcho(nil, "x-probe", "new", "old"))
	require.Equal(t, echoAbsent, classifyEcho(&httpx.Response{StatusCode: 200, Body: []byte(`{"headers":{}}`)}, "x-probe", "new", "old"))
	require.Equal(t, echoOther, classifyEcho(echo(200, ""), "x-probe", "new", ""), "an empty previous value matches nothing")
}

func TestAssertEchoedSecret(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	local.Set(keySecretValue, "sample-secret-1")
	cloudbase.PublishedResponse(t, steps.Steps, ctx, 200, `{"headers":{"X-Probe":"sample-secret-1"}}`)
	require.NoError(t, steps.assertEchoedSecret(ctx, "x-probe"))
	cloudbase.PublishedResponse(t, steps.Steps, ctx, 200, `{"headers":{}}`)
	require.ErrorContains(t, steps.assertEchoedSecret(ctx, "x-probe"), "no \"x-probe\" header")
	cloudbase.PublishedResponse(t, steps.Steps, ctx, 200, `{"headers":{"X-Probe":"another"}}`)
	err := steps.assertEchoedSecret(ctx, "x-probe")
	require.ErrorContains(t, err, "not the secret's value")
	require.NotContains(t, err.Error(), "another")
}

func TestSendUntilEchoesRotatedWaitsForTheNewValue(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		calls++
		attempt := calls
		mu.Unlock()
		value := "old"
		if attempt > 1 {
			value = "new"
		}
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"headers": map[string]string{"X-Probe": value}})
	}))
	defer server.Close()
	steps, ctx, local := dataPlaneSteps(t, func(http.ResponseWriter, *http.Request) {})
	steps.Funnel = httpx.NewFunnel(httpx.NewClient(httpx.Options{Timeout: 5 * time.Second, InsecureSkipVerify: true}), 0, 0)
	steps.Client = steps.Funnel.Client()
	local.Set(keySecretValue, "new")

	require.ErrorContains(t, steps.sendUntilEchoesRotated(ctx, "GET", server.URL+"/echo", "x-probe"), "not been rotated")
	local.Set(keySecretPreviousValue, "old")
	require.NoError(t, steps.sendUntilEchoesRotated(ctx, "get", server.URL+"/echo", "x-probe"))
	mu.Lock()
	defer mu.Unlock()
	require.GreaterOrEqual(t, calls, 2, "the first answer still carried the old value")
}

// dataPlaneSteps returns steps whose data-plane requests may go to the given local server.
func dataPlaneSteps(t *testing.T, controlPlane http.HandlerFunc) (*Steps, context.Context, *tcontext.Local) {
	t.Helper()
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, controlPlane)
	steps.DataPlaneURLResolver = func(_ context.Context, target string) (string, error) { return target, nil }
	return steps, ctx, local
}

// secretStub records the multipart secrets requests it receives and answers like the platform.
type secretStub struct {
	mu     sync.Mutex
	method string
	path   string
	fields map[string]string
}

func (c *secretStub) record(t *testing.T, r *http.Request) {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	reader := multipart.NewReader(r.Body, params["boundary"])
	fields := map[string]string{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		raw, err := io.ReadAll(part)
		require.NoError(t, err)
		fields[part.FormName()] = string(raw)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.method, c.path, c.fields = r.Method, r.URL.Path, fields
}

// cloudSteps wires the area's steps to a stub cloud API.
func cloudSteps(t *testing.T, handler http.HandlerFunc) (*Steps, context.Context, *tcontext.Local, *cleanup.Registry) {
	t.Helper()
	base, ctx, local, registry := cloudbase.CloudSteps(t, handler)
	return &Steps{Steps: base}, ctx, local, registry
}
