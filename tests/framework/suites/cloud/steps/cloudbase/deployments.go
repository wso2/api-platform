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

package cloudbase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// TestbenchURL reads the https base URL of a testbench service from the environment, falling
// back to the given default, without a trailing slash.
func TestbenchURL(env, fallback string) (string, error) {
	value := strings.TrimSpace(os.Getenv(env))
	if value == "" {
		value = fallback
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", fmt.Errorf("%s must be an https URL with a host", env)
	}
	return strings.TrimRight(value, "/"), nil
}

// APIScopePath is the project-and-API prefix of the cloud-extension build and deployment routes
// for the synthetic REST API.
func APIScopePath(ctx context.Context) (string, error) {
	return RefScopePath(ctx, PrimaryAPI)
}

// RefScopePath is the project-and-API prefix of the build and deployment routes for the API a ref
// points at. A route under it still needs the ref's kind.
func RefScopePath(ctx context.Context, ref APIRef) (string, error) {
	projectID, err := RequiredValue(ctx, KeyProjectID)
	if err != nil {
		return "", err
	}
	apiID, err := RequiredValue(ctx, ref.ID)
	if err != nil {
		return "", err
	}
	return "/projects/" + url.PathEscape(projectID) + "/apis/" + url.PathEscape(apiID), nil
}

