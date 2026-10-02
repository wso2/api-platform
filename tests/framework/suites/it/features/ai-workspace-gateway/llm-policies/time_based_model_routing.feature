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
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# --------------------------------------------------------------------

@time-based-model-routing
Feature: Time-based model routing
  As an API consumer
  I want requests to use the model configured for the current time
  So that model selection follows the proxy's routing schedule

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Invoke an LLM proxy using a policy from the supplied policy tree
    Given I generate a unique resource name from "gateway-controller-policy-provider" and store it as "providerName"
    And I generate a unique API context from "/gateway-controller-policy-provider" and store it as "providerContext"
    And I generate a unique resource name from "gateway-controller-policy-proxy" and store it as "proxyName"
    And I generate a unique API context from "/gateway-controller-policy-proxy" and store it as "proxyContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion} |
      | name               | ${CTX:providerName}              |
      | displayName        | Gateway-Controller-Policy-Provider |
      | version            | v1.0                              |
      | template           | openai                            |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008/openai/v1  |
      | accessControl.mode | allow_all                         |
    Then the response status code should be 201
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion     | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                  |
      | name           | ${CTX:proxyName}                                                                                                                                                                                                                                                                   |
      | displayName    | Gateway-Controller-Policy-Proxy                                                                                                                                                                                                                                                     |
      | version        | v1.0                                                                                                                                                                                                                                                                                 |
      | context        | ${CTX:proxyContext}                                                                                                                                                                                                                                                                 |
      | provider.id    | ${CTX:providerName}                                                                                                                                                                                                                                                                 |
      | spec.policies  | [{"name":"time-based-model-routing","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"timezone":"UTC","schedules":[{"name":"all-day","from":"00:00","to":"23:59","model":{"modelName":"gateway-controller-policy-routed-model"}}],"fallback":{"modelName":"gateway-controller-policy-routed-model"}}}]}] |
    Then the response status code should be 201
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model":"original-model","messages":[{"role":"user","content":"policy build smoke"}]}
      """
    Then the response body should contain "gateway-controller-policy-routed-model"
    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful
