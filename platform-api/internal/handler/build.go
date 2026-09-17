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

package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"

	"github.com/wso2/api-platform/httpkit/httputil"
)

// BuildEndpoints is the slice of a kind's deployment service that serves builds.
// Every kind's service satisfies it by delegating to the shared build store, so one
// set of handlers serves them all.
type BuildEndpoints interface {
	CreateBuildByHandle(handle, orgID, actor, description string,
		metadata map[string]interface{}) (*api.BuildResponse, error)
	GetBuildByHandle(handle, buildID, orgID string) (*api.BuildResponse, error)
	GetBuildsByHandle(handle, orgID string, limit int) (*api.BuildListResponse, error)
	DeleteBuildByHandle(handle, buildID, orgID string) error
}

// BuildRoutes serves one artifact kind's /builds endpoints.
//
// The routes differ between kinds only in the path they hang off and the words used
// in messages, so they are registered from this one implementation rather than
// copied per kind. Each kind keeps its own URL — /rest-apis/…/builds,
// /mcp-proxies/…/builds — because that is the contract callers already know; only
// the code behind them is shared.
type BuildRoutes struct {
	// Service serves the builds, already scoped to one artifact kind.
	Service BuildEndpoints
	// Segment is the kind's path segment, e.g. "mcp-proxies".
	Segment string
	// PathParam is the identifier's name in the route, e.g. "mcpProxyId".
	PathParam string
	// Subject names the kind in error messages, e.g. "MCP proxy".
	Subject  string
	Identity *service.IdentityService
	Slogger  *slog.Logger
}

// Register adds the kind's four build routes to the mux.
func (h BuildRoutes) Register(mux router.Router) {
	base := constants.APIBasePath + "/" + h.Segment + "/{" + h.PathParam + "}"
	mux.HandleFunc("POST "+base+"/builds", middleware.MapErrors(h.Slogger, h.create))
	mux.HandleFunc("GET "+base+"/builds", middleware.MapErrors(h.Slogger, h.list))
	mux.HandleFunc("GET "+base+"/builds/{buildId}", middleware.MapErrors(h.Slogger, h.get))
	mux.HandleFunc("DELETE "+base+"/builds/{buildId}", middleware.MapErrors(h.Slogger, h.delete))
}

// request pulls the organization and the artifact handle out of a request, which
// every one of these routes needs before it can do anything.
func (h BuildRoutes) request(r *http.Request) (orgID, handle string, err error) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return "", "", apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	handle = r.PathValue(h.PathParam)
	if handle == "" {
		return "", "", apperror.ValidationFailed.New(h.Subject + " ID is required")
	}
	return orgID, handle, nil
}

// create handles POST /<segment>/{id}/builds — renders the artifact's current
// definition into an immutable snapshot, without deploying it.
func (h BuildRoutes) create(w http.ResponseWriter, r *http.Request) error {
	orgID, handle, err := h.request(r)
	if err != nil {
		return err
	}
	createdBy, err := resolveActorErr(r, h.Identity, "prepare "+h.Subject+" build")
	if err != nil {
		return err
	}

	// The body is optional: preparing a build needs nothing beyond the artifact,
	// and description and metadata are there for callers that have something to
	// record.
	var req api.BuildRequest
	if r.Body != nil && r.ContentLength != 0 {
		// A chunked request carries no length, so an empty one only shows up here
		// as EOF; that is still an absent body rather than a malformed one.
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			return apperror.ValidationFailed.New("Request body is not valid JSON")
		}
	}
	var metadata map[string]interface{}
	if req.Metadata != nil {
		metadata = *req.Metadata
	}
	var description string
	if req.Description != nil {
		description = strings.TrimSpace(*req.Description)
	}

	build, err := h.Service.CreateBuildByHandle(handle, orgID, createdBy, description, metadata)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to prepare a build for %s %s", h.Subject, handle))
	}

	setLocation(w, h.Segment, handle, "builds", build.BuildId)
	httputil.WriteJSON(w, http.StatusCreated, build)
	return nil
}

// list handles GET /<segment>/{id}/builds — the artifact's builds, newest first.
func (h BuildRoutes) list(w http.ResponseWriter, r *http.Request) error {
	orgID, handle, err := h.request(r)
	if err != nil {
		return err
	}
	limit, _ := parsePagination(r)
	builds, err := h.Service.GetBuildsByHandle(handle, orgID, limit)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to get builds for %s %s", h.Subject, handle))
	}
	httputil.WriteJSON(w, http.StatusOK, builds)
	return nil
}

// get handles GET /<segment>/{id}/builds/{buildId}.
func (h BuildRoutes) get(w http.ResponseWriter, r *http.Request) error {
	orgID, handle, err := h.request(r)
	if err != nil {
		return err
	}
	buildID := r.PathValue("buildId")
	if buildID == "" {
		return apperror.ValidationFailed.New("Build ID is required")
	}
	build, err := h.Service.GetBuildByHandle(handle, buildID, orgID)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to get %s %s build %s", h.Subject, handle, buildID))
	}
	httputil.WriteJSON(w, http.StatusOK, build)
	return nil
}

// delete handles DELETE /<segment>/{id}/builds/{buildId} — how room is made once
// the artifact is at its build limit.
func (h BuildRoutes) delete(w http.ResponseWriter, r *http.Request) error {
	orgID, handle, err := h.request(r)
	if err != nil {
		return err
	}
	buildID := r.PathValue("buildId")
	if buildID == "" {
		return apperror.ValidationFailed.New("Build ID is required")
	}
	if err := h.Service.DeleteBuildByHandle(handle, buildID, orgID); err != nil {
		return serviceError(err, fmt.Sprintf("failed to delete %s %s build %s", h.Subject, handle, buildID))
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
