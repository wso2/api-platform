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
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/setup/starter"
)

// defaultGatewayDisplayName is the base of the display name of the gateway the starter creates
// when the organisation has none the scenarios can use. The starter appends its own name suffix.
// The name is not that of a synthetic gateway, so the scenarios choose it. It is short on purpose:
// the platform limits a gateway's handle to 37 minus twice the environment name's length (15
// characters for "development"), and the handle is the display name folded to lower case.
const defaultGatewayDisplayName = "dgw"

// isStarterGateway reports whether any of the names is that of a gateway a starter created.
func isStarterGateway(names []string) bool {
	for _, name := range names {
		if strings.HasPrefix(strings.ToLower(name), defaultGatewayDisplayName+"-") ||
			strings.Contains(strings.ToLower(name), "-"+defaultGatewayDisplayName+"-") {
			return true
		}
	}
	return false
}

// preferredGatewayEnvironment is the environment the starter creates the default gateway in when
// it is an entry environment of the deployment pipeline of the project.
const preferredGatewayEnvironment = "development"

// gatewayReadyTimeout bounds how long the starter waits for a gateway it created, or one that is
// still being built, to become active. The platform builds a gateway through its own control plane,
// which takes minutes; the wait never gets less than five.
const gatewayReadyTimeout = 15 * time.Minute

// gatewayPollInterval is how often the starter checks a gateway that is being built.
const gatewayPollInterval = 10 * time.Second

// gatewayStatusActive is the status of a gateway that is connected and serves traffic.
const gatewayStatusActive = "ACTIVE"

// defaultPortalName is the base of the name and handle of the managed API Portal the starter
// creates when the organisation has none. The portal stays after the run: the finalizer empties it
// of what the run published and never deletes it, so later runs reuse it instead of building
// another. It is a valid portal handle on its own: lower case letters, digits and dashes, between
// 3 and 40 characters.
const defaultPortalName = "dap"

// starterCreatesPortal is false while the cloud intermittently fails to create a managed API
// Portal. With no active portal the starter does not build one: the apip-portal scenarios are
// skipped and the run fails, since the organisation lacks what the starter is meant to provide.
const starterCreatesPortal = false

// errStartNotCalled is what a precondition check returns before Start has recorded the platform.
var errStartNotCalled = errors.New("cloud run starter: Start must be called before a precondition is checked")

// preconditionTimeout bounds how long the starter works on one precondition for the scenarios
// that wait for it, and so how long a gateway or portal it builds has to become active.
const preconditionTimeout = 20 * time.Minute

// removalTimeout bounds how long the starter tries to remove a gateway or portal it created and
// could not make ready.
const removalTimeout = 5 * time.Minute

// gatewayOutcome says which gateway the starter settled on and whether it created it.
type gatewayOutcome struct {
	// ID is the gateway the scenarios will find.
	ID string
	// Environment is the environment the gateway belongs to.
	Environment string
	// Created is true when the starter built the gateway. The finalizer never removes it, so the
	// next run finds it as a gateway the organisation already has.
	Created bool
}

// portalOutcome says which managed API Portal the starter settled on and whether it created it.
type portalOutcome struct {
	// ID is the portal the scenarios will find.
	ID string
	// Created is true when the starter built the portal. The finalizer never deletes a portal, so
	// the next run finds it as a portal the organisation already has.
	Created bool
}

