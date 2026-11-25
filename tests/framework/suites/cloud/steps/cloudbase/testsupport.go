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

package cloudbase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/components"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// ValuesTable builds a Gherkin values table from rows of cells.
func ValuesTable(t *testing.T, rows ...[]string) *godog.Table {
	t.Helper()
	type cell struct {
		Value string `json:"value"`
	}
	type row struct {
		Cells []cell `json:"cells"`
	}
	payload := struct {
		Rows []row `json:"rows"`
	}{}
	for _, values := range rows {
		cells := make([]cell, len(values))
		for i, value := range values {
			cells[i] = cell{Value: value}
		}
		payload.Rows = append(payload.Rows, row{Cells: cells})
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	var table godog.Table
	require.NoError(t, json.Unmarshal(raw, &table))
	return &table
}

// platformAPIRoute is where the control-plane gateway publishes the APIP platform API.
const platformAPIRoute = "/apip-platform-api-apip-platform-api-endpoint/api/v0.9"

// CloudAPIStub serves handler as the organization's platform API endpoint.
func CloudAPIStub(t *testing.T, handler http.HandlerFunc) *frameworkruntime.Topology {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The handler sees the platform API's routes as plain collection paths.
		r.URL.Path = strings.TrimPrefix(r.URL.Path, platformAPIRoute)
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	inst, err := components.NewExternalInstance(corecatalog.CloudConsole(), 0, 1,
		map[string]string{
			corecatalog.EndpointAPIPPlatformAPI:     server.URL + platformAPIRoute,
			corecatalog.EndpointGatewayControlPlane: "https://cloud-console.example.test/api-management/connect",
		})
	require.NoError(t, err)
	set := components.NewSet()
	require.NoError(t, set.Add(inst))
	return &frameworkruntime.Topology{Instances: set}
}

// CloudSteps returns steps wired to a stub cloud API, and a scenario context that holds a
// cleanup registry so created resources are recorded.
func CloudSteps(t *testing.T, handler http.HandlerFunc) (*Steps, context.Context, *tcontext.Local, *cleanup.Registry) {
	t.Helper()
	topo := CloudAPIStub(t, handler)
	funnel := httpx.NewFunnel(httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), 0, 0)
	steps := &Steps{Topo: topo, Funnel: funnel, Client: funnel.Client()}
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	registry := cleanup.NewRegistry(slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, cleanup.Install(ctx, registry))
	local, ok := tcontext.LocalOf(ctx)
	require.True(t, ok)
	return steps, ctx, local, registry
}

// WriteJSON answers a stub request with a JSON body.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// PublishedResponse publishes a stub response to the scenario, as a step that sent a request would.
func PublishedResponse(t *testing.T, steps *Steps, ctx context.Context, status int, body string) {
	t.Helper()
	require.NoError(t, steps.Funnel.Publish(ctx, &httpx.Response{StatusCode: status, Body: []byte(body), Method: "GET", URL: "https://cloud.example.com/x"}))
}

// ListedGateway returns a managed-gateway list entry for a stub.
func ListedGateway(id, environment string, extra map[string]any) map[string]any {
	gateway := map[string]any{
		"id": id, "isActive": true, "environment": environment,
		"endpoints": []any{"https://" + id + ".example.com/"},
	}
	for key, value := range extra {
		gateway[key] = value
	}
	return gateway
}

// HandleRefusal is the platform's refusal of a handle that is too long for an environment: the
// limit is 37 minus twice the environment name's length, and at least 1.
func HandleRefusal(environment string) string {
	budget := max(37-2*len(environment), 1)
	return fmt.Sprintf(`{"code":"invalid_request","message":"The gateway name is too long for the %q environment: its handle %q must be %d characters or fewer."}`,
		environment, strings.Repeat("a", HandleProbeLength), budget)
}

var stepKeyword = regexp.MustCompile(`^(Given|When|Then|And|But|\*) (.+)$`)

// FeatureSteps returns the text of every step a feature file runs, with each Scenario Outline
// expanded once per Examples row.
func FeatureSteps(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var steps, block []string
	var examples [][]string
	inExamples := false
	flush := func() {
		if len(examples) == 0 {
			steps = append(steps, block...)
		}
		for _, row := range examples[min(1, len(examples)):] {
			for _, step := range block {
				for i, name := range examples[0] {
					step = strings.ReplaceAll(step, "<"+name+">", row[i])
				}
				steps = append(steps, step)
			}
		}
		block, examples, inExamples = nil, nil, false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Background:"), strings.HasPrefix(line, "Scenario:"),
			strings.HasPrefix(line, "Scenario Outline:"):
			flush()
		case strings.HasPrefix(line, "Examples:"):
			inExamples = true
		case strings.HasPrefix(line, "|") && inExamples:
			cells := strings.Split(strings.Trim(line, "|"), "|")
			for i := range cells {
				cells[i] = strings.TrimSpace(cells[i])
			}
			examples = append(examples, cells)
		default:
			if match := stepKeyword.FindStringSubmatch(line); match != nil {
				block = append(block, match[2])
			}
		}
	}
	flush()
	return steps
}

