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

package graphql

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

func TestApplyGraphQLOverridesAcceptsOnlyTheAllowedFields(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	local, _ := tcontext.LocalOf(ctx)
	local.Set("echo", "https://echo.example.test")
	definition := map[string]any{"sdl": cloudbase.GraphQLSchema}

	require.NoError(t, cloudbase.ApplyGraphQLOverrides(ctx, definition, map[string]string{
		"policies": `[{"name":"api-key-auth"}]`, "upstream": "${CTX:echo}", "sdl": "type Query { ping: String }",
	}, map[string]bool{"policies": true, "upstream": true, "sdl": true}))
	require.Equal(t, []any{map[string]any{"name": "api-key-auth"}}, definition["policies"])
	require.Equal(t, map[string]any{"main": map[string]string{"url": "https://echo.example.test"}}, definition["upstream"])
	require.Equal(t, "type Query { ping: String }", definition["sdl"])

	definition = map[string]any{"schemaSource": "inline", "sdl": cloudbase.GraphQLSchema}
	require.NoError(t, cloudbase.ApplyGraphQLOverrides(ctx, definition, map[string]string{"schemaSource": "url", "sdlUrl": "${CTX:echo}"}, graphQLSubmitKeys))
	require.NotContains(t, definition, "sdl", "the default inline schema goes with the inline source")
	definition = map[string]any{"schemaSource": "inline", "sdl": cloudbase.GraphQLSchema}
	require.NoError(t, cloudbase.ApplyGraphQLOverrides(ctx, definition, map[string]string{"schemaSource": "url", "sdl": "type Query { a: String }"}, graphQLSubmitKeys))
	require.Equal(t, "type Query { a: String }", definition["sdl"], "a schema the table gives is kept")

	require.ErrorContains(t, cloudbase.ApplyGraphQLOverrides(ctx, definition, map[string]string{"policies": "[]"}, graphQLUpdateKeys),
		"unsupported GraphQL API value")
	require.ErrorContains(t, cloudbase.ApplyGraphQLOverrides(ctx, definition, map[string]string{"policies": "{"}, cloudbase.GraphQLCreateKeys),
		"not valid JSON")
}

// graphQLStub serves the GraphQL API routes the steps call and records what they sent.
type graphQLStub struct {
	t            *testing.T
	registry     *cleanup.Registry
	createStatus int
	updateStatus int
	created      map[string]any
	updated      map[string]any
	registered   []string
	sdl          map[string]string
	validated    map[string]any
	validation   string
	keyStatus    int
	keyRequests  []string
	apis         map[string]bool
	// keyAnswer, when set, replaces the answer to a key creation.
	keyAnswer map[string]string
	// keyNames are the display names of the keys created, in order.
	keyNames []string
}

// metadata reads the metadata part of a GraphQL API form; ok is false once a request it cannot
// read has been answered.
func (s *graphQLStub) metadata(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	fields, _, ok := cloudbase.HandlerForm(s.t, w, r)
	if !ok {
		return nil, false
	}
	var metadata map[string]any
	if !assert.NoError(s.t, json.Unmarshal([]byte(fields["metadata"]), &metadata)) {
		w.WriteHeader(http.StatusBadRequest)
		return nil, false
	}
	return metadata, true
}

