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

package selfhosted

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

// KindLocalGateway stops a gateway this run started on this machine. It is deleted after the APIs
// (so an undeploy still reaches the running gateway) and before kindSelfHostedGateway (so the
// platform never deletes the registration of a gateway that is still connected to it).
var KindLocalGateway = cleanup.Kind{Name: "local-self-hosted-gateway", Order: cleanup.KindGateway.Order - 1}

const (
	// localGatewayComponent is the block component the local gateway is another instance of.
	localGatewayComponent = "platform-gateway"
	// localGatewayStateTimeout bounds how long the platform may take to report a local gateway
	// active after it starts, or inactive after it stops. The controller dials in within seconds
	// of starting; the rest is the platform noticing a closed connection.
	localGatewayStateTimeout = 3 * time.Minute
	// keyLocalGatewaySuffix names the scenario's local gateway in the runner's registry.
	keyLocalGatewaySuffix = "cloudLocalGatewaySuffix"
	// keyLocalGatewayEndpoint is the local runtime's HTTP address, which serves what is deployed.
	keyLocalGatewayEndpoint = "cloudLocalGatewayEndpoint"
	envControlPlaneHost     = "APIP_GW_CONTROLLER_CONTROLPLANE_HOST"
	envControlPlaneToken    = "APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN"
	envControlPlaneInsecure = "APIP_GW_CONTROLLER_CONTROLPLANE_INSECURE_SKIP_VERIFY"
)

// registerLocalGatewaySteps binds the steps that run a self-hosted gateway on this machine.
func (s *Steps) registerLocalGatewaySteps(sc cloudbase.StepRegistrar) {
	sc.Step(`^I select the active APIP gateway's environment for a self-hosted gateway$`, s.selectActiveGatewayEnvironment)
	sc.Step(`^I start a local APIP gateway connected to the cloud control plane with a new registration token$`, s.startLocalGateway)
	sc.Step(`^the self-hosted APIP gateway should become (active|inactive)$`, s.awaitSelfHostedGatewayState)
	sc.Step(`^I use the local self-hosted APIP gateway as the active gateway$`, s.useLocalGatewayAsActive)
	sc.Step(`^I stop the local self-hosted APIP gateway$`, s.stopLocalGateway)
}

// selectActiveGatewayEnvironment creates the self-hosted gateway in the environment of the gateway
// "I find an active APIP gateway" chose. That environment is in the deployment pipeline of the APIP test
// project, so the project's APIs can be deployed to a gateway in it.
func (s *Steps) selectActiveGatewayEnvironment(ctx context.Context) error {
	environment, err := cloudbase.RequiredValue(ctx, cloudbase.KeyGatewayEnv)
	if err != nil {
		return fmt.Errorf("find an active APIP gateway first: %w", err)
	}
	return tcontext.Set(ctx, cloudbase.KeyEnvironmentName, environment)
}

// startLocalGateway mints a registration token for the scenario's self-hosted gateway and starts a
// gateway on this machine with it, configured as the console tells a customer to configure theirs.
// The token goes only into the gateway's environment file, which is removed when the gateway stops.
func (s *Steps) startLocalGateway(ctx context.Context) error {
	if s.Topo == nil {
		return fmt.Errorf("starting a local gateway: the block's topology is required")
	}
	id, err := cloudbase.RequiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	connect, err := s.Topo.URL(corecatalog.Name, corecatalog.EndpointGatewayControlPlane)
	if err != nil {
		return fmt.Errorf("starting a local gateway: %w", err)
	}
	host, err := controlPlaneHost(connect)
	if err != nil {
		return err
	}
	token, err := s.mintRegistrationTokenValue(ctx, id)
	if err != nil {
		return err
	}
	registry, err := cloudbase.LocalGatewaysOf(ctx, true)
	if err != nil {
		return err
	}
	// The run-unique name keeps the gateway's service names apart from every other runner's on
	// the block's network.
	name, err := unique.Unique(ctx, "shgw")
	if err != nil {
		return err
	}
	suffix := "-" + strings.ToLower(strings.ReplaceAll(name, "_", "-"))
	stack, err := s.Topo.LaunchComposeInstance(ctx, localGatewayComponent, suffix, localGatewayEnv(host, token))
	if err != nil {
		return fmt.Errorf("starting a local gateway for self-hosted gateway %q: %w", id, err)
	}
	// Record and register the gateway before anything else can fail, so a failure below stops it
	// rather than leaving it connected while the platform deletes its registration.
	registry.Add(suffix, stack)
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: KindLocalGateway, ID: suffix, Actor: cloudbase.ResourceActor,
		Description: "local gateway " + suffix + " for self-hosted gateway " + id,
	}); err != nil {
		err = fmt.Errorf("registering local gateway %s for cleanup: %w", suffix, err)
		return errors.Join(err, registry.Stop(ctx, suffix))
	}
	if err := tcontext.Set(ctx, keyLocalGatewaySuffix, suffix); err != nil {
		return errors.Join(err, registry.Stop(ctx, suffix))
	}
	endpoint, err := stack.Instance.URL("http")
	if err != nil {
		err = fmt.Errorf("reading the local gateway's runtime address: %w", err)
		return errors.Join(err, registry.Stop(ctx, suffix))
	}
	endpoint = strings.TrimRight(endpoint, "/")
	origin, err := originOf(endpoint)
	if err != nil {
		return errors.Join(err, registry.Stop(ctx, suffix))
	}
	registry.Approve(suffix, origin)
	return tcontext.Set(ctx, keyLocalGatewayEndpoint, endpoint)
}