// Starter makes sure the organisation a cloud run talks to holds what a scenario needs, and says
// which scenarios the organisation cannot support. A scenario's steps name what it needs (see
// stepNeeds); the first scenario to need a precondition has it checked, and the answer is shared
// by every scenario and runner after it, so a precondition no scenario of the run needs is never
// looked at.
//
// A precondition is one of two kinds. The starter builds a regular gateway and a managed API
// Portal when the organisation has none, because the scenarios never build them: both take
// minutes. The other preconditions, such as the suite's projects, a second environment and a
// pipeline-bound project, are only checked, because the starter cannot create them: environments
// and deployment pipelines are managed in the WSO2 Cloud console. A scenario whose precondition is
// not met is skipped. When the starter tried to build a gateway or portal and could not, the run
// fails as well, and the half-built resource is removed; when the platform refuses the starter's
// token the request that would build it, or when the resource is only checked and absent, the run
// does not fail for it.
//
// What the starter builds stays after the run, named with a suffix of its own, and the finalizer
// never removes it. Later runs find it and use it as they would a resource the organisation
// already had.
type Starter struct {
	starter *starter.Starter
	client  *httpx.Client
	suffix  string
	log     *slog.Logger
	// pollInterval is how often a gateway or portal that is being built is checked; zero selects
	// the defaults.
	pollInterval time.Duration

	mu    sync.Mutex
	base  string
	token string

	// gateways holds, for each project whose default gateway was prepared, that gateway.
	gateways map[string]gatewayOutcome
	portal   portalOutcome
	// admin is the suite's portal administrator, or why there is none.
	admin portalAdminState
	// pipelines holds, for each project whose pipeline was checked, the pipeline and its promotion
	// paths.
	pipelines map[string]pipelineState
	// unmet holds why each precondition is unavailable, and failures why the starter failed to
	// prepare one. skipped counts the scenarios skipped for each precondition.
	unmet    map[precondition]error
	failures map[precondition]error
	skipped  map[precondition]int
}

// NewStarter returns a starter that has prepared no organisation yet.
func NewStarter() (*Starter, error) {
	generator, err := unique.NewGenerator()
	if err != nil {
		return nil, fmt.Errorf("cloud run starter: %w", err)
	}
	log := slog.Default()
	return &Starter{
		starter:   starter.New(log),
		client:    httpx.NewClient(httpx.Options{MaxRetries: 1}),
		suffix:    strings.ToLower(generator.Suffix()),
		log:       log,
		gateways:  map[string]gatewayOutcome{},
		pipelines: map[string]pipelineState{},
		unmet:     map[precondition]error{},
		failures:  map[precondition]error{},
		skipped:   map[precondition]int{},
	}, nil
}

// gatewayDisplayName is the display name of the default gateway this starter creates.
func (s *Starter) gatewayDisplayName() string {
	return defaultGatewayDisplayName + "-" + s.suffix
}

// portalName is the name, and so the handle, of the managed API Portal this starter creates.
func (s *Starter) portalName() string {
	return defaultPortalName + "-" + s.suffix
}

// Suffix is the name suffix of the gateway and portal this starter creates. The finalizer does not
// remove them; a person who wants them gone names the suffix to the setup command.
func (s *Starter) Suffix() string { return s.suffix }

// Gateway reports the gateway the starter settled on and whether it created it, for the first
// project of the suite that has one. It is empty until a scenario, or Prepare, needed a gateway.
func (s *Starter) Gateway() (id string, created bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, project := range suiteProjects {
		if outcome, ok := s.gateways[project]; ok && outcome.ID != "" {
			return outcome.ID, outcome.Created
		}
	}
	return "", false
}

// Portal reports the managed API Portal the starter settled on and whether it created it. It is
// empty until a scenario, or Prepare, needed a portal.
func (s *Starter) Portal() (id string, created bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.portal.ID, s.portal.Created
}

// Start records the organisation behind the block's platform endpoint, for the scenarios that need
// something of it, and then checks the projects the suite works in: it is the first check made
// with the token, so a runner whose project the organisation lacks is skipped before anything else
// of the organisation is looked at. Every other precondition is checked when a scenario first
// needs it.
func (s *Starter) Start(ctx context.Context, topo *frameworkruntime.Topology) error {
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	if err != nil {
		return err
	}
	if err := s.StartAt(ctx, base); err != nil {
		return err
	}
	s.CheckProjects(ctx)
	return nil
}

// CheckProjects makes sure the organisation holds each project the suite works in, creating one it
// lacks. A project that is missing and cannot be created skips the scenarios that work in it and
// fails the run; the checks are made once.
func (s *Starter) CheckProjects(ctx context.Context) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), preconditionTimeout)
	defer cancel()
	for _, project := range suiteProjects {
		// The outcome is kept by require; Failures and Report say what became of it.
		_ = s.require(ctx, scoped(preProject, project))
	}
}

