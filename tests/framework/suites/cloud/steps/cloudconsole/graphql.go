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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const (
	kindGraphQLAPI        = "GraphQLApi"
	graphQLAPIsCollection = "/graphql-apis"
	graphQLAPILabel       = "synthetic GraphQL API"

	keyGraphQLDeploymentID  = "cloudGraphQLDeploymentID"
	keyGraphQLDefinition    = "cloudGraphQLDefinition"
	keyGraphQLBuildIDs      = "cloudGraphQLBuildIDs"
	keyGraphQLBuildsOwner   = "cloudGraphQLBuildsOwner"
	keySubmittedGraphQLID   = "cloudSubmittedGraphQLAPIID"
	keyGraphQLRequest       = "cloudGraphQLRequest"
	keyGraphQLKeyID         = "cloudGraphQLAPIKeyID"
	keyValidatedGraphQLText = "cloudValidatedGraphQLSchema"

	// graphQLBackendEnv overrides the backend the synthetic GraphQL APIs call.
	graphQLBackendEnv = "CLOUD_IT_GRAPHQL_BACKEND_URL"
	// defaultGraphQLBackend is the testbench GraphQL backend, which answers with the body it received.
	defaultGraphQLBackend = "https://development-wso2cloud.gateway.dev.cloud.wso2.com/apip-testbench-mocks/graphql-backend"

	graphQLSchema = "type Query { hello: String }"
)

// submittedGraphQLAPI is a GraphQL API a creation request was sent for without requiring that it
// be created.
var submittedGraphQLAPI = apiRef{id: keySubmittedGraphQLID, collection: graphQLAPIsCollection}

// GraphQL API fields a values table may set, by the step that accepts them. policies carries JSON,
// upstream the main upstream URL, and the rest text.
var (
	graphQLCreateKeys = map[string]bool{"policies": true, "sdl": true}
	graphQLSubmitKeys = map[string]bool{"schemaSource": true, "sdl": true, "sdlUrl": true}
	graphQLUpdateKeys = map[string]bool{"upstream": true, "sdl": true}
)

// registerGraphQLSteps binds the steps that create, change and call a GraphQL API.
func (s *Steps) registerGraphQLSteps(sc stepRegistrar) {
	sc.Step(`^I create a synthetic GraphQL API in the default project with:$`, s.createGraphQLAPIWithValues)
	sc.Step(`^I submit a synthetic GraphQL API creation request with:$`, s.submitGraphQLAPI)
	sc.Step(`^I update the synthetic GraphQL API with:$`, s.updateGraphQLAPI)
	sc.Step(`^I send the GraphQL request "([^"]*)" to the synthetic GraphQL API until status (\d+)$`, s.sendGraphQLRequest)
	sc.Step(`^I send the GraphQL request "([^"]*)" to the synthetic GraphQL API with header "([^"]*)" set to the API key until status (\d+)$`,
		s.sendGraphQLRequestWithAPIKey)
	sc.Step(`^I send the GraphQL request "([^"]*)" to the synthetic GraphQL API until the (GraphQL backend|echo backend) answers$`,
		s.sendGraphQLUntilAnsweredBy)
	sc.Step(`^the GraphQL response should come from the (GraphQL backend|echo backend)$`, s.assertGraphQLResponseSource)
	sc.Step(`^the (synthetic|submitted) GraphQL API should have the schema "([^"]*)"$`, s.assertGraphQLSchema)
	sc.Step(`^the (synthetic|submitted) GraphQL API should have an empty schema$`, s.assertEmptyGraphQLSchema)
	sc.Step(`^the submitted GraphQL API should not exist$`, s.assertSubmittedGraphQLAbsent)
	sc.Step(`^I validate the GraphQL schema "([^"]*)"$`, s.validateGraphQLSchema)
	sc.Step(`^the GraphQL schema should be reported as (resolved|not resolved)$`, s.assertGraphQLValidation)
	sc.Step(`^I create an API key for the synthetic GraphQL API$`, s.createGraphQLAPIKey)
	sc.Step(`^I revoke the API key of the synthetic GraphQL API$`, s.revokeGraphQLAPIKey)
}

