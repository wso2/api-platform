<h1 id="gateway-controller-management-api-llm-provider-management">LLM Provider Management</h1>

CRUD operations for LLM Provider configurations

## Create a new LLM provider

<a id="opIdcreateLLMProvider"></a>

`POST /llm-providers`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v0.9/llm-providers \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Add a new LLM provider to the Gateway. A provider defines how to interact with an LLM service, including upstream endpoints, authentication, access control, and policies.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProvider",
  "metadata": {
    "name": "wso2-openai-provider"
  },
  "spec": {
    "displayName": "OpenAI Provider",
    "version": "v1.0",
    "template": "openai",
    "context": "/openai/latest",
    "upstream": {
      "url": "https://api.openai.com/v1",
      "auth": {
        "type": "api-key",
        "header": "Authorization",
        "value": "Bearer sk-your-api-key"
      }
    },
    "accessControl": {
      "mode": "deny_all",
      "exceptions": [
        {
          "path": "/chat/completions",
          "methods": [
            "POST"
          ]
        },
        {
          "path": "/models",
          "methods": [
            "GET"
          ]
        },
        {
          "path": "/models/{modelId}",
          "methods": [
            "GET"
          ]
        }
      ]
    }
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="create-a-new-llm-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[LLMProviderConfigurationRequest](schemas.md#schemallmproviderconfigurationrequest)|true|LLM provider in YAML or JSON format|

> Example responses

> 201 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProvider",
  "metadata": {
    "name": "wso2-openai-provider"
  },
  "spec": {
    "displayName": "OpenAI Provider",
    "version": "v1.0",
    "template": "openai",
    "context": "/openai/latest",
    "upstream": {
      "url": "https://api.openai.com/v1",
      "auth": {
        "type": "api-key",
        "header": "Authorization",
        "value": "Bearer sk-your-api-key"
      }
    },
    "accessControl": {
      "mode": "deny_all",
      "exceptions": [
        {
          "path": "/chat/completions",
          "methods": [
            "POST"
          ]
        },
        {
          "path": "/models",
          "methods": [
            "GET"
          ]
        },
        {
          "path": "/models/{modelId}",
          "methods": [
            "GET"
          ]
        }
      ]
    }
  },
  "status": {
    "id": "wso2-openai-provider",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="create-a-new-llm-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|LLM provider created and deployed successfully|[LLMProviderConfiguration](schemas.md#schemallmproviderconfiguration)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - Provider with same name and version already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List all LLM providers

<a id="opIdlistLLMProviders"></a>

`GET /llm-providers`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v0.9/llm-providers \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List LLM providers registered in the Gateway, optionally filtered by name, version, context, status, or vhost.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="list-all-llm-providers-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|displayName|query|string|false|Filter by LLM provider display name|
|version|query|string|false|Filter by LLM provider version|
|context|query|string|false|Filter by LLM provider context/path|
|status|query|string|false|Filter by deployment status|
|vhost|query|string|false|Filter by LLM provider vhost|

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
  "providers": [
    {
      "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
      "kind": "LlmProvider",
      "metadata": {
        "name": "wso2-openai-provider"
      },
      "spec": {
        "displayName": "OpenAI Provider",
        "version": "v1.0",
        "template": "openai",
        "context": "/openai/latest",
        "upstream": {
          "url": "https://api.openai.com/v1",
          "auth": {
            "type": "api-key",
            "header": "Authorization",
            "value": "Bearer sk-your-api-key"
          }
        },
        "accessControl": {
          "mode": "deny_all",
          "exceptions": [
            {
              "path": "/chat/completions",
              "methods": [
                "POST"
              ]
            },
            {
              "path": "/models",
              "methods": [
                "GET"
              ]
            },
            {
              "path": "/models/{modelId}",
              "methods": [
                "GET"
              ]
            }
          ]
        }
      },
      "status": {
        "id": "wso2-openai-provider",
        "state": "deployed",
        "createdAt": "2026-04-24T07:21:13Z",
        "updatedAt": "2026-04-24T07:21:13Z",
        "deployedAt": "2026-04-24T07:21:13Z"
      }
    }
  ]
}
```

<h3 id="list-all-llm-providers-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of LLM providers|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-all-llm-providers-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» count|integer|false|none|none|
|» providers|[allOf]|false|none|none|

*allOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|[LLMProviderConfigurationRequest](schemas.md#schemallmproviderconfigurationrequest)|false|none|none|
|»»» apiVersion|string|true|none|Provider specification version|
|»»» kind|string|true|none|Provider kind|
|»»» metadata|[Metadata](schemas.md#schemametadata)|true|none|none|
|»»»» name|string|true|none|Unique handle for the resource|
|»»»» labels|object|false|none|Labels are key-value pairs for organizing and selecting APIs. Keys must not contain spaces.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»»» annotations|object|false|none|Annotations are arbitrary non-identifying metadata. Use domain-prefixed keys.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»» spec|[LLMProviderConfigData](schemas.md#schemallmproviderconfigdata)|true|none|none|
|»»»» displayName|string|true|none|Human-readable LLM Provider name|
|»»»» version|string|true|none|Semantic version of the LLM Provider|
|»»»» context|string|false|none|Base path for all API routes (must start with /, no trailing slash)|
|»»»» vhost|string|false|none|Virtual host name used for routing. Supports standard domain names, subdomains, or wildcard domains. Must follow RFC-compliant hostname rules. Wildcards are only allowed in the left-most label (e.g., *.example.com).|
|»»»» template|string|true|none|Template name to use for this LLM Provider|
|»»»» upstream|any|true|none|none|

*allOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»»» *anonymous*|[Upstream](schemas.md#schemaupstream)|false|none|Upstream backend configuration (single target or reference)|
|»»»»»» url|string(uri)|false|none|Direct backend URL to route traffic to|
|»»»»»» ref|string|false|none|Reference to a predefined upstreamDefinition|
|»»»»»» hostRewrite|string|false|none|Controls how the Host header is handled when routing to the upstream. `auto` delegates host rewriting to Envoy, which rewrites the Host header using the upstream cluster host. `manual` disables automatic rewriting and expects explicit configuration.|

*oneOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»»»» *anonymous*|object|false|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»»»» *anonymous*|object|false|none|none|

*and*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»»» *anonymous*|[UpstreamAuth](schemas.md#schemaupstreamauth)|false|none|none|
|»»»»»» auth|object|false|none|none|
|»»»»»»» type|string|true|none|none|
|»»»»»»» header|string|false|none|none|
|»»»»»»» value|string|false|none|none|

*continued*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»» accessControl|[LLMAccessControl](schemas.md#schemallmaccesscontrol)|true|none|none|
|»»»»» mode|string|true|none|Access control mode|
|»»»»» exceptions|[[RouteException](schemas.md#schemarouteexception)]|false|none|Path exceptions to the access control mode|
|»»»»»» path|string|true|none|Path pattern|
|»»»»»» methods|[string]|true|none|HTTP methods|
|»»»» policies|[[LLMPolicy](schemas.md#schemallmpolicy)]|false|none|List of policies applied only to this operation (overrides or adds to API-level policies)|
|»»»»» name|string|true|none|none|
|»»»»» version|string|true|none|none|
|»»»»» paths|[[LLMPolicyPath](schemas.md#schemallmpolicypath)]|true|none|none|
|»»»»»» path|string|true|none|none|
|»»»»»» methods|[string]|true|none|none|
|»»»»»» params|object|true|none|JSON Schema describing the parameters accepted by this policy. This itself is a JSON Schema document.|
|»»»» deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the LLM Provider is removed from router traffic but configuration and policies are preserved for potential redeployment.|

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
|kind|LlmProvider|
|hostRewrite|auto|
|hostRewrite|manual|
|type|api-key|
|mode|allow_all|
|mode|deny_all|
|deploymentState|deployed|
|deploymentState|undeployed|
|state|deployed|
|state|undeployed|

## Get LLM provider by identifier

<a id="opIdgetLLMProviderById"></a>

`GET /llm-providers/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v0.9/llm-providers/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Get an LLM provider by its ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="get-llm-provider-by-identifier-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique identifier of the LLM provider|

> Example responses

> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProvider",
  "metadata": {
    "name": "wso2-openai-provider"
  },
  "spec": {
    "displayName": "OpenAI Provider",
    "version": "v1.0",
    "template": "openai",
    "context": "/openai/latest",
    "upstream": {
      "url": "https://api.openai.com/v1",
      "auth": {
        "type": "api-key",
        "header": "Authorization",
        "value": "Bearer sk-your-api-key"
      }
    },
    "accessControl": {
      "mode": "deny_all",
      "exceptions": [
        {
          "path": "/chat/completions",
          "methods": [
            "POST"
          ]
        },
        {
          "path": "/models",
          "methods": [
            "GET"
          ]
        },
        {
          "path": "/models/{modelId}",
          "methods": [
            "GET"
          ]
        }
      ]
    }
  },
  "status": {
    "id": "wso2-openai-provider",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="get-llm-provider-by-identifier-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|LLM provider details|[LLMProviderConfiguration](schemas.md#schemallmproviderconfiguration)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM provider not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an existing LLM provider

<a id="opIdupdateLLMProvider"></a>

`PUT /llm-providers/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v0.9/llm-providers/{id} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update an existing LLM provider in the Gateway.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProvider",
  "metadata": {
    "name": "wso2-openai-provider"
  },
  "spec": {
    "displayName": "OpenAI Provider",
    "version": "v1.0",
    "template": "openai",
    "context": "/openai/latest",
    "upstream": {
      "url": "https://api.openai.com/v1",
      "auth": {
        "type": "api-key",
        "header": "Authorization",
        "value": "Bearer sk-your-api-key"
      }
    },
    "accessControl": {
      "mode": "deny_all",
      "exceptions": [
        {
          "path": "/chat/completions",
          "methods": [
            "POST"
          ]
        },
        {
          "path": "/models",
          "methods": [
            "GET"
          ]
        },
        {
          "path": "/models/{modelId}",
          "methods": [
            "GET"
          ]
        }
      ]
    }
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="update-an-existing-llm-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique identifier of the LLM provider|
|body|body|[LLMProviderConfigurationRequest](schemas.md#schemallmproviderconfigurationrequest)|true|Updated LLM provider|

> Example responses

> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProvider",
  "metadata": {
    "name": "wso2-openai-provider"
  },
  "spec": {
    "displayName": "OpenAI Provider",
    "version": "v1.0",
    "template": "openai",
    "context": "/openai/latest",
    "upstream": {
      "url": "https://api.openai.com/v1",
      "auth": {
        "type": "api-key",
        "header": "Authorization",
        "value": "Bearer sk-your-api-key"
      }
    },
    "accessControl": {
      "mode": "deny_all",
      "exceptions": [
        {
          "path": "/chat/completions",
          "methods": [
            "POST"
          ]
        },
        {
          "path": "/models",
          "methods": [
            "GET"
          ]
        },
        {
          "path": "/models/{modelId}",
          "methods": [
            "GET"
          ]
        }
      ]
    }
  },
  "status": {
    "id": "wso2-openai-provider",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="update-an-existing-llm-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|LLM provider updated successfully|[LLMProviderConfiguration](schemas.md#schemallmproviderconfiguration)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM provider not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete an LLM provider

<a id="opIddeleteLLMProvider"></a>

`DELETE /llm-providers/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v0.9/llm-providers/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete an LLM provider from the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="delete-an-llm-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique identifier of the LLM provider|

> Example responses

> 200 Response

```json
{
  "status": "success",
  "message": "LLM provider deleted successfully",
  "id": "wso2-openai-provider"
}
```

<h3 id="delete-an-llm-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|LLM provider deleted successfully|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM provider not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-an-llm-provider-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» id|string|false|none|none|

## Create a new API key for an LLM provider

<a id="opIdcreateLLMProviderAPIKey"></a>

`POST /llm-providers/{id}/api-keys`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v0.9/llm-providers/{id}/api-keys \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Generate a new API key for an LLM provider in the Gateway.

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

<h3 id="create-a-new-api-key-for-an-llm-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of the LLM provider to generate the key for|
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

<h3 id="create-a-new-api-key-for-an-llm-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|API key created successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM provider not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict (duplicate key or conflicting update)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Get the list of API keys for an LLM provider

<a id="opIdlistLLMProviderAPIKeys"></a>

`GET /llm-providers/{id}/api-keys`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v0.9/llm-providers/{id}/api-keys \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List all API keys for an LLM provider in the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="get-the-list-of-api-keys-for-an-llm-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of the LLM provider to retrieve keys for|

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

<h3 id="get-the-list-of-api-keys-for-an-llm-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of API keys|[APIKeyListResponse](schemas.md#schemaapikeylistresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM provider not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Regenerate API key for an LLM provider

<a id="opIdregenerateLLMProviderAPIKey"></a>

`POST /llm-providers/{id}/api-keys/{apiKeyName}/regenerate`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v0.9/llm-providers/{id}/api-keys/{apiKeyName}/regenerate \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Regenerate an existing API key for an LLM provider in the Gateway.

> Payload

```json
{}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="regenerate-api-key-for-an-llm-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of the LLM provider|
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

<h3 id="regenerate-api-key-for-an-llm-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key rotated successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM provider or API key not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an API key for an LLM provider

<a id="opIdupdateLLMProviderAPIKey"></a>

`PUT /llm-providers/{id}/api-keys/{apiKeyName}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v0.9/llm-providers/{id}/api-keys/{apiKeyName} \
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

<h3 id="update-an-api-key-for-an-llm-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of the LLM provider|
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

<h3 id="update-an-api-key-for-an-llm-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key updated successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid request (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM provider or API key not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict (duplicate key or conflicting update)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Revoke an API key for an LLM provider

<a id="opIdrevokeLLMProviderAPIKey"></a>

`DELETE /llm-providers/{id}/api-keys/{apiKeyName}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v0.9/llm-providers/{id}/api-keys/{apiKeyName} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Revoke an API key. Once revoked, it can no longer be used to authenticate requests.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="revoke-an-api-key-for-an-llm-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of the LLM provider|
|apiKeyName|path|string|true|Name of the API key to revoke|

> Example responses

> 200 Response

```json
{
  "status": "success",
  "message": "API key revoked successfully"
}
```

<h3 id="revoke-an-api-key-for-an-llm-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key revoked successfully|[APIKeyRevocationResponse](schemas.md#schemaapikeyrevocationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM provider or API key not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|
