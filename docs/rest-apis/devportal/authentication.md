# Authentication

- oAuth2 authentication. OAuth2/OIDC access token with fine-grained Developer Portal scopes. Each operation declares the exact resource/action scope it requires.

    - Flow: authorizationCode
    - Authorization URL = [https://localhost:9443/oauth2/authorize](https://localhost:9443/oauth2/authorize)
    - Token URL = [https://localhost:9443/oauth2/token](https://localhost:9443/oauth2/token)

|Scope|Scope Description|
|---|---|
|dp:org_read|Read organizations.|
|dp:org_write|Create or update organizations.|
|dp:org_delete|Delete organizations.|
|dp:org_manage|Manage organizations (including creating, updating, and deleting).|
|dp:idp_read|Read organization identity provider configuration.|
|dp:idp_write|Create or update identity provider configuration.|
|dp:idp_delete|Delete identity provider configuration.|
|dp:idp_manage|Manage organization identity provider configuration.|
|dp:provider_read|Read providers.|
|dp:provider_write|Create or update providers.|
|dp:provider_delete|Delete providers.|
|dp:provider_manage|Manage providers (including creating, updating, and deleting).|
|dp:org_content_read|Read organization layout or content.|
|dp:org_content_write|Create or update organization layout or content.|
|dp:org_content_delete|Delete organization layout or content.|
|dp:org_content_manage|Manage organization layout or content.|
|dp:api_read|Read API metadata.|
|dp:api_write|Create or update API metadata.|
|dp:api_delete|Delete API metadata.|
|dp:api_manage|Manage API metadata.|
|dp:api_content_read|Read API content.|
|dp:api_content_write|Create or update API content.|
|dp:api_content_delete|Delete API content.|
|dp:api_content_manage|Manage API content.|
|dp:sub_policy_read|Read subscription policies.|
|dp:sub_policy_write|Create or update subscription policies.|
|dp:sub_policy_delete|Delete subscription policies.|
|dp:sub_policy_manage|Manage subscription policies.|
|dp:label_read|Read labels.|
|dp:label_write|Create or update labels.|
|dp:label_delete|Delete labels.|
|dp:label_manage|Manage labels.|
|dp:app_read|Read applications.|
|dp:app_write|Create or update applications.|
|dp:app_delete|Delete applications.|
|dp:app_import|Import applications.|
|dp:app_manage|Manage applications.|
|dp:subscription_read|Read subscriptions.|
|dp:subscription_write|Create or update subscriptions.|
|dp:subscription_delete|Delete subscriptions.|
|dp:subscription_manage|Manage subscriptions.|
|dp:api_key_read|Read API keys.|
|dp:api_key_write|Generate or regenerate API keys.|
|dp:api_key_revoke|Revoke API keys.|
|dp:api_key_manage|Manage API keys.|
|dp:app_key_mapping_read|Read application key mappings.|
|dp:app_key_mapping_write|Create application key mappings.|
|dp:app_key_mapping_manage|Manage application key mappings.|
|dp:view_read|Read views.|
|dp:view_write|Create or update views.|
|dp:view_delete|Delete views.|
|dp:view_manage|Manage views.|
|dp:app_key_write|Generate, update, regenerate, or clean up application keys.|
|dp:app_key_revoke|Revoke application keys.|
|dp:app_key_manage|Manage application keys.|
|dp:billing_read|Read billing information, usage data, payment methods, and subscription billing status.|
|dp:billing_write|Create checkout sessions, register checkout sessions, create billing portal sessions, or cancel paid subscriptions.|
|dp:billing_manage|Manage billing information, checkout, portal, and subscription billing actions.|
|dp:billing_config_read|Read billing engine key configuration.|
|dp:billing_config_write|Create or update billing engine key configuration.|
|dp:billing_config_delete|Delete billing engine key configuration.|
|dp:billing_config_manage|Manage billing engine key configuration.|
|dp:usage_read|Read subscription usage.|
|dp:usage_manage|Manage subscription usage access.|
|dp:invoice_read|Read invoices and invoice links.|
|dp:invoice_manage|Manage invoice access.|
|dp:api_flow_read|Read API flows.|
|dp:api_flow_write|Create, update, or generate API flows.|
|dp:api_flow_delete|Delete API flows.|
|dp:api_flow_manage|Manage API flows.|
|dp:event_read|Read webhook events and delivery details.|
|dp:delivery_manage|Retry failed webhook deliveries.|
|dp:utility_write|Create temporary utility files.|
|dp:utility_manage|Manage utility operations.|
|dp:km_read|Read key manager configurations.|
|dp:km_write|Create or update key manager configurations.|
|dp:km_delete|Delete key manager configurations.|
|dp:km_manage|Manage key manager configurations (including creating, updating, and deleting).|

* API Key (apiKeyAuth)
    - Parameter Name: **x-api-key**, in: header. API key authentication. Server-side authorization should bind each key to the same fine-grained permissions used by OAuth2 scopes.
