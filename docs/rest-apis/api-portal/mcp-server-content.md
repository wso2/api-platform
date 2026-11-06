<h1 id="wso2-api-portal-and-mcp-hub-mcp-server-content">MCP Server Content</h1>

## Add MCP server content

<a id="opIdcreateMcpServerContent"></a>

`POST /mcp-servers/{mcpServerId}/assets`

> Code samples

```shell

curl -X POST https://localhost:9543/api-portal/api/v0.9/mcp-servers/{mcpServerId}/assets \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -F 'content=string' \
  -F 'docMetadata=[{"type":"DOC_LINK","links":[{"displayName":"External guide","url":"https://example.com/docs/guide"}]}]' \
  -F 'imageMetadata={"api-icon":"icon.png"}'

```

Adds new content to an MCP server; existing content is never changed or removed, and any clash rejects the whole request with `409`. Mirrors `POST /api-portal/api/v0.9/apis/{apiId}/assets`.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `dp:mcp_server_content:create`, `dp:mcp_server_content:manage`

</aside>

<h3 id="add-mcp-server-content-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|API content ZIP upload.|
|» content|body|string(binary)|true|ZIP upload field named `content`.|
|» docMetadata|body|string|false|Optional JSON array of document links, grouped by type. Each `type` must be `DOC_<Category>`. A link is stored as `LINK_<displayName>` under that type.|
|» imageMetadata|body|string|false|Optional JSON object mapping an image slot to an uploaded file name, overriding the slot taken from the file name. Each named file must be in the upload and fills one slot. Two images whose names differ only by extension take the same slot, so one of them must be named here.|
|mcpServerId|path|string|true|The MCP server's handle (unique per org).|

#### Detailed descriptions

**body**: API content ZIP upload.

Each path in the ZIP gets one outcome, from the first rule that matches:
- If the ZIP breaks an extraction limit or holds an unsafe path, the whole upload is rejected.
- If the only top-level entry is one folder other than `web/` or `docs/`, the ZIP is read from inside it.
- A path with a part starting with `.` (`.gitkeep`, `.git/`, `._A.md`, `.DS_Store`), or under `__MACOSX/`, is
  ignored.
- A path that appears more than once is rejected.
- `web/apiContent.md` and `web/api-content.hbs` are the landing page.
- `web/<name>` ending in `.png`, `.jpg`, `.jpeg`, `.gif`, `.svg` or `.ico` (any letter case) is an image. It
  fills the slot named by its file name without the extension, unless `imageMetadata` names another;
  `api-icon.png` or `api-icon.jpg` is the thumbnail.
- `docs/<name>.md` is a document stored as `DOC_Other`, and `docs/<Category>/<name>.md` (one folder level) as
  `DOC_<Category>`. The `.md` extension is lowercase.
- Anything else is rejected, including an unknown top-level folder that is empty.

If any path is rejected, the upload is rejected with `400` and nothing is stored; `errors` lists every rejected
path, sorted, as `<path>: <reason>`. An upload with no content is accepted by `PUT` (it removes all content)
and rejected by `POST`. Links in `docMetadata` count as content.

`docMetadata` and `imageMetadata` are JSON strings because they are submitted as multipart form fields;
text that isn't valid JSON, or of the wrong shape, is rejected with `400`.

> Example responses
>
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
  "message": "Conflict",
  "errors": [
    {
      "message": "Content already exists: A.md (DOC_HowTo)"
    },
    {
      "message": "Content already exists: image slot api-icon"
    }
  ]
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

<h3 id="add-mcp-server-content-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Some of the uploaded content already exists. Nothing was stored; every clash is listed.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="add-mcp-server-content-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

## Replace MCP server content

<a id="opIdreplaceMcpServerContent"></a>

`PUT /mcp-servers/{mcpServerId}/assets`

> Code samples

```shell

curl -X PUT https://localhost:9543/api-portal/api/v0.9/mcp-servers/{mcpServerId}/assets \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -F 'content=string' \
  -F 'docMetadata=[{"type":"DOC_LINK","links":[{"displayName":"External guide","url":"https://example.com/docs/guide"}]}]' \
  -F 'imageMetadata={"api-icon":"icon.png"}'

```

Replaces all content of an MCP server with the upload; anything not in it is removed, and an empty ZIP with no `docMetadata` links removes all content. The tools schema is never changed. Returns `201` when the server had no content and the upload adds some, and `200` otherwise. Mirrors `PUT /api-portal/api/v0.9/apis/{apiId}/assets`.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `dp:mcp_server_content:update`, `dp:mcp_server_content:manage`

