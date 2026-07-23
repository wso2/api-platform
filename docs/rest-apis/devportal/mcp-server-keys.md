<h1 id="wso2-api-developer-portal-core-devportal-routes-mcp-server-keys">MCP Server Keys</h1>

## Generate an MCP server API key

<a id="opIdgenerateMcpServerApiKey"></a>

`POST /mcp-servers/{mcpServerId}/api-keys/generate`

> Code samples

```shell

curl -X POST https://localhost:9543/api/v0.9/mcp-servers/{mcpServerId}/api-keys/generate \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Generates an API key for an MCP server. Mirrors `POST /api/v0.9/apis/{apiId}/api-keys/generate`.

> Payload

```json
{
  "id": "weather_prod_key",
  "expiresAt": "2026-12-31T23:59:59Z"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="generate-an-mcp-server-api-key-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[ApiKeyRequest](schemas.md#schemaapikeyrequest)|true|API key payload. `id` must be lowercase and may contain numbers, underscores, and hyphens. `displayName` is an optional human-readable label that defaults to `id` when omitted. `expiresAt` can be an ISO-8601 datetime with timezone, epoch seconds, or epoch milliseconds. The parent resource (API or MCP server, depending on the path) is identified by the corresponding path parameter.|
|mcpServerId|path|string|true|The MCP server's handle (unique per org).|

> Example responses

> 201 Response

```json
{
  "id": "weather_prod_key",
  "displayName": "Weather Prod Key",
  "key": "ak_dGhpcyBpcyBub3QgYSByZWFsIGtleQ",
  "expiresAt": "2026-12-31T23:59:59Z",
  "status": "ACTIVE"
}
```

> Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.

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

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "Write operations are disabled in read-only mode."
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

<h3 id="generate-an-mcp-server-api-key-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Generated API key. The plaintext `key` is returned exactly once.|[ApiKeyResponse](schemas.md#schemaapikeyresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden for the current runtime mode or caller permissions.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="generate-an-mcp-server-api-key-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the generated API key resource.|

## List MCP server API keys

<a id="opIdlistMcpServerApiKeys"></a>

`GET /mcp-servers/{mcpServerId}/api-keys`

> Code samples

```shell

curl -X GET https://localhost:9543/api/v0.9/mcp-servers/{mcpServerId}/api-keys \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Lists API keys for the given MCP server. Mirrors `GET /api/v0.9/apis/{apiId}/api-keys`.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-mcp-server-api-keys-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|appId|query|string|false|Optional application ID used to filter API keys associated with that application.|
|limit|query|integer|false|Maximum number of records to return.|
|offset|query|integer|false|Number of records to skip before returning results.|
|mcpServerId|path|string|true|The MCP server's handle (unique per org).|

> Example responses

> 200 Response

```json
{
  "list": [
    {
      "id": "weather_prod_key",
      "displayName": "Weather Prod Key",
      "apiId": "weather-api-v1",
      "appId": "my-weather-app",
      "appDisplayName": "My Mobile App",
      "status": "ACTIVE",
      "expiresAt": "2026-12-31T23:59:59Z",
      "createdAt": "2019-08-24T14:15:22Z",
      "revokedAt": "2019-08-24T14:15:22Z"
    }
  ],
  "count": 1,
  "pagination": {
    "total": 42,
    "limit": 20,
    "offset": 0
  }
}
```

> Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.

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

<h3 id="list-mcp-server-api-keys-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of API key metadata records.|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-mcp-server-api-keys-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» list|[[ApiKeyMetadataResponse](schemas.md#schemaapikeymetadataresponse)]|false|none|[API key metadata returned by list operations. Secret material is omitted.]|
|»» id|string|false|none|none|
|»» displayName|string|false|none|none|
|»» apiId|string|false|none|Developer Portal API ID the key belongs to.|
|»» appId|string¦null|false|none|ID of the application this key is associated with, if any. Analytics attribution only.|
|»» appDisplayName|string¦null|false|none|Display name of the associated application, if any.|
|»» status|string|false|none|none|
|»» expiresAt|string(date-time)¦null|false|none|none|
|»» createdAt|string(date-time)|false|none|none|
|»» revokedAt|string(date-time)¦null|false|none|none|
|» count|integer|false|none|Number of items returned in this page.|
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

## Regenerate an MCP server API key

<a id="opIdregenerateMcpServerApiKey"></a>

`POST /mcp-servers/{mcpServerId}/api-keys/regenerate`

> Code samples

```shell

curl -X POST https://localhost:9543/api/v0.9/mcp-servers/{mcpServerId}/api-keys/regenerate \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Regenerates the secret for an existing MCP server API key identified by `keyId` in the request body. Mirrors `POST /api/v0.9/apis/{apiId}/api-keys/regenerate`.

> Payload

```json
{
  "keyId": "weather_prod_key",
  "expiresAt": "2027-01-01T00:00:00Z"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="regenerate-an-mcp-server-api-key-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|Identifies the API key to regenerate by its `keyId`. `expiresAt` is optional and, if provided, updates the key's expiry; the key's `id`/`displayName` cannot be changed by this operation.|
|» keyId|body|string|true|The key's handle — the `id` returned by generate or list.|
|» expiresAt|body|any|false|New expiry for the key. Can be an ISO-8601 datetime with timezone, epoch seconds, or epoch milliseconds. Omit to leave the current expiry unchanged.|
|»» *anonymous*|body|string(date-time)|false|none|
|»» *anonymous*|body|number|false|none|
|mcpServerId|path|string|true|The MCP server's handle (unique per org).|

> Example responses

> 200 Response

```json
{
  "id": "weather_prod_key",
  "displayName": "Weather Prod Key",
  "key": "ak_dGhpcyBpcyBub3QgYSByZWFsIGtleQ",
  "expiresAt": "2026-12-31T23:59:59Z",
  "status": "ACTIVE"
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "Write operations are disabled in read-only mode."
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

<h3 id="regenerate-an-mcp-server-api-key-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Generated or regenerated API key. The plaintext `key` is returned exactly once.|[ApiKeyResponse](schemas.md#schemaapikeyresponse)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden for the current runtime mode or caller permissions.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The key has already been revoked and cannot be regenerated.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Revoke an MCP server API key

<a id="opIdrevokeMcpServerApiKey"></a>

`POST /mcp-servers/{mcpServerId}/api-keys/revoke`

> Code samples

```shell

curl -X POST https://localhost:9543/api/v0.9/mcp-servers/{mcpServerId}/api-keys/revoke \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Revokes an existing MCP server API key identified by `keyId` in the request body. Mirrors `POST /api/v0.9/apis/{apiId}/api-keys/revoke`.

> Payload

```json
{
  "keyId": "weather_prod_key"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="revoke-an-mcp-server-api-key-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|Identifies the API key to revoke by its `keyId`.|
|» keyId|body|string|true|The key's handle — the `id` returned by generate or list.|
|mcpServerId|path|string|true|The MCP server's handle (unique per org).|

> Example responses

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "Write operations are disabled in read-only mode."
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

<h3 id="revoke-an-mcp-server-api-key-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|API key revoked successfully.|None|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden for the current runtime mode or caller permissions.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The key has already been revoked.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Associate an MCP server API key with an application

<a id="opIdassociateMcpServerApiKeyApplication"></a>

`POST /mcp-servers/{mcpServerId}/api-keys/associate`

> Code samples

```shell

curl -X POST https://localhost:9543/api/v0.9/mcp-servers/{mcpServerId}/api-keys/associate \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Associates (or re-associates) an existing MCP server API key with an application, for analytics attribution only. Mirrors `POST /api/v0.9/apis/{apiId}/api-keys/associate`.

> Payload

```json
{
  "keyId": "weather_prod_key",
  "appId": "my-weather-app"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="associate-an-mcp-server-api-key-with-an-application-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|Identifies the API key and the application to associate it with.|
|» keyId|body|string|true|The key's handle — the `id` returned by generate or list.|
|» appId|body|string|true|Developer Portal application ID to associate the key with.|
|mcpServerId|path|string|true|The MCP server's handle (unique per org).|

> Example responses

> 200 Response

```json
{
  "application": {
    "id": "my-weather-app",
    "displayName": "My Mobile App"
  }
}
```

> Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.

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

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "Write operations are disabled in read-only mode."
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

<h3 id="associate-an-mcp-server-api-key-with-an-application-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Association updated.|[ApiKeyApplicationResponse](schemas.md#schemaapikeyapplicationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden for the current runtime mode or caller permissions.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The key has already been revoked and cannot be associated with an application.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="associate-an-mcp-server-api-key-with-an-application-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

## Remove an MCP server API key's application association

<a id="opIdremoveMcpServerApiKeyApplication"></a>

`POST /mcp-servers/{mcpServerId}/api-keys/dissociate`

> Code samples

```shell

curl -X POST https://localhost:9543/api/v0.9/mcp-servers/{mcpServerId}/api-keys/dissociate \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Removes the application association from an MCP server API key identified by `keyId` in the request body, if any. Mirrors `POST /api/v0.9/apis/{apiId}/api-keys/dissociate`.

> Payload

```json
{
  "keyId": "weather_prod_key"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="remove-an-mcp-server-api-key's-application-association-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|Identifies the API key to remove the application association from.|
|» keyId|body|string|true|The key's handle — the `id` returned by generate or list.|
|mcpServerId|path|string|true|The MCP server's handle (unique per org).|

> Example responses

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "Write operations are disabled in read-only mode."
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

<h3 id="remove-an-mcp-server-api-key's-application-association-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Association removed (or none existed).|None|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden for the current runtime mode or caller permissions.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|
