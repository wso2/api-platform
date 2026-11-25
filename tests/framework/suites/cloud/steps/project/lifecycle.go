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

package project

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

// CreateProject creates a uniquely named project and keeps it as the scenario's project.
func (s *Steps) CreateProject(ctx context.Context) error {
	return s.createProjectOf(ctx, cleanup.KindProject)
}

// createProjectOf creates a uniquely named project, registered for cleanup under the given kind
// before it is requested, and keeps it as the scenario's project.
func (s *Steps) createProjectOf(ctx context.Context, kind cleanup.Kind) error {
	id, resp, err := s.requestProjectOf(ctx, kind)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return cloudbase.ResponseError(resp)
	}
	return s.keepCreatedProject(ctx, kind, id, resp)
}

// requestProjectOf asks for a uniquely named project and returns the answer whatever it is. The
// project is registered for cleanup under the given kind before it is requested.
func (s *Steps) requestProjectOf(ctx context.Context, kind cleanup.Kind) (string, *httpx.Response, error) {
	if err := cloudbase.GenerateResourceAndStore(ctx, "syn-project", cloudbase.KeyProjectID); err != nil {
		return "", nil, err
	}
	id, err := cloudbase.RequiredValue(ctx, cloudbase.KeyProjectID)
	if err != nil {
		return "", nil, err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: kind, ID: id, Actor: cloudbase.ResourceActor, Description: "project " + id,
	}); err != nil {
		return "", nil, fmt.Errorf("registering project %q for cleanup: %w", id, err)
	}
	resp, err := s.DoAuthenticated(ctx, http.MethodPost, "/projects", map[string]string{
		"id": id, "displayName": "Synthetic " + id,
		"description": "Created and deleted by the APIP cloud integration test. Safe to delete.",
	})
	if err != nil {
		return "", nil, err
	}
	return id, resp, nil
}

// keepCreatedProject makes the project a create answered 201 for the scenario's project, and
// registers the id the platform gave it when that differs from the one requested.
func (s *Steps) keepCreatedProject(ctx context.Context, kind cleanup.Kind, id string, resp *httpx.Response) error {
	createdID, err := cloudbase.StringField(resp, "id")
	if err != nil {
		return fmt.Errorf("creating APIP project: %w", err)
	}
	if createdID != id {
		cloudbase.Deregister(ctx, kind, id)
		if err := cleanup.Register(ctx, cleanup.Resource{
			Kind: kind, ID: createdID, Actor: cloudbase.ResourceActor, Description: "project " + createdID,
		}); err != nil {
			return fmt.Errorf("registering project %q for cleanup: %w", createdID, err)
		}
	}
	return tcontext.Set(ctx, cloudbase.KeyProjectID, createdID)
}

// GetProject waits until the scenario's project can be read by its handle.
func (s *Steps) GetProject(ctx context.Context) error {
	id, err := cloudbase.RequiredValue(ctx, cloudbase.KeyProjectID)
	if err != nil {
		return err
	}
	err = retry.Await(ctx, retry.Options{Timeout: cloudbase.PropagationTimeout(s.Topo), Interval: 3 * time.Second},
		func(ctx context.Context) (*httpx.Response, error) {
			return s.DoAuthenticatedRaw(ctx, http.MethodGet, "/projects/"+url.PathEscape(id), nil)
		}, func(resp *httpx.Response) bool {
			if resp == nil || resp.StatusCode != http.StatusOK {
				return false
			}
			value, fieldErr := cloudbase.StringField(resp, "id")
			return fieldErr == nil && value == id
		}, "waiting for APIP project "+id+" to be retrievable")
	return err
}
