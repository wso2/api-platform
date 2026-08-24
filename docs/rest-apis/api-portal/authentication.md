# Authentication

## OAuth2 (OAuth2Security)

Bearer JWT access token, sent as `Authorization: Bearer <token>`. The API Portal issues no
token of its own — where you obtain one depends on `auth.mode`.

**Local auth mode** (`auth.mode = "local"`, the development default). The token comes from
the Platform API the portal is configured against (`auth.local.platform_api_url`), whose
login endpoint validates a file-configured username and password and returns a signed JWT:

```bash
TOKEN=$(curl -sk -X POST https://localhost:9243/api/portal/v0.9/auth/login \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  -d 'username=admin' -d 'password=admin' | jq -r .token)

curl -sk https://localhost:9543/api-portal/api/v0.9/apis \
  -H "Authorization: Bearer $TOKEN"
```

The portal verifies that token against the Platform API's public key
(`auth.local.public_key_path`), so both components must be pointed at the same key pair,
and the portal's `organization.handle` must match the organization in the Platform API's
file-auth configuration.

**IDP mode** (`auth.mode = "idp"`). The token comes from the OIDC identity provider
configured under `[api_portal.auth.idp]` — obtained through that provider's own
authorization code flow, not from the portal. Its authorization and token endpoints are
the ones in your provider's `.well-known/openid-configuration` document — configure them
as `auth.idp.authorization_url` and `auth.idp.token_url`.

**Which scopes a caller holds** depends on `auth.authorization.mode`. In the default
`role` mode the token's own `scope` claim is ignored: its roles claim (named by
`auth.claim_mappings.roles`) is expanded through the portal's role-to-scope grant table
(`auth.authorization.role_to_scope_mapping`), so an identity provider that knows nothing
about `dp:*` scopes can still authorize every operation. In `scope` mode the token must
itself carry the scope — register the scopes below with your identity provider and
request them at login.

|Scope|Scope Description|
|---|---|
|`dp:organization:read`|Read organizations.|
|`dp:organization:create`|Create organizations.|
|`dp:organization:update`|Update organizations.|
|`dp:organization:delete`|Delete organizations.|
|`dp:organization:manage`|Manage organizations (including creating, updating, and deleting).|
|`dp:organization_content:read`|Read organization theme assets.|
|`dp:organization_content:manage`|Apply or reset organization theme.|
|`dp:api:read`|Read API metadata.|
|`dp:api:create`|Create API metadata.|
|`dp:api:update`|Update API metadata.|
|`dp:api:delete`|Delete API metadata.|
|`dp:api:manage`|Manage API metadata.|
|`dp:api_content:read`|Read API content.|
|`dp:api_content:create`|Create API content.|
|`dp:api_content:update`|Update API content.|
|`dp:api_content:delete`|Delete API content.|
|`dp:api_content:manage`|Manage API content.|
|`dp:mcp_server:read`|Read MCP server metadata.|
|`dp:mcp_server:create`|Create MCP server metadata.|
|`dp:mcp_server:update`|Update MCP server metadata.|
|`dp:mcp_server:delete`|Delete MCP server metadata.|
|`dp:mcp_server:manage`|Manage MCP server metadata.|
|`dp:mcp_server_content:read`|Read MCP server content.|
|`dp:mcp_server_content:create`|Create MCP server content.|
|`dp:mcp_server_content:update`|Update MCP server content.|
|`dp:mcp_server_content:delete`|Delete MCP server content.|
|`dp:mcp_server_content:manage`|Manage MCP server content.|
|`dp:mcp_server_key:read`|Read MCP server API keys.|
|`dp:mcp_server_key:create`|Generate MCP server API keys.|
|`dp:mcp_server_key:update`|Regenerate MCP server API keys.|
|`dp:mcp_server_key:revoke`|Revoke MCP server API keys.|
|`dp:mcp_server_key:manage`|Manage MCP server API keys.|
|`dp:subscription_plan:read`|Read subscription plans.|
|`dp:subscription_plan:create`|Create subscription plans.|
|`dp:subscription_plan:update`|Update subscription plans.|
|`dp:subscription_plan:delete`|Delete subscription plans.|
|`dp:subscription_plan:manage`|Manage subscription plans.|
|`dp:label:read`|Read labels.|
|`dp:label:create`|Create labels.|
|`dp:label:update`|Update labels.|
|`dp:label:delete`|Delete labels.|
|`dp:label:manage`|Manage labels.|
|`dp:application:read`|Read applications.|
|`dp:application:create`|Create applications.|
|`dp:application:update`|Update applications.|
|`dp:application:delete`|Delete applications.|
|`dp:application:manage`|Manage applications.|
|`dp:subscription:read`|Read subscriptions.|
|`dp:subscription:create`|Create subscriptions.|
|`dp:subscription:update`|Update subscriptions.|
|`dp:subscription:delete`|Delete subscriptions.|
|`dp:subscription:manage`|Manage subscriptions.|
|`dp:api_key:read`|Read API keys.|
|`dp:api_key:create`|Generate API keys.|
|`dp:api_key:update`|Regenerate API keys.|
|`dp:api_key:revoke`|Revoke API keys.|
|`dp:api_key:manage`|Manage API keys.|
|`dp:application_key_mapping:read`|Read application key mappings.|
|`dp:application_key_mapping:create`|Create application key mappings.|
|`dp:application_key_mapping:manage`|Manage application key mappings.|
|`dp:view:read`|Read views.|
|`dp:view:create`|Create views.|
|`dp:view:update`|Update views.|
|`dp:view:delete`|Delete views.|
|`dp:view:manage`|Manage views.|
|`dp:application_key:create`|Generate and create application keys.|
|`dp:application_key:update`|Update application keys.|
|`dp:application_key:revoke`|Revoke application keys.|
|`dp:application_key:manage`|Manage application keys.|
|`dp:api_workflow:read`|Read API workflows.|
|`dp:api_workflow:create`|Create or generate API workflows.|
|`dp:api_workflow:update`|Update API workflows.|
|`dp:api_workflow:delete`|Delete API workflows.|
|`dp:api_workflow:manage`|Manage API workflows.|
|`dp:event:read`|Read webhook events and delivery details.|
|`dp:key_manager:read`|Read key manager configurations.|
|`dp:key_manager:create`|Create key manager configurations.|
|`dp:key_manager:update`|Update key manager configurations.|
|`dp:key_manager:delete`|Delete key manager configurations.|
|`dp:key_manager:manage`|Manage key manager configurations (including creating, updating, and deleting).|
|`dp:webhook_subscriber:read`|Read webhook subscriber configurations.|
|`dp:webhook_subscriber:create`|Create webhook subscriber configurations.|
|`dp:webhook_subscriber:update`|Update webhook subscriber configurations.|
|`dp:webhook_subscriber:delete`|Delete webhook subscriber configurations.|
|`dp:webhook_subscriber:manage`|Manage webhook subscriber configurations (including creating, updating, and deleting).|