// StartAt is Start for the platform API endpoint given as a URL, for a caller that has no topology.
func (s *Starter) StartAt(_ context.Context, base string) error {
	token, err := consoleToken()
	if err != nil {
		return fmt.Errorf("cloud run starter: preparing the organisation: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.base, s.token = base, token
	return nil
}

// Register skips each scenario whose preconditions the organisation does not meet. A scenario that
// needs nothing of the organisation, or whose needs are met, is left alone.
func (s *Starter) Register(sc *godog.ScenarioContext) {
	sc.Before(func(ctx context.Context, scenario *godog.Scenario) (context.Context, error) {
		return ctx, s.checkScenario(ctx, scenario.Name, needsOfScenario(scenario))
	})
}

// checkScenario makes sure the preconditions a scenario needs are met, and returns godog.ErrSkip
// for the first that is not.
func (s *Starter) checkScenario(ctx context.Context, name string, needs []precondition) error {
	if len(needs) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), preconditionTimeout)
	defer cancel()
	for _, need := range needs {
		err := s.require(ctx, need)
		if err == nil {
			continue
		}
		s.mu.Lock()
		s.skipped[need]++
		s.mu.Unlock()
		s.log.Warn("skipping: the organisation does not meet a precondition of the scenario",
			"scenario", name, "precondition", need, "reason", err)
		return godog.ErrSkip
	}
	return nil
}

// Preconditions names every precondition a scenario can need, in report order.
func Preconditions() []string {
	names := make([]string, 0, len(preconditionOrder))
	for _, need := range preconditionOrder {
		names = append(names, string(need))
	}
	return names
}

// Prepare checks the named preconditions, every one when none is named, building a gateway or
// portal where the organisation lacks one, for a caller that runs no scenarios. It returns what the
// starter failed to prepare; a precondition the organisation does not meet and the starter cannot
// build is not an error, and Report names it.
func (s *Starter) Prepare(ctx context.Context, names []string) error {
	needs := allPreconditions
	if len(names) > 0 {
		needs = make([]precondition, 0, len(names))
		for _, name := range names {
			named, err := preconditionsNamed(name)
			if err != nil {
				return err
			}
			needs = append(needs, named...)
		}
	}
	s.mu.Lock()
	started := s.base != ""
	s.mu.Unlock()
	if !started {
		return errStartNotCalled
	}
	for _, need := range needs {
		// The outcome is kept by require; Failures and Report say what became of it.
		_ = s.require(ctx, need)
	}
	return s.Failures()
}

// require prepares one precondition once, however many scenarios and runners ask for it, and
// returns nil when it is met. An error that matches errUnavailable means the organisation does not
// meet it; any other error means the starter failed to prepare it.
func (s *Starter) require(ctx context.Context, need precondition) error {
	s.mu.Lock()
	base, token := s.base, s.token
	s.mu.Unlock()
	if base == "" {
		return errStartNotCalled
	}
	// Scenarios ask for preconditions for the whole run, so the token is read again rather than
	// kept from the start; the one from the start is used only when it cannot be.
	if fresh, err := consoleToken(); err == nil {
		token = fresh
	}
	platform := organisationAPI{client: s.client, base: base, token: token, pollInterval: s.pollInterval}
	return s.starter.Ensure(ctx, base+"#"+string(need), []starter.Prerequisite{{
		Name: string(need),
		Ensure: func(ctx context.Context) error {
			err := s.prepare(ctx, platform, need)
			s.record(need, err)
			return err
		},
	}})
}

// record keeps why a precondition is not met.
func (s *Starter) record(need precondition, err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if errors.Is(err, errUnavailable) {
		s.unmet[need] = err
		return
	}
	s.failures[need] = err
}

