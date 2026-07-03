<h1 id="wso2-api-developer-portal-core-devportal-routes-application-keys">Application Keys</h1>

## Map an OAuth client_id to a Developer Portal application

<a id="opIdgenerateApplicationKeys"></a>

`POST /applications/{applicationId}/generate-keys`

> Code samples

```shell

curl -X POST https://localhost:3000/api/v0.9/applications/{applicationId}/generate-keys \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Maps an OAuth client_id — created directly in the selected key manager — to the specified application. The portal does not create or register OAuth clients; it only stores the client_id reference and later proxies token requests for it.

> Payload

```json
{
  "keyManager": "Resident Key Manager",
  "type": "PRODUCTION",
  "consumerKey": "consumer-key-123"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="map-an-oauth-client_id-to-a-developer-portal-application-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[AppKeyMappingRequest](schemas.md#schemaappkeymappingrequest)|true|Maps an OAuth client_id — created directly in the key manager — to this application. The application is identified by the `applicationId` path parameter.|
|applicationId|path|string|true|The application's handle (unique per org).|

> Example responses

> 200 Response

```json
{
  "keyMappingId": "km-12345",
  "keyManager": "Resident Key Manager",
  "type": "PRODUCTION",
  "consumerKey": "consumer-key-123",
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token"
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

<h3 id="map-an-oauth-client_id-to-a-developer-portal-application-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Application OAuth key mapping response.|[ApplicationOAuthKeyResponse](schemas.md#schemaapplicationoauthkeyresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="map-an-oauth-client_id-to-a-developer-portal-application-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Generate an OAuth access token

<a id="opIdgenerateOAuthKeys"></a>

`POST /applications/{applicationId}/oauth-keys/{keyMappingId}/generate-token`

> Code samples

```shell

curl -X POST https://localhost:3000/api/v0.9/applications/{applicationId}/oauth-keys/{keyMappingId}/generate-token \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Generates an access token for an existing application OAuth key mapping. The portal calls the Authorization Server token endpoint directly using the client appKeyMappings supplied in `consumerSecret`.

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
|body|body|[OAuthGenerateTokenRequest](schemas.md#schemaoauthgeneratetokenrequest)|false|OAuth token generation payload. The portal calls the Authorization Server token endpoint directly.|
|applicationId|path|string|true|The application's handle (unique per org).|
|keyMappingId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "accessToken": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.example",
  "validityTime": 3600,
  "tokenScopes": [
    "weather.read"
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

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_SERVER_ERROR",
  "message": "An unexpected error occurred."
}
```

<h3 id="generate-an-oauth-access-token-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|OAuth access token response.|[OAuthTokenResponse](schemas.md#schemaoauthtokenresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="generate-an-oauth-access-token-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Remove an OAuth client_id mapping

<a id="opIdrevokeOAuthKeys"></a>

`DELETE /applications/{applicationId}/oauth-keys/{keyMappingId}`

> Code samples

```shell

curl -X DELETE https://localhost:3000/api/v0.9/applications/{applicationId}/oauth-keys/{keyMappingId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Removes the local client_id mapping for an application. This does not affect the OAuth client in the key manager — that client is owned and managed externally.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="remove-an-oauth-client_id-mapping-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|applicationId|path|string|true|The application's handle (unique per org).|
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
  "status": "error",
  "code": "ORG_NOT_FOUND",
  "message": "Organization not found."
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

<h3 id="remove-an-oauth-client_id-mapping-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Message or generic response payload.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="remove-an-oauth-client_id-mapping-responseschema">Response Schema</h3>
