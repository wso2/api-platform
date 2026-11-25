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

package restapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
	"github.com/wso2/api-platform/tests/framework/testbench/services/specs"
)

func redeploymentStub(t *testing.T, probe func(call int) (answer bool, status int), deployedAfter int) (*Steps, context.Context, string) {
	t.Helper()
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var mu sync.Mutex
	calls := 0
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Connection", "close")
		if strings.Contains(r.URL.Path, "/data/probe") {
			mu.Lock()
			calls++
			call := calls
			mu.Unlock()
			answer, status := probe(call)
			if !answer {
				conn, _, err := w.(http.Hijacker).Hijack()
				require.NoError(t, err)
				require.NoError(t, conn.Close())
				return
			}
			w.WriteHeader(status)
			return
		}
		mu.Lock()
		state := "DEPLOYING"
		if calls >= deployedAfter {
			state = "DEPLOYED"
		}
		mu.Unlock()
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"list": []map[string]string{{"deploymentId": "dep-1", "status": state}}})
	})
	steps.DataPlaneURLResolver = func(_ context.Context, target string) (string, error) { return target, nil }
	local.Set(cloudbase.KeyAPIID, "syn-api-1")
	local.Set(cloudbase.KeyDeploymentID, "dep-1")
	local.Set(cloudbase.KeyGatewayID, "gw-1")
	target, err := steps.APIURL("/data/probe")
	require.NoError(t, err)
	return steps, ctx, target
}

func TestKeepServingUntilDeployedToleratesOneDroppedConnection(t *testing.T) {
	steps, ctx, target := redeploymentStub(t, func(call int) (bool, int) { return call != 1, http.StatusOK }, 1)
	require.NoError(t, steps.KeepServingUntilDeployed(ctx, "GET", target, http.StatusOK))
}

func TestKeepServingUntilDeployedFailsOnTwoDroppedConnectionsInARow(t *testing.T) {
	steps, ctx, target := redeploymentStub(t, func(int) (bool, int) { return false, 0 }, 1<<30)
	err := steps.KeepServingUntilDeployed(ctx, "GET", target, http.StatusOK)
	require.ErrorContains(t, err, "traffic was interrupted")
	require.ErrorContains(t, err, "2 requests in a row got no answer")
}

func TestKeepServingUntilDeployedCountsOnlyConsecutiveDrops(t *testing.T) {
	steps, ctx, target := redeploymentStub(t, func(call int) (bool, int) { return call%2 == 0, http.StatusOK }, 4)
	require.NoError(t, steps.KeepServingUntilDeployed(ctx, "GET", target, http.StatusOK))
}

func TestKeepServingUntilDeployedFailsOnAnotherStatus(t *testing.T) {
	steps, ctx, target := redeploymentStub(t, func(int) (bool, int) { return true, http.StatusServiceUnavailable }, 1)
	err := steps.KeepServingUntilDeployed(ctx, "GET", target, http.StatusOK)
	require.ErrorContains(t, err, "expected 200")
	require.ErrorContains(t, err, "503")
}

func TestDocumentFormUploadsFixturesAndGeneratedDocuments(t *testing.T) {
	body, contentType, err := documentForm([][2]string{
		{"id", "api-1"}, {"file", specFieldPrefix + "petstore-v1.yaml"}, {"large", generatedSpecFieldPrefix + "2048"},
	})
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	fields, files, err := cloudbase.FormOf(request)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"id": "api-1"}, fields)
	fixture, err := specs.Fixture("petstore-v1.yaml")
	require.NoError(t, err)
	require.Equal(t, fixture, files["file"])
	require.Len(t, files["large"], 2048)

	for _, value := range []string{generatedSpecFieldPrefix + "0", generatedSpecFieldPrefix + "x", specFieldPrefix + "missing.yaml"} {
		_, _, err := documentForm([][2]string{{"file", value}})
		require.Error(t, err, value)
	}
}

// importAnswerStub answers an import with the API the form describes, with server fields added,
// under answeredID when it is set. It records the form and what cleanup held when it was asked.
type importAnswerStub struct {
	t          *testing.T
	registry   *cleanup.Registry
	status     int
	answeredID string
	fields     map[string]string
	files      map[string][]byte
	registered []string
}

