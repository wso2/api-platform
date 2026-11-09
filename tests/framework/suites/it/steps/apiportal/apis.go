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

package apiportal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// noDefinition names the definition choice that sends the metadata part alone.
const noDefinition = "none"

// defaultAPICollection is where the template steps send an API when the step names no collection.
const defaultAPICollection = "/apis"

// apiCollections are the collections an API Portal API template can be sent to, each with the
// cleanup kind that deletes what it creates. A REST, GraphQL, SOAP, WebSocket or WebSub API lives
// under /apis and an MCP server under /mcp-servers; the portal resolves a handle only in its own
// collection.
var apiCollections = map[string]cleanup.Kind{
	"/apis":        portalAPIKind,
	"/mcp-servers": portalMCPServerKind,
}

// registerAPITemplateSteps binds the steps that create and update API Portal APIs from the
// canonical api-portal-api template. A values-table entry of null leaves that field out of the
// metadata, which is how a scenario sends a request without a field the template defaults.
func (s *Steps) registerAPITemplateSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I create an API Portal API from "([^"]*)"(?: at "([^"]*)")? as "([^"]*)"(?: with definition "([^"]*)")? with values:$`,
		s.createAPIFromTemplate)
	sc.Step(`^I update the API Portal API "([^"]*)" from "([^"]*)"(?: at "([^"]*)")? as "([^"]*)"(?: with definition "([^"]*)")? with values:$`,
		s.updateAPIFromTemplate)
}

// createAPIFromTemplate POSTs the rendered metadata and definition to the named collection,
// /apis when none is named. The response is published for the assertions that follow, and any
// API the portal accepted is registered for scenario cleanup before the step returns.
func (s *Steps) createAPIFromTemplate(ctx context.Context, template, collection, role, definition string, table *godog.Table) error {
	collection, kind, err := apiCollection(collection)
	if err != nil {
		return err
	}
	metadata, err := s.renderAPIMetadata(ctx, template, table)
	if err != nil {
		return err
	}
	if err := s.sendAPIMultipart(ctx, http.MethodPost, collection, role, definition, metadata); err != nil {
		return err
	}
	response, err := publishedResponse(ctx)
	if err != nil {
		return err
	}
	if !response.Succeeded() {
		// A refused create stores nothing; the steps that follow assert the refusal.
		return nil
	}
	id, err := createdAPIID(response.Body, metadata)
	if err != nil {
		return err
	}
	return s.registerPortalResource(ctx, kind, id, cleanup.ScenarioScope,
		portalCleanupRequest{method: http.MethodDelete, path: collection})
}

// updateAPIFromTemplate PUTs the rendered metadata and definition over the named API in the named
// collection, /apis when none is named.
func (s *Steps) updateAPIFromTemplate(ctx context.Context, apiID, template, collection, role, definition string, table *godog.Table) error {
	collection, _, err := apiCollection(collection)
	if err != nil {
		return err
	}
	resolved, err := stepscommon.Expand(ctx, apiID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(resolved) == "" {
		return fmt.Errorf("API Portal API id is required")
	}
	metadata, err := s.renderAPIMetadata(ctx, template, table)
	if err != nil {
		return err
	}
	return s.sendAPIMultipart(ctx, http.MethodPut, collection+"/"+url.PathEscape(resolved), role, definition, metadata)
}

// apiCollection resolves the collection a template step names and the cleanup kind for what it
// creates. Only the API collections are accepted, so a step cannot send API metadata elsewhere.
func apiCollection(collection string) (string, cleanup.Kind, error) {
	collection = strings.TrimSpace(collection)
	if collection == "" {
		collection = defaultAPICollection
	}
	kind, ok := apiCollections[collection]
	if !ok {
		return "", cleanup.Kind{}, fmt.Errorf("API Portal APIs are sent to /apis or /mcp-servers, not %q", collection)
	}
	return collection, kind, nil
}

func (s *Steps) renderAPIMetadata(ctx context.Context, template string, table *godog.Table) ([]byte, error) {
	rendered, err := stepscommon.RenderResourceTemplateJSON(ctx, s.featureRoot, template, table)
	if err != nil {
		return nil, err
	}
	return withoutNullFields(rendered)
}

func (s *Steps) sendAPIMultipart(ctx context.Context, method, path, role, definition string, metadata []byte) error {
	content, filename, contentType, err := apiDefinitionPart(ctx, definition)
	if err != nil {
		return err
	}
	return s.sendPortalMultipart(ctx, method, path, role, metadata, content, filename, contentType)
}

// apiDefinitionPart resolves the definition a step names. It defaults to the REST fixture, and
// "none" sends no definition part at all.
func apiDefinitionPart(ctx context.Context, definition string) ([]byte, string, string, error) {
	resolved, err := stepscommon.Expand(ctx, definition)
	if err != nil {
		return nil, "", "", err
	}
	switch strings.ToLower(strings.TrimSpace(resolved)) {
	case "":
		return portalDefinition("rest")
	case noDefinition:
		return nil, "", "", nil
	default:
		return portalDefinition(resolved)
	}
}

// withoutNullFields removes every null-valued field from a JSON object, at any depth.
func withoutNullFields(document []byte) ([]byte, error) {
	var value map[string]any
	if err := json.Unmarshal(document, &value); err != nil {
		return nil, fmt.Errorf("API Portal metadata is not a JSON object: %w", err)
	}
	encoded, err := json.Marshal(dropNulls(value))
	if err != nil {
		return nil, fmt.Errorf("encoding API Portal metadata: %w", err)
	}
	return encoded, nil
}

func dropNulls(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if child == nil {
				delete(typed, key)
				continue
			}
			typed[key] = dropNulls(child)
		}
		return typed
	case []any:
		for i, child := range typed {
			typed[i] = dropNulls(child)
		}
		return typed
	default:
		return value
	}
}

// createdAPIID returns the id the portal reports for a created API, falling back to the id
// the request sent.
func createdAPIID(body, metadata []byte) (string, error) {
	var created struct {
		ID string `json:"id"`
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &created); err != nil {
			return "", fmt.Errorf("decoding the created API Portal API: %w", err)
		}
	}
	if created.ID != "" {
		return created.ID, nil
	}
	var requested struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(metadata, &requested); err != nil {
		return "", fmt.Errorf("decoding API Portal metadata: %w", err)
	}
	if requested.ID == "" {
		return "", fmt.Errorf("the API Portal created an API without reporting its id")
	}
	return requested.ID, nil
}

func publishedResponse(ctx context.Context) (*httpx.Response, error) {
	value, ok := tcontext.Get(ctx, httpx.ResponseKey)
	if !ok {
		return nil, fmt.Errorf("no API Portal response was published")
	}
	response, ok := value.(*httpx.Response)
	if !ok || response == nil {
		return nil, fmt.Errorf("the published API Portal response has type %T", value)
	}
	return response, nil
}
