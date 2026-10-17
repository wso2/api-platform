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

@ap-cli-subscription
Feature: ap CLI subscription commands
  As a CLI user managing subscriptions on a gateway
  I want to address a subscription by my own ID and change its plan, token and status in place
  So that I never have to delete and recreate a subscription to change it

  Every scenario deploys its own plans and API through the management API. The CLI drives the
  subscriptions, and calls through the gateway confirm that each change reaches the data plane.

  Background:
    Given the "ap" CLI is available
    And the CLI uses the platform gateway as "subscription-gw"
    And I authenticate using basic auth as "admin"

    Given I generate a unique value from "cli-sub-gold" and store it as "goldPlanName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:goldPlanName}","throttleLimitCount":100,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "goldPlanId"
    And I register the "subscription-plan" "${CTX:goldPlanId}" for cleanup

    Given I generate a unique value from "cli-sub-silver" and store it as "silverPlanName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:silverPlanName}","throttleLimitCount":1,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "silverPlanId"
    And I register the "subscription-plan" "${CTX:silverPlanId}" for cleanup

    Given I generate a unique value from "cli-sub-api" and store it as "apiName"
    And I generate a unique API context from "/cli-sub-api" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                         |
      | name                   | ${CTX:apiName}                                    |
      | spec.displayName       | ${CTX:apiName}                                    |
      | spec.version           | v1.0                                              |
      | spec.context           | ${CTX:apiContext}/$version                        |
      | spec.subscriptionPlans | ["${CTX:goldPlanName}","${CTX:silverPlanName}"]  |
      | spec.upstream.main.url | http://testbench:3002                             |
      | spec.operations        | [{"method":"GET","path":"/health","policies":[{"name":"subscription-validation","version":"v1","params":{"subscriptionKeyHeader":"Subscription-Key"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 403

    Given I generate a unique value from "cli-sub-ref" and store it as "subRef"
    And I generate a unique value from "cli-sub-token" and store it as "subToken"

  Scenario: Create a subscription under my own ID and read it back
    When I create subscription from "resources/templates/subscription.yaml" through the CLI with values:
      | apiVersion              | ${CTX:gatewaySpecVersion} |
      | name                    | ${CTX:subRef}             |
      | spec.externalRefId      | ${CTX:subRef}             |
      | spec.apiId              | ${CTX:apiName}            |
      | spec.subscriptionPlanId | ${CTX:goldPlanId}         |
      | spec.subscriptionToken  | ${CTX:subToken}           |
    Then the CLI command should succeed
    And the CLI output should contain "created successfully"

    When I run ap with arguments "gateway subscription get --id ${CTX:subRef}"
    Then the CLI command should succeed
    And the CLI output should contain "${CTX:subRef}"
    And the CLI output should contain "${CTX:goldPlanId}"
    And the CLI output should not contain "${CTX:subToken}"

    When I run ap with arguments "gateway subscription list"
    Then the CLI command should succeed
    And the CLI output should contain "${CTX:subRef}"

    When I set header "Subscription-Key" to "${CTX:subToken}"
    Then I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200

  Scenario: Creating a subscription with a non-string externalRefId is rejected before it is sent
    When I create subscription from "resources/templates/subscription.yaml" through the CLI with values:
      | apiVersion         | ${CTX:gatewaySpecVersion} |
      | name               | ${CTX:subRef}             |
      | spec.externalRefId | 12345                     |
      | spec.apiId         | ${CTX:apiName}            |
    Then the CLI command should fail
    And the CLI error output should contain "spec.externalRefId must be a string"

  Scenario: Move a subscription to another plan, then remove its plan
    Given I create subscription from "resources/templates/subscription.yaml" through the CLI with values:
      | apiVersion              | ${CTX:gatewaySpecVersion} |
      | name                    | ${CTX:subRef}             |
      | spec.externalRefId      | ${CTX:subRef}             |
      | spec.apiId              | ${CTX:apiName}            |
      | spec.subscriptionPlanId | ${CTX:goldPlanId}         |
      | spec.subscriptionToken  | ${CTX:subToken}           |
    And the CLI command should succeed
    And I set header "Subscription-Key" to "${CTX:subToken}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200

    # Silver allows one request a minute, so the subscription is soon throttled.
    When I run ap with arguments "gateway subscription update --id ${CTX:subRef} --subscription-plan-id ${CTX:silverPlanId}"
    Then the CLI command should succeed
    And the CLI output should contain "${CTX:silverPlanId}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 429

    # Without a plan the subscription is no longer throttled.
    When I run ap with arguments "gateway subscription update --id ${CTX:subRef} --clear-subscription-plan"
    Then the CLI command should succeed
    And the CLI output should not contain "subscriptionPlanId"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200

  Scenario: Rotate a subscription's token
    Given I create subscription from "resources/templates/subscription.yaml" through the CLI with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:subRef}             |
      | spec.externalRefId     | ${CTX:subRef}             |
      | spec.apiId             | ${CTX:apiName}            |
      | spec.subscriptionToken | ${CTX:subToken}           |
    And the CLI command should succeed
    And I generate a unique value from "cli-sub-new-token" and store it as "newToken"

    When I run ap with arguments "gateway subscription update --id ${CTX:subRef} --subscription-token ${CTX:newToken}"
    Then the CLI command should succeed
    And the CLI output should not contain "${CTX:newToken}"

    When I set header "Subscription-Key" to "${CTX:subToken}"
    Then I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 403
    When I set header "Subscription-Key" to "${CTX:newToken}"
    Then I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200

  Scenario: Revoke a subscription and activate it again
    Given I create subscription from "resources/templates/subscription.yaml" through the CLI with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:subRef}             |
      | spec.externalRefId     | ${CTX:subRef}             |
      | spec.apiId             | ${CTX:apiName}            |
      | spec.subscriptionToken | ${CTX:subToken}           |
    And the CLI command should succeed
    And I set header "Subscription-Key" to "${CTX:subToken}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200

    When I run ap with arguments "gateway subscription update --id ${CTX:subRef} --status REVOKED"
    Then the CLI command should succeed
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 403

    When I run ap with arguments "gateway subscription update --id ${CTX:subRef} --status ACTIVE"
    Then the CLI command should succeed
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200

  # The CLI rejects these before calling the gateway. Without a terminal, --rotate-token cannot
  # prompt for the new token, so it fails rather than reading the token from anywhere else.
  Scenario Outline: An update with conflicting or missing options is rejected before it is sent
    When I run ap with arguments "gateway subscription update --id ${CTX:subRef} <options>"
    Then the CLI command should fail
    And the CLI error output should contain "<message>"

    Examples:
      | options                                                 | message                   |
      |                                                         | nothing to update         |
      | --subscription-plan-id plan-a --clear-subscription-plan | cannot be used together   |
      | --rotate-token --subscription-token token-a             | cannot be used together   |
      | --subscription-token=                                   | must not be empty         |
      | --rotate-token                                          | failed to read password   |

  Scenario: The gateway's reason is shown when it rejects an update, and nothing changes
    Given I create subscription from "resources/templates/subscription.yaml" through the CLI with values:
      | apiVersion              | ${CTX:gatewaySpecVersion} |
      | name                    | ${CTX:subRef}             |
      | spec.externalRefId      | ${CTX:subRef}             |
      | spec.apiId              | ${CTX:apiName}            |
      | spec.subscriptionPlanId | ${CTX:goldPlanId}         |
      | spec.subscriptionToken  | ${CTX:subToken}           |
    And the CLI command should succeed
    And I generate a unique value from "cli-sub-missing" and store it as "missingId"

    When I run ap with arguments "gateway subscription update --id ${CTX:subRef} --subscription-plan-id ${CTX:missingId}"
    Then the CLI command should fail
    And the CLI output should contain "Subscription plan not found or not enabled"

    When I run ap with arguments "gateway subscription update --id ${CTX:subRef} --status BOGUS"
    Then the CLI command should fail
    And the CLI output should contain "invalid status: BOGUS"

    When I run ap with arguments "gateway subscription update --id ${CTX:missingId} --status REVOKED"
    Then the CLI command should fail
    And the CLI output should contain "Subscription not found"

    When I run ap with arguments "gateway subscription get --id ${CTX:subRef}"
    Then the CLI command should succeed
    And the CLI output should contain "${CTX:goldPlanId}"
    And the CLI output should contain "ACTIVE"
    When I set header "Subscription-Key" to "${CTX:subToken}"
    Then I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200

  # A 409 never says which value collided, so it cannot reveal that a token is in use.
  Scenario: A token or ID that another subscription uses is rejected with a generic conflict
    Given I create subscription from "resources/templates/subscription.yaml" through the CLI with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:subRef}             |
      | spec.externalRefId     | ${CTX:subRef}             |
      | spec.apiId             | ${CTX:apiName}            |
      | spec.subscriptionToken | ${CTX:subToken}           |
    And the CLI command should succeed
    And I generate a unique value from "cli-sub-other-ref" and store it as "otherRef"
    And I generate a unique value from "cli-sub-other-token" and store it as "otherToken"
    And I create subscription from "resources/templates/subscription.yaml" through the CLI with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:otherRef}           |
      | spec.externalRefId     | ${CTX:otherRef}           |
      | spec.apiId             | ${CTX:apiName}            |
      | spec.subscriptionToken | ${CTX:otherToken}         |
    And the CLI command should succeed

    When I run ap with arguments "gateway subscription update --id ${CTX:subRef} --subscription-token ${CTX:otherToken}"
    Then the CLI command should fail
    And the CLI output should contain "409"
    And the CLI output should contain "The subscription conflicts with an existing subscription"
    And the CLI output should not contain "${CTX:otherToken}"

    When I create subscription from "resources/templates/subscription.yaml" through the CLI with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:subRef}             |
      | spec.externalRefId     | ${CTX:subRef}             |
      | spec.apiId             | ${CTX:apiName}            |
      | spec.subscriptionToken | ${CTX:otherToken}         |
    Then the CLI command should fail
    And the CLI output should contain "The subscription conflicts with an existing subscription"

    When I set header "Subscription-Key" to "${CTX:subToken}"
    Then I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200
