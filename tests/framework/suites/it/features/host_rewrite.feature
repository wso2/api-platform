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

@host-rewrite
Feature: Host rewrite routing
  As an API user
  I want gateway routing behavior to follow the configured rules
  So that requests reach the intended endpoint and response

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"


  Scenario: Host rewrite sets the Host header on upstream request
    Given I generate a unique value from "host-rewrite-1" and store it as "apiName1"
    And I generate a unique API context from "/host-rewrite-1" and store it as "apiContext1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1                              |
      | name                  | ${CTX:apiName1}                                               |
      | spec.displayName      | Host Rewrite Basic                                           |
      | spec.version          | v1.0                                                          |
      | spec.context          | ${CTX:apiContext1}/$version                                   |
      | spec.upstream.main.url| http://testbench:3002/anything                                |
      | spec.upstream.main.hostRewrite | manual                                    |
      | spec.operations       | [{"method":"GET","path":"/test","policies":[{"name":"host-rewrite","version":"v1","params":{"host":"example-updated.com"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/test" until status 200
    When I send a "GET" request to "${CTX:apiContext1}/v1.0/test"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "example-updated.com"


  Scenario: Host rewrite with port number
    Given I generate a unique value from "host-rewrite-2" and store it as "apiName2"
    And I generate a unique API context from "/host-rewrite-2" and store it as "apiContext2"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1                              |
      | name                  | ${CTX:apiName2}                                               |
      | spec.displayName      | Host Rewrite With Port                                       |
      | spec.version          | v1.0                                                          |
      | spec.context          | ${CTX:apiContext2}/$version                                   |
      | spec.upstream.main.url| http://testbench:3002/anything                                |
      | spec.upstream.main.hostRewrite | manual                                    |
      | spec.operations       | [{"method":"GET","path":"/test","policies":[{"name":"host-rewrite","version":"v1","params":{"host":"backend.example.com:8080"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/test" until status 200
    When I send a "GET" request to "${CTX:apiContext2}/v1.0/test"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "backend.example.com:8080"


  Scenario: Host rewrite at API level applies to all operations
    Given I generate a unique value from "host-rewrite-3" and store it as "apiName3"
    And I generate a unique API context from "/host-rewrite-3" and store it as "apiContext3"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1                              |
      | name                  | ${CTX:apiName3}                                               |
      | spec.displayName      | Host Rewrite API Level                                        |
      | spec.version          | v1.0                                                          |
      | spec.context          | ${CTX:apiContext3}/$version                                   |
      | spec.upstream.main.url| http://testbench:3002/anything                                |
      | spec.upstream.main.hostRewrite | manual                                    |
      | spec.policies         | [{"name":"host-rewrite","version":"v1","params":{"host":"api-level.example.com"}}] |
      | spec.operations       | [{"method":"GET","path":"/test1"},{"method":"POST","path":"/test2"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext3}/v1.0/test1" until status 200
    When I send a "GET" request to "${CTX:apiContext3}/v1.0/test1"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "api-level.example.com"
    When I send a "POST" request to "${CTX:apiContext3}/v1.0/test2"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "api-level.example.com"


  Scenario: Host rewrite without hostRewrite manual should not work
    Given I generate a unique value from "host-rewrite-4" and store it as "apiName4"
    And I generate a unique API context from "/host-rewrite-4" and store it as "apiContext4"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1                              |
      | name                  | ${CTX:apiName4}                                               |
      | spec.displayName      | Host Rewrite No Manual                                       |
      | spec.version          | v1.0                                                          |
      | spec.context          | ${CTX:apiContext4}/$version                                   |
      | spec.upstream.main.url| http://testbench:3002/anything                                |
      | spec.operations       | [{"method":"GET","path":"/test","policies":[{"name":"host-rewrite","version":"v1","params":{"host":"should-not-be-used.com"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext4}/v1.0/test" until status 200
    When I send a "GET" request to "${CTX:apiContext4}/v1.0/test"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should contain "testbench"


  Scenario: Operation-level policy overrides API-level policy
    Given I generate a unique value from "host-rewrite-5" and store it as "apiName5"
    And I generate a unique API context from "/host-rewrite-5" and store it as "apiContext5"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1                              |
      | name                  | ${CTX:apiName5}                                               |
      | spec.displayName      | Host Rewrite Override                                         |
      | spec.version          | v1.0                                                          |
      | spec.context          | ${CTX:apiContext5}/$version                                   |
      | spec.upstream.main.url| http://testbench:3002/anything                                |
      | spec.upstream.main.hostRewrite | manual                                    |
      | spec.policies         | [{"name":"host-rewrite","version":"v1","params":{"host":"api-level.example.com"}}] |
      | spec.operations       | [{"method":"GET","path":"/default"},{"method":"GET","path":"/override","policies":[{"name":"host-rewrite","version":"v1","params":{"host":"operation-level.example.com"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext5}/v1.0/default" until status 200
    When I send a "GET" request to "${CTX:apiContext5}/v1.0/default"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "api-level.example.com"
    When I send a "GET" request to "${CTX:apiContext5}/v1.0/override"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "operation-level.example.com"


  Scenario: Host rewrite works with different HTTP methods
    Given I generate a unique value from "host-rewrite-6" and store it as "apiName6"
    And I generate a unique API context from "/host-rewrite-6" and store it as "apiContext6"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1                              |
      | name                  | ${CTX:apiName6}                                               |
      | spec.displayName      | Host Rewrite HTTP Methods                                     |
      | spec.version          | v1.0                                                          |
      | spec.context          | ${CTX:apiContext6}/$version                                   |
      | spec.upstream.main.url| http://testbench:3002/anything                                |
      | spec.upstream.main.hostRewrite | manual                                    |
      | spec.operations       | [{"method":"GET","path":"/test","policies":[{"name":"host-rewrite","version":"v1","params":{"host":"get.example.com"}}]},{"method":"POST","path":"/test","policies":[{"name":"host-rewrite","version":"v1","params":{"host":"post.example.com"}}]},{"method":"PUT","path":"/test","policies":[{"name":"host-rewrite","version":"v1","params":{"host":"put.example.com"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext6}/v1.0/test" until status 200
    When I send a "GET" request to "${CTX:apiContext6}/v1.0/test"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "get.example.com"
    When I send a "POST" request to "${CTX:apiContext6}/v1.0/test"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "post.example.com"
    When I send a "PUT" request to "${CTX:apiContext6}/v1.0/test" with body:
      """
      {"test": "data"}
      """
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "put.example.com"