// Failures returns what the starter tried to prepare and could not, or nil when it prepared
// everything a scenario asked for. A run that has failures must fail, though the scenarios that
// needed the resource were skipped.
func (s *Starter) Failures() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var errs []error
	for _, need := range allPreconditions {
		if err := s.failures[need]; err != nil {
			errs = append(errs, fmt.Errorf("preparing %s: %w", need, err))
		}
	}
	return errors.Join(errs...)
}

// Report describes, one line each, every precondition the organisation does not meet and how many
// scenarios were skipped for it. It is empty when every scenario found what it needed.
func (s *Starter) Report() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var lines []string
	switch {
	case s.admin.unavailable != nil:
		lines = append(lines, fmt.Sprintf("the portal administrator was not made, so the portal's leftovers were not removed: %v", s.admin.unavailable))
	case s.admin.browser != nil:
		for i, sweep := range s.admin.sweeps {
			when := "at the start"
			if i > 0 {
				when = "at the end"
			}
			lines = append(lines, fmt.Sprintf("portal administrator %s on %s swept the portal %s: %s", s.admin.admin.username, s.admin.environment, when, sweep))
		}
	}
	for _, need := range allPreconditions {
		if err := s.unmet[need]; err != nil {
			lines = append(lines, fmt.Sprintf("%s is not met, %d scenario(s) skipped: %v", need, s.skipped[need], err))
		}
		if err := s.failures[need]; err != nil {
			lines = append(lines, fmt.Sprintf("%s could not be prepared, %d scenario(s) skipped: %v", need, s.skipped[need], err))
		}
	}
	return lines
}

// prepareGateway leaves the organisation with an active regular gateway the deployment
// pipeline of the project accepts a deployment to. A gateway the starter built and could not make
// active is removed.
func (s *Starter) prepareGateway(ctx context.Context, platform organisationAPI, project string) error {
	if err := s.requireProject(ctx, project); err != nil {
		return err
	}
	outcome, err := ensureDefaultGateway(ctx, platform, s.gatewayDisplayName(), project, s.log)
	if err != nil {
		if outcome.Created {
			err = s.removeCreated(ctx, platform, "gateway", managedGatewaysCollection, outcome.ID, err)
		}
		return err
	}
	s.mu.Lock()
	s.gateways[project] = outcome
	s.mu.Unlock()
	return nil
}

// preparePortal leaves the organisation with an active managed API Portal. A portal the starter
// built and could not make active is removed.
func (s *Starter) preparePortal(ctx context.Context, platform organisationAPI) error {
	outcome, err := ensureDefaultPortal(ctx, platform, s.portalName(), s.log)
	if err != nil {
		if outcome.Created {
			err = s.removeCreated(ctx, platform, "managed API Portal", managedPortalsCollection, outcome.ID, err)
		}
		return err
	}
	s.mu.Lock()
	s.portal = outcome
	s.mu.Unlock()
	s.preparePortalAdmin(ctx, platform, outcome.ID)
	return nil
}

// portalAdminState is the suite's administrator of the shared portal, signed in, and what its sweeps
// removed; or why the starter made none.
type portalAdminState struct {
	browser     *portalBrowser
	admin       portalAdmin
	portal      *url.URL
	platform    organisationAPI
	environment string
	sweeps      []portalSweep
	unavailable error
}

// preparePortalAdmin makes the suite's portal administrator, signs it in and sweeps the leftovers of
// every run from the shared portal. It never fails the portal precondition: without an
// administrator the scenarios still run, the leftovers stay, and Report says why.
func (s *Starter) preparePortalAdmin(ctx context.Context, platform organisationAPI, portalID string) {
	state := portalAdminState{platform: platform}
	state.unavailable = s.makePortalAdmin(ctx, platform, portalID, &state)
	if state.unavailable != nil {
		s.log.Warn("the portal administrator was not made; leftovers in the portal stay", "portal", portalID, "reason", state.unavailable)
	}
	s.mu.Lock()
	s.admin = state
	s.mu.Unlock()
}

