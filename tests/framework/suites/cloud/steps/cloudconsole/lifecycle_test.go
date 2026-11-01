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

package cloudconsole

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image/png"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

func TestBearerTokenReadsAndCachesEnvironmentToken(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "access-token")
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))

	token, err := bearerToken(ctx)
	require.NoError(t, err)
	require.Equal(t, "access-token", token)

	t.Setenv(cloudConsoleAccessTokenEnv, "a-different-token")
	token, err = bearerToken(ctx)
	require.NoError(t, err)
	require.Equal(t, "access-token", token)
}

func TestBearerTokenRequiresEnvironmentToken(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "")
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))

	_, err := bearerToken(ctx)
	require.ErrorContains(t, err, cloudConsoleAccessTokenEnv)
}

func TestCloudAPIURLUsesResolvedEndpointAsBase(t *testing.T) {
	require.Equal(t,
		"https://cloud.example.com/apip-platform-api-apip-platform-api-endpoint/api/v0.9/projects",
		cloudAPIURL("https://cloud.example.com/apip-platform-api-apip-platform-api-endpoint/api/v0.9/", "/projects"))
}

func TestCloudResourceURLEscapesID(t *testing.T) {
	require.Equal(t,
		"https://cloud.example.com/apip-platform-api-apip-platform-api-endpoint/api/v0.9/projects/id%2Fwith%20separator",
		cloudResourceURL("https://cloud.example.com/apip-platform-api-apip-platform-api-endpoint/api/v0.9", "/projects", "id/with separator"))
}

func TestResolveCloudDataPlaneURLRequiresHTTPSAndApproval(t *testing.T) {
	for _, target := range []string{
		"", "relative/path", "http://gateway.example.com/api", "ftp://gateway.example.com/api",
		"//gateway.example.com/api", "https://127.0.0.1/api", "https://10.0.0.1/api",
		"https://example.com/api", "https://synthetic-wso2cloud.gateway.development.example:8443/api",
		"https://user:password@synthetic-wso2cloud.gateway.development.example/api",
	} {
		_, err := resolveCloudDataPlaneURL(context.Background(), target)
		require.Error(t, err, target)
	}
	_, err := resolveCloudDataPlaneURL(context.Background(), "https://synthetic-wso2cloud.gateway.wso2.com/api")
	require.NoError(t, err)
}

func TestFirstApprovedDataPlaneEndpointSkipsInvalidCandidates(t *testing.T) {
	steps := &Steps{dataPlaneURLResolver: func(_ context.Context, target string) (string, error) {
		if target == "https://unapproved.example.com" {
			return "", errors.New("unapproved")
		}
		return target, nil
	}}

	endpoint, ok := firstApprovedDataPlaneEndpoint(context.Background(), steps, []any{
		"https://unapproved.example.com", " https://approved.example.com/// ", 42,
	})
	require.True(t, ok)
	require.Equal(t, "https://approved.example.com", endpoint)
}

func TestExpandCloudValueResolvesContextValues(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	local, ok := tcontext.LocalOf(ctx)
	require.True(t, ok)
	local.Set("gatewayEndpoint", "https://gateway.example.com")

	got, err := expandCloudValue(ctx, "${CTX:gatewayEndpoint}/api/v1/posts/1")
	require.NoError(t, err)
	require.Equal(t, "https://gateway.example.com/api/v1/posts/1", got)
}

func TestSendUntilStatusPollsUntilExpectedStatus(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	methods := make([]string, 0, 2)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		attempt := calls
		methods = append(methods, r.Method)
		mu.Unlock()
		if attempt == 1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	defer server.Close()

	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	steps := &Steps{funnel: httpx.NewFunnel(httpx.NewClient(httpx.Options{
		Timeout:            5 * time.Second,
		InsecureSkipVerify: true,
	}), 0, 0), dataPlaneURLResolver: func(_ context.Context, target string) (string, error) {
		return target, nil
	}}

	require.NoError(t, steps.sendUntilStatus(ctx, http.MethodGet, server.URL, http.StatusOK))
	mu.Lock()
	gotCalls := calls
	gotMethods := append([]string(nil), methods...)
	mu.Unlock()
	require.Equal(t, 2, gotCalls)
	require.Equal(t, []string{http.MethodGet, http.MethodGet}, gotMethods)
	response, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, `{"id":1}`, response.Text())
}

