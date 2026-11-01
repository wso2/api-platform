# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except in compliance
# with the License. You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# --------------------------------------------------------------------

@apip-cloud
Feature: APIP cloud publication drafts and the checks made before publishing
  As an APIP cloud user
  I want a publication draft to keep what I prepared and to be checked before anything is sent
  So that a listing never reaches my managed API Portal with missing or wrong content

  # Nothing here reaches the managed API Portal: a draft stays on the control plane, and every
  # publish is refused before the portal is called.
  Background:
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project

  @TC_APIP_PUBV_001 @ID_APIP_PUBLICATION_007
  Scenario: A saved draft keeps its details, its definition and a landing page without raw HTML
    When I save a publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    And I save a landing page on the publication draft of the synthetic REST API
    Then the publication draft of the synthetic REST API should keep its details and definition
    And the landing page of the publication draft of the synthetic REST API should have no raw HTML

  @TC_APIP_PUBV_002 @ID_APIP_PUBLICATION_008
  Scenario Outline: A draft naming something that does not exist is refused
    When I try to save a publication draft naming an unknown <reference>
    Then the cloud API response status code should be <status>
    And the cloud API error code should be "<code>"

    Examples:
      | reference         | status | code                          |
      | subscription plan | 400    | PUBLICATION_VALIDATION_FAILED |
      | API               | 404    | API_NOT_FOUND                 |
      | API Portal        | 404    | API_PORTAL_NOT_FOUND          |

  @TC_APIP_PUBV_003 @ID_APIP_PUBLICATION_009
  Scenario: Publishing needs a saved draft with a valid definition
    When I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 404
    And the cloud API error code should be "DRAFT_NOT_FOUND"
    When I save a publication draft of the synthetic REST API
    And I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 400
    And the cloud API error code should be "PUBLICATION_VALIDATION_FAILED"
    When I save an invalid definition on the publication draft of the synthetic REST API
    And I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 400
    And the cloud API error code should be "PUBLICATION_VALIDATION_FAILED"
    And the synthetic REST API should not be published on the managed API Portal

  @TC_APIP_PUBV_004 @ID_APIP_PUBLICATION_010
  Scenario Outline: An API that was never published can be neither unpublished nor deprecated
    When I <action> the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 409
    And the cloud API error code should be "PUBLICATION_STATE_CONFLICT"

    Examples:
      | action    |
      | unpublish |
      | deprecate |

  @TC_APIP_PUBV_005 @ID_APIP_PUBLICATION_011
  Scenario: An inactive subscription plan cannot be offered, even when it was active when the draft was saved
    Given I save a publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    When I deactivate the synthetic subscription plan
    And I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 400
    And the cloud API error code should be "PUBLICATION_VALIDATION_FAILED"
    And the synthetic REST API should not be published on the managed API Portal
    When I try to save a publication draft of the synthetic REST API
    Then the cloud API response status code should be 400
    And the cloud API error code should be "PUBLICATION_VALIDATION_FAILED"

  @TC_APIP_PUBV_006 @ID_APIP_PUBLICATION_012
  Scenario: A subscription plan offered by a draft cannot be deleted
    Given I save a publication draft of the synthetic REST API
    When I try to delete the synthetic subscription plan
    Then the cloud API response status code should be 409
    And the cloud API error code should be "SUBSCRIPTION_PLAN_IN_USE"

  @TC_APIP_PUBV_007 @ID_APIP_PUBLICATION_013
  Scenario: A GraphQL draft is published only through its own route, and only with a GraphQL definition
    Given I create a synthetic GraphQL API in the default project
    And I save a publication draft of the synthetic GraphQL API
    When I save an OpenAPI definition on the publication draft of the synthetic GraphQL API
    And I publish the synthetic GraphQL API through the REST API route
    Then the cloud API response status code should be 404
    And the cloud API error code should be "API_NOT_FOUND"
    When I publish the synthetic GraphQL API on the managed API Portal
    Then the cloud API response status code should be 400
    And the cloud API error code should be "PUBLICATION_VALIDATION_FAILED"
    And the synthetic GraphQL API should not be published on the managed API Portal

  @TC_APIP_PUBV_008 @ID_APIP_PUBLICATION_014
  Scenario Outline: Publication routes refuse a request <credential>
    When I send a "<method>" request <credential> to the publication path "<path>"
    Then the cloud API response status code should be 401

    Examples:
      | method | credential          | path                                                                                    |
      | GET    | without a token     | /api-publications?apiType=rest-api&apiId=${CTX:cloudAPIID}                              |
      | PUT    | without a token     | /api-portals/${CTX:cloudAPIPortalID}/apis/rest-api/${CTX:cloudAPIID}/draft              |
      | POST   | without a token     | /api-portals/${CTX:cloudAPIPortalID}/apis/rest-api/${CTX:cloudAPIID}/publish            |
      | POST   | without a token     | /api-portals/${CTX:cloudAPIPortalID}/apis/rest-api/${CTX:cloudAPIID}/unpublish          |
      | GET    | with a forged token | /api-publications?apiType=rest-api&apiId=${CTX:cloudAPIID}                              |
      | PUT    | with a forged token | /api-portals/${CTX:cloudAPIPortalID}/apis/rest-api/${CTX:cloudAPIID}/draft              |
      | POST   | with a forged token | /api-portals/${CTX:cloudAPIPortalID}/apis/graphql-api/${CTX:cloudAPIID}/publish         |
      | POST   | with a forged token | /api-portals/${CTX:cloudAPIPortalID}/apis/rest-api/${CTX:cloudAPIID}/deprecate          |
