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

Feature: API Portal API content

  Scenario: A publisher uploads and retrieves API marketing content
    Given a REST API is created in the API Portal and stored as "apiId"
    And I upload default API Portal content for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=MARKETING&fileName=api-content.hbs" as "publisher"
    Then the response status code should be 200
    And the response body should contain "API Portal content"

  Scenario: A publisher retrieves an uploaded API image
    Given a REST API is created in the API Portal and stored as "apiId"
    And I upload default API Portal content for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png" as "publisher"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "image"

  Scenario: A publisher replaces an API image and the bytes are preserved
    Given a REST API is created in the API Portal and stored as "apiId"
    And I upload default API Portal content for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 201
    When I PUT API Portal content "alternate" for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png" as "publisher"
    Then the response status code should be 200
    And the API Portal response body should equal hex "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000a49444154789c6300010000050001aabbccdd0000000049454e44ae426082"

  Scenario: A publisher can map an image explicitly during content upload
    Given a REST API is created in the API Portal and stored as "apiId"
    When I upload API Portal content "image-metadata" for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png" as "publisher"
    Then the response status code should be 200

  Scenario: A missing API content asset returns not found
    Given a REST API is created in the API Portal and stored as "apiId"
    And I upload default API Portal content for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=missing.png" as "publisher"
    Then the response status code should be 404

  Scenario: A developer cannot upload API content
    Given a REST API is created in the API Portal and stored as "apiId"
    When I upload default API Portal content for API "${CTX:apiId}" as "developer"
    Then the response status code should be 403

  Scenario: An unauthenticated caller cannot read private API marketing content
    Given a REST API is created in the API Portal and stored as "apiId"
    And I upload default API Portal content for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 201
    When I send an unauthenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=MARKETING&fileName=api-content.hbs"
    Then the response status code should be 401

  Scenario: An anonymous caller can read a public API image
    Given a REST API is created in the API Portal and stored as "apiId"
    And I upload default API Portal content for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "image"

  Scenario: An anonymous image request ignores a foreign organization identifier
    Given a REST API is created in the API Portal and stored as "apiId"
    And I upload default API Portal content for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png&orgId=00000000-0000-0000-0000-000000000000"
    Then the response status code should be 200

  Scenario: An anonymous marketing-content request remains protected
    Given a REST API is created in the API Portal and stored as "apiId"
    And I upload default API Portal content for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/apis/${CTX:apiId}/assets?type=MARKETING&fileName=api-content.hbs"
    Then the response status code should be 401

  Scenario: A publisher deletes one API content asset
    Given a REST API is created in the API Portal and stored as "apiId"
    And I upload default API Portal content for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png" as "publisher"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png" as "publisher"
    Then the response status code should be 404

  Scenario: A read-only developer can read a public API image
    Given a REST API is created in the API Portal and stored as "apiId"
    And I upload default API Portal content for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=IMAGE&fileName=api-icon.png" as "developer"
    Then the response status code should be 200
