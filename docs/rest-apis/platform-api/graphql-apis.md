<h1 id="wso2-api-platform-platform-api-graphql-apis">GraphQL APIs</h1>

GraphQL API management operations

## Get all GraphQL APIs for an organization

<a id="opIdListGraphQLAPIs"></a>

`GET /graphql-apis`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/graphql-apis?projectId=default-project \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves all GraphQL APIs belonging to an organization. Requires the
projectId query parameter to filter APIs by project. Access is validated
against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:graphql:read`, `ap:api:graphql:manage`, `ap:api:read`, `ap:api:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-all-graphql-apis-for-an-organization-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|projectId|query|string|true|**Project ID** consisting of the **handle** (unique slug identifier) of the Project whose resources should be returned.|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|
|sortBy|query|string|false|Field to sort the collection by. An unrecognized value falls back to the default sort (createdAt).|
|sortOrder|query|string|false|Sort direction applied to `sortBy`.|
|query|query|string|false|Case-insensitive substring filter matched against the resource display name and id (handle).|

#### Detailed descriptions

**projectId**: **Project ID** consisting of the **handle** (unique slug identifier) of the Project whose resources should be returned.

#### Enumerated Values

|Parameter|Value|
|---|---|
|sortBy|name|
|sortBy|createdAt|
|sortOrder|asc|
|sortOrder|desc|

> Example responses
>
> 200 Response

```json
{
  "count": 1,
  "list": [
    {
      "id": "countries-graphql-api",
      "displayName": "Countries GraphQL API",
      "description": "string",
      "context": "/countries/graphql",
      "version": "v1.0",
      "projectId": "default-project",
      "upstream": {
        "main": {
          "url": "http://prod-backend:5000/api/v2",
          "auth": {
            "type": "api-key",
            "header": "X-API-Key",
            "value": "my-api-key-value"
          }
        },
        "sandbox": {
          "url": "http://prod-backend:5000/api/v2",
          "auth": {
            "type": "api-key",
            "header": "X-API-Key",
            "value": "my-api-key-value"
          }
        }
      },
      "introspectionMode": "ENDPOINT",
      "kind": "GraphQLApi",
      "readOnly": false,
      "createdBy": "john.doe",
      "createdAt": "2019-08-24T14:15:22Z",
      "updatedAt": "2019-08-24T14:15:22Z"
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="get-all-graphql-apis-for-an-organization-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|GraphQL APIs retrieved successfully|[GraphQLAPIListResponse](schemas.md#schemagraphqlapilistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Create a new GraphQL API

<a id="opIdCreateGraphQLAPI"></a>

`POST /graphql-apis`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/graphql-apis \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -F 'metadata={"displayName":"Countries GraphQL API","context":"/countries","version":"v1.0","projectId":"default-project","schemaSource":"introspection","upstream":{"main":{"url":"https://countries.trevorblades.com/graphql"}}}
' \
  -F 'sdlFile=string'

```

Creates a new GraphQL API in the platform. `schemaSource` declares how the
schema is supplied: `inline` (the `sdl` field), `url` (fetched from
`sdlUrl`), `file` (the `sdlFile` multipart part), or `introspection` (the
default — `upstream.main.url` must expose standard GraphQL introspection).
Only the field matching the declared source may be present — a request
that supplies a field not matching the declared `schemaSource` (or more
than one schema field at once), omits the field/part its declared source
requires, or declares `introspection` against an `upstream.main.ref`
instead of a literal `url`, is a request-shape problem and is rejected
with `400` (`VALIDATION_FAILED`) describing exactly what's inconsistent.
Once the request shape itself is valid, schema resolution is best-effort:
if the declared source can't actually be resolved (unreachable URL,
invalid SDL, introspection failing/disabled), the API is still created
with an empty schema rather than failing — fetch it later via
`GET /graphql-apis/{graphqlApiId}/sdl` once it can be resolved. The API is
associated with a project, which must belong to the organization
specified in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:graphql:create`, `ap:api:graphql:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="create-a-new-graphql-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[GraphQLAPIMultipartRequest](schemas.md#schemagraphqlapimultipartrequest)|true|GraphQL API object that needs to be added, as `multipart/form-data` — see|

#### Detailed descriptions

**body**: GraphQL API object that needs to be added, as `multipart/form-data` — see
GraphQLAPIMultipartRequest. This is the only accepted content type, even
when `schemaSource` is `inline`, `url`, or `introspection` and no file is
being uploaded, so that every schema-source variant is expressed the
same way.

