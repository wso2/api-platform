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

@APIPORTAL
Feature: API Portal subscription plans, billing and paid subscriptions
  Journey APIPUI-012: an administrator adds a free plan with limits, offers it on an API and cannot
  delete it while it has a subscriber.
  Journey APIPUI-019: an administrator sets up billing with Stripe and Moesif, adds meters and
  prices plans.
  Journey APIPUI-020: a developer pays for plans, follows invoices and usage, and ends paid
  subscriptions.
  Billing is portal-wide: these journeys run one at a time on their own stack, and bill through the
  testbench's Stripe and Moesif services.

  Background:
    Given the user is signed in to the API Portal

  @APIP-023
  Scenario: Journey: an administrator adds a free plan with limits and offers it on an API
    When the administrator adds a free plan with a per-minute and a per-day limit in the settings
    And the administrator raises that plan's per-minute limit
    And the administrator offers that plan on an API in the settings
    Then the API page shows that plan's card with both of its limits
    And that plan cannot be deleted while a developer subscribes to it

  @APIP-039 @APIP-040 @APIP-041
  Scenario: Journey: an administrator sets up billing, adds meters and prices plans
    Then the administrator is sent to set up billing when pricing a plan without it
    When the administrator sets up billing in the settings, correcting the keys the form and Stripe refuse
    Then the administrator moves the billing webhook host and removes billing in the settings
    Given billing is set up in the API Portal
    When the administrator adds a Tokens meter with a suggested and a custom property in the settings
    Then the administrator edits that meter, and deletes it
    When the administrator prices a flat rate plan in the settings, correcting its limits and amount
    Then the administrator edits that plan, and deletes it
    Given the API Portal has a meter of requests by status and method
    Then the administrator prices a usage based plan in the settings, correcting each price

  @APIP-042 @APIP-043
  Scenario: Journey: a developer pays for a flat rate and a usage based plan, follows them and ends them
    Given billing is set up in the API Portal
    And the API Portal has an API with a free plan and a paid flat rate plan
    And the developer signs in to the API Portal
    When the developer starts paying for the flat rate plan, after checkouts that fail or are not paid
    And the developer pays for the plan and receives its token
    Then the developer pages through the invoices, and opens the billing page
    And the administrator signs in to the API Portal
    And the administrator reads the revenue of the paid plan by month
    And the developer signs in to the API Portal
    And the developer unsubscribes from the flat rate plan, and ends it now in Subscriptions
    Given the administrator signs in to the API Portal
    And the API Portal has a meter of requests by status and method
    And the API Portal has an MCP server with a paid usage based plan
    And the developer signs in to the API Portal
    When the developer starts paying for the usage based plan on the MCP server page
    And the developer pays for the plan and receives its token
    Then the developer follows the subscription's usage per hour, day and week
    And the developer sees the payment pending in Subscriptions and in Usage while Stripe can't collect it
    And the developer unsubscribes from the usage based plan on the MCP server page