func (s *importAnswerStub) handler(w http.ResponseWriter, r *http.Request) {
	assert.Equal(s.t, http.MethodPost, r.Method)
	assert.Equal(s.t, importOpenAPIPath, r.URL.Path)
	s.registered = cloudbase.PendingOf(s.registry, cleanup.KindAPI)
	fields, files, ok := cloudbase.HandlerForm(s.t, w, r)
	if !ok {
		return
	}
	s.fields, s.files = fields, files
	id := s.fields["id"]
	if s.answeredID != "" {
		id = s.answeredID
	}
	var upstream any
	_ = json.Unmarshal([]byte(s.fields["upstream"]), &upstream)
	cloudbase.WriteJSON(w, s.status, map[string]any{
		"id": id, "displayName": s.fields["displayName"], "context": s.fields["context"],
		"version": s.fields["version"], "projectId": s.fields["projectId"], "upstream": upstream,
		"operations":      []map[string]any{{"name": "listPets", "request": map[string]string{"method": "GET", "path": "/pets"}}},
		"lifeCycleStatus": "CREATED", "createdAt": "2026-11-18T00:00:00Z",
	})
}

func importSteps(t *testing.T) (*Steps, context.Context, *tcontext.Local, *importAnswerStub) {
	t.Helper()
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	t.Setenv(cloudbase.BackendEnv, "")
	stub := &importAnswerStub{t: t, status: http.StatusCreated}
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) { stub.handler(w, r) })
	stub.registry = registry
	local.Set(cloudbase.KeyProjectID, "default")
	return steps, ctx, local, stub
}

func TestImportAPIRegistersBeforeTheRequestAndKeepsTheDefinition(t *testing.T) {
	steps, ctx, _, stub := importSteps(t)

	require.NoError(t, steps.importAPI(ctx, "petstore-v1.yaml"))
	id := cloudbase.OptionalValue(ctx, cloudbase.KeyAPIID)
	require.Equal(t, []string{id}, stub.registered, "the API is registered before the import is sent")
	require.Equal(t, []string{id}, cloudbase.PendingOf(stub.registry, cleanup.KindAPI))
	fixture, err := specs.Fixture("petstore-v1.yaml")
	require.NoError(t, err)
	require.Equal(t, fixture, stub.files["file"])
	require.Equal(t, "/"+id+"/v1", stub.fields["context"])
	require.Equal(t, "default", stub.fields["projectId"])
	require.JSONEq(t, `{"main":{"url":"`+cloudbase.DefaultBackend+`"}}`, stub.fields["upstream"])

	definition, err := steps.StoredDefinition(ctx, cloudbase.PrimaryAPI)
	require.NoError(t, err)
	require.Equal(t, id, definition["id"])
	require.Contains(t, definition, "operations")
	require.NotContains(t, definition, "lifeCycleStatus", "server fields are not part of the definition")
	require.NotContains(t, definition, "createdAt")
}

func TestImportAPIKeepsOnlyWhatThePlatformCreated(t *testing.T) {
	steps, ctx, _, stub := importSteps(t)

	stub.status = http.StatusBadRequest
	require.Error(t, steps.importAPI(ctx, "petstore-v1.yaml"))
	refused := cloudbase.OptionalValue(ctx, cloudbase.KeyAPIID)
	require.Equal(t, []string{refused}, cloudbase.PendingOf(stub.registry, cleanup.KindAPI),
		"a refused import stays registered, since a refusal can follow a stored API")

	stub.status = http.StatusInternalServerError
	require.Error(t, steps.importAPI(ctx, "petstore-v1.yaml"))
	require.Len(t, cloudbase.PendingOf(stub.registry, cleanup.KindAPI), 2)

	stub.status, stub.answeredID = http.StatusCreated, "api-renamed"
	require.NoError(t, steps.importAPI(ctx, "petstore-v1.yaml"))
	require.Equal(t, "api-renamed", cloudbase.OptionalValue(ctx, cloudbase.KeyAPIID))
	require.Contains(t, cloudbase.PendingOf(stub.registry, cleanup.KindAPI), "api-renamed")
	require.Len(t, cloudbase.PendingOf(stub.registry, cleanup.KindAPI), 3, "the id the request named is dropped")

	require.Error(t, steps.importAPI(ctx, "missing.yaml"))
}

