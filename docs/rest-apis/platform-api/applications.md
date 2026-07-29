<h1 id="wso2-api-platform-platform-api-applications">Applications</h1>

Application management operations

## Create a new application

<a id="opIdCreateApplication"></a>

`POST /applications`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/applications \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Creates a new application within the organization specified in the JWT token.

> Payload

```json
{
  "id": "my-app-handle",
  "displayName": "GenAI Demo App",
  "projectId": "default-project",
  "type": "genai",
  "description": "Sample GenAI application"
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:application:create`, `ap:application:manage`

</aside>

<h3 id="create-a-new-application-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[CreateApplicationRequest](schemas.md#schemacreateapplicationrequest)|true|none|

> Example responses

> 201 Response

```json
{
  "id": "my-app-handle",
  "displayName": "GenAI Demo App",
  "projectId": "default-project",
  "type": "genai",
  "description": "Sample GenAI application",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "createdAt": "2025-11-15T10:30:00Z",
  "updatedAt": "2025-11-15T11:30:00Z"
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
      "field": "spec.context",
      "message": "must start with /"
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
  "message": "The specified resource already exists."
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

<h3 id="create-a-new-application-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Application created successfully|[Application](schemas.md#schemaapplication)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. Specified resource already exists.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## Get applications for current user's organization

<a id="opIdListApplications"></a>

`GET /applications`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/applications?projectId=default-project \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves applications belonging to the organization specified in the JWT token.
Filters by project using the required `projectId` query parameter.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:application:read`, `ap:application:manage`

</aside>

<h3 id="get-applications-for-current-user's-organization-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|projectId|query|string|true|**Project ID** consisting of the **handle** (unique slug identifier) of the Project to filter APIs by.|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|
|sortBy|query|string|false|Field to sort the collection by. An unrecognized value falls back to the default sort (createdAt).|
|sortOrder|query|string|false|Sort direction applied to `sortBy`.|
|query|query|string|false|Case-insensitive substring filter matched against the resource id (handle).|

#### Detailed descriptions

**projectId**: **Project ID** consisting of the **handle** (unique slug identifier) of the Project to filter APIs by.

#### Enumerated Values

|Parameter|Value|
|---|---|
|sortBy|name|
|sortBy|createdAt|
|sortOrder|asc|
|sortOrder|desc|

> Example responses

> 200 Response

