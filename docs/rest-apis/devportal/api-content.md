<h1 id="wso2-api-developer-portal-core-devportal-routes-api-content">API Content</h1>

## Upload API content

<a id="opIdcreateApiContent"></a>

`POST /apis/{apiId}/assets`

> Code samples

```shell

curl -X POST https://localhost:3000/api/v0.9/apis/{apiId}/assets \
  -u {username}:{password} \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Uploads the static content package for an API.

The `content` ZIP must contain at least one of these root directories:
- `web/` for API landing-page assets such as markdown, HTML, CSS, JavaScript, and images.
- `docs/` for downloadable API documents.

Use `docMetadata` to add external document links that are stored alongside uploaded documents.
Use `imageMetadata` to map uploaded images to API image roles such as the API icon.

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

<h3 id="upload-api-content-parameters">Parameters</h3>

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

<h3 id="upload-api-content-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="upload-api-content-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

## Replace API content

<a id="opIdreplaceApiContent"></a>

`PUT /apis/{apiId}/assets`

> Code samples

```shell

curl -X PUT https://localhost:3000/api/v0.9/apis/{apiId}/assets \
  -u {username}:{password} \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Replaces or adds static content files for an existing API.

The upload format is the same as `POST /api/v0.9/apis/{apiId}/assets`.
Existing files with the same stored `type` and `fileName` are updated; new files are created.
Image metadata is updated only when image metadata can be resolved from the upload or request body.

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

<h3 id="replace-api-content-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
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

curl -X GET https://localhost:3000/api/v0.9/apis/{apiId}/assets?type=document&fileName=getting-started.md \
  -u {username}:{password} \
  -H 'Accept: text/css'

```

Retrieves a single stored API content file.

The `type` query parameter selects the stored content category and `fileName` selects the file
within that category. Text files and external document links are returned as text. Image files are
returned as binary content with a media type derived from the file extension.

Image files (`type=IMAGE`) are publicly readable so that an API's icon renders on the public
listing and landing pages without a session — pass `orgId` to resolve the view when no session
is present (mirrors `GET /views/{viewId}/asset`). All other content categories require a session:
an anonymous request for a non-image type is rejected.

<h3 id="get-an-api-content-file-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|type|query|string|true|Stored API content type selector. Common values are `web`, `document`, `image`, and `link`, depending on how the uploaded ZIP content was classified.|
|fileName|query|string|true|Stored API content file name to retrieve.|
|orgId|query|string|false|Organization ID used to resolve the API's public image asset when no session is present (e.g. the pre-auth listing/landing page). Ignored for authenticated requests, which use the session organization. Only honored for `type=IMAGE`.|
|apiId|path|string|true|The API's handle (unique per org). Resolves only to REST/SOAP/WS/WebSub/GraphQL APIs — MCP servers are addressed via `/mcp-servers`.|

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

curl -X DELETE https://localhost:3000/api/v0.9/apis/{apiId}/assets?type=document \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes stored API content.

Send both `type` and `fileName` to delete one file. Send only `type` to delete all stored content
files matching that content category for the API.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-api-content-files-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|type|query|string|true|Stored API content type selector. Common values are `web`, `document`, `image`, and `link`, depending on how the uploaded ZIP content was classified.|
|fileName|query|string|false|File name selector used to delete a single stored API content file.|
|apiId|path|string|true|The API's handle (unique per org). Resolves only to REST/SOAP/WS/WebSub/GraphQL APIs — MCP servers are addressed via `/mcp-servers`.|

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
