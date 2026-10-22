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
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// gatewayActivationTimeout bounds how long a new managed gateway may take to connect its
// controller and report itself active. The platform reports a gateway as built (DISCONNECTED)
// within seconds, and the controller attaches after that, so a gateway that has not attached
// within minutes is not going to.
const gatewayActivationTimeout = 5 * time.Minute

// statusReasonCode is the only shape a deployment's status reason takes: an error code.
var statusReasonCode = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// createGatewaysInActiveEnvironment creates managed gateways in the environment of the active
// gateway, each registered for cleanup before it is requested, and records them as the
// scenario's synthetic gateways in place of any recorded before.
func (s *Steps) createGatewaysInActiveEnvironment(ctx context.Context, count int) error {
	environment, err := requiredValue(ctx, keyGatewayEnv)
	if err != nil {
		return fmt.Errorf("the active gateway does not report its environment: %w", err)
	}
	return s.createSyntheticGateways(ctx, count, environment)
}

// createGatewaysInSyntheticEnvironment creates managed gateways in the scenario's synthetic
// environment, on the same terms as createGatewaysInActiveEnvironment.
func (s *Steps) createGatewaysInSyntheticEnvironment(ctx context.Context, count int) error {
	environment, err := requiredValue(ctx, keyEnvironmentName)
	if err != nil {
		return fmt.Errorf("the scenario has created no synthetic environment: %w", err)
	}
	return s.createSyntheticGateways(ctx, count, environment)
}

// createSyntheticGateways creates the gateways in an environment and replaces the recorded list.
func (s *Steps) createSyntheticGateways(ctx context.Context, count int, environment string) error {
	if count < 1 {
		return fmt.Errorf("at least one synthetic gateway is required, got %d", count)
	}
	// Scenario state lives as long as the runner, so the gateways a previous scenario created
	// (and cleaned up) are not this scenario's.
	var ids []string
	for range count {
		id, err := s.createManagedGatewayIn(ctx, environment)
		if err != nil {
			return err
		}
		ids = append(ids, id)
		if err := tcontext.Set(ctx, keySyntheticGateways, strings.Join(ids, ",")); err != nil {
			return err
		}
	}
	return nil
}

// syntheticGatewayIDs lists the managed gateways the scenario created, in creation order.
func syntheticGatewayIDs(ctx context.Context) []string {
	recorded := optionalValue(ctx, keySyntheticGateways)
	if recorded == "" {
		return nil
	}
	return strings.Split(recorded, ",")
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
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, "/managed-gateways/"+url.PathEscape(id), nil)
	if err != nil {
		return gatewayProgress{}, retry.Transient(err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return gatewayProgress{}, fmt.Errorf("managed gateway %q no longer exists: %s", id, resp.Describe())
	}
	if resp.StatusCode != http.StatusOK {
		return gatewayProgress{}, retry.Transient(responseError(resp))
	}
	var document struct {
		IsActive     bool   `json:"isActive"`
		Status       string `json:"status"`
		StatusReason string `json:"statusReason"`
	}
	if err := json.Unmarshal(resp.Body, &document); err != nil {
		return gatewayProgress{}, retry.Transient(fmt.Errorf("decoding managed gateway %q: %w", id, err))
	}
	progress := gatewayProgress{ID: id, Active: document.IsActive, Status: document.Status, Reason: document.StatusReason}
	if progress.Status == "" {
		progress.Status = "UNKNOWN"
	}
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

// waitSyntheticGatewaysActive waits until every synthetic gateway reports itself active. It logs
// each change of a gateway's status with the time since the wait began, stops at once when a
// gateway is reported FAILED, and names every gateway's last status when the wait runs out.
func (s *Steps) waitSyntheticGatewaysActive(ctx context.Context) error {
	ids := syntheticGatewayIDs(ctx)
	if len(ids) == 0 {
		return fmt.Errorf("the scenario has created no synthetic gateway")
	}
	started := time.Now()
	seen := map[string]string{}
	return retry.Await(ctx, retry.Options{Timeout: gatewayActivationTimeout, Interval: 10 * time.Second},
		func(ctx context.Context) (string, error) {
			waiting := make([]string, 0, len(ids))
			for _, id := range ids {
				progress, err := s.gatewayProgressOf(ctx, id)
				if err != nil {
					return "", err
				}
				if line := progress.String(); seen[id] != line {
					seen[id] = line
					slog.Info("managed gateway progress", "after", time.Since(started).Round(time.Second).String(), "gateway", line)
				}
				if progress.Status == "FAILED" {
					return "", fmt.Errorf("the platform reports managed gateway %s", progress)
				}
				if !progress.Active {
					waiting = append(waiting, progress.String())
				}
			}
			sort.Strings(waiting)
			return strings.Join(waiting, "; "), nil
		},
		func(waiting string) bool { return waiting == "" },
		fmt.Sprintf("waiting up to %s for the synthetic gateways to become active", gatewayActivationTimeout))
}

func boolField(resp *httpx.Response, field string) (bool, error) {
	var document map[string]any
	if err := json.Unmarshal(resp.Body, &document); err != nil {
		return false, err
	}
	value, ok := document[field].(bool)
	if !ok {
		return false, fmt.Errorf("response field %q is missing or not a boolean", field)
	}
	return value, nil
}

// deployBuildPerGateway deploys a fresh build of the synthetic REST API to the active gateway and
// to every synthetic gateway through the platform route, which leaves each gateway holding a
// build of its own.
func (s *Steps) deployBuildPerGateway(ctx context.Context) error {
	apiID, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return err
	}
	activeGateway, err := requiredValue(ctx, keyGatewayID)
	if err != nil {
		return err
	}
	gateways := append([]string{activeGateway}, syntheticGatewayIDs(ctx)...)
	for index, gatewayID := range gateways {
		resp, err := s.doAuthenticated(ctx, http.MethodPost, "/rest-apis/"+url.PathEscape(apiID)+"/deployments",
			map[string]string{"name": fmt.Sprintf("%s-g%d", apiID, index), "base": "current", "gatewayId": gatewayID})
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("deploying to gateway %q: %w", gatewayID, responseError(resp))
		}
	}
	return nil
}