func (s *Starter) makePortalAdmin(ctx context.Context, platform organisationAPI, portalID string, state *portalAdminState) error {
	identity, err := identityPlatformFromEnv(s.client)
	if err != nil {
		return err
	}
	identity.pollInterval = s.pollInterval
	portal, environment, err := readPortalRecord(ctx, platform, portalID)
	if err != nil {
		return err
	}
	admin, err := ensurePortalAdmin(ctx, identity, environment, s.log)
	if err != nil {
		return err
	}
	browser, err := signInAs(ctx, s.client, portal, admin.username, admin.password)
	if err != nil {
		return err
	}
	state.browser, state.admin, state.portal, state.environment = browser, admin, portal, environment
	sweep, err := sweepPortal(ctx, browser, platformHoldsAPI(platform), sweepScope{now: time.Now()})
	state.sweeps = append(state.sweeps, sweep)
	if err != nil {
		s.log.Warn("the portal sweep at the start stopped; the one at the end tries again", "portal", portalID, "reason", err)
	}
	s.log.Info("swept the portal's leftovers of every run", "portal", portalID, "result", sweep.String())
	return nil
}

// SweepPortal removes the leftovers of every run from the shared portal with the administrator the
// starter made, signing it in again when the portal no longer takes its session. ownedByRun says
// which synthetic names this run made; their plans go whatever their age. It does nothing when the
// starter made no administrator, and returns what it removed.
func (s *Starter) SweepPortal(ctx context.Context, ownedByRun func(id string) bool) ([]string, error) {
	s.mu.Lock()
	state := s.admin
	s.mu.Unlock()
	if state.browser == nil {
		return nil, nil
	}
	// The sweep runs at the end of the run, long after the starter read its token.
	if fresh, err := consoleToken(); err == nil {
		state.platform.token = fresh
	}
	scope := sweepScope{ownedByRun: ownedByRun, now: time.Now()}
	sweep, err := sweepPortal(ctx, state.browser, platformHoldsAPI(state.platform), scope)
	if errors.Is(err, errPortalSessionEnded) {
		browser, signErr := signInAs(ctx, s.client, state.portal, state.admin.username, state.admin.password)
		if signErr != nil {
			err = fmt.Errorf("signing the portal administrator in again: %w", signErr)
		} else {
			state.browser = browser
			var again portalSweep
			again, err = sweepPortal(ctx, browser, platformHoldsAPI(state.platform), scope)
			sweep = sweep.merge(again)
		}
	}
	s.mu.Lock()
	s.admin.browser = state.browser
	s.admin.sweeps = append(s.admin.sweeps, sweep)
	s.mu.Unlock()
	if err == nil && (len(sweep.inUse) > 0 || len(sweep.failed) > 0) {
		err = fmt.Errorf("kept in use: %s; failed: %s", strings.Join(sweep.inUse, ", "), strings.Join(sweep.failed, ", "))
	}
	return sweep.removed, err
}

// removeCreated deletes a resource the starter created and could not make ready, and returns the
// failure with what became of the resource. The deletion has a context of its own, since the one
// the starter was prepared under may be spent.
func (s *Starter) removeCreated(ctx context.Context, platform organisationAPI, kind, collection, id string, cause error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), removalTimeout)
	defer cancel()
	if err := deleteLeftover(ctx, platform.client, platform.base, platform.token, collection, id); err != nil {
		s.log.Error("could not remove a resource the starter created and could not make ready", "kind", kind, "id", id, "error", err)
		return fmt.Errorf("%w; the %s %q the starter created could not be removed: %v", cause, kind, id, err)
	}
	s.log.Info("removed a resource the starter created and could not make ready", "kind", kind, "id", id)
	return fmt.Errorf("%w; the %s %q the starter created was removed", cause, kind, id)
}

// organisationAPI sends authenticated requests to the platform's API for one organisation.
type organisationAPI struct {
	client *httpx.Client
	base   string
	token  string
	// pollInterval is how often a gateway that is being built is checked; zero means
	// gatewayPollInterval.
	pollInterval time.Duration
}

