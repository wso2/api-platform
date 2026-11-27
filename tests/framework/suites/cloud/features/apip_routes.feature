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

# Every scenario deploys a synthetic REST API with the operations under test to the managed gateway
# and invokes it through the gateway's own host. The testbench backend reports the path and method it
# received; it sits behind another gateway that also merges duplicate slashes, so a normalization
# case shows the managed gateway's own handling by matching the exact route at all.
@apip-cloud
Feature: APIP cloud route and path matching
  As an APIP cloud user
  I want the managed gateway to route each request to the operation it names, and only that one
  So that a variant spelling cannot reach an operation without its policies, and the backend gets the path I expect

  # The cloud counterpart of on-premises "Wildcard path /* matches requests with a subpath".
  @ID_APIP_ROUTES_001
  Scenario: Wildcard path /* matches requests with a subpath
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"get","request":{"method":"GET","path":"/*"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/us/seattle" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/us/seattle"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | path | /us/seattle |
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/data"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | path | /data |

  # The cloud counterpart of on-premises "Wildcard path /* enforces HTTP method".
  @ID_APIP_ROUTES_002 @type:negative
  Scenario: Wildcard path /* enforces HTTP method
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"get","request":{"method":"GET","path":"/*"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/data" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/data"
    Then the cloud API response status code should be 200
    When I send a "POST" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/data"
    Then the cloud API response status code should be 404

  # The cloud counterpart of on-premises "Root path / matches request with trailing slash".
  @ID_APIP_ROUTES_003
  Scenario: Root path / matches a request with a trailing slash
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"get","request":{"method":"GET","path":"/"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | path | / |

  # The cloud counterpart of on-premises "Root path / matches request without trailing slash".
  @ID_APIP_ROUTES_004
  Scenario: Root path / matches a request without a trailing slash
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"get","request":{"method":"GET","path":"/"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1"
    Then the cloud API response status code should be 200

  # The cloud counterpart of on-premises "Wildcard /* does not match a sibling context prefix".
  @ID_APIP_ROUTES_005 @type:negative
  Scenario: Wildcard /* does not match a sibling context prefix
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"get","request":{"method":"GET","path":"/*"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/data" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/data"
    Then the cloud API response status code should be 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1"
    Then the cloud API response status code should be 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1beta/data"
    Then the cloud API response status code should be 404

  # The cloud counterpart of on-premises "Exact path matches both with and without trailing slash".
  @ID_APIP_ROUTES_006
  Scenario: Exact path matches both with and without a trailing slash
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"getWeather","request":{"method":"GET","path":"/weather"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/weather" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/weather"
    Then the cloud API response status code should be 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/weather/"
    Then the cloud API response status code should be 200

  # The cloud counterpart of on-premises "Exact path preserves trailing slash to upstream".
  @ID_APIP_ROUTES_007
  Scenario: Exact path preserves a trailing slash to the upstream
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    And an echo backend is available for deployment overrides
    When I create a synthetic REST API in the APIP test project with:
      | upstreamUrl | ${CTX:cloudEchoBackendURL}/anything |
      | operations  | [{"name":"getWeather","request":{"method":"GET","path":"/weather"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/weather" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/weather"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | url | /anything/weather |
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/weather/"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | url | /anything/weather/ |

  # The cloud counterpart of on-premises "Wildcard /foo/* preserves the matched prefix (and method) on the upstream path".
  @ID_APIP_ROUTES_008
  Scenario: Wildcard /foo/* preserves the matched prefix and method on the upstream path
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"getForecast","request":{"method":"GET","path":"/forecast/*"}},{"name":"putPut","request":{"method":"PUT","path":"/put/*"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/forecast" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/forecast"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | path | /forecast |
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/forecast/today"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | path | /forecast/today |
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/forecast/a/b/c"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | path | /forecast/a/b/c |
    When I send a "PUT" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/put"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | path | /put |
      | method | PUT |

  # The cloud counterpart of on-premises PATHV-001.
  @ID_APIP_ROUTES_009 @type:negative
  Scenario: A case variant of an operation's path gets 404 instead of reaching the wildcard operation
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"get","request":{"method":"GET","path":"/*"}},{"name":"getSecureData","request":{"method":"GET","path":"/secure/data"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/secure/data" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/secure/data"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | path | /secure/data |
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/Secure/data"
    Then the cloud API response status code should be 404
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/SECURE/DATA"
    Then the cloud API response status code should be 404
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/other"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | path | /other |

  # The cloud counterpart of on-premises PATHV-002.
  @ID_APIP_ROUTES_010 @type:negative
  Scenario: Case variants of several operations are all refused while their parameters keep any case
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"get","request":{"method":"GET","path":"/*"}},{"name":"getSecureData","request":{"method":"GET","path":"/secure/data"}},{"name":"getAlertsActive","request":{"method":"GET","path":"/alerts/active"}},{"name":"getOrdersOpen","request":{"method":"GET","path":"/orders/open"}},{"name":"getItemsId","request":{"method":"GET","path":"/items/{id}"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/orders/open" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/secure/data"
    Then the cloud API response status code should be 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/alerts/active"
    Then the cloud API response status code should be 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/items/ABC"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | path | /items/ABC |
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/SECURE/DATA"
    Then the cloud API response status code should be 404
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/Alerts/Active"
    Then the cloud API response status code should be 404
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/orders/OPEN"
    Then the cloud API response status code should be 404
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/Items/abc"
    Then the cloud API response status code should be 404
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/other"
    Then the cloud API response status code should be 200

  # The cloud counterpart of on-premises PATHV-003.
  @ID_APIP_ROUTES_011
  Scenario: An operation spelt with capitals keeps its own route next to a lowercase one
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"get","request":{"method":"GET","path":"/*","policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Matched-Operation","value":"wildcard"}]}}}]}},{"name":"getUsers","request":{"method":"GET","path":"/users","policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Matched-Operation","value":"lowercase"}]}}}]}},{"name":"getUsersCapitalised","request":{"method":"GET","path":"/Users","policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Matched-Operation","value":"capitalised"}]}}}]}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/users" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/users"
    Then the cloud API response status code should be 200
    And the response header "X-Matched-Operation" should be "lowercase"
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/Users"
    Then the cloud API response status code should be 200
    And the response header "X-Matched-Operation" should be "capitalised"
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/USERS"
    Then the cloud API response status code should be 404

  # The cloud counterpart of on-premises PATHV-004.
  @ID_APIP_ROUTES_012 @type:negative
  Scenario: A case variant is refused only for the methods of the operation it names
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"get","request":{"method":"GET","path":"/*"}},{"name":"getSecureData","request":{"method":"GET","path":"/secure/data"}},{"name":"post","request":{"method":"POST","path":"/*","policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Matched-Operation","value":"post-wildcard"}]}}}]}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/secure/data" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/Secure/data"
    Then the cloud API response status code should be 404
    When I send a "POST" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/Secure/data"
    Then the cloud API response status code should be 200
    And the response header "X-Matched-Operation" should be "post-wildcard"

  # The cloud counterpart of on-premises PATHV-005. The enclosing API's /* operation also answers the
  # inner path, so the scenario waits for the inner operation's own header before it checks the variant.
  @ID_APIP_ROUTES_013 @type:negative
  Scenario: A case variant is refused when another API's wildcard operation would catch it
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"get","request":{"method":"GET","path":"/*","policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Matched-Operation","value":"outer-wildcard"}]}}}]}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    When I create a new version "2.0.0" of the synthetic REST API with:
      | context    | /${CTX:cloudAPIID}/v1/inner/v1 |
      | operations | [{"name":"getSecureData","request":{"method":"GET","path":"/secure/data","policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Matched-Operation","value":"inner"}]}}}]}}] |
    And I deploy the new API version to the active gateway
    Then the new API version deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/inner/v1/secure/data" until the response header "X-Matched-Operation" is "inner"
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/other"
    Then the cloud API response status code should be 200
    And the response header "X-Matched-Operation" should be "outer-wildcard"
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/inner/v1/Secure/data"
    Then the cloud API response status code should be 404

  # The cloud counterpart of on-premises "Default normalization resolves dot-segments and merges duplicate slashes".
  @ID_APIP_ROUTES_014
  Scenario: Default normalization resolves dot-segments and merges duplicate slashes
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"getWeather","request":{"method":"GET","path":"/weather"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/other/../weather" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/other/../weather"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | path | /weather |
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1//weather"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | path | /weather |

  # The cloud counterpart of on-premises "Escaped slash (%2F) in a resource path segment is kept
  # unchanged and is not treated as a path separator". The cloud's edge answers a path with %2F with
  # its own 307 to the unescaped path, before the gateway sees it (see the scenario list entry).
  @ID_APIP_ROUTES_015 @type:negative @known-issue
  Scenario: An escaped slash in a path segment is kept and is not a path separator
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"getPetsName","request":{"method":"GET","path":"/pets/{name}"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/pets/pet%2FFarm" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/pets/pet%2FFarm"
    Then the cloud API response status code should be 200
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/pets/pet/Farm" until status 404

  # The cloud counterpart of on-premises "A path segment containing a literal dot is preserved by normalization".
  @ID_APIP_ROUTES_016
  Scenario: A path segment containing a literal dot is preserved by normalization
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"getPetApi","request":{"method":"GET","path":"/pet.api"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/pet.api" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/pet.api"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | path | /pet.api |
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/./pet.api"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | path | /pet.api |

  # The cloud counterpart of on-premises "Deploy API with invalid URL scheme in direct sandbox url should fail".
  # The cloud's destination guard refuses the URL when the API is created, naming the field.
  @ID_APIP_ROUTES_017 @type:negative
  Scenario: A REST API whose sandbox upstream URL is not http or https is refused
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    And an echo backend is available for deployment overrides
    When I submit a synthetic REST API creation request with:
      | upstream | {"main":{"url":"${CTX:cloudEchoBackendURL}"},"sandbox":{"url":"ftp://sandbox.example.com/sandbox"}} |
    Then the cloud API response status code should be 400
    And the cloud API error code should be "DESTINATION_NOT_ALLOWED"
    And the cloud API error should name the field "upstream.sandbox.url"
    And the submitted synthetic REST API should not exist

  # The cloud counterpart of on-premises "Missing resilience configuration uses the gateway default timeout".
  @ID_APIP_ROUTES_018
  Scenario: A backend slower than an API sets no timeout for is waited for up to the gateway default
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"getGet","request":{"method":"GET","path":"/get"}},{"name":"getDelay2","request":{"method":"GET","path":"/delay/2"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/get" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/delay/2"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | path | /delay/2 |

  # The cloud counterpart of on-premises "RestApi backend timeout uses the gateway global default".
  # On development the client gets the cloud edge's 504 "upstream request timeout" after 15 s instead
  # (see the scenario list entry).
  @ID_APIP_ROUTES_019 @type:negative @known-issue
  Scenario: An upstream that cannot be connected to times out after the gateway default and answers 503
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"get","request":{"method":"GET","path":"/"}}] |
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment}                  |
      | gatewayId   | ${CTX:cloudGatewayID}                           |
      | parameters  | {"productionEndpoint":"http://192.0.2.1:8080"} |
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/" until it times out after 5 seconds with status 503