func TestSendUntilStatusRejectsUnapprovedDestination(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	defer server.Close()

	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	steps := &Steps{funnel: httpx.NewFunnel(httpx.NewClient(httpx.Options{}), 0, 0)}

	err := steps.sendUntilStatus(ctx, http.MethodGet, server.URL, http.StatusOK)
	require.ErrorContains(t, err, "approved HTTPS")
	require.Zero(t, calls)
}

func TestListItems(t *testing.T) {
	items, err := listItems(&httpx.Response{Body: []byte(`{"list":[{"id":"default"}]}`)})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "default", items[0]["id"])

	_, err = listItems(&httpx.Response{Body: []byte("invalid")})
	require.Error(t, err)
}

func TestStringField(t *testing.T) {
	resp := &httpx.Response{Body: []byte(`{"id":"resource"}`)}
	got, err := stringField(resp, "id")
	require.NoError(t, err)
	require.Equal(t, "resource", got)

	_, err = stringField(resp, "missing")
	require.ErrorContains(t, err, "missing or empty")
	_, err = stringField(&httpx.Response{Body: []byte("invalid")}, "id")
	require.Error(t, err)
}

func TestGatewayDeploymentID(t *testing.T) {
	items := []map[string]any{
		{"gatewayId": "other", "deploymentId": "dep-other"},
		{"gatewayId": "gw-1", "deploymentId": "dep-1"},
	}
	got, err := gatewayDeploymentID(items, "gw-1")
	require.NoError(t, err)
	require.Equal(t, "dep-1", got)

	_, err = gatewayDeploymentID(items, "missing")
	require.ErrorContains(t, err, "no deployment was made")
	_, err = gatewayDeploymentID([]map[string]any{{"gatewayId": "gw-1"}}, "gw-1")
	require.ErrorContains(t, err, "has no deploymentId")
}

