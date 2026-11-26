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

# The block's overlay sets server.max_request_body_bytes to 64 KiB. The padding goes in a field
# the create request does not define, so the body's size is the only thing that differs.
@CPLIM
Feature: Platform API request body cap
  As an operator running a shared control plane
  I want every request body capped at a configured size
  So that one caller cannot make the server buffer an unbounded body

  Background:
    Given I generate a unique resource name from "body-limit-proj" and store it as "projectId"
    And I create a project "${CTX:projectId}" on the control plane

  @CPLIM-001 @type:negative
  Scenario: A REST API create over the request body cap is refused, and one under it is created
    Given I generate a unique resource name from "body-limit-ok" and store it as "underName"
    And I generate a unique API context from "/body-limit-ok" and store it as "underContext"
    And I generate a unique resource name from "body-limit-big" and store it as "overName"
    And I generate a unique API context from "/body-limit-big" and store it as "overContext"
    And I generate a string of 32768 characters and store it as "underPadding"
    And I generate a string of 131072 characters and store it as "overPadding"
    When I send a "POST" request to the control plane at "/rest-apis" with body:
      """
      {"displayName": "${CTX:underName}", "context": "${CTX:underContext}", "version": "v1", "projectId": "${CTX:projectId}", "upstream": {"main": {"url": "http://backend.example.com"}}, "padding": "${CTX:underPadding}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "id" as "underHandle"
    When I send a "POST" request to the control plane at "/rest-apis" with body:
      """
      {"displayName": "${CTX:overName}", "context": "${CTX:overContext}", "version": "v1", "projectId": "${CTX:projectId}", "upstream": {"main": {"url": "http://backend.example.com"}}, "padding": "${CTX:overPadding}"}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"
    When I send a "GET" request to the control plane at "/rest-apis?projectId=${CTX:projectId}"
    Then the response status code should be 200
    And the JSON response array field "list" should have 1 item
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:underHandle}"
