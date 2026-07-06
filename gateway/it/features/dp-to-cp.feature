Feature: Data-plane to control-plane artifact push (DP -> CP)
  # The gateway-controller pushes every gateway-originated artifact (LLM provider
  # template / provider / proxy, MCP proxy, REST API) up to the control plane on
  # create and update, tells the control plane to undeploy it on delete, and
  # re-pushes any pending/failed artifacts on (re)connect. This is gated on
  # deployment_sync_enabled (default true) and an active control-plane connection.
  #
  # These scenarios exercise the DATA-PLANE side of that flow. The control plane is
  # stood in for by mock-platform-api, which records what the gateway pushed and
  # mints a CP artifact UUID per artifact exactly like platform-api does; the
  # gateway then records that UUID and the sync outcome on its own artifacts row
  # (cp_sync_status / cp_artifact_id). We assert both the recorded push payload and
  # the gateway's bookkeeping.
  #
  # Out of scope here (they need the real platform-api, not a recorder, and are
  # covered by platform-api tests): control-plane working-copy conflict resolution,
  # read-only / runtime-immutable enforcement of DP-origin artifacts in the AI
  # Workspace, CP-side API-key generation, and multi-gateway deployments. The
  # deployment_sync_enabled=false gating is covered by gateway-controller unit
  # tests (pkg/controlplane/push_artifacts_test.go).

  Background:
    Given I authenticate using basic auth as "admin"
    And I reset the control plane recorder

  Scenario: An LLM provider template created on the gateway is pushed to the control plane
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: dp2cp-tmpl-basic
      spec:
        displayName: DP2CP Basic Template
        promptTokens:
          location: payload
          identifier: $.usage.prompt_tokens
        completionTokens:
          location: payload
          identifier: $.usage.completion_tokens
        totalTokens:
          location: payload
          identifier: $.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.model
        responseModel:
          location: payload
          identifier: $.model
      """
    Then the response should be successful
    And the control plane should receive the "LlmProviderTemplate" artifact "dp2cp-tmpl-basic"
    And the control plane copy of the "LlmProviderTemplate" artifact "dp2cp-tmpl-basic" configuration should contain "DP2CP Basic Template"
    And the control plane copy of the "LlmProviderTemplate" artifact "dp2cp-tmpl-basic" should carry a deployed timestamp
    And the gateway should record cp_sync_status "success" for the "LlmProviderTemplate" artifact "dp2cp-tmpl-basic"
    And the gateway should record a cp_artifact_id for the "LlmProviderTemplate" artifact "dp2cp-tmpl-basic"

  Scenario: An LLM provider and proxy are pushed with their cross-references carried as handles
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: dp2cp-tmpl-chain
      spec:
        displayName: DP2CP Chain Template
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
        name: dp2cp-prov-chain
      spec:
        displayName: DP2CP Chain Provider
        version: v1.0
        template: dp2cp-tmpl-chain
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
        name: dp2cp-proxy-chain
        annotations:
          "gateway.api-platform.wso2.com/project-id": "dp2cp-project"
      spec:
        displayName: DP2CP Chain Proxy
        version: v1.0
        provider:
          id: dp2cp-prov-chain
      """
    Then the response should be successful
    And the control plane should receive the "LlmProvider" artifact "dp2cp-prov-chain"
    And the control plane copy of the "LlmProvider" artifact "dp2cp-prov-chain" should reference template "dp2cp-tmpl-chain"
    And the gateway should record cp_sync_status "success" for the "LlmProvider" artifact "dp2cp-prov-chain"
    And the gateway should record a cp_artifact_id for the "LlmProvider" artifact "dp2cp-prov-chain"
    And the control plane should receive the "LlmProxy" artifact "dp2cp-proxy-chain"
    And the control plane copy of the "LlmProxy" artifact "dp2cp-proxy-chain" should reference provider "dp2cp-prov-chain"
    And the gateway should record cp_sync_status "success" for the "LlmProxy" artifact "dp2cp-proxy-chain"

  Scenario: An MCP proxy created on the gateway is pushed to the control plane
    When I deploy this MCP configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Mcp
      metadata:
        name: dp2cp-mcp-v1.0
        annotations:
          "gateway.api-platform.wso2.com/project-id": "dp2cp-project"
      spec:
        displayName: DP2CP MCP
        version: v1.0
        context: /dp2cp-everything
        specVersion: "2025-06-18"
        upstream:
          url: http://mcp-server-backend:3001
        tools: []
        resources: []
        prompts: []
      """
    Then the response should be successful
    And the control plane should receive the "Mcp" artifact "dp2cp-mcp-v1.0"
    And the control plane copy of the "Mcp" artifact "dp2cp-mcp-v1.0" configuration should contain "/dp2cp-everything"
    And the gateway should record cp_sync_status "success" for the "Mcp" artifact "dp2cp-mcp-v1.0"
    And the gateway should record a cp_artifact_id for the "Mcp" artifact "dp2cp-mcp-v1.0"

  Scenario: A REST API created on the gateway is pushed to the control plane
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: dp2cp-rest-v1.0
        annotations:
          "gateway.api-platform.wso2.com/project-id": "dp2cp-project"
      spec:
        displayName: DP2CP REST
        version: v1.0
        context: /dp2cp-rest
        upstream:
          main:
            url: http://sample-backend:9080/api/v2
        operations:
          - method: GET
            path: /resource
      """
    Then the response should be successful
    And the control plane should receive the "RestApi" artifact "dp2cp-rest-v1.0"
    And the control plane copy of the "RestApi" artifact "dp2cp-rest-v1.0" configuration should contain "/dp2cp-rest"
    And the gateway should record cp_sync_status "success" for the "RestApi" artifact "dp2cp-rest-v1.0"
    And the gateway should record a cp_artifact_id for the "RestApi" artifact "dp2cp-rest-v1.0"

  Scenario: Updating a gateway-originated LLM provider pushes a fresh deployment to the control plane
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: dp2cp-tmpl-upd
      spec:
        displayName: DP2CP Update Template
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
        name: dp2cp-prov-upd
      spec:
        displayName: DP2CP Provider Original
        version: v1.0
        template: dp2cp-tmpl-upd
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
    And the control plane should receive the "LlmProvider" artifact "dp2cp-prov-upd"
    When I update the LLM provider "dp2cp-prov-upd" with:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: dp2cp-prov-upd
      spec:
        displayName: DP2CP Provider Updated
        version: v1.0
        template: dp2cp-tmpl-upd
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
    And the control plane copy of the "LlmProvider" artifact "dp2cp-prov-upd" should have been pushed at least 2 times
    And the control plane copy of the "LlmProvider" artifact "dp2cp-prov-upd" configuration should contain "DP2CP Provider Updated"

  Scenario: Updating a gateway-originated LLM provider template re-pushes it carrying a deployment watermark
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: dp2cp-tmpl-updated
      spec:
        displayName: DP2CP Template Before
        requestModel:
          location: payload
          identifier: $.model
        responseModel:
          location: payload
          identifier: $.model
      """
    Then the response should be successful
    And the control plane should receive the "LlmProviderTemplate" artifact "dp2cp-tmpl-updated"
    When I update the LLM provider template "dp2cp-tmpl-updated" with:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: dp2cp-tmpl-updated
      spec:
        displayName: DP2CP Template After
        requestModel:
          location: payload
          identifier: $.model
        responseModel:
          location: payload
          identifier: $.model
      """
    Then the response should be successful
    And the control plane copy of the "LlmProviderTemplate" artifact "dp2cp-tmpl-updated" should have been pushed at least 2 times
    And the control plane copy of the "LlmProviderTemplate" artifact "dp2cp-tmpl-updated" configuration should contain "DP2CP Template After"
    And the control plane copy of the "LlmProviderTemplate" artifact "dp2cp-tmpl-updated" should carry a deployed timestamp

  Scenario: Deleting a gateway-originated artifact undeploys it on the control plane
    When I deploy this MCP configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Mcp
      metadata:
        name: dp2cp-mcp-del-v1.0
        annotations:
          "gateway.api-platform.wso2.com/project-id": "dp2cp-project"
      spec:
        displayName: DP2CP MCP Delete
        version: v1.0
        context: /dp2cp-mcp-del
        specVersion: "2025-06-18"
        upstream:
          url: http://mcp-server-backend:3001
        tools: []
        resources: []
        prompts: []
      """
    Then the response should be successful
    And the control plane should receive the "Mcp" artifact "dp2cp-mcp-del-v1.0" with status "deployed"
    When I delete the MCP proxy "dp2cp-mcp-del-v1.0"
    Then the response should be successful
    And the control plane should have undeployed the "Mcp" artifact "dp2cp-mcp-del-v1.0"

  Scenario: A push rejected by the control plane is recorded as failed and re-pushed on reconnect
    Given I make the control plane reject artifact imports
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: dp2cp-tmpl-reject
      spec:
        displayName: DP2CP Reject Template
        requestModel:
          location: payload
          identifier: $.model
        responseModel:
          location: payload
          identifier: $.model
      """
    Then the response should be successful
    And the gateway should record cp_sync_status "failed" for the "LlmProviderTemplate" artifact "dp2cp-tmpl-reject"
    And the control plane should not receive the "LlmProviderTemplate" artifact "dp2cp-tmpl-reject"
    When I make the control plane accept artifact imports
    And I restart the "gateway-controller" service
    Then the control plane should receive the "LlmProviderTemplate" artifact "dp2cp-tmpl-reject"
    And the gateway should record cp_sync_status "success" for the "LlmProviderTemplate" artifact "dp2cp-tmpl-reject"
