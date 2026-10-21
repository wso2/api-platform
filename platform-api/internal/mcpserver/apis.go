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
	"cmp"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
)

// The API tools. Each serves every API type in apiTypes; apiType picks one.

type apiIDIn struct {
	apiTypeIn
	APIID string `json:"apiId" jsonschema:"The API's id, as list_apis returns it."`
}

type apiOut struct {
	APIType string `json:"apiType"`
	ID      string `json:"id"`
	API     object `json:"api"`
}

func newAPIOut(t *apiType, o object) apiOut {
	if len(t.hiddenInOutput) > 0 {
		trimmed := make(object, len(o))
		for k, v := range o {
			trimmed[k] = v
		}
		for _, k := range t.hiddenInOutput {
			delete(trimmed, k)
		}
		o = trimmed
	}
	return apiOut{APIType: t.name, ID: str(o, "id"), API: o}
}

// list_apis

type listAPIsIn struct {
	apiTypeIn
	ProjectID string `json:"projectId,omitempty" jsonschema:"The project, from list_projects. Optional for agent-proxy; llm-provider has none; required for the rest."`
	pageIn
}

type apiSummary struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Version     string `json:"version,omitempty"`
	Context     string `json:"context,omitempty"`
	ProjectID   string `json:"projectId,omitempty"`
	Status      string `json:"status,omitempty"`
	ReadOnly    bool   `json:"readOnly,omitempty"`
}

type listAPIsOut struct {
	APIType    string       `json:"apiType"`
	Items      []apiSummary `json:"items"`
	NextCursor string       `json:"nextCursor,omitempty"`
	Total      int          `json:"total"`
}

func (s *Server) listAPIs(ctx context.Context, c caller, in listAPIsIn) (listAPIsOut, error) {
	t := in.apiType()
	if err := checkProject(t, in.ProjectID); err != nil {
		return listAPIsOut{}, err
	}
	p, err := in.page()
	if err != nil {
		return listAPIsOut{}, err
	}
	items, total, err := s.backend.listAPIs(ctx, c, t, in.ProjectID, p)
	if err != nil {
		return listAPIsOut{}, err
	}
	out := listAPIsOut{APIType: t.name, Items: make([]apiSummary, 0, len(items)), Total: total, NextCursor: p.next(len(items), total)}
	for _, o := range items {
		out.Items = append(out.Items, apiSummary{
			ID:          str(o, "id"),
			DisplayName: str(o, "displayName"),
			Version:     str(o, "version"),
			Context:     str(o, "context"),
			ProjectID:   str(o, "projectId"),
			Status:      cmp.Or(str(o, "lifeCycleStatus"), str(o, "status")),
			ReadOnly:    o["readOnly"] == true,
		})
	}
	return out, nil
}

func checkProject(t *apiType, projectID string) error {
	switch {
	case t.project == projectRequired && projectID == "":
		return fail("projectId is required for %s; get one from list_projects", t.name)
	case t.project == projectNone && projectID != "":
		return fail("%s has no project; leave projectId out", t.name)
	}
	return nil
}

// get_api

func (s *Server) getAPI(ctx context.Context, c caller, in apiIDIn) (apiOut, error) {
	t := in.apiType()
	o, err := s.backend.getAPI(ctx, c, t, in.APIID)
	if err != nil {
		return apiOut{}, err
	}
	return newAPIOut(t, o), nil
}

// create_api

type createAPIIn struct {
	apiTypeIn
	Payload object `json:"payload" jsonschema:"The API as JSON, as the Platform API's REST API takes it."`
}

func (s *Server) createAPI(ctx context.Context, c caller, in createAPIIn) (apiOut, error) {
	t := in.apiType()
	o, err := s.backend.createAPI(ctx, c, t, in.Payload)
	if err != nil {
		return apiOut{}, err
	}
	return newAPIOut(t, o), nil
}

// update_api

type updateAPIIn struct {
	apiTypeIn
	APIID   string `json:"apiId" jsonschema:"The API's id, as list_apis returns it."`
	Payload object `json:"payload" jsonschema:"Only the fields to change. Fields left out stay as they are. An array replaces the old one. null removes a field (to clear a rest-api description, send an empty string); to empty a list, send []."`
}

// updateAPI reads the API, merges the payload in and sends the whole object
// back: the Platform API's PUT replaces the API, so a field left out would be wiped.
func (s *Server) updateAPI(ctx context.Context, c caller, in updateAPIIn) (apiOut, error) {
	t := in.apiType()
	current, err := s.backend.getAPI(ctx, c, t, in.APIID)
	if err != nil {
		return apiOut{}, err
	}
	merged := merge(current, in.Payload)
	// merge has folded any spelling of "id" onto the stored key, so this is the value sent.
	if id, ok := merged["id"]; ok && id != in.APIID {
		return apiOut{}, fail("id cannot change after the API is created")
	}
	o, err := s.backend.updateAPI(ctx, c, t, in.APIID, merged)
	if err != nil {
		return apiOut{}, err
	}
	return newAPIOut(t, o), nil
}

// merge lays a partial payload over the current object, as a JSON merge patch:
// objects merge field by field, null removes a field, and anything else,
// arrays included, replaces the old value. Keys match ignoring case, like the
// Platform API's JSON decoder, so the existing spelling is kept.
func merge(current, patch map[string]any) map[string]any {
	out := make(map[string]any, len(current))
	for k, v := range current {
		out[k] = v
	}
	for k, v := range patch {
		key := k
		for existing := range out {
			if strings.EqualFold(existing, k) {
				key = existing
				break
			}
		}
		switch pv := v.(type) {
		case nil:
			delete(out, key)
		case map[string]any:
			base, _ := out[key].(map[string]any)
			out[key] = merge(base, pv)
		default:
			out[key] = v
		}
	}
	return out
}