func (s *graphQLStub) handler(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/graphql-apis":
		s.registered = cloudbase.PendingOf(s.registry, cleanup.KindGraphQLAPI)
		created, ok := s.metadata(w, r)
		if !ok {
			return
		}
		s.created = created
		cloudbase.WriteJSON(w, s.createStatus, s.created)
	case r.Method == http.MethodPut && r.URL.Path == "/graphql-apis/gql-1":
		updated, ok := s.metadata(w, r)
		if !ok {
			return
		}
		s.updated = updated
		cloudbase.WriteJSON(w, s.updateStatus, s.updated)
	case r.Method == http.MethodPost && r.URL.Path == "/graphql-apis/validate-schema":
		validated, ok := s.metadata(w, r)
		if !ok {
			return
		}
		s.validated = validated
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(s.validation))
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/sdl"):
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/graphql-apis/"), "/sdl")
		if sdl, ok := s.sdl[id]; ok {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(sdl))
			return
		}
		cloudbase.WriteJSON(w, http.StatusNotFound, map[string]string{"code": "NOT_FOUND"})
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/graphql-apis/"):
		if id := strings.TrimPrefix(r.URL.Path, "/graphql-apis/"); s.apis[id] {
			cloudbase.WriteJSON(w, http.StatusOK, map[string]string{"id": id})
			return
		}
		cloudbase.WriteJSON(w, http.StatusNotFound, map[string]string{"code": "NOT_FOUND"})
	case strings.HasPrefix(r.URL.Path, "/graphql-apis/gql-1/api-keys"):
		s.keyRequests = append(s.keyRequests, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodPost {
			var body map[string]string
			if !assert.NoError(s.t, json.NewDecoder(r.Body).Decode(&body)) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			s.keyNames = append(s.keyNames, body["displayName"])
			answer := map[string]string{"status": "success", "apiKey": "key-value", "keyId": "key-1"}
			if s.keyAnswer != nil {
				answer = s.keyAnswer
			}
			cloudbase.WriteJSON(w, s.keyStatus, answer)
			return
		}
		w.WriteHeader(s.keyStatus)
	default:
		w.WriteHeader(http.StatusTeapot)
	}
}

func graphQLSteps(t *testing.T) (*Steps, context.Context, *tcontext.Local, *graphQLStub) {
	t.Helper()
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	stub := &graphQLStub{t: t, createStatus: http.StatusCreated, updateStatus: http.StatusOK, keyStatus: http.StatusCreated, sdl: map[string]string{}, apis: map[string]bool{}}
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) { stub.handler(w, r) })
	stub.registry = registry
	local.Set(cloudbase.KeyProjectID, "default")
	return steps, ctx, local, stub
}

func TestCreateGraphQLAPIRegistersBeforeTheRequestAndKeepsTheMetadata(t *testing.T) {
	steps, ctx, _, stub := graphQLSteps(t)

	require.NoError(t, steps.createGraphQLAPIWithValues(ctx, cloudbase.ValuesTable(t, []string{"policies", `[{"name":"api-key-auth"}]`})))
	id := cloudbase.OptionalValue(ctx, cloudbase.KeyGraphQLAPIID)
	require.Equal(t, []string{id}, stub.registered)
	require.Equal(t, []any{map[string]any{"name": "api-key-auth"}}, stub.created["policies"])
	definition, err := steps.StoredDefinition(ctx, cloudbase.GraphQLAPI)
	require.NoError(t, err)
	require.Equal(t, stub.created, definition)

	stub.createStatus = http.StatusBadRequest
	require.Error(t, steps.CreateGraphQLAPI(ctx))
	require.Len(t, cloudbase.PendingOf(stub.registry, cleanup.KindGraphQLAPI), 2, "a refusal can follow a stored API")
	stub.createStatus = http.StatusInternalServerError
	require.Error(t, steps.CreateGraphQLAPI(ctx))
	require.Len(t, cloudbase.PendingOf(stub.registry, cleanup.KindGraphQLAPI), 3)
	require.Error(t, steps.createGraphQLAPIWithValues(ctx, cloudbase.ValuesTable(t, []string{"upstream", "https://x.example.test"})))
}

