<h1 id="wso2-api-developer-portal-core-devportal-routes-apis">APIs</h1>

## Create API metadata

<a id="opIdcreateApiMetadata"></a>

`POST /devportal/v1/apis`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/devportal/v1/apis \
  -u {username}:{password} \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates Developer Portal API metadata from either a full API artifact ZIP, an API metadata YAML file (`api.yaml` / `devportal.yaml` / `mcp.yaml`), or an `apiMetadata` JSON string. An API definition file is required unless supplied by the artifact ZIP. The YAML `spec` block accepts: `displayName`, `version`, `description`, `type`, `status`, `agentVisibility`, `tags`, `labels`, `referenceId`, `endpoints` (sandboxUrl, productionUrl), `businessInformation` (owners), and `subscriptionPlans`. The service also stores labels, subscription plan mappings, image metadata, and schema definitions for GraphQL APIs when provided. MCP servers must be created via `POST /devportal/v1/mcp-servers` instead — a request whose resolved `type` is `MCP` is rejected with `400`.
`subscriptionPlans` links existing org-level plans to this API by name — it does not create plans. In YAML it is a string array (`["Gold", "Silver"]`). In the JSON `apiMetadata` field it is an object array where only `id` is used (`[{"id":"Gold"}]`); extra fields such as `planId`, `displayName`, or `requestCount` are ignored.

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

<h3 id="create-api-metadata-parameters">Parameters</h3>

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
  "type": "REST",
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

<h3 id="create-api-metadata-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created API metadata payload returned by the service.|[ApiMetadataCreateResponse](schemas.md#schemaapimetadatacreateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-api-metadata-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the created API metadata resource.|

## List API metadata

<a id="opIdgetAllApiMetadataForOrganization"></a>

`GET /devportal/v1/apis`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/devportal/v1/apis \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Lists API metadata for an organization. The service supports exact filters by API name, version, and tags, free-text search with `query`, and view filtering. Unknown query parameters are rejected. MCP-typed records are never returned here — use `GET /devportal/v1/mcp-servers`.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-api-metadata-parameters">Parameters</h3>

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
      "type": "REST",
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

<h3 id="list-api-metadata-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of API metadata DTOs.|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-api-metadata-responseschema">Response Schema</h3>

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
|»»»» name|string|false|none|none|
|»»»» description|string|false|none|none|
|»»»» requestCount|string¦null|false|none|Always stored and returned as a string ("Unlimited" or a numeric string), regardless of the type (request-count or event-count) used to create the plan. Null if not set.|
|»»»» refId|string¦null|false|none|Platform API subscription plan UUID associated with this plan.|
|»»»» orgId|string|false|none|none|

*continued*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
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

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Get API metadata

<a id="opIdgetApiMetadata"></a>

`GET /devportal/v1/apis/{apiId}`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/devportal/v1/apis/{apiId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves a single API metadata record by Developer Portal API ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-api-metadata-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiId|path|string|true|The API's handle (unique per org). Resolves only to REST/SOAP/WS/WebSub/GraphQL APIs — MCP servers are addressed via `/mcp-servers`.|

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
  "type": "REST",
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

<h3 id="get-api-metadata-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API metadata DTO returned by the service.|[ApiMetadataResponse](schemas.md#schemaapimetadataresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Plain text success response.|string|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="get-api-metadata-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Update API metadata

<a id="opIdupdateApiMetadata"></a>

`PUT /devportal/v1/apis/{apiId}`

> Code samples

```shell

curl -X PUT https://devportal.api-platform.io/devportal/v1/apis/{apiId} \
  -u {username}:{password} \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates Developer Portal API metadata and its stored definition. Accepts the same YAML spec fields and `apiMetadata` JSON format as the create operation. The update flow can also adjust label mappings, subscription plan mappings, schema definitions, and image metadata. Status changes to unpublished are rejected when active subscriptions exist.

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

<h3 id="update-api-metadata-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|API metadata upload. Send either `artifact`, or `api` with `apiDefinition`, or `apiMetadata` with `apiDefinition`. `schemaDefinition` is used for MCP APIs and GraphQL schema updates.|
|» api|body|string(binary)|false|API metadata YAML file.|
|» apiDefinition|body|string(binary)|false|API definition file.|
|» artifact|body|string(binary)|false|Full API ZIP artifact containing metadata and definition files.|
|» schemaDefinition|body|string(binary)|false|Schema definition file, used by MCP APIs.|
|» apiMetadata|body|string|false|JSON string accepted by the service when the `api` YAML file is not supplied. Accepted top-level fields: `name`, `version`, `description`, `type`, `agentVisibility`, `status`, `referenceId`, `id`, `tags`, `labels`, `owners`, `endPoints` (productionURL, sandboxURL), and `subscriptionPlans` (array of `{ id }` objects — only `id` is read; the plan must already exist in the organization). `id` becomes the API's stored handle; when the API is created from a YAML artifact instead, the handle is always taken from `metadata.name`.|
|apiId|path|string|true|The API's handle (unique per org). Resolves only to REST/SOAP/WS/WebSub/GraphQL APIs — MCP servers are addressed via `/mcp-servers`.|

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
  "type": "REST",
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

<h3 id="update-api-metadata-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API metadata DTO returned by the service.|[ApiMetadataResponse](schemas.md#schemaapimetadataresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-api-metadata-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Delete API metadata

<a id="opIddeleteApiMetadata"></a>

`DELETE /devportal/v1/apis/{apiId}`

> Code samples

```shell

curl -X DELETE https://devportal.api-platform.io/devportal/v1/apis/{apiId} \
  -u {username}:{password} \
  -H 'Accept: text/plain' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes API metadata when the API has no active subscriptions.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-api-metadata-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiId|path|string|true|The API's handle (unique per org). Resolves only to REST/SOAP/WS/WebSub/GraphQL APIs — MCP servers are addressed via `/mcp-servers`.|

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

<h3 id="delete-api-metadata-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Plain text success response.|string|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-api-metadata-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|
