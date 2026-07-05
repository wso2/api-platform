Feature: DP -> CP push is suppressed when deployment sync is disabled
  # Covers dp-to-cp-testing.md items 1-2: with deployment_sync_enabled=false the gateway
  # still connects to the control plane (mock-platform-api) over the WebSocket, but it must
  # NOT push gateway-originated artifacts up — they stay local with cp_sync_status=pending.
  #
  # This feature runs ONLY under the sync-disabled compose overlay (the controller gets
  # APIP_GW_CONTROLLER_CONTROLPLANE_DEPLOYMENT_SYNC_ENABLED=false). Run it with:
  #   make test-dp-to-cp-nosync
  # It is deliberately NOT in the default suite (which runs with sync enabled to verify the
  # push happens — see dp-to-cp.feature).

  Background:
    Given I authenticate using basic auth as "admin"
    And I reset the control plane recorder

  Scenario: An LLM provider template is not pushed when sync is disabled
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: nosync-tmpl
      spec:
        displayName: NoSync Template
        requestModel:
          location: payload
          identifier: $.model
        responseModel:
          location: payload
          identifier: $.model
      """
    Then the response should be successful
    And the control plane should not receive the "LlmProviderTemplate" artifact "nosync-tmpl"
    And the gateway should record cp_sync_status "pending" for the "LlmProviderTemplate" artifact "nosync-tmpl"

  Scenario: An LLM provider and proxy are not pushed when sync is disabled
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: nosync-chain-tmpl
      spec:
        displayName: NoSync Chain Template
        requestModel:
          location: payload
          identifier: $.model
        responseModel:
          location: payload
          identifier: $.model
      """
    Then the response should be successful
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: nosync-prov
      spec:
        displayName: NoSync Provider
        version: v1.0
        template: nosync-chain-tmpl
        upstream:
          url: https://mock-openapi-https:9443/openai/v1
          auth:
            type: api-key
            header: Authorization
            value: Bearer sk-test-key
        accessControl:
          mode: allow_all
      """
    Then the response should be successful
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: nosync-proxy
        annotations:
          "gateway.api-platform.wso2.com/project-id": "dp2cp-project"
      spec:
        displayName: NoSync Proxy
        version: v1.0
        provider:
          id: nosync-prov
      """
    Then the response should be successful
    And the control plane should not receive the "LlmProvider" artifact "nosync-prov"
    And the control plane should not receive the "LlmProxy" artifact "nosync-proxy"
    And the gateway should record cp_sync_status "pending" for the "LlmProvider" artifact "nosync-prov"

  Scenario: An MCP proxy is not pushed when sync is disabled
    When I deploy this MCP configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Mcp
      metadata:
        name: nosync-mcp-v1.0
        annotations:
          "gateway.api-platform.wso2.com/project-id": "dp2cp-project"
      spec:
        displayName: NoSync MCP
        version: v1.0
        context: /nosync-mcp
        specVersion: "2025-06-18"
        upstream:
          url: http://mcp-server-backend:3001
        tools: []
        resources: []
        prompts: []
      """
    Then the response should be successful
    And the control plane should not receive the "Mcp" artifact "nosync-mcp-v1.0"
    And the gateway should record cp_sync_status "pending" for the "Mcp" artifact "nosync-mcp-v1.0"

  Scenario: A REST API is not pushed when sync is disabled
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: nosync-rest-v1.0
        annotations:
          "gateway.api-platform.wso2.com/project-id": "dp2cp-project"
      spec:
        displayName: NoSync REST
        version: v1.0
        context: /nosync-rest
        upstream:
          main:
            url: http://sample-backend:9080/api/v2
        operations:
          - method: GET
            path: /resource
      """
    Then the response should be successful
    And the control plane should not receive the "RestApi" artifact "nosync-rest-v1.0"
    And the gateway should record cp_sync_status "pending" for the "RestApi" artifact "nosync-rest-v1.0"
