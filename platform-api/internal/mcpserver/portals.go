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
	"regexp"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// The definition and API Portal tools. They serve only the types with a
// definition document, which a portal listing is built from.

func hasDefinition(t *apiType) bool { return t.definition != "" }
func publishable(t *apiType) bool   { return t.publishable && t.definition != "" }

// get_api_definition

type apiDefinitionOut struct {
	APIType     string `json:"apiType"`
	ID          string `json:"id"`
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

func (s *Server) getAPIDefinition(ctx context.Context, c caller, in apiIDIn) (apiDefinitionOut, error) {
	t := in.apiType()
	doc, err := s.backend.getDefinition(ctx, c, t, in.APIID)
	if apperror.NotFound.Is(err) && t.name == "rest-api" {
		// The API exists (getDefinition resolves it first); no document was ever uploaded.
		return apiDefinitionOut{}, fail("%s has no OpenAPI document; get_api shows its operations", in.APIID)
	}
	if err != nil {
		return apiDefinitionOut{}, err
	}
	return apiDefinitionOut{APIType: t.name, ID: in.APIID, ContentType: doc.ContentType, Content: doc.Content}, nil
}

// openAPIFromOperations is a minimal OpenAPI 3.0 document for a REST API with
// none stored, as one made from JSON is: its name, version and operations, and
// the production URL as its server. A portal listing needs a definition.
func openAPIFromOperations(o object, serverURL string) (definitionDoc, error) {
	ops, _ := o["operations"].([]any)
	if len(ops) == 0 {
		return definitionDoc{}, fail("%s has no OpenAPI document and no operations to describe; add operations with update_api", str(o, "id"))
	}
	paths := map[string]map[string]any{}
	for _, raw := range ops {
		op, _ := raw.(map[string]any)
		req, _ := op["request"].(map[string]any)
		method, path := strings.ToLower(str(req, "method")), str(req, "path")
		if method == "" || path == "" {
			continue
		}
		operation := map[string]any{"responses": map[string]any{"default": map[string]any{"description": "The backend's response"}}}
		if name := str(op, "name"); name != "" {
			operation["summary"] = name
		}
		if desc := str(op, "description"); desc != "" {
			operation["description"] = desc
		}
		var params []any
		for _, m := range pathParam.FindAllStringSubmatch(path, -1) {
			params = append(params, map[string]any{"name": m[1], "in": "path", "required": true, "schema": map[string]any{"type": "string"}})
		}
		if len(params) > 0 {
			operation["parameters"] = params
		}
		if paths[path] == nil {
			paths[path] = map[string]any{}
		}
		paths[path][method] = operation
	}
	info := map[string]any{"title": str(o, "displayName"), "version": str(o, "version")}
	if desc := str(o, "description"); desc != "" {
		info["description"] = desc
	}
	doc := map[string]any{"openapi": "3.0.3", "info": info, "paths": paths}
	if serverURL != "" {
		doc["servers"] = []any{map[string]any{"url": serverURL}}
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return definitionDoc{}, err
	}
	return definitionDoc{Content: string(b), ContentType: "application/json"}, nil
}

var pathParam = regexp.MustCompile(`\{([^{}/]+)\}`)

// list_api_portals

type listPortalsIn struct {
	pageIn
}

type portalSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

type listPortalsOut struct {
	Items      []portalSummary `json:"items"`
	NextCursor string          `json:"nextCursor,omitempty"`
	Total      int             `json:"total"`
}

func (s *Server) listPortals(ctx context.Context, c caller, in listPortalsIn) (listPortalsOut, error) {
	p, err := in.page()
	if err != nil {
		return listPortalsOut{}, err
	}
	items, total, err := s.backend.listPortals(ctx, c, p)
	if err != nil {
		return listPortalsOut{}, err
	}
	out := listPortalsOut{Items: make([]portalSummary, 0, len(items)), Total: total, NextCursor: p.next(len(items), total)}
	for _, it := range items {
		ps := portalSummary{ID: it.Id, Name: it.Name, URL: it.Url}
		if it.Description != nil {
			ps.Description = *it.Description
		}
		out.Items = append(out.Items, ps)
	}
	return out, nil
}

// publish_api, unpublish_api

type portalAPIIn struct {
	apiIDIn
	APIPortalID string `json:"apiPortalId" jsonschema:"The API Portal's exact id, from list_api_portals. If the user gave a name and no portal matches it exactly, ask; never substitute the closest match."`
}

type publishOut struct {
	APIType     string `json:"apiType"`
	ID          string `json:"id"`
	APIPortalID string `json:"apiPortalId"`
	Status      string `json:"status"`
	// ProductionURL is the address the listing gives consumers.
	ProductionURL string `json:"productionUrl,omitempty"`
	Message       string `json:"message,omitempty"`
}

// publishAPI publishes the API to a portal, as the console's Publish does. The
// console saves a draft listing first; when the API has none on this portal,
// the tool makes it the way the console would fill its form: the API's name,
// version and description, the URL of the first gateway it is deployed to, and
// its definition. A draft saved in the console is published as it is.
func (s *Server) publishAPI(ctx context.Context, c caller, in portalAPIIn) (publishOut, error) {
	t := in.apiType()
	portal := strings.TrimSpace(in.APIPortalID)
	if portal == "" {
		return publishOut{}, fail("apiPortalId is required; get one from list_api_portals")
	}
	out := publishOut{APIType: t.name, ID: in.APIID, APIPortalID: portal}

	draft, err := s.backend.getDraft(ctx, c, t, in.APIID, portal)
	if err != nil {
		return publishOut{}, err
	}
	if draft == nil {
		if draft, err = s.newDraft(ctx, c, t, in.APIID); err != nil {
			return publishOut{}, err
		}
		if err := s.backend.saveDraft(ctx, c, t, in.APIID, portal, draft); err != nil {
			return publishOut{}, err
		}
		if draft.ProductionURL == "" {
			out.Message = "The API is not deployed to a gateway yet, so the listing has no production URL. Deploy it, then publish again."
		}
	}
	hasDefinition, err := s.backend.hasDraftDefinition(ctx, c, t, in.APIID, portal)
	if err != nil {
		return publishOut{}, err
	}
	if !hasDefinition {
		doc, err := s.backend.getDefinition(ctx, c, t, in.APIID)
		if apperror.NotFound.Is(err) && t.name == "rest-api" {
			o, getErr := s.backend.getAPI(ctx, c, t, in.APIID)
			if getErr != nil {
				return publishOut{}, getErr
			}
			doc, err = openAPIFromOperations(o, draft.ProductionURL)
		}
		if err != nil {
			return publishOut{}, err
		}
		if err := s.backend.saveDraftDefinition(ctx, c, t, in.APIID, portal, doc); err != nil {
			return publishOut{}, err
		}
	}

	pub, err := s.backend.publish(ctx, c, t, in.APIID, portal)
	if err != nil {
		return publishOut{}, err
	}
	out.Status, out.ProductionURL = pub.Status, pub.ProductionURL
	return out, nil
}

// newDraft is the listing the console's form starts from for an API with none.
func (s *Server) newDraft(ctx context.Context, c caller, t *apiType, id string) (*model.Publication, error) {
	o, err := s.backend.getAPI(ctx, c, t, id)
	if err != nil {
		return nil, err
	}
	gateways, err := s.backend.listAPIGateways(ctx, c, t, id)
	if err != nil {
		return nil, err
	}
	draft := &model.Publication{
		DisplayName: str(o, "displayName"),
		Version:     str(o, "version"),
		Description: str(o, "description"),
	}
	for _, g := range gateways {
		if g.IsDeployed && g.Endpoints != nil && len(*g.Endpoints) > 0 {
			if url := invokeURL((*g.Endpoints)[0], str(o, "context"), draft.Version); url != "" {
				draft.ProductionURL = url
				break
			}
		}
	}
	return draft, nil
}

var trailingSlashes = regexp.MustCompile(`/+$`)

// invokeURL is {endpoint}{context}, with a scheme ensured and $version in the
// context resolved, as the console builds it (gatewayDisplay.ts buildInvokeUrl)
// and as the gateway routes it.
func invokeURL(endpoint, context, version string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	base := trailingSlashes.ReplaceAllString(endpoint, "")
	if lower := strings.ToLower(base); !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		base = "https://" + base
	}
	path := strings.TrimSpace(strings.ReplaceAll(context, "$version", version))
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

type unpublishOut struct {
	APIType     string `json:"apiType"`
	ID          string `json:"id"`
	APIPortalID string `json:"apiPortalId"`
	Status      string `json:"status"`
}

func (s *Server) unpublishAPI(ctx context.Context, c caller, in portalAPIIn) (unpublishOut, error) {
	t := in.apiType()
	portal := strings.TrimSpace(in.APIPortalID)
	if portal == "" {
		return unpublishOut{}, fail("apiPortalId is required; get one from list_api_portals")
	}
	if err := s.backend.unpublish(ctx, c, t, in.APIID, portal); err != nil {
		return unpublishOut{}, err
	}
	return unpublishOut{APIType: t.name, ID: in.APIID, APIPortalID: portal, Status: "UNPUBLISHED"}, nil
}
