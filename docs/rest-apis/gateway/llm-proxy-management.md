<h1 id="gateway-controller-management-api-llm-proxy-management">LLM Proxy Management</h1>

CRUD operations for LLM Proxy configurations

## Create a new LLM proxy

<a id="opIdcreateLLMProxy"></a>

`POST /llm-proxies`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v0.9/llm-proxies \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Add a new LLM proxy to the Gateway. A proxy defines how to interact with an LLM service deployed in the Gateway, including authentication and policies.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProxy",
  "metadata": {
    "name": "openai-proxy"
  },
  "spec": {
    "displayName": "OpenAI Proxy",
    "version": "v1.0",
    "context": "/openai-proxy",
    "provider": {
      "id": "wso2-openai-provider"
    },
    "policies": []
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="create-a-new-llm-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[LLMProxyConfigurationRequest](schemas.md#schemallmproxyconfigurationrequest)|true|LLM proxy in YAML or JSON format|

> Example responses

> 201 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProxy",
  "metadata": {
    "name": "openai-proxy"
  },
  "spec": {
    "displayName": "OpenAI Proxy",
    "version": "v1.0",
    "context": "/openai-proxy",
    "provider": {
      "id": "wso2-openai-provider"
    },
    "policies": []
  },
  "status": {
    "id": "openai-proxy",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="create-a-new-llm-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|LLM proxy created and deployed successfully|[LLMProxyConfiguration](schemas.md#schemallmproxyconfiguration)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - Proxy with same name and version already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List all LLM proxies

<a id="opIdlistLLMProxies"></a>

`GET /llm-proxies`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v0.9/llm-proxies \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List LLM proxies registered in the Gateway, optionally filtered by name, version, context, status, or vhost.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="list-all-llm-proxies-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|displayName|query|string|false|Filter by LLM proxy displayName|
|version|query|string|false|Filter by LLM proxy version|
|context|query|string|false|Filter by LLM proxy context/path|
|status|query|string|false|Filter by deployment status|
|vhost|query|string|false|Filter by LLM proxy vhost|

#### Enumerated Values

|Parameter|Value|
|---|---|
|status|deployed|
|status|undeployed|

> Example responses

> 200 Response

```json
{
  "status": "success",
  "count": 2,
  "proxies": [
    {
      "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
      "kind": "LlmProxy",
      "metadata": {
        "name": "openai-proxy"
      },
      "spec": {
        "displayName": "OpenAI Proxy",
        "version": "v1.0",
        "context": "/openai-proxy",
        "provider": {
          "id": "wso2-openai-provider"
        },
        "policies": []
      },
      "status": {
        "id": "openai-proxy",
        "state": "deployed",
        "createdAt": "2026-04-24T07:21:13Z",
        "updatedAt": "2026-04-24T07:21:13Z",
        "deployedAt": "2026-04-24T07:21:13Z"
      }
    }
  ]
}
```

<h3 id="list-all-llm-proxies-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of LLM proxies|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-all-llm-proxies-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» count|integer|false|none|none|
|» proxies|[allOf]|false|none|none|

*allOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|[LLMProxyConfigurationRequest](schemas.md#schemallmproxyconfigurationrequest)|false|none|none|
|»»» apiVersion|string|true|none|Proxy specification version|
|»»» kind|string|true|none|Proxy kind|
|»»» metadata|[Metadata](schemas.md#schemametadata)|true|none|none|
|»»»» name|string|true|none|Unique handle for the resource|
|»»»» labels|object|false|none|Labels are key-value pairs for organizing and selecting APIs. Keys must not contain spaces.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»»» annotations|object|false|none|Annotations are arbitrary non-identifying metadata. Use domain-prefixed keys.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»» spec|[LLMProxyConfigData](schemas.md#schemallmproxyconfigdata)|true|none|none|
|»»»» displayName|string|true|none|Human-readable LLM proxy name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)|
|»»»» version|string|true|none|Semantic version of the LLM proxy|
|»»»» context|string|false|none|Base path for all API routes (must start with /, no trailing slash)|
|»»»» vhost|string|false|none|Virtual host name used for routing. Supports standard domain names, subdomains, or wildcard domains. Must follow RFC-compliant hostname rules. Wildcards are only allowed in the left-most label (e.g., *.example.com).|
|»»»» provider|[LLMProxyProvider](schemas.md#schemallmproxyprovider)|true|none|none|
|»»»»» id|string|true|none|Unique id of a deployed llm provider|
|»»»»» auth|[LLMUpstreamAuth](schemas.md#schemallmupstreamauth)|false|none|none|
|»»»»»» type|string|true|none|none|
|»»»»»» header|string|false|none|none|
|»»»»»» value|string|false|none|none|
|»»»» policies|[[LLMPolicy](schemas.md#schemallmpolicy)]|false|none|List of policies applied only to this operation (overrides or adds to API-level policies)|
|»»»»» name|string|true|none|none|
|»»»»» version|string|true|none|none|
|»»»»» paths|[[LLMPolicyPath](schemas.md#schemallmpolicypath)]|true|none|none|
|»»»»»» path|string|true|none|none|
|»»»»»» methods|[string]|true|none|none|
|»»»»»» params|object|true|none|JSON Schema describing the parameters accepted by this policy. This itself is a JSON Schema document.|
|»»»» deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the LLM Proxy is removed from router traffic but configuration and policies are preserved for potential redeployment.|

*and*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|object|false|none|none|
|»»» status|[ResourceStatus](schemas.md#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|
|»»»» id|string|false|none|Unique identifier assigned by the server (equal to metadata.name)|
|»»»» state|string|false|none|Desired deployment state reported by the server|
|»»»» createdAt|string(date-time)|false|none|Timestamp when the resource was first created (UTC)|
|»»»» updatedAt|string(date-time)|false|none|Timestamp when the resource was last updated (UTC)|
|»»»» deployedAt|string(date-time)|false|none|Timestamp when the resource was last deployed (omitted when undeployed)|

#### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1alpha1|
|kind|LlmProxy|
|type|api-key|
|deploymentState|deployed|
|deploymentState|undeployed|
|state|deployed|
|state|undeployed|

## Get LLM proxy by unique identifier

<a id="opIdgetLLMProxyById"></a>

`GET /llm-proxies/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v0.9/llm-proxies/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Get an LLM proxy by its ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="get-llm-proxy-by-unique-identifier-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique identifier of the LLM proxy|

> Example responses

> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProxy",
  "metadata": {
    "name": "openai-proxy"
  },
  "spec": {
    "displayName": "OpenAI Proxy",
    "version": "v1.0",
    "context": "/openai-proxy",
    "provider": {
      "id": "wso2-openai-provider"
    },
    "policies": []
  },
  "status": {
    "id": "openai-proxy",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="get-llm-proxy-by-unique-identifier-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|LLM proxy details|[LLMProxyConfiguration](schemas.md#schemallmproxyconfiguration)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM proxy not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an existing LLM proxy

<a id="opIdupdateLLMProxy"></a>

`PUT /llm-proxies/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v0.9/llm-proxies/{id} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update an existing LLM proxy in the Gateway.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProxy",
  "metadata": {
    "name": "openai-proxy"
  },
  "spec": {
    "displayName": "OpenAI Proxy",
    "version": "v1.0",
    "context": "/openai-proxy",
    "provider": {
      "id": "wso2-openai-provider"
    },
    "policies": []
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="update-an-existing-llm-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique identifier of the LLM proxy|
|body|body|[LLMProxyConfigurationRequest](schemas.md#schemallmproxyconfigurationrequest)|true|Updated LLM proxy|

> Example responses

> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProxy",
  "metadata": {
    "name": "openai-proxy"
  },
  "spec": {
    "displayName": "OpenAI Proxy",
    "version": "v1.0",
    "context": "/openai-proxy",
    "provider": {
      "id": "wso2-openai-provider"
    },
    "policies": []
  },
  "status": {
    "id": "openai-proxy",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="update-an-existing-llm-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|LLM proxy updated successfully|[LLMProxyConfiguration](schemas.md#schemallmproxyconfiguration)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM proxy not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete an LLM proxy

<a id="opIddeleteLLMProxy"></a>

`DELETE /llm-proxies/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v0.9/llm-proxies/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete an LLM proxy from the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="delete-an-llm-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique identifier of the LLM proxy|

> Example responses

> 200 Response

```json
{
  "status": "success",
  "message": "LLM proxy deleted successfully",
  "id": "openai-proxy"
}
```

<h3 id="delete-an-llm-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|LLM proxy deleted successfully|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM proxy not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-an-llm-proxy-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» id|string|false|none|none|

## Create a new API key for an LLM proxy

<a id="opIdcreateLLMProxyAPIKey"></a>

`POST /llm-proxies/{id}/api-keys`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v0.9/llm-proxies/{id}/api-keys \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Generate a new API key for an LLM proxy in the Gateway.

> Payload

```json
{
  "name": "my-production-key"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="create-a-new-api-key-for-an-llm-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of the LLM proxy to generate the key for|
|body|body|[APIKeyCreationRequest](schemas.md#schemaapikeycreationrequest)|true|none|

> Example responses

> 201 Response

```json
{
  "status": "success",
  "message": "API key generated successfully",
  "remainingApiKeyQuota": 9,
  "apiKey": {
    "name": "my-production-key",
    "displayName": "My Production Key",
    "apiKey": "apip_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    "apiId": "reading-list-api-v1.0",
    "status": "active",
    "createdAt": "2026-04-01T10:30:00Z",
    "createdBy": "admin",
    "expiresAt": null,
    "source": "local"
  }
}
```

<h3 id="create-a-new-api-key-for-an-llm-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|API key created successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM proxy not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict (duplicate key or conflicting update)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Get the list of API keys for an LLM proxy

<a id="opIdlistLLMProxyAPIKeys"></a>

`GET /llm-proxies/{id}/api-keys`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v0.9/llm-proxies/{id}/api-keys \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List all API keys for an LLM proxy in the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="get-the-list-of-api-keys-for-an-llm-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of the LLM proxy to retrieve keys for|

> Example responses

> 200 Response

```json
{
  "apiKeys": [
    {
      "name": "my-production-key",
      "displayName": "My Production Key",
      "apiKey": "apip_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
      "apiId": "reading-list-api-v1.0",
      "status": "active",
      "createdAt": "2026-04-01T10:30:00Z",
      "createdBy": "admin",
      "expiresAt": null,
      "source": "local"
    }
  ],
  "totalCount": 3,
  "status": "success"
}
```

<h3 id="get-the-list-of-api-keys-for-an-llm-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of API keys|[APIKeyListResponse](schemas.md#schemaapikeylistresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM proxy not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Regenerate API key for an LLM proxy

<a id="opIdregenerateLLMProxyAPIKey"></a>

`POST /llm-proxies/{id}/api-keys/{apiKeyName}/regenerate`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v0.9/llm-proxies/{id}/api-keys/{apiKeyName}/regenerate \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Regenerate an existing API key for an LLM proxy in the Gateway.

> Payload

```json
{}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="regenerate-api-key-for-an-llm-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of the LLM proxy|
|apiKeyName|path|string|true|Name of the API key to regenerate|
|body|body|[APIKeyRegenerationRequest](schemas.md#schemaapikeyregenerationrequest)|true|none|

> Example responses

> 200 Response

```json
{
  "status": "success",
  "message": "API key generated successfully",
  "remainingApiKeyQuota": 9,
  "apiKey": {
    "name": "my-production-key",
    "displayName": "My Production Key",
    "apiKey": "apip_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    "apiId": "reading-list-api-v1.0",
    "status": "active",
    "createdAt": "2026-04-01T10:30:00Z",
    "createdBy": "admin",
    "expiresAt": null,
    "source": "local"
  }
}
```

<h3 id="regenerate-api-key-for-an-llm-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key rotated successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM proxy or API key not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an API key for an LLM proxy

<a id="opIdupdateLLMProxyAPIKey"></a>

`PUT /llm-proxies/{id}/api-keys/{apiKeyName}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v0.9/llm-proxies/{id}/api-keys/{apiKeyName} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update an API key with a custom value instead of auto-generating one.

> Payload

```json
{
  "name": "my-production-key"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="update-an-api-key-for-an-llm-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of the LLM proxy|
|apiKeyName|path|string|true|Name of the API key to update|
|body|body|[APIKeyUpdateRequest](schemas.md#schemaapikeyupdaterequest)|true|none|

> Example responses

> 200 Response

```json
{
  "status": "success",
  "message": "API key generated successfully",
  "remainingApiKeyQuota": 9,
  "apiKey": {
    "name": "my-production-key",
    "displayName": "My Production Key",
    "apiKey": "apip_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    "apiId": "reading-list-api-v1.0",
    "status": "active",
    "createdAt": "2026-04-01T10:30:00Z",
    "createdBy": "admin",
    "expiresAt": null,
    "source": "local"
  }
}
```

<h3 id="update-an-api-key-for-an-llm-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key updated successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid request (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM proxy or API key not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict (duplicate key or conflicting update)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Revoke an API key for an LLM proxy

<a id="opIdrevokeLLMProxyAPIKey"></a>

`DELETE /llm-proxies/{id}/api-keys/{apiKeyName}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v0.9/llm-proxies/{id}/api-keys/{apiKeyName} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Revoke an API key. Once revoked, it can no longer be used to authenticate requests.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="revoke-an-api-key-for-an-llm-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of the LLM proxy|
|apiKeyName|path|string|true|Name of the API key to revoke|

> Example responses

> 200 Response

```json
{
  "status": "success",
  "message": "API key revoked successfully"
}
```

<h3 id="revoke-an-api-key-for-an-llm-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key revoked successfully|[APIKeyRevocationResponse](schemas.md#schemaapikeyrevocationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM proxy or API key not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|
