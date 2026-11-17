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

Feature: Signing in
  The sign-in form is the one screen every user meets. It gets a real, through-the-UI
  scenario here; every other feature starts from a saved signed-in state instead, so the
  form is exercised deliberately rather than incidentally five hundred times.

  Scenario: An administrator signs in through the form
    When the user opens the workspace
    And the user signs in as the administrator
    Then the user lands on the organization home

  Scenario: A signed-in session is reusable without the form
    Given the user is signed in
    Then the user lands on the organization home
