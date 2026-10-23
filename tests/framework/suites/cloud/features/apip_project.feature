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
Feature: APIP cloud project lifecycle
  As an APIP cloud user
  I want to manage a project
  So that project creation, its refusals, its pipeline binding, its quota and deletion are verified

  @TC_APIP_PROJ_002 @TC_APIP_PROJ_003 @TC_APIP_PROJ_004 @TC_APIP_PROJ_005 @ID_APIP_PROJECT_001
  Scenario: A project can be retrieved by handle and deleted
    Given I obtain an APIP cloud console token
    When I create a synthetic APIP project
    Then the synthetic APIP project should be retrievable by handle
    When I delete the synthetic APIP project
    Then the synthetic APIP project should eventually be absent

  @TC_APIP_PROJ_001 @ID_APIP_PROJECT_002
  Scenario: The default project of the organisation is resolved and active
    Given I obtain an APIP cloud console token
    When I read the default APIP project
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | id     | default |
      | status | active  |

  @TC_APIP_PROJ_006 @ID_APIP_PROJECT_003
  Scenario: A project whose handle is already in use is rejected
    Given I obtain an APIP cloud console token
    When I create a synthetic APIP project
    And I create the synthetic APIP project again under the same handle
    Then the cloud API response status code should be 409
    And the cloud API error code should be "PROJECT_EXISTS"

  @TC_APIP_PROJ_007 @ID_APIP_PROJECT_004
  Scenario Outline: A project whose handle breaks the naming rules is rejected and leaves nothing behind
    Given I obtain an APIP cloud console token
    When I request a synthetic APIP project whose handle is "<defect>"
    Then the cloud API response status code should be 400
    And the rejected APIP project should not exist

    Examples:
      | defect                   |
      | uppercase                |
      | containing an underscore |
      | starting with a hyphen   |
      | ending with a hyphen     |

    # Known issue: the platform record skips the handle rules and stores the handle in 40 characters (B7).
    @known-issue
    Examples:
      | defect                         |
      | containing consecutive hyphens |
      | longer than 40 characters      |

  @TC_APIP_PROJ_009 @ID_APIP_PROJECT_005
  Scenario: Reading a project that does not exist returns not found
    Given I obtain an APIP cloud console token
    When I read an APIP project that does not exist
    Then the cloud API response status code should be 404
    And the cloud API error code should be "PROJECT_NOT_FOUND"

  @TC_APIP_PROJ_010 @ID_APIP_PROJECT_006
  Scenario: Deleting a project that does not exist returns not found
    Given I obtain an APIP cloud console token
    When I delete an APIP project that does not exist
    Then the cloud API response status code should be 404

  @TC_APIP_PROJ_011 @ID_APIP_PROJECT_007
  Scenario: A new project is active without any manual intervention
    Given I obtain an APIP cloud console token
    When I create a synthetic APIP project
    Then the cloud API response should report:
      | status | active |
    And the synthetic APIP project should become active

  @TC_APIP_PROJ_008 @ID_APIP_PROJECT_008
  Scenario: A project beyond the organisation's quota is refused with the quota error
    Given I obtain an APIP cloud console token
    When I create synthetic APIP projects until the organisation refuses one
    Then the cloud API response status code should be 402
    And the cloud API error code should be "quota_exceeded"
    And the rejected APIP project should not exist

  @TC_APIP_PROJ_012 @ID_APIP_PROJECT_009
  Scenario: A project is bound to a deployment pipeline and the binding is readable
    Given I obtain an APIP cloud console token
    And I create 2 synthetic non-production APIP environments
    And I create a synthetic deployment pipeline with the promotion paths:
      | source                            | targets                           |
      | ${CTX:cloudSyntheticEnvironment1} | ${CTX:cloudSyntheticEnvironment2} |
    When I create a synthetic APIP project bound to the synthetic deployment pipeline
    Then the synthetic APIP project should be bound to the deployment pipeline "${CTX:cloudPipelineID}"

  @TC_APIP_PROJ_013 @ID_APIP_PROJECT_010
  Scenario Outline: Binding a project to a deployment pipeline that does not exist is refused
    Given I obtain an APIP cloud console token
    When I create a synthetic APIP project
    And I bind the synthetic APIP project to the deployment pipeline "<pipeline>"
    Then the cloud API response status code should be 400
    And the cloud API error code should be "invalid_request"
    And the cloud API response body should contain "<message>"
    And the synthetic APIP project should be bound to the deployment pipeline "default"

    Examples:
      | pipeline                       | message        |
      | ${UNIQUE:syn-no-such-pipeline} | does not exist |
      |                                | is required    |

  @TC_APIP_PROJ_014 @ID_APIP_PROJECT_011
  Scenario: A project that was never bound explicitly reads back the default pipeline
    Given I obtain an APIP cloud console token
    When I create a synthetic APIP project
    And I read the deployment pipeline of the synthetic APIP project
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | pipeline | default |
