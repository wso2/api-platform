<h1 id="wso2-api-developer-portal-core-devportal-routes-api-keys">API Keys</h1>

## Generate an API key

<a id="opIdgenerateApiKey"></a>

`POST /organizations/{orgId}/api-keys/generate`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/organizations/{orgId}/api-keys/generate \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Generates an API key stored in the Developer Portal (devportal is source of truth). The plaintext secret is returned once in the response and never persisted. A `apikey.generated` webhook event is published so gateways can register the key. Key names must match `^[a-z0-9][a-z0-9_-]{0,127}$`, and `expiresAt` must include a timezone when sent as an ISO-8601 string.

> Payload

```json
{
  "apiId": "api-7f4c2a6b",
  "name": "weather_prod_key",
  "expiresAt": "2026-12-31T23:59:59Z"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="generate-an-api-key-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[ApiKeyRequest](schemas.md#schemaapikeyrequest)|true|API key payload. `apiId` is the Developer Portal API ID. `name` must be lowercase and may contain numbers, underscores, and hyphens. `expiresAt` can be an ISO-8601 datetime with timezone, epoch seconds, or epoch milliseconds.|
|orgId|path|string|true|none|

> Example responses

> 201 Response

```json
{
  "keyId": "key-12345",
  "name": "weather_prod_key",
  "apiId": "api-7f4c2a6b",
  "key": "ak_dGhpcyBpcyBub3QgYSByZWFsIGtleQ",
  "expiresAt": "2026-12-31T23:59:59Z",
  "status": "ACTIVE"
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

> 403 Response

```json
{
  "code": "403",
  "message": "Forbidden",
  "description": "Write operations are disabled in read-only mode"
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

<h3 id="generate-an-api-key-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Generated or regenerated API key. The plaintext `key` is returned exactly once.|[ApiKeyResponse](schemas.md#schemaapikeyresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden for the current runtime mode or caller permissions.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="generate-an-api-key-responseschema">Response Schema</h3>

## List API keys

<a id="opIdlistApiKeys"></a>

`GET /organizations/{orgId}/api-keys`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/api-keys?apiId=api-7f4c2a6b \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Lists API keys for an API. The `apiId` query parameter is required and is resolved from the Developer Portal API ID to the control-plane API reference.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-api-keys-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiId|query|string|true|Developer Portal API ID used to resolve the control-plane API reference.|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
[
  {
    "keyId": "key-12345",
    "name": "weather_prod_key",
    "apiId": "api-7f4c2a6b",
    "status": "ACTIVE",
    "expiresAt": "2026-12-31T23:59:59Z",
    "createdAt": "2019-08-24T14:15:22Z",
    "revokedAt": "2019-08-24T14:15:22Z"
  }
]
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

<h3 id="list-api-keys-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of API key metadata records.|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-api-keys-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[[ApiKeyMetadataResponse](schemas.md#schemaapikeymetadataresponse)]|false|none|[API key metadata returned by list operations. Secret material is omitted.]|
|» keyId|string|false|none|Developer Portal key identifier.|
|» name|string|false|none|none|
|» apiId|string|false|none|Developer Portal API ID the key belongs to.|
|» status|string|false|none|none|
|» expiresAt|string(date-time)¦null|false|none|none|
|» createdAt|string(date-time)|false|none|none|
|» revokedAt|string(date-time)¦null|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|REVOKED|

## Regenerate an API key

<a id="opIdregenerateApiKey"></a>

`POST /organizations/{orgId}/api-keys/{apiKeyId}/regenerate`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/organizations/{orgId}/api-keys/{apiKeyId}/regenerate \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Regenerates the secret for an existing API key identified by `apiKeyId`. The old secret is invalidated at connected gateways via an `apikey.regenerated` webhook event. The new plaintext secret is returned once and never persisted.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="regenerate-an-api-key-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|apiKeyId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "keyId": "key-12345",
  "name": "weather_prod_key",
  "apiId": "api-7f4c2a6b",
  "key": "ak_dGhpcyBpcyBub3QgYSByZWFsIGtleQ",
  "expiresAt": "2026-12-31T23:59:59Z",
  "status": "ACTIVE"
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

> 403 Response

```json
{
  "code": "403",
  "message": "Forbidden",
  "description": "Write operations are disabled in read-only mode"
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

<h3 id="regenerate-an-api-key-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Generated or regenerated API key. The plaintext `key` is returned exactly once.|[ApiKeyResponse](schemas.md#schemaapikeyresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden for the current runtime mode or caller permissions.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="regenerate-an-api-key-responseschema">Response Schema</h3>

## Revoke an API key

<a id="opIdrevokeApiKey"></a>

`POST /organizations/{orgId}/api-keys/{apiKeyId}/revoke`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/organizations/{orgId}/api-keys/{apiKeyId}/revoke \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Revokes an existing API key. An `apikey.revoked` webhook event is published so connected gateways can immediately reject requests carrying the key.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="revoke-an-api-key-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|apiKeyId|path|string|true|none|

> Example responses

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

> 403 Response

```json
{
  "code": "403",
  "message": "Forbidden",
  "description": "Write operations are disabled in read-only mode"
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

<h3 id="revoke-an-api-key-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|API key revoked successfully.|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden for the current runtime mode or caller permissions.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="revoke-an-api-key-responseschema">Response Schema</h3>
