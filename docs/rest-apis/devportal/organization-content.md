<h1 id="wso2-api-developer-portal-core-devportal-routes-organization-content">Organization Content</h1>

## Upload organization layout content

<a id="opIdcreateOrgContent"></a>

`POST /o/{orgId}/devportal/v1/views/{viewName}/layout`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/o/{orgId}/devportal/v1/views/{viewName}/layout \
  -u {username}:{password} \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Uploads a ZIP file containing organization layout assets for the selected view. The service extracts the archive, reads supported files, and stores each content item under the view.

> Payload

```yaml
file: string

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="upload-organization-layout-content-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|ZIP file upload. Organization content uploads are limited to 50 MB.|
|» file|body|string(binary)|true|ZIP file containing organization layout assets.|
|orgId|path|string|true|none|
|viewName|path|string|true|none|

> Example responses

> 201 Response

```json
{
  "orgId": "string",
  "fileName": "string"
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

<h3 id="upload-organization-layout-content-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Organization content upload accepted and stored successfully.|[OrganizationContentUploadResponse](schemas.md#schemaorganizationcontentuploadresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="upload-organization-layout-content-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Replace organization layout content

<a id="opIdupdateOrgContent"></a>

`PUT /o/{orgId}/devportal/v1/views/{viewName}/layout`

> Code samples

```shell

curl -X PUT https://devportal.api-platform.io/o/{orgId}/devportal/v1/views/{viewName}/layout \
  -u {username}:{password} \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Uploads a ZIP file and upserts the extracted organization layout assets for the selected view. Existing content records are updated and missing content records are created.

> Payload

```yaml
file: string

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="replace-organization-layout-content-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|ZIP file upload. Organization content uploads are limited to 50 MB.|
|» file|body|string(binary)|true|ZIP file containing organization layout assets.|
|orgId|path|string|true|none|
|viewName|path|string|true|none|

> Example responses

> 201 Response

```json
{
  "orgId": "string",
  "fileName": "string"
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

<h3 id="replace-organization-layout-content-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Organization content upload accepted and stored successfully.|[OrganizationContentUploadResponse](schemas.md#schemaorganizationcontentuploadresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="replace-organization-layout-content-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Get a single organization layout asset

<a id="opIdgetOrgLayoutContent"></a>

`GET /o/{orgId}/devportal/v1/views/{viewName}/layout`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/views/{viewName}/layout?fileType=string&fileName=string \
  -u {username}:{password} \
  -H 'Accept: text/css'

```

Retrieves one organization layout asset when `fileType` and `fileName` are supplied as query parameters. The response content type is derived from the stored file type and extension.

<h3 id="get-a-single-organization-layout-asset-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|fileType|query|string|true|Organization content file type, such as style, image, text, template, or partial.|
|fileName|query|string|true|Stored organization content file name.|
|filePath|query|string|false|Optional relative content path used together with `fileType` and `fileName`.|
|orgId|path|string|true|none|
|viewName|path|string|true|none|

> Example responses

> 200 Response

```
"string"
```

<h3 id="get-a-single-organization-layout-asset-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Stored organization content asset.|string|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Plain text success response.|string|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Plain text success response.|string|

## Delete organization layout content

<a id="opIddeleteOrgContent"></a>

`DELETE /o/{orgId}/devportal/v1/views/{viewName}/layout`

> Code samples

```shell

curl -X DELETE https://devportal.api-platform.io/o/{orgId}/devportal/v1/views/{viewName}/layout \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes all organization layout content for the selected view, or deletes one content item when `fileName` is supplied as a query parameter.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-organization-layout-content-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|fileName|query|string|false|File name selector used by file retrieval and organization content deletion.|
|orgId|path|string|true|none|
|viewName|path|string|true|none|

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

<h3 id="delete-organization-layout-content-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Organization content deleted successfully.|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-organization-layout-content-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## List organization layout assets by file type

<a id="opIdgetOrgLayoutContentByFileType"></a>

`GET /o/{orgId}/devportal/v1/views/{viewName}/layout/{fileType}`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/views/{viewName}/layout/{fileType} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Returns all stored organization layout content records for the selected file type and view.

<h3 id="list-organization-layout-assets-by-file-type-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|viewName|path|string|true|none|
|fileType|path|string|true|none|

> Example responses

> 200 Response

```json
[
  {
    "orgId": "string",
    "fileName": "string",
    "fileContent": "string"
  }
]
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

<h3 id="list-organization-layout-assets-by-file-type-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of organization content records for the requested file type.|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Plain text success response.|string|

<h3 id="list-organization-layout-assets-by-file-type-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[[OrganizationContentListItemResponse](schemas.md#schemaorganizationcontentlistitemresponse)]|false|none|none|
|» orgId|string|false|none|none|
|» fileName|string|false|none|none|
|» fileContent|string¦null|false|none|UTF-8 content string returned for stored organization content records.|

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|
