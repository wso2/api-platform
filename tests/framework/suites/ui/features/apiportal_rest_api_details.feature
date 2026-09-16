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

Feature: API Portal REST API details

  Background:
    Given the user is signed in to the API Portal
    And the API Portal has a REST API details fixture

  Scenario: The REST API overview shows its details
    When the user opens the seeded REST API overview and sees its details

  Scenario: The REST API documentation renders its OpenAPI specification
    When the user opens the seeded REST API specification and sees its OpenAPI document

  Scenario: The REST API documentation lists the specification and additional document
    When the user opens the seeded REST API documentation and sees the additional document

  Scenario: The REST API specification exposes the Try It console
    When the seeded REST API specification exposes the Try It console