func TestImportNewVersionAndReimportReuseTheFirstAPI(t *testing.T) {
	steps, ctx, local, stub := importSteps(t)
	require.NoError(t, steps.importAPI(ctx, "petstore-v1.yaml"))
	first := cloudbase.OptionalValue(ctx, cloudbase.KeyAPIID)
	name := stub.fields["displayName"]

	require.NoError(t, steps.importNewAPIVersion(ctx, "2.0.0", "inventory.yaml"))
	version := cloudbase.OptionalValue(ctx, cloudbase.KeyVersionAPIID)
	require.NotEqual(t, first, version)
	require.Equal(t, name, stub.fields["displayName"], "a new version keeps the display name")
	require.Equal(t, "2.0.0", stub.fields["version"])
	require.Equal(t, "/"+version+"/2.0.0", stub.fields["context"])

	require.NoError(t, steps.reimportAPI(ctx, "inventory.yaml"))
	require.Equal(t, first, stub.fields["id"])
	require.Equal(t, "/"+first+"/v1", stub.fields["context"])
	inventory, err := specs.Fixture("inventory.yaml")
	require.NoError(t, err)
	require.Equal(t, inventory, stub.files["file"])

	local.Set(cloudbase.KeyAPIDefinition, `{"id":"api-1"}`)
	require.ErrorContains(t, steps.reimportAPI(ctx, "inventory.yaml"), "to import again")
}

func TestSubmitAPIImportLeavesTheAnswerAndKeepsOnlyWhatWasCreated(t *testing.T) {
	steps, ctx, local, stub := importSteps(t)
	local.Set("specURL", "https://specs.example.test/petstore.yaml")

	require.ErrorContains(t, steps.submitAPIImport(ctx, cloudbase.ValuesTable(t, []string{"operations", "[]"})), "unsupported import value")
	require.Nil(t, stub.fields, "an unsupported value sends nothing")

	stub.status = http.StatusBadRequest
	require.NoError(t, steps.submitAPIImport(ctx, cloudbase.ValuesTable(t,
		[]string{"file", specFieldPrefix + "petstore-v1.yaml"}, []string{"url", "${CTX:specURL}"})))
	require.Equal(t, "https://specs.example.test/petstore.yaml", stub.fields["url"])
	require.Contains(t, stub.files, "file")
	refused := cloudbase.OptionalValue(ctx, cloudbase.KeyRejectedAPIID)
	require.Equal(t, []string{refused}, stub.registered, "registered before the request")
	require.Equal(t, []string{refused}, cloudbase.PendingOf(stub.registry, cleanup.KindAPI), "a refusal can follow a stored API")
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, published.StatusCode)

	stub.status = http.StatusCreated
	require.NoError(t, steps.submitAPIImport(ctx, cloudbase.ValuesTable(t, []string{"description", "Neither a file nor a URL"})))
	require.NotContains(t, stub.fields, "url")
	require.Empty(t, stub.files)
	require.ElementsMatch(t, []string{refused, cloudbase.OptionalValue(ctx, cloudbase.KeyRejectedAPIID)}, cloudbase.PendingOf(stub.registry, cleanup.KindAPI))

	stub.answeredID = "api-renamed"
	require.NoError(t, steps.submitAPIImport(ctx, cloudbase.ValuesTable(t, []string{"file", specFieldPrefix + "petstore-v1.yaml"})))
	require.Equal(t, "api-renamed", cloudbase.OptionalValue(ctx, cloudbase.KeyRejectedAPIID), "the API is followed under the id it was created with")
	require.Contains(t, cloudbase.PendingOf(stub.registry, cleanup.KindAPI), "api-renamed")
	require.NotContains(t, cloudbase.PendingOf(stub.registry, cleanup.KindAPI), stub.fields["id"])
	published, err = httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, published.StatusCode, "the import's answer stays published")
}

