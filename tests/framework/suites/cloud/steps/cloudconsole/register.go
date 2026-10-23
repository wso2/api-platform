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

	keyBuildID           = "cloudBuildID"
	keyBuildIDs          = "cloudBuildIDs"
	keyBuildsOwner       = "cloudBuildsOwner"
	keyBuildMetadata     = "cloudBuildMetadata"
	keyDeployedBuildID   = "cloudDeployedBuildID"
	keyDeployedTargets   = "cloudDeployedTargets"
	keySyntheticGateways = "cloudSyntheticGatewayIDs"
	keyEchoBackendURL    = "cloudEchoBackendURL"

	keyManagedGatewayName  = "cloudManagedGatewayName"
	keyManagedGatewayEnv   = "cloudManagedGatewayEnvironment"
	keyGatewayIdentity     = "cloudManagedGatewayIdentity"
	keyDefaultGatewayID    = "cloudDefaultManagedGatewayID"
	keyConfigGatewayID     = "cloudConfigGatewayID"
	keyConfigAttempts      = "cloudConfigAttempts"
	keyGatewayDeleteStatus = "cloudGatewayDeleteStatus"

	keyManagedGatewayEndpoint = "cloudManagedGatewayEndpoint"

	cloudConsoleAccessTokenEnv = "CLOUD_CONSOLE_ACCESS_TOKEN"

	resourceActor = "apip-console"
)

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

// Register binds APIP cloud-specific Gherkin steps.
func Register(sc *godog.ScenarioContext, topo *frameworkruntime.Topology, funnel *httpx.Funnel) {
	s := &Steps{topo: topo, funnel: funnel, client: funnel.Client()}
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
	s.registerResilienceSteps(sc)
	s.registerSecretSteps(sc)
	s.registerAIWorkspaceSteps(sc)

	sc.Step(`^I create a synthetic APIP project$`, s.createProject)
	sc.Step(`^the synthetic APIP project should be retrievable by handle$`, s.getProject)
	sc.Step(`^I delete the synthetic APIP project$`, s.deleteProject)
	sc.Step(`^the synthetic APIP project should eventually be absent$`, s.verifyProjectDeleted)

	sc.Step(`^I create a synthetic non-production APIP environment$`, s.createEnvironment)
	sc.Step(`^creating the same APIP environment again should return conflict$`, s.duplicateEnvironment)
	sc.Step(`^I delete the synthetic APIP environment$`, s.deleteEnvironment)
	sc.Step(`^deleting the synthetic APIP environment should eventually return not found$`, s.verifyEnvironmentDeleted)

	sc.Step(`^I select an existing APIP environment for a managed gateway$`, s.selectGatewayEnvironment)
	sc.Step(`^I create a synthetic managed APIP gateway$`, s.createManagedGateway)
	sc.Step(`^the managed APIP gateway should be retrievable through the managed-gateway endpoint$`, s.getManagedGateway)
	sc.Step(`^I delete the synthetic managed APIP gateway$`, s.deleteManagedGateway)
	sc.Step(`^the managed APIP gateway should eventually be absent$`, s.verifyManagedGatewayDeleted)
}

// registerIdentitySteps binds the steps that inspect the console token and send requests with
// controlled credentials.
func (s *Steps) registerIdentitySteps(sc *godog.ScenarioContext) {
	sc.Step(`^the APIP cloud console token should be a JWT that has not expired$`, s.assertTokenIsLiveJWT)
	sc.Step(`^the APIP cloud console token should carry an organisation claim$`, s.assertTokenCarriesOrganisation)
	sc.Step(`^I send an authenticated "([^"]*)" request to the APIP cloud path "([^"]*)"$`, s.sendAuthenticatedRequest)
	sc.Step(`^a "([^"]*)" request to the APIP cloud path "([^"]*)" should give the same result when the "([^"]*)" header names another organisation$`,
		s.assertForeignOrganisationHeaderIgnored)
	sc.Step(`^I send a "([^"]*)" request to the APIP cloud path "([^"]*)" without an Authorization header$`, s.sendWithoutAuthorization)
	sc.Step(`^I send a "([^"]*)" request to the APIP cloud path "([^"]*)" with the Authorization header "([^"]*)"$`, s.sendWithAuthorizationHeader)
	sc.Step(`^I send a "([^"]*)" request to the APIP cloud path "([^"]*)" bearing a token (that expired an hour ago|with no signature|signed with an unknown key|signed with an unknown shared secret)$`,
		s.sendWithForgedToken)
	sc.Step(`^the cloud API error response should not expose internal detail$`, s.assertNoInternalDetail)
	sc.Step(`^the cloud API response status code should be a client error$`, s.assertCloudResponseClientError)
	sc.Step(`^the cloud API response body should contain "([^"]*)"$`, s.assertCloudResponseContains)
	sc.Step(`^the cloud API error code should be "([^"]*)"$`, s.assertCloudErrorCode)
}

