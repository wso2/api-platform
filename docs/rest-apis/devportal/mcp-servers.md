<h1 id="wso2-api-developer-portal-core-devportal-routes-mcp-servers">MCP Servers</h1>

## Create MCP server metadata

<a id="opIdcreateMcpServer"></a>

`POST /mcp-servers`

> Code samples

```shell

curl -X POST https://localhost:3000/api/v0.9/mcp-servers \
  -u {username}:{password} \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates Developer Portal MCP server metadata. Mirrors `POST /api/v0.9/apis` — same artifact ZIP, YAML (`api.yaml` / `devportal.yaml` / `mcp.yaml`), and `apiMetadata` JSON input formats — but the created record is always typed `MCP`, regardless of what `type` is supplied.

> Payload

```yaml
api: string
apiDefinition: string
artifact: string
schemaDefinition: string
apiMetadata: '{"name":"Weather API","version":"v1","description":"Weather
  forecast API","type":"REST","agentVisibility":"VISIBLE",
  "status":"PUBLISHED","tags":["weather"],"labels":["default"],"endPoints":{
  "productionURL":"https://api.example.com/weather",
  "sandboxURL":"https://sandbox.example.com/weather"},"subscriptionPlans":[{"id":"Gold"}]}'

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-mcp-server-metadata-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|API metadata upload. Send either `artifact`, or `api` with `apiDefinition`, or `apiMetadata` with `apiDefinition`. `schemaDefinition` is used for MCP APIs and GraphQL schema updates.|
|» api|body|string(binary)|false|API metadata YAML file.|
|» apiDefinition|body|string(binary)|false|API definition file.|
|» artifact|body|string(binary)|false|Full API ZIP artifact containing metadata and definition files.|
|» schemaDefinition|body|string(binary)|false|Schema definition file, used by MCP APIs.|
|» apiMetadata|body|string|false|JSON string accepted by the service when the `api` YAML file is not supplied. Accepted top-level fields: `name`, `version`, `description`, `type`, `agentVisibility`, `status`, `referenceId`, `id`, `tags`, `labels`, `owners`, `endPoints` (productionURL, sandboxURL), and `subscriptionPlans` (array of `{ id }` objects — only `id` is read; the plan must already exist in the organization). `id` becomes the API's stored handle; when the API is created from a YAML artifact instead, the handle is always taken from `metadata.name`.|

> Example responses

> 201 Response

```json
{
  "id": "weather-api-v1",
  "refId": "cp-api-12345",
  "name": "Weather API",
  "apiTitle": "Weather Forecast API",
  "version": "v1",
  "status": "PUBLISHED",
  "description": "Weather forecast API.",
  "type": "MCP",
  "agentVisibility": "VISIBLE",
  "tags": [
    "weather"
  ],
  "labels": [
    "default"
  ],
  "endPoints": {
    "productionURL": "https://api.example.com/weather",
    "sandboxURL": "https://sandbox.example.com/weather"
  },
  "subscriptionPlans": [
    {
      "id": "Gold"
    }
  ]
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "status": "error",
    "code": "COMMON_VALIDATION_ERROR",
    "message": "Input validation failed.",
    "errors": [
      {
        "field": "name",
        "message": "name is required."
      }
    ]
  }
]
```

```json
{
  "status": "error",
  "code": "MISSING_REQUIRED_PARAMETER",
  "message": "Missing required parameter."
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "ORG_NOT_FOUND",
  "message": "Organization not found."
}
```

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "Conflict"
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_SERVER_ERROR",
  "message": "An unexpected error occurred."
}
```

<h3 id="create-mcp-server-metadata-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created MCP server metadata payload returned by the service. Always typed `MCP`.|[ApiMetadataCreateResponse](schemas.md#schemaapimetadatacreateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-mcp-server-metadata-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the created MCP server metadata resource.|

## List MCP server metadata

<a id="opIdgetAllMcpServersForOrganization"></a>

`GET /mcp-servers`

> Code samples

```shell

curl -X GET https://localhost:3000/api/v0.9/mcp-servers \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Lists MCP server metadata for an organization. Mirrors `GET /api/v0.9/apis` but only returns MCP-typed records.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-mcp-server-metadata-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|query|query|string|false|Free-text API metadata search term.|
|apiName|query|string|false|Exact API name filter.|
|version|query|string|false|Exact API version filter.|
|tags|query|string|false|Comma-separated tag names. Matches APIs tagged with any of the given names.|
|view|query|string|false|Developer Portal view name used to filter visible APIs.|
|limit|query|integer|false|Maximum number of records to return.|
|offset|query|integer|false|Number of records to skip before returning results.|

