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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// A managed gateway takes minutes to build and each one costs a slot of the organisation's
// gateway allowance, so the scenarios that do not test a gateway's creation share the ones a
// runner created: they are numbered in the order the runner created them and a scenario picks
// the one it works with.

const (
	// handleProbeLength is the length of the over-long handle a probe sends to learn how long a
	// handle the platform accepts in an environment.
	handleProbeLength = 60

	keyManagedGatewayEnvPrefix = "cloudSyntheticManagedGatewayEnvironment"
)

// kindDefaultRestore hands an environment's default gateway back to the gateway that held it. It
// is cleaned up before every gateway, so a scenario that moved the default never leaves a gateway
// that cannot be deleted because it is the default.
var kindDefaultRestore = cleanup.Kind{Name: "default-gateway-restore", Order: 5}

// handleBudgetPattern finds the limit in the platform's refusal of a handle that is too long.
var handleBudgetPattern = regexp.MustCompile(`(\d+) characters or fewer`)

// registerManagedGatewayLifecycleSteps binds the steps that create gateways for a runner to share
// and check what the platform reports about them while they are built, used, and deleted.
func (s *Steps) registerManagedGatewayLifecycleSteps(sc stepRegistrar) {
	sc.Step(`^I choose a synthetic managed APIP gateway name at the handle limit of the active gateway's environment and the synthetic environment$`,
		s.chooseGatewayNameAtLimit)
	sc.Step(`^I request a synthetic managed APIP gateway that lasts for the runner with:$`, s.requestRunnerManagedGateway)
	sc.Step(`^the managed gateway request should be accepted$`, s.assertGatewayRequestAccepted)
	sc.Step(`^I work with synthetic managed APIP gateway (\d+)$`, s.selectSyntheticGateway)
	sc.Step(`^synthetic managed APIP gateway (\d+) should become active$`, s.waitSyntheticGatewayActive)
	sc.Step(`^the managed APIP gateway should be retrievable through the managed-gateway endpoint$`, s.assertGatewayRetrievable)
	sc.Step(`^synthetic managed APIP gateway (\d+) should still be retrievable$`, s.assertSyntheticGatewayRetrievable)
	sc.Step(`^the active APIP gateway should still be active$`, s.assertActiveGatewayStillActive)
	sc.Step(`^the environment should hold exactly one managed gateway with the chosen name$`, s.assertGatewayListedOnce)
	sc.Step(`^the managed APIP gateway should be registered under the id its name gives it$`, s.assertGatewayHasItsNameAsID)
	sc.Step(`^I update the managed APIP gateway with:$`, s.updateManagedGateway)
	sc.Step(`^the managed APIP gateway should report:$`, s.assertManagedGatewayReports)
	sc.Step(`^the update should have left the managed APIP gateway's identity unchanged$`, s.assertIdentityUnchanged)
	sc.Step(`^the managed APIP gateway should be bound to the environment it was created in$`, s.assertBoundToEnvironment)
	sc.Step(`^the managed APIP gateway should report a recorded provisioning state on its first read$`, s.assertFirstReadHasStatus)
	sc.Step(`^the managed APIP gateway status should progress from provisioning to built without failing$`, s.assertStatusProgressesToBuilt)
	sc.Step(`^the managed APIP gateway should not report its controller as connected yet$`, s.assertGatewayNotConnected)
	sc.Step(`^the managed APIP gateway should report its controller as connected$`, s.assertGatewayConnected)
	sc.Step(`^the managed APIP gateway should report (no|a) version$`, s.assertGatewayVersion)
	sc.Step(`^the managed APIP gateway endpoint should answer requests$`, s.assertGatewayEndpointAnswers)
	sc.Step(`^I make the managed APIP gateway the default of its environment$`, s.makeGatewayDefault)
	sc.Step(`^the managed APIP gateway should be the default of its environment$`, s.assertGatewayIsDefault)
	sc.Step(`^I hand the default of the environment back to the gateway that held it$`, s.handDefaultBack)
	sc.Step(`^I request the deletion of the managed APIP gateway$`, s.requestGatewayDeletion)
	sc.Step(`^I delete the managed APIP gateway$`, s.deleteManagedGateway)
	sc.Step(`^I request a synthetic managed APIP gateway once its name is free with:$`, s.requestManagedGatewayOnceItsNameIsFree)
	sc.Step(`^the managed APIP gateway should eventually be absent$`, s.verifyManagedGatewayDeleted)
	sc.Step(`^I deploy a build of the synthetic REST API to synthetic managed APIP gateway (\d+)$`, s.deployBuildToSyntheticGateway)
	sc.Step(`^the synthetic REST API should be deployed on synthetic managed APIP gateway (\d+)$`,
		func(ctx context.Context, n int) error { return s.assertDeployedOnSyntheticGateway(ctx, n, true) })
	sc.Step(`^the synthetic REST API should not be deployed on synthetic managed APIP gateway (\d+)$`,
		func(ctx context.Context, n int) error { return s.assertDeployedOnSyntheticGateway(ctx, n, false) })
	sc.Step(`^I undeploy the synthetic REST API from synthetic managed APIP gateway (\d+)$`, s.undeployFromSyntheticGateway)
	sc.Step(`^the synthetic REST API should not be deployed on the active gateway$`, s.assertNotDeployedOnActiveGateway)
}

// syntheticGatewayIDs lists the managed gateways the runner created, in creation order.
func syntheticGatewayIDs(ctx context.Context) []string {
	recorded := optionalValue(ctx, keySyntheticGateways)
	if recorded == "" {
		return nil
	}
	return strings.Split(recorded, ",")
}

