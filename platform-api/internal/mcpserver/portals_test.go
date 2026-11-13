/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// portalFake adds the portal and definition calls to fake.
type portalFake struct {
	*fake
	draft         *model.Publication // the draft getDraft returns; nil for none
	savedDraft    *model.Publication
	hasDefinition bool
	definition    *definitionDoc // the API's stored definition; nil for none
	savedDoc      *definitionDoc
	gateways      []api.RESTAPIGatewayResponse
	published     string // "type/id@portal"
	unpublished   string
}

func (p *portalFake) getDefinition(context.Context, caller, *apiType, string) (definitionDoc, error) {
	if p.definition == nil {
		return definitionDoc{}, apperror.NotFound.New()
	}
	return *p.definition, nil
}

func (p *portalFake) listAPIGateways(context.Context, caller, *apiType, string) ([]api.RESTAPIGatewayResponse, error) {
	return p.gateways, nil
}

func (p *portalFake) listPortals(context.Context, caller, page) ([]api.ApiPortalListItem, int, error) {
	desc := "For partners"
	return []api.ApiPortalListItem{{Id: "default", Name: "Default", Url: "https://portal.example.com", Description: &desc}}, 1, nil
}

func (p *portalFake) getDraft(context.Context, caller, *apiType, string, string) (*model.Publication, error) {
	return p.draft, nil
}

func (p *portalFake) saveDraft(_ context.Context, _ caller, _ *apiType, _, _ string, d *model.Publication) error {
	p.savedDraft = d
	return nil
}

func (p *portalFake) hasDraftDefinition(context.Context, caller, *apiType, string, string) (bool, error) {
	return p.hasDefinition, nil
}

func (p *portalFake) saveDraftDefinition(_ context.Context, _ caller, _ *apiType, _, _ string, doc definitionDoc) error {
	p.savedDoc = &doc
	return nil
}

func (p *portalFake) publish(_ context.Context, _ caller, t *apiType, id, portal string) (*model.Publication, error) {
	p.published = t.name + "/" + id + "@" + portal
	url := ""
	if p.savedDraft != nil {
		url = p.savedDraft.ProductionURL
	}
	return &model.Publication{Status: "PUBLISHED", ProductionURL: url}, nil
}

func (p *portalFake) unpublish(_ context.Context, _ caller, t *apiType, id, portal string) error {
	p.unpublished = t.name + "/" + id + "@" + portal
	return nil
}

func newPortalServer(t *testing.T) (*Server, *portalFake) {
	s, f := newTestServer(t)
	p := &portalFake{fake: f}
	s.backend = p
	return s, p
}

// portalScopes may publish REST APIs.
var portalScopes = []string{"ap:rest_api:manage", "ap:api_portal:manage", "ap:api_portal:draft:manage", "ap:api_portal:publish"}

func TestPublishAPI_MakesTheListingTheConsoleWould(t *testing.T) {
	s, p := newPortalServer(t)
	p.api = object{"id": "todos", "displayName": "Todos", "version": "v1", "description": "Todo items",
		"context": "/todos/$version",
		"operations": []any{
			map[string]any{"name": "List todos", "request": map[string]any{"method": "GET", "path": "/"}},
			map[string]any{"name": "Get a todo", "request": map[string]any{"method": "get", "path": "/{id}"}},
		}}
	endpoints := []string{"gw.example.com/"}
	p.gateways = []api.RESTAPIGatewayResponse{
		{DisplayName: "idle", IsDeployed: false, Endpoints: &[]string{"https://idle.example.com"}},
		{DisplayName: "live", IsDeployed: true, Endpoints: &endpoints},
	}

	res, text := call(t, connect(t, s, who(testOrg, allScopes(t, s)...)), "publish_api",
		map[string]any{"apiType": "rest-api", "apiId": "todos", "apiPortalId": "default"})
	require.False(t, res.IsError, text)

	assert.Equal(t, "rest-api/todos@default", p.published)
	require.NotNil(t, p.savedDraft)
	assert.Equal(t, "Todos", p.savedDraft.DisplayName)
	assert.Equal(t, "v1", p.savedDraft.Version)
	assert.Equal(t, "https://gw.example.com/todos/v1", p.savedDraft.ProductionURL, "the first deployed gateway, $version resolved")

	require.NotNil(t, p.savedDoc, "the API had no OpenAPI document, so one is made from its operations")
	assert.Equal(t, "application/json", p.savedDoc.ContentType)
	sd, err := utils.LoadSpecDocument([]byte(p.savedDoc.Content))
	require.NoError(t, err)
	assert.True(t, utils.ValidateSpec(sd).IsValid, "Publish validates the definition the same way: %s", p.savedDoc.Content)

	var out publishOut
	require.NoError(t, json.Unmarshal([]byte(text), &out))
	assert.Equal(t, "PUBLISHED", out.Status)
	assert.Equal(t, "https://gw.example.com/todos/v1", out.ProductionURL)
	assert.Empty(t, out.Message)
}

func TestPublishAPI_KeepsWhatTheConsoleSaved(t *testing.T) {
	s, p := newPortalServer(t)
	p.draft = &model.Publication{DisplayName: "Edited in the console"}
	p.hasDefinition = true

	res, text := call(t, connect(t, s, who(testOrg, allScopes(t, s)...)), "publish_api",
		map[string]any{"apiType": "graphql-api", "apiId": "countries", "apiPortalId": "default"})
	require.False(t, res.IsError, text)
	assert.Nil(t, p.savedDraft, "the console's draft is published as it is")
	assert.Nil(t, p.savedDoc)
	assert.Equal(t, "graphql-api/countries@default", p.published)
}

