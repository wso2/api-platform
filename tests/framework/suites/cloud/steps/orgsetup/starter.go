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

package orgsetup

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
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/portal"
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

// defaultPortalName is the name and handle of the managed API Portal the starter creates when the
// organisation has none named with the suite's prefix. The portal stays after the run: the
// finalizer empties it of what the run published and never deletes it, so later runs reuse it
// instead of building another. It is a valid portal handle: lower case letters, digits and dashes,
// between 3 and 40 characters.
const defaultPortalName = cloudbase.TestResourcePrefix + "portal"

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
	admin *portal.Administrator
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
	return defaultPortalName
}

// Suffix is the name suffix of the gateway and portal this starter creates. The finalizer does not
// remove them; a person who wants them gone names the suffix to the setup command.
func (s *Starter) Suffix() string { return s.suffix }

// Gateway reports the gateway the starter settled on and whether it created it, for the first
// project of the suite that has one. It is empty until a scenario, or Prepare, needed a gateway.
func (s *Starter) Gateway() (id string, created bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, project := range cloudbase.SuiteProjectIDs {
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
	for _, project := range cloudbase.SuiteProjectIDs {
		// The outcome is kept by require; Failures and Report say what became of it.
		_ = s.require(ctx, scoped(preProject, project))
	}
}

// StartAt is Start for the platform API endpoint given as a URL, for a caller that has no topology.
func (s *Starter) StartAt(_ context.Context, base string) error {
	token := strings.TrimSpace(os.Getenv(cloudbase.CloudConsoleAccessTokenEnv))
	if token == "" {
		return fmt.Errorf("cloud run starter: environment variable %s is required to prepare the organisation", cloudbase.CloudConsoleAccessTokenEnv)
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
	platform := cloudbase.OrganisationAPI{Client: s.client, Base: base, Token: token, PollInterval: s.pollInterval}
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
	lines = append(lines, s.admin.Report()...)
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
func (s *Starter) prepareGateway(ctx context.Context, platform cloudbase.OrganisationAPI, project string) error {
	if err := s.requireProject(ctx, project); err != nil {
		return err
	}
	outcome, err := ensureDefaultGateway(ctx, platform, s.gatewayDisplayName(), project, s.log)
	if err != nil {
		if outcome.Created {
			err = s.removeCreated(ctx, platform, "gateway", cloudbase.ManagedGatewaysCollection, outcome.ID, err)
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
func (s *Starter) preparePortal(ctx context.Context, platform cloudbase.OrganisationAPI) error {
	outcome, err := ensureDefaultPortal(ctx, platform, s.portalName(), s.log)
	if err != nil {
		if outcome.Created {
			err = s.removeCreated(ctx, platform, "managed API Portal", cloudbase.ManagedPortalsCollection, outcome.ID, err)
		}
		return err
	}
	s.mu.Lock()
	s.portal = outcome
	s.mu.Unlock()
	s.preparePortalAdmin(ctx, platform, outcome.ID)
	return nil
}

// preparePortalAdmin makes the suite's portal administrator and sweeps the leftovers of every run from
// the shared portal. It never fails the portal precondition: without an administrator the scenarios
// still run, the leftovers stay, and Report says why.
func (s *Starter) preparePortalAdmin(ctx context.Context, platform cloudbase.OrganisationAPI, portalID string) {
	admin := portal.PrepareAdministrator(ctx, s.client, platform, portalID, s.pollInterval, s.log)
	s.mu.Lock()
	s.admin = admin
	s.mu.Unlock()
}

// SweepPortal removes the leftovers of every run from the shared portal with the administrator the
// starter made, signing it in again when the portal no longer takes its session. ownedByRun says
// which synthetic names this run made; their plans go whatever their age. It does nothing when the
// starter made no administrator, and returns what it removed.
func (s *Starter) SweepPortal(ctx context.Context, ownedByRun func(id string) bool) ([]string, error) {
	s.mu.Lock()
	admin := s.admin
	s.mu.Unlock()
	return admin.Sweep(ctx, ownedByRun)
}

// removeCreated deletes a resource the starter created and could not make ready, and returns the
// failure with what became of the resource. The deletion has a context of its own, since the one
// the starter was prepared under may be spent.
func (s *Starter) removeCreated(ctx context.Context, platform cloudbase.OrganisationAPI, kind, collection, id string, cause error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), removalTimeout)
	defer cancel()
	if err := deleteLeftover(ctx, platform.Client, platform.Base, platform.Token, collection, id); err != nil {
		s.log.Error("could not remove a resource the starter created and could not make ready", "kind", kind, "id", id, "error", err)
		return fmt.Errorf("%w; the %s %q the starter created could not be removed: %v", cause, kind, id, err)
	}
	s.log.Info("removed a resource the starter created and could not make ready", "kind", kind, "id", id)
	return fmt.Errorf("%w; the %s %q the starter created was removed", cause, kind, id)
}

// ensureDefaultGateway leaves the organisation with a regular gateway that is active and in an entry
// environment of the deployment pipeline of the project, the only kind of environment the platform
// accepts a deployment to without a promotion. A gateway that is still being
// built is waited for rather than duplicated; only when none exists is one created. The outcome
// names a gateway the starter created even when it did not become active, so the caller can remove
// it. An error that matches errUnavailable means the organisation lacks something the starter
// cannot build.
func ensureDefaultGateway(ctx context.Context, platform cloudbase.OrganisationAPI, displayName, project string, log *slog.Logger) (gatewayOutcome, error) {
	pipeline, paths, err := cloudbase.BoundPipelineOf(ctx, project, platform.Get)
	if err != nil {
		if pipelineMissing(err) {
			return gatewayOutcome{}, fmt.Errorf("the deployment pipeline of project %q cannot be read: %w", project, err)
		}
		return gatewayOutcome{}, fmt.Errorf("reading the deployment pipeline of project %q: %w", project, err)
	}
	if !cloudbase.OwnedPipeline(pipeline) {
		return gatewayOutcome{}, fmt.Errorf("project %q is bound to deployment pipeline %q, which is not named with the prefix %q",
			project, pipeline, cloudbase.TestResourcePrefix)
	}
	pin := strings.TrimSpace(os.Getenv(cloudbase.CloudGatewayPinEnv))
	filters := append(cloudbase.GatewayFilters(pin), cloudbase.InEntryEnvironments(paths))
	items, err := cloudbase.ListCollectionQuery(ctx, platform.Client, platform.Base, platform.Token, "/managed-gateways",
		url.Values{"functionalityType": {cloudbase.RegularGatewayKind}})
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
		case cloudbase.GatewayStatusActive:
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
		return gatewayOutcome{}, fmt.Errorf("%s names gateway %q, which is not an active regular gateway in an entry environment of the deployment pipeline of project %q", cloudbase.CloudGatewayPinEnv, pin, project)
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
func rejectedByAny(item map[string]any, filters []cloudbase.GatewayFilter) string {
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
func defaultGatewayEnvironment(paths []cloudbase.PromotionPath) (string, error) {
	entries := cloudbase.EntryEnvironmentsOf(paths)
	if entries[preferredGatewayEnvironment] {
		return preferredGatewayEnvironment, nil
	}
	if len(entries) == 0 {
		return "", errors.New("the deployment pipeline has no environment to create a gateway in")
	}
	return cloudbase.SortedKeys(entries)[0], nil
}

// createDefaultGateway asks the platform for the default gateway and returns the id it answered
// with. A conflict that leaves a gateway of that name in the organisation means another run has
// just asked for the same gateway, so that gateway is the one to wait for. A platform that refuses
// the token the request is not one the starter can build a gateway on.
func createDefaultGateway(ctx context.Context, platform cloudbase.OrganisationAPI, environment, displayName string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"displayName":       displayName,
		"environment":       environment,
		"functionalityType": cloudbase.RegularGatewayKind,
		"description":       "Gateway the APIP cloud integration tests deploy to. Created by the suite's starter and reused by every run. Do not delete.",
	})
	if err != nil {
		return "", err
	}
	resp, err := platform.Do(ctx, http.MethodPost, "/managed-gateways", body)
	if err != nil {
		return "", err
	}
	switch resp.StatusCode {
	case http.StatusCreated, http.StatusAccepted:
		id, err := cloudbase.StringField(resp, "id")
		if err != nil {
			return "", fmt.Errorf("creating the default gateway: %w", err)
		}
		return id, nil
	case http.StatusConflict:
		// A conflict can also mean the platform refused the request for a reason of its own,
		// such as a recipe it cannot run, so the gateway must be there before it is waited for.
		id := cloudbase.ExpectedGatewayID(environment, displayName, "")
		found, err := platform.Get(ctx, "/managed-gateways/"+url.PathEscape(id))
		if err != nil {
			return "", err
		}
		if found.StatusCode != http.StatusOK {
			return "", fmt.Errorf("creating the default gateway: %w", cloudbase.ResponseError(resp))
		}
		return id, nil
	default:
		err := fmt.Errorf("creating the default gateway: %w", cloudbase.ResponseError(resp))
		if refusesToken(resp.StatusCode) {
			return "", fmt.Errorf("%w: %w", errUnavailable, err)
		}
		return "", err
	}
}

// awaitGatewayActive waits until the managed gateway reports ACTIVE. A gateway that reports FAILED
// stops the wait at once with the platform's reason, since it will not recover.
func awaitGatewayActive(ctx context.Context, platform cloudbase.OrganisationAPI, id string) error {
	interval := platform.PollInterval
	if interval <= 0 {
		interval = cloudbase.GatewayPollInterval
	}
	return retry.Await(ctx, retry.Options{Timeout: cloudbase.GatewayReadyTimeout, Interval: interval},
		func(ctx context.Context) (string, error) {
			resp, err := platform.Get(ctx, "/managed-gateways/"+url.PathEscape(id))
			if err != nil {
				return "", retry.Transient(err)
			}
			if resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusNotFound {
				return "", retry.Transient(cloudbase.ResponseError(resp))
			}
			if resp.StatusCode != http.StatusOK {
				return "", cloudbase.ResponseError(resp)
			}
			status, err := cloudbase.StringField(resp, "status")
			if err != nil {
				return "", err
			}
			if status == "FAILED" {
				reason, _ := cloudbase.StringField(resp, "statusReason")
				return "", fmt.Errorf("gateway %q failed to provision: %s", id, reason)
			}
			return status, nil
		},
		func(status string) bool { return status == cloudbase.GatewayStatusActive },
		"waiting for gateway "+id+" to become active")
}

// ensureDefaultPortal leaves the organisation with a managed API Portal that is active. A portal
// that is still being built is waited for rather than duplicated; only when the organisation has
// none that is active or being built is one created. The outcome names a portal the starter
// created even when it did not become active, so the caller can remove it.
func ensureDefaultPortal(ctx context.Context, platform cloudbase.OrganisationAPI, name string, log *slog.Logger) (portalOutcome, error) {
	items, err := cloudbase.ListCollection(ctx, platform.Client, platform.Base, platform.Token, cloudbase.ManagedPortalsCollection)
	if errors.Is(err, cloudbase.ErrCollectionNotServed) {
		return portalOutcome{}, unavailable("the platform does not serve managed API Portals")
	}
	if err != nil {
		return portalOutcome{}, fmt.Errorf("listing the managed API Portals: %w", err)
	}

	var building []string
	for _, item := range items {
		id, _ := item["id"].(string)
		if id == "" || !cloudbase.OwnedPortal(item) {
			continue
		}
		switch status, _ := item["status"].(string); status {
		case cloudbase.PortalStatusActive:
			log.Info("the organisation already has a managed API Portal; leaving it as it is", "portal", id)
			return portalOutcome{ID: id}, nil
		case cloudbase.PortalStatusPending:
			building = append(building, id)
		}
	}
	if len(building) > 0 {
		log.Info("waiting for a managed API Portal that is still being built", "portal", building[0])
		return portalOutcome{ID: building[0]}, cloudbase.AwaitPortalActive(ctx, platform, building[0])
	}

	if !starterCreatesPortal {
		return portalOutcome{}, fmt.Errorf("the organisation has no active managed API Portal named with the prefix %q (such as %q), and the starter does not create one", cloudbase.TestResourcePrefix, defaultPortalName)
	}
	id, err := createDefaultPortal(ctx, platform, name)
	if err != nil {
		return portalOutcome{}, err
	}
	log.Info("created the managed API Portal; waiting for it to become active", "portal", id)
	return portalOutcome{ID: id, Created: true}, cloudbase.AwaitPortalActive(ctx, platform, id)
}

// createDefaultPortal asks the platform for the managed API Portal and returns the id it answered
// with. A conflict that leaves a portal of that handle in the organisation means another run has
// just asked for the same portal, so that portal is the one to wait for. A platform that refuses
// the token the request is not one the starter can build a portal on.
func createDefaultPortal(ctx context.Context, platform cloudbase.OrganisationAPI, name string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"name":        name,
		"handle":      name,
		"description": "API Portal the APIP cloud integration tests publish to, created by the suite's starter and reused by every run. Do not delete.",
	})
	if err != nil {
		return "", err
	}
	resp, err := platform.Do(ctx, http.MethodPost, cloudbase.ManagedPortalsCollection, body)
	if err != nil {
		return "", err
	}
	switch resp.StatusCode {
	case http.StatusCreated, http.StatusAccepted:
		id, err := cloudbase.StringField(resp, "id")
		if err != nil {
			return "", fmt.Errorf("creating the managed API Portal: %w", err)
		}
		return id, nil
	case http.StatusConflict:
		// A conflict can also mean the platform refused the request for a reason of its own, so
		// the portal must be there before it is waited for.
		found, err := platform.Get(ctx, cloudbase.ManagedPortalsCollection+"/"+url.PathEscape(name))
		if err != nil {
			return "", err
		}
		if found.StatusCode != http.StatusOK {
			return "", fmt.Errorf("creating the managed API Portal: %w", cloudbase.ResponseError(resp))
		}
		return name, nil
	default:
		err := fmt.Errorf("creating the managed API Portal: %w", cloudbase.ResponseError(resp))
		if refusesToken(resp.StatusCode) {
			return "", fmt.Errorf("%w: %w", errUnavailable, err)
		}
		return "", err
	}
}