// recordSyntheticGateway adds a gateway to the runner's list once, under the next number, and
// keeps its environment for the scenarios that pick it up.
func recordSyntheticGateway(ctx context.Context, id, environment string) error {
	ids := syntheticGatewayIDs(ctx)
	for _, known := range ids {
		if known == id {
			return nil
		}
	}
	ids = append(ids, id)
	n := len(ids)
	if err := tcontext.Set(ctx, numberedKey(keyManagedGatewayPrefix, n), id); err != nil {
		return err
	}
	if err := tcontext.Set(ctx, numberedKey(keyManagedGatewayEnvPrefix, n), environment); err != nil {
		return err
	}
	return tcontext.Set(ctx, keySyntheticGateways, strings.Join(ids, ","))
}

// syntheticGatewayAt resolves the nth (from one) gateway the runner created and its environment.
func syntheticGatewayAt(ctx context.Context, n int) (id, environment string, err error) {
	ids := syntheticGatewayIDs(ctx)
	if n < 1 || n > len(ids) {
		return "", "", fmt.Errorf("synthetic managed APIP gateway %d does not exist: the runner created %d", n, len(ids))
	}
	return ids[n-1], optionalValue(ctx, numberedKey(keyManagedGatewayEnvPrefix, n)), nil
}

// selectSyntheticGateway makes the nth gateway the one the managed gateway steps are about.
func (s *Steps) selectSyntheticGateway(ctx context.Context, n int) error {
	id, environment, err := syntheticGatewayAt(ctx, n)
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyManagedGatewayEnv, environment); err != nil {
		return err
	}
	// An update replaces the description, so a scenario that updates one keeps the marker by
	// starting its description with this value.
	if err := tcontext.Set(ctx, keyScenarioGatewayMark, scenarioGatewayDescription); err != nil {
		return err
	}
	return tcontext.Set(ctx, keyManagedGatewayID, id)
}

// handleBudgetOf reads the number of characters a handle may use from the platform's refusal of one
// that is too long.
func handleBudgetOf(body []byte) (int, error) {
	match := handleBudgetPattern.FindSubmatch(body)
	if match == nil {
		return 0, fmt.Errorf("the refusal does not say how long a handle may be: %s", strings.TrimSpace(string(body)))
	}
	budget, err := strconv.Atoi(string(match[1]))
	if err != nil || budget < 1 {
		return 0, fmt.Errorf("the refusal gives the handle limit as %q", match[1])
	}
	return budget, nil
}

// probeHandleBudget asks the platform for a gateway whose handle is far too long for the
// environment and reads the limit from its refusal. The refusal comes before anything is built, so
// the probe creates nothing; a probe that is accepted is cleaned up like any gateway and fails the
// step.
func (s *Steps) probeHandleBudget(ctx context.Context, environment string) (int, error) {
	err := s.requestManagedGatewayWith(ctx, map[string]string{
		"displayName": "handle limit probe",
		"environment": environment,
		"handle":      strings.Repeat("a", handleProbeLength),
	})
	if err != nil {
		return 0, err
	}
	resp, err := httpx.Published(ctx)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusBadRequest {
		return 0, fmt.Errorf("a %d-character handle was not refused in environment %q: %w", handleProbeLength, environment, responseError(resp))
	}
	return handleBudgetOf(resp.Body)
}

// chooseGatewayNameAtLimit picks a unique gateway name whose handle is as long as the tighter of
// the two environments allows, so the one name can be used in both and still sits at the edge of
// what the platform renders a service name from.
func (s *Steps) chooseGatewayNameAtLimit(ctx context.Context) error {
	limit := 0
	for _, key := range []string{keyGatewayEnv, keyEnvironmentName} {
		environment, err := requiredValue(ctx, key)
		if err != nil {
			return err
		}
		budget, err := s.probeHandleBudget(ctx, environment)
		if err != nil {
			return err
		}
		if limit == 0 || budget < limit {
			limit = budget
		}
	}
	name, err := freshGatewayName(ctx)
	if err != nil {
		return err
	}
	handle := handleOfLength(name, limit)
	if len(handle) != limit {
		return fmt.Errorf("cannot build a %d-character handle from %q", limit, name)
	}
	return tcontext.Set(ctx, keyManagedGatewayName, handle)
}

// handleOfLength builds a handle of exactly the given length from a unique name: it keeps the
// name's tail, which is what makes it unique, and pads short names with a letter.
func handleOfLength(name string, length int) string {
	if length < 1 {
		return ""
	}
	if len(name) >= length {
		name = name[len(name)-length:]
	} else {
		name += strings.Repeat("x", length-len(name))
	}
	if name[0] == '-' {
		name = "a" + name[1:]
	}
	return name
}

// freshGatewayName draws a unique gateway name without touching the scenario's chosen name.
func freshGatewayName(ctx context.Context) (string, error) {
	local, ok := tcontext.LocalOf(ctx)
	if !ok || local == nil {
		return "", errors.New("cannot draw a gateway name without runner context")
	}
	previous, _ := tcontext.ResolveString(ctx, keyManagedGatewayName)
	if err := generateResourceAndStore(ctx, syntheticGatewayBase, keyManagedGatewayName); err != nil {
		return "", err
	}
	name, err := requiredValue(ctx, keyManagedGatewayName)
	if err != nil {
		return "", err
	}
	if previous != "" {
		local.Set(keyManagedGatewayName, previous)
	}
	return name, nil
}

// fetchGatewayList reads the managed gateways, narrowed to one kind when given. The list includes
// a gateway that is still being built, which a read by id does not until the gateway is bound.
func (s *Steps) fetchGatewayList(ctx context.Context, kind string) ([]map[string]any, error) {
	path := "/managed-gateways"
	if kind != "" {
		path += "?functionalityType=" + url.QueryEscape(kind)
	}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, retry.Transient(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, retry.Transient(responseError(resp))
	}
	items, err := listItems(resp)
	if err != nil {
		return nil, retry.Transient(fmt.Errorf("decoding managed gateways: %w", err))
	}
	return items, nil
}