func TestSubmitGraphQLAPILeavesTheAnswer(t *testing.T) {
	steps, ctx, local, stub := graphQLSteps(t)
	local.Set("echo", "https://echo.example.test")

	stub.createStatus = http.StatusBadRequest
	require.NoError(t, steps.submitGraphQLAPI(ctx, cloudbase.ValuesTable(t, []string{"schemaSource", "inline"}, []string{"sdlUrl", "${CTX:echo}"})))
	require.Equal(t, "https://echo.example.test", stub.created["sdlUrl"])
	require.Equal(t, cloudbase.GraphQLSchema, stub.created["sdl"], "the default inline schema stays, so the fields disagree")
	refused := cloudbase.OptionalValue(ctx, keySubmittedGraphQLID)
	require.Equal(t, []string{refused}, cloudbase.PendingOf(stub.registry, cleanup.KindGraphQLAPI), "a refusal can follow a stored API")
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, published.StatusCode)

	stub.createStatus = http.StatusCreated
	require.NoError(t, steps.submitGraphQLAPI(ctx, cloudbase.ValuesTable(t, []string{"sdl", "type Query {"})))
	require.ElementsMatch(t, []string{refused, cloudbase.OptionalValue(ctx, keySubmittedGraphQLID)}, cloudbase.PendingOf(stub.registry, cleanup.KindGraphQLAPI))
	require.Error(t, steps.submitGraphQLAPI(ctx, cloudbase.ValuesTable(t, []string{"policies", "[]"})))
}

func TestUpdateGraphQLAPISendsTheStoredMetadataWithTheChanges(t *testing.T) {
	steps, ctx, local, stub := graphQLSteps(t)
	local.Set(cloudbase.KeyGraphQLAPIID, "gql-1")
	local.Set(cloudbase.KeyGraphQLDefinition, `{"id":"gql-1","context":"/gql-1/v1","sdl":"type Query { hello: String }","schemaSource":"inline"}`)

	require.NoError(t, steps.updateGraphQLAPI(ctx, cloudbase.ValuesTable(t, []string{"upstream", "https://echo.example.test"})))
	require.Equal(t, "/gql-1/v1", stub.updated["context"])
	require.Equal(t, map[string]any{"main": map[string]any{"url": "https://echo.example.test"}}, stub.updated["upstream"])
	definition, err := steps.StoredDefinition(ctx, cloudbase.GraphQLAPI)
	require.NoError(t, err)
	require.Equal(t, "https://echo.example.test", definition["upstream"].(map[string]any)["main"].(map[string]any)["url"])

	require.Error(t, steps.updateGraphQLAPI(ctx, cloudbase.ValuesTable(t, []string{"policies", "[]"})))
	stub.updateStatus = http.StatusBadRequest
	require.Error(t, steps.updateGraphQLAPI(ctx, cloudbase.ValuesTable(t, []string{"sdl", "type Query { ping: String }"})))
}

func TestGraphQLAnswerSource(t *testing.T) {
	require.Equal(t, "GraphQL backend", graphQLAnswerSource([]byte(`{"query":"{ hello }"}`), "{ hello }"))
	require.Equal(t, "GraphQL backend", graphQLAnswerSource([]byte(`{"query":""}`), ""), "an empty query echoed as sent")
	require.Equal(t, "", graphQLAnswerSource([]byte(`{}`), ""), "an answer without a query is not an echo of one")
	require.Equal(t, "", graphQLAnswerSource([]byte(`{"method":"POST","path":"/echo","json":{}}`), ""))
	require.Equal(t, "", graphQLAnswerSource([]byte(`{"query":"{ other }"}`), "{ hello }"))
	require.Equal(t, "echo backend", graphQLAnswerSource([]byte(`{"method":"POST","path":"/echo","json":{"query":"{ hello }"}}`), "{ hello }"))
	require.Equal(t, "", graphQLAnswerSource([]byte(`{"method":"POST","path":"/echo","json":{"query":"{ other }"}}`), "{ hello }"),
		"an echo of another request is not the answer to this one")
	require.Equal(t, "", graphQLAnswerSource([]byte(`{"method":"POST","path":"/echo"}`), "{ hello }"))
	require.Equal(t, "", graphQLAnswerSource([]byte(`{"method":"GET","path":"/echo"}`), "{ hello }"))
	require.Equal(t, "", graphQLAnswerSource([]byte(`not json`), "{ hello }"))
}

