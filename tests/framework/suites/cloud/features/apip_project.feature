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
  So that project creation and deletion are verified

  Scenario: A project can be retrieved by handle and deleted
    Given I obtain an APIP cloud console token
    When I create a synthetic APIP project
    Then the synthetic APIP project should be retrievable by handle
    When I delete the synthetic APIP project
    Then the synthetic APIP project should eventually be absent