// fetchManagedGateway reads one managed gateway by id.
func (s *Steps) fetchManagedGateway(ctx context.Context, id string) (map[string]any, error) {
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, "/managed-gateways/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, retry.Transient(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, retry.Transient(responseError(resp))
	}
	var document map[string]any
	if err := json.Unmarshal(resp.Body, &document); err != nil {
		return nil, retry.Transient(fmt.Errorf("decoding managed gateway %q: %w", id, err))
	}
	return document, nil
}

// gatewayEntry reads what the platform reports about a gateway at any point of its life: by id once
// the gateway is bound to its environment, and from the listing, which reports a gateway that is
// still being built, before that.
func (s *Steps) gatewayEntry(ctx context.Context, id string) (map[string]any, error) {
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, "/managed-gateways/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, retry.Transient(err)
	}
	switch {
	case resp.StatusCode == http.StatusOK:
		var document map[string]any
		if err := json.Unmarshal(resp.Body, &document); err != nil {
			return nil, retry.Transient(fmt.Errorf("decoding managed gateway %q: %w", id, err))
		}
		return document, nil
	case resp.StatusCode == http.StatusNotFound:
		items, err := s.fetchGatewayList(ctx, "")
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if item["id"] == id {
				return item, nil
			}
		}
		return nil, retry.Transient(fmt.Errorf("managed gateway %q is neither bound nor listed: %s", id, resp.Describe()))
	default:
		return nil, retry.Transient(responseError(resp))
	}
}

// currentGateway is the gateway the managed gateway steps are about.
func currentGateway(ctx context.Context) (id, environment string, err error) {
	id, err = requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return "", "", err
	}
	return id, optionalValue(ctx, keyManagedGatewayEnv), nil
}

// assertGatewayRetrievable waits until the gateway can be read by id and checks what it reports.
// The platform binds a gateway to its environment partway through the build, and only from then on
// does a read by id answer.
func (s *Steps) assertGatewayRetrievable(ctx context.Context) error {
	id, environment, err := currentGateway(ctx)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: gatewayReadyTimeout, Interval: 5 * time.Second},
		func(ctx context.Context) (string, error) {
			document, err := s.fetchManagedGateway(ctx, id)
			if err != nil {
				return "waiting for the gateway to be bound: " + err.Error(), nil
			}
			return gatewayDocumentProblem(document, id, environment), nil
		},
		func(problem string) bool { return problem == "" },
		"waiting for managed gateway "+id+" to be retrievable by id")
}

// gatewayDocumentProblem says why a gateway read by id does not report what it was created as, or
// returns "" when it does.
func gatewayDocumentProblem(document map[string]any, id, environment string) string {
	if got, _ := document["id"].(string); got != id {
		return fmt.Sprintf("it reports id %q, want %q", got, id)
	}
	if got, _ := document["environment"].(string); got != environment {
		return fmt.Sprintf("it reports environment %q, want %q", got, environment)
	}
	if status, _ := document["status"].(string); !gatewayStatuses[status] {
		return fmt.Sprintf("it reports status %q", status)
	}
	if kind := gatewayKind(document); kind != regularGatewayKind {
		return fmt.Sprintf("it reports functionality type %q, want %q", kind, regularGatewayKind)
	}
	return ""
}

// gatewayKind is the kind of a listed gateway; a gateway with none recorded is a regular one.
func gatewayKind(item map[string]any) string {
	if kind, _ := item["functionalityType"].(string); kind != "" {
		return kind
	}
	return regularGatewayKind
}

// assertSyntheticGatewayRetrievable checks one gateway of the runner can still be read.
func (s *Steps) assertSyntheticGatewayRetrievable(ctx context.Context, n int) error {
	id, _, err := syntheticGatewayAt(ctx, n)
	if err != nil {
		return err
	}
	if _, err := s.fetchManagedGateway(ctx, id); err != nil {
		return fmt.Errorf("synthetic managed APIP gateway %d (%s): %w", n, id, err)
	}
	return nil
}

// assertActiveGatewayStillActive checks the shared gateway every other scenario deploys to was left
// as it was.
func (s *Steps) assertActiveGatewayStillActive(ctx context.Context) error {
	id, err := requiredValue(ctx, keyGatewayID)
	if err != nil {
		return err
	}
	document, err := s.fetchManagedGateway(ctx, id)
	if err != nil {
		return fmt.Errorf("the active gateway %q can no longer be read: %w", id, err)
	}
	if active, _ := document["isActive"].(bool); !active {
		return fmt.Errorf("the active gateway %q no longer reports its controller connected", id)
	}
	return nil
}

// gatewayProgress is what the platform reports about a managed gateway that is being built.
type gatewayProgress struct {
	ID string
	// Active is true once the gateway's controller is connected.
	Active bool
	// Status is PROVISIONING, FAILED, ACTIVE or DISCONNECTED; Reason is set only for FAILED.
	Status, Reason string
	// Config is the reconcile phase of the gateway's release (applying, healthy or failed), read
	// while the gateway is not active yet, or why it could not be read.
	Config string
}

// String renders the gateway with its status, reason and release phase for logs and failures.
func (p gatewayProgress) String() string {
	text := p.ID + ": " + p.Status
	if p.Reason != "" {
		text += " (" + p.Reason + ")"
	}
	if p.Config != "" {
		text += ", release " + p.Config
	}
	return text
}

// gatewayProgressOf reads how far the platform got building a managed gateway.
func (s *Steps) gatewayProgressOf(ctx context.Context, id string) (gatewayProgress, error) {
	document, err := s.gatewayEntry(ctx, id)
	if err != nil {
		return gatewayProgress{}, err
	}
	progress := gatewayProgress{ID: id, Status: "UNKNOWN"}
	progress.Active, _ = document["isActive"].(bool)
	if status, _ := document["status"].(string); status != "" {
		progress.Status = status
	}
	progress.Reason, _ = document["statusReason"].(string)
	if progress.Active {
		return progress, nil
	}
	config, err := s.doAuthenticatedRaw(ctx, http.MethodGet, "/managed-gateways/"+url.PathEscape(id)+"/configuration", nil)
	switch {
	case err != nil:
		progress.Config = "unreadable"
	case config.StatusCode != http.StatusOK:
		progress.Config = fmt.Sprintf("unreadable (HTTP %d)", config.StatusCode)
	default:
		progress.Config = releasePhase(config.Body)
	}
	return progress, nil
}

