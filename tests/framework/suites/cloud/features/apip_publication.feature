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
Feature: APIP cloud API publishing to a managed API Portal
  As an APIP cloud user
  I want to publish, update, deprecate and unpublish an API on my managed API Portal
  So that API consumers see exactly the listing I prepared

  # Every scenario publishes to the organization's existing managed API Portal and verifies
  # the result through the control plane, which reports a listing only after the portal
  # accepted it. No API is deployed: publishing does not need a gateway.
  Background:
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project

  @TC_APIP_PUB_001 @ID_APIP_PUBLICATION_001
  Scenario: A REST API is published and unpublished, and its draft is kept
    Given I save a publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    When I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 201
    And the synthetic REST API should be PUBLISHED on the managed API Portal
    And the publication of the synthetic REST API should offer the synthetic subscription plan
    And the API publications of the synthetic REST API should show the managed API Portal as PUBLISHED
    When I unpublish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 204
    And the synthetic REST API should not be published on the managed API Portal
    And the API publications of the synthetic REST API should show the managed API Portal as NOT_PUBLISHED
    And the publication draft of the synthetic REST API should still exist
    When I unpublish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 409
    And the cloud API error code should be "PUBLICATION_STATE_CONFLICT"

  @TC_APIP_PUB_002 @ID_APIP_PUBLICATION_002
  Scenario: Republishing with a newly created plan offers it alongside the first
    Given I save a publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    And I publish the synthetic REST API on the managed API Portal
    And the cloud API response status code should be 201
    And I create another synthetic subscription plan
    When I save a publication draft of the synthetic REST API with:
      | subscriptionPlanIds | ["${CTX:cloudSubscriptionPlanID}", "${CTX:cloudSecondSubscriptionPlanID}"] |
    And I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 200
    And the publication of the synthetic REST API should offer both synthetic subscription plans

  @TC_APIP_PUB_003 @ID_APIP_PUBLICATION_003
  Scenario: Republishing an edited draft replaces the listing
    Given I save a publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    And I publish the synthetic REST API on the managed API Portal
    And the cloud API response status code should be 201
    When I save a publication draft of the synthetic REST API with:
      | displayName | Edited ${CTX:cloudAPIID}       |
      | description | Edited by the APIP cloud test. |
    And I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 200
    And the synthetic REST API should be PUBLISHED on the managed API Portal
    And the publication of the synthetic REST API should have the display name "Edited ${CTX:cloudAPIID}"

  @TC_APIP_PUB_004 @ID_APIP_PUBLICATION_004
  Scenario: Deprecating a published API changes only its status, and only once
    Given I save a publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    And I publish the synthetic REST API on the managed API Portal
    And the cloud API response status code should be 201
    When I deprecate the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 200
    And the synthetic REST API should be DEPRECATED on the managed API Portal
    And the API publications of the synthetic REST API should show the managed API Portal as DEPRECATED
    And the publication of the synthetic REST API should offer the synthetic subscription plan
    When I deprecate the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 409
    And the cloud API error code should be "PUBLICATION_STATE_CONFLICT"

  @TC_APIP_PUB_005 @ID_APIP_PUBLICATION_005
  Scenario: A document, a landing page and a thumbnail are published, and an unselected document is removed
    Given I create a synthetic document for the synthetic REST API
    And I save a publication draft of the synthetic REST API with:
      | docIds | ["${CTX:cloudDocumentID}"] |
    And I save a valid definition on the publication draft of the synthetic REST API
    And I save a landing page on the publication draft of the synthetic REST API
    And I save a PNG thumbnail on the publication draft of the synthetic REST API
    When I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 201
    And the publication of the synthetic REST API should include the synthetic document
    And the publication of the synthetic REST API should have a landing page and a thumbnail
    When I save a publication draft of the synthetic REST API with:
      | docIds | [] |
    And I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 200
    And the publication of the synthetic REST API should not include the synthetic document

  @TC_APIP_PUB_006 @ID_APIP_PUBLICATION_006
  Scenario: A GraphQL API is published, deprecated and unpublished
    Given I create a synthetic GraphQL API in the default project
    And I save a publication draft of the synthetic GraphQL API
    And I save a valid definition on the publication draft of the synthetic GraphQL API
    When I publish the synthetic GraphQL API on the managed API Portal
    Then the cloud API response status code should be 201
    And the synthetic GraphQL API should be PUBLISHED on the managed API Portal
    And the publication of the synthetic GraphQL API should offer the synthetic subscription plan
    When I deprecate the synthetic GraphQL API on the managed API Portal
    Then the cloud API response status code should be 200
    And the synthetic GraphQL API should be DEPRECATED on the managed API Portal
    When I unpublish the synthetic GraphQL API on the managed API Portal
    Then the cloud API response status code should be 204
    And the synthetic GraphQL API should not be published on the managed API Portal
    And the publication draft of the synthetic GraphQL API should still exist
