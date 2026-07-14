# Authentication

- oAuth2 authentication. OAuth2/OIDC access token with fine-grained Developer Portal scopes. Each operation declares the exact resource/action scope it requires.

    - Flow: authorizationCode
    - Authorization URL = [https://localhost:9443/oauth2/authorize](https://localhost:9443/oauth2/authorize)
    - Token URL = [https://localhost:9443/oauth2/token](https://localhost:9443/oauth2/token)

|Scope|Scope Description|
|---|---|
|dp:org_read|Read organizations.|
|dp:org_create|Create organizations.|
|dp:org_update|Update organizations.|
|dp:org_delete|Delete organizations.|
|dp:org_manage|Manage organizations (including creating, updating, and deleting).|
|dp:org_content_read|Read organization theme assets.|
|dp:org_content_manage|Apply or reset organization theme.|
|dp:api_read|Read API metadata.|
|dp:api_create|Create API metadata.|
|dp:api_update|Update API metadata.|
|dp:api_delete|Delete API metadata.|
|dp:api_manage|Manage API metadata.|
|dp:api_content_read|Read API content.|
|dp:api_content_create|Create API content.|
|dp:api_content_update|Update API content.|
|dp:api_content_delete|Delete API content.|
|dp:api_content_manage|Manage API content.|
|dp:mcp_read|Read MCP server metadata.|
|dp:mcp_create|Create MCP server metadata.|
|dp:mcp_update|Update MCP server metadata.|
|dp:mcp_delete|Delete MCP server metadata.|
|dp:mcp_manage|Manage MCP server metadata.|
|dp:mcp_content_read|Read MCP server content.|
|dp:mcp_content_create|Create MCP server content.|
|dp:mcp_content_update|Update MCP server content.|
|dp:mcp_content_delete|Delete MCP server content.|
|dp:mcp_content_manage|Manage MCP server content.|
|dp:mcp_key_read|Read MCP server API keys.|
|dp:mcp_key_create|Generate MCP server API keys.|
|dp:mcp_key_update|Regenerate MCP server API keys.|
|dp:mcp_key_revoke|Revoke MCP server API keys.|
|dp:mcp_key_manage|Manage MCP server API keys.|
|dp:sub_plan_read|Read subscription plans.|
|dp:sub_plan_create|Create subscription plans.|
|dp:sub_plan_update|Update subscription plans.|
|dp:sub_plan_delete|Delete subscription plans.|
|dp:sub_plan_manage|Manage subscription plans.|
|dp:label_read|Read labels.|
|dp:label_create|Create labels.|
|dp:label_update|Update labels.|
|dp:label_delete|Delete labels.|
|dp:label_manage|Manage labels.|
|dp:app_read|Read applications.|
|dp:app_create|Create applications.|
|dp:app_update|Update applications.|
|dp:app_delete|Delete applications.|
|dp:app_manage|Manage applications.|
|dp:subscription_read|Read subscriptions.|
|dp:subscription_create|Create subscriptions.|
|dp:subscription_update|Update subscriptions.|
|dp:subscription_delete|Delete subscriptions.|
|dp:subscription_manage|Manage subscriptions.|
|dp:api_key_read|Read API keys.|
|dp:api_key_create|Generate API keys.|
|dp:api_key_update|Regenerate API keys.|
|dp:api_key_revoke|Revoke API keys.|
|dp:api_key_manage|Manage API keys.|
|dp:app_key_mapping_read|Read application key mappings.|
|dp:app_key_mapping_create|Create application key mappings.|
|dp:app_key_mapping_manage|Manage application key mappings.|
|dp:view_read|Read views.|
|dp:view_create|Create views.|
|dp:view_update|Update views.|
|dp:view_delete|Delete views.|
|dp:view_manage|Manage views.|
|dp:app_key_create|Generate and create application keys.|
|dp:app_key_update|Update application keys.|
|dp:app_key_revoke|Revoke application keys.|
|dp:app_key_manage|Manage application keys.|
|dp:api_workflow_read|Read API workflows.|
|dp:api_workflow_create|Create or generate API workflows.|
|dp:api_workflow_update|Update API workflows.|
|dp:api_workflow_delete|Delete API workflows.|
|dp:api_workflow_manage|Manage API workflows.|
|dp:event_read|Read webhook events and delivery details.|
|dp:km_read|Read key manager configurations.|
|dp:km_create|Create key manager configurations.|
|dp:km_update|Update key manager configurations.|
|dp:km_delete|Delete key manager configurations.|
|dp:km_manage|Manage key manager configurations (including creating, updating, and deleting).|
|dp:webhook_subscriber_read|Read webhook subscriber configurations.|
|dp:webhook_subscriber_create|Create webhook subscriber configurations.|
|dp:webhook_subscriber_update|Update webhook subscriber configurations.|
|dp:webhook_subscriber_delete|Delete webhook subscriber configurations.|
|dp:webhook_subscriber_manage|Manage webhook subscriber configurations (including creating, updating, and deleting).|

* API Key (apiKeyAuth)
    - Parameter Name: **x-api-key**, in: header. API key authentication. Server-side authorization should bind each key to the same fine-grained permissions used by OAuth2 scopes.
