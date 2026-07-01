<h1 id="wso2-api-developer-portal-core-devportal-routes-api-keys">API Keys</h1>

## Generate an API key

<a id="opIdgenerateApiKey"></a>

`POST /devportal/v1/apis/{apiId}/api-keys/generate`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/devportal/v1/apis/{apiId}/api-keys/generate \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Generates an API key stored in the Developer Portal (devportal is source of truth). The plaintext secret is returned once in the response and never persisted. A `apikey.generated` webhook event is published to the organization's configured webhook subscribers so they can register the key (e.g. with a gateway). Key names must match `^[a-z0-9][a-z0-9_-]{0,127}$`, and `expiresAt` must include a timezone when sent as an ISO-8601 string.

> Payload

```json
{
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
|body|body|[ApiKeyRequest](schemas.md#schemaapikeyrequest)|true|API key payload. `name` must be lowercase and may contain numbers, underscores, and hyphens. `expiresAt` can be an ISO-8601 datetime with timezone, epoch seconds, or epoch milliseconds. The API is identified by the `{apiId}` path parameter.|
|apiId|path|string|true|none|

> Example responses

> 201 Response

```json
{
  "keyId": "key-12345",
  "name": "weather_prod_key",
  "key": "ak_dGhpcyBpcyBub3QgYSByZWFsIGtleQ",
  "expiresAt": "2026-12-31T23:59:59Z",
  "status": "ACTIVE"
}
```

> 400 Response

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "apiId is required"
}
```

> 403 Response

