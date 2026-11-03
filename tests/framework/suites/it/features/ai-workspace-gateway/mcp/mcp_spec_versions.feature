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

@mcp-spec-versions
Feature: MCP proxy specification versions
  As an API developer
  I want an MCP proxy to declare every specification revision it serves
  So that a proxy in front of a dual-era server describes what that server actually speaks

  # A server can serve several revisions at once, so the single-valued specVersion cannot
  # describe it. specVersions is the list form; specVersion is deprecated and honoured only
  # while the list is absent. The gateway validates the shape of a revision date and not its
  # value: which revisions it serves is its own property, so a revision it does not serve is a
  # limitation rather than a bad configuration, while a value that is not a date is a typo.
  #
  # The scenarios here assert behaviour that current source builds have. mcp_deploy.feature
  # keeps the one malformed-version case that holds across every supported gateway release.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Deploy an MCP proxy declaring several spec versions
    Given I generate a unique resource name from "mcp-multi-spec" and store it as "mcpName"
    And I generate a unique value from "mcp-multi-spec" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-multi-spec" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-multi-spec" and store it as "mcpContext"
    # A complete spec override, because the canonical template carries the deprecated scalar and
    # declaring both forms is itself an error. The placeholder values are harmless; the override
    # replaces the whole spec before the request is sent.
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | name        | ${CTX:mcpName}            |
      | displayName | Replaced                  |
      | version     | v1.0                      |
      | context     | /replaced                 |
      | specVersion | 2025-06-18                |
      | spec        | {"displayName":"${CTX:mcpDisplayName}","version":"${CTX:mcpVersion}","context":"${CTX:mcpContext}","specVersions":["2025-06-18","2026-07-28"],"upstream":{"url":"http://testbench:3009${CTX:gatewayMCPUpstreamPath}"},"tools":[],"resources":[],"prompts":[]} |
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"

    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # The two forms carry the same meaning, so resolving silently would pick for the operator.
  Scenario: Deploy an MCP proxy declaring both spec version forms returns 400
    Given I generate a unique resource name from "mcp-both-forms" and store it as "mcpName"
    And I generate a unique value from "mcp-both-forms" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-both-forms" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-both-forms" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | name        | ${CTX:mcpName}            |
      | displayName | Replaced                  |
      | version     | v1.0                      |
      | context     | /replaced                 |
      | specVersion | 2025-06-18                |
      | spec        | {"displayName":"${CTX:mcpDisplayName}","version":"${CTX:mcpVersion}","context":"${CTX:mcpContext}","specVersion":"2025-06-18","specVersions":["2026-07-28"],"upstream":{"url":"http://testbench:3009${CTX:gatewayMCPUpstreamPath}"},"tools":[],"resources":[],"prompts":[]} |
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  # A revision this gateway does not serve is a gateway limitation, not a bad configuration, so
  # the proxy deploys. Both directions are covered: the gateway used to reject revisions older
  # than the ones it serves, and a revision released after this build must not fail either.
  Scenario Outline: Deploy an MCP proxy declaring a revision this gateway does not serve (<reason>)
    Given I generate a unique resource name from "mcp-unsupported-<slug>" and store it as "mcpName"
    And I generate a unique value from "mcp-unsupported-<slug>" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-unsupported-<slug>" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-unsupported-<slug>" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | name        | ${CTX:mcpName}            |
      | displayName | Replaced                  |
      | version     | v1.0                      |
      | context     | /replaced                 |
      | specVersion | 2025-06-18                |
      | spec        | {"displayName":"${CTX:mcpDisplayName}","version":"${CTX:mcpVersion}","context":"${CTX:mcpContext}","specVersions":["2025-06-18","<version>"],"upstream":{"url":"http://testbench:3009${CTX:gatewayMCPUpstreamPath}"},"tools":[],"resources":[],"prompts":[]} |
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"

    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

    Examples:
      | slug   | version    | reason                               |
      | older  | 2025-03-26 | released before the oldest supported |
      | future | 2027-03-01 | released after this build            |

  # The deprecated scalar keeps working on its own, including for a revision that predates the
  # protected-resource model. The upstream is present, so the version is the only thing that
  # could fail the deployment.
  Scenario: Deploy an MCP proxy declaring only a legacy revision through the deprecated field
    Given I generate a unique resource name from "mcp-legacy-spec" and store it as "mcpName"
    And I generate a unique value from "mcp-legacy-spec" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-legacy-spec" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-legacy-spec" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2025-03-26                |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"

    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # A value that is not a revision date is a typo, not a limitation, and is still rejected.
  # "banana" is the one that matters most: revisions are compared as strings, and
  # "banana" >= "2025-06-18" is true, so without a date check it would read as modern.
  Scenario Outline: Deploy an MCP proxy with a malformed spec version returns 400 (<reason>)
    Given I generate a unique resource name from "mcp-malformed-<slug>" and store it as "mcpName"
    And I generate a unique value from "mcp-malformed-<slug>" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-malformed-<slug>" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-malformed-<slug>" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | name        | ${CTX:mcpName}            |
      | displayName | Replaced                  |
      | version     | v1.0                      |
      | context     | /replaced                 |
      | specVersion | 2025-06-18                |
      | spec        | {"displayName":"${CTX:mcpDisplayName}","version":"${CTX:mcpVersion}","context":"${CTX:mcpContext}","specVersions":["<version>"],"upstream":{"url":"http://testbench:3009${CTX:gatewayMCPUpstreamPath}"},"tools":[],"resources":[],"prompts":[]} |
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "<version>"
    And the response body should contain "expected a revision date"

    Examples:
      | slug        | version    | reason                         |
      | singledigit | 2025-6-18  | not a padded date              |
      | notaday     | 2025-13-45 | shaped like a date but is none |
      | notadate    | banana     | not a date at all              |
