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

@aws-bedrock-guardrail
Feature: AWS Bedrock guardrail policy
  As an API developer
  I want to validate request and response content using AWS Bedrock Guardrails
  So that I can prevent harmful content and protect PII
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Request with safe content passes through
    Given I generate a unique value from "bedrock-safe-request" and store it as "apiName"
    And I generate a unique API version from "bedrock-safe-request" and store it as "apiVersion"
    And I generate a unique API context from "/bedrock-safe-request" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"aws-bedrock-guardrail","version":"v1","params":{"region":"us-east-1","guardrailID":"test-guardrail-id","guardrailVersion":"DRAFT","awsAuth":{"authenticationType":"iam-user-access-key","awsAccessKeyID":"AKIAIOSFODNN7TEST","awsSecretAccessKey":"testsecretaccesskeytestsecretaccesskey1"},"request":{"jsonPath":"","redactPII":false,"passthroughOnError":false,"showAssessment":false}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" until status 200 with body:
      """
      {"message":"Hello, this is safe content"}
      """

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Request with violating content is blocked
    Given I generate a unique value from "bedrock-block-request" and store it as "apiName"
    And I generate a unique API version from "bedrock-block-request" and store it as "apiVersion"
    And I generate a unique API context from "/bedrock-block-request" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"aws-bedrock-guardrail","version":"v1","params":{"region":"us-east-1","guardrailID":"test-guardrail-id","guardrailVersion":"DRAFT","awsAuth":{"authenticationType":"iam-user-access-key","awsAccessKeyID":"AKIAIOSFODNN7TEST","awsSecretAccessKey":"testsecretaccesskeytestsecretaccesskey1"},"request":{"jsonPath":"","showAssessment":false}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message":"This content contains violence and illegal activities"}
      """
    Then the response status code should be 422
    And the response body should contain "AWS_BEDROCK_GUARDRAIL"
    And the response body should contain "GUARDRAIL_INTERVENED"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Request violation with detailed assessment
    Given I generate a unique value from "bedrock-assessment" and store it as "apiName"
    And I generate a unique API version from "bedrock-assessment" and store it as "apiVersion"
    And I generate a unique API context from "/bedrock-assessment" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"aws-bedrock-guardrail","version":"v1","params":{"region":"us-east-1","guardrailID":"test-guardrail-id","guardrailVersion":"DRAFT","awsAuth":{"authenticationType":"iam-user-access-key","awsAccessKeyID":"AKIAIOSFODNN7TEST","awsSecretAccessKey":"testsecretaccesskeytestsecretaccesskey1"},"request":{"showAssessment":true}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message":"This contains hate speech"}
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "assessments"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Response with safe content passes through
    Given I generate a unique value from "bedrock-safe-response" and store it as "apiName"
    And I generate a unique API version from "bedrock-safe-response" and store it as "apiVersion"
    And I generate a unique API context from "/bedrock-safe-response" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/data","policies":[{"name":"aws-bedrock-guardrail","version":"v1","params":{"region":"us-east-1","guardrailID":"test-guardrail-id","guardrailVersion":"DRAFT","awsAuth":{"authenticationType":"iam-user-access-key","awsAccessKeyID":"AKIAIOSFODNN7TEST","awsSecretAccessKey":"testsecretaccesskeytestsecretaccesskey1"},"response":{"jsonPath":"","showAssessment":false}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data" until status 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Request JSONPath extraction validates only the targeted field
    Given I generate a unique value from "bedrock-jsonpath" and store it as "apiName"
    And I generate a unique API version from "bedrock-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/bedrock-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"aws-bedrock-guardrail","version":"v1","params":{"region":"us-east-1","guardrailID":"test-guardrail-id","guardrailVersion":"DRAFT","awsAuth":{"authenticationType":"iam-user-access-key","awsAccessKeyID":"AKIAIOSFODNN7TEST","awsSecretAccessKey":"testsecretaccesskeytestsecretaccesskey1"},"request":{"jsonPath":"$.message"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # Safe message field, violating metadata field ignored since only $.message is checked
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" until status 200 with body:
      """
      {"message":"Safe content","metadata":"This contains violence but should be ignored"}
      """

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message":"This message contains violence","metadata":"Safe metadata"}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: PII is masked in request and restored in response
    Given I generate a unique value from "bedrock-pii-masking" and store it as "apiName"
    And I generate a unique API version from "bedrock-pii-masking" and store it as "apiVersion"
    And I generate a unique API context from "/bedrock-pii-masking" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/anything","policies":[{"name":"aws-bedrock-guardrail","version":"v1","params":{"region":"us-east-1","guardrailID":"test-guardrail-id","guardrailVersion":"DRAFT","awsAuth":{"authenticationType":"iam-user-access-key","awsAccessKeyID":"AKIAIOSFODNN7TEST","awsSecretAccessKey":"testsecretaccesskeytestsecretaccesskey1"},"request":{"jsonPath":"","redactPII":false},"response":{"enabled":true,"jsonPath":""}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # The echo upstream reflects the request body, so PII restoration is verified by seeing
    # the ORIGINAL address in the final client-visible response.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until status 200 with body:
      """
      {"message":"Contact me at mask-test@example.com"}
      """
    Then the response body should contain "mask-test@example.com"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: PII is permanently redacted with redactPII true
    Given I generate a unique value from "bedrock-pii-redaction" and store it as "apiName"
    And I generate a unique API version from "bedrock-pii-redaction" and store it as "apiVersion"
    And I generate a unique API context from "/bedrock-pii-redaction" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/anything","policies":[{"name":"aws-bedrock-guardrail","version":"v1","params":{"region":"us-east-1","guardrailID":"test-guardrail-id","guardrailVersion":"DRAFT","awsAuth":{"authenticationType":"iam-user-access-key","awsAccessKeyID":"AKIAIOSFODNN7TEST","awsSecretAccessKey":"testsecretaccesskeytestsecretaccesskey1"},"request":{"jsonPath":"","redactPII":true}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until status 200 with body:
      """
      {"message":"My SSN is test@example.com"}
      """
    Then the response body should contain "*****"
    And the response body should not contain "test@example.com"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Request and response phases each validate their own content independently
    Given I generate a unique value from "bedrock-both-phases" and store it as "apiName"
    And I generate a unique API version from "bedrock-both-phases" and store it as "apiVersion"
    And I generate a unique API context from "/bedrock-both-phases" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"aws-bedrock-guardrail","version":"v1","params":{"region":"us-east-1","guardrailID":"test-guardrail-id","guardrailVersion":"DRAFT","awsAuth":{"authenticationType":"iam-user-access-key","awsAccessKeyID":"AKIAIOSFODNN7TEST","awsSecretAccessKey":"testsecretaccesskeytestsecretaccesskey1"},"request":{"jsonPath":""},"response":{"jsonPath":""}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" until status 200 with body:
      """
      {"message":"Safe request content"}
      """

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An empty request body is handled gracefully
    Given I generate a unique value from "bedrock-empty-body" and store it as "apiName"
    And I generate a unique API version from "bedrock-empty-body" and store it as "apiVersion"
    And I generate a unique API context from "/bedrock-empty-body" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"aws-bedrock-guardrail","version":"v1","params":{"region":"us-east-1","guardrailID":"test-guardrail-id","guardrailVersion":"DRAFT","awsAuth":{"authenticationType":"iam-user-access-key","awsAccessKeyID":"AKIAIOSFODNN7TEST","awsSecretAccessKey":"testsecretaccesskeytestsecretaccesskey1"},"request":{"jsonPath":""}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" until status 200 with body:
      """
      """

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: passthroughOnError allows the request through despite a guardrail service failure
    Given I generate a unique value from "bedrock-passthrough" and store it as "apiName"
    And I generate a unique API version from "bedrock-passthrough" and store it as "apiVersion"
    And I generate a unique API context from "/bedrock-passthrough" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"aws-bedrock-guardrail","version":"v1","params":{"region":"us-east-1","guardrailID":"test-guardrail-id","guardrailVersion":"DRAFT","awsAuth":{"authenticationType":"iam-user-access-key","awsAccessKeyID":"AKIAIOSFODNN7TEST","awsSecretAccessKey":"testsecretaccesskeytestsecretaccesskey1"},"request":{"jsonPath":"","passthroughOnError":true}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # The mock simulates a guardrail-service 500 for this exact keyword combination
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" until status 200 with body:
      """
      {"message":"This will simulate error in mock service"}
      """

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A nested JSONPath extraction is validated
    Given I generate a unique value from "bedrock-nested-jsonpath" and store it as "apiName"
    And I generate a unique API version from "bedrock-nested-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/bedrock-nested-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"aws-bedrock-guardrail","version":"v1","params":{"region":"us-east-1","guardrailID":"test-guardrail-id","guardrailVersion":"DRAFT","awsAuth":{"authenticationType":"iam-user-access-key","awsAccessKeyID":"AKIAIOSFODNN7TEST","awsSecretAccessKey":"testsecretaccesskeytestsecretaccesskey1"},"request":{"jsonPath":"$.data.content"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # Safe nested content field, violating outer field ignored
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"data":{"content":"Safe nested message","timestamp":"2025-01-01"},"metadata":"This outer field contains violence but should be ignored"}
      """

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"data":{"content":"This nested content contains violence","timestamp":"2025-01-01"}}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An unresolvable JSONPath is handled as a guardrail error
    Given I generate a unique value from "bedrock-invalid-path" and store it as "apiName"
    And I generate a unique API version from "bedrock-invalid-path" and store it as "apiVersion"
    And I generate a unique API context from "/bedrock-invalid-path" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"aws-bedrock-guardrail","version":"v1","params":{"region":"us-east-1","guardrailID":"test-guardrail-id","guardrailVersion":"DRAFT","awsAuth":{"authenticationType":"iam-user-access-key","awsAccessKeyID":"AKIAIOSFODNN7TEST","awsSecretAccessKey":"testsecretaccesskeytestsecretaccesskey1"},"request":{"jsonPath":"$.nonexistent.field"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message":"This field exists but not the one we are looking for"}
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "AWS_BEDROCK_GUARDRAIL"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A blocked response carries the complete error structure
    Given I generate a unique value from "bedrock-error-structure" and store it as "apiName"
    And I generate a unique API version from "bedrock-error-structure" and store it as "apiVersion"
    And I generate a unique API context from "/bedrock-error-structure" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"aws-bedrock-guardrail","version":"v1","params":{"region":"us-east-1","guardrailID":"test-guardrail-id","guardrailVersion":"DRAFT","awsAuth":{"authenticationType":"iam-user-access-key","awsAccessKeyID":"AKIAIOSFODNN7TEST","awsSecretAccessKey":"testsecretaccesskeytestsecretaccesskey1"},"request":{"showAssessment":true}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message":"This contains hate speech"}
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the JSON response field "type" should be "AWS_BEDROCK_GUARDRAIL"
    And the response body should contain "assessments"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
