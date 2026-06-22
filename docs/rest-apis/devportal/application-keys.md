<h1 id="wso2-api-developer-portal-core-devportal-routes-application-keys">Application Keys</h1>

## Generate OAuth keys for a Developer Portal application

<a id="opIdgenerateApplicationKeys"></a>

`POST /o/{orgId}/devportal/v1/applications/{applicationId}/generate-keys`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/o/{orgId}/devportal/v1/applications/{applicationId}/generate-keys \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Generates or maps OAuth credentials for the specified application through the selected key manager. The portal calls the Authorization Server DCR endpoint directly using the configured key manager adapter and stores the resulting key mapping.

> Payload

```json
{
  "keyManager": "Resident Key Manager",
  "keyType": "PRODUCTION",
  "grantTypesToBeSupported": [
    "client_credentials",
    "refresh_token"
  ],
  "callbackUrl": "https://app.example.com/callback",
  "additionalProperties": {
    "application_access_token_expiry_time": "3600",
    "user_access_token_expiry_time": "3600"
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="generate-oauth-keys-for-a-developer-portal-application-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[AppKeyMappingRequest](schemas.md#schemaappkeymappingrequest)|true|OAuth key generation payload. The application is identified by the `applicationId` path parameter.|
|orgId|path|string|true|none|
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
    "status": "error",
    "code": "COMMON_VALIDATION_ERROR",
    "message": "Input validation failed.",
    "errors": [
      {
        "field": "orgName",
        "message": "orgName is required."
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
  "code": "ORG_ALREADY_EXISTS",
  "message": "Organization already exists."
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

<h3 id="generate-oauth-keys-for-a-developer-portal-application-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Application OAuth key response.|[ApplicationOAuthKeyResponse](schemas.md#schemaapplicationoauthkeyresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="generate-oauth-keys-for-a-developer-portal-application-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Generate an OAuth access token

<a id="opIdgenerateOAuthKeys"></a>

`POST /o/{orgId}/devportal/v1/applications/{applicationId}/oauth-keys/{keyMappingId}/generate-token`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/o/{orgId}/devportal/v1/applications/{applicationId}/oauth-keys/{keyMappingId}/generate-token \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Generates an access token for an existing application OAuth key mapping. The portal calls the Authorization Server token endpoint directly using the client credentials supplied in `consumerSecret`.

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
|orgId|path|string|true|none|
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
    "status": "error",
    "code": "COMMON_VALIDATION_ERROR",
    "message": "Input validation failed.",
    "errors": [
      {
        "field": "orgName",
        "message": "orgName is required."
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

## Revoke OAuth keys

<a id="opIdrevokeOAuthKeys"></a>

`DELETE /o/{orgId}/devportal/v1/applications/{applicationId}/oauth-keys/{keyMappingId}`

> Code samples

```shell

curl -X DELETE https://devportal.api-platform.io/o/{orgId}/devportal/v1/applications/{applicationId}/oauth-keys/{keyMappingId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Revokes an application OAuth key mapping and removes the registered OAuth client from the key manager.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="revoke-oauth-keys-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
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

<h3 id="revoke-oauth-keys-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Message or generic response payload.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="revoke-oauth-keys-responseschema">Response Schema</h3>

## Update OAuth keys

<a id="opIdupdateOAuthKeys"></a>

`PUT /o/{orgId}/devportal/v1/applications/{applicationId}/oauth-keys/{keyMappingId}`

> Code samples

```shell

curl -X PUT https://devportal.api-platform.io/o/{orgId}/devportal/v1/applications/{applicationId}/oauth-keys/{keyMappingId} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates an application OAuth key mapping via the configured key manager.

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
|body|body|[OAuthKeyUpdateRequest](schemas.md#schemaoauthkeyupdaterequest)|false|OAuth key update payload forwarded to the configured key manager.|
|orgId|path|string|true|none|
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
    "status": "error",
    "code": "COMMON_VALIDATION_ERROR",
    "message": "Input validation failed.",
    "errors": [
      {
        "field": "orgName",
        "message": "orgName is required."
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

<h3 id="update-oauth-keys-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Application OAuth key response.|[ApplicationOAuthKeyResponse](schemas.md#schemaapplicationoauthkeyresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-oauth-keys-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Clean up OAuth key artifacts

<a id="opIdcleanUpOAuthKeys"></a>

`POST /o/{orgId}/devportal/v1/applications/{applicationId}/oauth-keys/{keyMappingId}/clean-up`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/o/{orgId}/devportal/v1/applications/{applicationId}/oauth-keys/{keyMappingId}/clean-up \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Removes an OAuth key mapping and its associated OAuth client from the key manager. Used to clean up partially-created key artifacts.

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
|body|body|[OAuthKeyCleanUpRequest](schemas.md#schemaoauthkeycleanuprequest)|false|OAuth cleanup payload forwarded to the configured key manager.|
|orgId|path|string|true|none|
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
    "status": "error",
    "code": "COMMON_VALIDATION_ERROR",
    "message": "Input validation failed.",
    "errors": [
      {
        "field": "orgName",
        "message": "orgName is required."
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

<h3 id="clean-up-oauth-key-artifacts-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Message or generic response payload.|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="clean-up-oauth-key-artifacts-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|
