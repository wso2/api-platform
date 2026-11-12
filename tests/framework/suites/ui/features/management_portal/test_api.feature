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

@CONSOLE
Feature: The Management Portal's Test page calls a deployed API
  As an API publisher
  I want to call my deployed API from the console, or copy the command that calls it
  So that I can see how the gateway answers before anyone else does

  @CONSOLE-011 @CONSOLE-012 @CONSOLE-013
  Scenario: Journey: a publisher tests a deployed API through the proxy, directly from the browser and with a cURL command
    Given I generate a unique handle from "console-test-project" and store it as "project"
    And I generate a unique handle from "console-test" and store it as "api"
    And a project "${CTX:project}" exists on the control plane
    And a REST API "${CTX:api}" is imported from spec fixture "petstore-v1.yaml" in project "${CTX:project}" on the control plane
    # Only this operation lets another origin read its responses, which a call from the browser needs.
    And operation "GET /store/inventory" of REST API "${CTX:api}" has policy "cors" on the control plane with params:
      """
      {"allowedOrigins": ["*"]}
      """
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/deploy"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "first"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the gateway answers 200 to a "GET" request to "/${CTX:api}/store/inventory"
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/test"
    # Through the proxy, the default.
    And the user sends "GET /store/inventory" from the Test page's Console view
    Then the Test page shows the cURL command and request URL "http://gateway-runtime:8080/${CTX:api}/store/inventory" for "GET /store/inventory"
    And the Test page's response to "GET /store/inventory" is 200 with the backend echoing "GET /store/inventory" sent through the proxy
    # Directly from the browser: the operation with the CORS policy answers, the other is refused.
    When the user clicks "Direct" in the Management Portal
    And the user sends "GET /store/inventory" from the Test page's Console view
    Then the Test page's response to "GET /store/inventory" is 200 with the backend echoing "GET /store/inventory" sent from the browser
    When the user sends "GET /pets" from the Test page's Console view
    Then the Test page's response to "GET /pets" shows a CORS or network error
    # The cURL view: the command follows what is typed, and runs as the Console view's request did.
    When the user clicks "cURL view" in the Management Portal
    And the user enters "/pets" as the "Request path" in the Management Portal
    Then the cURL command in the Management Portal contains "'http://gateway-runtime:8080/${CTX:api}/pets'"
    When the user enters "/store/inventory" as the "Request path" in the Management Portal
    Then the cURL command in the Management Portal contains "'http://gateway-runtime:8080/${CTX:api}/store/inventory'"
    When the user chooses "Headers" in the Management Portal
    And the user adds "X-Console-Test" with value "one" in the cURL builder
    Then the cURL command in the Management Portal contains "-H 'X-Console-Test: one'"
    And running the cURL command in a terminal answers 200 with the backend echoing "GET /store/inventory" and header "X-Console-Test" set to "one"
