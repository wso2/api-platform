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

@template-functions @GWENV
Feature: Environment variable allow list for artifact templates
  As a gateway operator
  I want {{ env }} in a resource spec to read only the environment variables I
  allow, so that an API definition cannot pull the gateway-controller's own
  configuration (credentials, tokens) into an upstream or a policy parameter.

  # The suite's gateway-controller overlay sets
  # [policy_engine.env_access] allow_list = ["IT_*"]. Allowed reads are covered by
  # template_functions.feature and template_functions_typed.feature, whose IT_*
  # variables resolve under that allow list. APIP_GW_CONTROLLER_LOGGING_LEVEL is
  # set ("debug") on the controller but is not on the allow list.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  @GWENV-001 @type:negative
  Scenario: env template in upstream URL naming a variable outside the allow list is rejected
    # Control: a valid API created and read the same way, so the 404 below cannot
    # pass because the lookup itself is broken.
    Given I generate a unique resource name from "tpl-env-denied-upstream-control" and store it as "controlName"
    And I generate a unique API version from "tpl-env-denied-upstream-control" and store it as "controlVersion"
    And I generate a unique API context from "/tpl-env-denied-upstream-control" and store it as "controlContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}          |
      | name                   | ${CTX:controlName}                 |
      | spec.displayName       | Tpl-Env-Denied-Upstream-Control |
      | spec.version           | ${CTX:controlVersion}              |
      | spec.context           | ${CTX:controlContext}/$version     |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/probe"}] |
    Then the response status code should be 201

    Given I generate a unique resource name from "tpl-env-denied-upstream" and store it as "apiName"
    And I generate a unique API version from "tpl-env-denied-upstream" and store it as "apiVersion"
    And I generate a unique API context from "/tpl-env-denied-upstream" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                       |
      | name                   | ${CTX:apiName}                                                  |
      | spec.displayName       | Tpl-Env-Denied-Upstream                                         |
      | spec.version           | ${CTX:apiVersion}                                               |
      | spec.context           | ${CTX:apiContext}/$version                                      |
      | spec.upstream.main.url | http://testbench:3002/{{ env "APIP_GW_CONTROLLER_LOGGING_LEVEL" }} |
      | spec.operations        | [{"method":"GET","path":"/probe"}]                               |
    Then the response status code should be 400
    And the JSON response field "message" should be "environment variable is not permitted in artifact templates"
    And the response body should not contain "APIP_GW_CONTROLLER_LOGGING_LEVEL"
    And the response body should not contain "debug"

    # Nothing was persisted for the rejected definition; the control is found.
    When I get the API "${CTX:controlName}"
    Then the response status code should be 200
    When I get the API "${CTX:apiName}"
    Then the response status code should be 404

  @GWENV-002 @type:negative
  Scenario: env template in a policy parameter naming a variable outside the allow list is rejected
    # Control: a valid API created and read the same way, so the 404 below cannot
    # pass because the lookup itself is broken.
    Given I generate a unique resource name from "tpl-env-denied-policy-control" and store it as "controlName"
    And I generate a unique API version from "tpl-env-denied-policy-control" and store it as "controlVersion"
    And I generate a unique API context from "/tpl-env-denied-policy-control" and store it as "controlContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}          |
      | name                   | ${CTX:controlName}                 |
      | spec.displayName       | Tpl-Env-Denied-Policy-Control |
      | spec.version           | ${CTX:controlVersion}              |
      | spec.context           | ${CTX:controlContext}/$version     |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/probe"}] |
    Then the response status code should be 201

    Given I generate a unique resource name from "tpl-env-denied-policy" and store it as "apiName"
    And I generate a unique API version from "tpl-env-denied-policy" and store it as "apiVersion"
    And I generate a unique API context from "/tpl-env-denied-policy" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Tpl-Env-Denied-Policy     |
      | spec.version           | ${CTX:apiVersion}         |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002     |
      | spec.operations        | [{"method":"GET","path":"/probe","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Leak","value":"{{ env \"APIP_GW_CONTROLLER_LOGGING_LEVEL\" }}"}]}}}]}] |
    Then the response status code should be 400
    And the JSON response field "message" should be "environment variable is not permitted in artifact templates"
    And the response body should not contain "APIP_GW_CONTROLLER_LOGGING_LEVEL"

    When I get the API "${CTX:controlName}"
    Then the response status code should be 200
    When I get the API "${CTX:apiName}"
    Then the response status code should be 404

  @GWENV-003 @type:negative
  Scenario: updating a deployed API to read a variable outside the allow list is rejected and leaves it serving
    Given I generate a unique resource name from "tpl-env-denied-update" and store it as "apiName"
    And I generate a unique API version from "tpl-env-denied-update" and store it as "apiVersion"
    And I generate a unique API context from "/tpl-env-denied-update" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}          |
      | name                   | ${CTX:apiName}                     |
      | spec.displayName       | Tpl-Env-Denied-Update              |
      | spec.version           | ${CTX:apiVersion}                  |
      | spec.context           | ${CTX:apiContext}/$version         |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/probe"}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe" until status 200

    When I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}  |
      | name                   | ${CTX:apiName}             |
      | spec.displayName       | Tpl-Env-Denied-Update      |
      | spec.version           | ${CTX:apiVersion}          |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002      |
      | spec.operations        | [{"method":"GET","path":"/probe","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Leak","value":"{{ env \"APIP_GW_CONTROLLER_LOGGING_LEVEL\" }}"}]}}}]}] |
    Then the response status code should be 400
    And the JSON response field "message" should be "environment variable is not permitted in artifact templates"
    And the response body should not contain "APIP_GW_CONTROLLER_LOGGING_LEVEL"
    And the response body should not contain "debug"

    # The previous definition is kept and keeps serving, with no leaked header.
    When I get the API "${CTX:apiName}"
    Then the response status code should be 200
    And the response body should not contain "APIP_GW_CONTROLLER_LOGGING_LEVEL"
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe"
    Then the response status code should be 200
    And the response should not contain echoed header "X-Leak"