// graphQLRefFor resolves "synthetic" or "submitted" in step text to the GraphQL API it names.
func graphQLRefFor(which string) apiRef {
	if which == "submitted" {
		return submittedGraphQLAPI
	}
	return graphQLAPI
}

// newGraphQLDefinition returns the metadata of the default synthetic GraphQL API: an inline schema
// and the testbench GraphQL backend as its upstream.
func newGraphQLDefinition(ctx context.Context, id string) (map[string]any, error) {
	projectID, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return nil, err
	}
	backend, err := testbenchURL(graphQLBackendEnv, defaultGraphQLBackend)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id": id, "displayName": publicationTitle + id, "description": syntheticDescription,
		"context": "/" + id + "/v1", "version": "1.0.0", "projectId": projectID,
		"upstream":     map[string]any{"main": map[string]string{"url": backend}},
		"schemaSource": "inline", "sdl": graphQLSchema,
	}, nil
}

// applyGraphQLOverrides sets the fields a values table supplies, accepting only the allowed ones.
func applyGraphQLOverrides(ctx context.Context, definition map[string]any, overrides map[string]string, allowed map[string]bool) error {
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		if !allowed[key] {
			return fmt.Errorf("unsupported GraphQL API value %q", key)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value, err := expandCloudValue(ctx, overrides[key])
		if err != nil {
			return err
		}
		switch key {
		case "policies":
			var parsed any
			if err := json.Unmarshal([]byte(value), &parsed); err != nil {
				return fmt.Errorf("GraphQL API value %q is not valid JSON: %w", key, err)
			}
			definition[key] = parsed
		case "upstream":
			definition[key] = map[string]any{"main": map[string]string{"url": value}}
		default:
			definition[key] = value
		}
	}
	// The default inline schema goes with the inline source.
	if _, given := overrides["sdl"]; !given && definition["schemaSource"] != "inline" {
		delete(definition, "sdl")
	}
	return nil
}

// graphQLForm is the multipart form the GraphQL API routes take: the metadata as JSON.
func graphQLForm(definition map[string]any) ([]byte, string, error) {
	metadata, err := json.Marshal(definition)
	if err != nil {
		return nil, "", err
	}
	return multipartFields([][2]string{{"metadata", string(metadata)}})
}

// createGraphQLAPI creates the default synthetic GraphQL API. The control plane accepts a GraphQL
// API only as a multipart form whose metadata part carries the definition.
func (s *Steps) createGraphQLAPI(ctx context.Context) error {
	return s.createGraphQLAPIAs(ctx, nil)
}

func (s *Steps) createGraphQLAPIWithValues(ctx context.Context, table *godog.Table) error {
	overrides, err := tableValues(table)
	if err != nil {
		return err
	}
	return s.createGraphQLAPIAs(ctx, overrides)
}

// createGraphQLAPIAs creates the synthetic GraphQL API with the overrides applied and keeps its
// metadata. It is registered for cleanup before the request.
func (s *Steps) createGraphQLAPIAs(ctx context.Context, overrides map[string]string) error {
	if err := generateResourceAndStore(ctx, "syn-graphql", keyGraphQLAPIID); err != nil {
		return err
	}
	id, err := requiredValue(ctx, keyGraphQLAPIID)
	if err != nil {
		return err
	}
	definition, err := newGraphQLDefinition(ctx, id)
	if err != nil {
		return err
	}
	if err := applyGraphQLOverrides(ctx, definition, overrides, graphQLCreateKeys); err != nil {
		return err
	}
	resp, err := s.sendGraphQLCreation(ctx, id, definition)
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return responseError(resp)
	}
	return storeDefinition(ctx, graphQLAPI, definition)
}