// OptionalValue reads a scenario value that may be unset.
func OptionalValue(ctx context.Context, key string) string {
	value, err := tcontext.ResolveString(ctx, key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

// OwnBuildState drops the builds a previous scenario recorded for the synthetic REST API.
func OwnBuildState(ctx context.Context) error {
	return ownBuildsOf(ctx, PrimaryAPI, KeyBuildID, KeyBuildMetadata, KeyBuildMetadataOf)
}

// ownBuildsOf drops the builds, and the related keys, that the scenario state holds for an API
// other than the one a ref points at now. Scenario state lives as long as the runner and a build
// id is a day and an index that is only unique within one API, so builds recorded for another API
// must not be mistaken for this API's.
func ownBuildsOf(ctx context.Context, ref APIRef, related ...string) error {
	apiID, err := RequiredValue(ctx, ref.ID)
	if err != nil {
		return err
	}
	if OptionalValue(ctx, ref.BuildsOwner) == apiID {
		return nil
	}
	for _, key := range append([]string{ref.builds}, related...) {
		if err := tcontext.Set(ctx, key, ""); err != nil {
			return err
		}
	}
	return tcontext.Set(ctx, ref.BuildsOwner, apiID)
}

// ownRefBuilds drops the builds the scenario state holds for an API other than the one a ref
// points at, with the related keys of the synthetic REST API's builds.
func ownRefBuilds(ctx context.Context, ref APIRef) error {
	if ref.builds == PrimaryAPI.builds {
		return OwnBuildState(ctx)
	}
	return ownBuildsOf(ctx, ref)
}

// AppendBuild adds a build to the builds the scenario saw for the API a ref points at.
func AppendBuild(ctx context.Context, ref APIRef, buildID string) error {
	if err := ownRefBuilds(ctx, ref); err != nil {
		return err
	}
	if seen := OptionalValue(ctx, ref.builds); seen != "" {
		buildID = seen + "," + buildID
	}
	return tcontext.Set(ctx, ref.builds, buildID)
}

// SeenBuilds lists the builds the scenario saw for the API a ref points at, oldest first.
func SeenBuilds(ctx context.Context, ref APIRef) ([]string, error) {
	if err := ownRefBuilds(ctx, ref); err != nil {
		return nil, err
	}
	seen := OptionalValue(ctx, ref.builds)
	if seen == "" {
		return nil, nil
	}
	return strings.Split(seen, ","), nil
}

// LatestPreparedBuild is the most recently prepared build of the scenario's API.
func LatestPreparedBuild(ctx context.Context) (string, error) {
	if err := OwnBuildState(ctx); err != nil {
		return "", err
	}
	prepared, err := RequiredValue(ctx, KeyBuildIDs)
	if err != nil {
		return "", fmt.Errorf("no build has been prepared: %w", err)
	}
	ids := strings.Split(prepared, ",")
	return ids[len(ids)-1], nil
}

// FetchBuilds lists the synthetic REST API's builds, newest first.
func (s *Steps) FetchBuilds(ctx context.Context) ([]map[string]any, error) {
	scope, err := APIScopePath(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := s.DoAuthenticatedRaw(ctx, http.MethodGet, scope+"/builds", nil)
	if err != nil {
		return nil, retry.Transient(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, retry.Transient(ResponseError(resp))
	}
	items, err := ListItems(resp)
	if err != nil {
		return nil, retry.Transient(fmt.Errorf("decoding builds: %w", err))
	}
	return items, nil
}

// StillDeployed are the statuses a deployment listing gives a gateway that serves, or is about to
// serve, the API.
var StillDeployed = map[string]bool{"DEPLOYED": true, "DEPLOYING": true}

// RecordedTargets reads the deployments the last successful request created, by gateway.
func RecordedTargets(ctx context.Context) (map[string]string, error) {
	encoded, err := RequiredValue(ctx, KeyDeployedTargets)
	if err != nil {
		return nil, fmt.Errorf("no deployment has been requested: %w", err)
	}
	targets := map[string]string{}
	if err := json.Unmarshal([]byte(encoded), &targets); err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("the deployment request created no deployment")
	}
	return targets, nil
}

// DeploymentEntry finds the response entry for a gateway.
func DeploymentEntry(resp *httpx.Response, gatewayID string) (map[string]any, error) {
	items, err := ListItems(resp)
	if err != nil {
		return nil, fmt.Errorf("decoding the deployment response: %w", err)
	}
	for _, item := range items {
		if item["gatewayId"] == gatewayID {
			return item, nil
		}
	}
	return nil, fmt.Errorf("the deployment response does not list gateway %q: %s", gatewayID, resp.Describe())
}

// Stages reads the deployment stages of the synthetic REST API through its project.
func (s *Steps) Stages(ctx context.Context) ([]map[string]any, error) {
	scope, err := APIScopePath(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := s.DoAuthenticated(ctx, http.MethodGet, scope+"/deployments", nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ResponseError(resp)
	}
	return ListItems(resp)
}

// PipelineGetter reads a path of the platform's API.
type PipelineGetter func(ctx context.Context, path string) (*httpx.Response, error)

// EnvironmentsOf collects the environments a pipeline's promotion paths reach.
func EnvironmentsOf(paths []PromotionPath) map[string]bool {
	environments := map[string]bool{}
	for _, path := range paths {
		environments[path.SourceEnvironment] = true
		for _, target := range path.TargetEnvironments {
			environments[target] = true
		}
	}
	return environments
}

// EntryEnvironmentsOf collects the entry environments of a pipeline: the sources no promotion path
// targets. They are the only environments the platform accepts a deployment to without a
// promotion.
func EntryEnvironmentsOf(paths []PromotionPath) map[string]bool {
	targets := map[string]bool{}
	for _, path := range paths {
		for _, target := range path.TargetEnvironments {
			targets[target] = true
		}
	}
	entries := map[string]bool{}
	for _, path := range paths {
		if path.SourceEnvironment != "" && !targets[path.SourceEnvironment] {
			entries[path.SourceEnvironment] = true
		}
	}
	return entries
}

// pipelineProjectOf returns the project whose deployment pipeline a gateway choice follows: the
// project the scenario found or created, else the project of its feature file.
func pipelineProjectOf(ctx context.Context) string {
	if project := strings.TrimSpace(OptionalValue(ctx, KeyProjectID)); project != "" {
		return project
	}
	return strings.TrimSpace(OptionalValue(ctx, KeyScenarioProject))
}

// PipelinePaths reads the promotion paths of the deployment pipeline the scenario's project is
// bound to.
func (s *Steps) PipelinePaths(ctx context.Context) ([]PromotionPath, error) {
	projectID := pipelineProjectOf(ctx)
	if projectID == "" {
		return nil, errors.New("the scenario's project is not known")
	}
	return PipelinePathsOf(ctx, projectID, func(ctx context.Context, path string) (*httpx.Response, error) {
		return s.DoAuthenticatedRaw(ctx, http.MethodGet, path, nil)
	})
}

// PromotionPath is one step of a deployment pipeline: where a build is promoted from and to.
type PromotionPath struct {
	SourceEnvironment  string   `json:"sourceEnvironment"`
	TargetEnvironments []string `json:"targetEnvironments"`
}

// PipelinePathsOf reads the promotion paths of the deployment pipeline a project is bound to.
func PipelinePathsOf(ctx context.Context, projectID string, get PipelineGetter) ([]PromotionPath, error) {
	_, paths, err := BoundPipelineOf(ctx, projectID, get)
	return paths, err
}

// BoundPipelineOf reads which deployment pipeline a project is bound to and that pipeline's
// promotion paths.
func BoundPipelineOf(ctx context.Context, projectID string, get PipelineGetter) (string, []PromotionPath, error) {
	bound, err := get(ctx, "/projects/"+url.PathEscape(projectID)+"/deployment-pipeline")
	if err != nil {
		return "", nil, err
	}
	if bound.StatusCode == http.StatusNotFound {
		return "", nil, &PipelineBindingMissingError{Cause: ResponseError(bound)}
	}
	if bound.StatusCode != http.StatusOK {
		return "", nil, ResponseError(bound)
	}
	name, err := StringField(bound, "pipeline")
	if err != nil {
		return "", nil, fmt.Errorf("reading the project's deployment pipeline: %w", err)
	}
	listed, err := get(ctx, "/pipelines")
	if err != nil {
		return "", nil, err
	}
	if listed.StatusCode != http.StatusOK {
		return "", nil, ResponseError(listed)
	}
	listedName, paths, err := FindListedPipeline(listed, name)
	return listedName, paths, err
}

// FindListedPipeline finds the pipeline a listing names, by id or by name, and returns its id and promotion paths.
func FindListedPipeline(listed *httpx.Response, pipeline string) (string, []PromotionPath, error) {
	var document struct {
		List []struct {
			ID             string          `json:"id"`
			Name           string          `json:"name"`
			PromotionPaths []PromotionPath `json:"promotionPaths"`
		} `json:"list"`
	}
	if err := json.Unmarshal(listed.Body, &document); err != nil {
		return "", nil, fmt.Errorf("decoding deployment pipelines: %w", err)
	}
	for _, candidate := range document.List {
		if candidate.ID == pipeline || candidate.Name == pipeline {
			name := candidate.Name
			if strings.TrimSpace(name) == "" {
				name = candidate.ID
			}
			return name, candidate.PromotionPaths, nil
		}
	}
	return "", nil, &PipelineNotListedError{Pipeline: pipeline}
}

// SortedKeys returns the members of a set in ascending order.
func SortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
