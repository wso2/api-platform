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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
)

// precondition names something the organisation must hold for a scenario to test the product
// instead of the organisation.
type precondition string

const (
	preDefaultProject precondition = "default-project"
	preDefaultGateway precondition = "default-gateway"
	preEnvironment    precondition = "environment"
	preSecondEnv      precondition = "second-environment"
	prePipelineProj   precondition = "pipeline-project"
	prePipelineChain2 precondition = "pipeline-chain-2"
	prePipelineChain3 precondition = "pipeline-chain-3"
	preAPIPortal      precondition = "api-portal"
)

// allPreconditions lists every precondition in the order a report names them.
var allPreconditions = []precondition{
	preDefaultProject, preDefaultGateway, preEnvironment, preSecondEnv,
	prePipelineProj, prePipelineChain2, prePipelineChain3, preAPIPortal,
}

// preconditionNamed returns the precondition a name stands for.
func preconditionNamed(name string) (precondition, error) {
	for _, candidate := range allPreconditions {
		if string(candidate) == strings.TrimSpace(name) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unknown precondition %q", name)
}

// errUnavailable marks a precondition the organisation does not meet and the starter cannot meet
// with the token it has: the resource is absent and not one the starter builds, or the platform
// refuses the token the request that would build it. The scenarios that need it are skipped, and
// the run does not fail for it. Any other error from a precondition is a failure: the starter
// tried to build the resource, or to read it, and could not.
var errUnavailable = errors.New("precondition not met")

func unavailable(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errUnavailable, fmt.Sprintf(format, args...))
}

// refusesToken reports whether a status is the platform refusing the token itself.
func refusesToken(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden
}

// stepNeeds says what a scenario step asks of the organisation. A scenario needs what its steps
// say, so no scenario declares its needs twice.
var stepNeeds = []struct {
	step  *regexp.Regexp
	needs []precondition
}{
	{regexp.MustCompile(`^I find the default APIP project$`), []precondition{preDefaultProject}},
	{regexp.MustCompile(`^I find an active APIP gateway$`), []precondition{preDefaultGateway}},
	{regexp.MustCompile(`^I find an active managed API Portal$`), []precondition{preAPIPortal}},
	{regexp.MustCompile(`^I select an existing APIP environment for a (?:managed|self-hosted) gateway$`),
		[]precondition{preEnvironment}},
	{regexp.MustCompile(`^I select an existing APIP environment other than the active gateway's for a managed gateway$`),
		[]precondition{preDefaultGateway, preSecondEnv}},
	// A gateway a scenario works with is one an earlier scenario of the runner built, so it needs
	// what that scenario needs: the first and the second are built in the default gateway's and in
	// another environment.
	{regexp.MustCompile(`^I work with synthetic managed APIP gateway [12]$`),
		[]precondition{preDefaultGateway, preSecondEnv}},
	{regexp.MustCompile(`^I find the pipeline-bound APIP project$`), []precondition{prePipelineProj}},
	{regexp.MustCompile(`^I find a promotion chain of 2 environments `), []precondition{prePipelineProj, prePipelineChain2}},
	{regexp.MustCompile(`^I find a promotion chain of 3 environments `), []precondition{prePipelineProj, prePipelineChain3}},
}

// needsOfSteps returns the preconditions the step texts ask for, each once, in report order.
func needsOfSteps(steps []string) []precondition {
	asked := map[precondition]bool{}
	for _, text := range steps {
		for _, entry := range stepNeeds {
			if entry.step.MatchString(strings.TrimSpace(text)) {
				for _, need := range entry.needs {
					asked[need] = true
				}
			}
		}
	}
	var needs []precondition
	for _, need := range allPreconditions {
		if asked[need] {
			needs = append(needs, need)
		}
	}
	return needs
}

// needsOfScenario returns the preconditions a scenario's steps ask for.
func needsOfScenario(scenario *godog.Scenario) []precondition {
	steps := make([]string, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		steps = append(steps, step.Text)
	}
	return needsOfSteps(steps)
}

// pipelineState is the project CLOUD_IT_PIPELINE_PROJECT names and the promotion paths of the
// deployment pipeline it is bound to.
type pipelineState struct {
	project  string
	pipeline string
	paths    []promotionPath
}

// prepare brings one precondition about, or says why it cannot be. A precondition the starter
// builds is built when the organisation lacks it; every other one is only checked.
func (s *Starter) prepare(ctx context.Context, platform organisationAPI, need precondition) error {
	switch need {
	case preDefaultProject:
		return checkDefaultProject(ctx, platform)
	case preDefaultGateway:
		return s.prepareGateway(ctx, platform)
	case preEnvironment:
		return checkAnyEnvironment(ctx, platform)
	case preSecondEnv:
		return s.checkSecondEnvironment(ctx, platform)
	case prePipelineProj:
		return s.checkPipelineProject(ctx, platform)
	case prePipelineChain2:
		return s.checkPipelineChain(ctx, platform, 2)
	case prePipelineChain3:
		return s.checkPipelineChain(ctx, platform, 3)
	case preAPIPortal:
		return s.preparePortal(ctx, platform)
	default:
		return fmt.Errorf("no way to prepare precondition %q", need)
	}
}

func checkDefaultProject(ctx context.Context, platform organisationAPI) error {
	resp, err := platform.get(ctx, "/projects/"+url.PathEscape(defaultProjectID))
	if err != nil {
		return err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return unavailable("the organisation has no %q project", defaultProjectID)
	default:
		return fmt.Errorf("reading the %q project: %w", defaultProjectID, responseError(resp))
	}
}