// registerDeploymentSteps binds the build, deployment and synthetic-gateway steps.
func (s *Steps) registerDeploymentSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I choose a synthetic REST API handle without creating the API$`, s.chooseUnusedAPIHandle)
	sc.Step(`^I prepare a build of the synthetic REST API$`, func(ctx context.Context) error { return s.prepareBuild(ctx, nil) })
	sc.Step(`^I prepare a build of the synthetic REST API with metadata:$`, s.prepareBuild)
	sc.Step(`^the prepared build should have a day-and-index identifier$`, s.assertBuildIDShape)
	sc.Step(`^the synthetic REST API builds should include the prepared build$`, s.assertBuildListed)
	sc.Step(`^the synthetic REST API builds should list the prepared builds newest first$`, s.assertBuildsNewestFirst)
	sc.Step(`^the synthetic REST API builds should list the rendered builds newest first$`, s.assertBuildsNewestFirst)
	sc.Step(`^the deployment should have rendered a build of the API$`, s.recordRenderedBuild)
	sc.Step(`^the synthetic REST API should hold (\d+) builds$`, s.assertBuildCount)
	sc.Step(`^I request a deployment of the synthetic REST API with:$`, s.requestDeployment)
	sc.Step(`^the deployment response should list the active gateway$`, s.assertDeploymentListsActiveGateway)
	sc.Step(`^the deployment response should list (\d+) gateways$`, s.assertDeploymentCount)
	sc.Step(`^the deployed build should be the prepared build$`, s.assertDeployedBuildIsPrepared)
	sc.Step(`^the deployed build should be the rendered build$`, s.assertDeployedBuildIsPrepared)
	sc.Step(`^the synthetic REST API should be DEPLOYED on every gateway the deployment request named$`, s.waitDeployedOnTargets)
	sc.Step(`^the synthetic REST API deployments should have one stage per environment of its project's deployment pipeline$`, s.assertStagesMatchPipeline)
	sc.Step(`^the deployment stage of the active gateway's environment should show the active gateway as ([A-Z_]+)$`, s.assertActiveGatewayStageStatus)
	sc.Step(`^the synthetic REST API deployment should report a status reason code$`, s.assertDeploymentStatusReason)
	sc.Step(`^an echo backend is available for deployment overrides$`, s.useEchoBackend)
	sc.Step(`^the deployment response should report the echo backend as the production endpoint$`, s.assertDeploymentReportsEchoEndpoint)
	sc.Step(`^the echo backend should have received a request for "([^"]*)"$`, s.assertEchoBackendReceived)
	sc.Step(`^I create (\d+) synthetic managed APIP gateways? in the active gateway's environment$`, s.createGatewaysInActiveEnvironment)
	sc.Step(`^the synthetic managed APIP gateways should become active$`, s.waitSyntheticGatewaysActive)
	sc.Step(`^I deploy a build of the synthetic REST API to the active gateway and every synthetic gateway$`, s.deployBuildPerGateway)
}

