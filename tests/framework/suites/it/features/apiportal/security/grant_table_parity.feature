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
  I want no shared role to be granted more by platform-api than by the portal
  So that a role is never wider where it is checked second

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml). The role-to-scope mappings staged for the
  # portal and for platform-api are compared. The portal owns scopes platform-api never checks, so
  # the portal's grants may be wider; dp_narrow_it is the one role allowed to differ the other way.

  @APIP-033
  Scenario: The portal grants every shared role what platform-api grants it, apart from the deliberate narrow-role difference
    # The portal covers every platform-api grant of the shared roles
    Then the API Portal staged role mappings should cover every control plane grant except for role "dp_narrow_it"

    # The deliberate narrow-role grant difference remains enforced
    Then the API Portal IT role "dp_narrow_it" should be read-only on the portal side