func valuesTable(t *testing.T, rows ...[]string) *godog.Table {
	t.Helper()
	type cell struct {
		Value string `json:"value"`
	}
	type tableRow struct {
		Cells []cell `json:"cells"`
	}
	payload := struct {
		Rows []tableRow `json:"rows"`
	}{}
	for _, row := range rows {
		encoded := tableRow{}
		for _, value := range row {
			encoded.Cells = append(encoded.Cells, cell{Value: value})
		}
		payload.Rows = append(payload.Rows, encoded)
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	table := &godog.Table{}
	require.NoError(t, json.Unmarshal(raw, table))
	return table
}

var stepKeyword = regexp.MustCompile(`^(Given|When|Then|And|But|\*) (.+)$`)

// featureSteps returns the text of every step a feature file runs, with each Scenario Outline
// expanded once per Examples row.
func featureSteps(t *testing.T, path string) []string {
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

func TestEveryFeatureStepMatchesExactlyOneDefinition(t *testing.T) {
	definitions := (&Steps{}).stepDefinitions()
	patterns := make([]*regexp.Regexp, len(definitions))
	seen := map[string]bool{}
	for i, definition := range definitions {
		require.False(t, seen[definition.pattern], "duplicate step pattern %s", definition.pattern)
		seen[definition.pattern] = true
		patterns[i] = regexp.MustCompile(definition.pattern)
	}
	features, err := filepath.Glob(filepath.Join("..", "..", "features", "*.feature"))
	require.NoError(t, err)
	require.NotEmpty(t, features)
	used := make([]bool, len(patterns))
	for _, feature := range features {
		for _, step := range featureSteps(t, feature) {
			matches := 0
			for i, pattern := range patterns {
				if pattern.MatchString(step) {
					matches++
					used[i] = true
				}
			}
			require.Equal(t, 1, matches, "%s: step %q matches %d definitions", filepath.Base(feature), step, matches)
		}
	}
	for i, definition := range definitions {
		require.True(t, used[i], "step %s is used by no feature", definition.pattern)
	}
}

func TestFeatureStepsExpandsOutlines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outline.feature")
	require.NoError(t, os.WriteFile(path, []byte(`Feature: f
  Background:
    Given a start
  Scenario Outline: o
    When I <action> it
      | key | value |
    Then it is <status>

    Examples:
      | action | status |
      | open   | 200    |
      | close  | 409    |
  Scenario: s
    * a last step
`), 0o600))
	require.Equal(t, []string{
		"a start", "I open it", "it is 200", "I close it", "it is 409", "a last step",
	}, featureSteps(t, path))
}

func TestTableValues(t *testing.T) {
	values, err := tableValues(nil)
	require.NoError(t, err)
	require.Empty(t, values)

	values, err = tableValues(valuesTable(t, []string{" displayName ", "Edited"}, []string{"docIds", "[]"}))
	require.NoError(t, err)
	require.Equal(t, map[string]string{"displayName": "Edited", "docIds": "[]"}, values)

	for name, table := range map[string]*godog.Table{
		"three cells": valuesTable(t, []string{"a", "b", "c"}),
		"empty key":   valuesTable(t, []string{" ", "b"}),
		"duplicate":   valuesTable(t, []string{"a", "b"}, []string{"a", "c"}),
	} {
		_, err := tableValues(table)
		require.Error(t, err, name)
	}
}

func TestRefusedCreate(t *testing.T) {
	for status, want := range map[int]bool{
		http.StatusCreated: false, http.StatusBadRequest: true, http.StatusConflict: true,
		http.StatusInternalServerError: false, http.StatusBadGateway: false,
	} {
		require.Equal(t, want, refusedCreate(&httpx.Response{StatusCode: status}), status)
	}
}

func TestPlanBody(t *testing.T) {
	body := planBody("plan-1", "INACTIVE")
	require.Equal(t, "plan-1", body["id"])
	require.Equal(t, "Synthetic plan-1", body["displayName"])
	require.Equal(t, "INACTIVE", body["status"])
	require.Len(t, body["limits"], 1)
}

func TestDraftBody(t *testing.T) {
	body, err := draftBody("api-1", "plan-1", nil)
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"displayName": "Synthetic api-1", "version": "1.0.0", "subscriptionPlanIds": []string{"plan-1"},
	}, body)

	body, err = draftBody("api-1", "plan-1", map[string]string{
		"displayName": "Edited", "description": "d", "subscriptionPlanIds": `["a", "b"]`, "docIds": "[]",
	})
	require.NoError(t, err)
	require.Equal(t, "Edited", body["displayName"])
	require.Equal(t, "d", body["description"])
	require.Equal(t, []string{"a", "b"}, body["subscriptionPlanIds"])
	require.Equal(t, []string{}, body["docIds"])

	_, err = draftBody("api-1", "plan-1", map[string]string{"docIds": "doc-1"})
	require.ErrorContains(t, err, "JSON array")
	_, err = draftBody("api-1", "plan-1", map[string]string{"docIds": `[1]`})
	require.ErrorContains(t, err, "JSON array")
	_, err = draftBody("api-1", "plan-1", map[string]string{"labels": "[]"})
	require.ErrorContains(t, err, "unsupported draft field")
}

func TestActiveAPIPortalID(t *testing.T) {
	id, err := activeAPIPortalID([]map[string]any{
		{"id": "pending-portal", "status": "pending"},
		{"id": "", "status": "active"},
		{"id": "live-portal", "status": "active"},
	})
	require.NoError(t, err)
	require.Equal(t, "live-portal", id)

	_, err = activeAPIPortalID(nil)
	require.ErrorContains(t, err, "no managed API Portal")
	_, err = activeAPIPortalID([]map[string]any{{"id": "failed-portal", "status": "failed"}})
	require.ErrorContains(t, err, "failed-portal: failed")
}

func TestPublicationTargetPathAndID(t *testing.T) {
	target := publicationTarget{portal: "portal 1", apiType: "rest-api", api: "api/1"}
	require.Equal(t, "/api-portals/portal%201/apis/rest-api/api%2F1/publish", target.path("/publish"))

	target = publicationTarget{portal: "portal-1", apiType: "graphql-api", api: "api-1"}
	parts, err := splitPublicationID(target.id())
	require.NoError(t, err)
	require.Equal(t, []string{"portal-1", "graphql-api", "api-1"}, parts)

	for _, id := range []string{"", "a/b", "a/b/c/d", "a//c", "/b/c"} {
		_, err := splitPublicationID(id)
		require.Error(t, err, id)
	}
}

