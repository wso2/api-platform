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

Feature: API Portal settings views and labels

  Background:
    Given the user is signed in to the API Portal

  Scenario: An administrator creates a view from a display name
    Given the API Portal has a settings label fixture
    When the user creates a view from a display name

  Scenario: An administrator renames a view from the edit modal
    Given the API Portal has a settings view fixture
    When the user renames the settings view

  Scenario: The default view has an enabled delete control
    Given the API Portal has a settings view fixture
    When the default view has an enabled delete control

  Scenario: An administrator creates a label from a display name
    When the user creates a label from a display name
