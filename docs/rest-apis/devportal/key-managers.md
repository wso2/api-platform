<h1 id="wso2-api-developer-portal-core-devportal-routes-key-managers">Key Managers</h1>

## Create a key manager

<a id="opIdcreateKeyManager"></a>

`POST /api/v0.9/key-managers`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/api/v0.9/key-managers \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates a key manager configuration for the organization. Accepts either a `application/json` body or a `multipart/form-data` upload with a `keymanager` field containing the KeyManager YAML file. OAuth applications are created directly in the key manager itself, outside the portal — the portal only needs the token endpoint to proxy `client_appKeyMappings` token requests.

> Payload

```json
{
  "id": "Asgardeo",
  "type": "ASGARDEO",
  "enabled": true,
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token"
}
```

```yaml
id: Asgardeo
type: ASGARDEO
enabled: true
tokenEndpoint: https://api.asgardeo.io/t/myorg/oauth2/token

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-a-key-manager-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[KeyManagerRequest](schemas.md#schemakeymanagerrequest)|false|Key manager configuration payload. Submit as `application/json` or as `multipart/form-data` with a `keymanager` field containing a KeyManager YAML file.|

> Example responses

> 201 Response

```json
{
  "id": "Asgardeo",
  "orgId": "org-12345",
  "type": "ASGARDEO",
  "enabled": true,
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token",
  "createdBy": "alice@example.com",
  "updatedBy": "alice@example.com",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
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

<h3 id="create-a-key-manager-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Key manager configuration response.|[KeyManagerResponseSchema](schemas.md#schemakeymanagerresponseschema)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-a-key-manager-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the created key manager.|

## List key managers

<a id="opIdgetKeyManagers"></a>

`GET /api/v0.9/key-managers`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/api/v0.9/key-managers \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns key manager configurations for the organization. Admins receive the full configuration for every key manager, including disabled ones; other callers receive the minimal, developer-facing view of enabled key managers only, with no admin credentials. Admin appKeyMappings are never included in the response.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-key-managers-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|limit|query|integer|false|Maximum number of records to return.|
|offset|query|integer|false|Number of records to skip before returning results.|

> Example responses

> 200 Response

```json
{
  "list": [
    {
      "id": "km-uuid-12345",
      "orgId": "org-12345",
      "name": "Asgardeo",
      "type": "ASGARDEO",
      "enabled": true,
      "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token",
      "createdBy": "alice@example.com",
      "createdAt": "2026-05-07T08:30:00Z",
      "updatedAt": "2026-05-07T08:30:00Z"
    }
  ],
  "pagination": {
    "total": 42,
    "limit": 20,
    "offset": 0
  }
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

<h3 id="list-key-managers-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of key manager configurations. Admins receive KeyManagerResponseSchema items; other callers receive the minimal KeyManagerPublicResponseSchema items.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-key-managers-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» list|[anyOf]|false|none|none|

*anyOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|[KeyManagerResponseSchema](schemas.md#schemakeymanagerresponseschema)|false|none|Key manager configuration.|
|»»» id|string|false|none|The key manager's handle (unique per org). Not the internal database uuid.|
|»»» orgId|string|false|none|none|
|»»» type|string|false|none|none|
|»»» enabled|boolean|false|none|none|
|»»» tokenEndpoint|string(uri)|false|none|none|
|»»» createdBy|string|false|none|Identity of the user who created this key manager, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses and list items.|
|»»» updatedBy|string|false|none|Identity of the user who last updated this key manager, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses only, omitted on list items.|
|»»» createdAt|string(date-time)|false|none|none|
|»»» updatedAt|string(date-time)|false|none|none|

*or*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|[KeyManagerPublicResponseSchema](schemas.md#schemakeymanagerpublicresponseschema)|false|none|Minimal developer-facing key manager view.|
|»»» id|string|false|none|The key manager's handle (unique per org). Not the internal database uuid.|
|»»» type|string|false|none|none|
|»»» tokenEndpoint|string(uri)|false|none|none|

*continued*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» pagination|[Pagination](schemas.md#schemapagination)|false|none|Standard pagination metadata returned with collection responses.|
|»» total|integer|true|none|Total number of records matching the query.|
|»» limit|integer|true|none|Maximum number of records returned in this response.|
|»» offset|integer|true|none|Number of records skipped before this page.|

#### Enumerated Values

|Property|Value|
|---|---|
|type|ASGARDEO|
|type|WSO2IS|
|type|KEYCLOAK|
|type|GENERIC_OIDC|
|type|ASGARDEO|
|type|WSO2IS|
|type|KEYCLOAK|
|type|GENERIC_OIDC|

## Get a key manager

<a id="opIdgetKeyManager"></a>

`GET /api/v0.9/key-managers/{kmId}`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/api/v0.9/key-managers/{kmId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves a single key manager configuration by ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-a-key-manager-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|kmId|path|string|true|The key manager's handle (its `id` in request/response payloads), not the internal database uuid.|

> Example responses

> 200 Response

```json
{
  "id": "Asgardeo",
  "orgId": "org-12345",
  "type": "ASGARDEO",
  "enabled": true,
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token",
  "createdBy": "alice@example.com",
  "updatedBy": "alice@example.com",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
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

<h3 id="get-a-key-manager-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Key manager configuration response.|[KeyManagerResponseSchema](schemas.md#schemakeymanagerresponseschema)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update a key manager

<a id="opIdupdateKeyManager"></a>

`PUT /api/v0.9/key-managers/{kmId}`

> Code samples

```shell

curl -X PUT https://devportal.api-platform.io/api/v0.9/key-managers/{kmId} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates an existing key manager configuration. Accepts either a `application/json` body or a `multipart/form-data` upload with a `keymanager` YAML file. Only supplied fields are updated; omitted fields retain their stored values.

> Payload

```json
{
  "id": "Asgardeo",
  "type": "ASGARDEO",
  "enabled": true,
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token"
}
```

```yaml
id: Asgardeo
type: ASGARDEO
enabled: true
tokenEndpoint: https://api.asgardeo.io/t/myorg/oauth2/token

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="update-a-key-manager-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[KeyManagerUpdateRequest](schemas.md#schemakeymanagerupdaterequest)|false|Key manager update payload. All fields are optional; only supplied fields are updated. Submit as `application/json` or as `multipart/form-data` with a `keymanager` field containing a KeyManager YAML file.|
|kmId|path|string|true|The key manager's handle (its `id` in request/response payloads), not the internal database uuid.|

> Example responses

> 200 Response

```json
{
  "id": "Asgardeo",
  "orgId": "org-12345",
  "type": "ASGARDEO",
  "enabled": true,
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token",
  "createdBy": "alice@example.com",
  "updatedBy": "alice@example.com",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
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

<h3 id="update-a-key-manager-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Key manager configuration response.|[KeyManagerResponseSchema](schemas.md#schemakeymanagerresponseschema)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-a-key-manager-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Delete a key manager

<a id="opIddeleteKeyManager"></a>

`DELETE /api/v0.9/key-managers/{kmId}`

> Code samples

```shell

curl -X DELETE https://devportal.api-platform.io/api/v0.9/key-managers/{kmId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes a key manager configuration by ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-a-key-manager-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|kmId|path|string|true|The key manager's handle (its `id` in request/response payloads), not the internal database uuid.|

> Example responses

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

<h3 id="delete-a-key-manager-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Key manager deleted successfully.|None|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|
