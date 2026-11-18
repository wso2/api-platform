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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

// precondition names something the organisation must hold for a scenario to test the product
// instead of the organisation. A precondition asked of one project is written "<name>@<project>".
type precondition string

const (
	preProject        precondition = "project"
	preDefaultGateway precondition = "default-gateway"
	preEnvironment    precondition = "environment"
	preSecondEnv      precondition = "second-environment"
	prePipelineProj   precondition = "pipeline-project"
	prePipelineChain2 precondition = "pipeline-chain-2"
	prePipelineChain3 precondition = "pipeline-chain-3"
	preAPIPortal      precondition = "api-portal"
)

// projectScoped lists the preconditions asked of one of the suite's projects, so that each is
// prepared, and reported, once for every project.
var projectScoped = []precondition{preProject, prePipelineProj, prePipelineChain2, prePipelineChain3}

// preconditionOrder lists every precondition in the order a report names them. The projects come
// first: nothing else is looked at in an organisation that lacks the project a runner works in.
var preconditionOrder = []precondition{
	preProject, preDefaultGateway, preEnvironment, preSecondEnv,
	prePipelineProj, prePipelineChain2, prePipelineChain3, preAPIPortal,
}

// allPreconditions lists every precondition, a project-scoped one once for each project.
var allPreconditions = func() []precondition {
	var all []precondition
	for _, base := range preconditionOrder {
		if !slices.Contains(projectScoped, base) {
			all = append(all, base)
			continue
		}
		for _, project := range suiteProjects {
			all = append(all, scoped(base, project))
		}
	}
	return all
}()

// scoped returns a project-scoped precondition for one project.
func scoped(base precondition, project string) precondition {
	return precondition(string(base) + "@" + project)
}

// splitPrecondition returns the name of a precondition and the project it is asked of, which is
// empty for one that is not project-scoped.
func splitPrecondition(need precondition) (precondition, string) {
	base, project, _ := strings.Cut(string(need), "@")
	return precondition(base), project
}

// preconditionsNamed returns the preconditions a name stands for: the precondition itself, or for
// the name of a project-scoped one, its precondition for every project.
func preconditionsNamed(name string) ([]precondition, error) {
	name = strings.TrimSpace(name)
	if slices.Contains(allPreconditions, precondition(name)) {
		return []precondition{precondition(name)}, nil
	}
	if slices.Contains(projectScoped, precondition(name)) {
		var needs []precondition
		for _, project := range suiteProjects {
			needs = append(needs, scoped(precondition(name), project))
		}
		return needs, nil
	}
	return nil, fmt.Errorf("unknown precondition %q", name)
}

// errUnavailable marks a precondition the starter cannot meet with the token it has: the platform
// refuses the token the request that would build the resource, or the precondition depends on one
// that failed (whose own failure is reported). The scenarios that need it are skipped, and the run
// does not fail for it. Any other error from a precondition is a failure, the scenarios that need
// it are skipped too: the starter tried to build the resource, or to read it, and could not, or
// the organisation lacks an environment, a deployment pipeline or a promotion chain the suite
// selects, which the starter cannot create.
var errUnavailable = errors.New("precondition not met")

func unavailable(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errUnavailable, fmt.Sprintf(format, args...))
}

// refusesToken reports whether a status is the platform refusing the token itself.
func refusesToken(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden
}

// stepNeeds says what a scenario step asks of the organisation. A scenario needs what its steps
// say, so no scenario declares its needs twice. A project-scoped precondition is asked of the
// project the scenario's feature file works in.
var stepNeeds = []struct {
	step  *regexp.Regexp
	needs []precondition
}{
	{regexp.MustCompile(`^I find the APIP test project$`), []precondition{preProject}},
	{regexp.MustCompile(`^I read the APIP test project$`), []precondition{preProject}},
	{regexp.MustCompile(`^I create a synthetic (?:REST|GraphQL) API in the APIP test project`), []precondition{preProject}},
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
	// The configuration scenarios work with the gateway their setup scenario builds beside the
	// default gateway, in the default gateway's environment.
	{regexp.MustCompile(`^I work with the configuration of synthetic managed APIP gateway 1$`),
		[]precondition{preDefaultGateway}},
	{regexp.MustCompile(`^I find the pipeline-bound APIP project$`), []precondition{prePipelineProj}},
	{regexp.MustCompile(`^I find a promotion chain of 2 environments `), []precondition{prePipelineProj, prePipelineChain2}},
	{regexp.MustCompile(`^I find a promotion chain of 3 environments `), []precondition{prePipelineProj, prePipelineChain3}},
}

