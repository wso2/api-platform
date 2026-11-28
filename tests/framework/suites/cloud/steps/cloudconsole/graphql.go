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
	"slices"
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
	keyOtherAPIKey          = "cloudOtherAPIKey"
	keyOtherGraphQLKeyID    = "cloudOtherGraphQLAPIKeyID"
	keyValidatedGraphQLText = "cloudValidatedGraphQLSchema"
	// keyValidatedGraphQLSource is the schemaSource of the last validation request.
	keyValidatedGraphQLSource = "cloudValidatedGraphQLSource"

	// graphQLBackendEnv overrides the backend the synthetic GraphQL APIs call.
	graphQLBackendEnv = "CLOUD_IT_GRAPHQL_BACKEND_URL"
	// defaultGraphQLBackend is the testbench GraphQL backend, which answers with the body it received.
	defaultGraphQLBackend = "https://development-wso2cloud.gateway.dev.cloud.wso2.com/apip-testbench-mocks/graphql-backend"

	graphQLSchema = "type Query { hello: String }"
)

// submittedGraphQLAPI is a GraphQL API a creation request was sent for without requiring that it
// be created.
var submittedGraphQLAPI = apiRef{id: keySubmittedGraphQLID, collection: graphQLAPIsCollection}

// kindGraphQLAPIKey is an API key issued for a GraphQL API, revoked before the API is deleted.
var kindGraphQLAPIKey = cleanup.Kind{Name: "cloud-graphql-api-key", Order: 40}

// GraphQL API fields a values table may set, by the step that accepts them. policies carries JSON,
// upstream the main upstream URL, and the rest text.
var (
	graphQLCreateKeys = map[string]bool{"policies": true, "sdl": true}
	graphQLSubmitKeys = map[string]bool{"schemaSource": true, "sdl": true, "sdlUrl": true}
	graphQLUpdateKeys = map[string]bool{"upstream": true, "sdl": true}
)