func TestTargetOf(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	_, err := targetOf(ctx, "synthetic REST API")
	require.Error(t, err)

	local, ok := tcontext.LocalOf(ctx)
	require.True(t, ok)
	local.Set(keyAPIPortalID, "portal-1")
	local.Set(keyAPIID, "rest-1")
	local.Set(keyGraphQLAPIID, "graphql-1")

	target, err := targetOf(ctx, "synthetic REST API")
	require.NoError(t, err)
	require.Equal(t, publicationTarget{portal: "portal-1", apiType: "rest-api", api: "rest-1"}, target)
	target, err = targetOf(ctx, "synthetic GraphQL API")
	require.NoError(t, err)
	require.Equal(t, publicationTarget{portal: "portal-1", apiType: "graphql-api", api: "graphql-1"}, target)
	_, err = targetOf(ctx, "synthetic MCP server")
	require.ErrorContains(t, err, "unsupported publication subject")
}

func TestUnpublishSettled(t *testing.T) {
	for name, tc := range map[string]struct {
		resp *httpx.Response
		want bool
	}{
		"unpublished":       {&httpx.Response{StatusCode: http.StatusNoContent}, true},
		"no API":            {&httpx.Response{StatusCode: http.StatusNotFound}, true},
		"nothing published": {&httpx.Response{StatusCode: http.StatusConflict, Body: []byte(`{"code":"PUBLICATION_STATE_CONFLICT"}`)}, true},
		"portal conflict":   {&httpx.Response{StatusCode: http.StatusConflict, Body: []byte(`{"code":"PUBLICATION_PORTAL_CONFLICT"}`)}, false},
		"unreadable":        {&httpx.Response{StatusCode: http.StatusConflict, Body: []byte(`not json`)}, false},
		"server error":      {&httpx.Response{StatusCode: http.StatusServiceUnavailable}, false},
	} {
		require.Equal(t, tc.want, unpublishSettled(tc.resp), name)
	}
}

func TestCleanupRequests(t *testing.T) {
	method, path, settled, err := unpublishRequest("portal-1/rest-api/api-1")
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, method)
	require.Equal(t, "/api-portals/portal-1/apis/rest-api/api-1/unpublish", path)
	require.True(t, settled(&httpx.Response{StatusCode: http.StatusNoContent}))

	method, path, settled, err = deleteDocumentRequest("rest-api/api-1/doc-1")
	require.NoError(t, err)
	require.Equal(t, http.MethodDelete, method)
	require.Equal(t, "/apis/rest-api/api-1/docs/doc-1", path)
	require.True(t, settled(&httpx.Response{StatusCode: http.StatusNotFound}))
	require.False(t, settled(&httpx.Response{StatusCode: http.StatusConflict}))

	_, _, _, err = unpublishRequest("malformed")
	require.Error(t, err)
	_, _, _, err = deleteDocumentRequest("malformed")
	require.Error(t, err)
}

func TestSummaryStatusIs(t *testing.T) {
	items := []map[string]any{
		{"apiPortalId": "other", "status": "NOT_PUBLISHED"},
		{"apiPortalId": "portal-1", "status": "PUBLISHED"},
	}
	require.NoError(t, summaryStatusIs(items, "portal-1", "PUBLISHED"))
	require.ErrorContains(t, summaryStatusIs(items, "portal-1", "DEPRECATED"), "want DEPRECATED")
	require.ErrorContains(t, summaryStatusIs(items, "missing", "PUBLISHED"), "do not list")
	require.Error(t, summaryStatusIs(nil, "portal-1", "PUBLISHED"))
}