</aside>

<h3 id="replace-mcp-server-content-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|API content ZIP upload.|
|» content|body|string(binary)|true|ZIP upload field named `content`.|
|» docMetadata|body|string|false|Optional JSON array of document links, grouped by type. Each `type` must be `DOC_<Category>`. A link is stored as `LINK_<displayName>` under that type.|
|» imageMetadata|body|string|false|Optional JSON object mapping an image slot to an uploaded file name, overriding the slot taken from the file name. Each named file must be in the upload and fills one slot. Two images whose names differ only by extension take the same slot, so one of them must be named here.|
|mcpServerId|path|string|true|The MCP server's handle (unique per org).|

#### Detailed descriptions

**body**: API content ZIP upload.

Each path in the ZIP gets one outcome, from the first rule that matches:
- If the ZIP breaks an extraction limit or holds an unsafe path, the whole upload is rejected.
- If the only top-level entry is one folder other than `web/` or `docs/`, the ZIP is read from inside it.
- A path with a part starting with `.` (`.gitkeep`, `.git/`, `._A.md`, `.DS_Store`), or under `__MACOSX/`, is
  ignored.
- A path that appears more than once is rejected.
- `web/apiContent.md` and `web/api-content.hbs` are the landing page.
- `web/<name>` ending in `.png`, `.jpg`, `.jpeg`, `.gif`, `.svg` or `.ico` (any letter case) is an image. It
  fills the slot named by its file name without the extension, unless `imageMetadata` names another;
  `api-icon.png` or `api-icon.jpg` is the thumbnail.
- `docs/<name>.md` is a document stored as `DOC_Other`, and `docs/<Category>/<name>.md` (one folder level) as
  `DOC_<Category>`. The `.md` extension is lowercase.
- Anything else is rejected, including an unknown top-level folder that is empty.

If any path is rejected, the upload is rejected with `400` and nothing is stored; `errors` lists every rejected
path, sorted, as `<path>: <reason>`. An upload with no content is accepted by `PUT` (it removes all content)
and rejected by `POST`. Links in `docMetadata` count as content.

`docMetadata` and `imageMetadata` are JSON strings because they are submitted as multipart form fields;
text that isn't valid JSON, or of the wrong shape, is rejected with `400`.

> Example responses
>
> 200 Response

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

<h3 id="replace-mcp-server-content-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
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

curl -X GET https://localhost:9543/api-portal/api/v0.9/mcp-servers/{mcpServerId}/assets?type=DOC_HowTo&fileName=getting-started.md \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: text/css'

```

Retrieves a single stored MCP server content file. Mirrors `GET /api-portal/api/v0.9/apis/{apiId}/assets`.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `dp:mcp_server_content:read`, `dp:mcp_server_content:manage`

</aside>

<h3 id="get-an-mcp-server-content-file-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|type|query|string|true|Stored content type, matched exactly. `MARKETING` for landing-page files, `IMAGE` for images (including the thumbnail), and `DOC_<Category>` for documents and links (for example `DOC_HowTo`, `DOC_Other`, `DOC_LINK`). Reads also accept `API_DEFINITION` and `SCHEMA_DEFINITION`; deletes do not.|
|fileName|query|string|true|Stored API content file name to retrieve.|
|mcpServerId|path|string|true|The MCP server's handle (unique per org).|

> Example responses
>
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

curl -X DELETE https://localhost:9543/api-portal/api/v0.9/mcp-servers/{mcpServerId}/assets?type=DOC_HowTo \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Deletes stored MCP server content, matching `type` exactly. Mirrors `DELETE /api-portal/api/v0.9/apis/{apiId}/assets`.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `dp:mcp_server_content:delete`, `dp:mcp_server_content:manage`

</aside>

<h3 id="delete-mcp-server-content-files-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|type|query|string|true|Stored content type, matched exactly. `MARKETING` for landing-page files, `IMAGE` for images (including the thumbnail), and `DOC_<Category>` for documents and links (for example `DOC_HowTo`, `DOC_Other`, `DOC_LINK`). Reads also accept `API_DEFINITION` and `SCHEMA_DEFINITION`; deletes do not.|
|fileName|query|string|false|File name selector used to delete a single stored API content file.|
|mcpServerId|path|string|true|The MCP server's handle (unique per org).|

> Example responses
>
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

<h3 id="delete-mcp-server-content-files-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|MCP server content deleted successfully.|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-mcp-server-content-files-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
