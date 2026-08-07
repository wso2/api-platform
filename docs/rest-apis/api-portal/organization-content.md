<h1 id="wso2-api-portal-and-mcp-hub-organization-content">Organization Content</h1>

## Get a theme asset

<a id="opIdgetOrgAsset"></a>

`GET /views/{viewId}/asset`

> Code samples

```shell

curl -X GET https://localhost:9543/api-portal/api/v0.9/views/{viewId}/asset?fileType=string&fileName=string \
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
|orgId|query|string|false|DEPRECATED and ignored. Accepted only so existing callers (the portal's own style-URL rewrite appends it) are not rejected. The organization is always this instance's own — from the session when there is one, otherwise from `organization.handle` configuration. It was previously honoured on this unauthenticated endpoint, which made it a selector for any organization's branding in a shared database.|
|viewId|path|string|true|The view's handle (unique per org). Not the internal database uuid.|

> Example responses
>
> 200 Response

```
"string"
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

<h3 id="get-a-theme-asset-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Stored organization content asset.|string|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="get-a-theme-asset-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

## Apply a theme

<a id="opIdapplyTheme"></a>

`POST /views/{viewId}/apply-theme`

> Code samples

```shell

curl -X POST https://localhost:9543/api-portal/api/v0.9/views/{viewId}/apply-theme \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -F 'file=string'

```

Uploads a ZIP file and atomically replaces the view's theme assets. Only the assets contained in the uploaded ZIP are present afterward.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `dp:organization_content:manage`, `dp:organization:manage`

</aside>

<h3 id="apply-a-theme-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|ZIP file upload. Organization content uploads are limited to 50 MB.|
|» file|body|string(binary)|true|ZIP file containing organization theme assets.|
|viewId|path|string|true|The view's handle (unique per org). Not the internal database uuid.|

> Example responses
>
> 200 Response

```json
{
  "id": "string",
  "fileName": "string"
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

<h3 id="apply-a-theme-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Organization content upload accepted and stored successfully.|[OrganizationContentUploadResponse](schemas.md#schemaorganizationcontentuploadresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="apply-a-theme-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

## Reset theme to defaults

<a id="opIdresetTheme"></a>

`POST /views/{viewId}/reset-theme`

> Code samples

```shell

curl -X POST https://localhost:9543/api-portal/api/v0.9/views/{viewId}/reset-theme \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Deletes all stored theme assets for the view, reverting it to built-in defaults.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `dp:organization_content:manage`, `dp:organization:manage`

</aside>

<h3 id="reset-theme-to-defaults-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|viewId|path|string|true|The view's handle (unique per org). Not the internal database uuid.|

> Example responses
>
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

## Download the current theme

<a id="opIdexportTheme"></a>

`GET /views/{viewId}/export-theme`

> Code samples

```shell

curl -X GET https://localhost:9543/api-portal/api/v0.9/views/{viewId}/export-theme \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/zip'

```

Bundles the view's current custom theme assets into a single ZIP archive for download. The archive is wrapped in a top-level folder so it can be re-uploaded via the apply-theme endpoint. Returns 404 when the view has no custom theme.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `dp:organization_content:read`, `dp:organization:manage`

</aside>

<h3 id="download-the-current-theme-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|viewId|path|string|true|The view's handle (unique per org). Not the internal database uuid.|

> Example responses
>
> 200 Response

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

<h3 id="download-the-current-theme-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Theme archive.|string|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|