```json
{
  "code": "403",
  "message": "Read-only mode"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Not Found",
  "description": "Subscription not found"
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

<h3 id="generate-an-api-key-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Generated API key. The plaintext `key` is returned exactly once.|[ApiKeyResponse](schemas.md#schemaapikeyresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden for the current runtime mode (e.g. read-only mode).|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the generated API key resource.|

## List API keys

<a id="opIdlistApiKeys"></a>

`GET /devportal/v1/apis/{apiId}/api-keys`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/devportal/v1/apis/{apiId}/api-keys \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Lists API keys for the given API.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-api-keys-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|appId|query|string|false|Optional application ID used to filter API keys associated with that application.|
|limit|query|integer|false|Maximum number of records to return.|
|offset|query|integer|false|Number of records to skip before returning results.|
|apiId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "list": [
    {
      "keyId": "key-12345",
      "name": "weather_prod_key",
      "apiId": "api-7f4c2a6b",
      "appId": "app-12345",
      "appDisplayName": "My Mobile App",
      "status": "ACTIVE",
      "expiresAt": "2026-12-31T23:59:59Z",
      "createdAt": "2019-08-24T14:15:22Z",
      "revokedAt": "2019-08-24T14:15:22Z"
    }
  ],
  "pagination": {
    "total": 42,
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

<h3 id="list-api-keys-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of API key metadata records.|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-api-keys-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» list|[[ApiKeyMetadataResponse](schemas.md#schemaapikeymetadataresponse)]|false|none|[API key metadata returned by list operations. Secret material is omitted.]|
|»» keyId|string|false|none|Developer Portal key identifier.|
|»» name|string|false|none|none|
|»» apiId|string|false|none|Developer Portal API ID the key belongs to.|
|»» appId|string¦null|false|none|ID of the application this key is associated with, if any. Analytics attribution only.|
|»» appDisplayName|string¦null|false|none|Display name of the associated application, if any.|
|»» status|string|false|none|none|
|»» expiresAt|string(date-time)¦null|false|none|none|
|»» createdAt|string(date-time)|false|none|none|
|»» revokedAt|string(date-time)¦null|false|none|none|
|» pagination|[Pagination](schemas.md#schemapagination)|false|none|Standard pagination metadata returned with collection responses.|
|»» total|integer|true|none|Total number of records matching the query.|
|»» limit|integer|true|none|Maximum number of records returned in this response.|
|»» offset|integer|true|none|Number of records skipped before this page.|

#### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|REVOKED|

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Regenerate an API key

<a id="opIdregenerateApiKey"></a>

`POST /devportal/v1/apis/{apiId}/api-keys/regenerate`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/devportal/v1/apis/{apiId}/api-keys/regenerate \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Regenerates the secret for an existing API key identified by `keyId` in the request body. An `apikey.regenerated` webhook event is published to the organization's configured webhook subscribers so they can invalidate the old secret (e.g. at a gateway). The new plaintext secret is returned once and never persisted.

> Payload

```json
{
  "keyId": "key-12345",
  "expiresAt": "2027-01-01T00:00:00Z"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="regenerate-an-api-key-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|Identifies the API key to regenerate by its `keyId`. `expiresAt` is optional and, if provided, updates the key's expiry; the key's `name` cannot be changed by this operation.|
|» keyId|body|string|true|Developer Portal key ID returned by generate or list.|
|» expiresAt|body|string|false|New expiry for the key. Can be an ISO-8601 datetime with timezone, epoch seconds, or epoch milliseconds. Omit to leave the current expiry unchanged.|
|apiId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "keyId": "key-12345",
  "name": "weather_prod_key",
  "key": "ak_dGhpcyBpcyBub3QgYSByZWFsIGtleQ",
  "expiresAt": "2026-12-31T23:59:59Z",
  "status": "ACTIVE"
}
```

> 403 Response

```json
{
  "code": "403",
  "message": "Read-only mode"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Not Found",
  "description": "Subscription not found"
}
```

> 409 Response

```json
{
  "code": "409",
  "message": "Cannot regenerate a revoked key"
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

<h3 id="regenerate-an-api-key-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Generated or regenerated API key. The plaintext `key` is returned exactly once.|[ApiKeyResponse](schemas.md#schemaapikeyresponse)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden for the current runtime mode (e.g. read-only mode).|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The key has already been revoked and cannot be regenerated.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Revoke an API key

<a id="opIdrevokeApiKey"></a>

`POST /devportal/v1/apis/{apiId}/api-keys/revoke`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/devportal/v1/apis/{apiId}/api-keys/revoke \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Revokes an existing API key identified by `keyId` in the request body. An `apikey.revoked` webhook event is published to the organization's configured webhook subscribers so they can immediately reject requests carrying the key (e.g. at a gateway).

> Payload

```json
{
  "keyId": "key-12345"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="revoke-an-api-key-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|Identifies the API key to revoke by its `keyId`.|
|» keyId|body|string|true|Developer Portal key ID returned by generate or list.|
|apiId|path|string|true|none|

> Example responses

> 403 Response

```json
{
  "code": "403",
  "message": "Read-only mode"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Not Found",
  "description": "Subscription not found"
}
```

> 409 Response

```json
{
  "code": "409",
  "message": "Key already revoked or not found"
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

<h3 id="revoke-an-api-key-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|API key revoked successfully.|None|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden for the current runtime mode (e.g. read-only mode).|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The key has already been revoked.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Associate an API key with an application

<a id="opIdassociateApiKeyApplication"></a>

`POST /devportal/v1/apis/{apiId}/api-keys/associate`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/devportal/v1/apis/{apiId}/api-keys/associate \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Associates (or re-associates) an existing API key with an application, for analytics attribution only — it has no effect on the key's validity or authorization. An `apikey.application_updated` webhook event is published once for this key, with a payload of `{ key_id, application }`.

> Payload

```json
{
  "keyId": "key-12345",
  "appId": "app-12345"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="associate-an-api-key-with-an-application-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|Identifies the API key and the application to associate it with.|
|» keyId|body|string|true|Developer Portal key ID returned by generate or list.|
|» appId|body|string|true|Developer Portal application ID to associate the key with.|
|apiId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "keyId": "key-12345",
  "application": {
    "id": "app-12345",
    "displayName": "My Mobile App"
  }
}
```

> 400 Response

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "apiId is required"
}
```

> 403 Response

```json
{
  "code": "403",
  "message": "Read-only mode"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Not Found",
  "description": "Subscription not found"
}
```

> 409 Response

```json
{
  "code": "409",
  "message": "Cannot associate a revoked key"
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

<h3 id="associate-an-api-key-with-an-application-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Association updated.|[ApiKeyApplicationResponse](schemas.md#schemaapikeyapplicationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden for the current runtime mode (e.g. read-only mode).|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The key has already been revoked and cannot be associated with an application.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Remove an API key's application association

<a id="opIdremoveApiKeyApplication"></a>

`POST /devportal/v1/apis/{apiId}/api-keys/dissociate`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/devportal/v1/apis/{apiId}/api-keys/dissociate \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Removes the application association from an API key identified by `keyId` in the request body, if any. An `apikey.application_updated` webhook event is published once for this key, with `application` set to `null`.

> Payload

```json
{
  "keyId": "key-12345"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="remove-an-api-key's-application-association-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|Identifies the API key to remove the application association from.|
|» keyId|body|string|true|Developer Portal key ID returned by generate or list.|
|apiId|path|string|true|none|

> Example responses

> 403 Response

```json
{
  "code": "403",
  "message": "Read-only mode"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Not Found",
  "description": "Subscription not found"
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

<h3 id="remove-an-api-key's-application-association-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Association removed (or none existed).|None|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden for the current runtime mode (e.g. read-only mode).|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List API keys associated with an application

<a id="opIdlistApplicationApiKeys"></a>

`GET /devportal/v1/applications/{applicationId}/api-keys`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/devportal/v1/applications/{applicationId}/api-keys \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Lists all API keys (across every API) currently associated with the given application. Unlike `listApiKeys`, no `apiId` filter is required.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-api-keys-associated-with-an-application-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|limit|query|integer|false|Maximum number of records to return.|
|offset|query|integer|false|Number of records to skip before returning results.|
|applicationId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "list": [
    {
      "keyId": "key-12345",
      "name": "weather_prod_key",
      "apiId": "api-7f4c2a6b",
      "appId": "app-12345",
      "appDisplayName": "My Mobile App",
      "status": "ACTIVE",
      "expiresAt": "2026-12-31T23:59:59Z",
      "createdAt": "2019-08-24T14:15:22Z",
      "revokedAt": "2019-08-24T14:15:22Z"
    }
  ],
  "pagination": {
    "total": 42,
    "limit": 20,
    "offset": 0
  }
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Not Found",
  "description": "Subscription not found"
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

<h3 id="list-api-keys-associated-with-an-application-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of API key metadata records.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-api-keys-associated-with-an-application-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» list|[[ApiKeyMetadataResponse](schemas.md#schemaapikeymetadataresponse)]|false|none|[API key metadata returned by list operations. Secret material is omitted.]|
|»» keyId|string|false|none|Developer Portal key identifier.|
|»» name|string|false|none|none|
|»» apiId|string|false|none|Developer Portal API ID the key belongs to.|
|»» appId|string¦null|false|none|ID of the application this key is associated with, if any. Analytics attribution only.|
|»» appDisplayName|string¦null|false|none|Display name of the associated application, if any.|
|»» status|string|false|none|none|
|»» expiresAt|string(date-time)¦null|false|none|none|
|»» createdAt|string(date-time)|false|none|none|
|»» revokedAt|string(date-time)¦null|false|none|none|
|» pagination|[Pagination](schemas.md#schemapagination)|false|none|Standard pagination metadata returned with collection responses.|
|»» total|integer|true|none|Total number of records matching the query.|
|»» limit|integer|true|none|Maximum number of records returned in this response.|
|»» offset|integer|true|none|Number of records skipped before this page.|

#### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|REVOKED|