// sendGraphQLCreation registers a GraphQL API for cleanup, sends its creation and publishes the
// answer. A refused creation is deregistered.
func (s *Steps) sendGraphQLCreation(ctx context.Context, id string, definition map[string]any) (*httpx.Response, error) {
	body, contentType, err := graphQLForm(definition)
	if err != nil {
		return nil, err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindGraphQLAPI, ID: id, Actor: resourceActor, Description: "GraphQL API " + id,
	}); err != nil {
		return nil, fmt.Errorf("registering GraphQL API %q for cleanup: %w", id, err)
	}
	resp, err := s.doAuthenticatedContent(ctx, http.MethodPost, graphQLAPIsCollection, contentType, body)
	if err != nil {
		return nil, err
	}
	if refusedCreate(resp) {
		deregister(ctx, cleanup.KindGraphQLAPI, id)
	}
	return resp, nil
}

// submitGraphQLAPI sends a GraphQL API creation request with the values a table gives and leaves
// the answer for the scenario.
func (s *Steps) submitGraphQLAPI(ctx context.Context, table *godog.Table) error {
	overrides, err := tableValues(table)
	if err != nil {
		return err
	}
	if err := generateResourceAndStore(ctx, "syn-graphql", keySubmittedGraphQLID); err != nil {
		return err
	}
	id, err := requiredValue(ctx, keySubmittedGraphQLID)
	if err != nil {
		return err
	}
	definition, err := newGraphQLDefinition(ctx, id)
	if err != nil {
		return err
	}
	if err := applyGraphQLOverrides(ctx, definition, overrides, graphQLSubmitKeys); err != nil {
		return err
	}
	_, err = s.sendGraphQLCreation(ctx, id, definition)
	return err
}

// updateGraphQLAPI replaces the synthetic GraphQL API's metadata with the stored one plus the
// overrides.
func (s *Steps) updateGraphQLAPI(ctx context.Context, table *godog.Table) error {
	overrides, err := tableValues(table)
	if err != nil {
		return err
	}
	id, err := requiredValue(ctx, keyGraphQLAPIID)
	if err != nil {
		return err
	}
	definition, err := s.storedDefinition(ctx, graphQLAPI)
	if err != nil {
		return err
	}
	if err := applyGraphQLOverrides(ctx, definition, overrides, graphQLUpdateKeys); err != nil {
		return err
	}
	body, contentType, err := graphQLForm(definition)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticatedContent(ctx, http.MethodPut, graphQLAPIsCollection+"/"+url.PathEscape(id), contentType, body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	return storeDefinition(ctx, graphQLAPI, definition)
}

func (s *Steps) sendGraphQLRequest(ctx context.Context, request string, want int) error {
	return s.sendGraphQL(ctx, request, nil, want)
}

// sendGraphQLRequestWithAPIKey sends a GraphQL request with the stored API key in the named header.
func (s *Steps) sendGraphQLRequestWithAPIKey(ctx context.Context, request, header string, want int) error {
	key, err := requiredValue(ctx, keyAPIKey)
	if err != nil {
		return err
	}
	header = strings.TrimSpace(header)
	if header == "" {
		return fmt.Errorf("sending request: API key header name is required")
	}
	return s.sendGraphQL(ctx, request, map[string]string{header: key}, want)
}

// sendGraphQL posts a GraphQL request to the synthetic GraphQL API's context on the active
// gateway until the gateway answers with the status.
func (s *Steps) sendGraphQL(ctx context.Context, request string, headers map[string]string, want int) error {
	call, err := s.graphQLCall(ctx, request, headers)
	if err != nil {
		return err
	}
	return s.sendRequestUntilStatus(ctx, call, want)
}

// sendGraphQLUntilAnsweredBy posts a GraphQL request to the synthetic GraphQL API until the named
// backend answers it, so an answer from a deployment being replaced is not taken for the new one.
func (s *Steps) sendGraphQLUntilAnsweredBy(ctx context.Context, request, source string) error {
	call, err := s.graphQLCall(ctx, request, nil)
	if err != nil {
		return err
	}
	call, err = s.prepareDataPlaneRequest(ctx, call)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo)},
		func(ctx context.Context) (*httpx.Response, error) {
			response, sendErr := s.funnel.Send(ctx, call)
			if sendErr != nil {
				return nil, retry.Transient(sendErr)
			}
			return response, nil
		},
		func(response *httpx.Response) bool {
			return response != nil && response.StatusCode == http.StatusOK && graphQLAnswerSource(response.Body, request) == source
		},
		fmt.Sprintf("waiting for the %s to answer %s %s", source, call.Method, call.URL))
}

