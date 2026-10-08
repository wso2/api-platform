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
#
# NOT YET BOUND TO A RUNNER in ui-suite.yaml, deliberately. Every scenario here needs the
# AI Workspace change that records upstreamMcpSpecVersions and drops the hardcoded
# mcpSpecVersion; against a portal image built without it they fail. Add a
# `mcp-spec-versions` runner naming this file once that change is on main.

Feature: MCP proxy upstream protocol versions
  What the AI Workspace does with the protocol versions a fetch-server-info probe reports.
  It records them on the proxy for reference, declares none on the proxy's own behalf, and
  says so plainly when the server named none.

  Scenario: Creating a proxy records the versions the upstream reported and declares none of its own
    Given the user is signed in
    And the MCP server reports the protocol versions "2026-07-28, 2025-11-25"
    And the user creates a project named "TC105 MCP Versions Project"
    And the user opens the project "TC105 MCP Versions Project"
    And the user opens MCP Proxies
    When the user creates the MCP proxy "TC105 MCP Versions Server" at "https://sample.mcp.example.com/mcp" with the auth header "Authorization" set to "Bearer tok-setup-key"
    Then the MCP proxy was created with the upstream protocol versions "2026-07-28, 2025-11-25"
    And the MCP proxy create body declares no MCP spec version

  Scenario: Creating a proxy from a server that names no version records nothing rather than an empty list
    Given the user is signed in
    And the MCP server reports no protocol versions
    And the user creates a project named "TC108 MCP Versions Project"
    And the user opens the project "TC108 MCP Versions Project"
    And the user opens MCP Proxies
    When the user creates the MCP proxy "TC108 MCP Versions Server" at "https://sample.mcp.example.com/mcp" with the auth header "Authorization" set to "Bearer tok-setup-key"
    # The key must be absent, not []. The API stores whatever it is given, so an empty list
    # would record "asked, and it named none" where absence records nothing at all.
    Then the MCP proxy create body records no upstream protocol versions
    And the MCP proxy create body declares no MCP spec version

  Scenario: A server naming the same revision twice is recorded once
    Given the user is signed in
    And the MCP server reports the protocol versions "2026-07-28, 2025-06-18, 2026-07-28"
    And the user creates a project named "TC109 MCP Versions Project"
    And the user opens the project "TC109 MCP Versions Project"
    And the user opens MCP Proxies
    When the user creates the MCP proxy "TC109 MCP Versions Server" at "https://sample.mcp.example.com/mcp" with the auth header "Authorization" set to "Bearer tok-setup-key"
    Then the MCP proxy was created with the upstream protocol versions "2026-07-28, 2025-06-18"

  Scenario: A refetch that discovers versions shows them, and saving records them
    Given the user is signed in
    And the MCP server reports no protocol versions
    And the user creates a project named "TC106 MCP Versions Project"
    And the user opens the project "TC106 MCP Versions Project"
    And the user opens MCP Proxies
    And the user creates the MCP proxy "TC106 MCP Versions Server" at "https://sample.mcp.example.com/mcp" with the auth header "Authorization" set to "Bearer tok-setup-key"
    And the user opens the MCP proxy's Backend Connection tab
    # The proxy was created from a server naming nothing, so the refetch is the first thing
    # to discover a version set — which is what makes Save reachable on versions alone.
    When the MCP server reports the protocol versions "2026-07-28, 2025-11-25"
    And the user refetches the server info
    Then the user sees "Connection verified" on the page
    And the user sees "Supported MCP versions" on the page
    And the user sees "2026-07-28" on the page

    When the user saves the backend connection
    Then the MCP proxy update carries the upstream protocol versions "2026-07-28, 2025-11-25"

  Scenario: A server that stops reporting versions clears what the proxy recorded
    Given the user is signed in
    And the MCP server reports the protocol versions "2026-07-28, 2025-11-25"
    And the user creates a project named "TC110 MCP Versions Project"
    And the user opens the project "TC110 MCP Versions Project"
    And the user opens MCP Proxies
    And the user creates the MCP proxy "TC110 MCP Versions Server" at "https://sample.mcp.example.com/mcp" with the auth header "Authorization" set to "Bearer tok-setup-key"
    And the user opens the MCP proxy's Backend Connection tab
    # Now pointed at a backend that names none. The recorded set describes the old server, so
    # an empty result has to overwrite it rather than read as "nothing was learnt" — which is
    # what left one server's versions recorded against another.
    When the MCP server reports no protocol versions
    And the user refetches the server info
    Then the user sees "Connection verified" on the page
    And the user sees "Not detected" on the page

    When the user saves the backend connection
    Then the MCP proxy update carries the upstream protocol versions ""

  Scenario: A server that reports no versions is shown as not detected
    Given the user is signed in
    And the MCP server reports no protocol versions
    And the user creates a project named "TC107 MCP Versions Project"
    And the user opens the project "TC107 MCP Versions Project"
    And the user opens MCP Proxies
    And the user creates the MCP proxy "TC107 MCP Versions Server" at "https://sample.mcp.example.com/mcp" with the auth header "Authorization" set to "Bearer tok-setup-key"
    And the user opens the MCP proxy's Backend Connection tab
    When the user refetches the server info
    Then the user sees "Connection verified" on the page
    And the user sees "Supported MCP versions" on the page
    And the user sees "Not detected" on the page
