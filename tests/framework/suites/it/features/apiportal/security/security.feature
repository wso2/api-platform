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

Feature: API Portal response security

  Scenario: The API Portal does not expose its framework identity on a public route
    When I send an unauthenticated API Portal "GET" request to "/health"
    Then the response header "X-Powered-By" should not exist

  Scenario: The API Portal does not expose its framework identity on an API route
    When I send an unauthenticated API Portal "GET" request to "/organizations/default"
    Then the response header "X-Powered-By" should not exist