func checkAnyEnvironment(ctx context.Context, platform organisationAPI) error {
	items, err := listCollection(ctx, platform.client, platform.base, platform.token, "/environments")
	if errors.Is(err, errCollectionNotServed) {
		return unavailable("the platform does not serve environments")
	}
	if err != nil {
		return fmt.Errorf("listing the environments: %w", err)
	}
	if _, err := usableEnvironment(items, hostsSuiteGateways(ctx, platform.do)); err != nil {
		return unavailable("%v", err)
	}
	return nil
}

// checkSecondEnvironment checks the organisation lists a non-production environment besides the one
// the default gateway is in, which the scenarios that build a second gateway need.
func (s *Starter) checkSecondEnvironment(ctx context.Context, platform organisationAPI) error {
	if err := s.require(ctx, preDefaultGateway); err != nil {
		return unavailable("there is no default gateway whose environment could be set aside: %v", err)
	}
	s.mu.Lock()
	active := s.gateway.Environment
	s.mu.Unlock()
	items, err := listCollection(ctx, platform.client, platform.base, platform.token, "/environments")
	if errors.Is(err, errCollectionNotServed) {
		return unavailable("the platform does not serve environments")
	}
	if err != nil {
		return fmt.Errorf("listing the environments: %w", err)
	}
	if _, err := otherEnvironment(items, active, hostsSuiteGateways(ctx, platform.do)); err != nil {
		return unavailable("%v", err)
	}
	return nil
}

// checkPipelineProject checks the project CLOUD_IT_PIPELINE_PROJECT names is one the suite may use
// (the default project or one named with the suite's prefix), exists and is bound to a deployment
// pipeline the platform lists that the suite may use too, and keeps the pipeline's promotion paths.
func (s *Starter) checkPipelineProject(ctx context.Context, platform organisationAPI) error {
	project := strings.TrimSpace(os.Getenv(pipelineProjectEnv))
	if project == "" {
		return unavailable("%s is not set", pipelineProjectEnv)
	}
	if !ownedProject(project) {
		return unavailable("%s names project %q, which is neither %q nor named with the prefix %q", pipelineProjectEnv, project, defaultProjectID, testResourcePrefix)
	}
	resp, err := platform.get(ctx, "/projects/"+url.PathEscape(project))
	if err != nil {
		return err
	}
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return unavailable("%s names project %q, which the organisation does not have", pipelineProjectEnv, project)
	default:
		return fmt.Errorf("reading project %q: %w", project, responseError(resp))
	}
	pipeline, paths, err := boundPipelineOf(ctx, project, platform.get)
	if err != nil {
		if pipelineMissing(err) {
			return unavailable("the deployment pipeline of project %q cannot be read: %v", project, err)
		}
		return fmt.Errorf("reading the deployment pipeline of project %q: %w", project, err)
	}
	if !ownedPipeline(pipeline) {
		return unavailable("project %q is bound to deployment pipeline %q, which is neither %q nor named with the prefix %q",
			project, pipeline, defaultPipelineName, testResourcePrefix)
	}
	if len(environmentsOf(paths)) == 0 {
		return unavailable("the deployment pipeline %q of project %q has no promotion path", pipeline, project)
	}
	s.mu.Lock()
	s.pipeline = pipelineState{project: project, pipeline: pipeline, paths: paths}
	s.mu.Unlock()
	return nil
}

// checkPipelineChain checks the pipeline project's deployment pipeline holds a promotion chain of
// the given length whose environments are ones the suite may use and each hold an active regular
// gateway.
func (s *Starter) checkPipelineChain(ctx context.Context, platform organisationAPI, length int) error {
	if err := s.require(ctx, prePipelineProj); err != nil {
		return unavailable("there is no pipeline-bound project to promote through: %v", err)
	}
	s.mu.Lock()
	pipeline := s.pipeline
	s.mu.Unlock()
	items, err := listCollectionQuery(ctx, platform.client, platform.base, platform.token, "/managed-gateways",
		url.Values{"functionalityType": {regularGatewayKind}})
	if err != nil {
		return fmt.Errorf("listing the managed gateways: %w", err)
	}
	holdsGateway := func(environment string) bool {
		if !ownedEnvironment(environment) {
			return false
		}
		filters := append(gatewayFilters(""), inEnvironment(environment))
		for _, item := range items {
			if active, _ := item["isActive"].(bool); !active {
				continue
			}
			if endpoints, _ := item["endpoints"].([]any); len(endpoints) == 0 {
				continue
			}
			if rejectedByAny(item, filters) == "" {
				return true
			}
		}
		return false
	}
	if promotionChain(pipeline.paths, length, holdsGateway) == nil {
		return unavailable("the deployment pipeline %q of project %q has no promotion chain of %d environments (named %q or starting with %q) with an active gateway each",
			pipeline.pipeline, pipeline.project, length, defaultEnvironmentName, testResourcePrefix)
	}
	return nil
}

// pipelineNotListedError says a project is bound to a deployment pipeline the platform does not
// list.
type pipelineNotListedError struct{ pipeline string }

func (e *pipelineNotListedError) Error() string {
	return fmt.Sprintf("deployment pipeline %q is not listed", e.pipeline)
}

// pipelineMissing reports whether err says the pipeline, or the project's binding to it, is absent.
func pipelineMissing(err error) bool {
	var notListed *pipelineNotListedError
	return errors.As(err, &notListed) || errorStatus(err) == http.StatusNotFound
}