// releasePhase renders the reconcile phase of a gateway's configuration, with the platform's
// message when it gives one, or says that none was reported.
func releasePhase(body []byte) string {
	var document struct {
		Status struct {
			Phase   string `json:"phase"`
			Message string `json:"message"`
		} `json:"status"`
	}
	if json.Unmarshal(body, &document) != nil || document.Status.Phase == "" {
		return "not reported"
	}
	if document.Status.Message != "" {
		return document.Status.Phase + " (" + document.Status.Message + ")"
	}
	return document.Status.Phase
}

// waitGatewayActive waits until a gateway reports itself active. It logs each change of the
// gateway's status with the time since the wait began and stops at once when the gateway is
// reported FAILED.
func (s *Steps) waitGatewayActive(ctx context.Context, id string) error {
	started := time.Now()
	seen := ""
	return retry.Await(ctx, retry.Options{Timeout: gatewayReadyTimeout, Interval: gatewayPollInterval},
		func(ctx context.Context) (string, error) {
			progress, err := s.gatewayProgressOf(ctx, id)
			if err != nil {
				return "", err
			}
			if line := progress.String(); seen != line {
				seen = line
				slog.Info("managed gateway progress", "after", time.Since(started).Round(time.Second).String(), "gateway", line)
			}
			if progress.Status == "FAILED" {
				return "", fmt.Errorf("the platform reports managed gateway %s", progress)
			}
			if progress.Active {
				return "", nil
			}
			return progress.String(), nil
		},
		func(waiting string) bool { return waiting == "" },
		fmt.Sprintf("waiting up to %s for managed gateway %s to become active", gatewayReadyTimeout, id))
}

// waitSyntheticGatewayActive waits for the nth gateway of the runner. Gateways created together are
// built together, so the wait for the second is short.
func (s *Steps) waitSyntheticGatewayActive(ctx context.Context, n int) error {
	id, _, err := syntheticGatewayAt(ctx, n)
	if err != nil {
		return err
	}
	return s.waitGatewayActive(ctx, id)
}

// assertGatewayListedOnce checks the environment holds exactly one gateway with the scenario's
// chosen name. Counting by name rather than by id is what shows a repeated request that was
// answered with a second gateway under another id.
func (s *Steps) assertGatewayListedOnce(ctx context.Context) error {
	name, err := requiredValue(ctx, keyManagedGatewayName)
	if err != nil {
		return err
	}
	environment, err := requiredValue(ctx, keyManagedGatewayEnv)
	if err != nil {
		return err
	}
	items, err := s.fetchGatewayList(ctx, "")
	if err != nil {
		return err
	}
	var ids []string
	for _, item := range items {
		if item["displayName"] == name && item["environment"] == environment {
			id, _ := item["id"].(string)
			ids = append(ids, id)
		}
	}
	if len(ids) != 1 {
		return fmt.Errorf("environment %q holds %d managed gateways named %q (%s), want exactly one",
			environment, len(ids), name, strings.Join(ids, ", "))
	}
	return nil
}

// assertGatewayHasItsNameAsID checks the gateway is registered under the id its name gives it,
// which is what a name that was free again produces.
func (s *Steps) assertGatewayHasItsNameAsID(ctx context.Context) error {
	id, environment, err := currentGateway(ctx)
	if err != nil {
		return err
	}
	name, err := requiredValue(ctx, keyManagedGatewayName)
	if err != nil {
		return err
	}
	if want := expectedGatewayID(environment, name, ""); id != want {
		return fmt.Errorf("the gateway is registered as %q, want %q: its name was not free", id, want)
	}
	return nil
}

// gatewayNameReleaseTimeout bounds how long the platform may hold a deleted gateway's name, which
// it does while the gateway's infrastructure is removed.
const gatewayNameReleaseTimeout = 5 * time.Minute

// gatewayNameReleaseInterval is how often the request is made again while the name is held.
var gatewayNameReleaseInterval = 30 * time.Second

// requestManagedGatewayOnceItsNameIsFree requests the gateway until the platform registers it
// under the id its name gives it. A try answered with a suffixed id found the name still held, so
// that gateway is deleted at once and the request is made again.
func (s *Steps) requestManagedGatewayOnceItsNameIsFree(ctx context.Context, table *godog.Table) error {
	values, err := tableValues(table)
	if err != nil {
		return err
	}
	return s.requestManagedGatewayOnceItsNameIsFreeWith(ctx, values)
}

func (s *Steps) requestManagedGatewayOnceItsNameIsFreeWith(ctx context.Context, values map[string]string) error {
	return retry.Await(ctx, retry.Options{Timeout: gatewayNameReleaseTimeout, Interval: gatewayNameReleaseInterval},
		func(ctx context.Context) (string, error) {
			if err := s.requestManagedGatewayWith(ctx, values); err != nil {
				return "", err
			}
			if err := s.assertGatewayRequestAccepted(ctx); err != nil {
				return "", err
			}
			if s.assertGatewayHasItsNameAsID(ctx) == nil {
				return "", nil
			}
			id, _, err := currentGateway(ctx)
			if err != nil {
				return "", err
			}
			if err := s.deleteManagedGateway(ctx); err != nil {
				return "", err
			}
			deregister(ctx, cleanup.KindGateway, id)
			return "the name is still held; deleted " + id, nil
		},
		func(pending string) bool { return pending == "" },
		"waiting for the name of the gateway to be released")
}

// managedGatewayUpdateKeys lists the values a managed gateway update table may carry.
var managedGatewayUpdateKeys = map[string]bool{
	"displayName": true, "description": true, "isCritical": true, "properties": true,
}