// registerGraphQLSteps binds the steps that create, change and call a GraphQL API.
func (s *Steps) registerGraphQLSteps(sc stepRegistrar) {
	sc.Step(`^I create a synthetic GraphQL API in the APIP test project with:$`, s.createGraphQLAPIWithValues)
	sc.Step(`^I submit a synthetic GraphQL API creation request with:$`, s.submitGraphQLAPI)
	sc.Step(`^I update the synthetic GraphQL API with:$`, s.updateGraphQLAPI)
	sc.Step(`^I send the GraphQL request "([^"]*)" to the synthetic GraphQL API until status (\d+)$`, s.sendGraphQLRequest)
	sc.Step(`^I send the GraphQL request "([^"]*)" to the synthetic GraphQL API with header "([^"]*)" set to the (API key|other API key) until status (\d+)$`,
		s.sendGraphQLRequestWithAPIKey)
	sc.Step(`^I send the GraphQL request "([^"]*)" to the synthetic GraphQL API until the (GraphQL backend|echo backend) answers$`,
		s.sendGraphQLUntilAnsweredBy)
	sc.Step(`^the GraphQL response should come from the (GraphQL backend|echo backend)$`, s.assertGraphQLResponseSource)
	sc.Step(`^the (synthetic|submitted) GraphQL API should have the schema "([^"]*)"$`, s.assertGraphQLSchema)
	sc.Step(`^the (synthetic|submitted) GraphQL API should have an empty schema$`, s.assertEmptyGraphQLSchema)
	sc.Step(`^the submitted GraphQL API should (exist|not exist)$`, s.assertSubmittedGraphQL)
	sc.Step(`^I validate the GraphQL schema "([^"]*)"$`, s.validateGraphQLSchema)
	sc.Step(`^I validate a GraphQL schema with:$`, s.validateGraphQLSchemaWith)
	sc.Step(`^the GraphQL schema should be reported as (resolved|not resolved|not resolved with SDL errors)$`, s.assertGraphQLValidation)
	sc.Step(`^I create (an|another) API key for the synthetic GraphQL API$`, s.createGraphQLAPIKey)
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
// answer. The API stays registered whatever the answer, since a creation can fail after the API is
// stored; cleanup treats an absent API as deleted.
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
	return s.doAuthenticatedContent(ctx, http.MethodPost, graphQLAPIsCollection, contentType, body)
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

// graphQLKeySlots names the scenario keys that hold an API key's value and id: the first key
// created, or the other one.
func graphQLKeySlots(which string) (value, id string) {
	if which == "another" || which == "other API key" {
		return keyOtherAPIKey, keyOtherGraphQLKeyID
	}
	return keyAPIKey, keyGraphQLKeyID
}

// sendGraphQLRequestWithAPIKey sends a GraphQL request with a stored API key in the named header.
func (s *Steps) sendGraphQLRequestWithAPIKey(ctx context.Context, request, header, which string, want int) error {
	slot, _ := graphQLKeySlots(which)
	key, err := requiredValue(ctx, slot)
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
// backend answers with the request body, the echo backend with a description of the request whose
// JSON body is that request. It is empty for any other answer.
func graphQLAnswerSource(body []byte, request string) string {
	var answer struct {
		Query  *string `json:"query"`
		Method string  `json:"method"`
		Path   string  `json:"path"`
		JSON   struct {
			Query *string `json:"query"`
		} `json:"json"`
	}
	if json.Unmarshal(body, &answer) != nil {
		return ""
	}
	switch {
	case answer.Query != nil && *answer.Query == request && answer.Method == "":
		return "GraphQL backend"
	case answer.Query == nil && answer.Method == http.MethodPost && answer.Path != "" &&
		answer.JSON.Query != nil && *answer.JSON.Query == request:
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
	return slices.Equal(graphQLTokens(a), graphQLTokens(b))
}

// graphQLPunctuators are the GraphQL punctuators of one character.
const graphQLPunctuators = "!$&()[]{}:=@|"

// graphQLTokens splits SDL into its lexical tokens. Whitespace, commas and comments between tokens
// are dropped; strings are kept exactly as written.
func graphQLTokens(sdl string) []string {
	var tokens []string
	for i := 0; i < len(sdl); {
		c := sdl[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',':
			i++
		case c == '#':
			for i < len(sdl) && sdl[i] != '\n' && sdl[i] != '\r' {
				i++
			}
		case strings.HasPrefix(sdl[i:], `"""`):
			end := blockStringEnd(sdl, i+3)
			tokens = append(tokens, sdl[i:end])
			i = end
		case c == '"':
			j := i + 1
			for j < len(sdl) && sdl[j] != '"' && sdl[j] != '\n' {
				if sdl[j] == '\\' {
					j++
				}
				j++
			}
			end := min(j+1, len(sdl))
			tokens = append(tokens, sdl[i:end])
			i = end
		case strings.HasPrefix(sdl[i:], "..."):
			tokens = append(tokens, "...")
			i += 3
		case strings.IndexByte(graphQLPunctuators, c) >= 0:
			tokens = append(tokens, string(c))
			i++
		case c == '-' || isDigit(c):
			j := graphQLNumberEnd(sdl, i)
			tokens = append(tokens, sdl[i:j])
			i = j
		default:
			j := i + 1
			for isGraphQLNameByte(c) && j < len(sdl) && isGraphQLNameByte(sdl[j]) {
				j++
			}
			tokens = append(tokens, sdl[i:j])
			i = j
		}
	}
	return tokens
}

// graphQLNumberEnd is the index just past the number that starts at from: an optional minus, its
// integer digits, an optional fraction and an optional exponent with its own sign.
func graphQLNumberEnd(sdl string, from int) int {
	j := from
	if sdl[j] == '-' {
		j++
	}
	digits := func() {
		for j < len(sdl) && isDigit(sdl[j]) {
			j++
		}
	}
	digits()
	if j < len(sdl) && sdl[j] == '.' {
		j++
		digits()
	}
	if j < len(sdl) && (sdl[j] == 'e' || sdl[j] == 'E') {
		j++
		if j < len(sdl) && (sdl[j] == '+' || sdl[j] == '-') {
			j++
		}
		digits()
	}
	return j
}

// blockStringEnd is the index just past the block string whose content starts at from: after its
// closing triple quote, which an escaped triple quote does not end, or the end of the text.
func blockStringEnd(sdl string, from int) int {
	for i := from; i < len(sdl); i++ {
		if strings.HasPrefix(sdl[i:], `\"""`) {
			i += 3
			continue
		}
		if strings.HasPrefix(sdl[i:], `"""`) {
			return i + 3
		}
	}
	return len(sdl)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// isGraphQLNameByte reports whether a byte belongs to a GraphQL name.
func isGraphQLNameByte(c byte) bool {
	return c == '_' || isDigit(c) || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
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

// assertSubmittedGraphQL checks that the GraphQL API of the last creation request exists and is
// read back under its id, or that a refused one left no API behind.
func (s *Steps) assertSubmittedGraphQL(ctx context.Context, presence string) error {
	id, err := requiredValue(ctx, keySubmittedGraphQLID)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, graphQLAPIsCollection+"/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	if presence == "not exist" {
		if resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("expected GraphQL API %s not to exist, got %s", id, resp.Describe())
		}
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected GraphQL API %s to exist, got %s", id, resp.Describe())
	}
	if got, _ := stringField(resp, "id"); got != id {
		return fmt.Errorf("reading GraphQL API %s returned the API %q", id, got)
	}
	return nil
}

// graphQLValidateKeys are the values a schema validation request may take from a table:
// schemaSource, the SDL URL, and the upstream URL an introspection reads.
var graphQLValidateKeys = map[string]bool{"schemaSource": true, "sdlUrl": true, "upstreamUrl": true}

// validateGraphQLSchemaWith asks the platform to resolve a schema from the source the table names,
// without creating an API. A resolved outcome is checked for a schema, not for the text the URL
// served, which the scenario does not know.
func (s *Steps) validateGraphQLSchemaWith(ctx context.Context, table *godog.Table) error {
	// A request that fails before it is sent must not leave an earlier answer to be asserted.
	httpx.ClearPublished(ctx)
	values, err := tableValues(table)
	if err != nil {
		return err
	}
	request := map[string]any{}
	for key, raw := range values {
		if !graphQLValidateKeys[key] {
			return fmt.Errorf("unsupported GraphQL schema validation value %q", key)
		}
		value, err := expandCloudValue(ctx, raw)
		if err != nil {
			return err
		}
		if key == "upstreamUrl" {
			request["upstream"] = map[string]any{"main": map[string]string{"url": value}}
			continue
		}
		request[key] = value
	}
	if request["schemaSource"] == nil {
		return fmt.Errorf("a schema validation request names its schemaSource")
	}
	described, err := json.Marshal(request)
	if err != nil {
		return err
	}
	for key, value := range map[string]any{keyValidatedGraphQLText: string(described), keyValidatedGraphQLSource: request["schemaSource"]} {
		if err := tcontext.Set(ctx, key, value); err != nil {
			return err
		}
	}
	body, contentType, err := graphQLForm(request)
	if err != nil {
		return err
	}
	_, err = s.doAuthenticatedContent(ctx, http.MethodPost, graphQLAPIsCollection+"/validate-schema", contentType, body)
	return err
}

// validateGraphQLSchema asks the platform to resolve an inline schema without creating an API.
func (s *Steps) validateGraphQLSchema(ctx context.Context, schema string) error {
	for key, value := range map[string]string{keyValidatedGraphQLText: schema, keyValidatedGraphQLSource: "inline"} {
		if err := tcontext.Set(ctx, key, value); err != nil {
			return err
		}
	}
	body, contentType, err := graphQLForm(map[string]any{"schemaSource": "inline", "sdl": schema})
	if err != nil {
		return err
	}
	_, err = s.doAuthenticatedContent(ctx, http.MethodPost, graphQLAPIsCollection+"/validate-schema", contentType, body)
	return err
}

// assertGraphQLValidation checks the validation answer for the schema last validated: an inline
// schema that resolves comes back as it was given, a fetched one comes back as a schema, and one
// that does not resolve comes back empty with a diagnostic. "with SDL errors" requires the errors
// of a text that was fetched or given: the platform answers a failed fetch with a message only.
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
		Message   string `json:"message"`
		SDLErrors []*struct {
			Message string `json:"message"`
		} `json:"sdlErrors"`
	}
	if err := json.Unmarshal(response.Body, &answer); err != nil || answer.Resolved == nil {
		return fmt.Errorf("the answer is not a schema validation result: %s", response.Describe())
	}
	if outcome == "resolved" {
		source, err := requiredValue(ctx, keyValidatedGraphQLSource)
		if err != nil {
			return err
		}
		if source != "inline" {
			if !*answer.Resolved || len(graphQLTokens(answer.SDL)) == 0 {
				return fmt.Errorf("the schema from %s was not resolved: %s", schema, response.Describe())
			}
			return nil
		}
		if !*answer.Resolved || !sameSchema(answer.SDL, schema) {
			return fmt.Errorf("the schema %q was not resolved as given: %s", schema, response.Describe())
		}
		return nil
	}
	// An SDL that was fetched but does not parse is diagnosed in sdlErrors; an endpoint that could
	// not be introspected is diagnosed in the answer's message.
	sdlErrors := slices.ContainsFunc(answer.SDLErrors, func(issue *struct {
		Message string `json:"message"`
	}) bool {
		return issue != nil && strings.TrimSpace(issue.Message) != ""
	})
	diagnosed := sdlErrors || (outcome == "not resolved" && strings.TrimSpace(answer.Message) != "")
	if *answer.Resolved || strings.TrimSpace(answer.SDL) != "" || !diagnosed {
		return fmt.Errorf("the schema %q was not reported as unresolved with a diagnostic: %s", schema, response.Describe())
	}
	return nil
}

// createGraphQLAPIKey issues an API key for the synthetic GraphQL API, as the first key or as
// another one, and keeps its value in scenario state only; the answer is not published so the key
// never reaches a step's response slot. The key is registered for cleanup before its value is read.
func (s *Steps) createGraphQLAPIKey(ctx context.Context, which string) error {
	valueSlot, idSlot := graphQLKeySlots(which)
	id, err := requiredValue(ctx, keyGraphQLAPIID)
	if err != nil {
		return err
	}
	// The platform names a key after its display name, so each key of the API gets its own.
	displayName := "Synthetic key for " + id
	if valueSlot == keyOtherAPIKey {
		displayName = "Other synthetic key for " + id
	}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodPost, graphQLAPIsCollection+"/"+url.PathEscape(id)+"/api-keys",
		map[string]string{"displayName": displayName})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("creating an API key for %s: unexpected status %d", id, resp.StatusCode)
	}
	keyID, err := stringField(resp, "keyId")
	if err != nil {
		return fmt.Errorf("creating an API key for %s: %w", id, err)
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: kindGraphQLAPIKey, ID: graphQLKeyResource(id, keyID), Actor: resourceActor,
		Description: "API key " + keyID + " of GraphQL API " + id,
	}); err != nil {
		if revokeErr := s.revokeGraphQLKey(ctx, id, keyID); revokeErr != nil {
			return fmt.Errorf("registering the API key for cleanup: %w; revoking it: %v", err, revokeErr)
		}
		return fmt.Errorf("registering the API key for cleanup: %w", err)
	}
	if err := tcontext.Set(ctx, idSlot, keyID); err != nil {
		return err
	}
	key, err := stringField(resp, "apiKey")
	if err != nil {
		return fmt.Errorf("creating an API key for %s: %w", id, err)
	}
	return tcontext.Set(ctx, valueSlot, key)
}