// needsOfSteps returns the preconditions the step texts ask for, each once, in report order. The
// project comes first, so a scenario that works in a project the organisation lacks is skipped
// before anything else of the organisation is looked at. A scenario that uses no project, such as a
// sign-in one, does not ask for one.
func needsOfSteps(steps []string, project string) []precondition {
	asked := map[precondition]bool{}
	for _, text := range steps {
		for _, entry := range stepNeeds {
			if entry.step.MatchString(strings.TrimSpace(text)) {
				for _, need := range entry.needs {
					if slices.Contains(projectScoped, need) {
						need = scoped(need, project)
					}
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

// needsOfScenario returns the preconditions a scenario's steps ask for, in the project of the
// scenario's feature file.
func needsOfScenario(scenario *godog.Scenario) []precondition {
	steps := make([]string, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		steps = append(steps, step.Text)
	}
	return needsOfSteps(steps, projectOfFeature(scenario.Uri))
}

// pipelineState is a project of the suite and the promotion paths of the
// deployment pipeline it is bound to.
type pipelineState struct {
	project  string
	pipeline string
	paths    []promotionPath
}

// prepare brings one precondition about, or says why it cannot be. A precondition the starter
// builds is built when the organisation lacks it; every other one is only checked.
func (s *Starter) prepare(ctx context.Context, platform organisationAPI, need precondition) error {
	base, project := splitPrecondition(need)
	if slices.Contains(projectScoped, base) && !slices.Contains(suiteProjects, project) {
		return fmt.Errorf("precondition %q names no project of the suite", need)
	}
	switch base {
	case preProject:
		return s.ensureProject(ctx, platform, project)
	case preDefaultGateway:
		return s.prepareGateway(ctx, platform)
	case preEnvironment:
		return checkAnyEnvironment(ctx, platform)
	case preSecondEnv:
		return s.checkSecondEnvironment(ctx, platform)
	case prePipelineProj:
		return s.checkPipelineProject(ctx, platform, project)
	case prePipelineChain2:
		return s.checkPipelineChain(ctx, platform, project, 2)
	case prePipelineChain3:
		return s.checkPipelineChain(ctx, platform, project, 3)
	case preAPIPortal:
		return s.preparePortal(ctx, platform)
	default:
		return fmt.Errorf("no way to prepare precondition %q", need)
	}
}

// ensureProject leaves the organisation holding a project the suite works in: one it has is used
// as it is, and one it lacks is created, since the suite's projects are the starter's to provide.
// Only the project is created. Its deployment pipeline is managed in the WSO2 Cloud console, so a
// project the starter created is bound to the default pipeline and its pipeline preconditions stay
// unmet until the console binds it to a pipeline of the suite. A project that is absent and cannot
// be created is a failure, except when the platform refuses the token the request.
func (s *Starter) ensureProject(ctx context.Context, platform organisationAPI, project string) error {
	resp, err := platform.get(ctx, "/projects/"+url.PathEscape(project))
	if err != nil {
		return err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return s.createProject(ctx, platform, project)
	default:
		return fmt.Errorf("reading project %q: %w", project, responseError(resp))
	}
}

// createProject asks the platform for a project of the suite and checks it was created under the
// requested handle. A conflict means another run created it first, so it is read instead. The
// project stays after the run, for later runs to use.
func (s *Starter) createProject(ctx context.Context, platform organisationAPI, project string) error {
	body, err := json.Marshal(map[string]string{
		"id":          project,
		"displayName": project,
		"description": "Project the APIP cloud integration tests work in. Created by the suite's starter and reused by every run. Do not delete.",
	})
	if err != nil {
		return err
	}
	resp, err := platform.do(ctx, http.MethodPost, "/projects", body)
	if err != nil {
		return err
	}
	switch resp.StatusCode {
	case http.StatusCreated, http.StatusOK:
		id, err := stringField(resp, "id")
		if err != nil {
			return fmt.Errorf("creating project %q: %w", project, err)
		}
		if id != project {
			return fmt.Errorf("creating project %q: the platform created project %q instead", project, id)
		}
		s.log.Info("created a project of the suite", "project", project)
		return nil
	case http.StatusConflict:
		found, err := platform.get(ctx, "/projects/"+url.PathEscape(project))
		if err != nil {
			return err
		}
		if found.StatusCode != http.StatusOK {
			return fmt.Errorf("creating project %q: %w", project, responseError(resp))
		}
		return nil
	default:
		err := fmt.Errorf("creating project %q: %w", project, responseError(resp))
		if refusesToken(resp.StatusCode) {
			return fmt.Errorf("%w: %w", errUnavailable, err)
		}
		return err
	}
}

// requireProject makes sure a project the suite works in is there, as unavailable when it is not.
func (s *Starter) requireProject(ctx context.Context, project string) error {
	if err := s.require(ctx, scoped(preProject, project)); err != nil {
		return unavailable("project %q is not available: %v", project, err)
	}
	return nil
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
		return err
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
		return err
	}
	return nil
}

// checkPipelineProject checks the project is bound to a deployment pipeline the suite may use (one
// named with the suite's prefix) that the platform lists, with at least one promotion path, and
// keeps the pipeline's promotion paths.
func (s *Starter) checkPipelineProject(ctx context.Context, platform organisationAPI, project string) error {
	if err := s.requireProject(ctx, project); err != nil {
		return err
	}
	pipeline, paths, err := boundPipelineOf(ctx, project, platform.get)
	if err != nil {
		if pipelineMissing(err) {
			return fmt.Errorf("the deployment pipeline of project %q cannot be read: %w", project, err)
		}
		return fmt.Errorf("reading the deployment pipeline of project %q: %w", project, err)
	}
	if !ownedPipeline(pipeline) {
		return fmt.Errorf("project %q is bound to deployment pipeline %q, which is not named with the prefix %q",
			project, pipeline, testResourcePrefix)
	}
	if len(environmentsOf(paths)) == 0 {
		return fmt.Errorf("the deployment pipeline %q of project %q has no promotion path", pipeline, project)
	}
	s.mu.Lock()
	s.pipelines[project] = pipelineState{project: project, pipeline: pipeline, paths: paths}
	s.mu.Unlock()
	return nil
}

// checkPipelineChain checks the project's deployment pipeline holds a promotion chain of the given
// length whose environments are ones the suite may use and each hold an active regular gateway.
func (s *Starter) checkPipelineChain(ctx context.Context, platform organisationAPI, project string, length int) error {
	if err := s.require(ctx, scoped(prePipelineProj, project)); err != nil {
		return unavailable("there is no pipeline-bound project to promote through: %v", err)
	}
	s.mu.Lock()
	pipeline := s.pipelines[project]
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
			if id, _ := item["id"].(string); strings.TrimSpace(id) == "" {
				continue
			}
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
		return fmt.Errorf("the deployment pipeline %q of project %q has no promotion chain of %d environments (named %q or starting with %q) with an active gateway each",
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

// pipelineBindingMissingError says the platform has no deployment pipeline binding for a project.
type pipelineBindingMissingError struct{ cause error }

func (e *pipelineBindingMissingError) Error() string { return e.cause.Error() }

func (e *pipelineBindingMissingError) Unwrap() error { return e.cause }

// pipelineMissing reports whether err says the pipeline, or the project's binding to it, is absent:
// the binding route answered 404, or the pipeline list does not hold the bound pipeline. A failure
// to read the list itself, a 404 from it included, is not an absent pipeline.
func pipelineMissing(err error) bool {
	var notListed *pipelineNotListedError
	var noBinding *pipelineBindingMissingError
	return errors.As(err, &notListed) || errors.As(err, &noBinding)
}