// graphQLDataPlane serves the synthetic GraphQL API's context, answering with what answer returns
// for each call, and records the requests.
func graphQLDataPlane(t *testing.T, steps *Steps, local *tcontext.Local, answer func(call int, body []byte) (int, string)) *[]*http.Request {
	t.Helper()
	var mu sync.Mutex
	var seen []*http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, r.Clone(context.Background()))
		call := len(seen)
		mu.Unlock()
		status, text := answer(call, body)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(text))
	}))
	t.Cleanup(server.Close)
	steps.DataPlaneURLResolver = func(_ context.Context, target string) (string, error) { return target, nil }
	local.Set(cloudbase.KeyGatewayEndpoint, server.URL+"/")
	local.Set(cloudbase.KeyGraphQLDefinition, `{"id":"gql-1","context":"/gql-1/v1"}`)
	return &seen
}

func TestSendGraphQLRequestPostsTheQueryToTheAPIContext(t *testing.T) {
	steps, ctx, local, _ := graphQLSteps(t)
	var bodies []string
	seen := graphQLDataPlane(t, steps, local, func(_ int, body []byte) (int, string) {
		bodies = append(bodies, string(body))
		return http.StatusOK, string(body)
	})

	require.NoError(t, steps.sendGraphQLRequest(ctx, "{ hello }", http.StatusOK))
	require.Equal(t, "/gql-1/v1", (*seen)[0].URL.Path)
	require.Equal(t, http.MethodPost, (*seen)[0].Method)
	require.Equal(t, "application/json", (*seen)[0].Header.Get("Content-Type"))
	require.JSONEq(t, `{"query":"{ hello }"}`, bodies[0])
	require.NoError(t, steps.assertGraphQLResponseSource(ctx, "GraphQL backend"))
	require.Error(t, steps.assertGraphQLResponseSource(ctx, "echo backend"))

	require.ErrorContains(t, steps.sendGraphQLRequestWithAPIKey(ctx, "{ hello }", "X-API-Key", "API key", http.StatusOK), cloudbase.KeyAPIKey)
	local.Set(cloudbase.KeyAPIKey, "key-value")
	local.Set(keyOtherAPIKey, "other-value")
	require.Error(t, steps.sendGraphQLRequestWithAPIKey(ctx, "{ hello }", " ", "API key", http.StatusOK))
	require.NoError(t, steps.sendGraphQLRequestWithAPIKey(ctx, "{ hello }", "X-API-Key", "API key", http.StatusOK))
	require.Equal(t, "key-value", (*seen)[1].Header.Get("X-API-Key"))
	require.NoError(t, steps.sendGraphQLRequestWithAPIKey(ctx, "{ hello }", "X-API-Key", "other API key", http.StatusOK))
	require.Equal(t, "other-value", (*seen)[2].Header.Get("X-API-Key"))

	local.Set(cloudbase.KeyGraphQLDefinition, `{"id":"gql-1"}`)
	require.ErrorContains(t, steps.sendGraphQLRequest(ctx, "{ hello }", http.StatusOK), "no context")
}

func TestSendGraphQLUntilAnsweredByWaitsForTheNamedBackend(t *testing.T) {
	steps, ctx, local, _ := graphQLSteps(t)
	seen := graphQLDataPlane(t, steps, local, func(call int, body []byte) (int, string) {
		if call <= 2 {
			return http.StatusOK, string(body)
		}
		return http.StatusOK, `{"method":"POST","path":"/echo","json":{"query":"{ hello }"}}`
	})

	require.NoError(t, steps.sendGraphQLUntilAnsweredBy(ctx, "{ hello }", "echo backend"))
	require.Len(t, *seen, 3, "the answers of the GraphQL backend are not taken for the echo backend's")
	require.NoError(t, steps.assertGraphQLResponseSource(ctx, "echo backend"))

	short, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	require.Error(t, steps.sendGraphQLUntilAnsweredBy(short, "{ hello }", "GraphQL backend"))
}