func TestValidateSpecFixtureAndItsAssertions(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	invalid, err := specs.Fixture("invalid.yaml")
	require.NoError(t, err)
	var uploaded []byte
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, validateOpenAPIPath, r.URL.Path)
		_, files, ok := cloudbase.HandlerForm(t, w, r)
		if !ok {
			return
		}
		uploaded = files["file"]
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"isValid": false, "errors": []map[string]string{{"message": "m"}}, "content": string(invalid)})
	})

	require.NoError(t, steps.validateSpecFixture(ctx, "invalid.yaml"))
	require.Equal(t, invalid, uploaded)
	require.NoError(t, steps.assertValidation(ctx, "list the errors and return the definition"))
	require.Error(t, steps.assertValidation(ctx, "find no errors"))

	for name, body := range map[string]string{
		"other content": `{"isValid":false,"errors":[{"message":"m"}],"content":"other"}`,
		"no content":    `{"isValid":false,"errors":[{"message":"m"}]}`,
		"no errors":     `{"isValid":false,"errors":[],"content":` + strconv.Quote(string(invalid)) + `}`,
		"empty message": `{"isValid":false,"errors":[{"message":" "}],"content":` + strconv.Quote(string(invalid)) + `}`,
		"valid":         `{"isValid":true,"errors":[]}`,
		"not a result":  `{"errors":[]}`,
		"not JSON":      `nope`,
	} {
		cloudbase.PublishedResponse(t, steps.Steps, ctx, http.StatusOK, body)
		require.Error(t, steps.assertValidation(ctx, "list the errors and return the definition"), name)
	}
	cloudbase.PublishedResponse(t, steps.Steps, ctx, http.StatusOK, `{"isValid":true,"errors":[]}`)
	require.NoError(t, steps.assertValidation(ctx, "find no errors"))
	cloudbase.PublishedResponse(t, steps.Steps, ctx, http.StatusOK, `{"isValid":true,"errors":[{"message":"m"}]}`)
	require.Error(t, steps.assertValidation(ctx, "find no errors"))
	cloudbase.PublishedResponse(t, steps.Steps, ctx, http.StatusBadRequest, `{"isValid":true,"errors":[]}`)
	require.Error(t, steps.assertValidation(ctx, "find no errors"), "only a 200 is a validation result")
}

func TestReplaceDefinitionUploadsTheFixtureAndKeepsTheNewDefinition(t *testing.T) {
	steps, ctx, _, stub := restAPISteps(t)
	stub.Operations = cloudbase.OperationsOf([3]any{"listAnimals", "GET", "/animals"})

	require.NoError(t, steps.replaceDefinition(ctx, "petstore-renamed.yaml"))
	renamed, err := specs.Fixture("petstore-renamed.yaml")
	require.NoError(t, err)
	require.Equal(t, renamed, stub.Uploaded)
	definition, err := steps.StoredDefinition(ctx, cloudbase.PrimaryAPI)
	require.NoError(t, err)
	require.Equal(t, stub.Operations, definition["operations"], "the definition is read back after the upload")
	require.NotContains(t, definition, "createdAt")

	stub.PutStatus = http.StatusBadRequest
	require.Error(t, steps.replaceDefinition(ctx, "petstore-v2.yaml"))
}

func TestAddOperationPolicyGivesOnlyTheNamedOperationTheHeaderPolicy(t *testing.T) {
	steps, ctx, _, stub := restAPISteps(t)
	require.NoError(t, steps.refreshDefinition(ctx, cloudbase.PrimaryAPI))

	require.NoError(t, steps.addOperationPolicy(ctx, "listPets"))
	operations := stub.PutBody["operations"].([]any)
	listPets := operations[0].(map[string]any)["request"].(map[string]any)
	require.Len(t, listPets["policies"], 1)
	policy := listPets["policies"].([]any)[0].(map[string]any)
	require.Equal(t, headerPolicyName, policy["name"])
	require.Contains(t, mustJSON(t, policy), operationProbeHeader)
	require.NotContains(t, operations[1].(map[string]any)["request"], "policies")
	definition, err := steps.StoredDefinition(ctx, cloudbase.PrimaryAPI)
	require.NoError(t, err)
	require.Contains(t, mustJSON(t, definition), operationProbeHeader, "the stored definition carries the policy")

	require.ErrorContains(t, steps.addOperationPolicy(ctx, "deletePet"), "no operation named")
	stub.PutStatus = http.StatusBadRequest
	require.Error(t, steps.addOperationPolicy(ctx, "createPet"))
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
}

