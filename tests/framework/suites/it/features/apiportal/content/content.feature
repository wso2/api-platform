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


@APIPORTAL @apiportal-api-content
Feature: An API's content is uploaded and served through the API Portal REST API
  As an API publisher
  I want to upload an API's landing-page content and images, replace them and remove them
  So that developers see the API the way I describe it, and only I can change it

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-CORE-081..083 and 129..132, at the API
  # level, plus the public pages that render the content. Every content archive has one wrapper folder
  # holding web/ (landing page and images, first level only) and docs/<Section>/ (documents; a document
  # directly under docs/ is in section Other). The default fixture holds a marketing page
  # (api-content.hbs) and an image (api-icon.png). Images are public; other content needs a signed-in
  # caller. Every refused change is followed by a read showing the content is unchanged.

  Background:
    Given a REST API is created in the API Portal and stored as "apiId"

  @APIP-CORE-081 @type:smoke
  Scenario: A publisher uploads an API's content, replaces its image and removes it
    When I upload default API Portal content for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=MARKETING&fileName=api-content.hbs" as "publisher"
    Then the response status code should be 200
    And the response body should contain "API Portal content"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png" as "publisher"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "image"

    When I PUT API Portal content "alternate" for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png" as "publisher"
    Then the response status code should be 200
    And the API Portal response body should equal hex "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000a49444154789c6300010000050001aabbccdd0000000049454e44ae426082"

    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png" as "publisher"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=MARKETING&fileName=api-content.hbs" as "publisher"
    Then the response status code should be 200

  @APIP-CORE-082 @type:security
  Scenario: Only a publisher can change an API's content, and only its images are public
    Given I upload default API Portal content for API "${CTX:apiId}" as "publisher"
    And I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png" as "publisher"
    And I store the API Portal response body as "originalImage"
    When I upload default API Portal content for API "${CTX:apiId}" as "developer"
    Then the response status code should be 403
    When I PUT API Portal content "alternate" for API "${CTX:apiId}" as "developer"
    Then the response status code should be 403
    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png" as "developer"
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png" as "developer"
    Then the response status code should be 200
    And the API Portal response body should equal stored "originalImage"

    When I send an unauthenticated API Portal public "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "image"
    When I send an unauthenticated API Portal public "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png&orgId=00000000-0000-0000-0000-000000000000"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/apis/${CTX:apiId}/assets?type=MARKETING&fileName=api-content.hbs"
    Then the response status code should be 401
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=MARKETING&fileName=api-content.hbs" as "developer"
    Then the response status code should be 200

  @APIP-CORE-083 @type:negative
  Scenario: Content requests that name nothing, or repeat an upload, are refused
    Given I generate a unique resource name from "portal-missing-content-api" and store it as "missingApiId"
    And I upload default API Portal content for API "${CTX:apiId}" as "publisher"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=missing.png" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=missing.png" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:missingApiId}/assets?type=IMAGE&fileName=api-icon.png" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?fileName=api-icon.png" as "publisher"
    Then the response status code should be 400
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE" as "publisher"
    Then the response status code should be 400
    When I POST API Portal content "default" for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 409
    And the response body should not contain "constraint"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png" as "publisher"
    Then the response status code should be 200

  @APIP-CORE-129
  Scenario: An API's content gives it a landing page, an icon and documents in their sections
    When I upload API Portal content "with-docs" for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}"
    Then the response status code should be 200
    And the response body should contain "API Portal content"
    And the response body should contain "api-icon.png"
    And the response body should not contain "aov-api-avatar"

    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=DOC_HowTo&fileName=setup.md" as "developer"
    Then the response status code should be 200
    And the response body should contain "Setup guide"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=DOC_Other&fileName=overview.md" as "developer"
    Then the response status code should be 200
    And the response body should contain "This API answers ping requests."
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}/docs/HowTo/setup.md"
    Then the response status code should be 200
    And the response body should contain "Call the ping operation with your subscription token."
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}/docs/HowTo/missing.md"
    Then the response status code should be 404

    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png" as "publisher"
    Then the response status code should be 204
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}"
    Then the response status code should be 200
    And the response body should contain "aov-api-avatar"
    And the response body should contain "API Portal content"

  @APIP-CORE-130 @type:negative
  Scenario: A content upload holding files the portal does not serve is refused and stores nothing
    When I upload API Portal content "no-content-dirs" for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 400
    And the response body should contain "unknown top-level entry"
    And the response body should not contain "/tmp/"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=MARKETING&fileName=readme.txt" as "publisher"
    Then the response status code should be 404

    When I upload API Portal content "skips" for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 400
    And the response body should contain "web/ holds only"
    And the response body should not contain "/tmp/"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=MARKETING&fileName=api-content.hbs" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=MARKETING&fileName=deep.md" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=MARKETING&fileName=run.sh" as "publisher"
    Then the response status code should be 404

  @APIP-CORE-131
  Scenario: A second upload through PUT replaces everything the API held
    Given I upload default API Portal content for API "${CTX:apiId}" as "publisher"
    And the response status code should be 201
    When I PUT API Portal content "replace-and-add" for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png" as "publisher"
    Then the response status code should be 200
    And the API Portal response body should equal hex "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000a49444154789c6300010000050001aabbccdd0000000049454e44ae426082"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=DOC_HowTo&fileName=extra.md" as "publisher"
    Then the response status code should be 200
    And the response body should contain "Added by a later upload."
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=MARKETING&fileName=api-content.hbs" as "publisher"
    Then the response status code should be 404

  @APIP-CORE-132
  Scenario: An MCP server's content is uploaded, shown on its page and removed
    Given a unique API Portal MCP server with label "default" is created and stored as "mcpId"
    When I POST API Portal content "default" for MCP server "${CTX:mcpId}" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}/assets?type=MARKETING&fileName=api-content.hbs" as "publisher"
    Then the response status code should be 200
    And the response body should contain "API Portal content"
    # Like an API's images, an MCP server's images are public; its other content needs a signed-in caller.
    When I send an unauthenticated API Portal public "GET" request to "/mcp-servers/${CTX:mcpId}/assets?type=IMAGE&fileName=api-icon.png"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/mcp-servers/${CTX:mcpId}/assets?type=MARKETING&fileName=api-content.hbs"
    Then the response status code should be 401
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}/assets?type=IMAGE&fileName=api-icon.png" as "developer"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "image"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/mcp/${CTX:mcpId}"
    Then the response status code should be 200
    And the response body should contain "api-icon.png"

    When I POST API Portal content "default" for MCP server "${CTX:mcpId}" as "developer"
    Then the response status code should be 403
    When I send an authenticated API Portal "DELETE" request to "/mcp-servers/${CTX:mcpId}/assets?type=IMAGE&fileName=api-icon.png" as "publisher"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}/assets?type=IMAGE&fileName=api-icon.png" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}/assets?type=MARKETING&fileName=api-content.hbs" as "publisher"
    Then the response status code should be 200
