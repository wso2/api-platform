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

Feature: API Portal applications

  # Key generation used to live on the application detail page as a card per key
  # manager. That flow was removed when OAuth2 key generation moved to its own
  # page and a key manager stopped being bound to an application, so the
  # scenarios that drove those cards are gone with it. What remains here is what
  # the page still does: the application lifecycle, and the sections that
  # associate an already-created key with an application.

  Background:
    Given the user is signed in to the API Portal
    And the API Portal has an application fixture

  Scenario: An administrator creates edits and deletes an application
    When the user creates edits and deletes the application

  Scenario: An application detail page shows the key association sections
    When the application detail shows the key association sections
