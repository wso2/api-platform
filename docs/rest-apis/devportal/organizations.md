<h1 id="wso2-api-developer-portal-core-devportal-routes-organizations">Organizations</h1>

## Create an organization

<a id="opIdcreateOrganization"></a>

`POST /organizations`

> Code samples

```shell

curl -X POST https://localhost:3000/api/v0.9/organizations \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates a Developer Portal organization and initializes its default portal configuration, default label, default view, and default subscription plans when configured.

> Payload

```json
{
  "displayName": "Acme Corporation",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "id": "acme",
  "idpRefId": "string",
  "cpRefId": "string",
  "configuration": {
    "devportalMode": "DEFAULT"
  }
}
```

```yaml
displayName: Acme Corporation
businessOwner: string
businessOwnerContact: string
businessOwnerEmail: user@example.com
id: acme
idpRefId: string
cpRefId: string
configuration:
  devportalMode: DEFAULT

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-an-organization-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[OrganizationCreateRequest](schemas.md#schemaorganizationcreaterequest)|true|Organization creation payload. Send JSON or an organization YAML file in the `organization` multipart field. The JSON example below applies only to the `application/json` content type. When an organization YAML **file** is uploaded instead, its content must use `kind: Organization` with the nested shape `metadata.name` (handle, any top-level `id` is ignored) and `spec.displayName`; all other fields (including `cpRefId`) are read from `spec`. The YAML `spec` block additionally accepts `labels` (array of `{name, displayName}`) and `views` (array of `{id, displayName, labels}` — `id` becomes the view's handle) to bootstrap labels and views at creation time — these are not available via the `application/json` content type.|

> Example responses

> 201 Response

```json
{
  "id": "acme",
  "displayName": "Acme Corporation",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "idpRefId": "string",
  "cpRefId": "string",
  "configuration": {
    "devportalMode": "DEFAULT"
  },
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
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
  "message": "Conflict"
}
```

> 415 Response

```json
{
  "status": "error",
  "code": "UNSUPPORTED_MEDIA_TYPE",
  "message": "Content-Type must be application/json."
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

<h3 id="create-an-organization-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Organization created successfully.|[OrganizationResponse](schemas.md#schemaorganizationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|415|[Unsupported Media Type](https://tools.ietf.org/html/rfc7231#section-6.5.13)|Unsupported request media type.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-an-organization-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the created organization.|

## List organizations

<a id="opIdgetOrganizations"></a>

`GET /organizations`

> Code samples

```shell

curl -X GET https://localhost:3000/api/v0.9/organizations \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns all Developer Portal organizations visible to the admin context.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

> Example responses

> 200 Response

```json
{
  "list": [
    {
      "id": "acme",
      "displayName": "Acme Corporation",
      "businessOwner": "string",
      "businessOwnerContact": "string",
      "businessOwnerEmail": "user@example.com",
      "idpRefId": "string",
      "cpRefId": "string",
      "configuration": {
        "devportalMode": "DEFAULT"
      },
      "createdAt": "2019-08-24T14:15:22Z",
      "updatedAt": "2019-08-24T14:15:22Z"
    }
  ],
  "count": 1,
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

<h3 id="list-organizations-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of organization DTOs.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-organizations-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» list|[[OrganizationResponse](schemas.md#schemaorganizationresponse)]|false|none|none|
|»» id|string|false|none|The organization's handle (unique). Not the internal database uuid.|
|»» displayName|string|false|none|none|
|»» businessOwner|string¦null|false|none|none|
|»» businessOwnerContact|string¦null|false|none|none|
|»» businessOwnerEmail|string(email)¦null|false|none|none|
|»» idpRefId|string|false|none|The organization claim value asserted by the configured Identity Provider at SSO login. On every login, the portal matches the authenticated user's org claim against this value to resolve which organization they belong to — it must exactly match the IDP's claim, or login fails for that org's users. Distinct from `cpRefId`, which is unrelated to authentication.|
|»» cpRefId|string¦null|false|none|Control Plane reference ID. Included in outbound webhook event payloads so subscribers can correlate this organization with its Control Plane (Platform API) counterpart. Not used for authentication or org resolution.|
|»» configuration|object|false|none|Organization portal configuration. Always includes `devportalMode`; may contain additional free-form keys set by the caller.|
|»»» devportalMode|string|false|none|Controls the mode of the developer portal.|
|»» createdAt|string(date-time)¦null|false|none|none|
|»» updatedAt|string(date-time)¦null|false|none|none|
|» count|integer|false|none|Number of items returned in this page.|
|» pagination|[Pagination](schemas.md#schemapagination)|false|none|Standard pagination metadata returned with collection responses.|
|»» total|integer|true|none|Total number of records matching the query.|
|»» limit|integer|true|none|Maximum number of records returned in this response.|
|»» offset|integer|true|none|Number of records skipped before this page.|

#### Enumerated Values

|Property|Value|
|---|---|
|devportalMode|DEFAULT|
|devportalMode|MCP_SERVERS_ONLY|
|devportalMode|APIS_ONLY|

## Update an organization

<a id="opIdupdateOrganization"></a>

`PUT /organizations/{orgId}`

> Code samples

```shell

curl -X PUT https://localhost:3000/api/v0.9/organizations/{orgId} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates organization metadata, claim mappings, role mappings, and portal configuration.

> Payload

```json
{
  "displayName": "Acme Corporation",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "id": "acme",
  "idpRefId": "string",
  "cpRefId": "string",
  "configuration": {
    "devportalMode": "DEFAULT"
  }
}
```

```yaml
displayName: Acme Corporation
businessOwner: string
businessOwnerContact: string
businessOwnerEmail: user@example.com
id: acme
idpRefId: string
cpRefId: string
configuration:
  devportalMode: DEFAULT

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="update-an-organization-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[OrganizationUpdateRequest](schemas.md#schemaorganizationupdaterequest)|true|Organization update payload. Send JSON or an organization YAML file in the `organization` multipart field. The JSON example below applies only to the `application/json` content type. When an organization YAML **file** is uploaded instead, its content must use `kind: Organization` with the nested shape `metadata.name` (handle, any top-level `id` is ignored) and `spec.displayName`; all other fields (including `cpRefId`) are read from `spec`. The YAML `spec` block additionally accepts `labels` (upserted by name) and `views` (upserted by `id`, which becomes the view's handle, with `labels` replacing the view's label set) — these are not available via the `application/json` content type.|
|orgId|path|string|true|The organization's handle (also matches by name or IDP reference ID). Not the internal database uuid.|

> Example responses

> 200 Response

```json
{
  "id": "acme",
  "displayName": "Acme Corporation",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "idpRefId": "string",
  "cpRefId": "string",
  "configuration": {
    "devportalMode": "DEFAULT"
  },
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
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

<h3 id="update-an-organization-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Organization DTO returned by create, update, and lookup operations.|[OrganizationResponse](schemas.md#schemaorganizationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-an-organization-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

## Get an organization

<a id="opIdgetOrganization"></a>

`GET /organizations/{orgId}`

> Code samples

```shell

curl -X GET https://localhost:3000/api/v0.9/organizations/{orgId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves a single organization by organization ID, organization name, organization handle, or organization identifier.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-an-organization-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|The organization's handle (also matches by name or IDP reference ID). Not the internal database uuid.|

> Example responses

> 200 Response

```json
{
  "id": "acme",
  "displayName": "Acme Corporation",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "idpRefId": "string",
  "cpRefId": "string",
  "configuration": {
    "devportalMode": "DEFAULT"
  },
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

<h3 id="get-an-organization-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Organization DTO returned by create, update, and lookup operations.|[OrganizationResponse](schemas.md#schemaorganizationresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete an organization

<a id="opIddeleteOrganization"></a>

`DELETE /organizations/{orgId}`

> Code samples

```shell

curl -X DELETE https://localhost:3000/api/v0.9/organizations/{orgId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes an organization and returns no response body when deletion succeeds.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-an-organization-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|The organization's handle (also matches by name or IDP reference ID). Not the internal database uuid.|

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

<h3 id="delete-an-organization-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Organization deleted successfully.|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-an-organization-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