// graphQLCall is the request that posts a GraphQL request to the synthetic GraphQL API's context
// on the active gateway. The request text is kept for the assertions on the answer.
func (s *Steps) graphQLCall(ctx context.Context, request string, headers map[string]string) (httpx.Request, error) {
	endpoint, err := requiredValue(ctx, keyGatewayEndpoint)
	if err != nil {
		return httpx.Request{}, err
	}
	definition, err := s.storedDefinition(ctx, graphQLAPI)
	if err != nil {
		return httpx.Request{}, err
	}
	apiContext, _ := definition["context"].(string)
	if apiContext == "" {
		return httpx.Request{}, fmt.Errorf("the synthetic GraphQL API has no context")
	}
	body, err := json.Marshal(map[string]string{"query": request})
	if err != nil {
		return httpx.Request{}, err
	}
	if err := tcontext.Set(ctx, keyGraphQLRequest, request); err != nil {
		return httpx.Request{}, err
	}
	all := map[string]string{"Content-Type": "application/json"}
	for name, value := range headers {
		all[name] = value
	}
	return httpx.Request{
		Method: http.MethodPost, URL: strings.TrimRight(endpoint, "/") + apiContext, Headers: all, Body: body,
	}, nil
}

// graphQLAnswerSource names the backend an answer to a GraphQL request came from: the GraphQL
// backend answers with the request body, the echo backend with a description of the request. It
// is empty for any other answer.
func graphQLAnswerSource(body []byte, request string) string {
	var answer struct {
		Query  string `json:"query"`
		Method string `json:"method"`
		Path   string `json:"path"`
	}
	if json.Unmarshal(body, &answer) != nil {
		return ""
	}
	switch {
	case answer.Query == request && answer.Method == "":
		return "GraphQL backend"
	case answer.Method == http.MethodPost && answer.Path != "" && answer.Query == "":
		return "echo backend"
	default:
		return ""
	}
}

// assertGraphQLResponseSource checks which backend answered the last GraphQL request.
func (s *Steps) assertGraphQLResponseSource(ctx context.Context, source string) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	request, err := requiredValue(ctx, keyGraphQLRequest)
	if err != nil {
		return err
	}
	if got := graphQLAnswerSource(response.Body, request); got != source {
		return fmt.Errorf("the answer to %q did not come from the %s: %s", request, source, response.Describe())
	}
	return nil
}

// graphQLSchemaOf reads the schema the platform resolved for the GraphQL API a ref points at.
func (s *Steps) graphQLSchemaOf(ctx context.Context, ref apiRef) (string, error) {
	id, err := requiredValue(ctx, ref.id)
	if err != nil {
		return "", err
	}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, graphQLAPIsCollection+"/"+url.PathEscape(id)+"/sdl", nil)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", responseError(resp)
	}
	var answer struct {
		SDL *string `json:"sdl"`
	}
	if err := json.Unmarshal(resp.Body, &answer); err != nil || answer.SDL == nil {
		return "", fmt.Errorf("the GraphQL API has no schema field: %s", resp.Describe())
	}
	return *answer.SDL, nil
}

// sameSchema compares two SDL texts by their tokens, as the platform may lay a schema out anew.
func sameSchema(a, b string) bool {
	return strings.Join(strings.Fields(a), " ") == strings.Join(strings.Fields(b), " ")
}

func (s *Steps) assertGraphQLSchema(ctx context.Context, which, want string) error {
	got, err := s.graphQLSchemaOf(ctx, graphQLRefFor(which))
	if err != nil {
		return err
	}
	if !sameSchema(got, want) {
		return fmt.Errorf("the %s GraphQL API has the schema %q, want %q", which, got, want)
	}
	return nil
}

