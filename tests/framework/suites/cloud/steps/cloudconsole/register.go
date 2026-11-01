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
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

const (
	// ComponentName is the catalog component used by APIP cloud runners.
	ComponentName = corecatalog.Name

	keyToken            = "cloudConsoleToken"
	keyProjectID        = "cloudProjectID"
	keyGatewayID        = "cloudGatewayID"
	keyGatewayEndpoint  = "cloudGatewayEndpoint"
	keyGatewayEnv       = "cloudGatewayEnvironment"
	keyAPIID            = "cloudAPIID"
	keyDeploymentID     = "cloudDeploymentID"
	keyEnvironmentName  = "cloudEnvironmentName"
	keyManagedGatewayID = "cloudManagedGatewayID"

	keyAPIDefinition        = "cloudAPIDefinition"
	keyAPIKey               = "cloudAPIKey"
	keyRejectedAPIID        = "cloudRejectedAPIID"
	keyVersionAPIID         = "cloudVersionAPIID"
	keyVersionDeploymentID  = "cloudVersionDeploymentID"
	keyVersionAPIDefinition = "cloudVersionAPIDefinition"

	keyBuildID         = "cloudBuildID"
	keyBuildIDs        = "cloudBuildIDs"
	keyBuildsOwner     = "cloudBuildsOwner"
	keyBuildMetadata   = "cloudBuildMetadata"
	keyDeployedBuildID = "cloudDeployedBuildID"
	keyEchoBackendURL  = "cloudEchoBackendURL"

	keyManagedGatewayEnv   = "cloudManagedGatewayEnvironment"
	keyScenarioGatewayMark = "cloudScenarioGatewayMarker"
	keyGatewayDeleteStatus = "cloudGatewayDeleteStatus"
	keyManagedGatewayName  = "cloudManagedGatewayName"
	keyConfigGatewayID     = "cloudConfigGatewayID"
	keyConfigAttempts      = "cloudConfigAttempts"

	// keySyntheticGateways lists the managed gateways the runner created, in creation order; the
	// numbered keys name the nth of them for a table cell.
	keySyntheticGateways    = "cloudSyntheticGatewayIDs"
	keyManagedGatewayPrefix = "cloudSyntheticManagedGateway"
	keyGatewayIdentity      = "cloudManagedGatewayIdentity"
	keyDefaultGatewayID     = "cloudDefaultManagedGatewayID"
	keyDeployedTargets      = "cloudDeployedTargets"

	cloudConsoleAccessTokenEnv = "CLOUD_CONSOLE_ACCESS_TOKEN"

	cloudConsoleExpiredTokenEnv = "CLOUD_CONSOLE_EXPIRED_ACCESS_TOKEN"

	cloudGatewayPinEnv = "CLOUD_GATEWAY_ID"

	resourceActor = "apip-console"
)

// environmentsCollection is the collection an environment is deleted from.
const environmentsCollection = "/environments"

// managedGatewaysCollection is the collection a managed gateway is listed in and deleted from.
const managedGatewaysCollection = "/managed-gateways"

var configureCloudExpansion sync.Once

func expandCloudValue(ctx context.Context, value string) (string, error) {
	configureCloudExpansion.Do(func() {
		unique.ContextValue = func(ctx context.Context, name string) (string, error) {
			return tcontext.ResolveString(ctx, name)
		}
	})
	return unique.Expand(ctx, value)
}

// Steps contains APIP cloud step bindings for one resolved block.
type Steps struct {
	topo                 *frameworkruntime.Topology
	funnel               *httpx.Funnel
	client               *httpx.Client
	dataPlaneURLResolver func(context.Context, string) (string, error)
}

// stepDefinition pairs a Gherkin step pattern with its handler.
type stepDefinition struct {
	pattern string
	handler any
}

// stepRegistrar is the part of godog.ScenarioContext the step registration needs, so a test can
// record the registered patterns instead of running a scenario.
type stepRegistrar interface {
	Step(expr, stepFunc any)
}

// stepRecorder collects the steps registered through it.
type stepRecorder struct {
	definitions []stepDefinition
}

func (r *stepRecorder) Step(expr, stepFunc any) {
	r.definitions = append(r.definitions, stepDefinition{pattern: expr.(string), handler: stepFunc})
}

// Register binds APIP cloud-specific Gherkin steps.
func Register(sc *godog.ScenarioContext, topo *frameworkruntime.Topology, funnel *httpx.Funnel) {
	s := &Steps{topo: topo, funnel: funnel, client: funnel.Client()}
	s.registerSteps(sc)
}

