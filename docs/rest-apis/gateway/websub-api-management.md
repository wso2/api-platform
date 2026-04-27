<h1 id="gateway-controller-management-api-websub-api-management">WebSub API Management</h1>

## Create a new WebSubAPI

<a id="opIdcreateWebSubAPI"></a>

`POST /websub-apis`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v0.9/websub-apis \
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
    "name": "github-events-v1.0"
  },
  "spec": {
    "displayName": "GitHub Events",
    "version": "v1.0",
    "context": "/github-events/$version",
    "channels": [
      {
        "name": "issues",
        "method": "SUB"
      },
      {
        "name": "pull_requests",
        "method": "SUB"
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
|body|body|[WebSubAPIRequest](schemas.md#schemawebsubapirequest)|true|none|

> Example responses

> 201 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "WebSubApi",
  "metadata": {
    "name": "github-events-v1.0"
  },
  "spec": {
    "displayName": "GitHub Events",
    "version": "v1.0",
    "context": "/github-events/$version",
    "channels": [
      {
        "name": "issues",
        "method": "SUB"
      },
      {
        "name": "pull_requests",
        "method": "SUB"
      }
    ]
  },
  "status": {
    "id": "github-events-v1.0",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="create-a-new-websubapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|WebSubAPI created successfully|[WebSubAPI](schemas.md#schemawebsubapi)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - WebSub API with same name and version already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List all WebSubAPIs

<a id="opIdlistWebSubAPIs"></a>

`GET /websub-apis`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v0.9/websub-apis \
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
      "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
      "kind": "WebSubApi",
      "metadata": {
        "name": "github-events-v1.0"
      },
      "spec": {
        "displayName": "GitHub Events",
        "version": "v1.0",
        "context": "/github-events/$version",
        "channels": [
          {
            "name": "issues",
            "method": "SUB"
          },
          {
            "name": "pull_requests",
            "method": "SUB"
          }
        ]
      },
      "status": {
        "id": "github-events-v1.0",
        "state": "deployed",
        "createdAt": "2026-04-24T07:21:13Z",
        "updatedAt": "2026-04-24T07:21:13Z",
        "deployedAt": "2026-04-24T07:21:13Z"
      }
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
|» apis|[allOf]|false|none|none|

*allOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|[WebSubAPIRequest](schemas.md#schemawebsubapirequest)|false|none|none|
|»»» apiVersion|string|true|none|API specification version|
|»»» kind|string|true|none|API type|
|»»» metadata|[Metadata](schemas.md#schemametadata)|true|none|none|
|»»»» name|string|true|none|Unique handle for the resource|
|»»»» labels|object|false|none|Labels are key-value pairs for organizing and selecting APIs. Keys must not contain spaces.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»»» annotations|object|false|none|Annotations are arbitrary non-identifying metadata. Use domain-prefixed keys.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»» spec|[WebhookAPIData](schemas.md#schemawebhookapidata)|true|none|none|
|»»»» displayName|string|true|none|Human-readable API name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)|
|»»»» version|string|true|none|Semantic version of the API|
|»»»» context|string|true|none|Base path for all API routes (must start with /, no trailing slash)|
|»»»» vhosts|object|false|none|Custom virtual hosts/domains for the API|
|»»»»» main|string|true|none|Custom virtual host/domain for production traffic|
|»»»»» sandbox|string|false|none|Custom virtual host/domain for sandbox traffic|
|»»»» channels|[[Channel](schemas.md#schemachannel)]|true|none|List of channels - Async operations(SUB) for WebSub APIs|
|»»»»» name|string|true|none|Channel name or topic identifier relative to API context.|
|»»»»» method|string|true|none|Operation method type.|
|»»»»» policies|[[Policy](schemas.md#schemapolicy)]|false|none|List of policies applied only to this channel (overrides or adds to API-level policies)|
|»»»»»» name|string|true|none|Name of the policy|
|»»»»»» version|string|true|none|Version of the policy. Only major-only version is allowed (e.g., v0, v1). Full semantic version (e.g., v1.0.0) is not accepted and will be rejected. The Gateway Controller resolves the major version to the single matching full version installed in the gateway image.|
|»»»»»» executionCondition|string|false|none|Expression controlling conditional execution of the policy|
|»»»»»» params|object|false|none|Arbitrary parameters for the policy (free-form key/value structure)|
|»»»» policies|[[Policy](schemas.md#schemapolicy)]|false|none|List of API-level policies applied to all operations unless overridden|
|»»»» deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the API is removed from router traffic but configuration, API keys, and policies are preserved for potential redeployment.|

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
|kind|WebSubApi|
|method|SUB|
|deploymentState|deployed|
|deploymentState|undeployed|
|state|deployed|
|state|undeployed|

## Get WebSubAPI by id

<a id="opIdgetWebSubAPIById"></a>

`GET /websub-apis/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v0.9/websub-apis/{id} \
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
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "WebSubApi",
  "metadata": {
    "name": "github-events-v1.0"
  },
  "spec": {
    "displayName": "GitHub Events",
    "version": "v1.0",
    "context": "/github-events/$version",
    "channels": [
      {
        "name": "issues",
        "method": "SUB"
      },
      {
        "name": "pull_requests",
        "method": "SUB"
      }
    ]
  },
  "status": {
    "id": "github-events-v1.0",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="get-websubapi-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|WebSubAPI details|[WebSubAPI](schemas.md#schemawebsubapi)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|WebSubAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an existing WebSubAPI

<a id="opIdupdateWebSubAPI"></a>

`PUT /websub-apis/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v0.9/websub-apis/{id} \
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
    "name": "github-events-v1.0"
  },
  "spec": {
    "displayName": "GitHub Events",
    "version": "v1.0",
    "context": "/github-events/$version",
    "channels": [
      {
        "name": "issues",
        "method": "SUB"
      },
      {
        "name": "pull_requests",
        "method": "SUB"
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
|body|body|[WebSubAPIRequest](schemas.md#schemawebsubapirequest)|true|none|

#### Detailed descriptions

**id**: Unique public identifier of the WebSub API to update.

> Example responses

> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "WebSubApi",
  "metadata": {
    "name": "github-events-v1.0"
  },
  "spec": {
    "displayName": "GitHub Events",
    "version": "v1.0",
    "context": "/github-events/$version",
    "channels": [
      {
        "name": "issues",
        "method": "SUB"
      },
      {
        "name": "pull_requests",
        "method": "SUB"
      }
    ]
  },
  "status": {
    "id": "github-events-v1.0",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="update-an-existing-websubapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|WebSubAPI updated successfully|[WebSubAPI](schemas.md#schemawebsubapi)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|WebSubAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete a WebSubAPI

<a id="opIddeleteWebSubAPI"></a>

`DELETE /websub-apis/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v0.9/websub-apis/{id} \
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
