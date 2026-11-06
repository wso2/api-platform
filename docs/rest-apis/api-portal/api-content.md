<h1 id="wso2-api-portal-and-mcp-hub-api-content">API Content</h1>

## Add API content

<a id="opIdcreateApiContent"></a>

`POST /apis/{apiId}/assets`

> Code samples

```shell

curl -X POST https://localhost:9543/api-portal/api/v0.9/apis/{apiId}/assets \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -F 'content=string' \
  -F 'docMetadata=[{"name":"External guide","url":"https://example.com/docs/guide","type":"LINK"}]' \
  -F 'imageMetadata={"api-icon":"icon.png"}'

```

Adds new content to an API. Existing content is never changed or removed.

If any uploaded file, image slot or link already exists, the whole request is rejected with `409`,
and the response lists every clash. Nothing is stored. To change an existing file, delete it and add
it again, or replace all content with `PUT`.

An upload with no content is rejected with `400`. The upload format is described in the request body.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `dp:api_content:create`, `dp:api_content:manage`

</aside>

<h3 id="add-api-content-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|API content ZIP upload.|
|» content|body|string(binary)|true|ZIP upload field named `content`.|
|» docMetadata|body|string|false|Optional JSON string containing API document link metadata.|
|» imageMetadata|body|string|false|Optional JSON string containing API image metadata.|
|apiId|path|string|true|The API's handle (unique per org). Resolves only to REST/SOAP/WS/WebSub/GraphQL APIs — MCP servers are addressed via `/mcp-servers`.|

#### Detailed descriptions

**body**: API content ZIP upload.

Expected ZIP structure:
- `web/`: landing-page files (`.md`, `.hbs`, `.html`, `.json`, `.yaml`) and images. Every image fills the
  slot named by its file name without the extension; `api-icon.png` or `api-icon.jpg` is the thumbnail.
- `docs/`: documents. Files directly in `docs/` are stored as `DOC_Other`; files in `docs/<Category>/` as
  `DOC_<Category>`.

Only `web/` and `docs/` are read, optionally inside one wrapper folder (a lone `web/` or `docs/` is never
treated as a wrapper). An upload with no content is accepted by `PUT` (it removes all content) and
rejected by `POST`. Links in `docMetadata` count as content.

`docMetadata` and `imageMetadata` are JSON strings because they are submitted as multipart form fields.

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

<h3 id="add-api-content-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Some of the uploaded content already exists. Nothing was stored; every clash is listed.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="add-api-content-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

## Replace API content

<a id="opIdreplaceApiContent"></a>

`PUT /apis/{apiId}/assets`

> Code samples

```shell

curl -X PUT https://localhost:9543/api-portal/api/v0.9/apis/{apiId}/assets \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -F 'content=string' \
  -F 'docMetadata=[{"name":"External guide","url":"https://example.com/docs/guide","type":"LINK"}]' \
  -F 'imageMetadata={"api-icon":"icon.png"}'

```

Replaces all content of an API with the upload, in one step: documents, links, landing-page files
and images (including the thumbnail). Anything not in the upload is removed. The API definition and
schema are never changed.

An empty ZIP (or one holding only an empty wrapper folder) with no `docMetadata` links removes all
content. The upload format is the same as for `POST`.

Returns `201` when the API had no content and the upload adds some, and `200` otherwise.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `dp:api_content:update`, `dp:api_content:manage`

</aside>

<h3 id="replace-api-content-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|API content ZIP upload.|
|» content|body|string(binary)|true|ZIP upload field named `content`.|
|» docMetadata|body|string|false|Optional JSON string containing API document link metadata.|
|» imageMetadata|body|string|false|Optional JSON string containing API image metadata.|
|apiId|path|string|true|The API's handle (unique per org). Resolves only to REST/SOAP/WS/WebSub/GraphQL APIs — MCP servers are addressed via `/mcp-servers`.|

#### Detailed descriptions

**body**: API content ZIP upload.

Expected ZIP structure:
- `web/`: landing-page files (`.md`, `.hbs`, `.html`, `.json`, `.yaml`) and images. Every image fills the
  slot named by its file name without the extension; `api-icon.png` or `api-icon.jpg` is the thumbnail.
- `docs/`: documents. Files directly in `docs/` are stored as `DOC_Other`; files in `docs/<Category>/` as
  `DOC_<Category>`.

Only `web/` and `docs/` are read, optionally inside one wrapper folder (a lone `web/` or `docs/` is never
treated as a wrapper). An upload with no content is accepted by `PUT` (it removes all content) and
rejected by `POST`. Links in `docMetadata` count as content.

`docMetadata` and `imageMetadata` are JSON strings because they are submitted as multipart form fields.

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

<h3 id="replace-api-content-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="replace-api-content-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

## Get an API content file

<a id="opIdgetApiContentFile"></a>

`GET /apis/{apiId}/assets`

> Code samples

```shell

curl -X GET https://localhost:9543/api-portal/api/v0.9/apis/{apiId}/assets?type=document&fileName=getting-started.md \
  -u {username}:{password} \
  -H 'Accept: text/css'

```

Retrieves a single stored API content file.

The `type` query parameter selects the stored content category and `fileName` selects the file
within that category. Text files and external document links are returned as text. Image files are
returned as binary content with a media type derived from the file extension.

Image files (`type=IMAGE`) are publicly readable so that an API's icon renders on the public
listing and landing pages without a session; the organization is then this instance's own
configured one (mirrors `GET /views/{viewId}/asset`). All other content categories require a
session: an anonymous request for a non-image type is rejected.

<h3 id="get-an-api-content-file-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|type|query|string|true|Stored API content type selector. Common values are `web`, `document`, `image`, and `link`, depending on how the uploaded ZIP content was classified.|
|fileName|query|string|true|Stored API content file name to retrieve.|
|orgId|query|string|false|DEPRECATED and ignored. Accepted only so existing callers (the portal's own image-URL rewrite appends it) are not rejected. The organization is always this instance's own — from the session when there is one, otherwise from `organization.handle` configuration. It was previously honoured on this unauthenticated endpoint, which made it a selector for any organization's API icons in a shared database.|
|apiId|path|string|true|The API's handle (unique per org). Resolves only to REST/SOAP/WS/WebSub/GraphQL APIs — MCP servers are addressed via `/mcp-servers`.|

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

<h3 id="get-an-api-content-file-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Stored API content asset. The concrete media type depends on the stored file extension or whether the content is an external document link.|string|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Plain text success response.|string|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="get-an-api-content-file-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

## Delete API content files

<a id="opIddeleteApiContentFile"></a>

`DELETE /apis/{apiId}/assets`

> Code samples

```shell

curl -X DELETE https://localhost:9543/api-portal/api/v0.9/apis/{apiId}/assets?type=document \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Deletes stored API content.

Send both `type` and `fileName` to delete one file. Send only `type` to delete all stored content
files matching that content category for the API.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `dp:api_content:delete`, `dp:api_content:manage`

</aside>

<h3 id="delete-api-content-files-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|type|query|string|true|Stored API content type selector. Common values are `web`, `document`, `image`, and `link`, depending on how the uploaded ZIP content was classified.|
|fileName|query|string|false|File name selector used to delete a single stored API content file.|
|apiId|path|string|true|The API's handle (unique per org). Resolves only to REST/SOAP/WS/WebSub/GraphQL APIs — MCP servers are addressed via `/mcp-servers`.|

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

<h3 id="delete-api-content-files-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|API content deleted successfully.|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Plain text success response.|string|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-api-content-files-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