// assertBuildCount waits until the synthetic REST API holds exactly the given number of builds.
func (s *Steps) assertBuildCount(ctx context.Context, want int) error {
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		s.fetchBuilds,
		func(items []map[string]any) bool { return len(items) == want },
		fmt.Sprintf("waiting for the synthetic REST API to hold %d builds", want))
}

// assertDeploymentStatusReason checks that the synthetic REST API's deployment carries a status
// reason in the form of an error code.
func (s *Steps) assertDeploymentStatusReason(ctx context.Context) error {
	record, err := s.deploymentRecord(ctx, primaryAPI)
	if err != nil {
		return err
	}
	if record == nil {
		return fmt.Errorf("the deployment is not listed")
	}
	reason, _ := record["statusReason"].(string)
	if !statusReasonCode.MatchString(reason) {
		return fmt.Errorf("the deployment reports status reason %q, want an error code", reason)
	}
	return nil
}

// syntheticGatewayAt resolves the nth (from one) gateway the scenario created.
func syntheticGatewayAt(ctx context.Context, n int) (string, error) {
	ids := syntheticGatewayIDs(ctx)
	if n < 1 || n > len(ids) {
		return "", fmt.Errorf("synthetic gateway %d does not exist: the scenario created %d", n, len(ids))
	}
	return ids[n-1], nil
}

// deployBuildToSyntheticGateway deploys a build of the synthetic REST API to one synthetic gateway
// through the platform route, leaving every other gateway without it.
func (s *Steps) deployBuildToSyntheticGateway(ctx context.Context, n int) error {
	apiID, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return err
	}
	gatewayID, err := syntheticGatewayAt(ctx, n)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, "/rest-apis/"+url.PathEscape(apiID)+"/deployments",
		map[string]string{"name": fmt.Sprintf("%s-g%d", apiID, n), "base": "current", "gatewayId": gatewayID})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("deploying to synthetic gateway %d (%s): %w", n, gatewayID, responseError(resp))
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
// one synthetic gateway.
func (s *Steps) assertDeployedOnSyntheticGateway(ctx context.Context, n int, want bool) error {
	gatewayID, err := syntheticGatewayAt(ctx, n)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		func(ctx context.Context) (int, error) {
			items, err := s.deploymentsOn(ctx, gatewayID)
			return len(items), err
		},
		func(count int) bool { return (count > 0) == want },
		fmt.Sprintf("waiting for the synthetic REST API to be deployed=%t on synthetic gateway %d", want, n))
}

// deleteSyntheticGateway deletes one synthetic gateway, which must be accepted, and waits until it
// is gone.
func (s *Steps) deleteSyntheticGateway(ctx context.Context, n int) error {
	gatewayID, err := syntheticGatewayAt(ctx, n)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodDelete, "/managed-gateways/"+url.PathEscape(gatewayID), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("deleting synthetic gateway %d (%s): %w", n, gatewayID, responseError(resp))
	}
	if err := s.awaitNotFound(ctx, "/managed-gateways/"+url.PathEscape(gatewayID), "managed gateway "+gatewayID); err != nil {
		return err
	}
	deregister(ctx, cleanup.KindGateway, gatewayID)
	return nil
}

// assertSyntheticGatewayRetrievable checks one synthetic gateway can still be read.
func (s *Steps) assertSyntheticGatewayRetrievable(ctx context.Context, n int) error {
	gatewayID, err := syntheticGatewayAt(ctx, n)
	if err != nil {
		return err
	}
	if _, err := s.fetchManagedGateway(ctx, gatewayID); err != nil {
		return fmt.Errorf("synthetic gateway %d (%s): %w", n, gatewayID, err)
	}
	return nil
}
