<h1 id="wso2-api-platform-platform-api-secrets">Secrets</h1>

Encrypted secret management — create, rotate, and delete organization-scoped secrets referenced via {{ secret "name" }} placeholders

## Create a secret

<a id="opIdcreateSecret"></a>

`POST /secrets`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/secrets \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Create a new encrypted secret scoped to the organization. The plaintext value is never returned.

> Payload

```yaml
id: wso2-openai-key
displayName: WSO2 OpenAI API Key
description: Primary API key for WSO2 OpenAI integration
value: sk-xxx
type: GENERIC

```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:secret:create`, `ap:secret:manage`

</aside>

<h3 id="create-a-secret-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[SecretCreateRequest](schemas.md#schemasecretcreaterequest)|true|none|

> Example responses

> 201 Response

```json
{
  "id": "wso2-openai-key",
  "displayName": "WSO2 OpenAI API Key",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
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
      "field": "spec.context",
      "message": "must start with /"
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

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "The specified resource already exists."
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

> 503 Response

```json
{
  "status": "error",
  "code": "SERVICE_UNAVAILABLE",
  "message": "Secrets management is not configured.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="create-a-secret-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Secret created successfully.|[SecretResponse](schemas.md#schemasecretresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. Specified resource already exists.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|Service Unavailable. The secrets management feature is not configured.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## List secrets

<a id="opIdlistSecrets"></a>

`GET /secrets`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/secrets \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Returns metadata for all secrets in the organization. The plaintext value is
never included in list or get responses.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:secret:read`, `ap:secret:manage`

</aside>

<h3 id="list-secrets-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|
|updatedAfter|query|string(date-time)|false|RFC3339 timestamp — return only secrets updated after this time. Used by GW controller for incremental polling.|

> Example responses

> 200 Response

```json
{
  "count": 0,
  "list": [
    {
      "id": "wso2-openai-key",
      "displayName": "WSO2 OpenAI API Key",
      "description": "string",
      "type": "GENERIC",
      "provider": "IN_BUILT",
      "status": "ACTIVE",
      "hash": "hmac-sha256:b94d27b9934d3e08a52e52d7da7dabfac484efe04294e576d4b3d4c57e3f428a",
      "createdBy": "john.doe",
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

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "spec.context",
      "message": "must start with /"
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

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

> 503 Response

```json
{
  "status": "error",
  "code": "SERVICE_UNAVAILABLE",
  "message": "Secrets management is not configured.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="list-secrets-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of secret metadata|[SecretListResponse](schemas.md#schemasecretlistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|Service Unavailable. The secrets management feature is not configured.|[Error](schemas.md#schemaerror)|

## Get a secret by handle

<a id="opIdgetSecret"></a>

`GET /secrets/{secretId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/secrets/{secretId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Returns metadata for a single secret. The plaintext value is never returned.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:secret:read`, `ap:secret:manage`

</aside>

<h3 id="get-a-secret-by-handle-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|secretId|path|string|true|The secret handle|

> Example responses

> 200 Response

```json
{
  "id": "wso2-openai-key",
  "displayName": "WSO2 OpenAI API Key",
  "description": "string",
  "type": "GENERIC",
  "provider": "IN_BUILT",
  "status": "ACTIVE",
  "hash": "hmac-sha256:b94d27b9934d3e08a52e52d7da7dabfac484efe04294e576d4b3d4c57e3f428a",
  "createdBy": "john.doe",
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

> 503 Response

```json
{
  "status": "error",
  "code": "SERVICE_UNAVAILABLE",
  "message": "Secrets management is not configured.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="get-a-secret-by-handle-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Secret metadata|[SecretSummary](schemas.md#schemasecretsummary)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|Service Unavailable. The secrets management feature is not configured.|[Error](schemas.md#schemaerror)|

## Rotate a secret value

<a id="opIdrotateSecret"></a>

`PUT /secrets/{secretId}`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/secrets/{secretId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Re-encrypts and stores a new value for an existing secret. The handle is immutable
so all `{{ secret "handle" }}` placeholder references across resources remain valid
without modification.

> Payload

```yaml
id: wso2-openai-key
displayName: string
description: string
value: string

```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:secret:update`, `ap:secret:manage`

</aside>

<h3 id="rotate-a-secret-value-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|secretId|path|string|true|The secret handle|
|body|body|[SecretUpdateRequest](schemas.md#schemasecretupdaterequest)|true|none|

> Example responses

> 200 Response

```json
{
  "id": "wso2-openai-key",
  "displayName": "WSO2 OpenAI API Key",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
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
      "field": "spec.context",
      "message": "must start with /"
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

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

> 503 Response

```json
{
  "status": "error",
  "code": "SERVICE_UNAVAILABLE",
  "message": "Secrets management is not configured.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="rotate-a-secret-value-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Secret rotated successfully.|[SecretResponse](schemas.md#schemasecretresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|Service Unavailable. The secrets management feature is not configured.|[Error](schemas.md#schemaerror)|

## Delete a secret

<a id="opIddeleteSecret"></a>

`DELETE /secrets/{secretId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/secrets/{secretId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Soft-deletes a secret by marking it as DEPRECATED. Returns 409 if the secret is
still referenced by any LLM provider or API configuration.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:secret:delete`, `ap:secret:manage`

</aside>

<h3 id="delete-a-secret-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|secretId|path|string|true|The secret handle|

> Example responses

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
  "code": "SECRET_IN_USE",
  "message": "The secret is referenced by one or more active resources.",
  "details": {
    "references": [
      {
        "type": "llm_provider",
        "handle": "wso2-openai-provider",
        "name": "WSO2 OpenAI Provider"
      }
    ]
  }
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

> 503 Response

```json
{
  "status": "error",
  "code": "SERVICE_UNAVAILABLE",
  "message": "Secrets management is not configured.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="delete-a-secret-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Secret deleted successfully|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The secret is referenced by one or more active resources.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|Service Unavailable. The secrets management feature is not configured.|[Error](schemas.md#schemaerror)|
