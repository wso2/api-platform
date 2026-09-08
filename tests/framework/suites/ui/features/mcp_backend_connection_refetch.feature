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

Feature: MCP proxy Backend Connection refetch behavior
  The journey ported from the product's own Cypress suite
  (005-mcp-backend-connection-refetch), verifying "Refetch Server Info" sends the right
  request shape depending on which connection fields have been edited since load, and that
  saving preserves or rotates the stored credential correctly.

  Scenario: Refetching without any edits uses the stored proxy and omits auth entirely
    Given the user is signed in
    And the user creates a project named "TC98 MCP Refetch Project"
    And the user opens the project "TC98 MCP Refetch Project"
    And the user opens MCP Proxies
    And the user creates the MCP proxy "TC98 MCP Refetch Server" at "https://sample.mcp.example.com/mcp" with the auth header "Authorization" set to "Bearer tok-setup-key"
    And the user opens the MCP proxy's Backend Connection tab
    And the backend connection auth value field shows the masked sentinel
    When the user refetches the server info
    Then the refetch request used only the stored proxy
    And the user sees "Connection verified" on the page

  Scenario: Refetching after editing the endpoint, header, and value sends the live values directly
    Given the user is signed in
    And the user creates a project named "TC99 MCP Refetch Project"
    And the user opens the project "TC99 MCP Refetch Project"
    And the user opens MCP Proxies
    And the user creates the MCP proxy "TC99 MCP Refetch Server" at "https://sample.mcp.example.com/mcp" with the auth header "Authorization" set to "Bearer tok-setup-key"
    And the user opens the MCP proxy's Backend Connection tab
    And the backend connection auth value field shows the masked sentinel
    And the user edits the backend connection URL to "https://updated.mcp.example.com/mcp"
    And the user edits the backend connection auth header to "X-Api-Key"
    And the user edits the backend connection auth value to "super-secret-live-value"
    When the user refetches the server info
    Then the refetch request sent the live credential for "https://updated.mcp.example.com/mcp" with header "X-Api-Key" and value "super-secret-live-value"
    And the user sees "Connection verified" on the page

  Scenario: Saving edited connection details rotates the secret, and a later refetch goes back to using the stored proxy
    Given the user is signed in
    And the user creates a project named "TC100 MCP Refetch Project"
    And the user opens the project "TC100 MCP Refetch Project"
    And the user opens MCP Proxies
    And the user creates the MCP proxy "TC100 MCP Refetch Server" at "https://sample.mcp.example.com/mcp" with the auth header "Authorization" set to "Bearer tok-setup-key"
    And the user opens the MCP proxy's Backend Connection tab
    And the backend connection auth value field shows the masked sentinel
    And the user edits the backend connection URL to "https://updated-and-saved.mcp.example.com/mcp"
    And the user edits the backend connection auth header to "X-Api-Key"
    And the user edits the backend connection auth value to "super-secret-value-to-be-saved"
    When the user saves the backend connection
    Then the MCP proxy update carries the URL "https://updated-and-saved.mcp.example.com/mcp"
    And the MCP proxy was updated with a placeholder referencing that secret, not the credential "super-secret-value-to-be-saved"
    And a secret was created for that credential
    And the backend connection auth value field shows the masked sentinel

    When the user refetches the server info
    Then the refetch request used only the stored proxy

  Scenario: Saving a URL-only edit preserves the existing credential without creating a new secret
    Given the user is signed in
    And the user creates a project named "TC101 MCP Refetch Project"
    And the user opens the project "TC101 MCP Refetch Project"
    And the user opens MCP Proxies
    And the user creates the MCP proxy "TC101 MCP Refetch Server" at "https://sample.mcp.example.com/mcp" with the auth header "Authorization" set to "Bearer tok-setup-key"
    And the user opens the MCP proxy's Backend Connection tab
    And the backend connection auth value field shows the masked sentinel
    And the user edits the backend connection URL to "https://url-only-edit.mcp.example.com/mcp"
    When the user saves the backend connection
    Then the MCP proxy update carries the URL "https://url-only-edit.mcp.example.com/mcp"
    And the MCP proxy update kept the auth header "Authorization" and type "header"
    And no secret was created for that credential

  Scenario: Refetching after a URL-only edit sends the edited URL alongside the stored proxy
    Given the user is signed in
    And the user creates a project named "TC102 MCP Refetch Project"
    And the user opens the project "TC102 MCP Refetch Project"
    And the user opens MCP Proxies
    And the user creates the MCP proxy "TC102 MCP Refetch Server" at "https://sample.mcp.example.com/mcp" with the auth header "Authorization" set to "Bearer tok-setup-key"
    And the user opens the MCP proxy's Backend Connection tab
    And the backend connection auth value field shows the masked sentinel
    And the user edits the backend connection URL to "https://sample.mcp.example.com/v2/mcp"
    When the user refetches the server info
    Then the refetch request used the edited URL "https://sample.mcp.example.com/v2/mcp" alongside the stored proxy
    And the user sees "Connection verified" on the page
