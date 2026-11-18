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
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// This file runs a real gateway as a self-hosted gateway of the cloud: the block's
// platform-gateway component is started once more, on this machine, with the control-plane host
// and registration token the console's setup steps give a customer. Its controller dials the
// cloud console's /api-management/connect route, the platform marks the gateway active, and the
// scenario deploys to it and calls the API on the local runtime.

// kindLocalGateway stops a gateway this run started on this machine. It is deleted after the APIs
// (so an undeploy still reaches the running gateway) and before kindSelfHostedGateway (so the
// platform never deletes the registration of a gateway that is still connected to it).
var kindLocalGateway = cleanup.Kind{Name: "local-self-hosted-gateway", Order: cleanup.KindGateway.Order - 1}

const (
	// localGatewayComponent is the block component the local gateway is another instance of.
	localGatewayComponent = "platform-gateway"

	// localGatewayStateTimeout bounds how long the platform may take to report a local gateway
	// active after it starts, or inactive after it stops. The controller dials in within seconds
	// of starting; the rest is the platform noticing a closed connection.
	localGatewayStateTimeout = 3 * time.Minute

	// keyLocalGatewaySuffix names the scenario's local gateway in localGateways.
	keyLocalGatewaySuffix = "cloudLocalGatewaySuffix"
	// keyLocalGatewayEndpoint is the local runtime's HTTP address, which serves what is deployed.
	keyLocalGatewayEndpoint = "cloudLocalGatewayEndpoint"

	envControlPlaneHost     = "APIP_GW_CONTROLLER_CONTROLPLANE_HOST"
	envControlPlaneToken    = "APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN"
	envControlPlaneInsecure = "APIP_GW_CONTROLLER_CONTROLPLANE_INSECURE_SKIP_VERIFY"
)

// localGateways are the gateways this process started, by suffix, and the runtime origins they
// serve on. A local origin is a plain-http loopback address, which the data-plane check refuses for
// any other target: only an origin listed here is let through.
var localGateways = struct {
	sync.Mutex
	stacks  map[string]*frameworkruntime.ComposeStack
	origins map[string]bool
}{stacks: map[string]*frameworkruntime.ComposeStack{}, origins: map[string]bool{}}

// localGatewayCount numbers the local gateways, so each gets a suffix of its own on the block's
// network.
var localGatewayCount atomic.Int64

// registerLocalGatewaySteps binds the steps that run a self-hosted gateway on this machine.
func (s *Steps) registerLocalGatewaySteps(sc stepRegistrar) {
	sc.Step(`^I select the active APIP gateway's environment for a self-hosted gateway$`, s.selectActiveGatewayEnvironment)
	sc.Step(`^I start a local APIP gateway connected to the cloud control plane with a new registration token$`, s.startLocalGateway)
	sc.Step(`^the self-hosted APIP gateway should become (active|inactive)$`, s.awaitSelfHostedGatewayState)
	sc.Step(`^I use the local self-hosted APIP gateway as the active gateway$`, s.useLocalGatewayAsActive)
	sc.Step(`^I stop the local self-hosted APIP gateway$`, s.stopLocalGateway)
}

// selectActiveGatewayEnvironment creates the self-hosted gateway in the environment of the gateway
// "I find an active APIP gateway" chose. That environment is in the default project's deployment
// pipeline, so the default project's APIs can be deployed to a gateway in it.
func (s *Steps) selectActiveGatewayEnvironment(ctx context.Context) error {
	environment, err := requiredValue(ctx, keyGatewayEnv)
	if err != nil {
		return fmt.Errorf("find an active APIP gateway first: %w", err)
	}
	return tcontext.Set(ctx, keyEnvironmentName, environment)
}