// delete_api

type deleteAPIOut struct {
	APIType string `json:"apiType"`
	Deleted string `json:"deleted"`
}

func (s *Server) deleteAPI(ctx context.Context, c caller, in apiIDIn) (deleteAPIOut, error) {
	t := in.apiType()
	if err := s.backend.deleteAPI(ctx, c, t, in.APIID); err != nil {
		return deleteAPIOut{}, err
	}
	return deleteAPIOut{APIType: t.name, Deleted: in.APIID}, nil
}

// list_api_deployments

type listDeploymentsIn struct {
	apiIDIn
	pageIn
}

type deployment struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	GatewayID string `json:"gatewayId"`
	Status    string `json:"status"`
	// StatusReason is an error code, set only when Status is FAILED.
	StatusReason string `json:"statusReason,omitempty"`
	BuildID      string `json:"buildId,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
}

type listDeploymentsOut struct {
	Items      []deployment `json:"items"`
	NextCursor string       `json:"nextCursor,omitempty"`
	Total      int          `json:"total"`
}

func (s *Server) listDeployments(ctx context.Context, c caller, in listDeploymentsIn) (listDeploymentsOut, error) {
	t := in.apiType()
	p, err := in.page()
	if err != nil {
		return listDeploymentsOut{}, err
	}
	items, total, err := s.backend.listDeployments(ctx, c, t, in.APIID, "", "", p)
	if err != nil {
		return listDeploymentsOut{}, err
	}
	out := listDeploymentsOut{Items: make([]deployment, 0, len(items)), Total: total, NextCursor: p.next(len(items), total)}
	for _, d := range items {
		out.Items = append(out.Items, newDeployment(d))
	}
	return out, nil
}

func newDeployment(d api.DeploymentResponse) deployment {
	out := deployment{ID: d.DeploymentId.String(), Name: d.Name, GatewayID: d.GatewayId, Status: string(d.Status)}
	if d.StatusReason != nil {
		out.StatusReason = *d.StatusReason
	}
	if d.BuildId != nil {
		out.BuildID = *d.BuildId
	}
	if d.UpdatedAt != nil {
		out.UpdatedAt = d.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return out
}

// deploy_api

type deployIn struct {
	apiIDIn
	GatewayID string `json:"gatewayId" jsonschema:"The gateway, from list_gateways."`
}

type deployOut struct {
	APIID       string       `json:"apiId"`
	Deployments []deployment `json:"deployments"`
}

func (s *Server) deployAPI(ctx context.Context, c caller, in deployIn) (deployOut, error) {
	t := in.apiType()
	d, err := s.backend.deploy(ctx, c, t, in.APIID, api.DeployRequest{
		Name:      "mcp-" + time.Now().UTC().Format("20060102-150405"),
		Base:      "current",
		GatewayId: in.GatewayID,
	})
	if err != nil {
		return deployOut{}, err
	}
	return deployOut{APIID: in.APIID, Deployments: []deployment{newDeployment(*d)}}, nil
}

// undeploy_api

type undeployOut struct {
	APIID      string       `json:"apiId"`
	Undeployed []deployment `json:"undeployed"`
	Message    string       `json:"message,omitempty"`
}

// undeployAPI undeploys every deployment on the gateway that is running or
// still starting, as the Platform API's undeploy accepts both. It stops at the first
// failure and says which ones finished, so only the rest need a retry.
func (s *Server) undeployAPI(ctx context.Context, c caller, in deployIn) (undeployOut, error) {
	t := in.apiType()
	if strings.TrimSpace(in.GatewayID) == "" {
		return undeployOut{}, fail("gatewayId is required; get one from list_gateways")
	}
	var live []api.DeploymentResponse
	for _, status := range []api.DeploymentResponseStatus{api.DeploymentResponseStatusDEPLOYED, api.DeploymentResponseStatusDEPLOYING} {
		found, _, err := s.backend.listDeployments(ctx, c, t, in.APIID, in.GatewayID, string(status), page{limit: maxPageLimit})
		if err != nil {
			return undeployOut{}, err
		}
		live = append(live, found...)
	}
	out := undeployOut{APIID: in.APIID, Undeployed: []deployment{}}
	if len(live) == 0 {
		// A success, so a repeat is safe; the message keeps it from looking like work done.
		out.Message = fmt.Sprintf("Nothing to undeploy: %s has no running deployment on gateway %s. "+
			"Check the gateway with list_api_deployments.", in.APIID, in.GatewayID)
		return out, nil
	}
	for _, d := range live {
		done, err := s.backend.undeploy(ctx, c, t, in.APIID, d.DeploymentId.String(), in.GatewayID)
		if err != nil {
			ids := make([]string, len(out.Undeployed))
			for i, u := range out.Undeployed {
				ids[i] = u.ID
			}
			return out, fail("undeployed [%s], then deployment %s failed: %s",
				strings.Join(ids, ", "), d.DeploymentId, s.toolError("undeploy_api", err))
		}
		out.Undeployed = append(out.Undeployed, newDeployment(*done))
	}
	return out, nil
}

func str(o object, key string) string {
	if v, ok := o[key]; ok && v != nil {
		return fmt.Sprint(v)
	}
	return ""
}