func (p organisationAPI) do(ctx context.Context, method, path string, body []byte) (*httpx.Response, error) {
	return p.client.Do(ctx, httpx.Request{
		Method: method,
		URL:    cloudAPIURL(p.base, path),
		Headers: map[string]string{
			"Authorization": "Bearer " + p.token,
			"Content-Type":  "application/json",
		},
		Body: body,
	}, 0, 0)
}

func (p organisationAPI) get(ctx context.Context, path string) (*httpx.Response, error) {
	return p.do(ctx, http.MethodGet, path, nil)
}

// ensureDefaultGateway leaves the organisation with a regular gateway that is active and in an entry
// environment of the deployment pipeline of the project, the only kind of environment the platform
// accepts a deployment to without a promotion. A gateway that is still being
// built is waited for rather than duplicated; only when none exists is one created. The outcome
// names a gateway the starter created even when it did not become active, so the caller can remove
// it. An error that matches errUnavailable means the organisation lacks something the starter
// cannot build.
func ensureDefaultGateway(ctx context.Context, platform organisationAPI, displayName, project string, log *slog.Logger) (gatewayOutcome, error) {
	pipeline, paths, err := boundPipelineOf(ctx, project, platform.get)
	if err != nil {
		if pipelineMissing(err) {
			return gatewayOutcome{}, fmt.Errorf("the deployment pipeline of project %q cannot be read: %w", project, err)
		}
		return gatewayOutcome{}, fmt.Errorf("reading the deployment pipeline of project %q: %w", project, err)
	}
	if !ownedPipeline(pipeline) {
		return gatewayOutcome{}, fmt.Errorf("project %q is bound to deployment pipeline %q, which is not named with the prefix %q",
			project, pipeline, testResourcePrefix)
	}
	pin := strings.TrimSpace(os.Getenv(cloudGatewayPinEnv))
	filters := append(gatewayFilters(pin), inEntryEnvironments(paths))
	items, err := listCollectionQuery(ctx, platform.client, platform.base, platform.token, "/managed-gateways",
		url.Values{"functionalityType": {regularGatewayKind}})
	if err != nil {
		return gatewayOutcome{}, fmt.Errorf("listing the managed gateways: %w", err)
	}

	var building []gatewayOutcome
	for _, item := range items {
		id, _ := item["id"].(string)
		if id == "" || rejectedByAny(item, filters) != "" {
			continue
		}
		environment, _ := item["environment"].(string)
		switch status, _ := item["status"].(string); status {
		case gatewayStatusActive:
			log.Info("the organisation already has a gateway to deploy to; leaving it as it is", "gateway", id)
			return gatewayOutcome{ID: id, Environment: environment}, nil
		case "PROVISIONING":
			building = append(building, gatewayOutcome{ID: id, Environment: environment})
		}
	}
	if len(building) > 0 {
		log.Info("waiting for a gateway that is still being built", "gateway", building[0].ID)
		return building[0], awaitGatewayActive(ctx, platform, building[0].ID)
	}
	if pin != "" {
		return gatewayOutcome{}, fmt.Errorf("%s names gateway %q, which is not an active regular gateway in an entry environment of the deployment pipeline of project %q", cloudGatewayPinEnv, pin, project)
	}

	environment, err := defaultGatewayEnvironment(paths)
	if err != nil {
		return gatewayOutcome{}, err
	}
	id, err := createDefaultGateway(ctx, platform, environment, displayName)
	if err != nil {
		return gatewayOutcome{}, err
	}
	log.Info("created the default gateway; waiting for it to become active", "gateway", id, "environment", environment)
	return gatewayOutcome{ID: id, Environment: environment, Created: true}, awaitGatewayActive(ctx, platform, id)
}

// rejectedByAny returns why the first filter that rejects a gateway does, or "" when none does.
func rejectedByAny(item map[string]any, filters []gatewayFilter) string {
	for _, filter := range filters {
		if rejected := filter(item); rejected != "" {
			return rejected
		}
	}
	return ""
}

