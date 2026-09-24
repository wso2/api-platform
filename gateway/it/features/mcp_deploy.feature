# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

Feature: Test MCP CRUD and connectivity
    As an API developer
    I want to deploy an MCP Proxy configuration and connect to it
    So that I can verify the gateway routes the MCP requests correctly

    Background:
        Given the gateway services are running
        
    Scenario: Deploy a sample MCP Server and do a tools/call
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: everything-mcp-v1.0
            spec:
              displayName: Everything
              version: v1.0
              context: /everything
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"

        Given I authenticate using basic auth as "admin"
        When I list all MCP proxies
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And the JSON response field "count" should be 1
        And I wait for 2 seconds
    
        When I use the MCP Client to send an initialize request to "http://127.0.0.1:8080/everything/mcp"
        Then the response should be successful
        When I use the MCP Client to send "add" tools/call request to "http://127.0.0.1:8080/everything/mcp"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response should have field "result"
        And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."
        
        Given I authenticate using basic auth as "admin"
        When I update the MCP proxy "everything-mcp-v1.0" with:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: everything-mcp-v1.0
            spec:
              displayName: Everything
              version: v1.0
              context: /everything
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"

        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "everything-mcp-v1.0"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        When I list all MCP proxies
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And the JSON response field "count" should be 0

    # Regression test: an MCP proxy with no policies attached fronting a Streamable-HTTP
    # MCP server returned HTTP 500
    # from Envoy ("mismatch_between_content_length_and_the_length_of_the_mutated_body")
    # on messages around initialize. mcp-streamable-backend (the official MCP reference
    # server) reproduces it because its responses carry a content-length header on a
    # chunked text/event-stream body — unlike mcp-server-backend used elsewhere in this
    # file, which does not trigger it.
    Scenario: Deploy an MCP Proxy fronting a Streamable-HTTP server with no policies attached
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: everything-streamable-mcp-v1.0
            spec:
              displayName: Everything Streamable
              version: v1.0
              context: /everything-streamable
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-streamable-backend:3001/mcp
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        When I use the MCP Client to send an initialize request to "http://127.0.0.1:8080/everything-streamable/mcp"
        Then the response should be successful
        When I use the MCP Client to send a notifications/initialized notification to "http://127.0.0.1:8080/everything-streamable/mcp"
        Then the response should be successful
        When I use the MCP Client to send a tools/list request to "http://127.0.0.1:8080/everything-streamable/mcp"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response should have field "result.tools"

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "everything-streamable-mcp-v1.0"
        Then the response should be successful

    Scenario: Deploy an MCP Proxy and send an invalid tools/call request
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: invalid-tools-mcp-v1.0
            spec:
              displayName: Invalid Tools
              version: v1.0
              context: /invalid-tools
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        When I use the MCP Client to send a tools/call request with invalid params to "http://127.0.0.1:8080/invalid-tools/mcp"
        Then the response status code should be 400
        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "invalid-tools-mcp-v1.0"
        Then the response should be successful

    # ==================== MCP PROXY ERROR CASES ====================
    
    Scenario: List MCP proxies when none exist
        Given I authenticate using basic auth as "admin"
        When I list all MCP proxies
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And the JSON response field "count" should be 0

    Scenario: List MCP proxies with pagination parameters
        Given I authenticate using basic auth as "admin"
        When I send a GET request to the "gateway-controller" service at "/mcp-proxies?limit=10&offset=0"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"

    Scenario: Get non-existent MCP proxy returns 404
        Given I authenticate using basic auth as "admin"
        When I send a GET request to the "gateway-controller" service at "/mcp-proxies/non-existent-mcp-id"
        Then the response status should be 404
        And the response should be valid JSON
        And the JSON response field "status" should be "error"

    Scenario: Get MCP proxy with invalid ID format returns 404
        Given I authenticate using basic auth as "admin"
        When I send a GET request to the "gateway-controller" service at "/mcp-proxies/invalid@mcp#id"
        Then the response status should be 404
        And the response should be valid JSON

    Scenario: Delete non-existent MCP proxy returns 404
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "non-existent-mcp-delete"
        Then the response status should be 404
        And the response should be valid JSON
        And the JSON response field "status" should be "error"

    Scenario: Update non-existent MCP proxy returns 404
        Given I authenticate using basic auth as "admin"
        When I update the MCP proxy "non-existent-mcp-update" with:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: non-existent-mcp-update
            spec:
              version: v1.0
              context: /test
              upstream:
                url: http://test:3001
            """
        Then the response status should be 404
        And the response should be valid JSON

    Scenario: Deploy an MCP Proxy with labels and verify they are stored
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: labeled-mcp-v1.0
              labels:
                environment: production
                team: mcp-team
                service: mcp-proxy
            spec:
              displayName: Labeled MCP
              version: v1.0
              context: /labeled-mcp
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds
        
        Given I authenticate using basic auth as "admin"
        When I get the MCP proxy "labeled-mcp-v1.0"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "metadata.labels.environment" should be "production"
        And the JSON response field "metadata.labels.team" should be "mcp-team"
        And the JSON response field "metadata.labels.service" should be "mcp-proxy"
        
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "labeled-mcp-v1.0"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"

    Scenario: Deploy an MCP Proxy with invalid labels (spaces in keys) should fail
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: invalid-mcp-labels-v1.0
              labels:
                "Invalid Key": value
            spec:
              displayName: Invalid MCP Labels
              version: v1.0
              context: /invalid-mcp
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be a client error
        And the response should be valid JSON
        And the JSON response field "status" should be "error"
        And the response body should contain "configuration validation failed"

    # ==================== MCP PROXY ADDITIONAL ERROR CASES ====================

    Scenario: Deploy MCP proxy with invalid JSON body returns error
        Given I authenticate using basic auth as "admin"
        When I send a POST request to the "gateway-controller" service at "/mcp-proxies" with body:
            """
            { this is not valid json content
            """
        Then the response should be a client error
        And the response should be valid JSON

    Scenario: Deploy MCP proxy with missing required fields returns error
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: incomplete-mcp-v1.0
            spec:
              displayName: Incomplete MCP
            """
        Then the response should be a client error
        And the response should be valid JSON
        And the JSON response field "status" should be "error"

    Scenario: Deploy MCP proxy declaring multiple spec versions
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: multi-spec-version-mcp-v1.0
            spec:
              displayName: Multi Spec Version MCP
              version: v1.0
              context: /multi-spec-version-mcp
              specVersions:
                - "2025-06-18"
                - "2026-07-28"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"

    Scenario: Deploy MCP proxy declaring both spec version forms returns 400
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: both-spec-version-forms-mcp-v1.0
            spec:
              displayName: Both Spec Version Forms MCP
              version: v1.0
              context: /both-spec-version-forms-mcp
              specVersion: "2025-06-18"
              specVersions:
                - "2026-07-28"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              tools: []
              resources: []
              prompts: []
            """
        Then the response status should be 400
        And the response should be valid JSON
        And the JSON response field "status" should be "error"

    # A revision this gateway does not support is a gateway limitation, not a bad configuration,
    # so the proxy deploys. Both an older and a newer revision are covered, since the gateway
    # used to reject revisions older than the ones it supports.
    Scenario Outline: Deploy MCP proxy declaring a revision this gateway does not support (<reason>)
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: unsupported-spec-version-<slug>-v1.0
            spec:
              displayName: Unsupported Spec Version <slug>
              version: v1.0
              context: /unsupported-spec-version-<slug>
              specVersions:
                - "2025-06-18"
                - "<version>"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"

        Examples:
            | slug   | version    | reason                               |
            | older  | 2025-03-26 | released before the oldest supported |
            | future | 2027-03-01 | released after this build            |

    # A proxy whose only declared revision predates the protected-resource model deploys too.
    # The upstream is present so the version is the only thing that could fail.
    Scenario: Deploy MCP proxy declaring only a legacy revision
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: legacy-spec-version-mcp-v1.0
            spec:
              displayName: Legacy Spec Version MCP
              version: v1.0
              context: /legacy-spec-version-mcp
              specVersion: "2025-03-26"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"

    # A value that is not a revision date is a typo, not a limitation, and is still rejected.
    # "banana" is the one that matters most: versions are compared as strings, and
    # "banana" >= "2025-06-18" is true, so without the date check it would read as modern.
    Scenario Outline: Deploy MCP proxy with a malformed spec version returns 400 (<reason>)
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: malformed-spec-version-<slug>-v1.0
            spec:
              displayName: Malformed Spec Version <slug>
              version: v1.0
              context: /malformed-spec-version-<slug>
              specVersions:
                - "<version>"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              tools: []
              resources: []
              prompts: []
            """
        Then the response status should be 400
        And the response should be valid JSON
        And the JSON response field "status" should be "error"
        And the response body should contain "<version>"
        And the response body should contain "expected a revision date"

        Examples:
            | slug       | version    | reason                          |
            | singledigit| 2025-6-18  | not a padded date               |
            | notaday    | 2025-13-45 | shaped like a date but is none  |
            | notadate   | banana     | not a date at all               |

    Scenario: Deploy MCP proxy without upstream returns 400
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: missing-upstream-mcp-v1.0
            spec:
              displayName: Missing Upstream MCP
              version: v1.0
              context: /missing-upstream-mcp
              specVersion: "2025-06-18"
              tools: []
              resources: []
              prompts: []
            """
        Then the response status should be 400
        And the response should be valid JSON
        And the JSON response field "status" should be "error"

    Scenario: Deploy duplicate MCP proxy returns conflict
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: duplicate-mcp-v1.0
            spec:
              displayName: Duplicate MCP
              version: v1.0
              context: /duplicate-mcp
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        # Try to create duplicate
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: duplicate-mcp-v1.0
            spec:
              displayName: Duplicate MCP
              version: v1.0
              context: /duplicate-mcp
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              tools: []
              resources: []
              prompts: []
            """
        Then the response status should be 409
        And the response should be valid JSON
        And the JSON response field "status" should be "error"
        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "duplicate-mcp-v1.0"
        Then the response should be successful

    Scenario: Update MCP proxy with invalid JSON body returns error
        Given I authenticate using basic auth as "admin"
        When I send a PUT request to the "gateway-controller" service at "/mcp-proxies/some-mcp" with body:
            """
            { invalid json body
            """
        Then the response should be a client error
        And the response should be valid JSON

    # ==================== MCP PROXY FILTER TESTS ====================

    Scenario: List MCP proxies with displayName filter
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: filter-test-mcp-v1.0
            spec:
              displayName: UniqueMCPFilterTest
              version: v1.0
              context: /filter-test-mcp
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        When I send a GET request to the "gateway-controller" service at "/mcp-proxies?displayName=UniqueMCPFilterTest"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And the response body should contain "UniqueMCPFilterTest"
        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "filter-test-mcp-v1.0"
        Then the response should be successful

    Scenario: List MCP proxies with version filter
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: version-test-mcp-v99.0
            spec:
              displayName: Version Test MCP
              version: v99.0
              context: /version-test-mcp
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        When I send a GET request to the "gateway-controller" service at "/mcp-proxies?version=v99.0"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "version-test-mcp-v99.0"
        Then the response should be successful

