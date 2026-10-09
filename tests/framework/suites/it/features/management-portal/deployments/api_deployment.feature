# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@platform-api-deployment
Feature: Platform-API-driven API deployment is served by the gateway data plane
  As an API platform operator
  I want an API created and deployed through platform-api to be served by the real gateway
  data plane, and to stop being served once undeployed
  So that the control plane and data plane genuinely work together, not just at the
  management-plane level

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "papi-deploy-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  @smoke
  Scenario: An API deployed to a gateway is served by the data plane
    Given I generate a unique resource name from "papi-deploy" and store it as "apiHandle"
    And I generate a unique API context from "/papi-deploy" and store it as "apiContext"
    And I generate a unique API context from "/papi-deploy-unmapped" and store it as "unmappedContext"
    When I create a REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane
    Then I send a "GET" request to "${CTX:apiContext}/health" until status 200
    When I send a "GET" request to "${CTX:unmappedContext}/health"
    Then the response status code should be 404

  Scenario: Undeploying stops the gateway serving the API, and redeploying restores it
    Given I generate a unique resource name from "papi-redeploy" and store it as "apiHandle"
    And I generate a unique API context from "/papi-redeploy" and store it as "apiContext"
    And I create a REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to "${CTX:apiContext}/health" until status 200

    When I undeploy the "RestApi" "${CTX:apiHandle}" deployment "${CTX:deploymentId}" from the gateway via the control plane
    Then I send a "GET" request to "${CTX:apiContext}/health" until status 404

    When I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane
    Then I send a "GET" request to "${CTX:apiContext}/health" until status 200
