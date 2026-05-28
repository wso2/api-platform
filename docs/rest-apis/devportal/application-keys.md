<h1 id="wso2-api-developer-portal-core-devportal-routes-application-keys">Application Keys</h1>

## Generate OAuth keys for a control-plane application

<a id="opIdgenerateApplicationKeys"></a>

`POST /applications/{applicationId}/generate-keys`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/applications/{applicationId}/generate-keys \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Proxies key generation to the control-plane application key endpoint. Use this when the caller already has the control-plane application ID and wants to create OAuth credentials for a key manager.

> Payload

```json
{
  "keyType": "PRODUCTION",
  "keyManager": "Resident Key Manager",
  "grantTypesToBeSupported": [
    "client_credentials",
    "refresh_token"
  ],
  "callbackUrl": "https://app.example.com/callback",
  "additionalProperties": {}
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="generate-oauth-keys-for-a-control-plane-application-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[ApplicationKeysGenerateRequest](schemas.md#schemaapplicationkeysgeneraterequest)|false|OAuth key generation payload passed through to the configured control-plane application key endpoint.|
|applicationId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "keyMappingId": "km-12345",
  "keyManager": "Resident Key Manager",
  "keyType": "PRODUCTION",
  "consumerKey": "consumer-key-123",
  "consumerSecret": "consumer-secret-abc",
  "supportedGrantTypes": [
    "client_credentials",
    "refresh_token"
  ],
  "callbackUrl": "https://app.example.com/callback"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
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
  "code": "404",
  "message": "Resource Not Found",
  "description": "Organization not found"
}
```

> 409 Response

```json
{
  "code": "409",
  "message": "Conflict",
  "description": "Organization already exists"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="generate-oauth-keys-for-a-control-plane-application-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Application OAuth key response returned by the control plane.|[ApplicationOAuthKeyResponse](schemas.md#schemaapplicationoauthkeyresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="generate-oauth-keys-for-a-control-plane-application-responseschema">Response Schema</h3>

## Generate an OAuth access token

<a id="opIdgenerateOAuthKeys"></a>

`POST /applications/{applicationId}/oauth-keys/{keyMappingId}/generate-token`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/applications/{applicationId}/oauth-keys/{keyMappingId}/generate-token \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Generates an access token for an existing application OAuth key mapping. In control-plane mode the request is proxied to the control plane. In decoupled mode the portal calls the Authorization Server token endpoint directly using the client credentials supplied in `consumerSecret`.

> Payload

```json
{
  "consumerSecret": "my-consumer-secret",
  "scopes": [
    "weather.read"
  ],
  "validityPeriod": 3600
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="generate-an-oauth-access-token-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[OAuthGenerateTokenRequest](schemas.md#schemaoauthgeneratetokenrequest)|false|Passed through to the configured control-plane OAuth token generation endpoint.|
|applicationId|path|string|true|none|
|keyMappingId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.example",
  "token_type": "Bearer",
  "expires_in": 3600,
  "scope": "weather.read"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
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
  "code": "404",
  "message": "Resource Not Found",
  "description": "Organization not found"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="generate-an-oauth-access-token-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|OAuth access token response returned by the control plane.|[OAuthTokenResponse](schemas.md#schemaoauthtokenresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="generate-an-oauth-access-token-responseschema">Response Schema</h3>

## Revoke OAuth keys

<a id="opIdrevokeOAuthKeys"></a>

`DELETE /applications/{applicationId}/oauth-keys/{keyMappingId}`

> Code samples

```shell

curl -X DELETE http://localhost:3000/devportal/applications/{applicationId}/oauth-keys/{keyMappingId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Revokes an application OAuth key mapping in the control plane.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="revoke-oauth-keys-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|applicationId|path|string|true|none|
|keyMappingId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "message": "Operation completed successfully"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Resource Not Found",
  "description": "Organization not found"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="revoke-oauth-keys-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Message or control-plane response payload.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="revoke-oauth-keys-responseschema">Response Schema</h3>

## Update OAuth keys

<a id="opIdupdateOAuthKeys"></a>

`PUT /applications/{applicationId}/oauth-keys/{keyMappingId}`

> Code samples

```shell

curl -X PUT http://localhost:3000/devportal/applications/{applicationId}/oauth-keys/{keyMappingId} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates an application OAuth key mapping in the control plane.

> Payload

```json
{
  "supportedGrantTypes": [
    "client_credentials",
    "refresh_token"
  ],
  "callbackUrl": "https://app.example.com/new-callback",
  "additionalProperties": {}
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="update-oauth-keys-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[OAuthKeyUpdateRequest](schemas.md#schemaoauthkeyupdaterequest)|false|Passed through to the configured control-plane OAuth key update endpoint.|
|applicationId|path|string|true|none|
|keyMappingId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "keyMappingId": "km-12345",
  "keyManager": "Resident Key Manager",
  "keyType": "PRODUCTION",
  "consumerKey": "consumer-key-123",
  "consumerSecret": "consumer-secret-abc",
  "supportedGrantTypes": [
    "client_credentials",
    "refresh_token"
  ],
  "callbackUrl": "https://app.example.com/callback"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
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
  "code": "404",
  "message": "Resource Not Found",
  "description": "Organization not found"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="update-oauth-keys-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Application OAuth key response returned by the control plane.|[ApplicationOAuthKeyResponse](schemas.md#schemaapplicationoauthkeyresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-oauth-keys-responseschema">Response Schema</h3>

## Clean up OAuth key artifacts

<a id="opIdcleanUpOAuthKeys"></a>

`POST /applications/{applicationId}/oauth-keys/{keyMappingId}/clean-up`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/applications/{applicationId}/oauth-keys/{keyMappingId}/clean-up \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Proxies an OAuth key cleanup request to the control plane for a specific application key mapping. This is used to remove pending or partially-created OAuth key artifacts.

> Payload

```json
{
  "keyType": "PRODUCTION",
  "keyManager": "Resident Key Manager"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="clean-up-oauth-key-artifacts-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[OAuthKeyCleanUpRequest](schemas.md#schemaoauthkeycleanuprequest)|false|Passed through to the configured control-plane OAuth cleanup endpoint.|
|applicationId|path|string|true|none|
|keyMappingId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "message": "Operation completed successfully"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
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
  "code": "404",
  "message": "Resource Not Found",
  "description": "Organization not found"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="clean-up-oauth-key-artifacts-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Message or control-plane response payload.|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="clean-up-oauth-key-artifacts-responseschema">Response Schema</h3>