func TestGraphQLSchemaAssertions(t *testing.T) {
	steps, ctx, local, stub := graphQLSteps(t)
	local.Set(cloudbase.KeyGraphQLAPIID, "gql-1")
	local.Set(keySubmittedGraphQLID, "gql-2")
	stub.sdl["gql-1"] = `{"sdl":"type Query {\n  hello: String\n}"}`
	stub.sdl["gql-2"] = `{"sdl":""}`

	require.NoError(t, steps.assertGraphQLSchema(ctx, "synthetic", "type Query { hello: String }"))
	require.Error(t, steps.assertGraphQLSchema(ctx, "synthetic", "type Query { ping: String }"))
	require.Error(t, steps.assertEmptyGraphQLSchema(ctx, "synthetic"))
	require.NoError(t, steps.assertEmptyGraphQLSchema(ctx, "submitted"))
	require.Error(t, steps.assertGraphQLSchema(ctx, "submitted", cloudbase.GraphQLSchema))

	stub.sdl["gql-2"] = `{}`
	require.ErrorContains(t, steps.assertEmptyGraphQLSchema(ctx, "submitted"), "no schema field")
	delete(stub.sdl, "gql-2")
	require.Error(t, steps.assertEmptyGraphQLSchema(ctx, "submitted"), "an API that does not exist has no schema to read")
	require.NoError(t, steps.assertSubmittedGraphQL(ctx, "not exist"))
	require.Error(t, steps.assertSubmittedGraphQL(ctx, "exist"))
	stub.apis["gql-2"] = true
	require.NoError(t, steps.assertSubmittedGraphQL(ctx, "exist"))
	require.Error(t, steps.assertSubmittedGraphQL(ctx, "not exist"))
	local.Set(keySubmittedGraphQLID, "")
	require.Error(t, steps.assertSubmittedGraphQL(ctx, "exist"))
}

func TestSameSchemaComparesTokens(t *testing.T) {
	require.True(t, sameSchema("type Query { hello: String }", "type Query {\n  hello: String\n}\n"))
	require.True(t, sameSchema("type Query { hello: String }", "type Query{hello:String}"))
	require.True(t, sameSchema("type Query { a: String, b: Int }", "type Query {\n  a: String # first\n  b: Int\n}"),
		"commas and comments are not tokens")
	require.False(t, sameSchema("type Query { hello: String }", "type Query { hello: Int }"))
	require.False(t, sameSchema(`"a  b" type Query { a: String }`, `"a b" type Query { a: String }`),
		"a string keeps its spacing")
	require.False(t, sameSchema(`"""a  b""" type Query { a: String }`, `"""a b""" type Query { a: String }`))
	require.False(t, sameSchema("type Query { a: [Int!]! }", "type Query { a: [Int]! }"))
}

func TestGraphQLTokensSplitsSDLIntoTokens(t *testing.T) {
	require.Equal(t, []string{"type", "Query", "{", "a", "(", "n", ":", "Int", "=", "-1.5e+3", ")", ":", "[", "Int", "!", "]", "@", "x", "}"},
		graphQLTokens("type Query{a(n:Int=-1.5e+3):[Int!] @x}"))
	require.Equal(t, []string{"fragment", "F", "on", "Q", "{", "...", "G", "}"}, graphQLTokens("fragment F on Q { ...G }"))
	require.Equal(t, []string{"[", "1", "-2", "]"}, graphQLTokens("[1-2]"), "adjacent signed numbers stay apart")
	require.True(t, sameSchema("directive @d(a: [Int] = [1,-2]) on FIELD", "directive @d(a: [Int] = [1 -2]) on FIELD"))
	require.Equal(t, []string{`"""a \""" b"""`, "type"}, graphQLTokens(`"""a \""" b""" type`), "an escaped triple quote does not end a block string")
	require.Equal(t, []string{`"a \" b"`, "x"}, graphQLTokens(`"a \" b" x`))
	require.Equal(t, []string{`"open`}, graphQLTokens(`"open`), "an unterminated string ends with the text")
	require.Empty(t, graphQLTokens(" ,\n# only a comment\n"))
}

