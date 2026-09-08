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

@request-rewrite
Feature: Request transformation routing
  As an API user
  I want gateway routing behavior to follow the configured rules
  So that requests reach the intended endpoint and response

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"


  Scenario: ReplacePrefixMatch rewrites the path prefix
    Given I generate a unique value from "request-rewrite-1" and store it as "apiName1"
    And I generate a unique API context from "/request-rewrite-1" and store it as "apiContext1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName1}                    |
      | spec.displayName       | Request Transformation Prefix     |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext1}/$version        |
      | spec.upstream.main.url | http://testbench:3002/anything     |
      | spec.operations        | [{"method":"GET","path":"/api/v1","policies":[{"name":"request-rewrite","version":"v1","params":{"pathRewrite":{"type":"ReplacePrefixMatch","replacePrefixMatch":"/api/v2"}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/api/v1" until status 200
    And I set header "Content-Type" to "application/json"
    When I send a "GET" request to "${CTX:apiContext1}/v1.0/api/v1"
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/api/v2"


  Scenario: ReplaceFullPath rewrites the entire path
    Given I generate a unique value from "request-rewrite-2" and store it as "apiName2"
    And I generate a unique API context from "/request-rewrite-2" and store it as "apiContext2"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName2}                    |
      | spec.displayName       | Request Transformation Full Path  |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext2}/$version        |
      | spec.upstream.main.url | http://testbench:3002/anything     |
      | spec.operations        | [{"method":"GET","path":"/api/v1","policies":[{"name":"request-rewrite","version":"v1","params":{"pathRewrite":{"type":"ReplaceFullPath","replaceFullPath":"/fixed/path"}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/api/v1" until status 200
    And I set header "Content-Type" to "application/json"
    When I send a "GET" request to "${CTX:apiContext2}/v1.0/api/v1"
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/fixed/path"


  Scenario: ReplaceRegexMatch rewrites using regex substitution
    Given I generate a unique value from "request-rewrite-3" and store it as "apiName3"
    And I generate a unique API context from "/request-rewrite-3" and store it as "apiContext3"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName3}                    |
      | spec.displayName       | Request Transformation Regex      |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext3}/$version        |
      | spec.upstream.main.url | http://testbench:3002/anything     |
      | spec.operations        | [{"method":"GET","path":"/api/v1","policies":[{"name":"request-rewrite","version":"v1","params":{"pathRewrite":{"type":"ReplaceRegexMatch","replaceRegexMatch":{"pattern":"^/api/v1$","substitution":"/api/v2"}}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext3}/v1.0/api/v1" until status 200
    And I set header "Content-Type" to "application/json"
    When I send a "GET" request to "${CTX:apiContext3}/v1.0/api/v1"
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/api/v2"


  Scenario: ReplaceRegexMatch reorders captured segments
    Given I generate a unique value from "request-rewrite-4" and store it as "apiName4"
    And I generate a unique API context from "/request-rewrite-4" and store it as "apiContext4"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName4}                    |
      | spec.displayName       | Request Transformation Regex Capture |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext4}/$version        |
      | spec.upstream.main.url | http://testbench:3002/anything     |
      | spec.operations        | [{"method":"GET","path":"/*","policies":[{"name":"request-rewrite","version":"v1","params":{"pathRewrite":{"type":"ReplaceRegexMatch","replaceRegexMatch":{"pattern":"^/service/([^/]+)(/.*)$","substitution":"\\\\2/instance/\\\\1"}}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext4}/v1.0/service/foo/v1/api" until status 200
    And I set header "Content-Type" to "application/json"
    When I send a "GET" request to "${CTX:apiContext4}/v1.0/service/foo/v1/api"
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/v1/api/instance/foo"


  Scenario: ReplaceRegexMatch is case-insensitive
    Given I generate a unique value from "request-rewrite-5" and store it as "apiName5"
    And I generate a unique API context from "/request-rewrite-5" and store it as "apiContext5"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName5}                    |
      | spec.displayName       | Request Transformation Regex Case Insensitive |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext5}/$version        |
      | spec.upstream.main.url | http://testbench:3002/anything     |
      | spec.operations        | [{"method":"GET","path":"/*","policies":[{"name":"request-rewrite","version":"v1","params":{"pathRewrite":{"type":"ReplaceRegexMatch","replaceRegexMatch":{"pattern":"(?i)/xxx/","substitution":"/yyy/"}}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext5}/v1.0/aaa/XxX/bbb" until status 200
    And I set header "Content-Type" to "application/json"
    When I send a "GET" request to "${CTX:apiContext5}/v1.0/aaa/XxX/bbb"
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/aaa/yyy/bbb"


  Scenario: ReplaceRegexMatch replaces all matches
    Given I generate a unique value from "request-rewrite-6" and store it as "apiName6"
    And I generate a unique API context from "/request-rewrite-6" and store it as "apiContext6"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName6}                    |
      | spec.displayName       | Request Transformation Regex Replace All |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext6}/$version        |
      | spec.upstream.main.url | http://testbench:3002/anything     |
      | spec.operations        | [{"method":"GET","path":"/*","policies":[{"name":"request-rewrite","version":"v1","params":{"pathRewrite":{"type":"ReplaceRegexMatch","replaceRegexMatch":{"pattern":"one","substitution":"two"}}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext6}/v1.0/xxx/one/yyy/one/zzz" until status 200
    And I set header "Content-Type" to "application/json"
    When I send a "GET" request to "${CTX:apiContext6}/v1.0/xxx/one/yyy/one/zzz"
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/xxx/two/yyy/two/zzz"


  Scenario: Query rewrite adds, replaces, and removes parameters
    Given I generate a unique value from "request-rewrite-7" and store it as "apiName7"
    And I generate a unique API context from "/request-rewrite-7" and store it as "apiContext7"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName7}                    |
      | spec.displayName       | Request Transformation Query      |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext7}/$version        |
      | spec.upstream.main.url | http://testbench:3002/anything     |
      | spec.operations        | [{"method":"GET","path":"/search","policies":[{"name":"request-rewrite","version":"v1","params":{"queryRewrite":{"rules":[{"action":"Add","name":"source","value":"legacy"},{"action":"Replace","name":"q","value":"new-value"},{"action":"Remove","name":"debug"}]}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext7}/v1.0/search?q=old-value&debug=true" until status 200
    And I set header "Content-Type" to "application/json"
    When I send a "GET" request to "${CTX:apiContext7}/v1.0/search?q=old-value&debug=true"
    Then the response status code should be 200
    And the JSON response field "args.source" should be "legacy"
    And the JSON response field "args.q" should be "new-value"


  Scenario: Method rewrite changes the request method
    Given I generate a unique value from "request-rewrite-8" and store it as "apiName8"
    And I generate a unique API context from "/request-rewrite-8" and store it as "apiContext8"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName8}                    |
      | spec.displayName       | Request Transformation Method     |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext8}/$version        |
      | spec.upstream.main.url | http://testbench:3002/anything     |
      | spec.operations        | [{"method":"GET","path":"/test/*","policies":[{"name":"request-rewrite","version":"v1","params":{"methodRewrite":"POST"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext8}/v1.0/test/hello" until status 200
    And I set header "Content-Type" to "application/json"
    When I send a "GET" request to "${CTX:apiContext8}/v1.0/test/hello"
    Then the response status code should be 200
    And the JSON response field "method" should be "POST"


  Scenario: API-level policy rewrites the path prefix
    Given I generate a unique value from "request-rewrite-9" and store it as "apiName9"
    And I generate a unique API context from "/request-rewrite-9" and store it as "apiContext9"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName9}                    |
      | spec.displayName       | Request Transformation API Level Prefix |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext9}/$version        |
      | spec.upstream.main.url | http://testbench:3002/anything     |
      | spec.policies          | [{"name":"request-rewrite","version":"v1","params":{"pathRewrite":{"type":"ReplacePrefixMatch","replacePrefixMatch":"/api/v2"}}}] |
      | spec.operations        | [{"method":"GET","path":"/api/v1"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext9}/v1.0/api/v1" until status 200
    And I set header "Content-Type" to "application/json"
    When I send a "GET" request to "${CTX:apiContext9}/v1.0/api/v1"
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/api/v2"


  Scenario: API-level policy rewrites the method
    Given I generate a unique value from "request-rewrite-10" and store it as "apiName10"
    And I generate a unique API context from "/request-rewrite-10" and store it as "apiContext10"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName10}                   |
      | spec.displayName       | Request Transformation API Level Method |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext10}/$version       |
      | spec.upstream.main.url | http://testbench:3002/anything     |
      | spec.policies          | [{"name":"request-rewrite","version":"v1","params":{"methodRewrite":"POST"}}] |
      | spec.operations        | [{"method":"GET","path":"/test/*"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext10}/v1.0/test/hello" until status 200
    And I set header "Content-Type" to "application/json"
    When I send a "GET" request to "${CTX:apiContext10}/v1.0/test/hello"
    Then the response status code should be 200
    And the JSON response field "method" should be "POST"


  Scenario: Match conditions gate transformations
    Given I generate a unique value from "request-rewrite-11" and store it as "apiName11"
    And I generate a unique API context from "/request-rewrite-11" and store it as "apiContext11"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName11}                   |
      | spec.displayName       | Request Transformation Match      |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext11}/$version       |
      | spec.upstream.main.url | http://testbench:3002/anything     |
      | spec.operations        | [{"method":"GET","path":"/api/v1","policies":[{"name":"request-rewrite","version":"v1","params":{"match":{"headers":[{"name":"x-client-id","type":"Exact","value":"client-123"}]},"pathRewrite":{"type":"ReplacePrefixMatch","replacePrefixMatch":"/api/v2"}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext11}/v1.0/api/v1" until status 200
    And I set header "Content-Type" to "application/json"
    When I send a "GET" request to "${CTX:apiContext11}/v1.0/api/v1"
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/api/v1"
    And I set header "x-client-id" to "client-123"
    When I send a "GET" request to "${CTX:apiContext11}/v1.0/api/v1"
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/api/v2"