// PromotionChainPaths is a pipeline that leads from the first environment through each next one.
func PromotionChainPaths(environments ...string) []PromotionPath {
	var paths []PromotionPath
	for i := 0; i+1 < len(environments); i++ {
		paths = append(paths, PromotionPath{SourceEnvironment: environments[i], TargetEnvironments: []string{environments[i+1]}})
	}
	return paths
}

// PendingOf lists the ids of the resources of a kind that cleanup still holds.
func PendingOf(registry *cleanup.Registry, kind cleanup.Kind) []string {
	var ids []string
	for _, resource := range registry.Pending() {
		if resource.Kind == kind {
			ids = append(ids, resource.ID)
		}
	}
	return ids
}

// FormOf reads a multipart request into its text fields and its uploaded files.
func FormOf(r *http.Request) (map[string]string, map[string][]byte, error) {
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return nil, nil, err
	}
	reader := multipart.NewReader(r.Body, params["boundary"])
	fields, files := map[string]string{}, map[string][]byte{}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return fields, files, nil
		}
		if err != nil {
			return nil, nil, err
		}
		content, err := io.ReadAll(part)
		if err != nil {
			return nil, nil, err
		}
		if part.FileName() != "" {
			files[part.FormName()] = content
			continue
		}
		fields[part.FormName()] = string(content)
	}
}

// HandlerForm reads a stub handler's multipart request. A request it cannot read is reported
// without stopping the test from the handler's goroutine, and answered with 400; ok is false then.
func HandlerForm(t *testing.T, w http.ResponseWriter, r *http.Request) (map[string]string, map[string][]byte, bool) {
	fields, files, err := FormOf(r)
	if err != nil {
		t.Errorf("reading the multipart request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return nil, nil, false
	}
	return fields, files, true
}

// DeploymentAnswerStub answers every deployment request with one deployment on gateway gw that
// runs the build *answered names, and records each request's path, query and body.
func DeploymentAnswerStub(t *testing.T, answered *string, paths, queries *[]string, bodies *[]map[string]any) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		*paths = append(*paths, r.URL.Path)
		*queries = append(*queries, r.URL.RawQuery)
		var body map[string]any
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		*bodies = append(*bodies, body)
		WriteJSON(w, http.StatusCreated, map[string]any{"list": []map[string]string{
			{"gatewayId": "gw", "deploymentId": "dep-" + strconv.Itoa(len(*paths)), "buildId": *answered},
		}})
	}
}

// RestAPIStub serves one REST API's definition routes from a definition it keeps.
type RestAPIStub struct {
	t          *testing.T
	API        map[string]any
	Stored     []byte
	PutStatus  int
	PutBody    map[string]any
	Uploaded   []byte
	Operations []any
}

func (s *RestAPIStub) handler(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/rest-apis/api-1":
		WriteJSON(w, http.StatusOK, s.API)
	case r.Method == http.MethodPut && r.URL.Path == "/rest-apis/api-1":
		s.PutBody = map[string]any{}
		if !assert.NoError(s.t, json.NewDecoder(r.Body).Decode(&s.PutBody)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		WriteJSON(w, s.PutStatus, s.PutBody)
	case r.Method == http.MethodPut && r.URL.Path == "/rest-apis/api-1/openapi":
		_, files, ok := HandlerForm(s.t, w, r)
		if !ok {
			return
		}
		s.Uploaded = files["file"]
		s.API["operations"] = s.Operations
		WriteJSON(w, s.PutStatus, map[string]string{"content": string(s.Uploaded)})
	case r.Method == http.MethodGet && r.URL.Path == "/rest-apis/api-1/openapi":
		if s.Stored == nil {
			WriteJSON(w, http.StatusNotFound, map[string]string{"code": "NOT_FOUND"})
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"content": string(s.Stored)})
	case r.Method == http.MethodGet && r.URL.Path == "/rest-apis/rejected":
		WriteJSON(w, http.StatusNotFound, map[string]string{"code": "NOT_FOUND"})
	default:
		w.WriteHeader(http.StatusTeapot)
	}
}

// OperationsOf returns the operations of a stub API from entries of operation id, method and path.
func OperationsOf(entries ...[3]any) []any {
	operations := make([]any, 0, len(entries))
	for _, entry := range entries {
		request := map[string]any{"method": entry[1], "path": entry[2]}
		operations = append(operations, map[string]any{"name": entry[0], "request": request})
	}
	return operations
}

// RestAPISteps returns steps wired to a stub REST API, with the scenario's API already created.
func RestAPISteps(t *testing.T) (*Steps, context.Context, *tcontext.Local, *RestAPIStub) {
	t.Helper()
	t.Setenv(CloudConsoleAccessTokenEnv, "console-token")
	stub := &RestAPIStub{t: t, PutStatus: http.StatusOK, API: map[string]any{
		"id": "api-1", "context": "/api-1/v1", "version": "1.0.0", "createdAt": "x",
		"operations": OperationsOf([3]any{"listPets", "GET", "/pets"}, [3]any{"createPet", "POST", "/pets"}),
	}}
	steps, ctx, local, _ := CloudSteps(t, func(w http.ResponseWriter, r *http.Request) { stub.handler(w, r) })
	local.Set(KeyAPIID, "api-1")
	return steps, ctx, local, stub
}
