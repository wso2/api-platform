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
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const (
	keyPipelineID            = "cloudPipelineID"
	keySyntheticEnvironments = "cloudSyntheticEnvironments"

	// The numbered keys name the nth synthetic environment, its gateway and that gateway's endpoint.
	keySyntheticEnvironmentPrefix = "cloudSyntheticEnvironment"
	keySyntheticGatewayPrefix     = "cloudSyntheticGateway"
	keySyntheticEndpointPrefix    = "cloudSyntheticEndpoint"

	// maxPromotionEnvironments bounds the numbered keys a scenario can address and the state a
	// new scenario clears.
	maxPromotionEnvironments = 5
)

// A project holds the pipeline it is bound to, a pipeline holds the environments it names and
// is what makes a gateway its environment's default, and a gateway must be gone before its
// environment is. A promotion scenario therefore deletes project, pipeline, gateways and
// environments in that order, which the framework's own order (project last) cannot express.
var (
	kindPromotionProject = cleanup.Kind{Name: cleanup.KindProject.Name, Order: 86}
	kindPipeline         = cleanup.Kind{Name: "pipeline", Order: 87}
)

func numberedKey(prefix string, n int) string { return fmt.Sprintf("%s%d", prefix, n) }

// resetPromotionState drops what an earlier scenario of the runner recorded about its
// environments, gateways and deployments, so a numbered key can only name this scenario's own.
func resetPromotionState(ctx context.Context) error {
	keys := []string{keySyntheticEnvironments, keyPipelineID, keyDeployedTargets, keyDeployedBuildID, keyDeploymentID}
	for n := 1; n <= maxPromotionEnvironments; n++ {
		keys = append(keys, numberedKey(keySyntheticEnvironmentPrefix, n),
			numberedKey(keySyntheticGatewayPrefix, n), numberedKey(keySyntheticEndpointPrefix, n))
	}
	for _, key := range keys {
		if err := tcontext.Set(ctx, key, ""); err != nil {
			return err
		}
	}
	return nil
}

// createSyntheticEnvironments creates environments, each registered for cleanup before it is
// requested, and records them as synthetic environments 1 to count. The first is also the
// scenario's synthetic environment for the steps that take no number.
func (s *Steps) createSyntheticEnvironments(ctx context.Context, count int) error {
	if count < 1 || count > maxPromotionEnvironments {
		return fmt.Errorf("a scenario creates between 1 and %d synthetic environments, asked for %d", maxPromotionEnvironments, count)
	}
	if err := resetPromotionState(ctx); err != nil {
		return err
	}
	names := make([]string, 0, count)
	for n := 1; n <= count; n++ {
		if err := s.createEnvironment(ctx); err != nil {
			return err
		}
		name, err := requiredValue(ctx, keyEnvironmentName)
		if err != nil {
			return err
		}
		names = append(names, name)
		if err := tcontext.Set(ctx, numberedKey(keySyntheticEnvironmentPrefix, n), name); err != nil {
			return err
		}
	}
	if err := tcontext.Set(ctx, keyEnvironmentName, names[0]); err != nil {
		return err
	}
	return tcontext.Set(ctx, keySyntheticEnvironments, strings.Join(names, ","))
}

// promotionPathsFromTable reads a table whose header row is "source | targets" into promotion
// paths: one row per source environment, its targets comma separated.
func promotionPathsFromTable(table *godog.Table, expand func(string) (string, error)) ([]map[string]any, error) {
	if table == nil || len(table.Rows) < 2 {
		return nil, fmt.Errorf("a promotion paths table needs a header row and at least one path")
	}
	header := table.Rows[0].Cells
	if len(header) != 2 || strings.TrimSpace(header[0].Value) != "source" || strings.TrimSpace(header[1].Value) != "targets" {
		return nil, fmt.Errorf("a promotion paths table has the header \"source | targets\"")
	}
	paths := make([]map[string]any, 0, len(table.Rows)-1)
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != 2 {
			return nil, fmt.Errorf("a promotion path row has a source and its targets, got %d cells", len(row.Cells))
		}
		source, err := expand(row.Cells[0].Value)
		if err != nil {
			return nil, err
		}
		source = strings.TrimSpace(source)
		if source == "" {
			return nil, fmt.Errorf("a promotion path has no source environment")
		}
		var targets []string
		for _, target := range strings.Split(row.Cells[1].Value, ",") {
			expanded, err := expand(target)
			if err != nil {
				return nil, err
			}
			if expanded = strings.TrimSpace(expanded); expanded != "" {
				targets = append(targets, expanded)
			}
		}
		if len(targets) == 0 {
			return nil, fmt.Errorf("the promotion path from %q has no target environment", source)
		}
		paths = append(paths, map[string]any{"sourceEnvironment": source, "targetEnvironments": targets})
	}
	return paths, nil
}

