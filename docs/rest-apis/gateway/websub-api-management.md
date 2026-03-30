<h1 id="gateway-controller-management-api-websub-api-management">WebSub API Management</h1>

## Create a new WebSubAPI

<a id="opIdcreateWebSubAPI"></a>

`POST /websub-apis`

> Code samples

```shell

curl -X POST http://localhost:9090/websub-apis \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Add a new WebSubAPI to the Gateway.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "WebSubApi",
  "metadata": {
    "name": "weather-api-v1.0",
    "labels": {
      "environment": "production",
      "team": "backend",
      "version": "v1"
    }
  },
  "spec": {
    "displayName": "weather-api",
    "version": "v1.0",
    "context": "/weather",
    "vhosts": {
      "main": "api.example.com",
      "sandbox": "sandbox-api.example.com"
    },
    "channels": [
      {
        "name": "issues",
        "method": "SUB",
        "policies": [
          {
            "name": "apiKeyValidation",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ]
      }
    ],
    "policies": [
      {
        "name": "apiKeyValidation",
        "version": "v1",
        "executionCondition": "request.metadata[authenticated] != true",
        "params": {}
      }
    ]
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="create-a-new-websubapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[WebSubAPI](schemas.md#schemawebsubapi)|true|none|

> Example responses

> 201 Response

```json
{
  "status": "success",
  "message": "WebSubAPI created successfully",
  "id": "weather-websub-api",
  "createdAt": "2025-10-11T10:30:00Z"
}
```

<h3 id="create-a-new-websubapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|WebSubAPI created successfully|[WebSubAPICreateResponse](schemas.md#schemawebsubapicreateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - WebSub API with same name and version already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List all WebSubAPIs

<a id="opIdlistWebSubAPIs"></a>

`GET /websub-apis`

> Code samples

```shell

curl -X GET http://localhost:9090/websub-apis \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List WebSubAPIs registered in the Gateway, optionally filtered by name, version, context, or status.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="list-all-websubapis-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|displayName|query|string|false|Filter by WebSub API display name|
|version|query|string|false|Filter by WebSub API version|
|context|query|string|false|Filter by WebSub API context/path|
|status|query|string|false|Filter by deployment status|

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
  "count": 5,
  "apis": [
    {
      "id": "weather-websub-api",
      "displayName": "weather-websub",
      "version": "v1.0",
      "context": "/weather-events",
      "status": "deployed",
      "createdAt": "2025-10-11T10:30:00Z",
      "updatedAt": "2025-10-11T10:30:00Z"
    }
  ]
}
```

<h3 id="list-all-websubapis-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of WebSubAPIs|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-all-websubapis-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» count|integer|false|none|none|
|» apis|[[WebSubAPIListItem](schemas.md#schemawebsubapilistitem)]|false|none|none|
|»» id|string|false|none|none|
|»» displayName|string|false|none|none|
|»» version|string|false|none|none|
|»» context|string|false|none|none|
|»» status|string|false|none|none|
|»» createdAt|string(date-time)|false|none|none|
|»» updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|deployed|
|status|undeployed|

## Get WebSubAPI by id

<a id="opIdgetWebSubAPIById"></a>

`GET /websub-apis/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/websub-apis/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Get a WebSubAPI by its ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="get-websubapi-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier for the WebSub API.|

#### Detailed descriptions

**id**: Unique public identifier for the WebSub API.

> Example responses

> 200 Response

```json
{
  "status": "success",
  "api": {
    "id": "weather-websub-api",
    "configuration": {
      "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
      "kind": "WebSubApi",
      "metadata": {
        "name": "weather-api-v1.0",
        "labels": {
          "environment": "production",
          "team": "backend",
          "version": "v1"
        }
      },
      "spec": {
        "displayName": "weather-api",
        "version": "v1.0",
        "context": "/weather",
        "vhosts": {
          "main": "api.example.com",
          "sandbox": "sandbox-api.example.com"
        },
        "channels": [
          {
            "name": "issues",
            "method": "SUB",
            "policies": [
              {
                "name": "apiKeyValidation",
                "version": "v1",
                "executionCondition": "request.metadata[authenticated] != true",
                "params": {}
              }
            ]
          }
        ],
        "policies": [
          {
            "name": "apiKeyValidation",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ]
      }
    },
    "metadata": {
      "status": "deployed",
      "createdAt": "2025-10-11T10:30:00Z",
      "updatedAt": "2025-10-11T10:30:00Z",
      "deployedAt": "2025-10-11T10:30:05Z"
    }
  }
}
```

<h3 id="get-websubapi-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|WebSubAPI details|[WebSubAPIDetailResponse](schemas.md#schemawebsubapidetailresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|WebSubAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an existing WebSubAPI

<a id="opIdupdateWebSubAPI"></a>

`PUT /websub-apis/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/websub-apis/{id} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update an existing WebSubAPI in the Gateway.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "WebSubApi",
  "metadata": {
    "name": "weather-api-v1.0",
    "labels": {
      "environment": "production",
      "team": "backend",
      "version": "v1"
    }
  },
  "spec": {
    "displayName": "weather-api",
    "version": "v1.0",
    "context": "/weather",
    "vhosts": {
      "main": "api.example.com",
      "sandbox": "sandbox-api.example.com"
    },
    "channels": [
      {
        "name": "issues",
        "method": "SUB",
        "policies": [
          {
            "name": "apiKeyValidation",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ]
      }
    ],
    "policies": [
      {
        "name": "apiKeyValidation",
        "version": "v1",
        "executionCondition": "request.metadata[authenticated] != true",
        "params": {}
      }
    ]
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="update-an-existing-websubapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the WebSub API to update.|
|body|body|[WebSubAPI](schemas.md#schemawebsubapi)|true|none|

#### Detailed descriptions

**id**: Unique public identifier of the WebSub API to update.

> Example responses

> 200 Response

```json
{
  "status": "success",
  "message": "WebSubAPI updated successfully",
  "id": "weather-websub-api",
  "updatedAt": "2025-10-11T11:45:00Z"
}
```

<h3 id="update-an-existing-websubapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|WebSubAPI updated successfully|[WebSubAPIUpdateResponse](schemas.md#schemawebsubapiupdateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|WebSubAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete a WebSubAPI

<a id="opIddeleteWebSubAPI"></a>

`DELETE /websub-apis/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/websub-apis/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete a WebSubAPI from the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="delete-a-websubapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the WebSub API to delete.|

#### Detailed descriptions

**id**: Unique public identifier of the WebSub API to delete.

> Example responses

> 200 Response

```json
{
  "status": "success",
  "message": "WebSubAPI deleted successfully",
  "id": "weather-websub-api"
}
```

<h3 id="delete-a-websubapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|WebSubAPI deleted successfully|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|WebSubAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-a-websubapi-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» id|string|false|none|none|
