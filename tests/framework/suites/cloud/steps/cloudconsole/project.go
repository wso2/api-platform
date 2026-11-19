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
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const (
	// keyAbsentProjectID names a project the scenario never creates.
	keyAbsentProjectID = "cloudAbsentProjectID"
	// keyRejectedProjectID names the project whose creation the scenario expects to be refused.
	keyRejectedProjectID = "cloudRejectedProjectID"

	// handleLengthBeyondStorage is a handle length past the 40 characters the project record
	// stores and short of the 63 the handle rules allow.
	handleLengthBeyondStorage = 45
)

// handleDefects turns a valid generated project handle into one that breaks a naming rule, keyed
// by how a scenario describes the defect.
var handleDefects = map[string]func(string) string{
	"uppercase":                strings.ToUpper,
	"containing an underscore": func(handle string) string { return strings.ReplaceAll(handle, "-", "_") },
	"starting with a hyphen":   func(handle string) string { return "-" + handle },
	"ending with a hyphen":     func(handle string) string { return handle + "-" },
	"containing consecutive hyphens": func(handle string) string {
		return strings.Replace(handle, "-", "--", 1)
	},
	"longer than 40 characters": func(handle string) string {
		if len(handle) >= handleLengthBeyondStorage {
			return handle
		}
		return handle + "-" + strings.Repeat("x", handleLengthBeyondStorage-len(handle)-1)
	},
}

func handleDefectNames() string {
	names := make([]string, 0, len(handleDefects))
	for name := range handleDefects {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// registerProjectSteps binds the steps that read the scenario's project, probe projects that do not
// exist, refuse a project, and read a project's pipeline binding.
func (s *Steps) registerProjectSteps(sc stepRegistrar) {
	sc.Step(`^I read the APIP test project$`, s.readDefaultProject)
	sc.Step(`^I read an APIP project that does not exist$`, s.readAbsentProject)
	sc.Step(`^I create the synthetic APIP project again under the same handle$`, s.createDuplicateProject)
	sc.Step(`^I request a synthetic APIP project whose handle is "([^"]*)"$`, s.requestProjectWithDefectiveHandle)
	sc.Step(`^the rejected APIP project should not exist$`, s.assertRejectedProjectAbsent)
	sc.Step(`^the synthetic APIP project should become active$`, s.awaitProjectActive)
	sc.Step(`^I read the deployment pipeline of the APIP project$`, s.readProjectPipeline)
	sc.Step(`^the deployment pipeline the APIP project is bound to should be listed by the platform$`, s.assertBoundPipelineListed)
}

// readDefaultProject reads the scenario's project directly, which stays correct once
// the organisation holds more projects than the first page of the list.
func (s *Steps) readDefaultProject(ctx context.Context) error {
	project, err := requiredValue(ctx, keyScenarioProject)
	if err != nil {
		return fmt.Errorf("the scenario's project is not known: %w", err)
	}
	_, err = s.doAuthenticated(ctx, http.MethodGet, "/projects/"+url.PathEscape(project), nil)
	return err
}

// chooseAbsentProjectID generates a project handle nothing has created.
func chooseAbsentProjectID(ctx context.Context) (string, error) {
	if err := generateResourceAndStore(ctx, "syn-project", keyAbsentProjectID); err != nil {
		return "", err
	}
	return requiredValue(ctx, keyAbsentProjectID)
}

// readAbsentProject reads a project that does not exist and publishes the answer.
func (s *Steps) readAbsentProject(ctx context.Context) error {
	id, err := chooseAbsentProjectID(ctx)
	if err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodGet, "/projects/"+url.PathEscape(id), nil)
	return err
}

// createDuplicateProject asks for a second project under the handle of the scenario's project,
// with a display name of its own so only the handle can collide, and publishes the answer. The
// handle is already registered for cleanup.
func (s *Steps) createDuplicateProject(ctx context.Context) error {
	id, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return fmt.Errorf("the scenario has created no synthetic project: %w", err)
	}
	_, err = s.doAuthenticated(ctx, http.MethodPost, "/projects", map[string]string{
		"id": id, "displayName": "Synthetic duplicate of " + id,
		"description": "Created and deleted by the APIP cloud integration test. Safe to delete.",
	})
	return err
}