// gatewayIdentityFields are the fields of a managed gateway an update must leave as they are.
var gatewayIdentityFields = []string{"id", "environment", "host", "functionalityType", "version"}

// updateManagedGateway replaces the gateway's mutable fields with the table's and publishes the
// response. The gateway's identity is kept first, for the step that checks the update left it.
func (s *Steps) updateManagedGateway(ctx context.Context, table *godog.Table) error {
	id, _, err := currentGateway(ctx)
	if err != nil {
		return err
	}
	values, err := tableValues(table)
	if err != nil {
		return err
	}
	body := map[string]any{}
	for key, raw := range values {
		if !managedGatewayUpdateKeys[key] {
			return fmt.Errorf("unsupported managed gateway update value %q", key)
		}
		value, err := expandCloudValue(ctx, raw)
		if err != nil {
			return fmt.Errorf("expanding %q: %w", key, err)
		}
		if body[key], err = typedGatewayValue(key, value); err != nil {
			return err
		}
	}
	before, err := s.fetchManagedGateway(ctx, id)
	if err != nil {
		return fmt.Errorf("reading managed gateway %q before the update: %w", id, err)
	}
	identity := map[string]any{}
	for _, field := range gatewayIdentityFields {
		identity[field] = before[field]
	}
	raw, err := json.Marshal(identity)
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyGatewayIdentity, string(raw)); err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodPut, "/managed-gateways/"+url.PathEscape(id), body)
	return err
}

// gatewayFieldValue reads a field of a gateway document, or a property when the field is named
// "properties.<name>".
func gatewayFieldValue(document map[string]any, field string) any {
	if name, nested := strings.CutPrefix(field, "properties."); nested {
		properties, _ := document["properties"].(map[string]any)
		return properties[name]
	}
	return document[field]
}

// assertManagedGatewayReports waits until the gateway reports every field the table names.
func (s *Steps) assertManagedGatewayReports(ctx context.Context, table *godog.Table) error {
	id, _, err := currentGateway(ctx)
	if err != nil {
		return err
	}
	values, err := tableValues(table)
	if err != nil {
		return err
	}
	expected := map[string]any{}
	for field, raw := range values {
		value, err := expandCloudValue(ctx, raw)
		if err != nil {
			return fmt.Errorf("expanding %q: %w", field, err)
		}
		expected[field] = parseConfigCell(value)
	}
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		func(ctx context.Context) (string, error) {
			document, err := s.fetchManagedGateway(ctx, id)
			if err != nil {
				return "", err
			}
			var differing []string
			for field, want := range expected {
				if got := gatewayFieldValue(document, field); !sameValue(got, want) {
					differing = append(differing, fmt.Sprintf("%s is %v, want %v", field, got, want))
				}
			}
			sort.Strings(differing)
			return strings.Join(differing, "; "), nil
		},
		func(differing string) bool { return differing == "" },
		"waiting for managed gateway "+id+" to report the updated fields")
}

// assertIdentityUnchanged checks the update left the gateway's identity as it was.
func (s *Steps) assertIdentityUnchanged(ctx context.Context) error {
	id, _, err := currentGateway(ctx)
	if err != nil {
		return err
	}
	var before map[string]any
	if err := json.Unmarshal([]byte(optionalValue(ctx, keyGatewayIdentity)), &before); err != nil {
		return fmt.Errorf("no identity was recorded before the update: %w", err)
	}
	document, err := s.fetchManagedGateway(ctx, id)
	if err != nil {
		return err
	}
	for _, field := range gatewayIdentityFields {
		if !sameValue(document[field], before[field]) {
			return fmt.Errorf("the update changed %s from %v to %v", field, before[field], document[field])
		}
	}
	return nil
}

// assertBoundToEnvironment checks the gateway is bound to the environment it was created in: its
// own record and the listing both say so, with a host that carries no scheme.
func (s *Steps) assertBoundToEnvironment(ctx context.Context) error {
	id, environment, err := currentGateway(ctx)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: gatewayReadyTimeout, Interval: 5 * time.Second},
		func(ctx context.Context) (string, error) {
			document, err := s.fetchManagedGateway(ctx, id)
			if err != nil {
				return "waiting for the gateway to be bound: " + err.Error(), nil
			}
			if got, _ := document["environment"].(string); got != environment {
				return "", fmt.Errorf("managed gateway %q reports environment %q, want %q", id, got, environment)
			}
			host, _ := document["host"].(string)
			if strings.TrimSpace(host) == "" || strings.Contains(host, "://") {
				return "", fmt.Errorf("managed gateway %q reports host %q, want a host without a scheme", id, host)
			}
			if _, ok := document["isDefault"].(bool); !ok {
				return "", fmt.Errorf("managed gateway %q does not report isDefault", id)
			}
			if endpoints, _ := document["endpoints"].([]any); len(endpoints) == 0 {
				return "the gateway lists no endpoints derived from its host", nil
			}
			items, err := s.fetchGatewayList(ctx, "")
			if err != nil {
				return "", err
			}
			for _, item := range items {
				if item["id"] == id {
					if item["environment"] != environment {
						return fmt.Sprintf("the listing shows environment %v", item["environment"]), nil
					}
					return "", nil
				}
			}
			return "the gateway is not in the listing", nil
		},
		func(problem string) bool { return problem == "" },
		"waiting for managed gateway "+id+" to be listed for environment "+environment+" with endpoints")
}

// firstReadStatus checks the status a gateway reports on the first read after it was created: a
// documented one that is not a failure.
func firstReadStatus(document map[string]any) error {
	id, _ := document["id"].(string)
	status, _ := document["status"].(string)
	if !gatewayStatuses[status] {
		return fmt.Errorf("managed gateway %q reports status %q on its first read, want a recorded provisioning state", id, status)
	}
	if status == "FAILED" {
		reason, _ := document["statusReason"].(string)
		return fmt.Errorf("managed gateway %q is FAILED on its first read: %s", id, reason)
	}
	return nil
}

