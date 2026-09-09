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

@pii-masking-regex
Feature: PII masking regex policy
  As an API developer
  I want to mask or redact PII in requests and responses
  So that I can protect sensitive user data and comply with privacy regulations

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I resolve the "capture" service URL at "" and store it as "captureUpstream"

  Scenario: Mask email addresses in request and restore in response
    Given I generate a unique value from "pii-mask-email" and store it as "apiName"
    And I generate a unique API version from "pii-mask-email" and store it as "apiVersion"
    And I generate a unique API context from "/pii-mask-email" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | ${CTX:captureUpstream}           |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/echo","policies":[{"name":"pii-masking-regex","version":"v1","params":{"customPIIEntities":[{"piiEntity":"EMAIL","piiRegex":"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\\\.[a-zA-Z]{2,}"}],"jsonPath":"","redactPII":false}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/echo" with body:
      """
      Contact me at john.doe@example.com for more info
      """
    Then the response status code should be 200
    And the response body should contain "john.doe@example.com"
    And the response body should not contain "[EMAIL_"

    When I send a "GET" request to the "capture" service at "/test/captured?path=/echo"
    Then the response body should contain "[EMAIL_"
    And the response body should not contain "john.doe@example.com"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Mask phone numbers in request
    Given I generate a unique value from "pii-mask-phone" and store it as "apiName"
    And I generate a unique API version from "pii-mask-phone" and store it as "apiVersion"
    And I generate a unique API context from "/pii-mask-phone" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | ${CTX:captureUpstream}           |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/echo","policies":[{"name":"pii-masking-regex","version":"v1","params":{"customPIIEntities":[{"piiEntity":"PHONE","piiRegex":"\\\\b\\\\d{3}-\\\\d{3}-\\\\d{4}\\\\b"}],"jsonPath":"","redactPII":false}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/echo" with body:
      """
      Call me at 555-123-4567
      """
    Then the response status code should be 200
    And the response body should contain "555-123-4567"
    And the response body should not contain "[PHONE_"

    When I send a "GET" request to the "capture" service at "/test/captured?path=/echo"
    Then the response body should contain "[PHONE_"
    And the response body should not contain "555-123-4567"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Mask multiple PII entities in a single request
    Given I generate a unique value from "pii-mask-multi" and store it as "apiName"
    And I generate a unique API version from "pii-mask-multi" and store it as "apiVersion"
    And I generate a unique API context from "/pii-mask-multi" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | ${CTX:captureUpstream}           |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/echo","policies":[{"name":"pii-masking-regex","version":"v1","params":{"customPIIEntities":[{"piiEntity":"EMAIL","piiRegex":"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\\\.[a-zA-Z]{2,}"},{"piiEntity":"PHONE","piiRegex":"\\\\b\\\\d{3}-\\\\d{3}-\\\\d{4}\\\\b"},{"piiEntity":"SSN","piiRegex":"\\\\b\\\\d{3}-\\\\d{2}-\\\\d{4}\\\\b"}],"jsonPath":"","redactPII":false}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/echo" with body:
      """
      Reach john@example.com, call 555-123-4567, SSN 123-45-6789
      """
    Then the response status code should be 200
    And the response body should contain "john@example.com"
    And the response body should contain "555-123-4567"
    And the response body should contain "123-45-6789"
    And the response body should not contain "[EMAIL_"
    And the response body should not contain "[PHONE_"
    And the response body should not contain "[SSN_"

    When I send a "GET" request to the "capture" service at "/test/captured?path=/echo"
    Then the response body should contain "[EMAIL_"
    And the response body should contain "[PHONE_"
    And the response body should contain "[SSN_"
    And the response body should not contain "john@example.com"
    And the response body should not contain "555-123-4567"
    And the response body should not contain "123-45-6789"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Redact email addresses permanently
    Given I generate a unique value from "pii-redact-email" and store it as "apiName"
    And I generate a unique API version from "pii-redact-email" and store it as "apiVersion"
    And I generate a unique API context from "/pii-redact-email" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | ${CTX:captureUpstream}           |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/echo","policies":[{"name":"pii-masking-regex","version":"v1","params":{"customPIIEntities":[{"piiEntity":"EMAIL","piiRegex":"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\\\.[a-zA-Z]{2,}"}],"jsonPath":"","redactPII":true}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/echo" with body:
      """
      Email me at admin@company.com
      """
    Then the response status code should be 200
    And the response body should not contain "admin@company.com"
    And the response body should contain "*****"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Redact SSN permanently
    Given I generate a unique value from "pii-redact-ssn" and store it as "apiName"
    And I generate a unique API version from "pii-redact-ssn" and store it as "apiVersion"
    And I generate a unique API context from "/pii-redact-ssn" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | ${CTX:captureUpstream}           |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/echo","policies":[{"name":"pii-masking-regex","version":"v1","params":{"customPIIEntities":[{"piiEntity":"SSN","piiRegex":"\\\\b\\\\d{3}-\\\\d{2}-\\\\d{4}\\\\b"}],"jsonPath":"","redactPII":true}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/echo" with body:
      """
      My SSN is 987-65-4321
      """
    Then the response status code should be 200
    And the response body should not contain "987-65-4321"
    And the response body should contain "*****"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Redact multiple PII types
    Given I generate a unique value from "pii-redact-multi" and store it as "apiName"
    And I generate a unique API version from "pii-redact-multi" and store it as "apiVersion"
    And I generate a unique API context from "/pii-redact-multi" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | ${CTX:captureUpstream}           |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/echo","policies":[{"name":"pii-masking-regex","version":"v1","params":{"customPIIEntities":[{"piiEntity":"EMAIL","piiRegex":"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\\\.[a-zA-Z]{2,}"},{"piiEntity":"CREDIT_CARD","piiRegex":"\\\\b\\\\d{4}[\\\\s-]?\\\\d{4}[\\\\s-]?\\\\d{4}[\\\\s-]?\\\\d{4}\\\\b"}],"jsonPath":"","redactPII":true}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/echo" with body:
      """
      Send receipt to john@test.com. Card: 1234-5678-9012-3456
      """
    Then the response status code should be 200
    And the response body should not contain "john@test.com"
    And the response body should not contain "1234-5678-9012-3456"
    And the response body should contain "*****"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Mask PII in a specific JSON field only
    Given I generate a unique value from "pii-mask-jsonpath" and store it as "apiName"
    And I generate a unique API version from "pii-mask-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/pii-mask-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | ${CTX:captureUpstream}           |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/echo","policies":[{"name":"pii-masking-regex","version":"v1","params":{"customPIIEntities":[{"piiEntity":"EMAIL","piiRegex":"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\\\.[a-zA-Z]{2,}"}],"jsonPath":"$.message","redactPII":false}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/echo" with body:
      """
      {
        "message": "Contact admin@example.com",
        "metadata": "This also has email@test.com but should not be masked"
      }
      """
    Then the response status code should be 200
    And the response body should contain "admin@example.com"
    And the response body should contain "email@test.com"
    And the response body should not contain "[EMAIL_"

    When I send a "GET" request to the "capture" service at "/test/captured?path=/echo"
    Then the response body should contain "[EMAIL_"
    And the response body should not contain "admin@example.com"
    And the response body should contain "email@test.com"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Mask PII in a nested JSON field
    Given I generate a unique value from "pii-mask-nested" and store it as "apiName"
    And I generate a unique API version from "pii-mask-nested" and store it as "apiVersion"
    And I generate a unique API context from "/pii-mask-nested" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | ${CTX:captureUpstream}           |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/echo","policies":[{"name":"pii-masking-regex","version":"v1","params":{"customPIIEntities":[{"piiEntity":"PHONE","piiRegex":"\\\\b\\\\d{3}-\\\\d{3}-\\\\d{4}\\\\b"}],"jsonPath":"$.user.contact","redactPII":false}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/echo" with body:
      """
      {
        "user": {
          "name": "John",
          "contact": "Call me at 555-999-8888"
        }
      }
      """
    Then the response status code should be 200
    And the response body should contain "555-999-8888"
    And the response body should not contain "[PHONE_"

    When I send a "GET" request to the "capture" service at "/test/captured?path=/echo"
    Then the response body should contain "[PHONE_"
    And the response body should not contain "555-999-8888"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Content without PII passes through unchanged
    Given I generate a unique value from "pii-no-pii" and store it as "apiName"
    And I generate a unique API version from "pii-no-pii" and store it as "apiVersion"
    And I generate a unique API context from "/pii-no-pii" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | ${CTX:captureUpstream}           |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/echo","policies":[{"name":"pii-masking-regex","version":"v1","params":{"customPIIEntities":[{"piiEntity":"EMAIL","piiRegex":"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\\\.[a-zA-Z]{2,}"}],"jsonPath":"","redactPII":false}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/echo" with body:
      """
      This is a clean message with no PII
      """
    Then the response status code should be 200
    And the response body should contain "This is a clean message with no PII"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Empty request body passes through
    Given I generate a unique value from "pii-empty-body" and store it as "apiName"
    And I generate a unique API version from "pii-empty-body" and store it as "apiVersion"
    And I generate a unique API context from "/pii-empty-body" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | ${CTX:captureUpstream}           |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/echo","policies":[{"name":"pii-masking-regex","version":"v1","params":{"customPIIEntities":[{"piiEntity":"EMAIL","piiRegex":"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\\\.[a-zA-Z]{2,}"}],"jsonPath":"","redactPII":false}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/echo" with body:
      """
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A JSONPath referencing a missing field is rejected
    Given I generate a unique value from "pii-invalid-jsonpath" and store it as "apiName"
    And I generate a unique API version from "pii-invalid-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/pii-invalid-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | ${CTX:captureUpstream}           |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/echo","policies":[{"name":"pii-masking-regex","version":"v1","params":{"customPIIEntities":[{"piiEntity":"EMAIL","piiRegex":"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\\\.[a-zA-Z]{2,}"}],"jsonPath":"$.nonexistent.field","redactPII":false}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/echo" with body:
      """
      {
        "message": "test@example.com"
      }
      """
    Then the response status code should be 500

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Multiple emails in a single message are all masked
    Given I generate a unique value from "pii-mask-multi-emails" and store it as "apiName"
    And I generate a unique API version from "pii-mask-multi-emails" and store it as "apiVersion"
    And I generate a unique API context from "/pii-mask-multi-emails" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | ${CTX:captureUpstream}           |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/echo","policies":[{"name":"pii-masking-regex","version":"v1","params":{"customPIIEntities":[{"piiEntity":"EMAIL","piiRegex":"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\\\.[a-zA-Z]{2,}"}],"jsonPath":"","redactPII":false}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/echo" with body:
      """
      Recipients john@example.com, jane@test.org, admin@company.net
      """
    Then the response status code should be 200
    And the response body should contain "john@example.com"
    And the response body should contain "jane@test.org"
    And the response body should contain "admin@company.net"
    And the response body should not contain "[EMAIL_"

    When I send a "GET" request to the "capture" service at "/test/captured?path=/echo"
    Then the response body should contain "[EMAIL_"
    And the response body should not contain "john@example.com"
    And the response body should not contain "jane@test.org"
    And the response body should not contain "admin@company.net"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Redact credit card numbers permanently
    Given I generate a unique value from "pii-redact-cc" and store it as "apiName"
    And I generate a unique API version from "pii-redact-cc" and store it as "apiVersion"
    And I generate a unique API context from "/pii-redact-cc" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | ${CTX:captureUpstream}           |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/echo","policies":[{"name":"pii-masking-regex","version":"v1","params":{"customPIIEntities":[{"piiEntity":"CREDIT_CARD","piiRegex":"\\\\b\\\\d{4}[\\\\s-]?\\\\d{4}[\\\\s-]?\\\\d{4}[\\\\s-]?\\\\d{4}\\\\b"}],"jsonPath":"","redactPII":true}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/echo" with body:
      """
      Payment with card 4532-1234-5678-9012
      """
    Then the response status code should be 200
    And the response body should not contain "4532-1234-5678-9012"
    And the response body should contain "*****"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Comprehensive PII protection redacts every configured entity
    Given I generate a unique value from "pii-comprehensive" and store it as "apiName"
    And I generate a unique API version from "pii-comprehensive" and store it as "apiVersion"
    And I generate a unique API context from "/pii-comprehensive" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | ${CTX:captureUpstream}           |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/submit","policies":[{"name":"pii-masking-regex","version":"v1","params":{"customPIIEntities":[{"piiEntity":"EMAIL","piiRegex":"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\\\.[a-zA-Z]{2,}"},{"piiEntity":"PHONE","piiRegex":"\\\\b\\\\d{3}-\\\\d{3}-\\\\d{4}\\\\b"},{"piiEntity":"SSN","piiRegex":"\\\\b\\\\d{3}-\\\\d{2}-\\\\d{4}\\\\b"},{"piiEntity":"CREDIT_CARD","piiRegex":"\\\\b\\\\d{4}[\\\\s-]?\\\\d{4}[\\\\s-]?\\\\d{4}[\\\\s-]?\\\\d{4}\\\\b"}],"jsonPath":"","redactPII":true}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/submit" with body:
      """
      Reach john@example.com, call 555-123-4567, SSN 123-45-6789, card 4532 1234 5678 9012
      """
    Then the response status code should be 200
    And the response body should not contain "john@example.com"
    And the response body should not contain "555-123-4567"
    And the response body should not contain "123-45-6789"
    And the response body should not contain "4532 1234 5678 9012"
    And the response body should contain "*****"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
