
<h1 id="wso2-api-platform-platform-api">WSO2 API Platform - Platform API v0.9.0</h1>

This document specifies a **RESTful API** for WSO2 **API Platform** - **Platform API**.

## Authentication

Most API endpoints require a Bearer JWT token in the `Authorization` header. Tokens are
validated against a configured Identity Provider (IDP) using JWKS-based signature
verification.

**Required claim**: `organization` — UUID of the caller's organization. All operations are
automatically scoped to this organization.

**Supported signing algorithms**: RS256, RS384, RS512 (RSA) and ES256, ES384, ES512 (ECDSA).
Works with any standards-compliant IDP (Keycloak, Azure AD, Okta, Auth0, WSO2 IS, etc.).

## Scope-Based Access Control

Authorization is enforced via OAuth2 scopes carried in the JWT. Both read (GET) and write
operations require specific scopes declared in the `security` field of each operation.

Base URLs:
* <a href="https://localhost:9243/api/v0.9">https://localhost:9243/api/v0.9</a>
* <a href="https://api.platform.com/api/v0.9">https://api.platform.com/api/v0.9</a>

License: <a href="https://www.apache.org/licenses/LICENSE-2.0.html">Apache 2.0</a>

## Table of Contents

### [Authentication](authentication.md)

- [Overview](authentication.md#overview)
- [Authentication Modes](authentication.md#authentication-modes)
- [Required Claims](authentication.md#required-claims)
- [Authorization](authentication.md#authorization)
- [Error Responses](authentication.md#error-responses)
- [Troubleshooting](authentication.md#troubleshooting)
- [Scope Reference](authentication.md#scope-reference)

### [Organizations](organizations.md)

- [Register a new organization](organizations.md#register-a-new-organization)
- [List organizations](organizations.md#list-organizations)
- [Get organization by ID](organizations.md#get-organization-by-id)
- [Check if organization exists](organizations.md#check-if-organization-exists)

### [Projects](projects.md)

- [Create a new project](projects.md#create-a-new-project)
- [Get all projects for current user's organization](projects.md#get-all-projects-for-current-users-organization)
- [Get project by ID](projects.md#get-project-by-id)
- [Update project](projects.md#update-project)
- [Delete project](projects.md#delete-project)

### [Applications](applications.md)

- [Create a new application](applications.md#create-a-new-application)
- [Get applications for current user's organization](applications.md#get-applications-for-current-users-organization)
- [Get application by handle](applications.md#get-application-by-handle)
- [Update application](applications.md#update-application)
- [Delete application](applications.md#delete-application)
- [List application API key mappings](applications.md#list-application-api-key-mappings)
- [Add application API key mappings](applications.md#add-application-api-key-mappings)
- [Remove application API key mapping](applications.md#remove-application-api-key-mapping)
- [List application associations](applications.md#list-application-associations)
- [Add application associations](applications.md#add-application-associations)
- [Remove application association](applications.md#remove-application-association)
- [List application API key mappings for an association](applications.md#list-application-api-key-mappings-for-an-association)

### [REST APIs](rest-apis.md)

- [Get all REST APIs for an organization](rest-apis.md#get-all-rest-apis-for-an-organization)
- [Create a new REST API](rest-apis.md#create-a-new-rest-api)
- [Get REST API by ID](rest-apis.md#get-rest-api-by-id)
- [Update REST API](rest-apis.md#update-rest-api)
- [Delete REST API](rest-apis.md#delete-rest-api)
- [Get gateways for REST API](rest-apis.md#get-gateways-for-rest-api)
- [Add gateways for REST API](rest-apis.md#add-gateways-for-rest-api)
- [Create API key](rest-apis.md#create-api-key)
- [Update API key](rest-apis.md#update-api-key)
- [Revoke API key](rest-apis.md#revoke-api-key)

### [REST API Deployments](rest-api-deployments.md)

- [Create and deploy a new deployment](rest-api-deployments.md#create-and-deploy-a-new-deployment)
- [Get deployments for a REST API](rest-api-deployments.md#get-deployments-for-a-rest-api)
- [Get deployment by ID](rest-api-deployments.md#get-deployment-by-id)
- [Delete deployment](rest-api-deployments.md#delete-deployment)
- [Undeploy deployment from gateway](rest-api-deployments.md#undeploy-deployment-from-gateway)
- [Restore a previous deployment](rest-api-deployments.md#restore-a-previous-deployment)

### [Gateways](gateways.md)

- [Register a new gateway](gateways.md#register-a-new-gateway)
- [List all gateways](gateways.md#list-all-gateways)
- [Get gateway by ID](gateways.md#get-gateway-by-id)
- [Update gateway](gateways.md#update-gateway)
- [Delete gateway](gateways.md#delete-gateway)
- [Get gateway policy manifest](gateways.md#get-gateway-policy-manifest)

### [Gateway Tokens](gateway-tokens.md)

- [List active gateway tokens](gateway-tokens.md#list-active-gateway-tokens)
- [Rotate gateway token](gateway-tokens.md#rotate-gateway-token)
- [Revoke gateway token](gateway-tokens.md#revoke-gateway-token)

### [Gateway Policies](gateway-policies.md)

- [Get synced custom policies for the current organization](gateway-policies.md#get-synced-custom-policies-for-the-current-organization)
- [Sync a custom policy from the gateway manifest](gateway-policies.md#sync-a-custom-policy-from-the-gateway-manifest)
- [Get a specific custom policy version](gateway-policies.md#get-a-specific-custom-policy-version)
- [Delete a specific custom policy version](gateway-policies.md#delete-a-specific-custom-policy-version)

### [LLM Providers](llm-providers.md)

- [Create a new LLM provider](llm-providers.md#create-a-new-llm-provider)
- [List all LLM providers](llm-providers.md#list-all-llm-providers)
- [Get LLM provider by identifier](llm-providers.md#get-llm-provider-by-identifier)
- [Update an existing LLM provider](llm-providers.md#update-an-existing-llm-provider)
- [Delete an LLM provider](llm-providers.md#delete-an-llm-provider)
- [List LLM proxies by provider](llm-providers.md#list-llm-proxies-by-provider)
- [Create a new API key for an LLM provider](llm-providers.md#create-a-new-api-key-for-an-llm-provider)
- [List API keys for an LLM provider](llm-providers.md#list-api-keys-for-an-llm-provider)
- [Delete an API key for an LLM provider](llm-providers.md#delete-an-api-key-for-an-llm-provider)

### [LLM Proxies](llm-proxies.md)

- [Create a new LLM proxy](llm-proxies.md#create-a-new-llm-proxy)
- [List all LLM proxies](llm-proxies.md#list-all-llm-proxies)
- [Get LLM proxy by unique identifier](llm-proxies.md#get-llm-proxy-by-unique-identifier)
- [Update an existing LLM proxy](llm-proxies.md#update-an-existing-llm-proxy)
- [Delete an LLM proxy](llm-proxies.md#delete-an-llm-proxy)
- [Create a new API key for an LLM proxy](llm-proxies.md#create-a-new-api-key-for-an-llm-proxy)
- [List API keys for an LLM proxy](llm-proxies.md#list-api-keys-for-an-llm-proxy)
- [Delete an API key for an LLM proxy](llm-proxies.md#delete-an-api-key-for-an-llm-proxy)

### [LLM Provider Templates](llm-provider-templates.md)

- [Create a new LLM provider template family](llm-provider-templates.md#create-a-new-llm-provider-template-family)
- [List templates, list a family's versions, or get a single version](llm-provider-templates.md#list-templates-list-a-familys-versions-or-get-a-single-version)
- [Create a new version by copying an existing one](llm-provider-templates.md#create-a-new-version-by-copying-an-existing-one)
- [Get LLM provider template by id](llm-provider-templates.md#get-llm-provider-template-by-id)
- [Update an existing LLM provider template](llm-provider-templates.md#update-an-existing-llm-provider-template)
- [Enable or disable a template version](llm-provider-templates.md#enable-or-disable-a-template-version)
- [Delete a template version](llm-provider-templates.md#delete-a-template-version)

### [LLM Provider Deployments](llm-provider-deployments.md)

- [Create and deploy a new LLM provider deployment](llm-provider-deployments.md#create-and-deploy-a-new-llm-provider-deployment)
- [Get deployments for an LLM provider](llm-provider-deployments.md#get-deployments-for-an-llm-provider)
- [Get LLM provider deployment by ID](llm-provider-deployments.md#get-llm-provider-deployment-by-id)
- [Delete LLM provider deployment](llm-provider-deployments.md#delete-llm-provider-deployment)
- [Undeploy LLM provider deployment from gateway](llm-provider-deployments.md#undeploy-llm-provider-deployment-from-gateway)
- [Restore a previous LLM provider deployment](llm-provider-deployments.md#restore-a-previous-llm-provider-deployment)

### [LLM Proxy Deployments](llm-proxy-deployments.md)

- [Create and deploy a new LLM proxy deployment](llm-proxy-deployments.md#create-and-deploy-a-new-llm-proxy-deployment)
- [Get deployments for an LLM proxy](llm-proxy-deployments.md#get-deployments-for-an-llm-proxy)
- [Get LLM proxy deployment by ID](llm-proxy-deployments.md#get-llm-proxy-deployment-by-id)
- [Delete LLM proxy deployment](llm-proxy-deployments.md#delete-llm-proxy-deployment)
- [Undeploy LLM proxy deployment from gateway](llm-proxy-deployments.md#undeploy-llm-proxy-deployment-from-gateway)
- [Restore a previous LLM proxy deployment](llm-proxy-deployments.md#restore-a-previous-llm-proxy-deployment)

### [API Keys](api-keys.md)

- [List API keys for the current user, or for all users with `ap:api_key:all:manage`](api-keys.md#list-api-keys-for-the-current-user-or-for-all-users-with-apapikeyallmanage)

### [MCP Proxies](mcp-proxies.md)

- [Create a new MCP proxy](mcp-proxies.md#create-a-new-mcp-proxy)
- [List all MCP proxies](mcp-proxies.md#list-all-mcp-proxies)
- [Get MCP proxy by unique identifier](mcp-proxies.md#get-mcp-proxy-by-unique-identifier)
- [Update an existing MCP proxy](mcp-proxies.md#update-an-existing-mcp-proxy)
- [Delete an MCP proxy](mcp-proxies.md#delete-an-mcp-proxy)
- [Fetch server info from MCP proxy backend services](mcp-proxies.md#fetch-server-info-from-mcp-proxy-backend-services)

### [MCP Proxy Deployments](mcp-proxy-deployments.md)

- [Create and deploy a new deployment for MCP proxy](mcp-proxy-deployments.md#create-and-deploy-a-new-deployment-for-mcp-proxy)
- [Get deployments for an MCP proxy](mcp-proxy-deployments.md#get-deployments-for-an-mcp-proxy)
- [Get deployment by ID](mcp-proxy-deployments.md#get-deployment-by-id)
- [Delete deployment](mcp-proxy-deployments.md#delete-deployment)
- [Undeploy deployment from gateway](mcp-proxy-deployments.md#undeploy-deployment-from-gateway)
- [Restore a previous deployment](mcp-proxy-deployments.md#restore-a-previous-deployment)

### [SubscriptionPlans](subscriptionplans.md)

- [Create subscription plan](subscriptionplans.md#create-subscription-plan)
- [List subscription plans](subscriptionplans.md#list-subscription-plans)
- [Get subscription plan by ID](subscriptionplans.md#get-subscription-plan-by-id)
- [Update subscription plan](subscriptionplans.md#update-subscription-plan)
- [Delete subscription plan](subscriptionplans.md#delete-subscription-plan)

### [Subscriptions](subscriptions.md)

- [Create subscription](subscriptions.md#create-subscription)
- [List subscriptions](subscriptions.md#list-subscriptions)
- [Get subscription by ID](subscriptions.md#get-subscription-by-id)
- [Update subscription](subscriptions.md#update-subscription)
- [Delete subscription](subscriptions.md#delete-subscription)

### [Secrets](secrets.md)

- [Create a secret](secrets.md#create-a-secret)
- [List secrets](secrets.md#list-secrets)
- [Get a secret by handle](secrets.md#get-a-secret-by-handle)
- [Rotate a secret value](secrets.md#rotate-a-secret-value)
- [Delete a secret](secrets.md#delete-a-secret)

### [Schemas](schemas.md)

- [APIKeyItem](schemas.md#apikeyitem)
- [UserAPIKeyItem](schemas.md#userapikeyitem)
- [UserAPIKeyListResponse](schemas.md#userapikeylistresponse)
- [LLMProviderAPIKeyListResponse](schemas.md#llmproviderapikeylistresponse)
- [LLMProxyAPIKeyListResponse](schemas.md#llmproxyapikeylistresponse)
- [Organization](schemas.md#organization)
- [CreateProjectRequest](schemas.md#createprojectrequest)
- [Project](schemas.md#project)
- [ApplicationType](schemas.md#applicationtype)
- [CreateApplicationRequest](schemas.md#createapplicationrequest)
- [Application](schemas.md#application)
- [ApplicationListResponse](schemas.md#applicationlistresponse)
- [AddApplicationAPIKeysRequest](schemas.md#addapplicationapikeysrequest)
- [AddApplicationAssociationsRequest](schemas.md#addapplicationassociationsrequest)
- [ApplicationAssociationSelector](schemas.md#applicationassociationselector)
- [ApplicationAssociation](schemas.md#applicationassociation)
- [ApplicationAssociationListResponse](schemas.md#applicationassociationlistresponse)
- [APIKeyMappingSelector](schemas.md#apikeymappingselector)
- [APIKeyMappingAssociatedEntity](schemas.md#apikeymappingassociatedentity)
- [AssociatedEntity](schemas.md#associatedentity)
- [MappedAPIKey](schemas.md#mappedapikey)
- [MappedAPIKeyListResponse](schemas.md#mappedapikeylistresponse)
- [RESTAPI](schemas.md#restapi)
- [SecurityConfig](schemas.md#securityconfig)
- [APIKeySecurity](schemas.md#apikeysecurity)
- [Operation](schemas.md#operation)
- [Channel](schemas.md#channel)
- [OperationRequest](schemas.md#operationrequest)
- [ChannelRequest](schemas.md#channelrequest)
- [Policy](schemas.md#policy)
- [AddGatewayToRESTAPIRequest](schemas.md#addgatewaytorestapirequest)
- [RESTAPIGatewayListResponse](schemas.md#restapigatewaylistresponse)
- [RESTAPIGatewayResponse](schemas.md#restapigatewayresponse)
- [RESTAPIDeploymentDetails](schemas.md#restapideploymentdetails)
- [CreateGatewayRequest](schemas.md#creategatewayrequest)
- [CustomPolicyResponse](schemas.md#custompolicyresponse)
- [GatewayPolicyDefinition](schemas.md#gatewaypolicydefinition)
- [ManifestSyncResponse](schemas.md#manifestsyncresponse)
- [GatewayResponse](schemas.md#gatewayresponse)
- [Pagination](schemas.md#pagination)
- [GatewayListResponse](schemas.md#gatewaylistresponse)
- [GatewayStatusResponse](schemas.md#gatewaystatusresponse)
- [GatewayStatusListResponse](schemas.md#gatewaystatuslistresponse)
- [ProjectListResponse](schemas.md#projectlistresponse)
- [OrganizationListResponse](schemas.md#organizationlistresponse)
- [RESTAPIListResponse](schemas.md#restapilistresponse)
- [TokenRotationResponse](schemas.md#tokenrotationresponse)
- [TokenInfoResponse](schemas.md#tokeninforesponse)
- [CreateRESTAPIRequest](schemas.md#createrestapirequest)
- [TimeUnit](schemas.md#timeunit)
- [ExpirationDuration](schemas.md#expirationduration)
- [CreateAPIKeyRequest](schemas.md#createapikeyrequest)
- [CreateAPIKeyResponse](schemas.md#createapikeyresponse)
- [UpdateAPIKeyRequest](schemas.md#updateapikeyrequest)
- [UpdateAPIKeyResponse](schemas.md#updateapikeyresponse)
- [SubscriptionPlanLimit](schemas.md#subscriptionplanlimit)
- [CreateSubscriptionPlanRequest](schemas.md#createsubscriptionplanrequest)
- [SubscriptionPlan](schemas.md#subscriptionplan)
- [SubscriptionPlanListResponse](schemas.md#subscriptionplanlistresponse)
- [CreateSubscriptionRequest](schemas.md#createsubscriptionrequest)
- [Subscription](schemas.md#subscription)
- [SubscriptionListResponse](schemas.md#subscriptionlistresponse)
- [Error](schemas.md#error)
- [FieldError](schemas.md#fielderror)
- [DeployRequest](schemas.md#deployrequest)
- [DeploymentResponse](schemas.md#deploymentresponse)
- [DeploymentListResponse](schemas.md#deploymentlistresponse)
- [Upstream](schemas.md#upstream)
- [UpstreamDefinition](schemas.md#upstreamdefinition)
- [UpstreamAuth](schemas.md#upstreamauth)
- [ExtractionIdentifier](schemas.md#extractionidentifier)
- [LLMProviderTemplateAuth](schemas.md#llmprovidertemplateauth)
- [LLMProviderTemplateMetadata](schemas.md#llmprovidertemplatemetadata)
- [LLMProviderTemplateResourceMapping](schemas.md#llmprovidertemplateresourcemapping)
- [LLMProviderTemplateResourceMappings](schemas.md#llmprovidertemplateresourcemappings)
- [LLMProviderTemplate](schemas.md#llmprovidertemplate)
- [CreateLLMProviderTemplateVersionRequest](schemas.md#createllmprovidertemplateversionrequest)
- [LLMProviderTemplateListItem](schemas.md#llmprovidertemplatelistitem)
- [LLMProviderTemplateListResponse](schemas.md#llmprovidertemplatelistresponse)
- [LLMAccessControl](schemas.md#llmaccesscontrol)
- [RouteException](schemas.md#routeexception)
- [LLMPolicy](schemas.md#llmpolicy)
- [LLMPolicyPath](schemas.md#llmpolicypath)
- [OperationPolicy](schemas.md#operationpolicy)
- [OperationPolicyPath](schemas.md#operationpolicypath)
- [LLMRateLimitingConfig](schemas.md#llmratelimitingconfig)
- [RateLimitingScopeConfig](schemas.md#ratelimitingscopeconfig)
- [ResourceWiseRateLimitingConfig](schemas.md#resourcewiseratelimitingconfig)
- [RateLimitingResourceLimit](schemas.md#ratelimitingresourcelimit)
- [RateLimitResetWindow](schemas.md#ratelimitresetwindow)
- [RequestRateLimitDimension](schemas.md#requestratelimitdimension)
- [TokenRateLimitDimension](schemas.md#tokenratelimitdimension)
- [CostRateLimitDimension](schemas.md#costratelimitdimension)
- [RateLimitingLimitConfig](schemas.md#ratelimitinglimitconfig)
- [LLMProvider](schemas.md#llmprovider)
- [AssociatedGateway](schemas.md#associatedgateway)
- [LLMProviderListItem](schemas.md#llmproviderlistitem)
- [LLMModelProvider](schemas.md#llmmodelprovider)
- [LLMModel](schemas.md#llmmodel)
- [LLMProviderListResponse](schemas.md#llmproviderlistresponse)
- [LLMProxy](schemas.md#llmproxy)
- [LLMProxyListItem](schemas.md#llmproxylistitem)
- [LLMProxyListResponse](schemas.md#llmproxylistresponse)
- [LLMProxyProvider](schemas.md#llmproxyprovider)
- [LLMProxyAdditionalProvider](schemas.md#llmproxyadditionalprovider)
- [LLMProxyTransformer](schemas.md#llmproxytransformer)
- [CreateLLMProviderAPIKeyRequest](schemas.md#createllmproviderapikeyrequest)
- [CreateLLMProviderAPIKeyResponse](schemas.md#createllmproviderapikeyresponse)
- [CreateLLMProxyAPIKeyRequest](schemas.md#createllmproxyapikeyrequest)
- [CreateLLMProxyAPIKeyResponse](schemas.md#createllmproxyapikeyresponse)
- [MCPProxy](schemas.md#mcpproxy)
- [MCPProxyListItem](schemas.md#mcpproxylistitem)
- [MCPProxyListResponse](schemas.md#mcpproxylistresponse)
- [MCPServerInfoFetchRequest](schemas.md#mcpserverinfofetchrequest)
- [MCPServerInfoFetchResponse](schemas.md#mcpserverinfofetchresponse)
- [MCPProxyCapabilities](schemas.md#mcpproxycapabilities)
- [SecretCreateRequest](schemas.md#secretcreaterequest)
- [SecretUpdateRequest](schemas.md#secretupdaterequest)
- [SecretResponse](schemas.md#secretresponse)
- [SecretSummary](schemas.md#secretsummary)
- [SecretListResponse](schemas.md#secretlistresponse)
- [GatewayTokenListResponse](schemas.md#gatewaytokenlistresponse)
- [CustomPolicyListResponse](schemas.md#custompolicylistresponse)

