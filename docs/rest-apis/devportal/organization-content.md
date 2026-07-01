<h1 id="wso2-api-developer-portal-core-devportal-routes-organization-content">Organization Content</h1>

## Get a theme asset

<a id="opIdgetOrgAsset"></a>

`GET /devportal/v1/views/{viewId}/asset`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/devportal/v1/views/{viewId}/asset?fileType=string&fileName=string \
  -u {username}:{password} \
  -H 'Accept: text/css'

```

Retrieves a single organization theme asset (CSS, image, etc.) by `fileType` and `fileName` query parameters. The response content type is derived from the stored file type and extension.

<h3 id="get-a-theme-asset-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|fileType|query|string|true|Organization content file type, such as style, image, text, template, or partial.|
|fileName|query|string|true|Stored organization content file name.|
|filePath|query|string|false|Optional relative content path used together with `fileType` and `fileName`.|
|viewId|path|string|true|The view's handle (unique per org). Not the internal database uuid.|

> Example responses

> 200 Response

```
"string"
```

<h3 id="get-a-theme-asset-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Stored organization content asset.|string|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid or missing `fileType`/`fileName` query parameters.|string|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|No matching organization content asset was found.|string|

## Apply a theme

<a id="opIdapplyTheme"></a>

`POST /devportal/v1/views/{viewId}/apply-theme`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/devportal/v1/views/{viewId}/apply-theme \
  -u {username}:{password} \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Uploads a ZIP file and atomically replaces all existing theme assets for the view. All previously stored files are deleted before the new ones are extracted and stored.

> Payload

```yaml
file: string

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="apply-a-theme-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|ZIP file upload. Organization content uploads are limited to 50 MB.|
|» file|body|string(binary)|true|ZIP file containing organization layout assets.|
|viewId|path|string|true|The view's handle (unique per org). Not the internal database uuid.|

> Example responses

> 200 Response

```json
{
  "id": "string",
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

<h3 id="apply-a-theme-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Organization content upload accepted and stored successfully.|[OrganizationContentUploadResponse](schemas.md#schemaorganizationcontentuploadresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="apply-a-theme-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Reset theme to defaults

<a id="opIdresetTheme"></a>

`POST /devportal/v1/views/{viewId}/reset-theme`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/devportal/v1/views/{viewId}/reset-theme \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes all stored theme assets for the view, reverting it to built-in defaults.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="reset-theme-to-defaults-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|viewId|path|string|true|The view's handle (unique per org). Not the internal database uuid.|

> Example responses

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_SERVER_ERROR",
  "message": "An unexpected error occurred."
}
```

<h3 id="reset-theme-to-defaults-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Theme reset successfully.|None|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|
