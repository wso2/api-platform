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
	"strings"
	"unicode/utf8"

	"github.com/wso2/api-platform/platform-api/api"
)

// The platform tools: projects and gateways.

// list_projects

type listProjectsIn struct {
	pageIn
}

type listProjectsOut struct {
	Items      []api.Project `json:"items"`
	NextCursor string        `json:"nextCursor,omitempty"`
	Total      int           `json:"total"`
}

func (s *Server) listProjects(ctx context.Context, c caller, in listProjectsIn) (listProjectsOut, error) {
	p, err := in.page()
	if err != nil {
		return listProjectsOut{}, err
	}
	items, total, err := s.backend.listProjects(ctx, c, p)
	if err != nil {
		return listProjectsOut{}, err
	}
	return listProjectsOut{Items: items, Total: total, NextCursor: p.next(len(items), total)}, nil
}

// create_project

type createProjectIn struct {
	Name        string `json:"name" jsonschema:"The project's name, 1 to 128 characters."`
	Description string `json:"description,omitempty" jsonschema:"What the project is for."`
}

func (s *Server) createProject(ctx context.Context, c caller, in createProjectIn) (api.Project, error) {
	name := strings.TrimSpace(in.Name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 128 {
		return api.Project{}, fail("name must be 1 to 128 characters")
	}
	req := api.CreateProjectRequest{DisplayName: name}
	if in.Description != "" {
		req.Description = &in.Description
	}
	p, err := s.backend.createProject(ctx, c, req)
	if err != nil {
		return api.Project{}, err
	}
	return *p, nil
}

// delete_project

type projectIDIn struct {
	ProjectID string `json:"projectId" jsonschema:"The project's exact id, from list_projects. If the user gave a name and no project matches it exactly, ask; never substitute the closest match."`
}

type deletedOut struct {
	Deleted string `json:"deleted"`
}

func (s *Server) deleteProject(ctx context.Context, c caller, in projectIDIn) (deletedOut, error) {
	if err := s.backend.deleteProject(ctx, c, in.ProjectID); err != nil {
		return deletedOut{}, err
	}
	return deletedOut{Deleted: in.ProjectID}, nil
}

// list_gateways, get_gateway

type listGatewaysIn struct {
	pageIn
}

type listGatewaysOut struct {
	Items      []api.GatewayResponse `json:"items"`
	NextCursor string                `json:"nextCursor,omitempty"`
	Total      int                   `json:"total"`
}

func (s *Server) listGateways(ctx context.Context, c caller, in listGatewaysIn) (listGatewaysOut, error) {
	p, err := in.page()
	if err != nil {
		return listGatewaysOut{}, err
	}
	items, total, err := s.backend.listGateways(ctx, c, p)
	if err != nil {
		return listGatewaysOut{}, err
	}
	return listGatewaysOut{Items: items, Total: total, NextCursor: p.next(len(items), total)}, nil
}

type gatewayIDIn struct {
	GatewayID string `json:"gatewayId" jsonschema:"The gateway's exact id, from list_gateways. If the user gave a name and no gateway matches it exactly, ask; never substitute the closest match."`
}

func (s *Server) getGateway(ctx context.Context, c caller, in gatewayIDIn) (api.GatewayResponse, error) {
	g, err := s.backend.getGateway(ctx, c, in.GatewayID)
	if err != nil {
		return api.GatewayResponse{}, err
	}
	return *g, nil
}
