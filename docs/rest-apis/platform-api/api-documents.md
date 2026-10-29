<h1 id="wso2-api-platform-platform-api-api-documents">API Documents</h1>

User-authored documents (how-to, support/public forum, other) attached to any API artifact

## List API documents

<a id="opIdListAPIDocuments"></a>

`GET /apis/{apiType}/{apiId}/docs`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/apis/{apiType}/{apiId}/docs \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Returns metadata-only entries for every user-authored document attached to the API, paginated and optionally filtered by `type`.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:content:document:read`, `ap:content:document:manage`, `ap:content:read`, `ap:content:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="list-api-documents-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|type|query|string|false|Optional filter restricting the list to documents of a single type.|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**type**: Optional filter restricting the list to documents of a single type.
An unrecognised value yields an empty page rather than an error, and
the reserved `DEFINITION` type is never returned via this endpoint.

> Example responses
>
> 200 Response

```json
{
  "count": 2,
  "list": [
    {
      "id": "payment-webhook-howto",
      "type": "HOW_TO",
      "displayName": "Payment Webhook How-To",
      "fileName": "payment-webhook.md",
      "contentType": "text/markdown; charset=utf-8",
      "createdBy": "string",
      "updatedBy": "string",
      "createdAt": "2019-08-24T14:15:22Z",
      "updatedAt": "2019-08-24T14:15:22Z"
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="list-api-documents-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Documents listed successfully|[APIDocumentListResponse](schemas.md#schemaapidocumentlistresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Create an API document

<a id="opIdCreateAPIDocument"></a>

`POST /apis/{apiType}/{apiId}/docs`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/apis/{apiType}/{apiId}/docs \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -F 'id=payment-webhook-howto' \
  -F 'type=HowTo' \
  -F 'otherTypeName=FAQ' \
  -F 'displayName=Payment Webhook How-To' \
  -F 'inlineContent=string' \
  -F 'fileName=payment-webhook.md'

```

Creates a user-authored document on the API from a `file` upload or `inlineContent`.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:content:document:manage`, `ap:content:manage`, `ap:manage`

</aside>

<h3 id="create-an-api-document-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[APIDocumentRequest](schemas.md#schemaapidocumentrequest)|true|none|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

> Example responses
>
> 201 Response

```json
{
  "id": "payment-webhook-howto",
  "type": "HOW_TO",
  "displayName": "Payment Webhook How-To",
  "fileName": "payment-webhook.md",
  "contentType": "text/markdown; charset=utf-8",
  "createdBy": "string",
  "updatedBy": "string",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "The request conflicts with the current state of the resource."
}
```

> 413 Response

```json
{
  "status": "error",
  "code": "PAYLOAD_TOO_LARGE",
  "message": "The uploaded file exceeds the maximum allowed size."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="create-an-api-document-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Document created successfully|[APIDocumentMetadata](schemas.md#schemaapidocumentmetadata)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|413|[Payload Too Large](https://tools.ietf.org/html/rfc7231#section-6.5.11)|Payload Too Large. The uploaded file exceeds the maximum allowed size.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string||URL of the newly created document.|

## Get API document metadata

<a id="opIdGetAPIDocument"></a>

`GET /apis/{apiType}/{apiId}/docs/{docId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/apis/{apiType}/{apiId}/docs/{docId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Returns document metadata only. Use `GET …/{docId}/content` to retrieve
the raw document bytes.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:content:document:read`, `ap:content:document:manage`, `ap:content:read`, `ap:content:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-api-document-metadata-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|
|docId|path|string|true|Document handle (api_documents.handle), unique per API artifact.|

> Example responses
>
> 200 Response

```json
{
  "id": "payment-webhook-howto",
  "type": "HOW_TO",
  "displayName": "Payment Webhook How-To",
  "fileName": "payment-webhook.md",
  "contentType": "text/markdown; charset=utf-8",
  "createdBy": "string",
  "updatedBy": "string",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="get-api-document-metadata-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Document metadata retrieved successfully|[APIDocumentMetadata](schemas.md#schemaapidocumentmetadata)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Update an API document

<a id="opIdUpdateAPIDocument"></a>

`PUT /apis/{apiType}/{apiId}/docs/{docId}`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/apis/{apiType}/{apiId}/docs/{docId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -F 'id=payment-webhook-howto' \
  -F 'type=HowTo' \
  -F 'otherTypeName=FAQ' \
  -F 'displayName=Payment Webhook How-To' \
  -F 'inlineContent=string' \
  -F 'fileName=payment-webhook.md'

```

Updates metadata and/or content on an existing API document.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:content:document:manage`, `ap:content:manage`, `ap:manage`

</aside>

<h3 id="update-an-api-document-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[APIDocumentRequest](schemas.md#schemaapidocumentrequest)|true|none|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|
|docId|path|string|true|Document handle (api_documents.handle), unique per API artifact.|

> Example responses
>
> 200 Response

```json
{
  "id": "payment-webhook-howto",
  "type": "HOW_TO",
  "displayName": "Payment Webhook How-To",
  "fileName": "payment-webhook.md",
  "contentType": "text/markdown; charset=utf-8",
  "createdBy": "string",
  "updatedBy": "string",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 413 Response

```json
{
  "status": "error",
  "code": "PAYLOAD_TOO_LARGE",
  "message": "The uploaded file exceeds the maximum allowed size."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="update-an-api-document-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Document updated successfully|[APIDocumentMetadata](schemas.md#schemaapidocumentmetadata)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|413|[Payload Too Large](https://tools.ietf.org/html/rfc7231#section-6.5.11)|Payload Too Large. The uploaded file exceeds the maximum allowed size.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete an API document

<a id="opIdDeleteAPIDocument"></a>

`DELETE /apis/{apiType}/{apiId}/docs/{docId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/apis/{apiType}/{apiId}/docs/{docId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Deletes a user-authored document from the API.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:content:document:manage`, `ap:content:manage`, `ap:manage`

</aside>

<h3 id="delete-an-api-document-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|
|docId|path|string|true|Document handle (api_documents.handle), unique per API artifact.|

> Example responses
>
> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="delete-an-api-document-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Document deleted successfully|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get API document content

<a id="opIdGetAPIDocumentContent"></a>

`GET /apis/{apiType}/{apiId}/docs/{docId}/content`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/apis/{apiType}/{apiId}/docs/{docId}/content \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: */*'

```

Returns the raw document bytes with the stored `Content-Type` header
(e.g. `text/markdown; charset=utf-8` for markdown documents).
Extensible to any future content format without schema changes — the
stored content-type drives how the client interprets the response body.
A `Content-Disposition: inline; filename="…"` header is included when
a filename is stored.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:content:document:read`, `ap:content:document:manage`, `ap:content:read`, `ap:content:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-api-document-content-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|
|docId|path|string|true|Document handle (api_documents.handle), unique per API artifact.|

> Example responses
>
> 200 Response

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="get-api-document-content-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Document content retrieved successfully|string|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Document exists but has no content stored.|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
