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

Feature: LLM proxy credential secrecy
  As a security-conscious operator
  I want an app LLM proxy's provider credential to be stored as a secret and never
  persisted or displayed in plaintext
  So that a leaked configuration dump or shared screen never exposes it

  Scenario: Creating a proxy with a plaintext credential stores it as a secret placeholder
    Given the user is signed in
    And the user creates a project named "TC1 Secret Project"
    And the user starts adding a provider from the "OpenAI" template
    And the user creates the provider "TC1 Secret Provider" using the template's built-in endpoint
    Then the user is on the provider's overview page
    When the user creates an app LLM proxy "TC1 Secret Proxy" in project "TC1 Secret Project" using the API key "sk-tc1-proxy-plaintext-key"
    Then the user is on the proxy's overview page
    And the proxy was created with a placeholder referencing that secret, not the credential "sk-tc1-proxy-plaintext-key"
    And a secret was created for that credential
    And the page never shows the credential "sk-tc1-proxy-plaintext-key"

  Scenario: Creating a proxy whose credential is already a secret placeholder does not mint a new secret
    Given the user is signed in
    And a secret "tc2-existing-key" already holds the value "sk-tc2-pre-existing-value"
    And the user creates a project named "TC2 Secret Project"
    And the user starts adding a provider from the "OpenAI" template
    And the user creates the provider "TC2 Secret Provider" using the template's built-in endpoint
    Then the user is on the provider's overview page
    When the user creates an app LLM proxy "TC2 Secret Proxy" in project "TC2 Secret Project" using the API key placeholder referencing "tc2-existing-key"
    Then the user is on the proxy's overview page
    And the proxy was created with the placeholder referencing "tc2-existing-key"
    And no secret was created for that credential

  Scenario: A failure to store the credential aborts proxy creation
    Given the user is signed in
    And the user creates a project named "TC3 Secret Project"
    And the user starts adding a provider from the "OpenAI" template
    And the user creates the provider "TC3 Secret Provider" using the template's built-in endpoint
    Then the user is on the provider's overview page
    And creating a secret always fails
    When the user creates an app LLM proxy "TC3 Secret Proxy" in project "TC3 Secret Project" using the API key "sk-tc3-will-fail"
    Then the user sees an error notification
    And no proxy was created

  Scenario: Editing a proxy's credential rotates its secret and cleans up the old one
    Given the user is signed in
    And the user creates a project named "TC4 Secret Project"
    And the user starts adding a provider from the "OpenAI" template
    And the user creates the provider "TC4 Secret Provider" using the template's built-in endpoint
    Then the user is on the provider's overview page
    And the user creates an app LLM proxy "TC4 Secret Proxy" in project "TC4 Secret Project" using the API key "sk-proxy-update-initial"
    And the user is on the proxy's overview page
    And a secret was created for that credential
    And the current secret is remembered as the original
    And the user opens the proxy's Provider tab
    When the user changes the proxy's credential to "sk-proxy-update-new"
    Then the proxy was updated
    And a secret was created for that credential
    And the proxy was updated with a placeholder referencing that secret, not the credential "sk-proxy-update-new"
    And the page never shows the credential "sk-proxy-update-new"
    And the original secret is now deprecated

  Scenario: Typing an explicit secret placeholder as the new proxy credential skips secret creation
    Given the user is signed in
    And a secret "tc5-explicit-handle" already holds the value "sk-tc5-explicit-handle-value"
    And the user creates a project named "TC5 Secret Project"
    And the user starts adding a provider from the "OpenAI" template
    And the user creates the provider "TC5 Secret Provider" using the template's built-in endpoint
    Then the user is on the provider's overview page
    And the user creates an app LLM proxy "TC5 Secret Proxy" in project "TC5 Secret Project" using the API key "sk-proxy-update-initial"
    And the user is on the proxy's overview page
    And the user opens the proxy's Provider tab
    When the user changes the proxy's credential to the placeholder referencing "tc5-explicit-handle"
    Then the proxy was updated
    And no secret was created for that credential
    And the proxy was updated with the placeholder referencing "tc5-explicit-handle"

  Scenario: A failure to store the new proxy credential aborts the update
    Given the user is signed in
    And the user creates a project named "TC6 Secret Project"
    And the user starts adding a provider from the "OpenAI" template
    And the user creates the provider "TC6 Secret Provider" using the template's built-in endpoint
    Then the user is on the provider's overview page
    And the user creates an app LLM proxy "TC6 Secret Proxy" in project "TC6 Secret Project" using the API key "sk-proxy-update-initial"
    And the user is on the proxy's overview page
    And the user opens the proxy's Provider tab
    And creating a secret always fails
    When the user changes the proxy's credential to "sk-tc6-will-fail"
    Then the user sees an error notification
    And the proxy was not updated