func TestValidateGraphQLSchemaAndItsAssertions(t *testing.T) {
	steps, ctx, _, stub := graphQLSteps(t)
	stub.validation = `{"resolved":true,"sdl":"type Query {\n  hello: String\n}"}`

	require.NoError(t, steps.validateGraphQLSchema(ctx, cloudbase.GraphQLSchema))
	require.Equal(t, map[string]any{"schemaSource": "inline", "sdl": cloudbase.GraphQLSchema}, stub.validated)
	require.NoError(t, steps.assertGraphQLValidation(ctx, "resolved"))
	require.Error(t, steps.assertGraphQLValidation(ctx, "not resolved"))

	stub.validation = `{"resolved":false,"sdl":"","sdlErrors":[{"message":"Syntax Error"}]}`
	require.NoError(t, steps.validateGraphQLSchema(ctx, "type Query {"))
	require.NoError(t, steps.assertGraphQLValidation(ctx, "not resolved"))
	require.Error(t, steps.assertGraphQLValidation(ctx, "resolved"))

	for name, body := range map[string]string{
		"no errors":    `{"resolved":false,"sdl":""}`,
		"a null error": `{"resolved":false,"sdl":"","sdlErrors":[null]}`,
		"no message":   `{"resolved":false,"sdl":"","sdlErrors":[{"message":" "}]}`,
		"a schema":     `{"resolved":false,"sdl":"type Query { a: String }","sdlErrors":[{}]}`,
		"not a result": `{"sdl":""}`,
	} {
		cloudbase.PublishedResponse(t, steps.Steps, ctx, http.StatusOK, body)
		require.Error(t, steps.assertGraphQLValidation(ctx, "not resolved"), name)
	}
	cloudbase.PublishedResponse(t, steps.Steps, ctx, http.StatusOK, `{"resolved":true,"sdl":"type Query { other: String }"}`)
	require.Error(t, steps.assertGraphQLValidation(ctx, "resolved"), "a schema other than the one given")
}

func TestGraphQLAPIKeyIsKeptOutOfTheAnswerSlotAndRevokedByItsID(t *testing.T) {
	steps, ctx, local, stub := graphQLSteps(t)
	local.Set(cloudbase.KeyGraphQLAPIID, "gql-1")

	require.NoError(t, steps.createGraphQLAPIKey(ctx, "an"))
	require.Equal(t, "key-value", cloudbase.OptionalValue(ctx, cloudbase.KeyAPIKey))
	require.Equal(t, "key-1", cloudbase.OptionalValue(ctx, keyGraphQLKeyID))
	_, err := httpx.Published(ctx)
	require.Error(t, err, "the answer that carries the key is never published")

	stub.keyStatus = http.StatusNoContent
	require.NoError(t, steps.revokeGraphQLAPIKey(ctx))
	require.Equal(t, []string{"POST /graphql-apis/gql-1/api-keys", "DELETE /graphql-apis/gql-1/api-keys/key-1"}, stub.keyRequests)

	stub.keyStatus = http.StatusNotFound
	require.Error(t, steps.revokeGraphQLAPIKey(ctx))
	err = steps.createGraphQLAPIKey(ctx, "an")
	require.ErrorContains(t, err, "unexpected status 404")
	require.NotContains(t, err.Error(), "key-value", "a refused key request is reported without its body")
}