// stepDefinitions lists every step pattern Register binds.
func (s *Steps) stepDefinitions() []stepDefinition {
	recorder := &stepRecorder{}
	s.registerSteps(recorder)
	return recorder.definitions
}

func (s *Steps) registerSteps(sc stepRegistrar) {
	sc.Step(`^I obtain an APIP cloud console token$`, s.obtainToken)
	sc.Step(`^I find the default APIP project$`, s.findDefaultProject)
	sc.Step(`^I find an active APIP gateway$`, s.findActiveGateway)
	sc.Step(`^I create a synthetic REST API in the default project$`, s.createDefaultAPI)
	sc.Step(`^I create a synthetic REST API in the default project with:$`, s.createAPIWithValues)
	sc.Step(`^I create a new version "([^"]*)" of the synthetic REST API with:$`, s.createNewAPIVersion)
	sc.Step(`^I submit a synthetic REST API creation request with:$`, s.submitAPICreation)
	sc.Step(`^the cloud API response status code should be (\d+)$`, s.assertCloudResponseStatus)
	sc.Step(`^I update the synthetic REST API with:$`, s.updateAPI)
	sc.Step(`^I deploy the (synthetic REST API|new API version) to the active gateway$`, s.deployAPI)
	sc.Step(`^I redeploy the synthetic REST API to the active gateway$`, s.redeployAPI)
	sc.Step(`^I undeploy the synthetic REST API$`, s.undeployAPI)
	sc.Step(`^the (synthetic REST API|new API version) deployment should become (DEPLOYED|UNDEPLOYED|FAILED)$`, s.waitDeployment)
	sc.Step(`^a "([^"]*)" request to "([^"]*)" should keep returning (\d+) until the redeployment is DEPLOYED$`, s.keepServingUntilDeployed)
	sc.Step(`^I create an API key for the synthetic REST API$`, s.createAPIKey)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" with header "([^"]*)" set to the API key until status (\d+)$`, s.sendWithAPIKeyUntilStatus)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" until status (\d+)$`, s.sendUntilStatus)
	sc.Step(`^the synthetic REST API data-plane response should contain post (\d+)$`, s.assertAPIResponse)
	sc.Step(`^I delete the synthetic REST API$`, s.deleteAPI)
	sc.Step(`^the synthetic REST API should eventually be absent$`, s.verifyAPIDeleted)

	s.registerIdentitySteps(sc)
	s.registerDeploymentSteps(sc)
	s.registerGatewaySteps(sc)
	s.registerGatewayConfigSteps(sc)
	s.registerPromotionSteps(sc)
	s.registerSecretSteps(sc)
	s.registerProjectSteps(sc)
	s.registerPortalSteps(sc)
	s.registerPortalSignInSteps(sc)
	sc.Step(`^the cloud API response should report:$`, s.assertResponseReports)

	sc.Step(`^I create a synthetic APIP project$`, s.createProject)
	sc.Step(`^the synthetic APIP project should be retrievable by handle$`, s.getProject)

	sc.Step(`^I create a synthetic non-production APIP environment$`, s.createEnvironment)
	sc.Step(`^I create a synthetic non-production APIP environment that lasts for the runner$`, s.createRunnerEnvironment)
	sc.Step(`^creating the same APIP environment again should return conflict$`, s.duplicateEnvironment)
	sc.Step(`^I delete the synthetic APIP environment$`, s.deleteEnvironment)
	sc.Step(`^deleting the synthetic APIP environment should eventually return not found$`, s.verifyEnvironmentDeleted)

	sc.Step(`^I select an existing APIP environment for a managed gateway$`, s.selectGatewayEnvironment)
}

// registerIdentitySteps binds the steps that inspect the console token and send requests with
// controlled credentials.
func (s *Steps) registerIdentitySteps(sc stepRegistrar) {
	sc.Step(`^the APIP cloud console token should be a JWT that has not expired$`, s.assertTokenIsLiveJWT)
	sc.Step(`^the APIP cloud console token should carry an organisation claim$`, s.assertTokenCarriesOrganisation)
	sc.Step(`^I send an authenticated "([^"]*)" request to the APIP cloud path "([^"]*)"$`, s.sendAuthenticatedRequest)
	sc.Step(`^a "([^"]*)" request to the APIP cloud path "([^"]*)" should give the same result when the "([^"]*)" header names another organisation$`,
		s.assertForeignOrganisationHeaderIgnored)
	sc.Step(`^I send a "([^"]*)" request to the APIP cloud path "([^"]*)" without an Authorization header$`, s.sendWithoutAuthorization)
	sc.Step(`^I send a "([^"]*)" request to the APIP cloud path "([^"]*)" with the Authorization header "([^"]*)"$`, s.sendWithAuthorizationHeader)
	sc.Step(`^I send a "([^"]*)" request to the APIP cloud path "([^"]*)" bearing a token (carrying the real claims and key id with a forged signature|with no signature|signed with an unknown key|signed with an unknown shared secret)$`,
		s.sendWithForgedToken)
	sc.Step(`^I send a "([^"]*)" request to the APIP cloud path "([^"]*)" bearing an expired console token$`, s.sendWithExpiredToken)
	sc.Step(`^the cloud API error response should not expose internal detail$`, s.assertNoInternalDetail)
	sc.Step(`^the cloud API response status code should be a client error$`, s.assertCloudResponseClientError)
	sc.Step(`^the cloud API response body should contain "([^"]*)"$`, s.assertCloudResponseContains)
	sc.Step(`^the cloud API error code should be "([^"]*)"$`, s.assertCloudErrorCode)
}

// registerDeploymentSteps binds the build and deployment steps.
func (s *Steps) registerDeploymentSteps(sc stepRegistrar) {
	sc.Step(`^I prepare a build of the synthetic REST API with metadata:$`, s.prepareBuild)
	sc.Step(`^the prepared build should have a day-and-index identifier$`, s.assertBuildIDShape)
	sc.Step(`^the synthetic REST API builds should include the prepared build$`, s.assertBuildListed)
	sc.Step(`^the synthetic REST API builds should list the rendered builds newest first$`, s.assertBuildsNewestFirst)
	sc.Step(`^the deployment should have rendered a build of the API$`, s.recordRenderedBuild)
	sc.Step(`^I request a deployment of the synthetic REST API with:$`, s.requestDeployment)
	sc.Step(`^the deployment response should list the active gateway$`, s.assertDeploymentListsActiveGateway)
	sc.Step(`^the deployed build should be the rendered build$`, s.assertDeployedBuildIsPrepared)
	sc.Step(`^the synthetic REST API deployments should have one stage per environment of its project's deployment pipeline$`, s.assertStagesMatchPipeline)
	sc.Step(`^the deployment stage of the active gateway's environment should show the active gateway as ([A-Z_]+)$`, s.assertActiveGatewayStageStatus)
	sc.Step(`^the synthetic REST API deployment should report a status reason code$`, s.assertDeploymentStatusReason)
	sc.Step(`^an echo backend is available for deployment overrides$`, s.useEchoBackend)
	sc.Step(`^the deployment response should report the echo backend as the production endpoint$`, s.assertDeploymentReportsEchoEndpoint)
	sc.Step(`^the echo backend should have received a request for "([^"]*)"$`, s.assertEchoBackendReceived)
}

// registerPromotionSteps binds the steps that build a pipeline over synthetic environments, bind a
// project to it and check what a promotion along it does to each environment.
func (s *Steps) registerPromotionSteps(sc stepRegistrar) {
	sc.Step(`^I create (\d+) synthetic non-production APIP environments?$`, s.createSyntheticEnvironments)
	sc.Step(`^I have (\d+) synthetic non-production APIP environments? for the runner with an active gateway each$`, s.ensureRunnerEnvironments)
	sc.Step(`^I create a synthetic deployment pipeline with the promotion paths:$`, s.createPipeline)
	sc.Step(`^I replace the promotion paths of the synthetic deployment pipeline with:$`, s.replacePipelinePaths)
	sc.Step(`^I create a synthetic APIP project bound to the synthetic deployment pipeline$`, s.createProjectBoundToPipeline)
	sc.Step(`^I create a synthetic REST API in the synthetic APIP project$`,
		func(ctx context.Context) error { return s.createAPIInSyntheticProject(ctx, nil) })
	sc.Step(`^the deployment stage of synthetic APIP environment (\d+) should show no gateway as deployed$`, s.assertStageHoldsNothing)
	sc.Step(`^the deployment response should list the gateway of synthetic APIP environment (\d+)$`, s.assertDeploymentListsGatewayOf)
	sc.Step(`^the deployment stage of synthetic APIP environment (\d+) should show its gateway running the rendered build$`, s.assertStageRunsRenderedBuild)
	sc.Step(`^the deployment stage of synthetic APIP environment (\d+) should show its gateway as no longer deployed$`, s.assertStageNoLongerDeployed)
	sc.Step(`^I undeploy the promoted deployment from synthetic APIP environment (\d+)$`, s.undeployFromSyntheticEnvironment)
	sc.Step(`^I read the endpoint of the gateway of synthetic APIP environment (\d+)$`, s.recordSyntheticEnvironmentEndpoint)
	sc.Step(`^the synthetic REST API should be DEPLOYED on every gateway the deployment request named$`, s.waitDeployedOnTargets)
	sc.Step(`^the synthetic REST API should hold (\d+) builds$`, s.assertBuildCount)
}

// registerGatewaySteps binds the steps that list the managed gateways and request a gateway the
// platform refuses.
func (s *Steps) registerGatewaySteps(sc stepRegistrar) {
	sc.Step(`^I choose a synthetic managed APIP gateway name$`, s.chooseGatewayName)
	sc.Step(`^I request a synthetic managed APIP gateway with:$`, s.requestManagedGateway)
	sc.Step(`^I list the managed APIP gateways$`, s.listManagedGateways)
	sc.Step(`^the listed managed gateways should each report an identity, an environment and a status$`, s.assertListingIsValid)
	sc.Step(`^the listed managed gateways should include the active gateway$`, s.assertListingIncludesActiveGateway)
	sc.Step(`^the selected APIP environment should be listed by the platform$`, s.assertSelectedEnvironmentListed)
	s.registerManagedGatewayLifecycleSteps(sc)
}

// registerGatewayConfigSteps binds the steps that read the active gateway's configuration and
// write settings the platform refuses.
func (s *Steps) registerGatewayConfigSteps(sc stepRegistrar) {
	sc.Step(`^I read the configuration of the active APIP gateway$`, s.readGatewayConfiguration)
	sc.Step(`^the active APIP gateway's configuration should be readable$`, s.awaitConfigurationReadable)
	sc.Step(`^the configuration of the selected gateway should become readable$`, s.awaitConfigurationReadable)
	sc.Step(`^the gateway configuration should be a valid document$`, s.assertConfigurationDocument)
	sc.Step(`^I write the gateway configuration with:$`, s.writeGatewayConfiguration)
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

// registerSecretSteps binds the steps that create, rotate, read and delete a secret and read back
// what a gateway sends upstream with it.
func (s *Steps) registerSecretSteps(sc stepRegistrar) {
	sc.Step(`^I create a synthetic secret$`, s.createSecret)
	sc.Step(`^I choose a synthetic secret handle without creating the secret$`, s.chooseSecretHandle)
	sc.Step(`^I rotate the synthetic secret$`, s.rotateSecret)
	sc.Step(`^I read the synthetic secret$`, s.readSecret)
	sc.Step(`^I list the secrets$`, s.listSecrets)
	sc.Step(`^I delete the synthetic secret$`, s.deleteSecret)
	sc.Step(`^the response should not reveal the synthetic secret's value$`, s.assertSecretValueHidden)
	sc.Step(`^the synthetic secret should be described by a hash$`, s.assertSecretHash)
	sc.Step(`^the echoed request header "([^"]*)" should carry the synthetic secret's value$`, s.assertEchoedSecret)
	sc.Step(`^a "([^"]*)" request to "([^"]*)" should echo the header "([^"]*)" with the rotated secret value$`, s.sendUntilEchoesRotated)
}

// RegisterDeleters installs cloud-target cleanup handlers for a block containing cloud-console.
func RegisterDeleters(reg *cleanup.Registry, topo *frameworkruntime.Topology) error {
	if reg == nil {
		return fmt.Errorf("cloud-console: cleanup registry is required")
	}
	if topo == nil {
		return fmt.Errorf("cloud-console: topology is required")
	}
	if _, err := topo.Component(corecatalog.Name); err != nil {
		return nil
	}
	client := httpx.NewClient(httpx.Options{MaxRetries: 1})
	if err := reg.RegisterDeleter(cleanup.KindAPI, cloudDeleter(topo, client, "/rest-apis", false)); err != nil {
		return err
	}
	if err := reg.RegisterDeleter(cleanup.KindProject, projectDeleter(topo, client)); err != nil {
		return err
	}
	if err := reg.RegisterDeleter(cleanup.KindEnvironment, cloudDeleter(topo, client, "/environments", true)); err != nil {
		return err
	}
	if err := reg.RegisterDeleter(kindPipeline, cloudDeleter(topo, client, "/pipelines", true)); err != nil {
		return err
	}
	if err := reg.RegisterDeleter(kindSecret, cloudDeleter(topo, client, secretsCollection, false)); err != nil {
		return err
	}
	if err := reg.RegisterDeleter(kindDefaultRestore, restoreDefaultDeleter(topo, client)); err != nil {
		return err
	}
	if err := reg.RegisterDeleter(kindAPIPortal, portalDeleter(topo, client)); err != nil {
		return err
	}
	return reg.RegisterDeleter(cleanup.KindGateway, cloudDeleter(topo, client, managedGatewaysCollection, false))
}

// emptyEnvironment deletes the managed gateways of an environment, which the platform refuses
// to delete the environment while it holds, including a gateway the platform created with it.
func emptyEnvironment(ctx context.Context, client *httpx.Client, base, bearer, environment string) error {
	headers := map[string]string{"Authorization": "Bearer " + bearer}
	listed, err := client.Do(ctx, httpx.Request{Method: http.MethodGet, URL: cloudAPIURL(base, "/managed-gateways"), Headers: headers}, 0, 0)
	if err != nil {
		return err
	}
	if listed.StatusCode != http.StatusOK {
		return responseError(listed)
	}
	items, err := listItems(listed)
	if err != nil {
		return fmt.Errorf("decoding managed gateways: %w", err)
	}
	for _, item := range items {
		id, _ := item["id"].(string)
		if item["environment"] != environment || id == "" {
			continue
		}
		resp, err := client.Do(ctx, httpx.Request{Method: http.MethodDelete,
			URL: cloudResourceURL(base, "/managed-gateways", id), Headers: headers}, 0, 0)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusNotFound && !resp.Succeeded() {
			return responseError(resp)
		}
	}
	return nil
}

func cloudDeleter(topo *frameworkruntime.Topology, client *httpx.Client, collection string, waitForNotFound bool) cleanup.Deleter {
	return func(ctx context.Context, resource cleanup.Resource) error {
		if waitForNotFound {
			// A deletion that waits for the platform to finish has a budget of its own, so a slow
			// earlier deletion in the sweep cannot use up the time of the next.
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(context.WithoutCancel(ctx), environmentDeletionTimeout+30*time.Second)
			defer cancel()
		}
		base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
		if err != nil {
			return err
		}
		bearer, err := bearerToken(ctx)
		if err != nil {
			return err
		}
		return deleteCloudResource(ctx, client, base, bearer, collection, resource.ID, waitForNotFound)
	}
}

// deleteCloudResource deletes one resource from a collection and treats an absent one as
// deleted. With waitForNotFound it keeps deleting until the platform answers 404.
func deleteCloudResource(ctx context.Context, client *httpx.Client, base, bearer, collection, id string, waitForNotFound bool) error {
	if collection == environmentsCollection {
		if err := emptyEnvironment(ctx, client, base, bearer, id); err != nil {
			return err
		}
	}
	deleteOne := func(ctx context.Context) (*httpx.Response, error) {
		return client.Do(ctx, httpx.Request{
			Method: http.MethodDelete,
			URL:    cloudResourceURL(base, collection, id),
			Headers: map[string]string{
				"Authorization": "Bearer " + bearer,
			},
		}, 0, 0)
	}
	if !waitForNotFound {
		resp, err := deleteOne(ctx)
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusNotFound || resp.Succeeded() {
			return nil
		}
		return responseError(resp)
	}
	return retry.Await(ctx, retry.Options{Timeout: environmentDeletionTimeout, Interval: 3 * time.Second},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, err := deleteOne(ctx)
			if err != nil {
				return nil, retry.Transient(err)
			}
			if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusConflict {
				return nil, responseError(resp)
			}
			return resp, nil
		},
		func(resp *httpx.Response) bool { return resp != nil && resp.StatusCode == http.StatusNotFound },
		"waiting for environment "+id+" to be deleted")
}

func bearerToken(ctx context.Context) (string, error) {
	if token, err := tcontext.ResolveString(ctx, keyToken); err == nil && strings.TrimSpace(token) != "" {
		return token, nil
	}
	token := strings.TrimSpace(os.Getenv(cloudConsoleAccessTokenEnv))
	if token == "" {
		return "", fmt.Errorf("environment variable %s is required", cloudConsoleAccessTokenEnv)
	}
	if err := tcontext.Set(ctx, keyToken, token); err != nil {
		return "", err
	}
	return token, nil
}

func responseError(resp *httpx.Response) error {
	if resp == nil {
		return errors.New("cloud-console: no response")
	}
	return fmt.Errorf("cloud-console: unexpected response: %s", resp.Describe())
}

func cloudAPIURL(base, path string) string {
	return strings.TrimRight(base, "/") + path
}

func cloudResourceURL(base, collection, id string) string {
	return cloudAPIURL(base, collection+"/"+url.PathEscape(id))
}

func propagationTimeout(topo *frameworkruntime.Topology) time.Duration {
	if topo != nil && topo.PropagationTimeout > 0 {
		return topo.PropagationTimeout
	}
	return retry.PropagationCeiling
}
