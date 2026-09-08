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

@redirect
Feature: Redirect responses
  As an API user
  I want gateway routing behavior to follow the configured rules
  So that requests reach the intended endpoint and response

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"


  Scenario: Redirect to a different host defaults to 302 and preserves scheme, port and path
    Given I generate a unique value from "redirect-1" and store it as "apiName1"
    And I generate a unique API context from "/redirect-1" and store it as "apiContext1"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                 | gateway.api-platform.wso2.com/v1 |
      | name                       | ${CTX:apiName1}                  |
      | spec.displayName            | Redirect-Host-Test                |
      | spec.version                | v1.0.0                            |
      | spec.context                | ${CTX:apiContext1}/$version       |
      | spec.upstream.main.url      | http://testbench:3000             |
      | spec.operations             | [{"method":"GET","path":"/probe"},{"method":"GET","path":"/go","policies":[{"name":"redirect","version":"v0","params":{"hostname":"example.org"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext1}/v1.0.0/probe" until status 200
    When I send a "GET" request to "${CTX:apiContext1}/v1.0.0/go"
    Then the response status code should be 302
    And the response header "Location" should match pattern "^http://example\.org:[0-9]+${CTX:apiContext1}/v1\.0\.0/go$"


  Scenario: Explicit status code produces a permanent 301 redirect
    Given I generate a unique value from "redirect-2" and store it as "apiName2"
    And I generate a unique API context from "/redirect-2" and store it as "apiContext2"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                 | gateway.api-platform.wso2.com/v1 |
      | name                       | ${CTX:apiName2}                  |
      | spec.displayName            | Redirect-Permanent-Test           |
      | spec.version                | v1.0.0                            |
      | spec.context                | ${CTX:apiContext2}/$version       |
      | spec.upstream.main.url      | http://testbench:3000             |
      | spec.operations             | [{"method":"GET","path":"/probe"},{"method":"GET","path":"/go","policies":[{"name":"redirect","version":"v0","params":{"statusCode":301,"hostname":"example.org"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext2}/v1.0.0/probe" until status 200
    When I send a "GET" request to "${CTX:apiContext2}/v1.0.0/go"
    Then the response status code should be 301
    And the response header "Location" should match pattern "^http://example\.org:[0-9]+${CTX:apiContext2}/v1\.0\.0/go$"


  Scenario: Scheme upgrade to https drops the default port and preserves host and path
    Given I generate a unique value from "redirect-3" and store it as "apiName3"
    And I generate a unique API context from "/redirect-3" and store it as "apiContext3"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                 | gateway.api-platform.wso2.com/v1 |
      | name                       | ${CTX:apiName3}                  |
      | spec.displayName            | Redirect-Scheme-Test              |
      | spec.version                | v1.0.0                            |
      | spec.context                | ${CTX:apiContext3}/$version       |
      | spec.upstream.main.url      | http://testbench:3000             |
      | spec.operations             | [{"method":"GET","path":"/probe"},{"method":"GET","path":"/go","policies":[{"name":"redirect","version":"v0","params":{"scheme":"https"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext3}/v1.0.0/probe" until status 200
    When I send a "GET" request to "${CTX:apiContext3}/v1.0.0/go"
    Then the response status code should be 302
    And the response header "Location" should match pattern "^https://[^/]+${CTX:apiContext3}/v1\.0\.0/go$"


  Scenario: Full path replacement rewrites the entire path
    Given I generate a unique value from "redirect-4" and store it as "apiName4"
    And I generate a unique API context from "/redirect-4" and store it as "apiContext4"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                 | gateway.api-platform.wso2.com/v1 |
      | name                       | ${CTX:apiName4}                  |
      | spec.displayName            | Redirect-FullPath-Test            |
      | spec.version                | v1.0.0                            |
      | spec.context                | ${CTX:apiContext4}/$version       |
      | spec.upstream.main.url      | http://testbench:3000             |
      | spec.operations             | [{"method":"GET","path":"/probe"},{"method":"GET","path":"/go","policies":[{"name":"redirect","version":"v0","params":{"hostname":"example.org","path":{"mode":"full","value":"/v2/target"}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext4}/v1.0.0/probe" until status 200
    When I send a "GET" request to "${CTX:apiContext4}/v1.0.0/go"
    Then the response status code should be 302
    And the response header "Location" should match pattern "^http://example\.org:[0-9]+/v2/target$"


  Scenario: Overriding every component builds a fully rewritten Location with a 308 status
    Given I generate a unique value from "redirect-5" and store it as "apiName5"
    And I generate a unique API context from "/redirect-5" and store it as "apiContext5"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                 | gateway.api-platform.wso2.com/v1 |
      | name                       | ${CTX:apiName5}                  |
      | spec.displayName            | Redirect-Full-Override-Test       |
      | spec.version                | v1.0.0                            |
      | spec.context                | ${CTX:apiContext5}/$version       |
      | spec.upstream.main.url      | http://testbench:3000             |
      | spec.operations             | [{"method":"GET","path":"/probe"},{"method":"GET","path":"/go","policies":[{"name":"redirect","version":"v0","params":{"statusCode":308,"scheme":"https","hostname":"newhost.example.com","port":8443,"path":{"mode":"full","value":"/moved"}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext5}/v1.0.0/probe" until status 200
    When I send a "GET" request to "${CTX:apiContext5}/v1.0.0/go"
    Then the response status code should be 308
    And the response header "Location" should be "https://newhost.example.com:8443/moved"
