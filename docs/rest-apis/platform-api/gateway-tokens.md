<h1 id="wso2-api-platform-platform-api-gateway-tokens">Gateway Tokens</h1>

Gateway token rotation and revocation operations

## List active gateway tokens

<a id="opIdlistGatewayTokens"></a>

`GET /gateways/{gatewayId}/tokens`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/gateways/{gatewayId}/tokens \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Returns all active tokens for the specified gateway. Token hashes and salts are never exposed.
Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:gateway:token:read`, `ap:gateway:token:manage`, `ap:gateway:manage`

</aside>

<h3 id="list-active-gateway-tokens-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|gatewayId|path|string|true|**Gateway ID** (handle — unique slug identifier) of the Gateway.|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

#### Detailed descriptions

**gatewayId**: **Gateway ID** (handle — unique slug identifier) of the Gateway.

> Example responses
>
> 200 Response

```json
{
  "count": 0,
  "list": [
    {
      "id": "abc12345-f678-90de-f123-456789abcdef",
      "status": "active",
      "createdAt": "2025-10-14T10:30:00Z",
      "revokedAt": null
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

<h3 id="list-active-gateway-tokens-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of active tokens|[GatewayTokenListResponse](schemas.md#schemagatewaytokenlistresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Rotate gateway token

<a id="opIdrotateGatewayToken"></a>

`POST /gateways/{gatewayId}/tokens`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/gateways/{gatewayId}/tokens \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Generates a new authentication token for the gateway. The existing token remains active 
to enable zero-downtime rotation. Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:gateway:token:create`, `ap:gateway:token:manage`, `ap:gateway:manage`

</aside>

<h3 id="rotate-gateway-token-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|gatewayId|path|string|true|**Gateway ID** (handle — unique slug identifier) of the Gateway.|

#### Detailed descriptions

**gatewayId**: **Gateway ID** (handle — unique slug identifier) of the Gateway.

> Example responses
>
> 201 Response

```json
{
  "id": "def45678-g901-23hi-j456-789012klmnop",
  "token": "REDACTED_TOKEN",
  "createdAt": "2025-10-15T14:20:00Z",
  "message": "New token generated successfully. Old token remains active until revoked."
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

<h3 id="rotate-gateway-token-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|New token generated successfully|[TokenRotationResponse](schemas.md#schematokenrotationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## Revoke gateway token

<a id="opIdrevokeGatewayToken"></a>

`DELETE /gateways/{gatewayId}/tokens/{tokenId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/gateways/{gatewayId}/tokens/{tokenId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Revokes a specific gateway token. Operation is idempotent - revoking an already-revoked 
token succeeds. Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:gateway:token:delete`, `ap:gateway:token:manage`, `ap:gateway:manage`

</aside>

<h3 id="revoke-gateway-token-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|gatewayId|path|string|true|**Gateway ID** (handle — unique slug identifier) of the Gateway.|
|tokenId|path|string(uuid)|true|**Token ID** consisting of the **UUID** of the Token.|

#### Detailed descriptions

**gatewayId**: **Gateway ID** (handle — unique slug identifier) of the Gateway.

**tokenId**: **Token ID** consisting of the **UUID** of the Token.

> Example responses
>
> 200 Response

```json
{
  "message": "Token revoked successfully"
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

<h3 id="revoke-gateway-token-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Token revoked successfully|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

<h3 id="revoke-gateway-token-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» message|string|false|none|none|
