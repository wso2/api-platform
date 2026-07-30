# Authentication

- oAuth2 authentication. OAuth2/OIDC access token with fine-grained API Portal scopes. Each operation declares the exact resource/action scope it requires.

    - Flow: authorizationCode
    - Authorization URL = [https://localhost:9443/oauth2/authorize](https://localhost:9443/oauth2/authorize)
    - Token URL = [https://localhost:9443/oauth2/token](https://localhost:9443/oauth2/token)

|Scope|Scope Description|
|---|---|
|dp:organization:read|Read organizations.|
|dp:organization:create|Create organizations.|
|dp:organization:update|Update organizations.|
|dp:organization:delete|Delete organizations.|
|dp:organization:manage|Manage organizations (including creating, updating, and deleting).|
|dp:organization_content:read|Read organization theme assets.|
|dp:organization_content:manage|Apply or reset organization theme.|
|dp:api:read|Read API metadata.|
|dp:api:create|Create API metadata.|
|dp:api:update|Update API metadata.|
|dp:api:delete|Delete API metadata.|
|dp:api:manage|Manage API metadata.|
|dp:api_content:read|Read API content.|
|dp:api_content:create|Create API content.|
|dp:api_content:update|Update API content.|
|dp:api_content:delete|Delete API content.|
|dp:api_content:manage|Manage API content.|
|dp:mcp_server:read|Read MCP server metadata.|
|dp:mcp_server:create|Create MCP server metadata.|
|dp:mcp_server:update|Update MCP server metadata.|
|dp:mcp_server:delete|Delete MCP server metadata.|
|dp:mcp_server:manage|Manage MCP server metadata.|
|dp:mcp_server_content:read|Read MCP server content.|
|dp:mcp_server_content:create|Create MCP server content.|
|dp:mcp_server_content:update|Update MCP server content.|
|dp:mcp_server_content:delete|Delete MCP server content.|
|dp:mcp_server_content:manage|Manage MCP server content.|
|dp:mcp_server_key:read|Read MCP server API keys.|
|dp:mcp_server_key:create|Generate MCP server API keys.|
|dp:mcp_server_key:update|Regenerate MCP server API keys.|
|dp:mcp_server_key:revoke|Revoke MCP server API keys.|
|dp:mcp_server_key:manage|Manage MCP server API keys.|
|dp:subscription_plan:read|Read subscription plans.|
|dp:subscription_plan:create|Create subscription plans.|
|dp:subscription_plan:update|Update subscription plans.|
|dp:subscription_plan:delete|Delete subscription plans.|
|dp:subscription_plan:manage|Manage subscription plans.|
|dp:label:read|Read labels.|
|dp:label:create|Create labels.|
|dp:label:update|Update labels.|
|dp:label:delete|Delete labels.|
|dp:label:manage|Manage labels.|
|dp:application:read|Read applications.|
|dp:application:create|Create applications.|
|dp:application:update|Update applications.|
|dp:application:delete|Delete applications.|
|dp:application:manage|Manage applications.|
|dp:subscription:read|Read subscriptions.|
|dp:subscription:create|Create subscriptions.|
|dp:subscription:update|Update subscriptions.|
|dp:subscription:delete|Delete subscriptions.|
|dp:subscription:manage|Manage subscriptions.|
|dp:api_key:read|Read API keys.|
|dp:api_key:create|Generate API keys.|
|dp:api_key:update|Regenerate API keys.|
|dp:api_key:revoke|Revoke API keys.|
|dp:api_key:manage|Manage API keys.|
|dp:application_key_mapping:read|Read application key mappings.|
|dp:application_key_mapping:create|Create application key mappings.|
|dp:application_key_mapping:manage|Manage application key mappings.|
|dp:view:read|Read views.|
|dp:view:create|Create views.|
|dp:view:update|Update views.|
|dp:view:delete|Delete views.|
|dp:view:manage|Manage views.|
|dp:application_key:create|Generate and create application keys.|
|dp:application_key:update|Update application keys.|
|dp:application_key:revoke|Revoke application keys.|
|dp:application_key:manage|Manage application keys.|
|dp:api_workflow:read|Read API workflows.|
|dp:api_workflow:create|Create or generate API workflows.|
|dp:api_workflow:update|Update API workflows.|
|dp:api_workflow:delete|Delete API workflows.|
|dp:api_workflow:manage|Manage API workflows.|
|dp:event:read|Read webhook events and delivery details.|
|dp:key_manager:read|Read key manager configurations.|
|dp:key_manager:create|Create key manager configurations.|
|dp:key_manager:update|Update key manager configurations.|
|dp:key_manager:delete|Delete key manager configurations.|
|dp:key_manager:manage|Manage key manager configurations (including creating, updating, and deleting).|
|dp:webhook_subscriber:read|Read webhook subscriber configurations.|
|dp:webhook_subscriber:create|Create webhook subscriber configurations.|
|dp:webhook_subscriber:update|Update webhook subscriber configurations.|
|dp:webhook_subscriber:delete|Delete webhook subscriber configurations.|
|dp:webhook_subscriber:manage|Manage webhook subscriber configurations (including creating, updating, and deleting).|

* API Key (apiKeyAuth)
    - Parameter Name: **x-api-key**, in: header. API key authentication. Server-side authorization should bind each key to the same fine-grained permissions used by OAuth2 scopes.
