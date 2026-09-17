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

@json-schema-guardrail
Feature: JSON schema guardrail policy
  As an API developer
  I want to validate request and response payloads against a JSON Schema
  So that I can enforce structural and type contracts on API traffic

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Valid request passes schema validation
    Given I generate a unique value from "jsg-valid-request" and store it as "apiName"
    And I generate a unique API version from "jsg-valid-request" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-valid-request" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"name\":{\"type\":\"string\"},\"age\":{\"type\":\"integer\"}},\"required\":[\"name\",\"age\"]}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"name": "John Doe", "age": 30}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Invalid request fails schema validation
    Given I generate a unique value from "jsg-invalid-request" and store it as "apiName"
    And I generate a unique API version from "jsg-invalid-request" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-invalid-request" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"name\":{\"type\":\"string\"},\"age\":{\"type\":\"integer\"}},\"required\":[\"name\",\"age\"]}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"name": "John Doe"}
      """
    Then the response status code should be 422
    And the response body should contain "JSON_SCHEMA_GUARDRAIL"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Missing required field fails validation
    Given I generate a unique value from "jsg-missing-field" and store it as "apiName"
    And I generate a unique API version from "jsg-missing-field" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-missing-field" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"username\":{\"type\":\"string\"},\"email\":{\"type\":\"string\"}},\"required\":[\"username\",\"email\"]}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"username": "johndoe"}
      """
    Then the response status code should be 422
    And the response body should contain "GUARDRAIL_INTERVENED"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Wrong type fails validation
    Given I generate a unique value from "jsg-wrong-type" and store it as "apiName"
    And I generate a unique API version from "jsg-wrong-type" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-wrong-type" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"name\":{\"type\":\"string\"},\"age\":{\"type\":\"integer\"}},\"required\":[\"name\",\"age\"]}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"name": "John", "age": "thirty"}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Valid response passes schema validation
    Given I generate a unique value from "jsg-valid-response" and store it as "apiName"
    And I generate a unique API version from "jsg-valid-response" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-valid-response" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/echo","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"response":{"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"method\":{\"type\":\"string\"},\"path\":{\"type\":\"string\"}},\"required\":[\"method\",\"path\"]}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/echo"
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Both request and response are validated against their own schemas
    Given I generate a unique value from "jsg-both-validation" and store it as "apiName"
    And I generate a unique API version from "jsg-both-validation" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-both-validation" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"input\":{\"type\":\"string\"}},\"required\":[\"input\"]}"},"response":{"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"method\":{\"type\":\"string\"}},\"required\":[\"method\"]}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"input": "test data"}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Validate specific field with JSONPath
    Given I generate a unique value from "jsg-jsonpath" and store it as "apiName"
    And I generate a unique API version from "jsg-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"$.user","schema":"{\"type\":\"object\",\"properties\":{\"name\":{\"type\":\"string\"},\"age\":{\"type\":\"integer\",\"minimum\":18}},\"required\":[\"name\",\"age\"]}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"user": {"name": "Alice", "age": 25}, "metadata": "ignored"}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: JSONPath extraction with invalid data
    Given I generate a unique value from "jsg-jsonpath-invalid" and store it as "apiName"
    And I generate a unique API version from "jsg-jsonpath-invalid" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-jsonpath-invalid" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"$.user","schema":"{\"type\":\"object\",\"properties\":{\"age\":{\"type\":\"integer\",\"minimum\":18}},\"required\":[\"age\"]}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"user": {"age": 15}, "other": "data"}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Validate nested object with JSONPath
    Given I generate a unique value from "jsg-nested-jsonpath" and store it as "apiName"
    And I generate a unique API version from "jsg-nested-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-nested-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"$.order.shippingAddress","schema":"{\"type\":\"object\",\"properties\":{\"street\":{\"type\":\"string\"},\"city\":{\"type\":\"string\"},\"zipCode\":{\"type\":\"string\"}},\"required\":[\"street\",\"city\",\"zipCode\"]}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"order": {"shippingAddress": {"street": "123 Main St", "city": "Boston", "zipCode": "02101"}}}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Invert logic passes when schema validation fails
    Given I generate a unique value from "jsg-invert-pass" and store it as "apiName"
    And I generate a unique API version from "jsg-invert-pass" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-invert-pass" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"dangerousCommand\":{\"type\":\"string\",\"pattern\":\"^(rm\|delete\|drop).*\"}},\"required\":[\"dangerousCommand\"]}","invert":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"safeCommand": "list files"}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Invert logic blocks when schema validation succeeds
    Given I generate a unique value from "jsg-invert-block" and store it as "apiName"
    And I generate a unique API version from "jsg-invert-block" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-invert-block" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"command\":{\"type\":\"string\",\"pattern\":\"^(rm\|delete\|drop).*\"}},\"required\":[\"command\"]}","invert":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"command": "rm -rf /"}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Inverted schema targeting a malicious pattern passes safe content
    Given I generate a unique value from "jsg-block-malicious" and store it as "apiName"
    And I generate a unique API version from "jsg-block-malicious" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-block-malicious" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"query\":{\"type\":\"string\",\"pattern\":\".*DROP TABLE.*\"}}}","invert":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"query": "SELECT * FROM users"}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Blocked response includes the assessment detail when showAssessment is enabled
    Given I generate a unique value from "jsg-show-assessment" and store it as "apiName"
    And I generate a unique API version from "jsg-show-assessment" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-show-assessment" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"name\":{\"type\":\"string\",\"minLength\":3},\"age\":{\"type\":\"integer\",\"minimum\":18}},\"required\":[\"name\",\"age\"]}","showAssessment":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"name": "Jo", "age": 15}
      """
    Then the response status code should be 422
    And the response body should contain "assessments"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Hide assessment details when showAssessment is false
    Given I generate a unique value from "jsg-hide-assessment" and store it as "apiName"
    And I generate a unique API version from "jsg-hide-assessment" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-hide-assessment" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"name\":{\"type\":\"string\"}},\"required\":[\"name\"]}","showAssessment":false}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"age": 25}
      """
    Then the response status code should be 422
    And the response body should contain "GUARDRAIL_INTERVENED"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Validate string length constraints
    Given I generate a unique value from "jsg-string-length" and store it as "apiName"
    And I generate a unique API version from "jsg-string-length" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-string-length" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"username\":{\"type\":\"string\",\"minLength\":3,\"maxLength\":20}},\"required\":[\"username\"]}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"username": "ab"}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Validate numeric range constraints
    Given I generate a unique value from "jsg-numeric-range" and store it as "apiName"
    And I generate a unique API version from "jsg-numeric-range" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-numeric-range" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"price\":{\"type\":\"number\",\"minimum\":0,\"maximum\":10000}},\"required\":[\"price\"]}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"price": -5}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Validate array constraints
    Given I generate a unique value from "jsg-array-constraints" and store it as "apiName"
    And I generate a unique API version from "jsg-array-constraints" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-array-constraints" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"tags\":{\"type\":\"array\",\"items\":{\"type\":\"string\"},\"minItems\":1,\"maxItems\":5}},\"required\":[\"tags\"]}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"tags": []}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Validate enum constraints
    Given I generate a unique value from "jsg-enum-constraints" and store it as "apiName"
    And I generate a unique API version from "jsg-enum-constraints" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-enum-constraints" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"status\":{\"type\":\"string\",\"enum\":[\"pending\",\"processing\",\"completed\",\"cancelled\"]}},\"required\":[\"status\"]}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"status": "invalid-status"}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Validate pattern constraints
    Given I generate a unique value from "jsg-pattern-constraints" and store it as "apiName"
    And I generate a unique API version from "jsg-pattern-constraints" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-pattern-constraints" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"email\":{\"type\":\"string\",\"pattern\":\"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\\\\\\\.[a-zA-Z]{2,}$\"}},\"required\":[\"email\"]}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"email": "invalid-email"}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Validate nested object schema
    Given I generate a unique value from "jsg-nested-object" and store it as "apiName"
    And I generate a unique API version from "jsg-nested-object" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-nested-object" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"name\":{\"type\":\"string\"},\"address\":{\"type\":\"object\",\"properties\":{\"street\":{\"type\":\"string\"},\"city\":{\"type\":\"string\"}},\"required\":[\"street\",\"city\"]}},\"required\":[\"name\",\"address\"]}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"name": "John", "address": {"street": "Main St"}}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Validate array of objects
    Given I generate a unique value from "jsg-array-objects" and store it as "apiName"
    And I generate a unique API version from "jsg-array-objects" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-array-objects" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"items\":{\"type\":\"array\",\"items\":{\"type\":\"object\",\"properties\":{\"productId\":{\"type\":\"string\"},\"quantity\":{\"type\":\"integer\",\"minimum\":1}},\"required\":[\"productId\",\"quantity\"]}}},\"required\":[\"items\"]}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"items": [{"productId": "123", "quantity": 2}, {"productId": "456", "quantity": 0}]}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Handle empty request body
    Given I generate a unique value from "jsg-empty-body" and store it as "apiName"
    And I generate a unique API version from "jsg-empty-body" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-empty-body" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"data\":{\"type\":\"string\"}},\"required\":[\"data\"]}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Handle invalid JSON
    Given I generate a unique value from "jsg-invalid-json" and store it as "apiName"
    And I generate a unique API version from "jsg-invalid-json" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-invalid-json" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\"}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      not valid json {
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Handle invalid JSONPath
    Given I generate a unique value from "jsg-invalid-jsonpath" and store it as "apiName"
    And I generate a unique API version from "jsg-invalid-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-invalid-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"$.nonexistent.field","schema":"{\"type\":\"object\"}"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"data": "test"}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: User registration with comprehensive validation
    Given I generate a unique value from "jsg-registration" and store it as "apiName"
    And I generate a unique API version from "jsg-registration" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-registration" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"username\":{\"type\":\"string\",\"minLength\":3,\"maxLength\":20,\"pattern\":\"^[a-zA-Z0-9_]+$\"},\"email\":{\"type\":\"string\",\"pattern\":\"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\\\\\\\.[a-zA-Z]{2,}$\"},\"password\":{\"type\":\"string\",\"minLength\":8},\"age\":{\"type\":\"integer\",\"minimum\":13},\"termsAccepted\":{\"type\":\"boolean\",\"enum\":[true]}},\"required\":[\"username\",\"email\",\"password\",\"age\",\"termsAccepted\"]}","showAssessment":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"username": "john_doe", "email": "john@example.com", "password": "SecurePass123", "age": 25, "termsAccepted": true}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Block SQL injection patterns
    Given I generate a unique value from "jsg-sql-injection" and store it as "apiName"
    And I generate a unique API version from "jsg-sql-injection" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-sql-injection" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"query\":{\"type\":\"string\",\"pattern\":\".*((DROP\|DELETE\|INSERT\|UPDATE\|SELECT).*(TABLE\|FROM\|WHERE)).*\"}}}","invert":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"query": "normal search term"}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: E-commerce order validation
    Given I generate a unique value from "jsg-ecommerce-order" and store it as "apiName"
    And I generate a unique API version from "jsg-ecommerce-order" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-ecommerce-order" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"customerId\":{\"type\":\"string\",\"minLength\":1},\"items\":{\"type\":\"array\",\"items\":{\"type\":\"object\",\"properties\":{\"productId\":{\"type\":\"string\"},\"quantity\":{\"type\":\"integer\",\"minimum\":1},\"price\":{\"type\":\"number\",\"minimum\":0}},\"required\":[\"productId\",\"quantity\",\"price\"]},\"minItems\":1},\"shippingAddress\":{\"type\":\"object\",\"properties\":{\"street\":{\"type\":\"string\"},\"city\":{\"type\":\"string\"},\"zipCode\":{\"type\":\"string\",\"pattern\":\"^[0-9]{5}$\"}},\"required\":[\"street\",\"city\",\"zipCode\"]},\"paymentMethod\":{\"type\":\"string\",\"enum\":[\"credit_card\",\"paypal\",\"bank_transfer\"]}},\"required\":[\"customerId\",\"items\",\"shippingAddress\",\"paymentMethod\"]}","showAssessment":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"customerId": "C123", "items": [{"productId": "P456", "quantity": 2, "price": 29.99}], "shippingAddress": {"street": "123 Main St", "city": "Boston", "zipCode": "02101"}, "paymentMethod": "credit_card"}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: API response contract enforcement
    Given I generate a unique value from "jsg-response-contract" and store it as "apiName"
    And I generate a unique API version from "jsg-response-contract" and store it as "apiVersion"
    And I generate a unique API context from "/jsg-response-contract" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/echo","policies":[{"name":"json-schema-guardrail","version":"v1","params":{"response":{"jsonPath":"","schema":"{\"type\":\"object\",\"properties\":{\"method\":{\"type\":\"string\"},\"path\":{\"type\":\"string\"},\"headers\":{\"type\":\"object\"}},\"required\":[\"method\",\"path\",\"headers\"]}","showAssessment":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/echo"
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
