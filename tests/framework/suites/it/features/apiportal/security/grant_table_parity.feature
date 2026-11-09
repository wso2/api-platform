# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License. You may obtain a copy of the License at
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


@APIPORTAL @apiportal-security
Feature: API Portal authorization grant-table parity
  As an API Portal operator who maps the same roles in the portal and in platform-api
  I want each shared role to grant the same permissions on both sides
  So that a role means the same thing wherever it is checked

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml). The role-to-scope
  # mappings staged for the portal and for platform-api are compared; dp_narrow_it is the one
  # documented difference.

  @APIP-033
  Scenario: Shared roles have identical grants on both components, apart from the deliberate narrow-role difference
    # Shared IT roles have identical grants on both component sides
    Then the API Portal IT role mappings should match except for role "dp_narrow_it"

    # The deliberate narrow-role grant difference remains enforced
    Then the API Portal IT role "dp_narrow_it" should be read-only on the portal side