func TestGraphQLAPIKeyIsRegisteredForCleanupUntilItIsRevoked(t *testing.T) {
	steps, ctx, local, stub := graphQLSteps(t)
	local.Set(cloudbase.KeyGraphQLAPIID, "gql-1")

	require.NoError(t, steps.createGraphQLAPIKey(ctx, "an"))
	require.Equal(t, []string{"gql-1/key-1"}, cloudbase.PendingOf(stub.registry, KindGraphQLAPIKey))
	stub.keyStatus = http.StatusNoContent
	require.NoError(t, steps.revokeGraphQLAPIKey(ctx))
	require.Empty(t, cloudbase.PendingOf(stub.registry, KindGraphQLAPIKey), "a revoked key is no longer cleanup's to revoke")

	method, path, settled, err := RevokeGraphQLKeyRequest("gql-1/key-1")
	require.NoError(t, err)
	require.Equal(t, http.MethodDelete, method)
	require.Equal(t, "/graphql-apis/gql-1/api-keys/key-1", path)
	require.True(t, settled(&httpx.Response{StatusCode: http.StatusNoContent}))
	require.True(t, settled(&httpx.Response{StatusCode: http.StatusNotFound}), "a key the platform no longer has is revoked")
	require.False(t, settled(&httpx.Response{StatusCode: http.StatusInternalServerError}))
	for _, id := range []string{"gql-1", "/key-1", "gql-1/", ""} {
		_, _, _, err := RevokeGraphQLKeyRequest(id)
		require.Error(t, err, id)
	}
	require.Less(t, KindGraphQLAPIKey.Order, cleanup.KindGraphQLAPI.Order, "a key is revoked before its API is deleted")
}

func TestGraphQLAPIKeyIsRegisteredBeforeItsValueIsRead(t *testing.T) {
	steps, ctx, local, stub := graphQLSteps(t)
	local.Set(cloudbase.KeyGraphQLAPIID, "gql-1")
	stub.keyAnswer = map[string]string{"status": "success", "keyId": "key-1"}

	require.ErrorContains(t, steps.createGraphQLAPIKey(ctx, "an"), "apiKey")
	require.Equal(t, []string{"gql-1/key-1"}, cloudbase.PendingOf(stub.registry, KindGraphQLAPIKey), "the key is cleaned up though its value never came")

	stub.keyAnswer = map[string]string{"status": "success", "apiKey": "second-value", "keyId": "key-2"}
	require.NoError(t, steps.createGraphQLAPIKey(ctx, "another"))
	require.Equal(t, "second-value", cloudbase.OptionalValue(ctx, keyOtherAPIKey))
	require.Equal(t, "key-2", cloudbase.OptionalValue(ctx, keyOtherGraphQLKeyID))
	require.Empty(t, cloudbase.OptionalValue(ctx, cloudbase.KeyAPIKey), "the other key does not take the first key's place")
	require.Len(t, stub.keyNames, 2)
	require.NotEqual(t, stub.keyNames[0], stub.keyNames[1], "each key of an API has a display name of its own")

	for status, ok := range map[int]bool{http.StatusNoContent: true, http.StatusNotFound: true, http.StatusServiceUnavailable: false} {
		stub.keyStatus = status
		err := steps.revokeGraphQLKey(ctx, "gql-1", "key-9")
		if ok {
			require.NoError(t, err, status)
			continue
		}
		require.ErrorContains(t, err, "unexpected status 503")
		require.ErrorContains(t, err, "key-9")
	}
}

// cloudSteps wires the area's steps to a stub cloud API.
func cloudSteps(t *testing.T, handler http.HandlerFunc) (*Steps, context.Context, *tcontext.Local, *cleanup.Registry) {
	t.Helper()
	base, ctx, local, registry := cloudbase.CloudSteps(t, handler)
	return &Steps{Steps: base}, ctx, local, registry
}