// revokeGraphQLKey revokes an API key of a GraphQL API. A key the platform no longer has counts as
// revoked; any other answer is reported by its status only, never with the body.
func (s *Steps) revokeGraphQLKey(ctx context.Context, apiID, keyID string) error {
	resp, err := s.sendGraphQLKeyRevoke(ctx, apiID, keyID)
	if err != nil {
		return err
	}
	if !resp.Succeeded() && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("revoking API key %s of GraphQL API %s: unexpected status %d", keyID, apiID, resp.StatusCode)
	}
	return nil
}

// graphQLKeyResource is the cleanup id of an API key of a GraphQL API.
func graphQLKeyResource(apiID, keyID string) string {
	return apiID + publicationIDGlue + keyID
}

// revokeGraphQLKeyRequest is the request that revokes the GraphQL API key a cleanup id names. A
// key the platform no longer has counts as revoked.
func revokeGraphQLKeyRequest(id string) (string, string, func(*httpx.Response) bool, error) {
	apiID, keyID, found := strings.Cut(id, publicationIDGlue)
	if !found || apiID == "" || keyID == "" {
		return "", "", nil, fmt.Errorf("GraphQL API key cleanup id %q does not name an API and a key", id)
	}
	return http.MethodDelete, graphQLKeyPath(apiID, keyID),
		func(resp *httpx.Response) bool { return resp.Succeeded() || resp.StatusCode == http.StatusNotFound }, nil
}

func graphQLKeyPath(apiID, keyID string) string {
	return graphQLAPIsCollection + "/" + url.PathEscape(apiID) + "/api-keys/" + url.PathEscape(keyID)
}

// sendGraphQLKeyRevoke revokes an API key of a GraphQL API without publishing the answer.
func (s *Steps) sendGraphQLKeyRevoke(ctx context.Context, apiID, keyID string) (*httpx.Response, error) {
	return s.doAuthenticatedRaw(ctx, http.MethodDelete, graphQLKeyPath(apiID, keyID), nil)
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
	resp, err := s.sendGraphQLKeyRevoke(ctx, id, keyID)
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return responseError(resp)
	}
	deregister(ctx, kindGraphQLAPIKey, graphQLKeyResource(id, keyID))
	return nil
}