func TestPublishAPI_SaysWhenTheAPIIsNotDeployed(t *testing.T) {
	s, p := newPortalServer(t)
	p.api = object{"id": "countries", "displayName": "Countries", "version": "v1", "context": "/countries"}
	p.definition = &definitionDoc{Content: testSDL, ContentType: "application/graphql"}

	res, text := call(t, connect(t, s, who(testOrg, allScopes(t, s)...)), "publish_api",
		map[string]any{"apiType": "graphql-api", "apiId": "countries", "apiPortalId": "default"})
	require.False(t, res.IsError, text)
	assert.Equal(t, "", p.savedDraft.ProductionURL)
	assert.Equal(t, "application/graphql", p.savedDoc.ContentType, "the stored SDL is the definition")
	assert.Contains(t, text, "not deployed to a gateway yet")
}

func TestPublishAPI_ServesOnlyTypesWithADefinition(t *testing.T) {
	s, _ := newPortalServer(t)
	cs := connect(t, s, who(testOrg, allScopes(t, s)...))
	res, text := call(t, cs, "publish_api", map[string]any{"apiType": "mcp-proxy", "apiId": "docs", "apiPortalId": "default"})
	assert.True(t, res.IsError)
	assert.Contains(t, text, "mcp-proxy")

	tools, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	for _, tool := range tools.Tools {
		if tool.Name == "publish_api" || tool.Name == "get_api_definition" {
			assert.ElementsMatch(t, []string{"rest-api", "graphql-api"}, enumOf(t, tool.InputSchema, "apiType"), tool.Name)
		}
	}
}

func TestUnpublishAPI_TakesConfirm(t *testing.T) {
	s, p := newPortalServer(t)
	cs := connect(t, s, who(testOrg, allScopes(t, s)...))
	args := map[string]any{"apiType": "rest-api", "apiId": "todos", "apiPortalId": "default"}
	res, _ := call(t, cs, "unpublish_api", args)
	assert.True(t, res.IsError, "refused without confirm")
	assert.Empty(t, p.unpublished)

	args["confirm"] = true
	res, text := call(t, cs, "unpublish_api", args)
	require.False(t, res.IsError, text)
	assert.Equal(t, "rest-api/todos@default", p.unpublished)
}

func TestListAPIPortals(t *testing.T) {
	s, _ := newPortalServer(t)
	res, text := call(t, connect(t, s, who(testOrg, allScopes(t, s)...)), "list_api_portals", nil)
	require.False(t, res.IsError, text)
	var out listPortalsOut
	require.NoError(t, json.Unmarshal([]byte(text), &out))
	assert.Equal(t, []portalSummary{{ID: "default", Name: "Default", URL: "https://portal.example.com", Description: "For partners"}}, out.Items)
}

func TestGetAPIDefinition(t *testing.T) {
	s, p := newPortalServer(t)
	cs := connect(t, s, who(testOrg, allScopes(t, s)...))
	res, text := call(t, cs, "get_api_definition", map[string]any{"apiType": "rest-api", "apiId": "todos"})
	assert.True(t, res.IsError)
	assert.Contains(t, text, "get_api shows its operations", "a REST API with no document says where to look")

	p.definition = &definitionDoc{Content: testSDL, ContentType: "application/graphql"}
	res, text = call(t, cs, "get_api_definition", map[string]any{"apiType": "graphql-api", "apiId": "countries"})
	require.False(t, res.IsError, text)
	var out apiDefinitionOut
	require.NoError(t, json.Unmarshal([]byte(text), &out))
	assert.Equal(t, apiDefinitionOut{APIType: "graphql-api", ID: "countries", ContentType: "application/graphql", Content: testSDL}, out)
}

func TestInvokeURL(t *testing.T) {
	for _, tc := range []struct{ endpoint, context, version, want string }{
		{"https://gw.example.com", "/todos", "v1", "https://gw.example.com/todos"},
		{"gw.example.com//", "todos/$version", "v2", "https://gw.example.com/todos/v2"},
		{"HTTP://gw.example.com", "", "", "HTTP://gw.example.com/"},
		{"  ", "/todos", "v1", ""},
	} {
		assert.Equal(t, tc.want, invokeURL(tc.endpoint, tc.context, tc.version), tc)
	}
}

func TestOpenAPIFromOperations_NeedsOperations(t *testing.T) {
	_, err := openAPIFromOperations(object{"id": "empty"}, "")
	assert.ErrorContains(t, err, "no operations")
}

// allScopes is every scope any tool needs.
func allScopes(t *testing.T, s *Server) []string {
	t.Helper()
	all := s.allScopes()
	require.NotEmpty(t, all)
	return all
}

func enumOf(t *testing.T, schema any, property string) []string {
	t.Helper()
	var in struct {
		Properties map[string]struct {
			Enum []string `json:"enum"`
		} `json:"properties"`
	}
	b, _ := json.Marshal(schema)
	require.NoError(t, json.Unmarshal(b, &in))
	return in.Properties[property].Enum
}
