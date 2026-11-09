# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except in compliance
# with the License. You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# --------------------------------------------------------------------

@apip-cloud
Feature: APIP cloud secrets
  As an APIP cloud user
  I want secrets to be stored without ever being shown again and to reach the gateways that use them
  So that credentials never appear in a read response and a deployed API sends the value I stored

  @TC_APIP_SEC_001 @ID_APIP_SECRETS_001
  Scenario: A secret created in the console is sent by the gateway to the upstream of a deployed API
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    And an echo backend is available for deployment overrides
    And I create a synthetic secret
    When I create a synthetic REST API in the default project with:
      | policies | [{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"x-apip-secret-probe","value":"${CTX:cloudSecretReference}"}]}}}] |
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment}                         |
      | gatewayId   | ${CTX:cloudGatewayID}                                  |
      | parameters  | {"productionEndpoint":"${CTX:cloudEchoBackendURL}"} |
    Then the cloud API response status code should be 201
    And the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    And the echoed request header "x-apip-secret-probe" should carry the synthetic secret's value

  # Known issue: the dev CP gateway does not route PUT /secrets/{id} (B33); behind it, nothing pushes a rotation to gateways (B12).
  @TC_APIP_SEC_003 @known-issue @ID_APIP_SECRETS_002
  Scenario: A rotated secret reaches the gateway without a redeployment
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    And an echo backend is available for deployment overrides
    And I create a synthetic secret
    When I create a synthetic REST API in the default project with:
      | policies | [{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"x-apip-secret-probe","value":"${CTX:cloudSecretReference}"}]}}}] |
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment}                         |
      | gatewayId   | ${CTX:cloudGatewayID}                                  |
      | parameters  | {"productionEndpoint":"${CTX:cloudEchoBackendURL}"} |
    Then the cloud API response status code should be 201
    And the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    And the echoed request header "x-apip-secret-probe" should carry the synthetic secret's value
    When I rotate the synthetic secret
    Then a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" should echo the header "x-apip-secret-probe" with the rotated secret value

  @TC_APIP_SEC_004 @ID_APIP_SECRETS_003
  Scenario: A secret in use cannot be deleted, and once deleted it can no longer be used
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    And an echo backend is available for deployment overrides
    And I create a synthetic secret
    When I create a synthetic REST API in the default project with:
      | policies | [{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"x-apip-secret-probe","value":"${CTX:cloudSecretReference}"}]}}}] |
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment}                         |
      | gatewayId   | ${CTX:cloudGatewayID}                                  |
      | parameters  | {"productionEndpoint":"${CTX:cloudEchoBackendURL}"} |
    Then the cloud API response status code should be 201
    And the synthetic REST API deployment should become DEPLOYED
    When I delete the synthetic secret
    Then the cloud API response status code should be 409
    And the cloud API error code should be "SECRET_IN_USE"
    And the cloud API response body should contain "${CTX:cloudAPIID}"
    When I undeploy the synthetic REST API
    Then the synthetic REST API deployment should become UNDEPLOYED
    When I delete the synthetic REST API
    And I delete the synthetic secret
    Then the cloud API response status code should be 204
    When I submit a synthetic REST API creation request with:
      | policies | [{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"x-apip-secret-probe","value":"${CTX:cloudSecretReference}"}]}}}] |
    Then the cloud API response status code should be 400
    And the cloud API error code should be "VALIDATION_FAILED"

  # Known issue: the dev CP gateway does not route GET /secrets/{id} (B33).
  @TC_APIP_SEC_005 @known-issue @ID_APIP_SECRETS_004
  Scenario: A secret's value is never returned in a read response
    Given I obtain an APIP cloud console token
    When I create a synthetic secret
    Then the response should not reveal the synthetic secret's value
    When I read the synthetic secret
    Then the response should not reveal the synthetic secret's value
    And the synthetic secret should be described by a hash
    When I list the secrets
    Then the response should not reveal the synthetic secret's value

  @TC_APIP_SEC_006 @ID_APIP_SECRETS_005
  Scenario: A REST API that references a secret that does not exist is refused when it is created
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I choose a synthetic secret handle without creating the secret
    When I submit a synthetic REST API creation request with:
      | policies | [{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"x-apip-secret-probe","value":"${CTX:cloudSecretReference}"}]}}}] |
    Then the cloud API response status code should be 400
    And the cloud API error code should be "VALIDATION_FAILED"
    And the cloud API response body should contain "${CTX:cloudSecretID}"