func TestFixtureOperationsListEveryOperationByNameMethodAndPath(t *testing.T) {
	operations, err := fixtureOperations("petstore-v1.yaml")
	require.NoError(t, err)
	require.Equal(t, []string{
		"createPet POST /pets", "getInventory GET /store/inventory", "getPet GET /pets/{petId}", "listPets GET /pets",
	}, operations)
	_, err = fixtureOperations("missing.yaml")
	require.Error(t, err)
	operations, err = fixtureOperations("empty-paths.yaml")
	require.NoError(t, err)
	require.Empty(t, operations, "a document with no paths defines no operations")
	_, err = fixtureOperations("not-a-spec.txt")
	require.Error(t, err, "a document that is not YAML defines nothing")
}

func TestAssertOperationsOfFixtureNeedsExactlyTheFixturesOperations(t *testing.T) {
	steps, ctx, _, stub := restAPISteps(t)
	stub.API["operations"] = cloudbase.OperationsOf(
		[3]any{"listPets", "get", "/pets"}, [3]any{"createPet", "POST", "/pets"},
		[3]any{"getPet", "GET", "/pets/{petId}"}, [3]any{"getInventory", "GET", "/store/inventory"},
	)
	require.NoError(t, steps.assertOperationsOfFixture(ctx, "synthetic REST API", "petstore-v1.yaml"))
	require.Error(t, steps.assertOperationsOfFixture(ctx, "synthetic REST API", "petstore-v2.yaml"))

	stub.API["operations"] = cloudbase.OperationsOf([3]any{"listPets", "GET", "/pets"})
	require.ErrorContains(t, steps.assertOperationsOfFixture(ctx, "synthetic REST API", "petstore-v1.yaml"), "defines")
}

func TestAssertOperationPolicyReadsTheNamedOperation(t *testing.T) {
	steps, ctx, _, stub := restAPISteps(t)
	stub.API["operations"] = []any{
		map[string]any{"name": "listPets", "request": map[string]any{"method": "GET", "path": "/pets", "policies": []any{headerPolicy()}}},
		map[string]any{"name": "listAnimals", "request": map[string]any{"method": "GET", "path": "/animals"}},
	}
	require.NoError(t, steps.assertOperationPolicy(ctx, "listPets", "keep"))
	require.Error(t, steps.assertOperationPolicy(ctx, "listPets", "not have"))
	require.NoError(t, steps.assertOperationPolicy(ctx, "listAnimals", "not have"))
	require.Error(t, steps.assertOperationPolicy(ctx, "listAnimals", "keep"))
	require.ErrorContains(t, steps.assertOperationPolicy(ctx, "getPet", "keep"), "no operation named")
}

func TestStoredDefinitionAndAbsenceAssertions(t *testing.T) {
	steps, ctx, local, stub := restAPISteps(t)
	fixture, err := specs.Fixture("petstore-v1.yaml")
	require.NoError(t, err)

	require.NoError(t, steps.assertNoStoredDefinition(ctx))
	require.Error(t, steps.assertStoredDefinition(ctx, "synthetic REST API", "petstore-v1.yaml"))
	stub.Stored = fixture
	require.NoError(t, steps.assertStoredDefinition(ctx, "synthetic REST API", "petstore-v1.yaml"))
	require.ErrorContains(t, steps.assertStoredDefinition(ctx, "synthetic REST API", "petstore-v2.yaml"), "differs")
	require.Error(t, steps.assertNoStoredDefinition(ctx))

	local.Set(cloudbase.KeyRejectedAPIID, "rejected")
	require.NoError(t, steps.assertSubmittedAPI(ctx, "not exist"))
	require.Error(t, steps.assertSubmittedAPI(ctx, "exist"))
	local.Set(cloudbase.KeyRejectedAPIID, "api-1")
	require.Error(t, steps.assertSubmittedAPI(ctx, "not exist"), "an API that exists is not absent")
	require.NoError(t, steps.assertSubmittedAPI(ctx, "exist"))
}