// startLocalGateway mints a registration token for the scenario's self-hosted gateway and starts a
// gateway on this machine with it, configured as the console tells a customer to configure theirs.
// The token goes only into the gateway's environment file, which is removed when the gateway stops.
func (s *Steps) startLocalGateway(ctx context.Context) error {
	if s.topo == nil {
		return fmt.Errorf("starting a local gateway: the block's topology is required")
	}
	id, err := requiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	connect, err := s.topo.URL(corecatalog.Name, corecatalog.EndpointGatewayControlPlane)
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
	suffix := "-shgw" + strconv.FormatInt(localGatewayCount.Add(1), 10)
	stack, err := s.topo.LaunchComposeInstance(ctx, localGatewayComponent, suffix, localGatewayEnv(host, token))
	if err != nil {
		return fmt.Errorf("starting a local gateway for self-hosted gateway %q: %w", id, err)
	}
	endpoint, err := stack.Instance.URL("http")
	if err != nil {
		return fmt.Errorf("reading the local gateway's runtime address: %w", err)
	}
	endpoint = strings.TrimRight(endpoint, "/")
	origin, err := originOf(endpoint)
	if err != nil {
		return err
	}
	localGateways.Lock()
	localGateways.stacks[suffix] = stack
	localGateways.origins[origin] = true
	localGateways.Unlock()
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: kindLocalGateway, ID: suffix, Actor: resourceActor,
		Description: "local gateway " + suffix + " for self-hosted gateway " + id,
	}); err != nil {
		// The block stops the gateway at teardown in any case; stop it now so it does not stay
		// connected while the platform deletes its registration.
		return fmt.Errorf("registering local gateway %s for cleanup: %w (stopping it: %v)", suffix, err, stopLocalGateway(ctx, suffix))
	}
	if err := tcontext.Set(ctx, keyLocalGatewaySuffix, suffix); err != nil {
		return err
	}
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

// isLocalGatewayTarget reports whether target is on the runtime of a gateway this process started.
func isLocalGatewayTarget(target *url.URL) bool {
	if target == nil || target.User != nil || target.Host == "" {
		return false
	}
	origin := strings.ToLower(target.Scheme + "://" + target.Host)
	localGateways.Lock()
	defer localGateways.Unlock()
	return localGateways.origins[origin]
}

// awaitSelfHostedGatewayState waits until the platform reports the scenario's self-hosted gateway
// active (its controller holds a connection) or inactive (it no longer does).
func (s *Steps) awaitSelfHostedGatewayState(ctx context.Context, state string) error {
	id, err := requiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	want := state == "active"
	return retry.Await(ctx, retry.Options{Timeout: localGatewayStateTimeout, Interval: 3 * time.Second},
		func(ctx context.Context) (bool, error) {
			resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, selfHostedGatewayPath(id), nil)
			if err != nil {
				return false, retry.Transient(err)
			}
			if resp.StatusCode != http.StatusOK {
				return false, retry.Transient(responseError(resp))
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
	id, err := requiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	environment, err := requiredValue(ctx, keyEnvironmentName)
	if err != nil {
		return err
	}
	endpoint, err := requiredValue(ctx, keyLocalGatewayEndpoint)
	if err != nil {
		return fmt.Errorf("start a local APIP gateway first: %w", err)
	}
	for key, value := range map[string]string{keyGatewayID: id, keyGatewayEnv: environment, keyGatewayEndpoint: endpoint} {
		if err := tcontext.Set(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

// stopLocalGateway stops the scenario's local gateway, as a customer stopping theirs would.
func (s *Steps) stopLocalGateway(ctx context.Context) error {
	suffix, err := requiredValue(ctx, keyLocalGatewaySuffix)
	if err != nil {
		return err
	}
	if err := stopLocalGateway(ctx, suffix); err != nil {
		return err
	}
	deregister(ctx, kindLocalGateway, suffix)
	return nil
}

// stopLocalGateway stops the local gateway with the suffix and forgets it. A gateway already
// stopped is not an error.
func stopLocalGateway(ctx context.Context, suffix string) error {
	localGateways.Lock()
	stack := localGateways.stacks[suffix]
	delete(localGateways.stacks, suffix)
	localGateways.Unlock()
	if stack == nil {
		return nil
	}
	if err := stack.Stop(ctx); err != nil {
		localGateways.Lock()
		localGateways.stacks[suffix] = stack
		localGateways.Unlock()
		return fmt.Errorf("stopping local gateway %s: %w", suffix, err)
	}
	return nil
}

// localGatewayDeleter stops a local gateway a scenario left running.
func localGatewayDeleter(ctx context.Context, res cleanup.Resource) error {
	return stopLocalGateway(ctx, res.ID)
}