```json
{
  "count": 2,
  "list": [
    {
      "id": "my-app-handle",
      "displayName": "GenAI Demo App",
      "projectId": "default-project",
      "type": "genai",
      "description": "Sample GenAI application",
      "createdBy": "john.doe",
      "updatedBy": "john.doe",
      "createdAt": "2025-11-15T10:30:00Z",
      "updatedAt": "2025-11-15T11:30:00Z"
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
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

<h3 id="get-applications-for-current-user's-organization-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Applications retrieved successfully|[ApplicationListResponse](schemas.md#schemaapplicationlistresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get application by handle

<a id="opIdGetApplication"></a>

`GET /applications/{applicationId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/applications/{applicationId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves a specific application by handle. Access is validated against
the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:application:read`, `ap:application:manage`

</aside>

<h3 id="get-application-by-handle-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|applicationId|path|string|true|**Application ID** consisting of the **handle** of the application.|

#### Detailed descriptions

**applicationId**: **Application ID** consisting of the **handle** of the application.

> Example responses

> 200 Response

```json
{
  "id": "my-app-handle",
  "displayName": "GenAI Demo App",
  "projectId": "default-project",
  "type": "genai",
  "description": "Sample GenAI application",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "createdAt": "2025-11-15T10:30:00Z",
  "updatedAt": "2025-11-15T11:30:00Z"
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
      "field": "spec.context",
      "message": "must start with /"
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

<h3 id="get-application-by-handle-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Application retrieved successfully|[Application](schemas.md#schemaapplication)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Update application

<a id="opIdUpdateApplication"></a>

`PUT /applications/{applicationId}`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/applications/{applicationId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Updates an existing application by handle.

> Payload

```json
{
  "id": "my-app-handle",
  "displayName": "GenAI Demo App",
  "projectId": "default-project",
  "type": "genai",
  "description": "Sample GenAI application"
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:application:update`, `ap:application:manage`

</aside>

<h3 id="update-application-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|applicationId|path|string|true|**Application ID** consisting of the **handle** of the application.|
|body|body|[Application](schemas.md#schemaapplication)|true|none|

#### Detailed descriptions

**applicationId**: **Application ID** consisting of the **handle** of the application.

> Example responses

> 200 Response

```json
{
  "id": "my-app-handle",
  "displayName": "GenAI Demo App",
  "projectId": "default-project",
  "type": "genai",
  "description": "Sample GenAI application",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "createdAt": "2025-11-15T10:30:00Z",
  "updatedAt": "2025-11-15T11:30:00Z"
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
      "field": "spec.context",
      "message": "must start with /"
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
  "message": "The specified resource already exists."
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

<h3 id="update-application-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Application updated successfully|[Application](schemas.md#schemaapplication)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. Specified resource already exists.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete application

<a id="opIdDeleteApplication"></a>

`DELETE /applications/{applicationId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/applications/{applicationId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Deletes an existing application by handle.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:application:delete`, `ap:application:manage`

</aside>

<h3 id="delete-application-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|applicationId|path|string|true|**Application ID** consisting of the **handle** of the application.|

#### Detailed descriptions

**applicationId**: **Application ID** consisting of the **handle** of the application.

> Example responses

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "spec.context",
      "message": "must start with /"
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

<h3 id="delete-application-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Application deleted successfully|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## List application API key mappings

<a id="opIdListApplicationAPIKeys"></a>

`GET /applications/{applicationId}/api-keys`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/applications/{applicationId}/api-keys \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Lists all API keys mapped to the specified application.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:application:api_key:read`, `ap:application:api_key:manage`, `ap:application:manage`

</aside>

<h3 id="list-application-api-key-mappings-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|applicationId|path|string|true|**Application ID** consisting of the **handle** of the application.|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

#### Detailed descriptions

**applicationId**: **Application ID** consisting of the **handle** of the application.

> Example responses

> 200 Response

```json
{
  "count": 2,
  "list": [
    {
      "keyId": "client-key-1",
      "associatedEntity": {
        "id": "pizza-api",
        "kind": "RestApi"
      },
      "status": "ACTIVE",
      "userId": "john.doe",
      "createdAt": "2025-11-15T10:30:00Z",
      "updatedAt": "2025-11-15T11:30:00Z",
      "expiresAt": "2026-11-15T10:30:00Z"
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
      "field": "spec.context",
      "message": "must start with /"
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

<h3 id="list-application-api-key-mappings-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Mapped API keys retrieved successfully|[MappedAPIKeyListResponse](schemas.md#schemamappedapikeylistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Add application API key mappings

<a id="opIdAddApplicationAPIKeys"></a>

`POST /applications/{applicationId}/api-keys`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/applications/{applicationId}/api-keys \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Adds API key mappings to the specified application.

> Payload

```json
{
  "apiKeys": [
    {
      "keyId": "client-key-1",
      "associatedEntity": {
        "id": "pizza-api"
      }
    }
  ]
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:application:api_key:create`, `ap:application:api_key:manage`, `ap:application:manage`

</aside>

<h3 id="add-application-api-key-mappings-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|applicationId|path|string|true|**Application ID** consisting of the **handle** of the application.|
|body|body|[AddApplicationAPIKeysRequest](schemas.md#schemaaddapplicationapikeysrequest)|true|none|

#### Detailed descriptions

**applicationId**: **Application ID** consisting of the **handle** of the application.

> Example responses

> 200 Response

```json
{
  "count": 2,
  "list": [
    {
      "keyId": "client-key-1",
      "associatedEntity": {
        "id": "pizza-api",
        "kind": "RestApi"
      },
      "status": "ACTIVE",
      "userId": "john.doe",
      "createdAt": "2025-11-15T10:30:00Z",
      "updatedAt": "2025-11-15T11:30:00Z",
      "expiresAt": "2026-11-15T10:30:00Z"
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
      "field": "spec.context",
      "message": "must start with /"
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

<h3 id="add-application-api-key-mappings-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key mappings added successfully|[MappedAPIKeyListResponse](schemas.md#schemamappedapikeylistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Remove application API key mapping

<a id="opIdRemoveApplicationAPIKey"></a>

`DELETE /applications/{applicationId}/api-keys/{apiKeyId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/applications/{applicationId}/api-keys/{apiKeyId}?entityID=my-rest-api-handle \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Removes a mapped API key from the specified application.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:application:api_key:delete`, `ap:application:api_key:manage`, `ap:application:manage`

</aside>

<h3 id="remove-application-api-key-mapping-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|applicationId|path|string|true|**Application ID** consisting of the **handle** of the application.|
|apiKeyId|path|string|true|**API Key ID** consisting of the **name** of the API key.|
|entityID|query|string|true|**Entity ID** of the artifact associated with the API key mapping.|

#### Detailed descriptions

**applicationId**: **Application ID** consisting of the **handle** of the application.

**apiKeyId**: **API Key ID** consisting of the **name** of the API key.

**entityID**: **Entity ID** of the artifact associated with the API key mapping.

> Example responses

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "spec.context",
      "message": "must start with /"
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

<h3 id="remove-application-api-key-mapping-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|API key mapping removed successfully|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## List application associations

<a id="opIdListApplicationAssociations"></a>

`GET /applications/{applicationId}/associations`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/applications/{applicationId}/associations \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Lists association targets mapped to the specified application.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:application:association:read`, `ap:application:association:manage`, `ap:application:manage`

</aside>

<h3 id="list-application-associations-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|applicationId|path|string|true|**Application ID** consisting of the **handle** of the application.|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

#### Detailed descriptions

**applicationId**: **Application ID** consisting of the **handle** of the application.

> Example responses

> 200 Response

```json
{
  "count": 2,
  "list": [
    {
      "id": "provider-handle",
      "displayName": "OpenAI Provider",
      "version": "v1.0",
      "kind": "LlmProvider",
      "createdAt": "2025-11-15T10:30:00Z",
      "updatedAt": "2025-11-15T11:30:00Z"
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
      "field": "spec.context",
      "message": "must start with /"
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

<h3 id="list-application-associations-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Application associations retrieved successfully|[ApplicationAssociationListResponse](schemas.md#schemaapplicationassociationlistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Add application associations

<a id="opIdAddApplicationAssociations"></a>

`POST /applications/{applicationId}/associations`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/applications/{applicationId}/associations \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Adds association targets to the specified application.

> Payload

```json
{
  "associations": [
    {
      "id": "provider-handle",
      "kind": "LlmProvider"
    }
  ]
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:application:association:create`, `ap:application:association:manage`, `ap:application:manage`

</aside>

<h3 id="add-application-associations-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|applicationId|path|string|true|**Application ID** consisting of the **handle** of the application.|
|body|body|[AddApplicationAssociationsRequest](schemas.md#schemaaddapplicationassociationsrequest)|true|none|

#### Detailed descriptions

**applicationId**: **Application ID** consisting of the **handle** of the application.

> Example responses

> 200 Response

```json
{
  "count": 2,
  "list": [
    {
      "id": "provider-handle",
      "displayName": "OpenAI Provider",
      "version": "v1.0",
      "kind": "LlmProvider",
      "createdAt": "2025-11-15T10:30:00Z",
      "updatedAt": "2025-11-15T11:30:00Z"
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
      "field": "spec.context",
      "message": "must start with /"
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

<h3 id="add-application-associations-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Application associations added successfully|[ApplicationAssociationListResponse](schemas.md#schemaapplicationassociationlistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Remove application association

<a id="opIdRemoveApplicationAssociation"></a>

`DELETE /applications/{applicationId}/associations/{associationId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/applications/{applicationId}/associations/{associationId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Removes an association target from the specified application.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:application:association:delete`, `ap:application:association:manage`, `ap:application:manage`

</aside>

<h3 id="remove-application-association-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|applicationId|path|string|true|**Application ID** consisting of the **handle** of the application.|
|associationId|path|string|true|**Association ID** consisting of the **handle** or **UUID** of the association target.|

#### Detailed descriptions

**applicationId**: **Application ID** consisting of the **handle** of the application.

**associationId**: **Association ID** consisting of the **handle** or **UUID** of the association target.

> Example responses

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "spec.context",
      "message": "must start with /"
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

<h3 id="remove-application-association-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Application association removed successfully|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## List application API key mappings for an association

<a id="opIdListApplicationAssociationAPIKeys"></a>

`GET /applications/{applicationId}/associations/{associationId}/api-keys`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/applications/{applicationId}/associations/{associationId}/api-keys \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Lists API keys mapped to the specified application for the given associated target.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:application:association:api_key:read`, `ap:application:association:manage`, `ap:application:manage`

</aside>

<h3 id="list-application-api-key-mappings-for-an-association-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|applicationId|path|string|true|**Application ID** consisting of the **handle** of the application.|
|associationId|path|string|true|**Association ID** consisting of the **handle** or **UUID** of the association target.|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

#### Detailed descriptions

**applicationId**: **Application ID** consisting of the **handle** of the application.

**associationId**: **Association ID** consisting of the **handle** or **UUID** of the association target.

> Example responses

> 200 Response

```json
{
  "count": 2,
  "list": [
    {
      "keyId": "client-key-1",
      "associatedEntity": {
        "id": "pizza-api",
        "kind": "RestApi"
      },
      "status": "ACTIVE",
      "userId": "john.doe",
      "createdAt": "2025-11-15T10:30:00Z",
      "updatedAt": "2025-11-15T11:30:00Z",
      "expiresAt": "2026-11-15T10:30:00Z"
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
      "field": "spec.context",
      "message": "must start with /"
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

<h3 id="list-application-api-key-mappings-for-an-association-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Mapped API keys retrieved successfully|[MappedAPIKeyListResponse](schemas.md#schemamappedapikeylistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
