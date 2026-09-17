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
Feature: APIP cloud managed gateway lifecycle
  As an APIP cloud user
  I want to manage a gateway in an existing environment
  So that managed gateway registration and deletion are verified

  Scenario: A managed gateway can be retrieved and deleted
    Given I obtain an APIP cloud console token
    And I select an existing APIP environment for a managed gateway
    When I create a synthetic managed APIP gateway
    Then the managed APIP gateway should be retrievable through the gateways endpoint
    When I delete the synthetic managed APIP gateway
    Then the managed APIP gateway should eventually be absent
