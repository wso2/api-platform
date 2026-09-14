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

Feature: Custom LLM provider template lifecycle
  The journey ported from the product's own Cypress suite (005-custom-provider-template),
  creating a custom template, adding a second version, using it to create a provider, and
  confirming a version still referenced by a provider cannot be deleted until that provider
  is removed.

  Scenario: An administrator versions a custom template, uses it for a provider, and is blocked from deleting the version in use
    Given the user is signed in
    When the user creates the LLM provider template "${UNIQUE:E2E-Custom-Template}" at "https://api.e2e-custom-template.example.com"
    And the user opens the LLM provider template "${UNIQUE:E2E-Custom-Template}"
    Then the user sees "${UNIQUE:E2E-Custom-Template}" on the page
    And the user sees a "v1.0" version button

    When the user creates version "v2.0" of the template from version "v1.0" at "https://api.e2e-custom-template.example.com"
    Then the user sees "${UNIQUE:E2E-Custom-Template}" on the page
    And the user sees a "v2.0" version button

    When the user creates the provider "${UNIQUE:E2E-Custom-Template-Provider}" from the "${UNIQUE:E2E-Custom-Template}" template's "v2.0" version
    Then the user is on the provider's overview page
    And the user sees "${UNIQUE:E2E-Custom-Template-Provider}" on the page

    When the user opens the LLM provider template "${UNIQUE:E2E-Custom-Template}"
    And the user attempts to delete the current template version
    Then the user sees "Cannot delete: one or more providers were created from this template." on the page

    When the user deletes the provider "${UNIQUE:E2E-Custom-Template-Provider}" directly
    And the user deletes the template's "v2.0" version
    Then the user sees a "v1.0" version button

    When the user deletes the template's "v1.0" version
    Then the user no longer sees "${UNIQUE:E2E-Custom-Template}"