// assertFirstReadHasStatus reads the gateway once, straight after it was created.
func (s *Steps) assertFirstReadHasStatus(ctx context.Context) error {
	id, _, err := currentGateway(ctx)
	if err != nil {
		return err
	}
	document, err := s.gatewayEntry(ctx, id)
	if err != nil {
		return err
	}
	return firstReadStatus(document)
}

// statusProgression records the statuses a gateway reported, in order, without repeats.
type statusProgression []string

// add records a status when it differs from the last one.
func (p *statusProgression) add(status string) {
	if n := len(*p); n == 0 || (*p)[n-1] != status {
		*p = append(*p, status)
	}
}

// check reports why a progression is not a gateway being built: it must never fail, and once
// built it must not go back to being built.
func (p statusProgression) check() error {
	built := false
	for _, status := range p {
		switch status {
		case "FAILED":
			return fmt.Errorf("the gateway reported FAILED (statuses seen: %s)", strings.Join(p, " -> "))
		case "PROVISIONING":
			if built {
				return fmt.Errorf("the gateway went back to PROVISIONING after it was built (statuses seen: %s)", strings.Join(p, " -> "))
			}
		default:
			built = true
		}
	}
	return nil
}

// assertStatusProgressesToBuilt reads the gateway until it stops being built and checks the
// statuses it passed through were those of a build that succeeds.
func (s *Steps) assertStatusProgressesToBuilt(ctx context.Context) error {
	id, _, err := currentGateway(ctx)
	if err != nil {
		return err
	}
	var seen statusProgression
	err = retry.Await(ctx, retry.Options{Timeout: gatewayReadyTimeout, Interval: gatewayPollInterval},
		func(ctx context.Context) (string, error) {
			document, err := s.gatewayEntry(ctx, id)
			if err != nil {
				return "", err
			}
			status, _ := document["status"].(string)
			if !gatewayStatuses[status] {
				return "", fmt.Errorf("managed gateway %q reports status %q", id, status)
			}
			seen.add(status)
			if err := seen.check(); err != nil {
				return "", err
			}
			return status, nil
		},
		func(status string) bool { return status != "PROVISIONING" },
		"waiting for managed gateway "+id+" to be built")
	if err != nil {
		return fmt.Errorf("%w (statuses seen: %s)", err, strings.Join(seen, " -> "))
	}
	return nil
}

// assertGatewayNotConnected checks registration alone did not mark the gateway connected.
func (s *Steps) assertGatewayNotConnected(ctx context.Context) error {
	id, _, err := currentGateway(ctx)
	if err != nil {
		return err
	}
	document, err := s.gatewayEntry(ctx, id)
	if err != nil {
		return err
	}
	if active, _ := document["isActive"].(bool); active {
		return fmt.Errorf("managed gateway %q reports its controller connected straight after registration", id)
	}
	if status, _ := document["status"].(string); status == "ACTIVE" {
		return fmt.Errorf("managed gateway %q reports ACTIVE straight after registration", id)
	}
	return nil
}

// assertGatewayConnected checks the gateway reports its controller connected.
func (s *Steps) assertGatewayConnected(ctx context.Context) error {
	id, _, err := currentGateway(ctx)
	if err != nil {
		return err
	}
	document, err := s.gatewayEntry(ctx, id)
	if err != nil {
		return err
	}
	if active, _ := document["isActive"].(bool); !active {
		return fmt.Errorf("managed gateway %q does not report its controller connected", id)
	}
	if status, _ := document["status"].(string); status != gatewayStatusActive {
		return fmt.Errorf("managed gateway %q is connected but reports status %q", id, status)
	}
	return nil
}

// assertGatewayVersion checks whether the gateway reports a version: none until its controller
// has reported one, and one afterwards.
func (s *Steps) assertGatewayVersion(ctx context.Context, state string) error {
	id, _, err := currentGateway(ctx)
	if err != nil {
		return err
	}
	document, err := s.gatewayEntry(ctx, id)
	if err != nil {
		return err
	}
	version, _ := document["version"].(string)
	switch state {
	case "no":
		if strings.TrimSpace(version) != "" {
			return fmt.Errorf("managed gateway %q reports version %q before its controller reported one", id, version)
		}
	default:
		if strings.TrimSpace(version) == "" {
			return fmt.Errorf("managed gateway %q still reports no version after its controller connected", id)
		}
	}
	return nil
}

// gatewayEndpoint reads the first approved data-plane endpoint of a managed gateway.
func (s *Steps) gatewayEndpoint(ctx context.Context, id string) (string, error) {
	document, err := s.gatewayEntry(ctx, id)
	if err != nil {
		return "", err
	}
	endpoints, _ := document["endpoints"].([]any)
	endpoint, ok, err := firstApprovedDataPlaneEndpoint(ctx, s, endpoints)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("managed gateway %q lists no endpoint that passed the data-plane host check: %v", id, endpoints)
	}
	return endpoint, nil
}

// assertGatewayEndpointAnswers waits until the endpoint of the gateway answers an HTTP request,
// which is what a gateway whose service is routable does whatever the path.
func (s *Steps) assertGatewayEndpointAnswers(ctx context.Context) error {
	id, _, err := currentGateway(ctx)
	if err != nil {
		return err
	}
	endpoint, err := s.gatewayEndpoint(ctx, id)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 5 * time.Second},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, err := s.client.Do(ctx, httpx.Request{Method: http.MethodGet, URL: endpoint + "/"}, 0, 0)
			if err != nil {
				return nil, retry.Transient(err)
			}
			return resp, nil
		},
		func(resp *httpx.Response) bool {
			return resp != nil && resp.StatusCode < http.StatusInternalServerError
		},
		"waiting for the endpoint of managed gateway "+id+" to answer")
}

