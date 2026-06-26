<h1 id="wso2-api-developer-portal-core-devportal-routes-api-content">API Content</h1>

## Upload API content

<a id="opIdcreateApiContent"></a>

`POST /o/{orgId}/devportal/v1/apis/{apiId}/content`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/o/{orgId}/devportal/v1/apis/{apiId}/content \
  -u {username}:{password} \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Uploads the static content package for an API.

The `apiContent` ZIP must contain at least one of these root directories:
- `web/` for API landing-page assets such as markdown, HTML, CSS, JavaScript, and images.
- `docs/` for downloadable API documents.

Use `docMetadata` to add external document links that are stored alongside uploaded documents.
Use `imageMetadata` to map uploaded images to API image roles such as the API icon.

> Payload

```yaml
apiContent: string
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
|» apiContent|body|string(binary)|true|ZIP upload field named `apiContent`.|
|» docMetadata|body|string|false|Optional JSON string containing API document link metadata.|
|» imageMetadata|body|string|false|Optional JSON string containing API image metadata.|
|orgId|path|string|true|none|
|apiId|path|string|true|none|

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
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="upload-api-content-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Replace API content

<a id="opIdreplaceApiContent"></a>

`PUT /o/{orgId}/devportal/v1/apis/{apiId}/content`

> Code samples

```shell

curl -X PUT https://devportal.api-platform.io/o/{orgId}/devportal/v1/apis/{apiId}/content \
  -u {username}:{password} \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Replaces or adds static content files for an existing API.

The upload format is the same as `POST /o/{orgId}/devportal/v1/apis/{apiId}/content`.
Existing files with the same stored `type` and `fileName` are updated; new files are created.
Image metadata is updated only when image metadata can be resolved from the upload or request body.

> Payload

```yaml
apiContent: string
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
|» apiContent|body|string(binary)|true|ZIP upload field named `apiContent`.|
|» docMetadata|body|string|false|Optional JSON string containing API document link metadata.|
|» imageMetadata|body|string|false|Optional JSON string containing API image metadata.|
|orgId|path|string|true|none|
|apiId|path|string|true|none|

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
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="replace-api-content-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Get an API content file

<a id="opIdgetApiContentFile"></a>

`GET /o/{orgId}/devportal/v1/apis/{apiId}/content`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/apis/{apiId}/content?type=document&fileName=getting-started.md \
  -u {username}:{password} \
  -H 'Accept: text/css' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves a single stored API content file.

The `type` query parameter selects the stored content category and `fileName` selects the file
within that category. Text files and external document links are returned as text. Image files are
returned as binary content with a media type derived from the file extension.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-an-api-content-file-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|type|query|string|true|Stored API content type selector. Common values are `web`, `document`, `image`, and `link`, depending on how the uploaded ZIP content was classified.|
|fileName|query|string|true|Stored API content file name to retrieve.|
|orgId|path|string|true|none|
|apiId|path|string|true|none|

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
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Plain text success response.|string|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="get-an-api-content-file-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Delete API content files

<a id="opIddeleteApiContentFile"></a>

`DELETE /o/{orgId}/devportal/v1/apis/{apiId}/content`

> Code samples

```shell

curl -X DELETE https://devportal.api-platform.io/o/{orgId}/devportal/v1/apis/{apiId}/content?type=document \
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
|fileName|query|string|false|File name selector used by file retrieval and organization content deletion.|
|orgId|path|string|true|none|
|apiId|path|string|true|none|

> Example responses

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
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Plain text success response.|string|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-api-content-files-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## List API document file names

<a id="opIdlistApiDocs"></a>

`GET /o/{orgId}/devportal/v1/apis/{apiId}/docs`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/apis/{apiId}/docs \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns the file names of all documents uploaded for an API via the wizard.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-api-document-file-names-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|apiId|path|string|true|none|

> Example responses

> 200 Response

```json
[
  {
    "fileName": "string"
  }
]
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_SERVER_ERROR",
  "message": "An unexpected error occurred."
}
```

<h3 id="list-api-document-file-names-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of document file names|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-api-document-file-names-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» fileName|string|false|none|none|
