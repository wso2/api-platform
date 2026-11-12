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

Feature: API Portal billing

  # Billing is portal-wide: these scenarios run one at a time on their own stack, and bill through
  # the testbench's Stripe and Moesif services.

  Scenario: An administrator sets up billing, moves its webhook host and removes it
    Given the user is signed in to the API Portal
    When the administrator sets up billing in the settings, correcting the keys the form and Stripe refuse
    Then the administrator moves the billing webhook host and removes billing in the settings

  Scenario: An administrator adds a meter, edits it and deletes it
    Given the user is signed in to the API Portal
    When the administrator adds a Tokens meter with a suggested and a custom property in the settings
    Then the administrator edits that meter, and deletes it

  Scenario: A plan can be paid only once billing is set up
    Given the user is signed in to the API Portal
    Then the administrator is sent to set up billing when pricing a plan without it

  Scenario: An administrator prices a flat rate plan, edits it and deletes it
    Given the user is signed in to the API Portal
    And billing is set up in the API Portal
    When the administrator prices a flat rate plan in the settings, correcting its limits and amount
    Then the administrator edits that plan, and deletes it

  Scenario: An administrator prices a usage based plan with two prices on one meter
    Given the user is signed in to the API Portal
    And billing is set up in the API Portal
    And the API Portal has a meter of requests by status and method
    Then the administrator prices a usage based plan in the settings, correcting each price

  Scenario: A developer pays for a flat rate plan on an API page, and ends the subscription
    Given the user is signed in to the API Portal
    And billing is set up in the API Portal
    And the API Portal has an API with a free plan and a paid flat rate plan
    And the developer signs in to the API Portal
    When the developer starts paying for the flat rate plan, after checkouts that fail or are not paid
    And the developer pays for the plan and receives its token
    Then the developer pages through the invoices, and opens the billing page
    And the administrator signs in to the API Portal
    And the administrator reads the revenue of the paid plan by month
    And the developer signs in to the API Portal
    And the developer unsubscribes from the flat rate plan, and ends it now in Subscriptions

  Scenario: A developer pays for a usage based plan on an MCP server page, and follows its usage
    Given the user is signed in to the API Portal
    And billing is set up in the API Portal
    And the API Portal has a meter of requests by status and method
    And the API Portal has an MCP server with a paid usage based plan
    And the developer signs in to the API Portal
    When the developer starts paying for the usage based plan on the MCP server page
    And the developer pays for the plan and receives its token
    Then the developer follows the subscription's usage per hour, day and week
    And the developer sees the payment pending in Subscriptions and in Usage while Stripe can't collect it
    And the developer unsubscribes from the usage based plan on the MCP server page