// registerGatewaySteps binds the steps that list, create, update and read managed gateways.
func (s *Steps) registerGatewaySteps(sc *godog.ScenarioContext) {
	sc.Step(`^I create a synthetic managed APIP gateway in the active gateway's environment$`,
		func(ctx context.Context) error { return s.createGatewaysInActiveEnvironment(ctx, 1) })
	sc.Step(`^I create (\d+) synthetic managed APIP gateways? in the synthetic APIP environment$`, s.createGatewaysInSyntheticEnvironment)
	sc.Step(`^I choose a synthetic managed APIP gateway name$`, s.chooseGatewayName)
	sc.Step(`^I request a synthetic managed APIP gateway with:$`, s.requestManagedGateway)
	sc.Step(`^I create a synthetic managed APIP gateway whose handle is (\d+) characters? short of the environment's limit$`,
		s.createGatewayBelowHandleLimit)
	sc.Step(`^I deploy a build of the synthetic REST API to synthetic gateway (\d+)$`, s.deployBuildToSyntheticGateway)
	sc.Step(`^the synthetic REST API should be deployed on synthetic gateway (\d+)$`, func(ctx context.Context, n int) error {
		return s.assertDeployedOnSyntheticGateway(ctx, n, true)
	})
	sc.Step(`^the synthetic REST API should not be deployed on synthetic gateway (\d+)$`, func(ctx context.Context, n int) error {
		return s.assertDeployedOnSyntheticGateway(ctx, n, false)
	})
	sc.Step(`^I delete synthetic gateway (\d+)$`, s.deleteSyntheticGateway)
	sc.Step(`^synthetic gateway (\d+) should still be retrievable$`, s.assertSyntheticGatewayRetrievable)
	sc.Step(`^I list the managed APIP gateways$`, s.listManagedGateways)
	sc.Step(`^I list the managed APIP gateways of type "([^"]*)"$`, s.listManagedGatewaysOfType)
	sc.Step(`^the listed managed gateways should each report an identity, an environment and a status$`, s.assertListingIsValid)
	sc.Step(`^the listed managed gateways should include the active gateway$`, s.assertListingIncludesActiveGateway)
	sc.Step(`^every listed managed gateway should be of type "([^"]*)"$`, s.assertListingOnlyOfType)
	sc.Step(`^the managed gateway types should partition the unfiltered listing$`, s.assertTypesPartitionTheListing)
	sc.Step(`^the selected APIP environment should be listed by the platform$`, s.assertSelectedEnvironmentListed)
	sc.Step(`^every managed gateway the scenario created should be retrievable$`, s.assertEveryCreatedGatewayExists)
	sc.Step(`^the environment should hold exactly one managed gateway with the chosen name$`, s.assertGatewayListedOnce)
	sc.Step(`^the managed APIP gateway should be registered under the id its name gives it$`, s.assertGatewayHasItsNameAsID)
	sc.Step(`^I update the synthetic managed APIP gateway with:$`, s.updateManagedGateway)
	sc.Step(`^the managed APIP gateway should report:$`, s.assertManagedGatewayReports)
	sc.Step(`^the update should have left the managed APIP gateway's identity unchanged$`, s.assertIdentityUnchanged)
	sc.Step(`^the managed APIP gateway should be bound to the environment it was created in$`, s.assertBoundToEnvironment)
	sc.Step(`^I delete the default gateway of the synthetic APIP environment$`, s.deleteDefaultGatewayOfSyntheticEnvironment)
	sc.Step(`^the refused default gateway should still be its environment's default$`, s.assertRefusedDefaultStillExists)
	sc.Step(`^the managed APIP gateway should report a recorded provisioning state on its first read$`, s.assertFirstReadHasStatus)
	sc.Step(`^the managed APIP gateway status should progress from provisioning to built without failing$`, s.assertStatusProgressesToBuilt)
	sc.Step(`^the managed APIP gateway should not report its controller as connected yet$`, s.assertGatewayNotConnected)
	sc.Step(`^the managed APIP gateway should report its controller as connected$`, s.assertGatewayConnected)
	sc.Step(`^the managed APIP gateway should report (no|a) version$`, s.assertGatewayVersion)
	sc.Step(`^the managed APIP gateway endpoint should answer requests$`, s.assertGatewayEndpointAnswers)
}

// registerGatewayConfigSteps binds the steps that read and write a managed gateway's configuration.
func (s *Steps) registerGatewayConfigSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I read the configuration of the (active|synthetic managed) APIP gateway$`, s.readGatewayConfiguration)
	sc.Step(`^the synthetic managed APIP gateway's configuration should become readable$`, s.awaitConfigurationReadable)
	sc.Step(`^the gateway configuration should be a valid document$`, s.assertConfigurationDocument)
	sc.Step(`^I write the gateway configuration with:$`, s.writeGatewayConfiguration)
	sc.Step(`^I save the gateway configuration with:$`, s.saveGatewayConfiguration)
	sc.Step(`^I write the gateway configuration path "([^"]*)" with a string of (\d+) characters$`, s.writeLongString)
	sc.Step(`^I write each of these gateway configuration values separately:$`, s.writeEachValue)
	sc.Step(`^every one of those writes should have been refused as invalid$`, s.assertEveryWriteRejected)
	sc.Step(`^the gateway configuration phase should be "([^"]*)"$`, s.assertConfigurationPhase)
	sc.Step(`^the gateway configuration should report:$`, s.assertConfigurationReports)
	sc.Step(`^the gateway configuration should no longer report "([^"]*)"$`, s.assertConfigurationOmits)
	sc.Step(`^I write the gateway configuration from (\d+) writers at once until one is rejected$`, s.writeConcurrently)
}

