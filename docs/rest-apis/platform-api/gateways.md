<h1 id="wso2-api-platform-platform-api-gateways">Gateways</h1>

Gateway registration and management operations

## Register a new gateway

<a id="opIdCreateGateway"></a>

`POST /gateways`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/gateways \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Creates a new gateway within the organization specified in the JWT token.
Organization ID is automatically extracted from the token and does not need to be provided.

> Payload

```json
{
  "id": "prod-gateway-01",
  "displayName": "Production Gateway 01",
  "description": "Production gateway for handling API traffic",
  "endpoints": [
    "https://api.example.com:8443/api/v1",
    "wss://events.example.com:8444"
  ],
  "isCritical": true,
  "functionalityType": "regular",
  "properties": {
    "region": "us-west",
    "tier": "premium"
  },
  "version": "1.0"
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:gateway:create`, `ap:gateway:manage`

</aside>

<h3 id="register-a-new-gateway-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[CreateGatewayRequest](schemas.md#schemacreategatewayrequest)|true|Gateway registration details|

> Example responses

> 201 Response

```json
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
  "updatedAt": "2025-10-14T10:30:00Z"
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

<h3 id="register-a-new-gateway-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Gateway registered successfully|[GatewayResponse](schemas.md#schemagatewayresponse)|
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

## List all gateways

<a id="opIdListGateways"></a>

`GET /gateways`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/gateways \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves a list of all registered gateways for the organization specified in the JWT token.
Organization ID is automatically extracted from the token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:gateway:read`, `ap:gateway:manage`

</aside>

<h3 id="list-all-gateways-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|
|sortBy|query|string|false|Field to sort the collection by. An unrecognized value falls back to the default sort (createdAt).|
|sortOrder|query|string|false|Sort direction applied to `sortBy`.|
|query|query|string|false|Case-insensitive substring filter matched against the resource id (handle).|

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
      "updatedAt": "2025-10-14T10:30:00Z"
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

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="list-all-gateways-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Gateways retrieved successfully|[GatewayListResponse](schemas.md#schemagatewaylistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get gateway by UUID

<a id="opIdGetGateway"></a>

`GET /gateways/{gatewayId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/gateways/{gatewayId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves a specific gateway by its UUID. Access is validated against the organization 
in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:gateway:read`, `ap:gateway:manage`

</aside>

<h3 id="get-gateway-by-uuid-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|gatewayId|path|string|true|**Gateway ID** (handle — unique slug identifier) of the Gateway.|

#### Detailed descriptions

**gatewayId**: **Gateway ID** (handle — unique slug identifier) of the Gateway.

> Example responses

> 200 Response

```json
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
  "updatedAt": "2025-10-14T10:30:00Z"
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

<h3 id="get-gateway-by-uuid-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Gateway retrieved successfully|[GatewayResponse](schemas.md#schemagatewayresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Update gateway

<a id="opIdUpdateGateway"></a>

`PUT /gateways/{gatewayId}`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/gateways/{gatewayId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Updates an existing gateway's mutable fields (description).
Access is validated against the organization in the JWT token.

> Payload

```json
{
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
  "createdAt": "2025-10-14T10:30:00Z",
  "updatedAt": "2025-10-14T10:30:00Z"
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:gateway:update`, `ap:gateway:manage`

</aside>

<h3 id="update-gateway-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|gatewayId|path|string|true|**Gateway ID** (handle — unique slug identifier) of the Gateway.|
|body|body|[GatewayResponse](schemas.md#schemagatewayresponse)|true|Gateway object that needs to be updated|

#### Detailed descriptions

**gatewayId**: **Gateway ID** (handle — unique slug identifier) of the Gateway.

> Example responses

> 200 Response

```json
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
  "updatedAt": "2025-10-14T10:30:00Z"
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

<h3 id="update-gateway-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Gateway updated successfully|[GatewayResponse](schemas.md#schemagatewayresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete gateway

<a id="opIdDeleteGateway"></a>

`DELETE /gateways/{gatewayId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/gateways/{gatewayId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Permanently deletes a gateway and all associated tokens (CASCADE).
Deletion is blocked if the gateway has active API deployments or WebSocket connections.
Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:gateway:delete`, `ap:gateway:manage`

</aside>

<h3 id="delete-gateway-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|gatewayId|path|string|true|**Gateway ID** (handle — unique slug identifier) of the Gateway.|

#### Detailed descriptions

**gatewayId**: **Gateway ID** (handle — unique slug identifier) of the Gateway.

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

<h3 id="delete-gateway-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Gateway deleted successfully|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. Specified resource already exists.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get gateway policy manifest

<a id="opIdGetGatewayManifest"></a>

`GET /gateways/{gatewayId}/manifest`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/gateways/{gatewayId}/manifest \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Returns the policy manifest for the specified gateway. The manifest is populated by the gateway
controller when it connects to the platform API, and contains all installed policies. Custom
policies additionally include their full policy definition schema.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:gateway:manifest:read`, `ap:gateway:manage`

</aside>

<h3 id="get-gateway-policy-manifest-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|gatewayId|path|string|true|**Gateway ID** (handle — unique slug identifier) of the Gateway.|

#### Detailed descriptions

**gatewayId**: **Gateway ID** (handle — unique slug identifier) of the Gateway.

> Example responses

> 200 Response

```json
{
  "policies": [
    {
      "name": "set-wso2-headers",
      "version": "v0.8.0",
      "description": "Sets WSO2-specific headers in the request and response.",
      "isCustomPolicy": true,
      "policyDefinition": {}
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

<h3 id="get-gateway-policy-manifest-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Gateway policy manifest|[ManifestSyncResponse](schemas.md#schemamanifestsyncresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
