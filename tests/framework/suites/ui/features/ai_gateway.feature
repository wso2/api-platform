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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

Feature: AI gateway lifecycle
  The journey ported from the product's own Cypress suite (003-ai-gateway), registering
  and retiring an AI gateway entirely through the UI.

  Scenario: An administrator registers an AI gateway, then removes it
    Given the user is signed in
    When the user creates the AI gateway "${UNIQUE:e2e-ai-gateway}" at "https://localhost:8443"
    Then the user is on the AI gateway's overview page
    And the user sees "${UNIQUE:e2e-ai-gateway}" on the page

    When the user opens AI Gateways
    And the user deletes the AI gateway "${UNIQUE:e2e-ai-gateway}"
    Then the user no longer sees "${UNIQUE:e2e-ai-gateway}"
