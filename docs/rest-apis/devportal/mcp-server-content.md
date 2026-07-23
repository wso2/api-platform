<h1 id="wso2-api-developer-portal-core-devportal-routes-mcp-server-content">MCP Server Content</h1>

## Upload MCP server content

<a id="opIdcreateMcpServerContent"></a>

`POST /mcp-servers/{mcpServerId}/assets`

> Code samples

```shell

curl -X POST https://localhost:9543/api/v0.9/mcp-servers/{mcpServerId}/assets \
  -u {username}:{password} \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Uploads the static content package for an MCP server. Mirrors `POST /api/v0.9/apis/{apiId}/assets`.

> Payload

```yaml
content: string
docMetadata: '[{"name":"External
  guide","url":"https://example.com/docs/guide","type":"LINK"}]'
imageMetadata: '{"api-icon":"icon.png"}'

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="upload-mcp-server-content-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|API content ZIP upload.|
|» content|body|string(binary)|true|ZIP upload field named `content`.|
|» docMetadata|body|string|false|Optional JSON string containing API document link metadata.|
|» imageMetadata|body|string|false|Optional JSON string containing API image metadata.|
|mcpServerId|path|string|true|The MCP server's handle (unique per org).|

#### Detailed descriptions

**body**: API content ZIP upload.

Expected ZIP structure:
- `web/`: optional API landing-page files and images.
- `docs/`: optional downloadable documents.

At least one of `web/` or `docs/` must exist at the ZIP root.
`docMetadata` and `imageMetadata` are JSON strings because they are submitted as multipart form fields.

> Example responses

> 201 Response

```json
{
  "message": "string"
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

<h3 id="upload-mcp-server-content-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="upload-mcp-server-content-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

## Replace MCP server content

<a id="opIdreplaceMcpServerContent"></a>

`PUT /mcp-servers/{mcpServerId}/assets`

> Code samples

```shell

curl -X PUT https://localhost:9543/api/v0.9/mcp-servers/{mcpServerId}/assets \
  -u {username}:{password} \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Replaces or adds static content files for an existing MCP server. Mirrors `PUT /api/v0.9/apis/{apiId}/assets`.

> Payload

```yaml
content: string
docMetadata: '[{"name":"External
  guide","url":"https://example.com/docs/guide","type":"LINK"}]'
imageMetadata: '{"api-icon":"icon.png"}'

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="replace-mcp-server-content-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|API content ZIP upload.|
|» content|body|string(binary)|true|ZIP upload field named `content`.|
|» docMetadata|body|string|false|Optional JSON string containing API document link metadata.|
|» imageMetadata|body|string|false|Optional JSON string containing API image metadata.|
|mcpServerId|path|string|true|The MCP server's handle (unique per org).|

#### Detailed descriptions

**body**: API content ZIP upload.

Expected ZIP structure:
- `web/`: optional API landing-page files and images.
- `docs/`: optional downloadable documents.

At least one of `web/` or `docs/` must exist at the ZIP root.
`docMetadata` and `imageMetadata` are JSON strings because they are submitted as multipart form fields.

> Example responses

> 201 Response

```json
{
  "message": "string"
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

<h3 id="replace-mcp-server-content-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="replace-mcp-server-content-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

## Get an MCP server content file

<a id="opIdgetMcpServerContentFile"></a>

`GET /mcp-servers/{mcpServerId}/assets`

> Code samples

```shell

curl -X GET https://localhost:9543/api/v0.9/mcp-servers/{mcpServerId}/assets?type=document&fileName=getting-started.md \
  -u {username}:{password} \
  -H 'Accept: text/css' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves a single stored MCP server content file. Mirrors `GET /api/v0.9/apis/{apiId}/assets`.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-an-mcp-server-content-file-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|type|query|string|true|Stored API content type selector. Common values are `web`, `document`, `image`, and `link`, depending on how the uploaded ZIP content was classified.|
|fileName|query|string|true|Stored API content file name to retrieve.|
|mcpServerId|path|string|true|The MCP server's handle (unique per org).|

> Example responses

> 200 Response

```
"<section>API overview</section>"
```

```
"https://example.com/docs/guide"
```

```json
{
  "title": "API overview"
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

> 404 Response

```
"string"
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_SERVER_ERROR",
  "message": "An unexpected error occurred."
}
```

<h3 id="get-an-mcp-server-content-file-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Stored API content asset. The concrete media type depends on the stored file extension or whether the content is an external document link.|string|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Plain text success response.|string|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="get-an-mcp-server-content-file-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

## Delete MCP server content files

<a id="opIddeleteMcpServerContentFile"></a>

`DELETE /mcp-servers/{mcpServerId}/assets`

> Code samples

```shell

curl -X DELETE https://localhost:9543/api/v0.9/mcp-servers/{mcpServerId}/assets?type=document \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes stored MCP server content. Mirrors `DELETE /api/v0.9/apis/{apiId}/assets`.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-mcp-server-content-files-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|type|query|string|true|Stored API content type selector. Common values are `web`, `document`, `image`, and `link`, depending on how the uploaded ZIP content was classified.|
|fileName|query|string|false|File name selector used to delete a single stored API content file.|
|mcpServerId|path|string|true|The MCP server's handle (unique per org).|

> Example responses

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

> 404 Response

```
"string"
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_SERVER_ERROR",
  "message": "An unexpected error occurred."
}
```

<h3 id="delete-mcp-server-content-files-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|MCP server content deleted successfully.|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Plain text success response.|string|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-mcp-server-content-files-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
