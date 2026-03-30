<h1 id="gateway-controller-management-api-llm-provider-management">LLM Provider Management</h1>

CRUD operations for LLM Provider configurations

## Create a new LLM provider

<a id="opIdcreateLLMProvider"></a>

`POST /llm-providers`

> Code samples

```shell

curl -X POST http://localhost:9090/llm-providers \
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
    "name": "weather-api-v1.0",
    "labels": {
      "environment": "production",
      "team": "backend",
      "version": "v1"
    }
  },
  "spec": {
    "displayName": "WSO2 OpenAI Provider",
    "version": "v1.0",
    "context": "/openai",
    "vhost": "api.openai",
    "template": "openai",
    "upstream": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "hostRewrite": "auto",
      "auth": {
        "type": "api-key",
        "header": "string",
        "value": "string"
      }
    },
    "accessControl": {
      "mode": "deny_all",
      "exceptions": [
        {
          "path": "/chat/completions",
          "methods": [
            "GET"
          ]
        }
      ]
    },
    "policies": [
      {
        "name": "budgetControl",
        "version": "v1.0.0",
        "paths": [
          {
            "path": "/chat/completions",
            "methods": [
              "GET"
            ],
            "params": {}
          }
        ]
      }
    ],
    "deploymentState": "deployed"
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
|body|body|[LLMProviderConfiguration](schemas.md#schemallmproviderconfiguration)|true|LLM provider in YAML or JSON format|

> Example responses

> 201 Response

```json
{
  "status": "success",
  "message": "LLM provider created successfully",
  "id": "openai",
  "createdAt": "2025-11-25T10:30:00Z"
}
```

<h3 id="create-a-new-llm-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|LLM provider created and deployed successfully|[LLMProviderCreateResponse](schemas.md#schemallmprovidercreateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - Provider with same name and version already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List all LLM providers

<a id="opIdlistLLMProviders"></a>

`GET /llm-providers`

> Code samples

```shell

curl -X GET http://localhost:9090/llm-providers \
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
      "id": "openai-provider",
      "displayName": "WSO2 OpenAI Provider",
      "version": "v1.0",
      "template": "openai",
      "status": "deployed",
      "createdAt": "2025-11-25T10:30:00Z",
      "updatedAt": "2025-11-25T10:30:00Z"
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
|» providers|[[LLMProviderListItem](schemas.md#schemallmproviderlistitem)]|false|none|none|
|»» id|string|false|none|none|
|»» displayName|string|false|none|none|
|»» version|string|false|none|none|
|»» template|string|false|none|none|
|»» status|string|false|none|none|
|»» createdAt|string(date-time)|false|none|none|
|»» updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|deployed|
|status|undeployed|

## Get LLM provider by identifier

<a id="opIdgetLLMProviderById"></a>

`GET /llm-providers/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/llm-providers/{id} \
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
  "status": "success",
  "provider": {
    "id": "wso2-openai",
    "configuration": {
      "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
      "kind": "LlmProvider",
      "metadata": {
        "name": "weather-api-v1.0",
        "labels": {
          "environment": "production",
          "team": "backend",
          "version": "v1"
        }
      },
      "spec": {
        "displayName": "WSO2 OpenAI Provider",
        "version": "v1.0",
        "context": "/openai",
        "vhost": "api.openai",
        "template": "openai",
        "upstream": {
          "url": "http://prod-backend:5000/api/v2",
          "ref": "string",
          "hostRewrite": "auto",
          "auth": {
            "type": "api-key",
            "header": "string",
            "value": "string"
          }
        },
        "accessControl": {
          "mode": "deny_all",
          "exceptions": [
            {
              "path": "/chat/completions",
              "methods": [
                "GET"
              ]
            }
          ]
        },
        "policies": [
          {
            "name": "budgetControl",
            "version": "v1.0.0",
            "paths": [
              {
                "path": "/chat/completions",
                "methods": [
                  "GET"
                ],
                "params": {}
              }
            ]
          }
        ],
        "deploymentState": "deployed"
      }
    },
    "deploymentStatus": "deployed",
    "metadata": {
      "createdAt": "2025-11-25T10:30:00Z",
      "updatedAt": "2025-11-25T10:30:00Z",
      "deployedAt": "2025-11-25T10:35:00Z"
    }
  }
}
```

<h3 id="get-llm-provider-by-identifier-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|LLM provider details|[LLMProviderDetailResponse](schemas.md#schemallmproviderdetailresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM provider not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an existing LLM provider

<a id="opIdupdateLLMProvider"></a>

`PUT /llm-providers/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/llm-providers/{id} \
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
    "name": "weather-api-v1.0",
    "labels": {
      "environment": "production",
      "team": "backend",
      "version": "v1"
    }
  },
  "spec": {
    "displayName": "WSO2 OpenAI Provider",
    "version": "v1.0",
    "context": "/openai",
    "vhost": "api.openai",
    "template": "openai",
    "upstream": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "hostRewrite": "auto",
      "auth": {
        "type": "api-key",
        "header": "string",
        "value": "string"
      }
    },
    "accessControl": {
      "mode": "deny_all",
      "exceptions": [
        {
          "path": "/chat/completions",
          "methods": [
            "GET"
          ]
        }
      ]
    },
    "policies": [
      {
        "name": "budgetControl",
        "version": "v1.0.0",
        "paths": [
          {
            "path": "/chat/completions",
            "methods": [
              "GET"
            ],
            "params": {}
          }
        ]
      }
    ],
    "deploymentState": "deployed"
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
|body|body|[LLMProviderConfiguration](schemas.md#schemallmproviderconfiguration)|true|Updated LLM provider|

> Example responses

> 200 Response

```json
{
  "status": "success",
  "message": "LLM provider updated successfully",
  "id": "wso2-openai-provider",
  "updatedAt": "2025-11-25T11:45:00Z"
}
```

<h3 id="update-an-existing-llm-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|LLM provider updated successfully|[LLMProviderUpdateResponse](schemas.md#schemallmproviderupdateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM provider not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete an LLM provider

<a id="opIddeleteLLMProvider"></a>

`DELETE /llm-providers/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/llm-providers/{id} \
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
