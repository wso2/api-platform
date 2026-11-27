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

# Gateway 1.1.0 drops a /foo/* route's literal prefix on the way upstream (#2071, fixed in
# 1.2.0), so this feature runs on gateway 1.2.0 and later.
@SVR @sandbox-routing-wildcard
Feature: Sandbox routing for wildcard operations
  As an API developer
  I want a wildcard operation's full path to reach the sandbox upstream
  So that sandbox traffic reaches the same resource as main traffic

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  @SVR-023
  Scenario: Wildcard route should work with sandbox ref
    Given I generate a unique value from "sandbox-routing-13" and store it as "apiName13"
    And I generate a unique API context from "/sandbox-routing-13" and store it as "apiContext13"
    And I generate a unique resource name from "sandbox-routing-13-mainHost13" and store it as "mainHost13"
    And I generate a unique resource name from "sandbox-routing-13-sandboxHost13" and store it as "sandboxHost13"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | ${CTX:gatewaySpecVersion} |
      | name | ${CTX:apiName13} |
      | spec | {"displayName":"Env-Routing-Sandbox-Wildcard-Ref-API","version":"v1.0","context":"${CTX:apiContext13}/$version","vhosts":{"main":"${CTX:mainHost13}","sandbox":"${CTX:sandboxHost13}"},"upstreamDefinitions":[{"name":"sandbox-upstream","basePath":"/sandbox","upstreams":[{"url":"http://testbench:3000"}]}],"upstream":{"main":{"url":"http://testbench:3000"},"sandbox":{"ref":"sandbox-upstream"}},"operations":[{"method":"GET","path":"/assets/*"}]} |
    Then the response should be successful
    And I set request host to "${CTX:mainHost13}"
    And I send a "GET" request to "${CTX:apiContext13}/v1.0/assets/a/b/c" until status 200

    When I clear all headers
    And I set request host to "${CTX:sandboxHost13}"
    And I send a "GET" request to "${CTX:apiContext13}/v1.0/assets/a/b/c"
    Then the response should be successful
    And the JSON response field "path" should be "/sandbox/assets/a/b/c"

    Given I authenticate using basic auth as "admin"
    When I delete the API "${CTX:apiName13}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext13}/v1.0/assets/a/b/c" until status 404