> Example responses

> 200 Response

```json
{
  "list": [
    {
      "id": "weather-api-v1",
      "refId": "cp-api-12345",
      "name": "Weather API",
      "version": "v1",
      "status": "PUBLISHED",
      "description": "Weather forecast API.",
      "type": "MCP",
      "agentVisibility": "VISIBLE",
      "labels": [
        "default"
      ],
      "endPoints": {
        "sandboxURL": "https://sandbox.example.com/weather",
        "productionURL": "https://api.example.com/weather"
      }
    }
  ],
  "pagination": {
    "total": 1,
    "limit": 20,
    "offset": 0
  }
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "status": "error",
    "code": "COMMON_VALIDATION_ERROR",
    "message": "Input validation failed.",
    "errors": [
      {
        "field": "name",
        "message": "name is required."
      }
    ]
  }
]
```

```json
{
  "status": "error",
  "code": "MISSING_REQUIRED_PARAMETER",
  "message": "Missing required parameter."
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_SERVER_ERROR",
  "message": "An unexpected error occurred."
}
```

<h3 id="list-mcp-server-metadata-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of MCP server metadata DTOs. Always typed `MCP`.|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-mcp-server-metadata-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» list|[allOf]|false|none|none|

*allOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|[ApiInfoResponse](schemas.md#schemaapiinforesponse)|false|none|Fields are returned at the root of ApiMetadataResponse / ApiMetadataCreateResponse (not nested under an `apiInfo` key) — this schema exists only to share the field set between the two via `allOf`.|
|»»» name|string|false|none|none|
|»»» apiTitle|string¦null|false|none|none|
|»»» remotes|[object]|false|none|none|
|»»» version|string|false|none|none|
|»»» status|string|false|none|API lifecycle status.|
|»»» description|string|false|none|none|
|»»» type|string|false|none|none|
|»»» referenceId|string¦null|false|none|External reference ID. Present when the API was created from a `devportal.yaml` artifact whose `spec` block sets `referenceId` — the create response echoes the parsed YAML back.|
|»»» agentVisibility|string|false|none|none|
|»»» addedLabels|[string]|false|none|none|
|»»» removedLabels|[string]|false|none|none|
|»»» owners|[ApiOwnersResponse](schemas.md#schemaapiownersresponse)|false|none|none|
|»»»» technicalOwner|string|false|none|none|
|»»»» businessOwner|string|false|none|none|
|»»»» businessOwnerEmail|string|false|none|none|
|»»»» technicalOwnerEmail|string|false|none|none|
|»»» apiImageMetadata|[ApiImageMetadataResponse](schemas.md#schemaapiimagemetadataresponse)|false|none|none|
|»»»» **additionalProperties**|string|false|none|none|
|»»» tags|[string]|false|none|none|
|»»» labels|[string]|false|none|none|

*and*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|object|false|none|none|
|»»» id|string|false|none|The API's handle (unique per org). Not the internal database uuid.|
|»»» refId|string¦null|false|none|Platform API (Control Plane) reference ID for this API. Used for MCP registry visibility filtering and included in outbound webhook event payloads. Null/absent for APIs that exist only in the Developer Portal and are not registered with the Platform API — e.g. MCP servers published via the registry.|
|»»» dataSource|string¦null|false|none|Indicates which content matched the search term: `METADATA` if the match was in the API's own metadata, or a content type (e.g. a value from the API Content `type` field) if the match was inside an uploaded content file. Only computed by getAllApiMetadataForOrganization when both the `query` search parameter is supplied and the database is PostgreSQL — absent on SQLite (the dev default) and absent from every other operation (get/create/update single API).|
|»»» planId|string|false|none|none|
|»»» endPoints|[ApiEndpointsResponse](schemas.md#schemaapiendpointsresponse)|false|none|none|
|»»»» sandboxURL|string|false|none|none|
|»»»» productionURL|string|false|none|none|
|»»» subscriptionPlans|[[SubscriptionPlanResponse](schemas.md#schemasubscriptionplanresponse)]|false|none|none|
|»»»» id|string|false|none|The plan's handle (unique per org). Not the internal database uuid.|
|»»»» displayName|string|false|none|none|
|»»»» description|string|false|none|none|
|»»»» limits|[object]|false|none|Rate/quota limits enforced for this plan. Empty when the plan is unlimited.|
|»»»»» limitType|string|false|none|none|
|»»»»» limitCount|any|false|none|Returned as a string when the stored count exceeds the safe integer range, otherwise a number. Unlimited plans have no limit entries — the `limits` array is empty.|

*oneOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»»»» *anonymous*|integer|false|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»»»» *anonymous*|string|false|none|none|

*continued*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»»» timeUnit|string¦null|false|none|none|
|»»»»» timeAmount|integer|false|none|none|
|»»»» refId|string¦null|false|none|Platform API subscription plan UUID associated with this plan.|
|»»»» orgId|string|false|none|none|
|»»»» createdBy|string|false|none|Identity of the user who created this subscription plan, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses and list items.|
|»»»» updatedBy|string|false|none|Identity of the user who last updated this subscription plan, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses only, omitted on list items.|
|»»»» createdAt|string(date-time)|false|none|none|
|»»»» updatedAt|string(date-time)|false|none|none|
|»»» createdBy|string|false|none|Identity of the user who created this API, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses and list items.|
|»»» updatedBy|string|false|none|Identity of the user who last updated this API, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses only, omitted on list items.|
|»»» createdAt|string(date-time)|false|none|none|
|»»» updatedAt|string(date-time)|false|none|none|
|» pagination|[Pagination](schemas.md#schemapagination)|false|none|Standard pagination metadata returned with collection responses.|
|»» total|integer|true|none|Total number of records matching the query.|
|»» limit|integer|true|none|Maximum number of records returned in this response.|
|»» offset|integer|true|none|Number of records skipped before this page.|

#### Enumerated Values

|Property|Value|
|---|---|
|status|PUBLISHED|
|status|DEPRECATED|
|type|REST|
|type|SOAP|
|type|MCP|
|type|WS|
|type|WEBSUB|
|type|GRAPHQL|
|agentVisibility|VISIBLE|
|agentVisibility|HIDDEN|
|limitType|REQUEST_COUNT|
|limitType|EVENT_COUNT|
|limitType|BANDWIDTH|
|limitType|TOTAL_TOKEN_COUNT|
|timeUnit|MINUTE|
|timeUnit|HOUR|
|timeUnit|DAY|
|timeUnit|MONTH|
|timeUnit|null|

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Get MCP server metadata

<a id="opIdgetMcpServer"></a>

`GET /mcp-servers/{mcpServerId}`

> Code samples

```shell

curl -X GET https://localhost:3000/api/v0.9/mcp-servers/{mcpServerId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves a single MCP server metadata record by Developer Portal MCP server ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-mcp-server-metadata-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|mcpServerId|path|string|true|The MCP server's handle (unique per org).|

> Example responses

> 200 Response

```json
{
  "id": "weather-api-v1",
  "refId": "cp-api-12345",
  "name": "Weather API",
  "apiTitle": "Weather Forecast API",
  "remotes": [],
  "version": "v1",
  "status": "PUBLISHED",
  "description": "Weather forecast API.",
  "type": "MCP",
  "agentVisibility": "VISIBLE",
  "labels": [
    "default"
  ],
  "endPoints": {
    "sandboxURL": "https://sandbox.example.com/weather",
    "productionURL": "https://api.example.com/weather"
  },
  "subscriptionPlans": [
    {
      "id": "Gold"
    }
  ]
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "status": "error",
    "code": "COMMON_VALIDATION_ERROR",
    "message": "Input validation failed.",
    "errors": [
      {
        "field": "name",
        "message": "name is required."
      }
    ]
  }
]
```

```json
{
  "status": "error",
  "code": "MISSING_REQUIRED_PARAMETER",
  "message": "Missing required parameter."
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 404 Response

```
"string"
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_SERVER_ERROR",
  "message": "An unexpected error occurred."
}
```

<h3 id="get-mcp-server-metadata-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|MCP server metadata DTO returned by the service. Always typed `MCP`.|[ApiMetadataResponse](schemas.md#schemaapimetadataresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Plain text success response.|string|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="get-mcp-server-metadata-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Update MCP server metadata

<a id="opIdupdateMcpServer"></a>

`PUT /mcp-servers/{mcpServerId}`

> Code samples

```shell

curl -X PUT https://localhost:3000/api/v0.9/mcp-servers/{mcpServerId} \
  -u {username}:{password} \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates Developer Portal MCP server metadata and its stored definition. Mirrors `PUT /api/v0.9/apis/{apiId}`.

> Payload

```yaml
api: string
apiDefinition: string
artifact: string
schemaDefinition: string
apiMetadata: '{"name":"Weather API","version":"v1","description":"Weather
  forecast API","type":"REST","agentVisibility":"VISIBLE",
  "status":"PUBLISHED","tags":["weather"],"labels":["default"],"endPoints":{
  "productionURL":"https://api.example.com/weather",
  "sandboxURL":"https://sandbox.example.com/weather"},"subscriptionPlans":[{"id":"Gold"}]}'

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="update-mcp-server-metadata-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|API metadata upload. Send either `artifact`, or `api` with `apiDefinition`, or `apiMetadata` with `apiDefinition`. `schemaDefinition` is used for MCP APIs and GraphQL schema updates.|
|» api|body|string(binary)|false|API metadata YAML file.|
|» apiDefinition|body|string(binary)|false|API definition file.|
|» artifact|body|string(binary)|false|Full API ZIP artifact containing metadata and definition files.|
|» schemaDefinition|body|string(binary)|false|Schema definition file, used by MCP APIs.|
|» apiMetadata|body|string|false|JSON string accepted by the service when the `api` YAML file is not supplied. Accepted top-level fields: `name`, `version`, `description`, `type`, `agentVisibility`, `status`, `referenceId`, `id`, `tags`, `labels`, `owners`, `endPoints` (productionURL, sandboxURL), and `subscriptionPlans` (array of `{ id }` objects — only `id` is read; the plan must already exist in the organization). `id` becomes the API's stored handle; when the API is created from a YAML artifact instead, the handle is always taken from `metadata.name`.|
|mcpServerId|path|string|true|The MCP server's handle (unique per org).|

> Example responses

> 200 Response

```json
{
  "id": "weather-api-v1",
  "refId": "cp-api-12345",
  "name": "Weather API",
  "apiTitle": "Weather Forecast API",
  "remotes": [],
  "version": "v1",
  "status": "PUBLISHED",
  "description": "Weather forecast API.",
  "type": "MCP",
  "agentVisibility": "VISIBLE",
  "labels": [
    "default"
  ],
  "endPoints": {
    "sandboxURL": "https://sandbox.example.com/weather",
    "productionURL": "https://api.example.com/weather"
  },
  "subscriptionPlans": [
    {
      "id": "Gold"
    }
  ]
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "status": "error",
    "code": "COMMON_VALIDATION_ERROR",
    "message": "Input validation failed.",
    "errors": [
      {
        "field": "name",
        "message": "name is required."
      }
    ]
  }
]
```

```json
{
  "status": "error",
  "code": "MISSING_REQUIRED_PARAMETER",
  "message": "Missing required parameter."
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "ORG_NOT_FOUND",
  "message": "Organization not found."
}
```

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "Conflict"
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_SERVER_ERROR",
  "message": "An unexpected error occurred."
}
```

<h3 id="update-mcp-server-metadata-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|MCP server metadata DTO returned by the service. Always typed `MCP`.|[ApiMetadataResponse](schemas.md#schemaapimetadataresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-mcp-server-metadata-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Delete MCP server metadata

<a id="opIddeleteMcpServer"></a>

`DELETE /mcp-servers/{mcpServerId}`

> Code samples

```shell

curl -X DELETE https://localhost:3000/api/v0.9/mcp-servers/{mcpServerId} \
  -u {username}:{password} \
  -H 'Accept: text/plain' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes MCP server metadata when the MCP server has no active subscriptions.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-mcp-server-metadata-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|mcpServerId|path|string|true|The MCP server's handle (unique per org).|

> Example responses

> 200 Response

```
"string"
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "status": "error",
    "code": "COMMON_VALIDATION_ERROR",
    "message": "Input validation failed.",
    "errors": [
      {
        "field": "name",
        "message": "name is required."
      }
    ]
  }
]
```

```json
{
  "status": "error",
  "code": "MISSING_REQUIRED_PARAMETER",
  "message": "Missing required parameter."
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "ORG_NOT_FOUND",
  "message": "Organization not found."
}
```

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "Conflict"
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_SERVER_ERROR",
  "message": "An unexpected error occurred."
}
```

<h3 id="delete-mcp-server-metadata-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Plain text success response.|string|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-mcp-server-metadata-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|