// requestProjectWithDefectiveHandle asks for a project whose handle breaks the named naming rule
// and publishes the answer without requiring success. The handle is registered for cleanup before
// the request, so a project the platform accepts anyway is removed.
func (s *Steps) requestProjectWithDefectiveHandle(ctx context.Context, defect string) error {
	corrupt, ok := handleDefects[defect]
	if !ok {
		return fmt.Errorf("unknown handle defect %q: the defects are %s", defect, handleDefectNames())
	}
	if err := generateResourceAndStore(ctx, "syn-project", keyRejectedProjectID); err != nil {
		return err
	}
	valid, err := requiredValue(ctx, keyRejectedProjectID)
	if err != nil {
		return err
	}
	handle := corrupt(valid)
	if err := tcontext.Set(ctx, keyRejectedProjectID, handle); err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindProject, ID: handle, Actor: resourceActor, Description: "project " + handle,
	}); err != nil {
		return fmt.Errorf("registering project %q for cleanup: %w", handle, err)
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, "/projects", map[string]string{
		"id": handle, "displayName": "Synthetic " + valid,
		"description": "Created and deleted by the APIP cloud integration test. Safe to delete.",
	})
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusCreated {
		if createdID, idErr := stringField(resp, "id"); idErr == nil && createdID != handle {
			return cleanup.Register(ctx, cleanup.Resource{
				Kind: cleanup.KindProject, ID: createdID, Actor: resourceActor, Description: "project " + createdID,
			})
		}
	}
	return nil
}

// assertRejectedProjectAbsent checks the project whose creation was refused left no record
// behind. It reads once: a refused create is rolled back before it answers.
func (s *Steps) assertRejectedProjectAbsent(ctx context.Context) error {
	id, err := requiredValue(ctx, keyRejectedProjectID)
	if err != nil {
		return fmt.Errorf("the scenario has requested no project that should be refused: %w", err)
	}
	resp, err := s.doAuthenticated(ctx, http.MethodGet, "/projects/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("project %q should not exist after its creation was refused: %s", id, resp.Describe())
	}
	return nil
}

// awaitProjectActive waits until the scenario's project reports the status active, and stops at
// once if it reports failed.
func (s *Steps) awaitProjectActive(ctx context.Context) error {
	id, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		func(ctx context.Context) (string, error) {
			resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, "/projects/"+url.PathEscape(id), nil)
			if err != nil {
				return "", retry.Transient(err)
			}
			if resp.StatusCode != http.StatusOK {
				return "", retry.Transient(responseError(resp))
			}
			status, err := stringField(resp, "status")
			if err != nil {
				return "", fmt.Errorf("reading the status of project %q: %w (%s)", id, err, resp.Describe())
			}
			if status == "failed" {
				return "", fmt.Errorf("project %q reports the status failed", id)
			}
			return status, nil
		},
		func(status string) bool { return status == "active" },
		"waiting for project "+id+" to become active")
}

// readProjectPipeline reads the deployment pipeline the scenario's project is bound to and
// publishes the answer.
func (s *Steps) readProjectPipeline(ctx context.Context) error {
	id, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return fmt.Errorf("the scenario has no project: %w", err)
	}
	_, err = s.doAuthenticated(ctx, http.MethodGet, "/projects/"+url.PathEscape(id)+"/deployment-pipeline", nil)
	return err
}

// assertBoundPipelineListed checks that the deployment pipeline the last response names as the
// project's is one the platform lists. The listing read is not published, so the binding stays the
// response under assertion.
func (s *Steps) assertBoundPipelineListed(ctx context.Context) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	pipeline, err := stringField(resp, "pipeline")
	if err != nil {
		return fmt.Errorf("reading the project's deployment pipeline: %w (%s)", err, resp.Describe())
	}
	listed, err := s.doAuthenticatedRaw(ctx, http.MethodGet, "/pipelines", nil)
	if err != nil {
		return err
	}
	if listed.StatusCode != http.StatusOK {
		return responseError(listed)
	}
	_, err = listedPipelinePaths(listed, pipeline)
	return err
}
