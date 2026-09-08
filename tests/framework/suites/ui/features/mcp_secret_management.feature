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

Feature: MCP proxy credential secrecy
  As a security-conscious operator
  I want an MCP proxy's upstream credential to be stored as a secret and never persisted
  or displayed in plaintext
  So that a leaked configuration dump or shared screen never exposes it

  Scenario: Creating an MCP proxy with a credential stores it as a secret placeholder
    Given the user is signed in
    And the user creates a project named "TC80 MCP Secret Project"
    And the user opens the project "TC80 MCP Secret Project"
    And the user opens MCP Proxies
    When the user creates the MCP proxy "TC80 MCP Secret Server" at "https://sample.mcp.example.com/mcp" with the auth header "Authorization" set to "Bearer tok-tc80-plaintext"
    Then the user is on the MCP proxy's overview page
    And the MCP proxy was created with a placeholder referencing that secret, not the credential "Bearer tok-tc80-plaintext"
    And a secret was created for that credential
    And the secret handle is a random UUID
    And the page never shows the credential "Bearer tok-tc80-plaintext"

  Scenario: Validating an MCP proxy endpoint does not create a secret
    Given the user is signed in
    And the user creates a project named "TC81 MCP Secret Project"
    And the user opens the project "TC81 MCP Secret Project"
    And the user opens MCP Proxies
    When the user validates the MCP proxy endpoint "https://sample.mcp.example.com/mcp" with the auth header "Authorization" set to "Bearer tok-tc81-validate-only"
    Then no secret was created for that credential

  Scenario: Creating an MCP proxy whose credential is already a secret placeholder does not mint a new secret
    Given the user is signed in
    And a secret "tc82-existing-key" already holds the value "Bearer tok-tc82-original"
    And the user creates a project named "TC82 MCP Secret Project"
    And the user opens the project "TC82 MCP Secret Project"
    And the user opens MCP Proxies
    When the user creates the MCP proxy "TC82 MCP Secret Server" at "https://sample.mcp.example.com/mcp" with the auth header "Authorization" set to the placeholder referencing "tc82-existing-key"
    Then the user is on the MCP proxy's overview page
    And no secret was created for that credential

  Scenario: A failure to store the credential aborts MCP proxy creation
    Given the user is signed in
    And the user creates a project named "TC83 MCP Secret Project"
    And the user opens the project "TC83 MCP Secret Project"
    And the user opens MCP Proxies
    And creating a secret always fails
    When the user creates the MCP proxy "TC83 MCP Secret Server" at "https://sample.mcp.example.com/mcp" with the auth header "Authorization" set to "Bearer tok-tc83-will-fail"
    Then the user sees an error notification
    And no MCP proxy was created

  Scenario: Creating an MCP proxy without a credential omits the auth block entirely
    Given the user is signed in
    And the user creates a project named "TC84 MCP Secret Project"
    And the user opens the project "TC84 MCP Secret Project"
    And the user opens MCP Proxies
    When the user creates the MCP proxy "TC84 MCP Secret Server" at "https://sample.mcp.example.com/mcp" with no credential
    Then the user is on the MCP proxy's overview page
    And no secret was created for that credential
    And the MCP proxy was created without an auth block

  Scenario: The secret handle is a random UUID, not derived from the proxy's name
    Given the user is signed in
    And the user creates a project named "TC85 MCP Secret Project"
    And the user opens the project "TC85 MCP Secret Project"
    And the user opens MCP Proxies
    When the user creates the MCP proxy "My MCP TC85" at "https://sample.mcp.example.com/mcp" with the auth header "Authorization" set to "Bearer tok-tc85-handle-check"
    Then the user is on the MCP proxy's overview page
    And the secret handle is a random UUID