func (s *Steps) assertEmptyGraphQLSchema(ctx context.Context, which string) error {
	got, err := s.graphQLSchemaOf(ctx, graphQLRefFor(which))
	if err != nil {
		return err
	}
	if strings.TrimSpace(got) != "" {
		return fmt.Errorf("the %s GraphQL API has the schema %q, want none", which, got)
	}
	return nil
}

// assertSubmittedGraphQLAbsent checks that a refused GraphQL API creation left no API behind.
func (s *Steps) assertSubmittedGraphQLAbsent(ctx context.Context) error {
	id, err := requiredValue(ctx, keySubmittedGraphQLID)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, graphQLAPIsCollection+"/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("expected GraphQL API %s not to exist, got %s", id, resp.Describe())
	}
	return nil
}

// validateGraphQLSchema asks the platform to resolve an inline schema without creating an API.
func (s *Steps) validateGraphQLSchema(ctx context.Context, schema string) error {
	if err := tcontext.Set(ctx, keyValidatedGraphQLText, schema); err != nil {
		return err
	}
	body, contentType, err := graphQLForm(map[string]any{"schemaSource": "inline", "sdl": schema})
	if err != nil {
		return err
	}
	_, err = s.doAuthenticatedContent(ctx, http.MethodPost, graphQLAPIsCollection+"/validate-schema", contentType, body)
	return err
}

// assertGraphQLValidation checks the validation answer for the schema last validated: a schema
// that resolves comes back as it was given, one that does not comes back empty with its errors.
func (s *Steps) assertGraphQLValidation(ctx context.Context, outcome string) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return responseError(response)
	}
	schema, err := requiredValue(ctx, keyValidatedGraphQLText)
	if err != nil {
		return err
	}
	var answer struct {
		Resolved  *bool  `json:"resolved"`
		SDL       string `json:"sdl"`
		SDLErrors []any  `json:"sdlErrors"`
	}
	if err := json.Unmarshal(response.Body, &answer); err != nil || answer.Resolved == nil {
		return fmt.Errorf("the answer is not a schema validation result: %s", response.Describe())
	}
	if outcome == "resolved" {
		if !*answer.Resolved || !sameSchema(answer.SDL, schema) {
			return fmt.Errorf("the schema %q was not resolved as given: %s", schema, response.Describe())
		}
		return nil
	}
	if *answer.Resolved || strings.TrimSpace(answer.SDL) != "" || len(answer.SDLErrors) == 0 {
		return fmt.Errorf("the schema %q was not reported as unresolved with its errors: %s", schema, response.Describe())
	}
	return nil
}

// createGraphQLAPIKey issues an API key for the synthetic GraphQL API and keeps its value in
// scenario state only; the answer is not published so the key never reaches a step's response slot.
func (s *Steps) createGraphQLAPIKey(ctx context.Context) error {
	id, err := requiredValue(ctx, keyGraphQLAPIID)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodPost, graphQLAPIsCollection+"/"+url.PathEscape(id)+"/api-keys",
		map[string]string{"displayName": "Synthetic key for " + id})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("creating an API key for %s: unexpected status %d", id, resp.StatusCode)
	}
	key, err := stringField(resp, "apiKey")
	if err != nil {
		return fmt.Errorf("creating an API key for %s: %w", id, err)
	}
	keyID, err := stringField(resp, "keyId")
	if err != nil {
		return fmt.Errorf("creating an API key for %s: %w", id, err)
	}
	if err := tcontext.Set(ctx, keyGraphQLKeyID, keyID); err != nil {
		return err
	}
	return tcontext.Set(ctx, keyAPIKey, key)
}

func (s *Steps) revokeGraphQLAPIKey(ctx context.Context) error {
	id, err := requiredValue(ctx, keyGraphQLAPIID)
	if err != nil {
		return err
	}
	keyID, err := requiredValue(ctx, keyGraphQLKeyID)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodDelete,
		graphQLAPIsCollection+"/"+url.PathEscape(id)+"/api-keys/"+url.PathEscape(keyID), nil)
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return responseError(resp)
	}
	return nil
}
