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

package gateway

import (
	"context"
	"errors"
	"net/http"
	"time"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

const (
	keyManagedGatewayID    = "cloudManagedGatewayID"
	keyManagedGatewayEnv   = "cloudManagedGatewayEnvironment"
	keyScenarioGatewayMark = "cloudScenarioGatewayMarker"
	keyManagedGatewayName  = "cloudManagedGatewayName"
	keyConfigGatewayID     = "cloudConfigGatewayID"
	keyConfigAttempts      = "cloudConfigAttempts"
	// keySyntheticGateways lists the managed gateways the runner created, in creation order; the
	// numbered keys name the nth of them for a table cell.
	keySyntheticGateways    = "cloudSyntheticGatewayIDs"
	keyManagedGatewayPrefix = "cloudSyntheticManagedGateway"
	keyGatewayIdentity      = "cloudManagedGatewayIdentity"
	keyDefaultGatewayID     = "cloudDefaultManagedGatewayID"
)

// RegisterGatewaySteps binds the steps that list the managed gateways and request a gateway the
// platform refuses.
func (s *Steps) RegisterGatewaySteps(sc cloudbase.StepRegistrar) {
	sc.Step(`^I choose a synthetic managed APIP gateway name$`, s.chooseGatewayName)
	sc.Step(`^I request a synthetic managed APIP gateway with:$`, s.requestManagedGateway)
	sc.Step(`^I list the managed APIP gateways$`, s.listManagedGateways)
	sc.Step(`^the listed managed gateways should each report an identity, an environment and a status$`, s.assertListingIsValid)
	sc.Step(`^the listed managed gateways should include the active gateway$`, s.assertListingIncludesActiveGateway)
	sc.Step(`^the selected APIP environment should be listed by the platform$`, s.assertSelectedEnvironmentListed)
	s.registerManagedGatewayLifecycleSteps(sc)
}

// RegisterGatewayConfigSteps binds the steps that read the active gateway's configuration and
// write settings the platform refuses.
func (s *Steps) RegisterGatewayConfigSteps(sc cloudbase.StepRegistrar) {
	sc.Step(`^I read the configuration of the active APIP gateway$`, s.readGatewayConfiguration)
	sc.Step(`^the active APIP gateway's configuration should be readable$`, s.awaitConfigurationReadable)
	sc.Step(`^the configuration of the selected gateway should become readable$`, s.awaitConfigurationReadable)
	sc.Step(`^the gateway configuration should be a valid document$`, s.assertConfigurationDocument)
	sc.Step(`^I write the gateway configuration path "([^"]*)" with a string of (\d+) characters$`, s.writeLongString)
	sc.Step(`^I write each of these gateway configuration values separately:$`, s.writeEachValue)
	sc.Step(`^I work with the configuration of synthetic managed APIP gateway (\d+)$`, s.selectConfigurationGateway)
	sc.Step(`^I save the gateway configuration with:$`, s.saveGatewayConfiguration)
	sc.Step(`^the gateway configuration phase should be "([^"]*)"$`, s.assertConfigurationPhase)
	sc.Step(`^the gateway configuration should report:$`, s.assertConfigurationReports)
	sc.Step(`^the gateway configuration should no longer report "([^"]*)"$`, s.assertConfigurationOmits)
	sc.Step(`^I write the gateway configuration from (\d+) writers at once until one is rejected$`, s.writeConcurrently)
	sc.Step(`^every one of those writes should have been refused as invalid$`, s.assertEveryWriteRejected)
}

// gatewayDeleteTimeout bounds how long a gateway's deletion is retried while the platform refuses it
// because the gateway is still being built or changed.
const gatewayDeleteTimeout = 5 * time.Minute

// gatewayDeleteInterval is how often a refused gateway deletion is asked again.
const gatewayDeleteInterval = 10 * time.Second

// gatewayStillChanging reports whether err is the platform refusing to delete a gateway for now: it
// answers 409 with the code "conflict" while the gateway is being built, and that changes once it
// is. Any other refusal, such as a default gateway that others depend on, does not change by waiting.
func gatewayStillChanging(err error) bool {
	var answer *cloudbase.StatusError
	return errors.As(err, &answer) && answer.Status == http.StatusConflict && answer.Code == "conflict"
}

// GatewayDeleter deletes a managed gateway the run created and asks again while the platform says
// the gateway is still changing state, since a gateway a scenario asked for just before it ended may
// not be deletable yet.
func GatewayDeleter(topo *frameworkruntime.Topology, client *httpx.Client) cleanup.Deleter {
	return func(ctx context.Context, resource cleanup.Resource) error {
		base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
		if err != nil {
			return err
		}
		bearer, err := cloudbase.BearerToken(ctx)
		if err != nil {
			return err
		}
		return deleteGatewayWhenItCanBe(ctx, client, base, bearer, resource.ID, gatewayDeleteTimeout, gatewayDeleteInterval)
	}
}

// deleteGatewayWhenItCanBe deletes a managed gateway, asking again every interval, for up to
// timeout, while the platform refuses because the gateway is still changing state.
func deleteGatewayWhenItCanBe(ctx context.Context, client *httpx.Client, base, bearer, id string, timeout, interval time.Duration) error {
	return retry.Await(ctx, retry.Options{Timeout: timeout, Interval: interval},
		func(ctx context.Context) (bool, error) {
			err := cloudbase.DeleteCloudResource(ctx, client, base, bearer, cloudbase.ManagedGatewaysCollection, id)
			switch {
			case err == nil:
				return true, nil
			case gatewayStillChanging(err):
				return false, retry.Transient(err)
			default:
				return false, err
			}
		},
		func(deleted bool) bool { return deleted },
		"deleting managed gateway "+id)
}