func TestSelectionIDs(t *testing.T) {
	publication := map[string]any{"subscriptionPlans": []any{
		map[string]any{"id": "plan-1"}, map[string]any{"displayName": "no id"}, "not a selection",
		map[string]any{"id": "plan-2"},
	}}
	require.Equal(t, []string{"plan-1", "plan-2"}, selectionIDs(publication, "subscriptionPlans"))
	require.Empty(t, selectionIDs(publication, "documents"))
	require.Empty(t, selectionIDs(map[string]any{"documents": "wrong"}, "documents"))
}

func TestDefinitionFor(t *testing.T) {
	body, contentType, err := definitionFor("valid", "graphql-api", "api-1")
	require.NoError(t, err)
	require.Equal(t, "application/graphql", contentType)
	require.Equal(t, graphQLSchema, string(body))

	for _, tc := range []struct{ kind, apiType string }{{"valid", "rest-api"}, {"OpenAPI", "graphql-api"}} {
		body, contentType, err = definitionFor(tc.kind, tc.apiType, "api-1")
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		var document map[string]any
		require.NoError(t, json.Unmarshal(body, &document))
		require.Equal(t, "3.0.3", document["openapi"])
		require.Equal(t, "api-1", document["info"].(map[string]any)["title"])
	}

	body, _, err = definitionFor("invalid", "rest-api", "api-1")
	require.NoError(t, err)
	require.NotContains(t, string(body), "openapi")
	_, _, err = definitionFor("missing", "rest-api", "api-1")
	require.ErrorContains(t, err, "unsupported definition")
}

func TestForgedTokenIsAnUnsignedJWT(t *testing.T) {
	parts := strings.Split(forgedToken(), ".")
	require.Len(t, parts, 3)
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	require.JSONEq(t, `{"alg":"RS256","typ":"JWT"}`, string(header))
	claims, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(claims, &decoded))
	require.Greater(t, decoded["exp"], float64(time.Now().Unix()))
	require.NotEmpty(t, parts[2])
}

func TestThumbnailFormHoldsAPNG(t *testing.T) {
	body, contentType, err := thumbnailForm()
	require.NoError(t, err)
	_, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	part, err := multipart.NewReader(bytes.NewReader(body), params["boundary"]).NextPart()
	require.NoError(t, err)
	require.Equal(t, "file", part.FormName())
	require.Equal(t, "image/png", part.Header.Get("Content-Type"))
	_, err = png.Decode(part)
	require.NoError(t, err)
}

func TestMultipartFields(t *testing.T) {
	body, contentType, err := multipartFields([][2]string{{"metadata", `{"id":"api-1"}`}})
	require.NoError(t, err)
	_, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	form, err := multipart.NewReader(bytes.NewReader(body), params["boundary"]).ReadForm(1 << 20)
	require.NoError(t, err)
	require.Equal(t, []string{`{"id":"api-1"}`}, form.Value["metadata"])
}

func TestLandingPageCarriesRawHTML(t *testing.T) {
	require.Contains(t, landingPage, "<script>")
	require.Contains(t, landingPage, landingPageText)
}

func TestAssertCloudResponseStatusAndErrorCode(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	steps := &Steps{}
	require.Error(t, steps.assertCloudResponseStatus(ctx, http.StatusOK))

	funnel := httpx.NewFunnel(httpx.NewClient(httpx.Options{}), 0, 0)
	require.NoError(t, funnel.Publish(ctx, &httpx.Response{
		StatusCode: http.StatusConflict, Body: []byte(`{"code":"PUBLICATION_STATE_CONFLICT"}`),
	}))
	require.NoError(t, steps.assertCloudResponseStatus(ctx, http.StatusConflict))
	require.ErrorContains(t, steps.assertCloudResponseStatus(ctx, http.StatusOK), "expected response status 200")
	require.NoError(t, steps.assertCloudErrorCode(ctx, "PUBLICATION_STATE_CONFLICT"))
	require.ErrorContains(t, steps.assertCloudErrorCode(ctx, "DRAFT_NOT_FOUND"), "expected error code")

	require.NoError(t, funnel.Publish(ctx, &httpx.Response{StatusCode: http.StatusUnauthorized, Body: []byte(`{}`)}))
	require.ErrorContains(t, steps.assertCloudErrorCode(ctx, "UNAUTHORIZED"), "reading the error code")
}
