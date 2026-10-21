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

@platform-api-resource-search
Feature: Gateway and project lists can be searched and paged
  As an API platform administrator
  I want to find gateways and projects by display name or handle and page through them
  So that a long list stays navigable on every supported database

  # Every searched name carries the runner's generated word, so a query matches only this
  # scenario's resources even while other runners create their own.
  Background:
    Given I generate a unique value from "srch" and store it as "word"

  Scenario: A gateway search matches display-name words in any case, and handles
    Given I generate a unique resource name from "pay-gw" and store it as "paymentGateway"
    And I generate a unique resource name from "bill-gw" and store it as "billingGateway"
    When I send a "POST" request to the control plane at "/gateways" with body:
      """
      {"id": "${CTX:paymentGateway}", "displayName": "${CTX:word} Payment API Gateway", "endpoints": ["http://payment.example.com"], "functionalityType": "regular"}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/gateways" with body:
      """
      {"id": "${CTX:billingGateway}", "displayName": "${CTX:word} Billing Gateway", "endpoints": ["http://billing.example.com"], "functionalityType": "regular"}
      """
    Then the response status code should be 201
    When I send a "GET" request to the control plane at "/gateways?query=${CTX:word}%20Payment%20API"
    Then the response status code should be 200
    And the JSON response array field "list" should have 1 item
    And the JSON response field "pagination.total" should be 1
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:paymentGateway}"
    When I send a "GET" request to the control plane at "/gateways?query=${CTX:word}%20payment%20api"
    Then the JSON response array field "list" should have 1 item
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:paymentGateway}"
    When I send a "GET" request to the control plane at "/gateways?query=${CTX:word}%20PAYMENT%20API"
    Then the JSON response array field "list" should have 1 item
    When I send a "GET" request to the control plane at "/gateways?query=${CTX:billingGateway}"
    Then the JSON response array field "list" should have 1 item
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:billingGateway}"
    When I send a "GET" request to the control plane at "/gateways?query=${CTX:word}%20Gateway&limit=100"
    Then the JSON response array field "list" should have 0 items
    When I send a "GET" request to the control plane at "/gateways?query=${CTX:word}&limit=100"
    Then the JSON response array field "list" should have 2 items
    And the JSON response field "pagination.total" should be 2

  Scenario Outline: A gateway search treats SQL wildcard characters literally
    Given I generate a unique resource name from "wild-gw" and store it as "gateway"
    When I send a "POST" request to the control plane at "/gateways" with body:
      """
      {"id": "${CTX:gateway}", "displayName": "${CTX:word} Payment Gateway", "endpoints": ["http://wild.example.com"], "functionalityType": "regular"}
      """
    Then the response status code should be 201
    When I send a "GET" request to the control plane at "/gateways?query=<query>"
    Then the response status code should be 200
    And the JSON response array field "list" should have 0 items
    And the JSON response field "pagination.total" should be 0

    Examples:
      | query                    |
      | ${CTX:word}%25           |
      | ${CTX:word}_Payment      |
      | ${CTX:word}%25Gateway    |

  Scenario: A project search matches display-name words and handles
    Given I generate a unique resource name from "pay-proj" and store it as "paymentProject"
    And I generate a unique resource name from "bill-proj" and store it as "billingProject"
    When I send a "POST" request to the control plane at "/projects" with body:
      """
      {"id": "${CTX:paymentProject}", "displayName": "${CTX:word} Payment API Project"}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/projects" with body:
      """
      {"id": "${CTX:billingProject}", "displayName": "${CTX:word} Billing Project"}
      """
    Then the response status code should be 201
    When I send a "GET" request to the control plane at "/projects?query=${CTX:word}%20payment%20API"
    Then the response status code should be 200
    And the JSON response array field "list" should have 1 item
    And the JSON response field "pagination.total" should be 1
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:paymentProject}"
    When I send a "GET" request to the control plane at "/projects?query=${CTX:billingProject}"
    Then the JSON response array field "list" should have 1 item
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:billingProject}"
    When I send a "GET" request to the control plane at "/projects?query=${CTX:word}%25"
    Then the JSON response array field "list" should have 0 items

  Scenario: Project pages split a filtered list without gaps or overlap
    Given I generate a unique resource name from "page-a" and store it as "projectA"
    And I generate a unique resource name from "page-b" and store it as "projectB"
    And I generate a unique resource name from "page-c" and store it as "projectC"
    When I send a "POST" request to the control plane at "/projects" with body:
      """
      {"id": "${CTX:projectA}", "displayName": "${CTX:word} Page A"}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/projects" with body:
      """
      {"id": "${CTX:projectB}", "displayName": "${CTX:word} Page B"}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/projects" with body:
      """
      {"id": "${CTX:projectC}", "displayName": "${CTX:word} Page C"}
      """
    Then the response status code should be 201
    When I send a "GET" request to the control plane at "/projects?query=${CTX:word}&limit=2&offset=0"
    Then the response status code should be 200
    And the JSON response array field "list" should have 2 items
    And the JSON response field "pagination.total" should be 3
    And the JSON response field "pagination.limit" should be 2
    And the JSON response field "pagination.offset" should be 0
    And I store the JSON response field "list[0].id" as "firstOnPageOne"
    And I store the JSON response field "list[1].id" as "secondOnPageOne"
    When I send a "GET" request to the control plane at "/projects?query=${CTX:word}&limit=2&offset=2"
    Then the response status code should be 200
    And the JSON response array field "list" should have 1 item
    And the JSON response field "pagination.total" should be 3
    And the JSON response field "pagination.offset" should be 2
    And the JSON response array "list" should not contain an item with "id" equal to "${CTX:firstOnPageOne}"
    And the JSON response array "list" should not contain an item with "id" equal to "${CTX:secondOnPageOne}"
    When I send a "GET" request to the control plane at "/projects?query=${CTX:word}&limit=2&offset=4"
    Then the JSON response array field "list" should have 0 items
    And the JSON response field "pagination.total" should be 3
