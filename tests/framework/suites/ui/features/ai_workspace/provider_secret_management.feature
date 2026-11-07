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

Feature: LLM provider credential secrecy
  As a security-conscious operator
  I want a provider's upstream credential to be stored as a secret and never persisted or
  displayed in plaintext
  So that a leaked configuration dump or shared screen never exposes it

  Scenario: Creating a provider with a plaintext credential stores it as a secret placeholder
    Given the user is signed in
    When the user starts adding a provider from the "OpenAI" template
    And the user creates the provider "TC57 Secret Provider" using the template's built-in endpoint with the credential "sk-tc57-plaintext-key"
    Then the user is on the provider's overview page
    And the provider was created with a placeholder referencing that secret, not the credential "sk-tc57-plaintext-key"
    And a secret was created for that credential
    And the page never shows the credential "sk-tc57-plaintext-key"

  Scenario: Creating a provider whose credential is already a secret placeholder does not mint a new secret
    Given the user is signed in
    And a secret "tc58-existing-key" already holds the value "sk-tc58-pre-existing-value"
    When the user starts adding a provider from the "OpenAI" template
    And the user creates the provider "TC58 Secret Provider" using the template's built-in endpoint with the credential placeholder "tc58-existing-key"
    Then the user is on the provider's overview page
    And the provider was created with the placeholder referencing "tc58-existing-key"
    And no secret was created for that credential

  Scenario: A failure to store the credential aborts provider creation
    Given the user is signed in
    And the user starts adding a provider from the "OpenAI" template
    And creating a secret always fails
    When the user creates the provider "TC59 Secret Provider" using the template's built-in endpoint with the credential "sk-tc59-will-fail"
    Then the user sees an error notification
    And no provider was created

  Scenario: Fetching a secret directly never returns its plaintext value
    Given the user is signed in
    And a secret "tc63-encryption-proof" already holds the value "sk-tc63-encryption-proof"
    Then fetching the secret "tc63-encryption-proof" directly returns no plaintext value

  Scenario: Editing a provider's credential rotates its secret
    Given the user is signed in
    And the user starts adding a provider from the "OpenAI" template
    And the user creates the provider "TC60 Update Provider" using the template's built-in endpoint with the credential "sk-update-initial"
    And the user is on the provider's overview page
    And the user opens the provider's Connection tab
    When the user changes the provider's credential to "sk-update-new"
    Then the provider was updated
    And a secret was created for that credential
    And the provider was updated with a placeholder referencing that secret, not the credential "sk-update-new"
    And the page never shows the credential "sk-update-new"

  Scenario: Typing an explicit secret placeholder as the new credential skips secret creation
    Given the user is signed in
    And a secret "tc61-explicit-handle" already holds the value "sk-tc61-explicit-handle-value"
    And the user starts adding a provider from the "OpenAI" template
    And the user creates the provider "TC61 Update Provider" using the template's built-in endpoint with the credential "sk-update-initial"
    And the user is on the provider's overview page
    And the user opens the provider's Connection tab
    When the user changes the provider's credential to the placeholder referencing "tc61-explicit-handle"
    Then the provider was updated
    And no secret was created for that credential
    And the provider was updated with the placeholder referencing "tc61-explicit-handle"

  Scenario: A failure to store the new credential aborts the update
    Given the user is signed in
    And the user starts adding a provider from the "OpenAI" template
    And the user creates the provider "TC62 Update Provider" using the template's built-in endpoint with the credential "sk-update-initial"
    And the user is on the provider's overview page
    And the user opens the provider's Connection tab
    And creating a secret always fails
    When the user changes the provider's credential to "sk-tc62-will-fail"
    Then the user sees an error notification
    And the provider was not updated

  Scenario: Editing a provider's credential from the provider list rotates its secret the same way
    Given the user is signed in
    And the user starts adding a provider from the "OpenAI" template
    And the user creates the provider "TC64 List Update Provider" using the template's built-in endpoint with the credential "sk-update-initial"
    And the user is on the provider's overview page
    When the user opens the provider "TC64 List Update Provider" from the provider list
    And the user opens the provider's Connection tab
    And the user changes the provider's credential to "sk-update-via-list"
    Then the provider was updated
    And a secret was created for that credential
    And the provider was updated with a placeholder referencing that secret, not the credential "sk-update-via-list"
