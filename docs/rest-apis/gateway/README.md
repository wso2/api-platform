
<h1 id="gateway-controller-management-api">Gateway Controller Management API v1.0.0</h1>

REST API for managing API configurations in the WSO2 API Platform Gateway.

Base URLs:
* <a href="http://localhost:9090">http://localhost:9090</a>
* <a href="http://gateway-controller:9090">http://gateway-controller:9090</a>

## Table of Contents

### [Authentication](authentication.md)

- [Overview](authentication.md#overview)
- [How It Works](authentication.md#how-it-works)
- [Configuration](authentication.md#configuration)
- [Role Mapping Semantics](authentication.md#role-mapping-semantics)
- [Troubleshooting (What you’ll observe)](authentication.md#troubleshooting-what-youll-observe)
- [Testing](authentication.md#testing)

### [Rest API Management](rest-api-management.md)

- [Create a new RestAPI](rest-api-management.md#create-a-new-restapi)
- [List all RestAPIs](rest-api-management.md#list-all-restapis)
- [Get RestAPI by id](rest-api-management.md#get-restapi-by-id)
- [Update an existing RestAPI](rest-api-management.md#update-an-existing-restapi)
- [Delete a RestAPI](rest-api-management.md#delete-a-restapi)
- [Create a new API key for an API](rest-api-management.md#create-a-new-api-key-for-an-api)
- [Get the list of API keys for an API](rest-api-management.md#get-the-list-of-api-keys-for-an-api)
- [Regenerate API key for an API](rest-api-management.md#regenerate-api-key-for-an-api)
- [Update an API key with a new regenerated value](rest-api-management.md#update-an-api-key-with-a-new-regenerated-value)
- [Revoke an API key](rest-api-management.md#revoke-an-api-key)
- [Create a subscription plan](rest-api-management.md#create-a-subscription-plan)
- [List subscription plans](rest-api-management.md#list-subscription-plans)
- [Get a subscription plan by ID](rest-api-management.md#get-a-subscription-plan-by-id)
- [Update a subscription plan](rest-api-management.md#update-a-subscription-plan)
- [Delete a subscription plan](rest-api-management.md#delete-a-subscription-plan)
- [Create a subscription](rest-api-management.md#create-a-subscription)
- [List subscriptions](rest-api-management.md#list-subscriptions)
- [Get a subscription by ID](rest-api-management.md#get-a-subscription-by-id)
- [Update a subscription](rest-api-management.md#update-a-subscription)
- [Delete a subscription](rest-api-management.md#delete-a-subscription)

### [MCP Proxy Management](mcp-proxy-management.md)

- [Create a new MCPProxy](mcp-proxy-management.md#create-a-new-mcpproxy)
- [List all MCPProxies](mcp-proxy-management.md#list-all-mcpproxies)
- [Get MCPProxy by id](mcp-proxy-management.md#get-mcpproxy-by-id)
- [Update an existing MCPProxy](mcp-proxy-management.md#update-an-existing-mcpproxy)
- [Delete a MCPProxy](mcp-proxy-management.md#delete-a-mcpproxy)

### [Certificate Management](certificate-management.md)

- [List all custom certificates](certificate-management.md#list-all-custom-certificates)
- [Upload a new certificate](certificate-management.md#upload-a-new-certificate)
- [Delete a certificate](certificate-management.md#delete-a-certificate)
- [Manually reload certificates](certificate-management.md#manually-reload-certificates)

### [LLM Provider Template Management](llm-provider-template-management.md)

- [Create a new LLM provider template](llm-provider-template-management.md#create-a-new-llm-provider-template)
- [List all LLM provider templates](llm-provider-template-management.md#list-all-llm-provider-templates)
- [Get LLM provider template by id](llm-provider-template-management.md#get-llm-provider-template-by-id)
- [Update an existing LLM provider template](llm-provider-template-management.md#update-an-existing-llm-provider-template)
- [Delete an LLM provider template](llm-provider-template-management.md#delete-an-llm-provider-template)

### [LLM Provider Management](llm-provider-management.md)

- [Create a new LLM provider](llm-provider-management.md#create-a-new-llm-provider)
- [List all LLM providers](llm-provider-management.md#list-all-llm-providers)
- [Get LLM provider by identifier](llm-provider-management.md#get-llm-provider-by-identifier)
- [Update an existing LLM provider](llm-provider-management.md#update-an-existing-llm-provider)
- [Delete an LLM provider](llm-provider-management.md#delete-an-llm-provider)
- [Create a new API key for an LLM provider](llm-provider-management.md#create-a-new-api-key-for-an-llm-provider)
- [Get the list of API keys for an LLM provider](llm-provider-management.md#get-the-list-of-api-keys-for-an-llm-provider)
- [Regenerate API key for an LLM provider](llm-provider-management.md#regenerate-api-key-for-an-llm-provider)
- [Update an API key for an LLM provider](llm-provider-management.md#update-an-api-key-for-an-llm-provider)
- [Revoke an API key for an LLM provider](llm-provider-management.md#revoke-an-api-key-for-an-llm-provider)

### [LLM Proxy Management](llm-proxy-management.md)

- [Create a new LLM proxy](llm-proxy-management.md#create-a-new-llm-proxy)
- [List all LLM proxies](llm-proxy-management.md#list-all-llm-proxies)
- [Get LLM proxy by unique identifier](llm-proxy-management.md#get-llm-proxy-by-unique-identifier)
- [Update an existing LLM proxy](llm-proxy-management.md#update-an-existing-llm-proxy)
- [Delete an LLM proxy](llm-proxy-management.md#delete-an-llm-proxy)
- [Create a new API key for an LLM proxy](llm-proxy-management.md#create-a-new-api-key-for-an-llm-proxy)
- [Get the list of API keys for an LLM proxy](llm-proxy-management.md#get-the-list-of-api-keys-for-an-llm-proxy)
- [Regenerate API key for an LLM proxy](llm-proxy-management.md#regenerate-api-key-for-an-llm-proxy)
- [Update an API key for an LLM proxy](llm-proxy-management.md#update-an-api-key-for-an-llm-proxy)
- [Revoke an API key for an LLM proxy](llm-proxy-management.md#revoke-an-api-key-for-an-llm-proxy)

### [Secrets Management](secrets-management.md)

- [List all secrets](secrets-management.md#list-all-secrets)
- [Create a new secret](secrets-management.md#create-a-new-secret)
- [Retrieve a secret](secrets-management.md#retrieve-a-secret)
- [Update a secret](secrets-management.md#update-a-secret)
- [Delete a secret](secrets-management.md#delete-a-secret)

### [WebSub API Management](websub-api-management.md)

- [Create a new WebSubAPI](websub-api-management.md#create-a-new-websubapi)
- [List all WebSubAPIs](websub-api-management.md#list-all-websubapis)
- [Get WebSubAPI by id](websub-api-management.md#get-websubapi-by-id)
- [Update an existing WebSubAPI](websub-api-management.md#update-an-existing-websubapi)
- [Delete a WebSubAPI](websub-api-management.md#delete-a-websubapi)

### [Schemas](schemas.md)