// createPipeline creates a deployment pipeline with the table's promotion paths. The pipeline is
// registered for cleanup before it is requested.
func (s *Steps) createPipeline(ctx context.Context, table *godog.Table) error {
	paths, err := promotionPathsFromTable(table, func(value string) (string, error) { return expandCloudValue(ctx, value) })
	if err != nil {
		return err
	}
	if err := generateResourceAndStore(ctx, "syn-pipeline", keyPipelineID); err != nil {
		return err
	}
	id, err := requiredValue(ctx, keyPipelineID)
	if err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: kindPipeline, ID: id, Actor: resourceActor, Description: "deployment pipeline " + id,
	}); err != nil {
		return fmt.Errorf("registering deployment pipeline %q for cleanup: %w", id, err)
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, "/pipelines", map[string]any{"name": id, "promotionPaths": paths})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return responseError(resp)
	}
	createdID, err := stringField(resp, "id")
	if err != nil {
		return fmt.Errorf("creating a deployment pipeline: %w", err)
	}
	if createdID != id {
		deregister(ctx, kindPipeline, id)
		if err := cleanup.Register(ctx, cleanup.Resource{
			Kind: kindPipeline, ID: createdID, Actor: resourceActor, Description: "deployment pipeline " + createdID,
		}); err != nil {
			return fmt.Errorf("registering deployment pipeline %q for cleanup: %w", createdID, err)
		}
	}
	return tcontext.Set(ctx, keyPipelineID, createdID)
}

// createProjectBoundToPipeline creates a project that the scenario then keeps as its project, and
// binds it to the scenario's pipeline. The project is registered for cleanup before it is
// requested, ahead of the pipeline it is bound to.
func (s *Steps) createProjectBoundToPipeline(ctx context.Context) error {
	pipeline, err := requiredValue(ctx, keyPipelineID)
	if err != nil {
		return fmt.Errorf("the scenario has created no deployment pipeline: %w", err)
	}
	if err := s.createProjectOf(ctx, kindPromotionProject); err != nil {
		return err
	}
	project, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return err
	}
	var bound *httpx.Response
	err = retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, err := s.doAuthenticatedRaw(ctx, http.MethodPut,
				"/projects/"+url.PathEscape(project)+"/deployment-pipeline", map[string]string{"pipeline": pipeline})
			if err != nil {
				return nil, retry.Transient(err)
			}
			switch {
			case resp.StatusCode == http.StatusOK:
				return resp, nil
			case resp.StatusCode == http.StatusNotFound || resp.StatusCode >= http.StatusInternalServerError:
				// The project is created asynchronously and may not be readable yet.
				return nil, retry.Transient(responseError(resp))
			default:
				return nil, responseError(resp)
			}
		},
		func(resp *httpx.Response) bool {
			bound = resp
			return resp != nil
		},
		"binding project "+project+" to deployment pipeline "+pipeline)
	if err != nil {
		return err
	}
	got, err := stringField(bound, "pipeline")
	if err != nil {
		return fmt.Errorf("binding project %q to a deployment pipeline: %w", project, err)
	}
	if got != pipeline {
		return fmt.Errorf("project %q is bound to deployment pipeline %q, want %q", project, got, pipeline)
	}
	return nil
}