// defaultGatewayEnvironment picks the environment the default gateway is created in: the
// preferred one when it is an entry environment of the pipeline, otherwise the first entry
// environment in name order.
func defaultGatewayEnvironment(paths []promotionPath) (string, error) {
	entries := entryEnvironmentsOf(paths)
	if entries[preferredGatewayEnvironment] {
		return preferredGatewayEnvironment, nil
	}
	if len(entries) == 0 {
		return "", errors.New("the deployment pipeline has no environment to create a gateway in")
	}
	return sortedKeys(entries)[0], nil
}

// createDefaultGateway asks the platform for the default gateway and returns the id it answered
// with. A conflict that leaves a gateway of that name in the organisation means another run has
// just asked for the same gateway, so that gateway is the one to wait for. A platform that refuses
// the token the request is not one the starter can build a gateway on.
func createDefaultGateway(ctx context.Context, platform organisationAPI, environment, displayName string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"displayName":       displayName,
		"environment":       environment,
		"functionalityType": regularGatewayKind,
		"description":       "Gateway the APIP cloud integration tests deploy to. Created by the suite's starter and reused by every run. Do not delete.",
	})
	if err != nil {
		return "", err
	}
	resp, err := platform.do(ctx, http.MethodPost, "/managed-gateways", body)
	if err != nil {
		return "", err
	}
	switch resp.StatusCode {
	case http.StatusCreated, http.StatusAccepted:
		id, err := stringField(resp, "id")
		if err != nil {
			return "", fmt.Errorf("creating the default gateway: %w", err)
		}
		return id, nil
	case http.StatusConflict:
		// A conflict can also mean the platform refused the request for a reason of its own,
		// such as a recipe it cannot run, so the gateway must be there before it is waited for.
		id := expectedGatewayID(environment, displayName, "")
		found, err := platform.get(ctx, "/managed-gateways/"+url.PathEscape(id))
		if err != nil {
			return "", err
		}
		if found.StatusCode != http.StatusOK {
			return "", fmt.Errorf("creating the default gateway: %w", responseError(resp))
		}
		return id, nil
	default:
		err := fmt.Errorf("creating the default gateway: %w", responseError(resp))
		if refusesToken(resp.StatusCode) {
			return "", fmt.Errorf("%w: %w", errUnavailable, err)
		}
		return "", err
	}
}

// awaitGatewayActive waits until the managed gateway reports ACTIVE. A gateway that reports FAILED
// stops the wait at once with the platform's reason, since it will not recover.
func awaitGatewayActive(ctx context.Context, platform organisationAPI, id string) error {
	interval := platform.pollInterval
	if interval <= 0 {
		interval = gatewayPollInterval
	}
	return retry.Await(ctx, retry.Options{Timeout: gatewayReadyTimeout, Interval: interval},
		func(ctx context.Context) (string, error) {
			resp, err := platform.get(ctx, "/managed-gateways/"+url.PathEscape(id))
			if err != nil {
				return "", retry.Transient(err)
			}
			if resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusNotFound {
				return "", retry.Transient(responseError(resp))
			}
			if resp.StatusCode != http.StatusOK {
				return "", responseError(resp)
			}
			status, err := stringField(resp, "status")
			if err != nil {
				return "", err
			}
			if status == "FAILED" {
				reason, _ := stringField(resp, "statusReason")
				return "", fmt.Errorf("gateway %q failed to provision: %s", id, reason)
			}
			return status, nil
		},
		func(status string) bool { return status == gatewayStatusActive },
		"waiting for gateway "+id+" to become active")
}

