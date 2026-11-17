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

@APIDEF
Feature: An OpenAPI definition is validated before an API is created from it
  As an API publisher
  I want to check a definition and see what is wrong with it
  So that I can fix it before creating an API

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "apidef-proj" and store it as "project"
    And I generate a unique resource name from "apidef-api" and store it as "api"
    And I generate a unique API context from "/apidef" and store it as "apiContext"
    And I create a project "${CTX:project}" on the control plane

  @APIDEF-010
  Scenario: Validating an invalid OpenAPI file lists its errors and returns its content
    When I send a multipart "POST" request to the control plane at "/rest-apis/validate-openapi" with fields:
      | file | @spec:invalid.yaml |
    Then the response status code should be 200
    And the JSON response field "isValid" should be false
    And the JSON response should have field "errors[0].message"
    And the JSON response should have field "content"
    When I send a multipart "POST" request to the control plane at "/rest-apis/validate-openapi" with fields:
      | file | @spec:petstore-v1.yaml |
    Then the response status code should be 200
    And the JSON response field "isValid" should be true

  @APIDEF-011
  Scenario: Validating a URL that serves no OpenAPI definition lists its errors and returns no content
    Given I store the https URL of spec fixture "not-a-spec.txt" as "badUrl"
    And I store the https URL of spec fixture "petstore-v1.yaml" as "goodUrl"
    When I send a multipart "POST" request to the control plane at "/rest-apis/validate-openapi" with fields:
      | url | ${CTX:badUrl} |
    Then the response status code should be 200
    And the JSON response field "isValid" should be false
    And the JSON response should have field "errors[0].message"
    And the JSON response field "content" should not exist
    When I send a multipart "POST" request to the control plane at "/rest-apis/validate-openapi" with fields:
      | url | ${CTX:goodUrl} |
    Then the response status code should be 200
    And the JSON response field "isValid" should be true
