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

# Every scenario deploys a synthetic REST API with the policies under test to the managed gateway
# and invokes it through the gateway's own host. The testbench backend reports the request headers it
# received, which is how a request-side policy's effect is seen.
@apip-cloud
Feature: APIP cloud policies and conditions
  As an APIP cloud user
  I want the policies I attach to an API or an operation to take effect, and only when their conditions hold
  So that the gateway applies exactly the behaviour I configured

  # The cloud counterpart of on-premises "Deploy API without any policies".
  @ID_APIP_POLICIES_001
  Scenario: An API without any policies deploys and is served
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"getTest","request":{"method":"GET","path":"/test"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/test" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/test"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | path | /test |

  # The cloud counterpart of on-premises "Deploy API with operation-level policy"; the clone also checks the policy's effect.
  @ID_APIP_POLICIES_002
  Scenario: An API with an operation-level CORS policy deploys and allows any origin on that operation
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"getTest","request":{"method":"GET","path":"/test","policies":[{"name":"cors","params":{"allowedOrigins":["*"],"allowedMethods":["GET","POST"],"allowedHeaders":["*"]},"version":"v1"}]}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/test" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/test" with the header "Origin" set to "https://app.example.com"
    Then the cloud API response status code should be 200
    And the response header "Access-Control-Allow-Origin" should be "*"

  # The cloud counterpart of on-premises "Deploy API with API-level policy"; the clone also checks the policy's effect.
  @ID_APIP_POLICIES_003
  Scenario: An API with an API-level CORS policy deploys and allows its configured origin
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | policies   | [{"name":"cors","params":{"allowedOrigins":["https://app.example.com"],"allowedMethods":["GET"],"allowedHeaders":["Content-Type"]},"version":"v1"}] |
      | operations | [{"name":"getTest","request":{"method":"GET","path":"/test"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/test" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/test" with the header "Origin" set to "https://app.example.com"
    Then the cloud API response status code should be 200
    And the response header "Access-Control-Allow-Origin" should be "https://app.example.com"

  # The cloud counterpart of on-premises "Update API to add policies"; the clone redeploys the API and checks the policy's effect.
  @ID_APIP_POLICIES_004
  Scenario: A policy added by an update takes effect once the API is redeployed
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"getTest","request":{"method":"GET","path":"/test"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/test" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/test"
    Then the cloud API response status code should be 200
    And the response header "X-Policy-Applied" should not be "true"
    When I update the synthetic REST API with:
      | policies   | [{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Policy-Applied","value":"true"}]}}}] |
      | operations | [{"name":"getTest","request":{"method":"GET","path":"/test"}}] |
    Then the cloud API response status code should be 200
    When I redeploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/test" until the response header "X-Policy-Applied" is "true"

  # The cloud counterpart of on-premises "Update API to remove policies"; the clone redeploys the API and checks the policy's effect.
  @ID_APIP_POLICIES_005
  Scenario: A policy removed by an update stops applying once the API is redeployed
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | policies   | [{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Policy-Applied","value":"true"}]}}}] |
      | operations | [{"name":"getTest","request":{"method":"GET","path":"/test"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/test" until the response header "X-Policy-Applied" is "true"
    When I update the synthetic REST API with:
      | policies   | [] |
      | operations | [{"name":"getTest","request":{"method":"GET","path":"/test"}}] |
    Then the cloud API response status code should be 200
    When I redeploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/test" until the response header "X-Policy-Applied" is ""
    And the cloud API response status code should be 200

  # The cloud counterpart of on-premises "Deploy API with API-level policy using empty version resolves to latest".
  @ID_APIP_POLICIES_006
  Scenario: An API-level policy given without a version resolves to its latest version
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | policies   | [{"name":"cors","params":{"allowedOrigins":["https://app.example.com"],"allowedMethods":["GET"],"allowedHeaders":["Content-Type"]}}] |
      | operations | [{"name":"getCity","request":{"method":"GET","path":"/{country_code}/{city}"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/us/seattle" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/us/seattle" with the header "Origin" set to "https://app.example.com"
    Then the cloud API response status code should be 200
    And the response header "Access-Control-Allow-Origin" should be "https://app.example.com"

  # The cloud counterpart of on-premises "Deploy API with operation-level policy using empty version resolves to latest".
  @ID_APIP_POLICIES_007
  Scenario: An operation-level policy given without a version resolves to its latest version
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"getCity","request":{"method":"GET","path":"/{country_code}/{city}","policies":[{"name":"cors","params":{"allowedOrigins":["https://app.example.com"],"allowedMethods":["GET"],"allowedHeaders":["Content-Type"]}}]}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/us/seattle" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/us/seattle" with the header "Origin" set to "https://app.example.com"
    Then the cloud API response status code should be 200
    And the response header "Access-Control-Allow-Origin" should be "https://app.example.com"

  # The cloud counterpart of on-premises "Deploy API with different HTTP methods"; the clone also invokes each operation.
  @ID_APIP_POLICIES_008
  Scenario: An API with GET, POST, PUT, DELETE and PATCH operations serves each method
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"get","request":{"method":"GET","path":"/resource"}},{"name":"post","request":{"method":"POST","path":"/resource"}},{"name":"put","request":{"method":"PUT","path":"/resource"}},{"name":"delete","request":{"method":"DELETE","path":"/resource"}},{"name":"patch","request":{"method":"PATCH","path":"/resource"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/resource" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/resource"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | method | GET |
    When I send a "POST" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/resource" with a JSON body of 32 bytes
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | method | POST |
    When I send a "PUT" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/resource" with a JSON body of 32 bytes
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | method | PUT |
    When I send a "DELETE" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/resource"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | method | DELETE |
    When I send a "PATCH" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/resource" with a JSON body of 32 bytes
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | method | PATCH |

  # The cloud counterpart of on-premises "Policy executes only on POST requests using a method condition".
  @ID_APIP_POLICIES_009
  Scenario: A policy with a method condition runs only on POST requests
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"getGet","request":{"method":"GET","path":"/get"}},{"name":"getResource","request":{"method":"GET","path":"/resource","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Cel-Executed","value":"true"}]}},"executionCondition":"request.Method == \"POST\""}]}},{"name":"postResource","request":{"method":"POST","path":"/resource","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Cel-Executed","value":"true"}]}},"executionCondition":"request.Method == \"POST\""}]}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/get" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/resource"
    Then the cloud API response status code should be 200
    And the cloud API response should not report "headers.X-Cel-Executed"
    When I send a "POST" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/resource" with a JSON body of 32 bytes
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | headers.X-Cel-Executed | ["true"] |

  # The cloud counterpart of on-premises "Policy executes on multiple methods using the in operator".
  @ID_APIP_POLICIES_010
  Scenario: A policy with an in condition runs on each listed method
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"getGet","request":{"method":"GET","path":"/get"}},{"name":"getData","request":{"method":"GET","path":"/data","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Write-Operation","value":"true"}]}},"executionCondition":"request.Method in [\"POST\", \"PUT\", \"DELETE\"]"}]}},{"name":"postData","request":{"method":"POST","path":"/data","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Write-Operation","value":"true"}]}},"executionCondition":"request.Method in [\"POST\", \"PUT\", \"DELETE\"]"}]}},{"name":"putData","request":{"method":"PUT","path":"/data","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Write-Operation","value":"true"}]}},"executionCondition":"request.Method in [\"POST\", \"PUT\", \"DELETE\"]"}]}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/get" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/data"
    Then the cloud API response status code should be 200
    And the cloud API response should not report "headers.X-Write-Operation"
    When I send a "POST" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/data" with a JSON body of 32 bytes
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | headers.X-Write-Operation | ["true"] |
    When I send a "PUT" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/data" with a JSON body of 32 bytes
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | headers.X-Write-Operation | ["true"] |

  # The cloud counterpart of on-premises "Policy executes only when a specific header is present".
  @ID_APIP_POLICIES_011
  Scenario: A policy runs only when a specific header is present
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"getGet","request":{"method":"GET","path":"/get"}},{"name":"getCheck","request":{"method":"GET","path":"/check","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Token-Validated","value":"true"}]}},"executionCondition":"\"x-special-token\" in request.Headers"}]}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/get" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/check"
    Then the cloud API response status code should be 200
    And the cloud API response should not report "headers.X-Token-Validated"
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/check" with the header "X-Special-Token" set to "my-secret-token"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | headers.X-Token-Validated | ["true"] |

  # The cloud counterpart of on-premises "Policy executes based on a header value".
  @ID_APIP_POLICIES_012
  Scenario: A policy runs based on a header value
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"getGet","request":{"method":"GET","path":"/get"}},{"name":"getPremium","request":{"method":"GET","path":"/premium","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Premium-Access","value":"granted"}]}},"executionCondition":"\"x-tier\" in request.Headers && request.Headers[\"x-tier\"][0] == \"premium\""}]}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/get" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/premium" with the header "X-Tier" set to "basic"
    Then the cloud API response status code should be 200
    And the cloud API response should not report "headers.X-Premium-Access"
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/premium" with the header "X-Tier" set to "premium"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | headers.X-Premium-Access | ["granted"] |

  # The cloud counterpart of on-premises "Policy executes based on a path prefix".
  @ID_APIP_POLICIES_013
  Scenario: A policy runs based on a path prefix
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"getGet","request":{"method":"GET","path":"/get"}},{"name":"getPublicInfo","request":{"method":"GET","path":"/public/info","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Admin-Request","value":"true"}]}},"executionCondition":"request.Path.startsWith(\"/${CTX:cloudAPIID}/v1/admin\")"}]}},{"name":"getAdminSettings","request":{"method":"GET","path":"/admin/settings","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Admin-Request","value":"true"}]}},"executionCondition":"request.Path.startsWith(\"/${CTX:cloudAPIID}/v1/admin\")"}]}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/get" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/public/info"
    Then the cloud API response status code should be 200
    And the cloud API response should not report "headers.X-Admin-Request"
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/admin/settings"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | headers.X-Admin-Request | ["true"] |

  # The cloud counterpart of on-premises "Policy executes only when combined method and header conditions both hold".
  @ID_APIP_POLICIES_014
  Scenario: A policy runs only when combined method and header conditions both hold
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | operations | [{"name":"getGet","request":{"method":"GET","path":"/get"}},{"name":"getSecure","request":{"method":"GET","path":"/secure","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Secure-Write","value":"authorized"}]}},"executionCondition":"request.Method == \"POST\" && \"x-auth-token\" in request.Headers"}]}},{"name":"postSecure","request":{"method":"POST","path":"/secure","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Secure-Write","value":"authorized"}]}},"executionCondition":"request.Method == \"POST\" && \"x-auth-token\" in request.Headers"}]}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/get" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/secure"
    Then the cloud API response status code should be 200
    And the cloud API response should not report "headers.X-Secure-Write"
    When I send a "POST" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/secure" with a JSON body of 32 bytes
    Then the cloud API response status code should be 200
    And the cloud API response should not report "headers.X-Secure-Write"
    When I send a "POST" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/secure" with the header "X-Auth-Token" set to "valid-token"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | headers.X-Secure-Write | ["authorized"] |

  # The cloud counterpart of on-premises "secret template in set-headers policy value is rendered upstream but unrendered in response and stored configuration". The gateway's stored configuration cannot be read in the cloud, so the clone checks the API's create and read answers.
  @ID_APIP_POLICIES_015
  Scenario: A secret template in a set-headers value is rendered for the upstream but stays unrendered in the API
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    And an echo backend is available for deployment overrides
    And I create a synthetic secret
    When I create a synthetic REST API in the APIP test project with:
      | upstreamUrl | ${CTX:cloudEchoBackendURL} |
      | operations  | [{"name":"probe","request":{"method":"GET","path":"/probe","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Auth-Token","value":"Bearer ${CTX:cloudSecretReference}"}]}}}]}}] |
    And the cloud API response body should contain "{{ secret"
    And the cloud API response body should contain "${CTX:cloudSecretID}"
    And the cloud API response body should not contain "${CTX:cloudSecretValue}"
    When I read the synthetic REST API
    Then the cloud API response status code should be 200
    And the cloud API response body should contain "{{ secret"
    And the cloud API response body should not contain "${CTX:cloudSecretValue}"
    When I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/probe" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/probe"
    Then the cloud API response status code should be 200
    And the echoed request header "X-Auth-Token" should be "Bearer ${CTX:cloudSecretValue}"

  # The cloud counterpart of on-premises "default function returns fallback when env is missing". The gateway's stored configuration cannot be read in the cloud, so the clone checks the API's read answer.
  # On development the deployment fails with GATEWAY_PROCESSING_ERROR (see the scenario list entry).
  @ID_APIP_POLICIES_016 @known-issue
  Scenario: The default function supplies a fallback when an environment value is missing
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    And an echo backend is available for deployment overrides
    When I create a synthetic REST API in the APIP test project with:
      | upstreamUrl | ${CTX:cloudEchoBackendURL} |
      | operations  | [{"name":"probe","request":{"method":"GET","path":"/probe","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Fallback","value":"{{ env \"IT_DEFINITELY_MISSING_KEY\" \| default \"fallback-value\" }}"}]}}}]}}] |
    When I read the synthetic REST API
    Then the cloud API response status code should be 200
    And the cloud API response body should contain "{{ env"
    When I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/probe" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/probe"
    Then the cloud API response status code should be 200
    And the echoed request header "X-Fallback" should be "fallback-value"

  # The cloud counterpart of on-premises "Deploying an API resolves a version placeholder in its context". The clone checks the resolved path is served and the literal one is not.
  # The platform refuses the context with "Invalid API context format" (see the scenario list entry).
  @ID_APIP_POLICIES_017 @known-issue
  Scenario: A version placeholder in an API context resolves to the API version
    Given I obtain an APIP cloud console token
    And I find the APIP test project
    And I find an active APIP gateway
    When I create a synthetic REST API in the APIP test project with:
      | context    | /${CTX:cloudAPIID}/$version |
      | operations | [{"name":"getData","request":{"method":"GET","path":"/data"}}] |
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/1.0.0/data" until status 200
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/1.0.0/data"
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | path | /data |
    When I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/$version/data"
    Then the cloud API response status code should be 404
