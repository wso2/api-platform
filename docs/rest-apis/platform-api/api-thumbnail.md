<h1 id="wso2-api-platform-platform-api-api-thumbnail">API Thumbnail</h1>

## Get API thumbnail

<a id="opIdGetAPIThumbnail"></a>

`GET /apis/{apiType}/{apiId}/thumbnail`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/apis/{apiType}/{apiId}/thumbnail \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: image/jpeg'

```

Streams the stored thumbnail bytes with the sniffed `Content-Type`
header (`image/jpeg` or `image/png`).

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:content:thumbnail:read`, `ap:content:thumbnail:manage`, `ap:content:read`, `ap:content:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-api-thumbnail-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

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

<h3 id="get-api-thumbnail-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Thumbnail bytes|string|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|No thumbnail is set for this API|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Set or replace the API thumbnail

<a id="opIdUpsertAPIThumbnail"></a>

`PUT /apis/{apiType}/{apiId}/thumbnail`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/apis/{apiType}/{apiId}/thumbnail \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -F 'file=string'

```

Creates or replaces the API's singleton thumbnail.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:content:thumbnail:manage`, `ap:content:manage`, `ap:manage`

</aside>

<h3 id="set-or-replace-the-api-thumbnail-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[APIThumbnailRequest](schemas.md#schemaapithumbnailrequest)|true|none|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

> Example responses
>
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

<h3 id="set-or-replace-the-api-thumbnail-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Thumbnail stored successfully|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|413|[Payload Too Large](https://tools.ietf.org/html/rfc7231#section-6.5.11)|Payload Too Large. The uploaded file exceeds the maximum allowed size.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete the API thumbnail

<a id="opIdDeleteAPIThumbnail"></a>

`DELETE /apis/{apiType}/{apiId}/thumbnail`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/apis/{apiType}/{apiId}/thumbnail \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Removes the stored thumbnail. Subsequent `GET` returns `204` (no
thumbnail set) and the client falls back to rendering the API's name initials.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:content:thumbnail:manage`, `ap:content:manage`, `ap:manage`

</aside>

<h3 id="delete-the-api-thumbnail-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

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

<h3 id="delete-the-api-thumbnail-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Thumbnail deleted successfully|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