// localGatewayEnv is the gateway configuration the console's setup steps give a customer. The
// certificate of the cloud console host is verified, as a customer's gateway would.
func localGatewayEnv(host, token string) map[string]string {
	return map[string]string{
		envControlPlaneHost:     host,
		envControlPlaneToken:    token,
		envControlPlaneInsecure: "false",
	}
}

// controlPlaneHost turns the gateway control-plane endpoint into the value of
// APIP_GW_CONTROLLER_CONTROLPLANE_HOST: host and path with no scheme, since the controller adds
// wss:// and https:// itself.
func controlPlaneHost(endpoint string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("the gateway control-plane endpoint must be an https URL with a host, got %q", endpoint)
	}
	return parsed.Host + strings.TrimRight(parsed.Path, "/"), nil
}

// originOf returns scheme://host[:port] of an absolute URL.
func originOf(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("%q is not an absolute URL", raw)
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host), nil
}

// awaitSelfHostedGatewayState waits until the platform reports the scenario's self-hosted gateway
// active (its controller holds a connection) or inactive (it no longer does).
func (s *Steps) awaitSelfHostedGatewayState(ctx context.Context, state string) error {
	id, err := cloudbase.RequiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	want := state == "active"
	return retry.Await(ctx, retry.Options{Timeout: localGatewayStateTimeout, Interval: 3 * time.Second},
		func(ctx context.Context) (bool, error) {
			resp, err := s.DoAuthenticatedRaw(ctx, http.MethodGet, selfHostedGatewayPath(id), nil)
			if err != nil {
				return false, retry.Transient(err)
			}
			if resp.StatusCode != http.StatusOK {
				return false, retry.Transient(cloudbase.ResponseError(resp))
			}
			return selfHostedGatewayActive(resp.Body)
		}, func(active bool) bool { return active == want },
		fmt.Sprintf("waiting for self-hosted gateway %q to become %s", id, state))
}

// selfHostedGatewayActive reads isActive from a self-hosted gateway document.
func selfHostedGatewayActive(body []byte) (bool, error) {
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		return false, fmt.Errorf("decoding APIP self-hosted gateway: %w", err)
	}
	active, ok := document["isActive"].(bool)
	if !ok {
		return false, fmt.Errorf("the self-hosted gateway has no boolean isActive field")
	}
	return active, nil
}

// useLocalGatewayAsActive makes the scenario's self-hosted gateway the gateway the deployment steps
// work with: deployments name it, and its endpoint is the local runtime.
func (s *Steps) useLocalGatewayAsActive(ctx context.Context) error {
	id, err := cloudbase.RequiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	environment, err := cloudbase.RequiredValue(ctx, cloudbase.KeyEnvironmentName)
	if err != nil {
		return err
	}
	endpoint, err := cloudbase.RequiredValue(ctx, keyLocalGatewayEndpoint)
	if err != nil {
		return fmt.Errorf("start a local APIP gateway first: %w", err)
	}
	for key, value := range map[string]string{cloudbase.KeyGatewayID: id, cloudbase.KeyGatewayEnv: environment, cloudbase.KeyGatewayEndpoint: endpoint} {
		if err := tcontext.Set(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

// stopLocalGateway stops the scenario's local gateway, as a customer stopping theirs would.
func (s *Steps) stopLocalGateway(ctx context.Context) error {
	suffix, err := cloudbase.RequiredValue(ctx, keyLocalGatewaySuffix)
	if err != nil {
		return err
	}
	if err := stopLocalGateway(ctx, suffix); err != nil {
		return err
	}
	cloudbase.Deregister(ctx, KindLocalGateway, suffix)
	return nil
}

// stopLocalGateway stops the runner's local gateway with the suffix and forgets it. A gateway
// already stopped, or never started by this runner, is not an error.
func stopLocalGateway(ctx context.Context, suffix string) error {
	registry, err := cloudbase.LocalGatewaysOf(ctx, false)
	if err != nil || registry == nil {
		return err
	}
	return registry.Stop(ctx, suffix)
}

// LocalGatewayDeleter stops a local gateway a scenario left running.
func LocalGatewayDeleter(ctx context.Context, res cleanup.Resource) error {
	return stopLocalGateway(ctx, res.ID)
}
