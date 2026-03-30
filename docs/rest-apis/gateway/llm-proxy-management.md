<h1 id="gateway-controller-management-api-llm-proxy-management">LLM Proxy Management</h1>

CRUD operations for LLM Proxy configurations

## Create a new LLM proxy

<a id="opIdcreateLLMProxy"></a>

`POST /llm-proxies`

> Code samples

```shell

curl -X POST http://localhost:9090/llm-proxies \
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
    "name": "weather-api-v1.0",
    "labels": {
      "environment": "production",
      "team": "backend",
      "version": "v1"
    }
  },
  "spec": {
    "displayName": "wso2-con-assistant",
    "version": "v1.0",
    "context": "/openai",
    "vhost": "api.openai",
    "provider": {
      "id": "wso2-openai-provider",
      "auth": {
        "type": "api-key",
        "header": "string",
        "value": "string"
      }
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

Required roles: `admin`, `developer`

</aside>

<h3 id="create-a-new-llm-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[LLMProxyConfiguration](schemas.md#schemallmproxyconfiguration)|true|LLM proxy in YAML or JSON format|

> Example responses

> 201 Response

```json
{
  "status": "success",
  "message": "LLM proxy created successfully",
  "id": "wso2-con-assistant",
  "createdAt": "2025-11-25T10:30:00Z"
}
```

<h3 id="create-a-new-llm-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|LLM proxy created and deployed successfully|[LLMProxyCreateResponse](schemas.md#schemallmproxycreateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - Proxy with same name and version already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List all LLM proxies

<a id="opIdlistLLMProxies"></a>

`GET /llm-proxies`

> Code samples

```shell

curl -X GET http://localhost:9090/llm-proxies \
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
      "id": "wso2-con-assistant",
      "displayName": "WSO2 Con Assistant",
      "version": "v1.0",
      "provider": "wso2-openai-provider",
      "status": "deployed",
      "createdAt": "2025-11-25T10:30:00Z",
      "updatedAt": "2025-11-25T10:30:00Z"
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
|» proxies|[[LLMProxyListItem](schemas.md#schemallmproxylistitem)]|false|none|none|
|»» id|string|false|none|none|
|»» displayName|string|false|none|none|
|»» version|string|false|none|none|
|»» provider|string|false|none|Unique id of a deployed llm provider|
|»» status|string|false|none|none|
|»» createdAt|string(date-time)|false|none|none|
|»» updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|deployed|
|status|undeployed|

## Get LLM proxy by unique identifier

<a id="opIdgetLLMProxyById"></a>

`GET /llm-proxies/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/llm-proxies/{id} \
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
  "status": "success",
  "proxy": {
    "id": "wso2-docs-assistant",
    "configuration": {
      "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
      "kind": "LlmProxy",
      "metadata": {
        "name": "weather-api-v1.0",
        "labels": {
          "environment": "production",
          "team": "backend",
          "version": "v1"
        }
      },
      "spec": {
        "displayName": "wso2-con-assistant",
        "version": "v1.0",
        "context": "/openai",
        "vhost": "api.openai",
        "provider": {
          "id": "wso2-openai-provider",
          "auth": {
            "type": "api-key",
            "header": "string",
            "value": "string"
          }
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

<h3 id="get-llm-proxy-by-unique-identifier-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|LLM proxy details|[LLMProxyDetailResponse](schemas.md#schemallmproxydetailresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM proxy not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an existing LLM proxy

<a id="opIdupdateLLMProxy"></a>

`PUT /llm-proxies/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/llm-proxies/{id} \
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
    "name": "weather-api-v1.0",
    "labels": {
      "environment": "production",
      "team": "backend",
      "version": "v1"
    }
  },
  "spec": {
    "displayName": "wso2-con-assistant",
    "version": "v1.0",
    "context": "/openai",
    "vhost": "api.openai",
    "provider": {
      "id": "wso2-openai-provider",
      "auth": {
        "type": "api-key",
        "header": "string",
        "value": "string"
      }
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

Required roles: `admin`, `developer`

</aside>

<h3 id="update-an-existing-llm-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique identifier of the LLM proxy|
|body|body|[LLMProxyConfiguration](schemas.md#schemallmproxyconfiguration)|true|Updated LLM proxy|

> Example responses

> 200 Response

```json
{
  "status": "success",
  "message": "LLM proxy updated successfully",
  "id": "wso2-con-assistant",
  "updatedAt": "2025-11-25T11:45:00Z"
}
```

<h3 id="update-an-existing-llm-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|LLM proxy updated successfully|[LLMProxyUpdateResponse](schemas.md#schemallmproxyupdateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM proxy not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete an LLM proxy

<a id="opIddeleteLLMProxy"></a>

`DELETE /llm-proxies/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/llm-proxies/{id} \
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
  "id": "wso2-docs-assistant"
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