// defaultGatewayOf finds the gateway an environment resolves to for a kind. At most one gateway
// per environment and kind carries the flag.
func defaultGatewayOf(items []map[string]any, environment, kind string) (string, error) {
	var found []string
	for _, item := range items {
		if item["environment"] != environment || gatewayKind(item) != kind {
			continue
		}
		if isDefault, _ := item["isDefault"].(bool); isDefault {
			id, _ := item["id"].(string)
			found = append(found, id)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return "", fmt.Errorf("environment %q has no default %s gateway", environment, kind)
	}
	return "", fmt.Errorf("environment %q has %d default %s gateways (%s), want one", environment, len(found), kind, strings.Join(found, ", "))
}

// updateBodyOf is the body of an update that leaves a gateway's mutable fields as the document
// reads and marks it the default of its environment. The platform replaces the mutable fields of a
// gateway with the ones it is given, so the ones that must stay have to be given back.
func updateBodyOf(document map[string]any) map[string]any {
	body := map[string]any{"isDefault": true}
	if name, _ := document["displayName"].(string); name != "" {
		body["displayName"] = name
	}
	if description, _ := document["description"].(string); description != "" {
		body["description"] = description
	}
	if critical, _ := document["isCritical"].(bool); critical {
		body["isCritical"] = true
	}
	if properties, _ := document["properties"].(map[string]any); len(properties) > 0 {
		body["properties"] = properties
	}
	return body
}

// makeGatewayDefault hands the default of the gateway's environment to the gateway, keeping the
// gateway that held it so the default can be handed back, and registers that handing back for the
// end of the scenario before anything changes. The response of the change is published.
func (s *Steps) makeGatewayDefault(ctx context.Context) error {
	id, environment, err := currentGateway(ctx)
	if err != nil {
		return err
	}
	items, err := s.fetchGatewayList(ctx, "")
	if err != nil {
		return err
	}
	previous, err := defaultGatewayOf(items, environment, regularGatewayKind)
	if err != nil {
		return err
	}
	if previous == id {
		return fmt.Errorf("gateway %q is already the default of environment %q", id, environment)
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: kindDefaultRestore, ID: previous, Actor: resourceActor, Description: "default gateway of " + environment + " back to " + previous,
	}); err != nil {
		return fmt.Errorf("registering the return of the default to %q: %w", previous, err)
	}
	if err := tcontext.Set(ctx, keyDefaultGatewayID, previous); err != nil {
		return err
	}
	document, err := s.fetchManagedGateway(ctx, id)
	if err != nil {
		return fmt.Errorf("reading managed gateway %q before it becomes the default: %w", id, err)
	}
	_, err = s.doAuthenticated(ctx, http.MethodPut, "/managed-gateways/"+url.PathEscape(id), updateBodyOf(document))
	return err
}

// assertGatewayIsDefault checks the gateway is the default of its environment.
func (s *Steps) assertGatewayIsDefault(ctx context.Context) error {
	id, _, err := currentGateway(ctx)
	if err != nil {
		return err
	}
	document, err := s.fetchManagedGateway(ctx, id)
	if err != nil {
		return fmt.Errorf("gateway %q cannot be read: %w", id, err)
	}
	if isDefault, _ := document["isDefault"].(bool); !isDefault {
		return fmt.Errorf("gateway %q is no longer its environment's default", id)
	}
	return nil
}

// handDefaultBack marks the gateway that held the default before makeGatewayDefault as the default
// again, and no longer leaves that to the end of the scenario.
func (s *Steps) handDefaultBack(ctx context.Context) error {
	previous, err := requiredValue(ctx, keyDefaultGatewayID)
	if err != nil {
		return fmt.Errorf("no default was handed over: %w", err)
	}
	if err := restoreDefault(ctx, s.doAuthenticatedRaw, previous); err != nil {
		return err
	}
	deregister(ctx, kindDefaultRestore, previous)
	return nil
}

// restoreDefault marks a gateway as the default of its environment, leaving its other fields as
// they are. A gateway that is gone has nothing to hand back to.
func restoreDefault(ctx context.Context, do func(context.Context, string, string, any) (*httpx.Response, error), id string) error {
	path := "/managed-gateways/" + url.PathEscape(id)
	current, err := do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if current.StatusCode == http.StatusNotFound {
		return nil
	}
	if current.StatusCode != http.StatusOK {
		return responseError(current)
	}
	var document map[string]any
	if err := json.Unmarshal(current.Body, &document); err != nil {
		return fmt.Errorf("decoding managed gateway %q: %w", id, err)
	}
	if isDefault, _ := document["isDefault"].(bool); isDefault {
		return nil
	}
	resp, err := do(ctx, http.MethodPut, path, updateBodyOf(document))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	return nil
}

// requestGatewayDeletion asks for the gateway to be deleted and publishes the response without
// requiring success, so a scenario can assert a refusal.
func (s *Steps) requestGatewayDeletion(ctx context.Context) error {
	id, _, err := currentGateway(ctx)
	if err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodDelete, "/managed-gateways/"+url.PathEscape(id), nil)
	return err
}

// deleteManagedGateway deletes the gateway, which the platform must accept: it answers 202 when
// the teardown is started and 204 when it was done at once.
func (s *Steps) deleteManagedGateway(ctx context.Context) error {
	id, _, err := currentGateway(ctx)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodDelete, "/managed-gateways/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("deleting managed gateway %q: %w", id, responseError(resp))
	}
	return nil
}

// gatewayDeletedTimeout bounds how long a deleted gateway may stay readable or listed.
const gatewayDeletedTimeout = 5 * time.Minute