> Example responses
>
> 201 Response

```json
{
  "id": "countries-graphql-api",
  "displayName": "Countries GraphQL API",
  "description": "Public GraphQL API for querying country/region reference data",
  "context": "/countries/graphql",
  "version": "v1.0",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "projectId": "default-project",
  "createdAt": "2026-08-11T10:00:00Z",
  "updatedAt": "2026-08-11T10:00:00Z",
  "readOnly": false,
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    }
  },
  "kind": "GraphQLApi",
  "schemaSource": "introspection",
  "sdl": "type Query {\n  countries: [Country]\n  country(code: ID!): Country\n}\ntype Country {\n  code: String\n  name: String\n  capital: String\n}\n",
  "introspectionMode": "ENDPOINT",
  "policies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ],
  "subscriptionPlans": [
    "Gold",
    "Silver"
  ]
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "The request conflicts with the current state of the resource."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="create-a-new-graphql-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|GraphQL API created successfully|[GraphQLAPI](schemas.md#schemagraphqlapi)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## Get GraphQL API by ID

<a id="opIdGetGraphQLAPI"></a>

`GET /graphql-apis/{graphqlApiId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves the GraphQL API's metadata and configuration. The `sdl` field
is deliberately omitted from this response — it can be large, and most
callers only need the metadata — fetch it separately via
`GET /graphql-apis/{graphqlApiId}/sdl`.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:graphql:read`, `ap:api:graphql:manage`, `ap:api:read`, `ap:api:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-graphql-api-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|**GraphQL API ID** consisting of the **handle** (unique identifier) of the API.|

#### Detailed descriptions

**graphqlApiId**: **GraphQL API ID** consisting of the **handle** (unique identifier) of the API.

> Example responses
>
> 200 Response

```json
{
  "id": "countries-graphql-api",
  "displayName": "Countries GraphQL API",
  "description": "Public GraphQL API for querying country/region reference data",
  "context": "/countries/graphql",
  "version": "v1.0",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "projectId": "default-project",
  "createdAt": "2026-08-11T10:00:00Z",
  "updatedAt": "2026-08-11T10:00:00Z",
  "readOnly": false,
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    }
  },
  "kind": "GraphQLApi",
  "introspectionMode": "ENDPOINT",
  "schemaSource": "introspection",
  "policies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ],
  "subscriptionPlans": [
    "Gold",
    "Silver"
  ]
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="get-graphql-api-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|GraphQL API retrieved successfully|[GraphQLAPIDetail](schemas.md#schemagraphqlapidetail)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Update GraphQL API

<a id="opIdUpdateGraphQLAPI"></a>

`PUT /graphql-apis/{graphqlApiId}`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -F 'metadata={"displayName":"Countries GraphQL API","context":"/countries","version":"v1.0","projectId":"default-project","schemaSource":"introspection","upstream":{"main":{"url":"https://countries.trevorblades.com/graphql"}}}
' \
  -F 'sdlFile=string'

```

Updates an existing GraphQL API's details. `schemaSource` behaves as on
create (see `POST /graphql-apis`), including the same `400`
(`VALIDATION_FAILED`) response for a request shape that's inconsistent
with the declared `schemaSource` — re-supply `sdl`/`sdlUrl`/`sdlFile`, or
leave it as `introspection` to re-query `upstream.main.url` and pick up a
changed backend schema. If resolution fails (the source can't actually be
resolved right now), the previously-stored schema is left unchanged rather
than being cleared.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:graphql:update`, `ap:api:graphql:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="update-graphql-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|**GraphQL API ID** consisting of the **handle** (unique identifier) of the API.|
|body|body|[GraphQLAPIMultipartRequest](schemas.md#schemagraphqlapimultipartrequest)|true|As `multipart/form-data` only — see GraphQLAPIMultipartRequest and the|

#### Detailed descriptions

**graphqlApiId**: **GraphQL API ID** consisting of the **handle** (unique identifier) of the API.

**body**: As `multipart/form-data` only — see GraphQLAPIMultipartRequest and the
note on `POST /graphql-apis`.

> Example responses
>
> 200 Response

```json
{
  "id": "countries-graphql-api",
  "displayName": "Countries GraphQL API",
  "description": "Public GraphQL API for querying country/region reference data",
  "context": "/countries/graphql",
  "version": "v1.0",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "projectId": "default-project",
  "createdAt": "2026-08-11T10:00:00Z",
  "updatedAt": "2026-08-11T10:00:00Z",
  "readOnly": false,
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    }
  },
  "kind": "GraphQLApi",
  "schemaSource": "introspection",
  "sdl": "type Query {\n  countries: [Country]\n  country(code: ID!): Country\n}\ntype Country {\n  code: String\n  name: String\n  capital: String\n}\n",
  "introspectionMode": "ENDPOINT",
  "policies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ],
  "subscriptionPlans": [
    "Gold",
    "Silver"
  ]
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "The request conflicts with the current state of the resource."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="update-graphql-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|GraphQL API updated successfully|[GraphQLAPI](schemas.md#schemagraphqlapi)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete GraphQL API

<a id="opIdDeleteGraphQLAPI"></a>

`DELETE /graphql-apis/{graphqlApiId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:graphql:delete`, `ap:api:graphql:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="delete-graphql-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|**GraphQL API ID** consisting of the **handle** (unique identifier) of the API.|

#### Detailed descriptions

**graphqlApiId**: **GraphQL API ID** consisting of the **handle** (unique identifier) of the API.

> Example responses
>
> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="delete-graphql-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|GraphQL API deleted successfully|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get the SDL for a GraphQL API

<a id="opIdGetGraphQLAPISDL"></a>

`GET /graphql-apis/{graphqlApiId}/sdl`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId}/sdl \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves the GraphQL API's resolved schema in SDL form — the same text
`GET /graphql-apis/{graphqlApiId}` would have returned in its `sdl` field
before that field was split out into this dedicated endpoint (large, and
rarely needed alongside the rest of the metadata).

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:graphql:read`, `ap:api:graphql:manage`, `ap:api:read`, `ap:api:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-the-sdl-for-a-graphql-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|**GraphQL API ID** consisting of the **handle** (unique identifier) of the API.|

#### Detailed descriptions

**graphqlApiId**: **GraphQL API ID** consisting of the **handle** (unique identifier) of the API.

> Example responses
>
> 200 Response

```json
{
  "sdl": "type Query {\n  countries: [Country]\n  country(code: ID!): Country\n}\ntype Country {\n  code: String\n  name: String\n  capital: String\n}\n"
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="get-the-sdl-for-a-graphql-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|SDL retrieved successfully|[GraphQLAPISDLResponse](schemas.md#schemagraphqlapisdlresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Dry-run GraphQL schema resolution

<a id="opIdValidateGraphQLSchema"></a>

`POST /graphql-apis/validate-schema`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/graphql-apis/validate-schema \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -F 'metadata={"schemaSource":"introspection","upstream":{"main":{"url":"https://countries.trevorblades.com/graphql"}}}
' \
  -F 'sdlFile=string'

```

Attempts to resolve a schema exactly as `POST`/`PUT /graphql-apis`
would — the same `schemaSource`-driven structural validation, the
same best-effort resolution (§5.2) — without persisting anything. A
request-shape mismatch (`schemaSource` inconsistent with the fields
supplied) is a `400` (`VALIDATION_FAILED`), same as create/update. An
actual resolution failure (bad SDL, an unreachable `sdlUrl`, a failed
introspection query) is **not** an error here either — the response
reports `resolved: false` so the caller can decide what to do, rather
than having to create a real API just to find out.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:graphql:create`, `ap:api:graphql:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="dry-run-graphql-schema-resolution-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[ValidateGraphQLSchemaMultipartRequest](schemas.md#schemavalidategraphqlschemamultipartrequest)|true|As `multipart/form-data` only, following the same convention as|

#### Detailed descriptions

**body**: As `multipart/form-data` only, following the same convention as
`POST /graphql-apis` — see `GraphQLAPIMultipartRequest`.

> Example responses
>
> 200 Response

```json
{
  "resolved": true,
  "sdl": "string",
  "introspectionMode": "ENDPOINT",
  "message": "The provided endpoint could not be used to derive a GraphQL schema, or the supplied SDL could not be parsed.",
  "sdlErrors": [
    {
      "message": "Unexpected Name \"this\"",
      "line": 3,
      "column": 12
    }
  ]
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="dry-run-graphql-schema-resolution-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Schema resolution attempted — see `resolved` for the outcome.|[ValidateGraphQLSchemaResponse](schemas.md#schemavalidategraphqlschemaresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get gateways for GraphQL API

<a id="opIdGetGraphQLAPIGateways"></a>

`GET /graphql-apis/{graphqlApiId}/gateways`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId}/gateways \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves all gateways associated with the specified API, including deployment details.
Returns gateway information along with association timestamps and deployment status.
Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:graphql:gateway:read`, `ap:api:graphql:gateway:manage`, `ap:api:graphql:read`, `ap:api:graphql:manage`, `ap:api:read`, `ap:api:manage`, `ap:read`, `ap:manage`, `ap:gateway:manage`, `ap:gateway:read`

</aside>

<h3 id="get-gateways-for-graphql-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|**GraphQL API ID** consisting of the **handle** (unique identifier) of the API.|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

#### Detailed descriptions

**graphqlApiId**: **GraphQL API ID** consisting of the **handle** (unique identifier) of the API.

> Example responses
>
> 200 Response

```json
{
  "count": 3,
  "list": [
    {
      "id": "prod-gateway-01",
      "organizationId": "acme",
      "displayName": "Production Gateway 01",
      "description": "Production gateway for handling API traffic",
      "properties": {
        "region": "us-west",
        "tier": "premium"
      },
      "endpoints": [
        "https://api.example.com:8443/api/v1",
        "wss://events.example.com:8444"
      ],
      "isCritical": true,
      "functionalityType": "regular",
      "version": "1.0",
      "isActive": true,
      "createdBy": "john.doe",
      "updatedBy": "john.doe",
      "createdAt": "2025-10-14T10:30:00Z",
      "updatedAt": "2025-10-14T10:30:00Z",
      "associatedAt": "2025-10-15T10:30:00Z",
      "isDeployed": true,
      "deployment": {
        "status": "CREATED",
        "deployedAt": "2025-10-15T11:00:00Z"
      }
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="get-gateways-for-graphql-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of gateways associated with the API, including deployment details|[RESTAPIGatewayListResponse](schemas.md#schemarestapigatewaylistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Add gateways for GraphQL API

<a id="opIdAddGatewaysToGraphQLAPI"></a>

`POST /graphql-apis/{graphqlApiId}/gateways`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId}/gateways \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Associates gateways to the specified API. If gateways are already associated,
updates the association timestamp. Returns all gateways associated with the API
including deployment details. Access is validated against the organization
in the JWT token.

> Payload

```json
[
  {
    "gatewayId": "prod-gateway-01"
  }
]
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:graphql:gateway:create`, `ap:api:graphql:gateway:manage`, `ap:api:graphql:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="add-gateways-for-graphql-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|**GraphQL API ID** consisting of the **handle** (unique identifier) of the API.|
|body|body|[AddGatewayToRESTAPIRequest](schemas.md#schemaaddgatewaytorestapirequest)|false|List of gateways to associate with the API|

#### Detailed descriptions

**graphqlApiId**: **GraphQL API ID** consisting of the **handle** (unique identifier) of the API.

> Example responses
>
> 200 Response

```json
{
  "count": 3,
  "list": [
    {
      "id": "prod-gateway-01",
      "organizationId": "acme",
      "displayName": "Production Gateway 01",
      "description": "Production gateway for handling API traffic",
      "properties": {
        "region": "us-west",
        "tier": "premium"
      },
      "endpoints": [
        "https://api.example.com:8443/api/v1",
        "wss://events.example.com:8444"
      ],
      "isCritical": true,
      "functionalityType": "regular",
      "version": "1.0",
      "isActive": true,
      "createdBy": "john.doe",
      "updatedBy": "john.doe",
      "createdAt": "2025-10-14T10:30:00Z",
      "updatedAt": "2025-10-14T10:30:00Z",
      "associatedAt": "2025-10-15T10:30:00Z",
      "isDeployed": true,
      "deployment": {
        "status": "CREATED",
        "deployedAt": "2025-10-15T11:00:00Z"
      }
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="add-gateways-for-graphql-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of all gateways associated with the API, including deployment details|[RESTAPIGatewayListResponse](schemas.md#schemarestapigatewaylistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Create API key

<a id="opIdCreateGraphQLAPIKey"></a>

`POST /graphql-apis/{graphqlApiId}/api-keys`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId}/api-keys \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Creates a new API key for the specified GraphQL API. The API key will be hashed before
storage and broadcasted to all gateways where the API is deployed. This endpoint
allows external platforms to inject API keys to hybrid gateways.

> Payload

```json
{
  "id": "production-key-01",
  "displayName": "Production API Key",
  "apiKey": "sk_example_1234567890abcdef",
  "externalRefId": "ext-ref-12345",
  "expiresAt": "2026-12-31T23:59:59Z",
  "expiresIn": {
    "duration": 30,
    "unit": "days"
  },
  "issuer": "api-platform-devportal"
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:graphql:api_key:create`, `ap:api:graphql:api_key:manage`, `ap:api:graphql:manage`, `ap:api:manage`, `ap:manage`, `ap:credential:api_key:all:manage`

</aside>

<h3 id="create-api-key-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|**GraphQL API ID** consisting of the **handle** (unique identifier) of the API.|
|body|body|[CreateAPIKeyRequest](schemas.md#schemacreateapikeyrequest)|true|API key creation request|

#### Detailed descriptions

**graphqlApiId**: **GraphQL API ID** consisting of the **handle** (unique identifier) of the API.

> Example responses
>
> 201 Response

```json
{
  "status": "success",
  "message": "API key created and broadcasted to gateways successfully",
  "keyId": "production-key-01",
  "apiKey": "sk_example_1234567890abcdef"
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

> 503 Response

```json
{
  "status": "error",
  "code": "GATEWAY_CONNECTION_UNAVAILABLE",
  "message": "No gateway connections are currently available.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="create-api-key-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|API key created successfully|[CreateAPIKeyResponse](schemas.md#schemacreateapikeyresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|Service Unavailable. No gateway connections are currently available to service this request.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## Update API key

<a id="opIdUpdateGraphQLAPIKey"></a>

`PUT /graphql-apis/{graphqlApiId}/api-keys/{apiKeyId}`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId}/api-keys/{apiKeyId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Updates an existing API key for the specified GraphQL API. The new API key value will
be hashed before storage and broadcasted to all gateways where the API is deployed.
This endpoint allows external platforms to rotate API keys on hybrid gateways.

> Payload

```json
{
  "name": "production-key-01",
  "displayName": "Production API Key (Updated)",
  "apiKey": "sk_example_new1234567890abcdef",
  "externalRefId": "ext-ref-12345",
  "expiresAt": "2027-12-31T23:59:59Z",
  "expiresIn": {
    "duration": 30,
    "unit": "days"
  },
  "issuer": "api-platform-devportal"
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:graphql:api_key:update`, `ap:api:graphql:api_key:manage`, `ap:api:graphql:manage`, `ap:api:manage`, `ap:manage`, `ap:credential:api_key:all:manage`

</aside>

<h3 id="update-api-key-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|**GraphQL API ID** consisting of the **handle** (unique identifier) of the API.|
|apiKeyId|path|string|true|The unique name/identifier of the API key|
|body|body|[UpdateAPIKeyRequest](schemas.md#schemaupdateapikeyrequest)|true|API key update request|

#### Detailed descriptions

**graphqlApiId**: **GraphQL API ID** consisting of the **handle** (unique identifier) of the API.

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "message": "API key updated and broadcasted to gateways successfully",
  "keyId": "production-key-01"
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

> 503 Response

```json
{
  "status": "error",
  "code": "GATEWAY_CONNECTION_UNAVAILABLE",
  "message": "No gateway connections are currently available.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="update-api-key-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key updated successfully|[UpdateAPIKeyResponse](schemas.md#schemaupdateapikeyresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|Service Unavailable. No gateway connections are currently available to service this request.|[Error](schemas.md#schemaerror)|

## Revoke API key

<a id="opIdRevokeGraphQLAPIKey"></a>

`DELETE /graphql-apis/{graphqlApiId}/api-keys/{apiKeyId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId}/api-keys/{apiKeyId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Revokes an API key for the specified GraphQL API. The revocation will be broadcasted
to all gateways where the API is deployed. This endpoint allows external platforms
to revoke API keys on hybrid gateways.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:graphql:api_key:delete`, `ap:api:graphql:api_key:manage`, `ap:api:graphql:manage`, `ap:api:manage`, `ap:manage`, `ap:credential:api_key:all:manage`

</aside>

<h3 id="revoke-api-key-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|**GraphQL API ID** consisting of the **handle** (unique identifier) of the API.|
|apiKeyId|path|string|true|The unique name/identifier of the API key to revoke|

#### Detailed descriptions

**graphqlApiId**: **GraphQL API ID** consisting of the **handle** (unique identifier) of the API.

> Example responses
>
> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

> 503 Response

```json
{
  "status": "error",
  "code": "GATEWAY_CONNECTION_UNAVAILABLE",
  "message": "No gateway connections are currently available.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="revoke-api-key-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|API key revoked successfully (no content)|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|Service Unavailable. No gateway connections are currently available to service this request.|[Error](schemas.md#schemaerror)|
