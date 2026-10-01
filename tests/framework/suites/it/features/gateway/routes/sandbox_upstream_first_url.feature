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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@sandbox-upstream-first-url
Feature: Sandbox upstream first URL selection
  As an API developer
  I want the first configured upstream URL to serve sandbox traffic
  So that multiple upstream definitions route predictably

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Sandbox ref with multiple URLs should use the first URL
    Given I generate a unique value from "sandbox-routing-first-url-multi" and store it as "firstURLAPIName"
    And I generate a unique API context from "/sandbox-routing-first-url-multi" and store it as "firstURLAPIContext"
    And I generate a unique resource name from "sandbox-routing-first-url-multi-mainHost" and store it as "firstURLMainHost"
    And I generate a unique resource name from "sandbox-routing-first-url-multi-sandboxHost" and store it as "firstURLSandboxHost"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | ${CTX:gatewaySpecVersion} |
      | name | ${CTX:firstURLAPIName} |
      | spec | {"displayName":"Env-Routing-Sandbox-Multi-URL-API","version":"v1.0","context":"${CTX:firstURLAPIContext}/$version","vhosts":{"main":"${CTX:firstURLMainHost}","sandbox":"${CTX:firstURLSandboxHost}"},"upstreamDefinitions":[{"name":"first-url-sandbox-upstream","upstreams":[{"url":"http://testbench:3000/first"},{"url":"http://testbench:3000/sandbox"}]}],"upstream":{"main":{"url":"http://testbench:3000"},"sandbox":{"ref":"first-url-sandbox-upstream"}},"operations":[{"method":"GET","path":"/whoami"}]} |
    Then the response should be successful
    And I set request host to "${CTX:firstURLMainHost}"
    And I send a "GET" request to "${CTX:firstURLAPIContext}/v1.0/whoami" until status 200

    When I clear all headers
    And I set request host to "${CTX:firstURLSandboxHost}"
    And I send a "GET" request to "${CTX:firstURLAPIContext}/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/first/whoami"

    Given I authenticate using basic auth as "admin"
    When I delete the API "${CTX:firstURLAPIName}"
    Then the response should be successful
    And I set request host to "${CTX:firstURLMainHost}"
    And I send a "GET" request to "${CTX:firstURLAPIContext}/v1.0/whoami" until status 404