// registerPromotionSteps binds the steps that build a pipeline over synthetic environments, bind a
// project to it and check what a promotion along it does to each environment.
func (s *Steps) registerPromotionSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I find the environment of the default APIP gateway$`, s.findDefaultGatewayEnvironment)
	sc.Step(`^I create (\d+) synthetic non-production APIP environments?$`, s.createSyntheticEnvironments)
	sc.Step(`^I create a synthetic managed APIP gateway in each synthetic APIP environment$`, s.createGatewayInEachSyntheticEnvironment)
	sc.Step(`^I create a synthetic deployment pipeline with the promotion paths:$`, s.createPipeline)
	sc.Step(`^I replace the promotion paths of the synthetic deployment pipeline with:$`, s.replacePipelinePaths)
	sc.Step(`^I create a synthetic APIP project bound to the synthetic deployment pipeline$`, s.createProjectBoundToPipeline)
	sc.Step(`^I create a synthetic REST API in the synthetic APIP project$`,
		func(ctx context.Context) error { return s.createAPIInSyntheticProject(ctx, nil) })
	sc.Step(`^I create a synthetic REST API in the synthetic APIP project with:$`, s.createAPIInSyntheticProject)
	sc.Step(`^I request a deployment of an LLM provider that does not exist with:$`, s.requestProviderDeployment)
	sc.Step(`^the deployment response should list the gateway of synthetic APIP environment (\d+)$`, s.assertDeploymentListsGatewayOf)
	sc.Step(`^the deployment stage of synthetic APIP environment (\d+) should show its gateway running the rendered build$`, s.assertStageRunsRenderedBuild)
	sc.Step(`^the deployment stage of synthetic APIP environment (\d+) should show no gateway as deployed$`, s.assertStageHoldsNothing)
	sc.Step(`^the deployment stage of synthetic APIP environment (\d+) should show its gateway as no longer deployed$`, s.assertStageNoLongerDeployed)
	sc.Step(`^I undeploy the promoted deployment from synthetic APIP environment (\d+)$`, s.undeployFromSyntheticEnvironment)
	sc.Step(`^I read the endpoint of the gateway of synthetic APIP environment (\d+)$`, s.recordSyntheticEnvironmentEndpoint)
}

// registerResilienceSteps binds the steps that send oversized requests, reach a backend that is
// not there, and read how many replicas of a scaled gateway serve traffic.
func (s *Steps) registerResilienceSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" with a body of (\d+) (KiB|MiB) until status (\d+)$`, s.sendSizedUntilStatus)
	sc.Step(`^a (chunked )?"([^"]*)" request to "([^"]*)" with a body of (\d+) (KiB|MiB) should be refused as too large$`, s.assertOversizedRefused)
	sc.Step(`^an unreachable backend is available for deployment overrides$`, s.useUnreachableBackend)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" until the gateway routes it$`, s.sendUntilRouted)
	sc.Step(`^the gateway response should be the router's documented failure for an unreachable backend$`, s.assertRouterFailure)
	sc.Step(`^I read the endpoint of the synthetic managed APIP gateway$`, s.recordManagedGatewayEndpoint)
	sc.Step(`^the synthetic managed APIP gateway's configuration phase should become "([^"]*)"$`, s.awaitConfigurationPhase)
	sc.Step(`^requests to "([^"]*)" should be served by at least (\d+) different gateway runtime pods$`, s.assertServedByManyPods)
}

// registerSecretSteps binds the steps that create, rotate, read and delete a secret and read back
// what a gateway sends upstream with it.
func (s *Steps) registerSecretSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I create a synthetic secret$`, s.createSecret)
	sc.Step(`^I create a synthetic secret holding a sample key with the prefix "([^"]*)"$`, s.createSecretWithPrefix)
	sc.Step(`^I create a replacement synthetic secret$`, s.createReplacementSecret)
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
	if err := reg.RegisterDeleter(cleanup.KindProject, cloudDeleter(topo, client, "/projects", false)); err != nil {
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
	if err := reg.RegisterDeleter(kindLLMProxy, cloudDeleter(topo, client, llmProxiesCollection, false)); err != nil {
		return err
	}
	if err := reg.RegisterDeleter(kindLLMProvider, cloudDeleter(topo, client, llmProvidersCollection, false)); err != nil {
		return err
	}
	return reg.RegisterDeleter(cleanup.KindGateway, cloudDeleter(topo, client, "/managed-gateways", false))
}

// environmentsCollection is the collection an environment is deleted from.
const environmentsCollection = "/environments"

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
		base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
		if err != nil {
			return err
		}
		bearer, err := bearerToken(ctx)
		if err != nil {
			return err
		}
		if collection == environmentsCollection {
			if err := emptyEnvironment(ctx, client, base, bearer, resource.ID); err != nil {
				return err
			}
		}
		deleteOne := func(ctx context.Context) (*httpx.Response, error) {
			return client.Do(ctx, httpx.Request{
				Method: http.MethodDelete,
				URL:    cloudResourceURL(base, collection, resource.ID),
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
		return retry.Await(ctx, retry.Options{Timeout: 2 * time.Minute, Interval: 3 * time.Second},
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
			"waiting for environment "+resource.ID+" to be deleted")
	}
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