// ensureDefaultPortal leaves the organisation with a managed API Portal that is active. A portal
// that is still being built is waited for rather than duplicated; only when the organisation has
// none that is active or being built is one created. The outcome names a portal the starter
// created even when it did not become active, so the caller can remove it.
func ensureDefaultPortal(ctx context.Context, platform organisationAPI, name string, log *slog.Logger) (portalOutcome, error) {
	items, err := listCollection(ctx, platform.client, platform.base, platform.token, managedPortalsCollection)
	if errors.Is(err, errCollectionNotServed) {
		return portalOutcome{}, unavailable("the platform does not serve managed API Portals")
	}
	if err != nil {
		return portalOutcome{}, fmt.Errorf("listing the managed API Portals: %w", err)
	}

	var building []string
	for _, item := range items {
		id, _ := item["id"].(string)
		if id == "" {
			continue
		}
		switch status, _ := item["status"].(string); status {
		case portalStatusActive:
			log.Info("the organisation already has a managed API Portal; leaving it as it is", "portal", id)
			return portalOutcome{ID: id}, nil
		case portalStatusPending:
			building = append(building, id)
		}
	}
	if len(building) > 0 {
		log.Info("waiting for a managed API Portal that is still being built", "portal", building[0])
		return portalOutcome{ID: building[0]}, awaitPortalActive(ctx, platform, building[0])
	}

	if !starterCreatesPortal {
		return portalOutcome{}, errors.New("the organisation has no active managed API Portal, and the starter does not create one")
	}
	id, err := createDefaultPortal(ctx, platform, name)
	if err != nil {
		return portalOutcome{}, err
	}
	log.Info("created the managed API Portal; waiting for it to become active", "portal", id)
	return portalOutcome{ID: id, Created: true}, awaitPortalActive(ctx, platform, id)
}

// createDefaultPortal asks the platform for the managed API Portal and returns the id it answered
// with. A conflict that leaves a portal of that handle in the organisation means another run has
// just asked for the same portal, so that portal is the one to wait for. A platform that refuses
// the token the request is not one the starter can build a portal on.
func createDefaultPortal(ctx context.Context, platform organisationAPI, name string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"name":        name,
		"handle":      name,
		"description": "API Portal the APIP cloud integration tests publish to, created by the suite's starter and reused by every run. Do not delete.",
	})
	if err != nil {
		return "", err
	}
	resp, err := platform.do(ctx, http.MethodPost, managedPortalsCollection, body)
	if err != nil {
		return "", err
	}
	switch resp.StatusCode {
	case http.StatusCreated, http.StatusAccepted:
		id, err := stringField(resp, "id")
		if err != nil {
			return "", fmt.Errorf("creating the managed API Portal: %w", err)
		}
		return id, nil
	case http.StatusConflict:
		// A conflict can also mean the platform refused the request for a reason of its own, so
		// the portal must be there before it is waited for.
		found, err := platform.get(ctx, managedPortalsCollection+"/"+url.PathEscape(name))
		if err != nil {
			return "", err
		}
		if found.StatusCode != http.StatusOK {
			return "", fmt.Errorf("creating the managed API Portal: %w", responseError(resp))
		}
		return name, nil
	default:
		err := fmt.Errorf("creating the managed API Portal: %w", responseError(resp))
		if refusesToken(resp.StatusCode) {
			return "", fmt.Errorf("%w: %w", errUnavailable, err)
		}
		return "", err
	}
}

// awaitPortalActive waits until the managed API Portal reports active. A portal that reports
// failed stops the wait at once, since it will not recover.
func awaitPortalActive(ctx context.Context, platform organisationAPI, id string) error {
	interval := platform.pollInterval
	if interval <= 0 {
		interval = portalPollInterval
	}
	return retry.Await(ctx, retry.Options{Timeout: portalBuildTimeout, Interval: interval},
		func(ctx context.Context) (string, error) {
			resp, err := platform.get(ctx, managedPortalsCollection+"/"+url.PathEscape(id))
			if err != nil {
				return "", retry.Transient(err)
			}
			if resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusNotFound {
				return "", retry.Transient(responseError(resp))
			}
			if resp.StatusCode != http.StatusOK {
				return "", responseError(resp)
			}
			status, err := stringField(resp, "status")
			if err != nil {
				return "", err
			}
			if status == portalStatusFailed {
				return "", fmt.Errorf("managed API Portal %q failed to provision", id)
			}
			return status, nil
		},
		func(status string) bool { return status == portalStatusActive },
		"waiting for managed API Portal "+id+" to become active")
}
