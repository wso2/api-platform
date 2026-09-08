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

@respond
Feature: Response policy behavior
  As an API developer
  I want response policies to control status, headers, and bodies
  So that clients receive the configured response

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"


  Scenario: Return simple 200 OK response with plain text body
    Given I generate a unique value from "respond-1" and store it as "apiName1"
    And I generate a unique API context from "/respond-1" and store it as "apiContext1"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName1} |
      | spec | {"displayName":"Respond-Simple-200-Test","version":"v1.0.0","context":"${CTX:apiContext1}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"OK"}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext1}/v1.0.0/test" until status 200
    When I send a "GET" request to "${CTX:apiContext1}/v1.0.0/test"
    Then the response status code should be 200
    And the response body should contain "OK"


  Scenario: Return 201 Created with JSON body and headers
    Given I generate a unique value from "respond-2" and store it as "apiName2"
    And I generate a unique API context from "/respond-2" and store it as "apiContext2"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName2} |
      | spec | {"displayName":"Respond-201-Created-Test","version":"v1.0.0","context":"${CTX:apiContext2}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/health","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"OK"}}]},{"method":"POST","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":201,"body":"{\"id\": 123, \"name\": \"Created Resource\", \"status\": \"success\"}","headers":[{"name":"Content-Type","value":"application/json"},{"name":"Location","value":"/api/resource/123"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext2}/v1.0.0/health" until status 200
    When I send a "POST" request to "${CTX:apiContext2}/v1.0.0/test" with body:
      """
      {"test": "data"}
      """
    Then the response status code should be 201
    And the response body should contain "Created Resource"
    And the response header "Content-Type" should be "application/json"
    And the response header "Location" should be "/api/resource/123"


  Scenario: Return 204 No Content with empty body
    Given I generate a unique value from "respond-3" and store it as "apiName3"
    And I generate a unique API context from "/respond-3" and store it as "apiContext3"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName3} |
      | spec | {"displayName":"Respond-204-NoContent-Test","version":"v1.0.0","context":"${CTX:apiContext3}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/health","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"OK"}}]},{"method":"DELETE","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":204}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext3}/v1.0.0/health" until status 200
    When I send a "DELETE" request to "${CTX:apiContext3}/v1.0.0/test"
    Then the response status code should be 204
    And the response body should be empty


  Scenario: Default status code is 200 when not specified
    Given I generate a unique value from "respond-4" and store it as "apiName4"
    And I generate a unique API context from "/respond-4" and store it as "apiContext4"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName4} |
      | spec | {"displayName":"Respond-Default-200-Test","version":"v1.0.0","context":"${CTX:apiContext4}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test","policies":[{"name":"respond","version":"v1","params":{"body":"Default response"}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext4}/v1.0.0/test" until status 200
    When I send a "GET" request to "${CTX:apiContext4}/v1.0.0/test"
    Then the response status code should be 200
    And the response body should contain "Default response"


  Scenario: Return 400 Bad Request error
    Given I generate a unique value from "respond-5" and store it as "apiName5"
    And I generate a unique API context from "/respond-5" and store it as "apiContext5"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName5} |
      | spec | {"displayName":"Respond-400-BadRequest-Test","version":"v1.0.0","context":"${CTX:apiContext5}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/health","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"OK"}}]},{"method":"POST","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":400,"body":"{\"error\": \"Bad Request\", \"message\": \"Invalid input data\"}","headers":[{"name":"Content-Type","value":"application/json"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext5}/v1.0.0/health" until status 200
    When I send a "POST" request to "${CTX:apiContext5}/v1.0.0/test" with body:
      """
      {"invalid": "data"}
      """
    Then the response status code should be 400
    And the response body should contain "Bad Request"
    And the response body should contain "Invalid input data"


  Scenario: Return 401 Unauthorized error
    Given I generate a unique value from "respond-6" and store it as "apiName6"
    And I generate a unique API context from "/respond-6" and store it as "apiContext6"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName6} |
      | spec | {"displayName":"Respond-401-Unauthorized-Test","version":"v1.0.0","context":"${CTX:apiContext6}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/health","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"OK"}}]},{"method":"GET","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":401,"body":"{\"error\": \"Unauthorized\", \"message\": \"Authentication required\"}","headers":[{"name":"Content-Type","value":"application/json"},{"name":"WWW-Authenticate","value":"Bearer realm=\"api\""}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext6}/v1.0.0/health" until status 200
    When I send a "GET" request to "${CTX:apiContext6}/v1.0.0/test"
    Then the response status code should be 401
    And the response body should contain "Unauthorized"
    And the response header "WWW-Authenticate" should contain "Bearer"


  Scenario: Return 403 Forbidden error
    Given I generate a unique value from "respond-7" and store it as "apiName7"
    And I generate a unique API context from "/respond-7" and store it as "apiContext7"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName7} |
      | spec | {"displayName":"Respond-403-Forbidden-Test","version":"v1.0.0","context":"${CTX:apiContext7}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/health","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"OK"}}]},{"method":"GET","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":403,"body":"{\"error\": \"Forbidden\", \"message\": \"Access denied to this resource\"}","headers":[{"name":"Content-Type","value":"application/json"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext7}/v1.0.0/health" until status 200
    When I send a "GET" request to "${CTX:apiContext7}/v1.0.0/test"
    Then the response status code should be 403
    And the response body should contain "Forbidden"
    And the response body should contain "Access denied"


  Scenario: Return 404 Not Found error
    Given I generate a unique value from "respond-8" and store it as "apiName8"
    And I generate a unique API context from "/respond-8" and store it as "apiContext8"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName8} |
      | spec | {"displayName":"Respond-404-NotFound-Test","version":"v1.0.0","context":"${CTX:apiContext8}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/health","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"OK"}}]},{"method":"GET","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":404,"body":"{\"error\": \"Not Found\", \"message\": \"The requested resource does not exist\"}","headers":[{"name":"Content-Type","value":"application/json"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext8}/v1.0.0/health" until status 200
    When I send a "GET" request to "${CTX:apiContext8}/v1.0.0/test"
    Then the response status code should be 404
    And the response body should contain "Not Found"


  Scenario: Return 429 Too Many Requests
    Given I generate a unique value from "respond-9" and store it as "apiName9"
    And I generate a unique API context from "/respond-9" and store it as "apiContext9"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName9} |
      | spec | {"displayName":"Respond-429-RateLimit-Test","version":"v1.0.0","context":"${CTX:apiContext9}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/health","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"OK"}}]},{"method":"GET","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":429,"body":"{\"error\": \"Too many requests\", \"message\": \"Rate limit exceeded\"}","headers":[{"name":"Content-Type","value":"application/json"},{"name":"Retry-After","value":"60"},{"name":"X-RateLimit-Limit","value":"100"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext9}/v1.0.0/health" until status 200
    When I send a "GET" request to "${CTX:apiContext9}/v1.0.0/test"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"
    And the response header "Retry-After" should be "60"
    And the response header "X-RateLimit-Limit" should be "100"


  Scenario: Return 500 Internal Server Error
    Given I generate a unique value from "respond-10" and store it as "apiName10"
    And I generate a unique API context from "/respond-10" and store it as "apiContext10"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName10} |
      | spec | {"displayName":"Respond-500-InternalError-Test","version":"v1.0.0","context":"${CTX:apiContext10}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/health","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"OK"}}]},{"method":"GET","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":500,"body":"{\"error\": \"Internal Server Error\", \"message\": \"An unexpected error occurred\"}","headers":[{"name":"Content-Type","value":"application/json"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext10}/v1.0.0/health" until status 200
    When I send a "GET" request to "${CTX:apiContext10}/v1.0.0/test"
    Then the response status code should be 500
    And the response body should contain "Internal Server Error"


  Scenario: Return 503 Service Unavailable (maintenance mode)
    Given I generate a unique value from "respond-11" and store it as "apiName11"
    And I generate a unique API context from "/respond-11" and store it as "apiContext11"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName11} |
      | spec | {"displayName":"Respond-503-Maintenance-Test","version":"v1.0.0","context":"${CTX:apiContext11}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/health","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"OK"}}]},{"method":"GET","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":503,"body":"{\"error\": \"Service Unavailable\", \"message\": \"System under maintenance. Please try again later.\"}","headers":[{"name":"Content-Type","value":"application/json"},{"name":"Retry-After","value":"3600"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext11}/v1.0.0/health" until status 200
    When I send a "GET" request to "${CTX:apiContext11}/v1.0.0/test"
    Then the response status code should be 503
    And the response body should contain "under maintenance"
    And the response header "Retry-After" should be "3600"


  Scenario: Return 301 Moved Permanently redirect
    Given I generate a unique value from "respond-12" and store it as "apiName12"
    And I generate a unique API context from "/respond-12" and store it as "apiContext12"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName12} |
      | spec | {"displayName":"Respond-301-Redirect-Test","version":"v1.0.0","context":"${CTX:apiContext12}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/health","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"OK"}}]},{"method":"GET","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":301,"headers":[{"name":"Location","value":"https://example.com/new-location"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext12}/v1.0.0/health" until status 200
    When I send a "GET" request to "${CTX:apiContext12}/v1.0.0/test"
    Then the response status code should be 301
    And the response header "Location" should be "https://example.com/new-location"


  Scenario: Return 302 Found temporary redirect
    Given I generate a unique value from "respond-13" and store it as "apiName13"
    And I generate a unique API context from "/respond-13" and store it as "apiContext13"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName13} |
      | spec | {"displayName":"Respond-302-Redirect-Test","version":"v1.0.0","context":"${CTX:apiContext13}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/health","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"OK"}}]},{"method":"GET","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":302,"headers":[{"name":"Location","value":"/temporary-location"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext13}/v1.0.0/health" until status 200
    When I send a "GET" request to "${CTX:apiContext13}/v1.0.0/test"
    Then the response status code should be 302
    And the response header "Location" should be "/temporary-location"


  Scenario: Return XML response
    Given I generate a unique value from "respond-14" and store it as "apiName14"
    And I generate a unique API context from "/respond-14" and store it as "apiContext14"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName14} |
      | spec | {"displayName":"Respond-XML-Test","version":"v1.0.0","context":"${CTX:apiContext14}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"<?xml version=\"1.0\"?><response><status>success</status><message>XML response</message></response>","headers":[{"name":"Content-Type","value":"application/xml"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext14}/v1.0.0/test" until status 200
    When I send a "GET" request to "${CTX:apiContext14}/v1.0.0/test"
    Then the response status code should be 200
    And the response body should contain "<status>success</status>"
    And the response header "Content-Type" should be "application/xml"


  Scenario: Return HTML response
    Given I generate a unique value from "respond-15" and store it as "apiName15"
    And I generate a unique API context from "/respond-15" and store it as "apiContext15"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName15} |
      | spec | {"displayName":"Respond-HTML-Test","version":"v1.0.0","context":"${CTX:apiContext15}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"<html><head><title>API Documentation</title></head><body><h1>Welcome</h1><p>This is a static HTML response.</p></body></html>","headers":[{"name":"Content-Type","value":"text/html"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext15}/v1.0.0/test" until status 200
    When I send a "GET" request to "${CTX:apiContext15}/v1.0.0/test"
    Then the response status code should be 200
    And the response body should contain "<h1>Welcome</h1>"
    And the response header "Content-Type" should be "text/html"


  Scenario: Return plain text response
    Given I generate a unique value from "respond-16" and store it as "apiName16"
    And I generate a unique API context from "/respond-16" and store it as "apiContext16"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName16} |
      | spec | {"displayName":"Respond-Text-Test","version":"v1.0.0","context":"${CTX:apiContext16}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"This is a plain text response with multiple lines.\nLine 2\nLine 3","headers":[{"name":"Content-Type","value":"text/plain"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext16}/v1.0.0/test" until status 200
    When I send a "GET" request to "${CTX:apiContext16}/v1.0.0/test"
    Then the response status code should be 200
    And the response body should contain "plain text response"
    And the response header "Content-Type" should be "text/plain"


  Scenario: API mocking - return mocked user data
    Given I generate a unique value from "respond-17" and store it as "apiName17"
    And I generate a unique API context from "/respond-17" and store it as "apiContext17"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName17} |
      | spec | {"displayName":"Respond-Mock-User-Test","version":"v1.0.0","context":"${CTX:apiContext17}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/health","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"OK"}}]},{"method":"GET","path":"/users/{id}","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"{\"id\": 123, \"name\": \"John Doe\", \"email\": \"john@example.com\", \"role\": \"admin\"}","headers":[{"name":"Content-Type","value":"application/json"},{"name":"X-Mock-Response","value":"true"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext17}/v1.0.0/health" until status 200
    When I send a "GET" request to "${CTX:apiContext17}/v1.0.0/users/123"
    Then the response status code should be 200
    And the response body should contain "John Doe"
    And the response body should contain "john@example.com"
    And the response header "X-Mock-Response" should be "true"


  Scenario: Deprecated API endpoint notice
    Given I generate a unique value from "respond-18" and store it as "apiName18"
    And I generate a unique API context from "/respond-18" and store it as "apiContext18"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName18} |
      | spec | {"displayName":"Respond-Deprecated-Test","version":"v1.0.0","context":"${CTX:apiContext18}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/health","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"OK"}}]},{"method":"GET","path":"/v1/users","policies":[{"name":"respond","version":"v1","params":{"statusCode":410,"body":"{\"error\": \"Gone\", \"message\": \"This endpoint is deprecated. Please use /v2/users instead.\", \"migration_guide\": \"https://docs.example.com/migration\"}","headers":[{"name":"Content-Type","value":"application/json"},{"name":"X-API-Deprecated","value":"true"},{"name":"X-API-Sunset-Date","value":"2026-12-31"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext18}/v1.0.0/health" until status 200
    When I send a "GET" request to "${CTX:apiContext18}/v1.0.0/v1/users"
    Then the response status code should be 410
    And the response body should contain "deprecated"
    And the response body should contain "/v2/users"
    And the response header "X-API-Deprecated" should be "true"


  Scenario: Health check stub
    Given I generate a unique value from "respond-19" and store it as "apiName19"
    And I generate a unique API context from "/respond-19" and store it as "apiContext19"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName19} |
      | spec | {"displayName":"Respond-Health-Test","version":"v1.0.0","context":"${CTX:apiContext19}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/health","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"{\"status\": \"healthy\", \"version\": \"1.0.0\", \"uptime\": 3600}","headers":[{"name":"Content-Type","value":"application/json"},{"name":"Cache-Control","value":"no-cache"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext19}/v1.0.0/health" until status 200
    When I send a "GET" request to "${CTX:apiContext19}/v1.0.0/health"
    Then the response status code should be 200
    And the response body should contain "healthy"
    And the response header "Cache-Control" should be "no-cache"


  Scenario: CORS preflight OPTIONS response
    Given I generate a unique value from "respond-20" and store it as "apiName20"
    And I generate a unique API context from "/respond-20" and store it as "apiContext20"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName20} |
      | spec | {"displayName":"Respond-CORS-Test","version":"v1.0.0","context":"${CTX:apiContext20}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/health","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"OK"}}]},{"method":"OPTIONS","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":204,"headers":[{"name":"Access-Control-Allow-Origin","value":"*"},{"name":"Access-Control-Allow-Methods","value":"GET, POST, PUT, DELETE, OPTIONS"},{"name":"Access-Control-Allow-Headers","value":"Content-Type, Authorization"},{"name":"Access-Control-Max-Age","value":"86400"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext20}/v1.0.0/health" until status 200
    When I send a "OPTIONS" request to "${CTX:apiContext20}/v1.0.0/test"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should be "*"
    And the response header "Access-Control-Allow-Methods" should be "GET, POST, PUT, DELETE, OPTIONS"
    And the response header "Access-Control-Max-Age" should be "86400"


  Scenario: Response with no body and no headers
    Given I generate a unique value from "respond-21" and store it as "apiName21"
    And I generate a unique API context from "/respond-21" and store it as "apiContext21"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName21} |
      | spec | {"displayName":"Respond-Minimal-Test","version":"v1.0.0","context":"${CTX:apiContext21}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":200}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext21}/v1.0.0/test" until status 200
    When I send a "GET" request to "${CTX:apiContext21}/v1.0.0/test"
    Then the response status code should be 200


  Scenario: Response with empty body string
    Given I generate a unique value from "respond-22" and store it as "apiName22"
    And I generate a unique API context from "/respond-22" and store it as "apiContext22"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName22} |
      | spec | {"displayName":"Respond-Empty-Body-Test","version":"v1.0.0","context":"${CTX:apiContext22}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":""}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext22}/v1.0.0/test" until status 200
    When I send a "GET" request to "${CTX:apiContext22}/v1.0.0/test"
    Then the response status code should be 200
    And the response body should be empty


  Scenario: Response with multiple custom headers
    Given I generate a unique value from "respond-23" and store it as "apiName23"
    And I generate a unique API context from "/respond-23" and store it as "apiContext23"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName23} |
      | spec | {"displayName":"Respond-Multiple-Headers-Test","version":"v1.0.0","context":"${CTX:apiContext23}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"{\"message\": \"success\"}","headers":[{"name":"Content-Type","value":"application/json"},{"name":"X-Custom-Header-1","value":"value1"},{"name":"X-Custom-Header-2","value":"value2"},{"name":"X-Custom-Header-3","value":"value3"},{"name":"Cache-Control","value":"max-age=3600"},{"name":"X-Request-ID","value":"req-abc-123"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext23}/v1.0.0/test" until status 200
    When I send a "GET" request to "${CTX:apiContext23}/v1.0.0/test"
    Then the response status code should be 200
    And the response header "X-Custom-Header-1" should be "value1"
    And the response header "X-Custom-Header-2" should be "value2"
    And the response header "X-Custom-Header-3" should be "value3"
    And the response header "Cache-Control" should be "max-age=3600"
    And the response header "X-Request-ID" should be "req-abc-123"


  Scenario: Large JSON response body
    Given I generate a unique value from "respond-24" and store it as "apiName24"
    And I generate a unique API context from "/respond-24" and store it as "apiContext24"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName24} |
      | spec | {"displayName":"Respond-Large-JSON-Test","version":"v1.0.0","context":"${CTX:apiContext24}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"{\"users\": [{\"id\": 1, \"name\": \"User 1\"}, {\"id\": 2, \"name\": \"User 2\"}, {\"id\": 3, \"name\": \"User 3\"}, {\"id\": 4, \"name\": \"User 4\"}, {\"id\": 5, \"name\": \"User 5\"}], \"total\": 5, \"page\": 1, \"pageSize\": 10, \"metadata\": {\"timestamp\": \"2026-01-28T10:00:00Z\", \"version\": \"v1\"}}","headers":[{"name":"Content-Type","value":"application/json"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext24}/v1.0.0/test" until status 200
    When I send a "GET" request to "${CTX:apiContext24}/v1.0.0/test"
    Then the response status code should be 200
    And the response body should contain "User 1"
    And the response body should contain "User 5"
    And the response body should contain "total"


  Scenario: Response with special characters in body
    Given I generate a unique value from "respond-25" and store it as "apiName25"
    And I generate a unique API context from "/respond-25" and store it as "apiContext25"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName25} |
      | spec | {"displayName":"Respond-Special-Chars-Test","version":"v1.0.0","context":"${CTX:apiContext25}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body": '{"message": "Special chars: <>&\"''\n\t", "emoji": "🎉✅❌", "unicode": "Hello 世界"}',"headers":[{"name":"Content-Type","value":"application/json; charset=utf-8"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext25}/v1.0.0/test" until status 200
    When I send a "GET" request to "${CTX:apiContext25}/v1.0.0/test"
    Then the response status code should be 200
    And the response body should contain "Special chars"
    And the response body should contain "🎉"


  Scenario: Return 206 Partial Content status code
    Given I generate a unique value from "respond-26" and store it as "apiName26"
    And I generate a unique API context from "/respond-26" and store it as "apiContext26"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName26} |
      | spec | {"displayName":"Respond-206-Partial-Test","version":"v1.0.0","context":"${CTX:apiContext26}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/health","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"OK"}}]},{"method":"POST","path":"/test","policies":[{"name":"respond","version":"v1","params":{"statusCode":206,"body":"{\"status\": \"Partial Content\", \"range\": \"bytes 0-1023/2048\"}","headers":[{"name":"Content-Type","value":"application/json"},{"name":"Content-Range","value":"bytes 0-1023/2048"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext26}/v1.0.0/health" until status 200
    When I send a "POST" request to "${CTX:apiContext26}/v1.0.0/test" with body:
      """
      {"test": "data"}
      """
    Then the response status code should be 206
    And the response body should contain "Partial Content"
    And the response header "Content-Range" should be "bytes 0-1023/2048"


  Scenario: Return custom error code 418 I'm a teapot
    Given I generate a unique value from "respond-27" and store it as "apiName27"
    And I generate a unique API context from "/respond-27" and store it as "apiContext27"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName27} |
      | spec | {"displayName":"Respond-418-Teapot-Test","version":"v1.0.0","context":"${CTX:apiContext27}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/health","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"OK"}}]},{"method":"POST","path":"/brew-coffee","policies":[{"name":"respond","version":"v1","params":{"statusCode":418,"body":"{\"error\": \"I am a teapot\", \"message\": \"This server refuses to brew coffee\"}","headers":[{"name":"Content-Type","value":"application/json"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext27}/v1.0.0/health" until status 200
    When I send a "POST" request to "${CTX:apiContext27}/v1.0.0/brew-coffee" with body:
      """
      {"beverage": "coffee"}
      """
    Then the response status code should be 418
    And the response body should contain "teapot"


  Scenario: Return different response based on path but same policy
    Given I generate a unique value from "respond-28" and store it as "apiName28"
    And I generate a unique API context from "/respond-28" and store it as "apiContext28"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName28} |
      | spec | {"displayName":"Respond-Multi-Path-Test","version":"v1.0.0","context":"${CTX:apiContext28}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/json","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"{\"format\": \"json\"}","headers":[{"name":"Content-Type","value":"application/json"}]}}]},{"method":"GET","path":"/xml","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"<?xml version=\"1.0\"?><response><format>xml</format></response>","headers":[{"name":"Content-Type","value":"application/xml"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext28}/v1.0.0/json" until status 200
    When I send a "GET" request to "${CTX:apiContext28}/v1.0.0/json"
    Then the response status code should be 200
    And the response body should contain "json"
    When I send a "GET" request to "${CTX:apiContext28}/v1.0.0/xml"
    Then the response status code should be 200
    And the response body should contain "<format>xml</format>"


  Scenario: Response with cache control headers
    Given I generate a unique value from "respond-29" and store it as "apiName29"
    And I generate a unique API context from "/respond-29" and store it as "apiContext29"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName29} |
      | spec | {"displayName":"Respond-Cache-Control-Test","version":"v1.0.0","context":"${CTX:apiContext29}/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/static","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"body":"{\"data\": \"static content\"}","headers":[{"name":"Content-Type","value":"application/json"},{"name":"Cache-Control","value":"public, max-age=86400"},{"name":"ETag","value":"abc123"},{"name":"Expires","value":"Tue, 28 Jan 2026 12:00:00 GMT"}]}}]}]} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext29}/v1.0.0/static" until status 200
    When I send a "GET" request to "${CTX:apiContext29}/v1.0.0/static"
    Then the response status code should be 200
    And the response header "Cache-Control" should be "public, max-age=86400"
    And the response header "ETag" should contain "abc123"
    And the response header "Expires" should be "Tue, 28 Jan 2026 12:00:00 GMT"