// verifyManagedGatewayDeleted waits until the gateway can no longer be read and is no longer
// listed. The platform tears the gateway down before it removes the record, so this takes minutes.
func (s *Steps) verifyManagedGatewayDeleted(ctx context.Context) error {
	id, _, err := currentGateway(ctx)
	if err != nil {
		return err
	}
	err = retry.Await(ctx, retry.Options{Timeout: gatewayDeletedTimeout, Interval: gatewayPollInterval},
		func(ctx context.Context) (string, error) {
			resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, "/managed-gateways/"+url.PathEscape(id), nil)
			if err != nil {
				return "", retry.Transient(err)
			}
			if resp.StatusCode != http.StatusNotFound {
				return fmt.Sprintf("HTTP %d", resp.StatusCode), nil
			}
			items, err := s.fetchGatewayList(ctx, "")
			if err != nil {
				return "", err
			}
			for _, item := range items {
				if item["id"] == id {
					return "still listed", nil
				}
			}
			return "", nil
		},
		func(pending string) bool { return pending == "" },
		"waiting for managed gateway "+id+" to be deleted")
	if err != nil {
		return err
	}
	deregister(ctx, cleanup.KindGateway, id)
	return nil
}

// deployBuildToSyntheticGateway deploys a build of the synthetic REST API to one gateway of the
// runner through the project route, leaving every other gateway of the environment without it.
func (s *Steps) deployBuildToSyntheticGateway(ctx context.Context, n int) error {
	apiID, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return err
	}
	id, environment, err := syntheticGatewayAt(ctx, n)
	if err != nil {
		return err
	}
	if _, err := s.deployToGateway(ctx, apiID, environment, id); err != nil {
		return fmt.Errorf("deploying to synthetic managed APIP gateway %d (%s): %w", n, id, err)
	}
	return nil
}

// assertGatewayRequestAccepted checks the platform took the last gateway request: it started the
// build (202) or registered the gateway at once (201).
func (s *Steps) assertGatewayRequestAccepted(ctx context.Context) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if !gatewayRequestAccepted(resp.StatusCode) {
		return fmt.Errorf("the managed gateway request was not accepted: %w", responseError(resp))
	}
	return nil
}

// undeployFromSyntheticGateway undeploys the synthetic REST API from one gateway of the runner
// through the project route and requires the platform to accept it. A gateway that still holds a
// deployment cannot be deleted.
func (s *Steps) undeployFromSyntheticGateway(ctx context.Context, n int) error {
	id, environment, err := syntheticGatewayAt(ctx, n)
	if err != nil {
		return err
	}
	items, err := s.deploymentsOn(ctx, id)
	if err != nil {
		return err
	}
	scope, err := apiScopePath(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if status, _ := item["status"].(string); !stillDeployed[status] {
			continue
		}
		deploymentID, _ := item["deploymentId"].(string)
		resp, err := s.doAuthenticated(ctx, http.MethodPost, scope+"/deployments/"+url.PathEscape(deploymentID)+
			"/undeploy?environment="+url.QueryEscape(environment)+"&gatewayId="+url.QueryEscape(id), nil)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("undeploying from synthetic managed APIP gateway %d (%s): %w", n, id, responseError(resp))
		}
	}
	return nil
}

// deploymentsOn lists the deployments the synthetic REST API has on one gateway.
func (s *Steps) deploymentsOn(ctx context.Context, gatewayID string) ([]map[string]any, error) {
	apiID, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return nil, err
	}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet,
		"/rest-apis/"+url.PathEscape(apiID)+"/deployments?gatewayId="+url.QueryEscape(gatewayID), nil)
	if err != nil {
		return nil, retry.Transient(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, retry.Transient(responseError(resp))
	}
	items, err := listItems(resp)
	if err != nil {
		return nil, retry.Transient(fmt.Errorf("decoding APIP deployments: %w", err))
	}
	return items, nil
}

// assertDeployedOnSyntheticGateway waits until the synthetic REST API is, or is not, deployed on
// one gateway of the runner.
func (s *Steps) assertDeployedOnSyntheticGateway(ctx context.Context, n int, want bool) error {
	id, _, err := syntheticGatewayAt(ctx, n)
	if err != nil {
		return err
	}
	return s.awaitDeployedOn(ctx, id, want)
}

// assertNotDeployedOnActiveGateway checks the shared gateway holds no deployment of the synthetic
// REST API, which is what a deployment to another gateway of its environment leaves.
func (s *Steps) assertNotDeployedOnActiveGateway(ctx context.Context) error {
	id, err := requiredValue(ctx, keyGatewayID)
	if err != nil {
		return err
	}
	return s.awaitDeployedOn(ctx, id, false)
}

func (s *Steps) awaitDeployedOn(ctx context.Context, gatewayID string, want bool) error {
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		func(ctx context.Context) (int, error) {
			items, err := s.deploymentsOn(ctx, gatewayID)
			if err != nil {
				return 0, err
			}
			held := 0
			for _, item := range items {
				if status, _ := item["status"].(string); stillDeployed[status] {
					held++
				}
			}
			return held, nil
		},
		func(held int) bool { return (held > 0) == want },
		fmt.Sprintf("waiting for the synthetic REST API to be deployed=%t on gateway %s", want, gatewayID))
}

// restoreDefaultDeleter is the cleanup handler of kindDefaultRestore.
func restoreDefaultDeleter(topo *frameworkruntime.Topology, client *httpx.Client) cleanup.Deleter {
	return func(ctx context.Context, resource cleanup.Resource) error {
		base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
		if err != nil {
			return err
		}
		bearer, err := bearerToken(ctx)
		if err != nil {
			return err
		}
		return restoreDefault(ctx, func(ctx context.Context, method, path string, body any) (*httpx.Response, error) {
			var payload []byte
			if body != nil {
				var err error
				if payload, err = json.Marshal(body); err != nil {
					return nil, err
				}
			}
			return client.Do(ctx, httpx.Request{
				Method:  method,
				URL:     cloudAPIURL(base, path),
				Headers: map[string]string{"Authorization": "Bearer " + bearer, "Content-Type": "application/json"},
				Body:    payload,
			}, 0, 0)
		}, resource.ID)
	}
}