func TestProbeHeaderAssertionsReadTheEcho(t *testing.T) {
	steps, ctx, _, _ := restAPISteps(t)
	cloudbase.PublishedResponse(t, steps.Steps, ctx, http.StatusOK, `{"path":"/pets","headers":{"X-Apip-Operation-Probe":["kept"]}}`)
	require.NoError(t, steps.assertProbeHeaderEchoed(ctx))
	require.Error(t, steps.assertProbeHeaderAbsent(ctx))

	cloudbase.PublishedResponse(t, steps.Steps, ctx, http.StatusOK, `{"path":"/pets","headers":{"X-Apip-Operation-Probe":["other"]}}`)
	require.Error(t, steps.assertProbeHeaderEchoed(ctx))

	cloudbase.PublishedResponse(t, steps.Steps, ctx, http.StatusOK, `{"path":"/animals","headers":{"Accept":["*/*"]}}`)
	require.NoError(t, steps.assertProbeHeaderAbsent(ctx))
	require.Error(t, steps.assertProbeHeaderEchoed(ctx))

	cloudbase.PublishedResponse(t, steps.Steps, ctx, http.StatusOK, `{"id":1}`)
	require.ErrorContains(t, steps.assertProbeHeaderAbsent(ctx), "not an echo", "an answer that is not an echo proves nothing")
}

func TestReadAPIPublishesTheAPI(t *testing.T) {
	steps, ctx, local, _ := restAPISteps(t)
	require.NoError(t, steps.readAPI(ctx))
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Contains(t, published.Text(), `"context":"/api-1/v1"`)
	local.Set(cloudbase.KeyAPIID, "api-missing")
	require.Error(t, steps.readAPI(ctx))
}

func TestNewVersionAndOperationAssertionsReadThePlatform(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	apis := map[string]map[string]any{
		"api-1": {"id": "api-1", "displayName": "Pets", "version": "1.0.0", "operations": cloudbase.OperationsOf([3]any{"getPostTwo", "GET", "/posts/2"})},
		"api-2": {"id": "api-2", "displayName": "Pets", "version": "2.0.0"},
	}
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if api, ok := apis[strings.TrimPrefix(r.URL.Path, "/rest-apis/")]; ok {
			cloudbase.WriteJSON(w, http.StatusOK, api)
			return
		}
		cloudbase.WriteJSON(w, http.StatusNotFound, map[string]string{"code": "NOT_FOUND"})
	})
	local.Set(cloudbase.KeyAPIID, "api-1")
	local.Set(cloudbase.KeyVersionAPIID, "api-2")
	local.Set(cloudbase.KeyAPIDefinition, `{"id":"api-1","version":"1.0.0"}`)

	require.NoError(t, steps.assertNewVersionOfTheAPI(ctx, "2.0.0"))
	for name, version := range map[string]any{"a missing version": nil, "another version": "1.5.0"} {
		apis["api-1"]["version"] = version
		require.ErrorContains(t, steps.assertNewVersionOfTheAPI(ctx, "2.0.0"), "it was imported with", name)
	}
	apis["api-1"]["version"] = "1.0.0"
	require.ErrorContains(t, steps.assertNewVersionOfTheAPI(ctx, "3.0.0"), "has the version")
	apis["api-2"]["displayName"] = "Other"
	require.ErrorContains(t, steps.assertNewVersionOfTheAPI(ctx, "2.0.0"), "is named")
	apis["api-2"]["displayName"], apis["api-2"]["version"] = "Pets", "1.0.0"
	require.ErrorContains(t, steps.assertNewVersionOfTheAPI(ctx, "1.0.0"), "it was imported with",
		"a new version under the first API's version is not a new version")

	require.NoError(t, steps.assertOperationNamed(ctx, "getPostTwo"))
	require.ErrorContains(t, steps.assertOperationNamed(ctx, "getPostThree"), "no operation named")
	local.Set(cloudbase.KeyAPIID, "api-missing")
	require.Error(t, steps.assertOperationNamed(ctx, "getPostTwo"))
}

func TestReimportStartsAFreshBuildHistory(t *testing.T) {
	steps, ctx, _, _ := importSteps(t)
	require.NoError(t, steps.importAPI(ctx, "petstore-v1.yaml"))
	require.NoError(t, cloudbase.AppendBuild(ctx, cloudbase.PrimaryAPI, "2026-11-18-1"))

	require.NoError(t, steps.reimportAPI(ctx, "inventory.yaml"))
	builds, err := cloudbase.SeenBuilds(ctx, cloudbase.PrimaryAPI)
	require.NoError(t, err)
	require.Empty(t, builds, "the deleted API's builds are not the new API's under the same id")
}

func TestProbeHeaderAbsenceCountsAHeaderWithoutAValue(t *testing.T) {
	steps, ctx, _, _ := restAPISteps(t)
	cloudbase.PublishedResponse(t, steps.Steps, ctx, http.StatusOK, `{"path":"/animals","headers":{"X-Apip-Operation-Probe":null}}`)
	require.ErrorContains(t, steps.assertProbeHeaderAbsent(ctx), "want none")
}

func TestAddOperationPolicyKeepsTheOperationsOtherPolicies(t *testing.T) {
	steps, ctx, _, stub := restAPISteps(t)
	other := map[string]any{"name": "rate-limit", "version": "v1"}
	stub.API["operations"] = []any{
		map[string]any{"name": "listPets", "request": map[string]any{"method": "GET", "path": "/pets", "policies": []any{other}}},
	}
	require.NoError(t, steps.refreshDefinition(ctx, cloudbase.PrimaryAPI))

	require.NoError(t, steps.addOperationPolicy(ctx, "listPets"))
	policies := stub.PutBody["operations"].([]any)[0].(map[string]any)["request"].(map[string]any)["policies"].([]any)
	require.Len(t, policies, 2, "the probe is added beside the policy the operation had")
	require.Equal(t, other, policies[0])
	require.Equal(t, headerPolicyName, policies[1].(map[string]any)["name"])

	stub.API["operations"] = stub.PutBody["operations"]
	require.NoError(t, steps.refreshDefinition(ctx, cloudbase.PrimaryAPI))
	require.NoError(t, steps.addOperationPolicy(ctx, "listPets"))
	require.Len(t, stub.PutBody["operations"].([]any)[0].(map[string]any)["request"].(map[string]any)["policies"], 2,
		"an operation that already has the probe keeps one")
}

func TestBuildStateAfterAReimportIsTheNewAPIs(t *testing.T) {
	steps, ctx, local, _ := importSteps(t)
	require.NoError(t, steps.importAPI(ctx, "petstore-v1.yaml"))
	require.NoError(t, cloudbase.AppendBuild(ctx, cloudbase.PrimaryAPI, "2026-11-18-1"))
	local.Set(cloudbase.KeyBuildMetadata, `{"commit":"abc"}`)
	local.Set(cloudbase.KeyBuildMetadataOf, "2026-11-18-1")

	require.NoError(t, steps.reimportAPI(ctx, "inventory.yaml"))
	for _, key := range []string{cloudbase.KeyBuildIDs, cloudbase.KeyBuildID, cloudbase.KeyBuildMetadata, cloudbase.KeyBuildMetadataOf} {
		require.Empty(t, cloudbase.OptionalValue(ctx, key), "%s is dropped at the reimport", key)
	}
	_, err := cloudbase.LatestPreparedBuild(ctx)
	require.ErrorContains(t, err, "no build has been prepared")
}

// cloudSteps wires the area's steps to a stub cloud API.
func cloudSteps(t *testing.T, handler http.HandlerFunc) (*Steps, context.Context, *tcontext.Local, *cleanup.Registry) {
	t.Helper()
	base, ctx, local, registry := cloudbase.CloudSteps(t, handler)
	return &Steps{Steps: base}, ctx, local, registry
}

// restAPISteps wires the area's steps to a stub REST API.
func restAPISteps(t *testing.T) (*Steps, context.Context, *tcontext.Local, *cloudbase.RestAPIStub) {
	t.Helper()
	base, ctx, local, stub := cloudbase.RestAPISteps(t)
	return &Steps{Steps: base}, ctx, local, stub
}
